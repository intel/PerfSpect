// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package instructions

import "sort"

// SymbolAggregate combines every instance of one symbol across the profile. JIT profiles
// repeat symbols: each process has its own compiled copy, and a tiered JIT recompiles hot
// methods into new DSOs, so a method can appear a dozen times, each unremarkable alone.
type SymbolAggregate struct {
	Name string
	// Instances is how many separate compiled copies were analyzed.
	Instances int
	// DSOs are the distinct binaries the instances came from, heaviest first.
	DSOs []string
	// HotPct and GlobalStackPct are shares of the whole profile, so they sum.
	HotPct         float64
	GlobalStackPct float64
	// Pressure holds the combined profile. Its percentages are shares of a symbol's own
	// weight, so they are averaged rather than summed; its counts are taken as maxima.
	Pressure RegPressure
}

// AggregateBySymbol groups instances of a symbol, ranked by combined GlobalStackPct.
// Profile-wide shares (HotPct, GlobalStackPct) sum; per-symbol shares are HotPct-weighted
// means; register and instruction counts describe the code, so the maximum is taken.
func AggregateBySymbol(symbols []AnalyzedSymbol) []SymbolAggregate {
	type acc struct {
		agg SymbolAggregate
		// weighted sums of the per-symbol percentages, and the weight they carry
		wStack, wSpill, wReload, wGP, wSIMD, wUnk, weight float64
		// plain sums, used only when every instance carried zero weight
		sStack, sSpill, sReload, sGP, sSIMD, sUnk float64
		// the heaviest instance, whose hot instructions represent the group
		topHot float64
		dsoHot map[string]float64
	}
	groups := make(map[string]*acc)
	order := []string{}

	for _, s := range symbols {
		a, ok := groups[s.Name]
		if !ok {
			a = &acc{dsoHot: make(map[string]float64)}
			a.agg.Name = s.Name
			groups[s.Name] = a
			order = append(order, s.Name)
		}
		p := s.Pressure

		a.agg.Instances++
		a.agg.HotPct += s.HotPct
		a.agg.GlobalStackPct += s.GlobalStackPct
		a.dsoHot[s.DSO] += s.HotPct

		// Weight the per-symbol percentages by how much of the profile each instance
		// accounts for, so a cold recompilation cannot skew the result.
		w := s.HotPct
		a.weight += w
		a.wStack += p.StackPct * w
		a.wSpill += p.SpillPct * w
		a.wReload += p.ReloadPct * w
		a.wGP += p.GPPct * w
		a.wSIMD += p.SIMDPct * w
		a.wUnk += p.UnknownPct * w

		a.sStack += p.StackPct
		a.sSpill += p.SpillPct
		a.sReload += p.ReloadPct
		a.sGP += p.GPPct
		a.sSIMD += p.SIMDPct
		a.sUnk += p.UnknownPct

		a.agg.Pressure.GPRegsUsed = max(a.agg.Pressure.GPRegsUsed, p.GPRegsUsed)
		a.agg.Pressure.SIMDRegsUsed = max(a.agg.Pressure.SIMDRegsUsed, p.SIMDRegsUsed)
		a.agg.Pressure.InsnCount = max(a.agg.Pressure.InsnCount, p.InsnCount)
		a.agg.Pressure.UsesFramePointer = a.agg.Pressure.UsesFramePointer || p.UsesFramePointer

		if s.HotPct >= a.topHot {
			a.topHot = s.HotPct
			a.agg.Pressure.TopOps = p.TopOps
		}
	}

	out := make([]SymbolAggregate, 0, len(groups))
	for _, name := range order {
		a := groups[name]
		if a.weight > 0 {
			a.agg.Pressure.StackPct = a.wStack / a.weight
			a.agg.Pressure.SpillPct = a.wSpill / a.weight
			a.agg.Pressure.ReloadPct = a.wReload / a.weight
			a.agg.Pressure.GPPct = a.wGP / a.weight
			a.agg.Pressure.SIMDPct = a.wSIMD / a.weight
			a.agg.Pressure.UnknownPct = a.wUnk / a.weight
		} else {
			// Every instance was too cold to carry weight; fall back to a plain mean so
			// the row still says something rather than reporting zeros.
			n := float64(a.agg.Instances)
			a.agg.Pressure.StackPct = a.sStack / n
			a.agg.Pressure.SpillPct = a.sSpill / n
			a.agg.Pressure.ReloadPct = a.sReload / n
			a.agg.Pressure.GPPct = a.sGP / n
			a.agg.Pressure.SIMDPct = a.sSIMD / n
			a.agg.Pressure.UnknownPct = a.sUnk / n
		}
		a.agg.DSOs = sortDSOsByWeight(a.dsoHot)
		out = append(out, a.agg)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].GlobalStackPct != out[j].GlobalStackPct {
			return out[i].GlobalStackPct > out[j].GlobalStackPct
		}
		if out[i].HotPct != out[j].HotPct {
			return out[i].HotPct > out[j].HotPct
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// sortDSOsByWeight returns the DSO names ordered by the profile weight attributed to
// each, heaviest first, with ties broken by name so the output is deterministic.
func sortDSOsByWeight(weights map[string]float64) []string {
	names := make([]string, 0, len(weights))
	for name := range weights {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if weights[names[i]] != weights[names[j]] {
			return weights[names[i]] > weights[names[j]]
		}
		return names[i] < names[j]
	})
	return names
}
