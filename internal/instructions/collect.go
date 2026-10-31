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
)

// AnalyzedSymbol pairs a hot symbol with its register-pressure profile.
type AnalyzedSymbol struct {
	Symbol
	Pressure RegPressure
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
		out = append(out, DSOWeight{DSO: dso, HotPct: pct})
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
}

// LostHotPct is the total profile weight in symbols that could not be annotated.
func (r Result) LostHotPct() float64 {
	var total float64
	for _, s := range r.Unannotated {
		total += s.HotPct
	}
	return total
}

// Analyze parses the hot-code collection script's output and classifies every symbol it
// annotated. topOps caps the hot stack instructions retained per symbol; pass 0 to skip.
//
// Symbols are returned ranked by GlobalStackPct, which is the ordering that reflects
// actual time spent on spill and reload rather than either hotness or local density alone.
func Analyze(scriptStdout string, topOps int) (Result, error) {
	sections := extract.GetSectionsFromOutput(scriptStdout)

	var result Result
	result.PerfEvent = strings.TrimSpace(sections[sectionPerfEvent])

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
		pressure := AnalyzeRegPressure(insns, topOps)
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
