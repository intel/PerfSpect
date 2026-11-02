// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package instructions

import (
	"reflect"
	"testing"
)

// TestAggregateBySymbolCombinesCorrectly uses figures taken from a real Spark profile,
// where org.apache...GeneratedIteratorForCodegenStage16.processNext() appeared 10 times
// across 2 executors because a tiered JIT recompiles hot methods.
//
// The two metric families must combine differently, which is the whole point of this
// function: HotPct is a share of the entire profile and sums, while StackPct is a share of
// one instance's own weight and must be averaged. Summing ten instances of ~10% StackPct
// would claim 100% stack traffic for a symbol that has nothing like that.
func TestAggregateBySymbolCombinesCorrectly(t *testing.T) {
	symbols := []AnalyzedSymbol{
		{
			Symbol:         Symbol{Name: "processNext()", DSO: "jitted-2291-39327.so", HotPct: 2.26},
			Pressure:       RegPressure{StackPct: 10.46, GPPct: 10.46, GPRegsUsed: 15, InsnCount: 2446},
			GlobalStackPct: 0.24,
		},
		{
			Symbol:         Symbol{Name: "processNext()", DSO: "jitted-2233-39658.so", HotPct: 2.08},
			Pressure:       RegPressure{StackPct: 9.27, GPPct: 9.27, GPRegsUsed: 15, InsnCount: 2475},
			GlobalStackPct: 0.19,
		},
	}
	got := AggregateBySymbol(symbols)
	if len(got) != 1 {
		t.Fatalf("aggregated to %d rows, want 1", len(got))
	}
	a := got[0]

	if a.Instances != 2 {
		t.Errorf("Instances = %d, want 2", a.Instances)
	}
	// Profile-wide shares sum.
	checkPct(t, "HotPct", a.HotPct, 4.34)
	checkPct(t, "GlobalStackPct", a.GlobalStackPct, 0.43)
	// Per-symbol shares are weighted by HotPct: (10.46*2.26 + 9.27*2.08) / 4.34
	want := (10.46*2.26 + 9.27*2.08) / 4.34
	checkPct(t, "StackPct", a.Pressure.StackPct, want)
	if a.Pressure.StackPct > 10.46 || a.Pressure.StackPct < 9.27 {
		t.Errorf("StackPct = %.4f, want it between the two inputs, never their sum", a.Pressure.StackPct)
	}
	// Counts describe the code, so the maximum is taken.
	if a.Pressure.GPRegsUsed != 15 {
		t.Errorf("GPRegsUsed = %d, want 15", a.Pressure.GPRegsUsed)
	}
	if a.Pressure.InsnCount != 2475 {
		t.Errorf("InsnCount = %d, want 2475 (max across recompilations)", a.Pressure.InsnCount)
	}
	// DSOs are listed heaviest first.
	if !reflect.DeepEqual(a.DSOs, []string{"jitted-2291-39327.so", "jitted-2233-39658.so"}) {
		t.Errorf("DSOs = %v, want the heavier instance first", a.DSOs)
	}
}

func TestAggregateBySymbolRanksByCombinedImpact(t *testing.T) {
	// A symbol that is unremarkable per instance but appears many times must outrank a
	// single heavier instance once combined. This is the case the aggregate table exists
	// to surface, and it is invisible in the per-instance table.
	symbols := []AnalyzedSymbol{
		{Symbol: Symbol{Name: "one_big", DSO: "a.so", HotPct: 3.0},
			Pressure: RegPressure{StackPct: 10}, GlobalStackPct: 0.30},
	}
	for i := 0; i < 8; i++ {
		symbols = append(symbols, AnalyzedSymbol{
			Symbol:   Symbol{Name: "many_small", DSO: "jitted-" + string(rune('a'+i)) + ".so", HotPct: 1.0},
			Pressure: RegPressure{StackPct: 10}, GlobalStackPct: 0.10,
		})
	}
	got := AggregateBySymbol(symbols)
	if len(got) != 2 {
		t.Fatalf("aggregated to %d rows, want 2", len(got))
	}
	if got[0].Name != "many_small" {
		t.Errorf("first = %q, want many_small (0.80 combined vs 0.30)", got[0].Name)
	}
	checkPct(t, "many_small GlobalStackPct", got[0].GlobalStackPct, 0.80)
	if got[0].Instances != 8 {
		t.Errorf("Instances = %d, want 8", got[0].Instances)
	}
	if len(got[0].DSOs) != 8 {
		t.Errorf("DSOs = %d, want 8 distinct", len(got[0].DSOs))
	}
}

func TestAggregateBySymbolTierRecomputed(t *testing.T) {
	// The tier must follow the combined StackPct, not any single instance's.
	symbols := []AnalyzedSymbol{
		{Symbol: Symbol{Name: "s", DSO: "a.so", HotPct: 1.0}, Pressure: RegPressure{StackPct: 40}},
		{Symbol: Symbol{Name: "s", DSO: "b.so", HotPct: 9.0}, Pressure: RegPressure{StackPct: 2}},
	}
	got := AggregateBySymbol(symbols)
	// weighted mean = (40*1 + 2*9) / 10 = 5.8 -> MED, not the HIGH of the first instance
	checkPct(t, "StackPct", got[0].Pressure.StackPct, 5.8)
	if tier := got[0].Pressure.Tier(); tier != TierMed {
		t.Errorf("Tier() = %q, want %q", tier, TierMed)
	}
}

func TestAggregateBySymbolAllCold(t *testing.T) {
	// Every instance at zero weight: a plain mean rather than a divide by zero.
	symbols := []AnalyzedSymbol{
		{Symbol: Symbol{Name: "s", DSO: "a.so", HotPct: 0}, Pressure: RegPressure{StackPct: 10}},
		{Symbol: Symbol{Name: "s", DSO: "b.so", HotPct: 0}, Pressure: RegPressure{StackPct: 20}},
	}
	got := AggregateBySymbol(symbols)
	if len(got) != 1 {
		t.Fatalf("aggregated to %d rows, want 1", len(got))
	}
	checkPct(t, "StackPct", got[0].Pressure.StackPct, 15.0)
}

func TestAggregateBySymbolEmpty(t *testing.T) {
	if got := AggregateBySymbol(nil); len(got) != 0 {
		t.Errorf("AggregateBySymbol(nil) = %v, want empty", got)
	}
}

// TestAggregateBySymbolOnRealFixtures checks the end-to-end path: two distinct symbols
// stay distinct, and nothing is merged that should not be.
func TestAggregateBySymbolOnRealFixtures(t *testing.T) {
	gp := AnalyzedSymbol{
		Symbol:   Symbol{Name: "gp_pressure", DSO: "workload", HotPct: 23.43},
		Pressure: AnalyzeRegPressure(loadFixture(t, "annotate_gp_pressure.stdio"), 0),
	}
	simd := AnalyzedSymbol{
		Symbol:   Symbol{Name: "simd_pressure", DSO: "workload", HotPct: 75.27},
		Pressure: AnalyzeRegPressure(loadFixture(t, "annotate_simd_pressure.stdio"), 0),
	}
	gp.GlobalStackPct = GlobalStackPct(gp.HotPct, gp.Pressure)
	simd.GlobalStackPct = GlobalStackPct(simd.HotPct, simd.Pressure)

	got := AggregateBySymbol([]AnalyzedSymbol{simd, gp})
	if len(got) != 2 {
		t.Fatalf("aggregated to %d rows, want 2 distinct symbols", len(got))
	}
	// Ranked by combined global impact, so the spill-dense symbol leads despite being
	// the colder of the two.
	if got[0].Name != "gp_pressure" {
		t.Errorf("first = %q, want gp_pressure", got[0].Name)
	}
	// A single instance must pass its figures through untouched.
	checkPct(t, "StackPct", got[0].Pressure.StackPct, 91.3791)
	if got[0].Instances != 1 {
		t.Errorf("Instances = %d, want 1", got[0].Instances)
	}
}
