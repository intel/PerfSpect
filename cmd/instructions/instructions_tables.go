// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package instructions

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"perfspect/internal/cpus"
	"perfspect/internal/instructions"
	"perfspect/internal/script"
	"perfspect/internal/table"
)

// instruction analysis table names
const (
	RegisterPressureTableName = "Register Pressure"
	APXEstimateTableName      = "APX Benefit Estimate"
	CollectionInfoTableName   = "Instruction Analysis Collection"
)

// Menu labels
const (
	RegisterPressureMenuLabel = "Register Pressure"
	APXEstimateMenuLabel      = "APX Estimate"
)

// topOpsPerSymbol is how many hot stack instructions are retained per symbol for display.
const topOpsPerSymbol = 5

var tableDefinitions = map[string]table.TableDefinition{
	RegisterPressureTableName: {
		Name:          RegisterPressureTableName,
		MenuLabel:     RegisterPressureMenuLabel,
		HasRows:       true,
		Architectures: []string{cpus.X86Architecture},
		ScriptNames: []string{
			script.HotCodeAnnotateScriptName,
		},
		NoDataFound: "No annotatable hot symbols were found. If the profiled binaries are " +
			"stripped, rebuild them with symbols or install their debug symbol packages.",
		FieldsFunc: registerPressureTableValues,
	},
	APXEstimateTableName: {
		Name:          APXEstimateTableName,
		MenuLabel:     APXEstimateMenuLabel,
		Architectures: []string{cpus.X86Architecture},
		ScriptNames: []string{
			script.HotCodeAnnotateScriptName,
		},
		NoDataFound: "No annotatable hot symbols were found, so there is nothing to project.",
		FieldsFunc:  apxEstimateTableValues,
	},
	CollectionInfoTableName: {
		Name:          CollectionInfoTableName,
		Architectures: []string{cpus.X86Architecture},
		ScriptNames: []string{
			script.HotCodeAnnotateScriptName,
		},
		FieldsFunc: collectionInfoTableValues,
	},
}

// analyze runs the shared parse-and-classify step over the collection script's output.
func analyze(outputs map[string]script.ScriptOutput) instructions.Result {
	stdout := outputs[script.HotCodeAnnotateScriptName].Stdout
	result, err := instructions.Analyze(stdout, topOpsPerSymbol)
	if err != nil {
		slog.Error("failed to analyze hot code", slog.String("error", err.Error()))
		return instructions.Result{}
	}
	return result
}

// registerPressureTableValues renders one row per annotated symbol.
//
// Rows are ordered by GSTK%, not HOT%: a symbol can be very hot with little stack traffic,
// or carry dense spill code while being too cold to matter. GSTK% is HOT% x STACK% / 100,
// so it ranks by the share of total profile time actually spent on spill and reload.
func registerPressureTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	result := analyze(outputs)

	var (
		gstk, hot, stack, spill, reload []string
		gp, simd, unknown, tier         []string
		symbol, dso, gpRegs, simdRegs   []string
		insnCount, topOps               []string
	)
	for _, s := range result.Analyzed {
		p := s.Pressure
		gstk = append(gstk, pct(s.GlobalStackPct))
		hot = append(hot, pct(s.HotPct))
		stack = append(stack, pct(p.StackPct))
		spill = append(spill, pct(p.SpillPct))
		reload = append(reload, pct(p.ReloadPct))
		gp = append(gp, pct(p.GPPct))
		simd = append(simd, pct(p.SIMDPct))
		unknown = append(unknown, pct(p.UnknownPct))
		tier = append(tier, p.Tier())
		symbol = append(symbol, s.Name)
		dso = append(dso, s.DSO)
		gpRegs = append(gpRegs, strconv.Itoa(p.GPRegsUsed))
		simdRegs = append(simdRegs, strconv.Itoa(p.SIMDRegsUsed))
		insnCount = append(insnCount, strconv.Itoa(p.InsnCount))
		topOps = append(topOps, formatTopOps(p.TopOps))
	}

	return []table.Field{
		{Name: "GSTK%", Description: "share of total profile weight this symbol spends on stack-memory instructions (HOT% x STACK% / 100)", Values: gstk},
		{Name: "HOT%", Description: "share of all profile samples in this symbol", Values: hot},
		{Name: "STACK%", Description: "share of this symbol's weight on stack-memory instructions; read-modify-write counts twice, so this can exceed 100%", Values: stack},
		{Name: "SPILL%", Description: "share of this symbol's weight storing registers to the frame", Values: spill},
		{Name: "RELD%", Description: "share of this symbol's weight loading registers back from the frame", Values: reload},
		{Name: "GP%", Description: "portion of stack traffic attributed to general-purpose registers", Values: gp},
		{Name: "SIMD%", Description: "portion of stack traffic attributed to vector registers", Values: simd},
		{Name: "UNK%", Description: "stack traffic whose direction could not be determined", Values: unknown},
		{Name: "PRES", Description: fmt.Sprintf("pressure tier: HIGH >= %.0f%% STACK, MED >= %.0f%%, else LOW", instructions.TierHighThreshold, instructions.TierMedThreshold), Values: tier},
		{Name: "Symbol", Values: symbol},
		{Name: "DSO", Values: dso},
		{Name: "GP Regs", Description: "distinct general-purpose registers referenced in this symbol", Values: gpRegs},
		{Name: "SIMD Regs", Description: "distinct vector registers referenced in this symbol", Values: simdRegs},
		{Name: "Instructions", Description: "instructions considered, excluding headers and stack-canary traffic", Values: insnCount},
		{Name: "Hot Stack Instructions", Description: "heaviest stack-memory instructions in this symbol", Values: topOps},
	}
}

// collectionInfoTableValues reports how the collection went, including any binaries that
// could not be annotated. A stripped binary yields no per-symbol detail at all, so it is
// surfaced explicitly rather than left to look like a thin result.
func collectionInfoTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	result := analyze(outputs)

	strippedNote := "none"
	if lost := result.LostHotPct(); lost > 0 {
		// The remedy depends on why each DSO failed, so advice is given per binary
		// rather than as one blanket instruction: telling someone to rebuild JIT code
		// with -g would send them down a dead end.
		var parts []string
		for _, d := range result.UnannotatedDSOs() {
			if advice := d.Reason.Advice(); advice != "" {
				parts = append(parts, fmt.Sprintf("%s (%.2f%% of samples, %s)", d.DSO, d.HotPct, advice))
			} else {
				parts = append(parts, fmt.Sprintf("%s (%.2f%% of samples)", d.DSO, d.HotPct))
			}
		}
		strippedNote = fmt.Sprintf("%.2f%% of sampled weight could not be annotated: %s",
			lost, strings.Join(parts, "; "))
	}

	jitNote := "not requested"
	if result.JITInjected != nil {
		jitSeen := result.JITMethodCount()
		jitOK := result.JITMethodsAnalyzed()
		switch {
		case !*result.JITInjected:
			// Almost always the runtime emitted no jitdump, which for the JVM means the
			// JVMTI agent was not loaded at launch. Say so, because the run otherwise
			// looks like a successful one that simply found no JIT code.
			jitNote = "perf inject --jit failed; JIT-compiled code was not analyzed"
		case jitSeen > 0 && jitOK == 0 && result.Symfs != "":
			// --jit and --symfs are mutually exclusive in practice. perf inject writes
			// the per-method ELF images to host paths, but --symfs makes perf prefix
			// every lookup with the given root, so it looks for them in the wrong place
			// and finds nothing. The run otherwise looks successful, reporting whatever
			// cold symbols did resolve, which is why this is called out explicitly.
			jitNote = fmt.Sprintf("BROKEN BY --symfs: %d JIT methods were found but none "+
				"could be annotated. perf inject writes per-method images to host paths, "+
				"which --symfs=%s prevents perf from finding. Re-run without --symfs to "+
				"analyze JIT code; the two cannot be combined in one collection.",
				jitSeen, result.Symfs)
		case jitSeen > 0 && jitOK == 0:
			jitNote = fmt.Sprintf("%d JIT methods were found but none could be annotated", jitSeen)
		case jitOK > 0:
			jitNote = fmt.Sprintf("%d JIT-compiled methods analyzed", jitOK)
		default:
			// inject succeeds trivially with nothing to fold in, so this is the common
			// case of the runtime never having emitted a jitdump.
			jitNote = "no JIT-compiled code was found. The runtime must emit a jitdump: " +
				"for the JVM, launch it with -XX:+UnlockDiagnosticVMOptions " +
				"-XX:+DumpPerfMapAtExit -agentpath:<linux-tools>/libperf-jvmti.so. " +
				"This cannot be enabled on an already-running process."
		}
	}

	return []table.Field{
		{Name: "Sampling Event", Values: []string{result.PerfEvent}},
		{Name: "JIT Analysis", Description: "whether JIT-compiled code was folded into the recording for analysis", Values: []string{jitNote}},
		{Name: "Symbols Analyzed", Values: []string{strconv.Itoa(len(result.Analyzed))}},
		{Name: "Symbols Not Annotatable", Values: []string{strconv.Itoa(len(result.Unannotated))}},
		{Name: "Unannotatable Binaries", Values: []string{strippedNote}},
	}
}

func pct(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

// topOpsSeparator joins the hot stack instructions into one line. It must stay on a
// single line so the txt, json and xlsx tables keep their row alignment; the HTML renderer
// splits on it again to show one instruction per line. A pipe cannot appear in x86 AT&T
// operands, so it is unambiguous.
const topOpsSeparator = " | "

// formatTopOps renders the hot stack instructions for one symbol as a single-line cell.
func formatTopOps(ops []instructions.HotOp) string {
	if len(ops) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ops))
	for _, op := range ops {
		// Collapse internal whitespace: perf pads mnemonics out to a fixed column, which
		// looks wrong once several instructions share a cell.
		text := strings.Join(strings.Fields(op.Text), " ")
		parts = append(parts, fmt.Sprintf("%.1f%% %s", op.Pct, text))
	}
	return strings.Join(parts, topOpsSeparator)
}

// apxEstimateTableValues projects what Intel APX might recover from the profiled code.
//
// The caveat field is not decoration. This is a model with hand-chosen coefficients,
// carried over from the reference implementation and not validated against APX hardware
// with an APX toolchain. Reported without that context, "~3.20%" reads like a measurement.
func apxEstimateTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	result := analyze(outputs)
	e := instructions.EstimateAPX(result.Analyzed)

	headline := fmt.Sprintf("%.2f%%", e.RealisticPct)
	if e.AlreadyUsesAPX {
		headline = fmt.Sprintf("%.2f%% (note: this code already uses r16-r31, so the "+
			"register file is not doubling and most of this projection does not apply)",
			e.RealisticPct)
	}

	avgRegs := "n/a"
	if e.AvgGPRegistersUsed >= 0 {
		avgRegs = fmt.Sprintf("%.0f of %d", e.AvgGPRegistersUsed, e.MaxGPRegisters)
	}

	return []table.Field{
		{Name: "Estimated Gain (realistic)", Description: "headline projection: GP spill cost x realization factor x secondary-feature lift", Values: []string{headline}},
		{Name: "Estimated Gain (upper bound)", Description: "if every GP spill were eliminated", Values: []string{fmt.Sprintf("%.2f%%", e.UpperBoundPct)}},
		{Name: "GP Spill/Reload Cost", Description: "share of total profile weight spent on general-purpose spill and reload; the ceiling this model works from", Values: []string{fmt.Sprintf("%.2f%%", e.GPStackPct)}},
		{Name: "GP Register Utilization", Description: "spill-weighted mean of distinct GP registers per symbol", Values: []string{avgRegs}},
		{Name: "Realization Factor", Description: "assumed fraction of GP spill traffic APX recovers, interpolated from register-file utilization", Values: []string{fmt.Sprintf("%.3f", e.GPRFactor)}},
		{Name: "Secondary Feature Lift", Description: "multiplier from NDD, CCMP and PUSH2/POP2 together", Values: []string{fmt.Sprintf("%.3f", e.Lift)}},
		{Name: "Attribution", Description: "how the realistic estimate splits across features", Values: []string{fmt.Sprintf(
			"GP registers %.2f%%, NDD %.2f%%, CCMP %.2f%%, PUSH2 %.2f%%",
			e.GPRContribPct, e.NDDContribPct, e.CCMPContribPct, e.Push2ContribPct)}},
		{Name: "Symbols Included", Description: fmt.Sprintf("symbols below %.1f%% HOT%% are excluded as too cold to be meaningful", instructions.APXMinSamplePct), Values: []string{fmt.Sprintf("%d (%d excluded)", e.SymbolsIncluded, e.SymbolsSkipped)}},
		{Name: "Caveat", Values: []string{"This is a projection from a heuristic model, not a measurement. " +
			"Its coefficients are judgement calls inherited from the reference tool and have " +
			"not been validated on APX hardware with an APX toolchain. Treat it as an " +
			"indication of whether this workload is worth investigating, not as a predicted speedup."}},
	}
}
