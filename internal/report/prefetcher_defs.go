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
	ShortName   string
	Description string
	Msr         int
	Bit         int
	Uarchs      []string
}

var PrefetcherDefs = []Prefetcher{
	{
		ShortName:   "L2 HW",
		Description: "L2 Hardware (MLC Streamer) is an L2 cache prefetcher",
		Msr:         MsrPrefetchControl,
		Bit:         0,
		Uarchs:      []string{"all"},
	},
	{
		ShortName:   "L2 Adj",
		Description: "Adjacent Cache Line (MLC Spatial) is an L2 cache prefetcher",
		Msr:         MsrPrefetchControl,
		Bit:         1,
		Uarchs:      []string{"all"},
	},
	{
		ShortName:   "DCU HW",
		Description: "L1 Data Cache Unit Hardware (DCU Streamer) Prefetcher",
		Msr:         MsrPrefetchControl,
		Bit:         2,
		Uarchs:      []string{"all"},
	},
	{
		ShortName:   "DCU IP",
		Description: "DCU Instruction Pointer prefetcher is an L1 cache prefetcher",
		Msr:         MsrPrefetchControl,
		Bit:         3,
		Uarchs:      []string{"all"},
	},
	{
		ShortName:   "DCU NP",
		Description: "DCU Next Page (DCU Next Page) is an L1 data cache prefetcher",
		Msr:         MsrPrefetchControl,
		Bit:         4,
		Uarchs:      []string{"all"},
	},
	{
		ShortName:   "AMP",
		Description: "Adaptive Multipath Probability (MLC AMP) is an L2 cache prefetcher. It predicts access patterns based on previous patterns and prefetches the corresponding cache lines.",
		Msr:         MsrPrefetchControl,
		Bit:         5,
		Uarchs:      []string{"SPR", "EMR", "GNR"},
	},
	{
		ShortName:   "LLCPP",
		Description: "Last Level Cache Page (MLC LLC Page) Prefetcher",
		Msr:         MsrPrefetchControl,
		Bit:         6,
		Uarchs:      []string{"GNR"},
	},
	{
		ShortName:   "AOP",
		Description: "L2 Array of Pointers (DCU AOP) Prefetcher",
		Msr:         MsrPrefetchControl,
		Bit:         7,
		Uarchs:      []string{"GNR"},
	},
	{
		ShortName:   "Homeless",
		Description: "Homeless prefetch allows early fetch of the demand miss into the MLC when we don’t have enough resources to track this demand in the L1 cache.",
		Msr:         MsrPrefetchers,
		Bit:         14,
		Uarchs:      []string{"SPR", "EMR", "GNR"},
	},
	{
		ShortName:   "LLC",
		Description: "Last level cache gives the core prefetcher the ability to prefetch data directly into the LLC without necessarily filling into the L1 and L2 caches first.",
		Msr:         MsrPrefetchers,
		Bit:         42,
		Uarchs:      []string{"SPR", "EMR", "GNR"},
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
