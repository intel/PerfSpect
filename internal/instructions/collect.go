// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package instructions

import (
	"sort"
	"strconv"
	"strings"

	"perfspect/internal/extract"
)

// Section names emitted by the hot-code collection script.
const (
	sectionPerfEvent      = "perf_event"
	sectionPerfReport     = "perf_report"
	sectionAnnotatePrefix = "annotate_"
	sectionUnresolved     = "unresolved"
	sectionJITInject      = "jit_inject"
	sectionSymfs          = "symfs"
)

// AnalyzedSymbol pairs a hot symbol with its register-pressure profile and APX signals.
type AnalyzedSymbol struct {
	Symbol
	Pressure RegPressure
	APX      APXSignals
	// GlobalStackPct is HotPct scaled by the symbol-local StackPct: the share of total
	// profile weight this symbol spends on stack-memory instructions.
	GlobalStackPct float64
}

// Result is the outcome of analyzing one target's hot-code collection.
type Result struct {
	// PerfEvent is the sampling event actually used, after precise-IP fallback.
	PerfEvent string
	// Analyzed holds the symbols that could be annotated, ranked by GlobalStackPct.
	Analyzed []AnalyzedSymbol
	// Unannotated holds symbols that were hot enough to select but produced no
	// disassembly, almost always because their binary is stripped.
	Unannotated []Symbol
	// Symfs is the filesystem root perf was told to resolve binaries under, empty if
	// --symfs was not used.
	Symfs string
	// JITInjected reports whether JIT analysis was requested and, if so, whether
	// folding the runtime's jitdump into the recording succeeded. Nil when --jit was
	// not used.
	JITInjected *bool
}

// UnannotatedDSOs lists the distinct DSOs that could not be annotated, with the share of
// profile weight lost to each, descending. Used to tell the user which binaries need
// symbols rather than silently reporting a thinner analysis.
func (r Result) UnannotatedDSOs() []DSOWeight {
	byDSO := make(map[string]float64)
	for _, s := range r.Unannotated {
		byDSO[s.DSO] += s.HotPct
	}
	out := make([]DSOWeight, 0, len(byDSO))
	for dso, pct := range byDSO {
		out = append(out, DSOWeight{DSO: dso, HotPct: pct, Reason: classifyDSO(dso)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].HotPct != out[j].HotPct {
			return out[i].HotPct > out[j].HotPct
		}
		return out[i].DSO < out[j].DSO
	})
	return out
}

// DSOWeight is a DSO and the profile weight attributed to it.
type DSOWeight struct {
	DSO    string
	HotPct float64
	Reason UnannotatableReason
}

// UnannotatableReason explains why a DSO produced no disassembly. The remedies differ
// completely, so telling a user to rebuild with -g when the code is JIT-compiled, or
// kernel-resident, is worse than saying nothing.
type UnannotatableReason string

const (
	// ReasonStripped is an ordinary binary built without a symbol table.
	ReasonStripped UnannotatableReason = "stripped"
	// ReasonJIT is code generated at runtime by a JIT such as the JVM or V8. Annotating
	// it requires the runtime to emit a jitdump and "perf inject --jit" to fold the
	// resulting ELF images into the recording, which this command does not yet do.
	ReasonJIT UnannotatableReason = "jit"
	// ReasonKernel is kernel-resident code, which needs kernel symbols enabled.
	ReasonKernel UnannotatableReason = "kernel"
	// ReasonPseudoDSO is a kernel-provided mapping such as [vdso], which has no
	// on-disk image to disassemble and is normally too small to matter.
	ReasonPseudoDSO UnannotatableReason = "pseudo-dso"
	// ReasonUnknown is a mapping perf could not attribute at all.
	ReasonUnknown UnannotatableReason = "unknown"
)

// Advice returns the remedy that applies to this reason, or "" when none does.
func (r UnannotatableReason) Advice() string {
	switch r {
	case ReasonStripped:
		return "rebuild with symbols (-g) or install the matching debug symbol package"
	case ReasonJIT:
		return "JIT-compiled code; per-symbol analysis of it is not supported yet"
	case ReasonKernel:
		return "pass --kernel, and provide a vmlinux with debug info"
	case ReasonPseudoDSO:
		return "kernel-provided mapping with no on-disk image; normally safe to ignore"
	default:
		return ""
	}
}

// classifyDSO infers why a DSO could not be annotated from the name perf gave it.
func classifyDSO(dso string) UnannotatableReason {
	switch {
	case strings.HasPrefix(dso, "[JIT]"):
		return ReasonJIT
	case dso == "[unknown]":
		return ReasonUnknown
	case strings.HasPrefix(dso, "[kernel"), strings.HasPrefix(dso, "[guest"):
		return ReasonKernel
	case strings.HasPrefix(dso, "["):
		return ReasonPseudoDSO
	default:
		return ReasonStripped
	}
}

// JITMethodCount returns the number of symbols from perf's per-method JIT ELF images. This,
// not the exit status of "perf inject --jit", says whether JIT analysis worked: that
// command succeeds even when there was no jitdump to fold in.
func (r Result) JITMethodCount() int {
	var n int
	for _, s := range r.Analyzed {
		if strings.HasPrefix(s.DSO, "jitted-") {
			n++
		}
	}
	for _, s := range r.Unannotated {
		if strings.HasPrefix(s.DSO, "jitted-") {
			n++
		}
	}
	return n
}

// JITMethodsAnalyzed returns how many JIT-compiled methods were successfully
// classified, as opposed to merely seen.
func (r Result) JITMethodsAnalyzed() int {
	var n int
	for _, s := range r.Analyzed {
		if strings.HasPrefix(s.DSO, "jitted-") {
			n++
		}
	}
	return n
}

// LostHotPct is the total profile weight in symbols that could not be annotated.
func (r Result) LostHotPct() float64 {
	var total float64
	for _, s := range r.Unannotated {
		total += s.HotPct
	}
	return total
}

// Analyze parses the hot-code collection script's output and classifies every annotated
// symbol, ranked by GlobalStackPct. topOps caps the hot stack instructions kept per
// symbol; pass 0 to skip.
func Analyze(scriptStdout string, topOps int) (Result, error) {
	sections := extract.GetSectionsFromOutput(scriptStdout)

	var result Result
	result.PerfEvent = strings.TrimSpace(sections[sectionPerfEvent])
	result.Symfs = strings.TrimSpace(sections[sectionSymfs])
	if status, ok := sections[sectionJITInject]; ok {
		injected := strings.TrimSpace(status) == "ok"
		result.JITInjected = &injected
	}

	reported, err := ParseReport(strings.NewReader(sections[sectionPerfReport]))
	if err != nil {
		return result, err
	}
	// Index pass-1 weights so pass-2 results can be matched back to them.
	type key struct{ sym, dso string }
	hotPct := make(map[key]float64, len(reported))
	kind := make(map[key]SymbolKind, len(reported))
	for _, s := range reported {
		k := key{s.Name, s.DSO}
		// perf can list the same symbol under multiple comms; accumulate.
		hotPct[k] += s.HotPct
		kind[k] = s.Kind
	}

	// Symbols perf could not resolve to a name. Their binaries are stripped, so there is
	// nothing to disassemble, but their weight still counts against coverage.
	result.Unannotated = append(result.Unannotated, parseUnresolved(sections[sectionUnresolved])...)

	for name, body := range sections {
		if !strings.HasPrefix(name, sectionAnnotatePrefix) {
			continue
		}
		sym, dso, disasm := splitAnnotateSection(body)
		if sym == "" {
			continue
		}
		k := key{sym, dso}
		s := Symbol{
			Name:   sym,
			DSO:    dso,
			Kind:   kind[k],
			HotPct: hotPct[k],
		}

		insns, err := ParseAnnotate(strings.NewReader(disasm))
		if err != nil {
			return result, err
		}
		pressure, apx := analyzeInstructions(insns, topOps)
		if pressure.InsnCount == 0 {
			// Selected as hot but nothing to classify: stripped, or annotation failed.
			result.Unannotated = append(result.Unannotated, s)
			continue
		}
		s.Insns = insns
		s.Annotated = true
		result.Analyzed = append(result.Analyzed, AnalyzedSymbol{
			Symbol:         s,
			Pressure:       pressure,
			APX:            apx,
			GlobalStackPct: GlobalStackPct(s.HotPct, pressure),
		})
	}

	// Sections arrive from a map, so impose a deterministic order. Rank by global stack
	// impact, falling back to hotness then name so equal values stay stable.
	sort.Slice(result.Analyzed, func(i, j int) bool {
		a, b := result.Analyzed[i], result.Analyzed[j]
		if a.GlobalStackPct != b.GlobalStackPct {
			return a.GlobalStackPct > b.GlobalStackPct
		}
		if a.HotPct != b.HotPct {
			return a.HotPct > b.HotPct
		}
		return a.Name < b.Name
	})
	sort.Slice(result.Unannotated, func(i, j int) bool {
		if result.Unannotated[i].HotPct != result.Unannotated[j].HotPct {
			return result.Unannotated[i].HotPct > result.Unannotated[j].HotPct
		}
		return result.Unannotated[i].Name < result.Unannotated[j].Name
	})
	return result, nil
}

// parseUnresolved reads the tab-separated "pct<TAB>dso<TAB>symbol" rows the collection
// script writes for symbols perf could not resolve to a name.
func parseUnresolved(body string) []Symbol {
	var out []Symbol
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		pct, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil {
			continue
		}
		out = append(out, Symbol{
			HotPct: pct,
			DSO:    strings.TrimSpace(parts[1]),
			Name:   strings.TrimSpace(parts[2]),
		})
	}
	return out
}

// splitAnnotateSection separates the "sym=" and "dso=" header lines the collection script
// writes from the disassembly that follows.
func splitAnnotateSection(body string) (sym, dso, disasm string) {
	lines := strings.Split(body, "\n")
	var start int
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "sym="):
			sym = strings.TrimPrefix(line, "sym=")
		case strings.HasPrefix(line, "dso="):
			dso = strings.TrimPrefix(line, "dso=")
		default:
			if sym != "" || dso != "" {
				start = i
			}
		}
		if sym != "" && dso != "" {
			start = i + 1
			break
		}
	}
	if start < len(lines) {
		disasm = strings.Join(lines[start:], "\n")
	}
	return sym, dso, disasm
}
