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
	CollectionInfoTableName   = "Instruction Analysis Collection"
)

// Menu labels
const (
	RegisterPressureMenuLabel = "Register Pressure"
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
		var parts []string
		for _, d := range result.UnannotatedDSOs() {
			parts = append(parts, fmt.Sprintf("%s (%.2f%% of samples)", d.DSO, d.HotPct))
		}
		strippedNote = fmt.Sprintf("%.2f%% of sampled weight could not be annotated: %s. "+
			"Rebuild these with symbols (-g) or install their debug symbol packages to get "+
			"per-symbol detail.", lost, strings.Join(parts, ", "))
	}

	return []table.Field{
		{Name: "Sampling Event", Values: []string{result.PerfEvent}},
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
