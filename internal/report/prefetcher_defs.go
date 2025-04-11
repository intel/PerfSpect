package report

import "fmt"

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// prefetcher_defs.go
// prefetchers are enabled when associated bit in msr is 0

const (
	MsrPrefetchControl = 0x1a4
	MsrPrefetchers     = 0x6d
)

type Prefetcher struct {
	ShortName string
	LongName  string
	Msr       int
	Bit       int
	Uarchs    []string
}

var PrefetcherDefs = []Prefetcher{
	{
		ShortName: "L2 HW",
		LongName:  "L2 Hardware",
		Msr:       MsrPrefetchControl,
		Bit:       0,
		Uarchs:    []string{"all"},
	},
	{
		ShortName: "L2 Adj",
		LongName:  "L2 Adjacent Cache Line",
		Msr:       MsrPrefetchControl,
		Bit:       1,
		Uarchs:    []string{"all"},
	},
	{
		ShortName: "DCU HW",
		LongName:  "L1 Data Cache Unit Hardware",
		Msr:       MsrPrefetchControl,
		Bit:       2,
		Uarchs:    []string{"all"},
	},
	{
		ShortName: "DCU IP",
		LongName:  "L1 Data Cache Unit Instruction",
		Msr:       MsrPrefetchControl,
		Bit:       3,
		Uarchs:    []string{"all"},
	},
	{
		ShortName: "AMP",
		LongName:  "AMP",
		Msr:       MsrPrefetchControl,
		Bit:       5,
		Uarchs:    []string{"SPR", "EMR", "GNR"},
	},
	{
		ShortName: "LLCPP",
		LongName:  "LLC Page",
		Msr:       MsrPrefetchControl,
		Bit:       6,
		Uarchs:    []string{"GNR"},
	},
	{
		ShortName: "AOP",
		LongName:  "Array of Pointers",
		Msr:       MsrPrefetchControl,
		Bit:       7,
		Uarchs:    []string{"GNR"},
	},
	{
		ShortName: "Homeless",
		LongName:  "Homeless",
		Msr:       MsrPrefetchers,
		Bit:       14,
		Uarchs:    []string{"SPR", "EMR", "GNR"},
	},
	{
		ShortName: "LLC",
		LongName:  "LLC",
		Msr:       MsrPrefetchers,
		Bit:       42,
		Uarchs:    []string{"SPR", "EMR", "GNR"},
	},
}

// GetPrefetcherDefByName returns the Prefetcher definition by its short name.
// It returns error if the Prefetcher is not found.
func GetPrefetcherDefByName(name string) (Prefetcher, error) {
	for _, p := range PrefetcherDefs {
		if p.ShortName == name {
			return p, nil
		}
	}
	return Prefetcher{}, fmt.Errorf("prefetcher %s not found", name)
}
