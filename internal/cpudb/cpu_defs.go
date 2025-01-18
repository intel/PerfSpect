package cpudb

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// CPU - used to lookup micro architecture and channels by family, model, and stepping
//
//	The model and stepping fields will be interpreted as regular expressions
//	An empty stepping field means 'any' stepping
type CPU struct {
	MicroArchitecture  string
	Family             string
	Model              string
	Stepping           string
	Architecture       string
	MemoryChannelCount int
	LogicalThreadCount int
	CacheWayCount      int
}

var cpus = CPUDB{
	// Intel Core CPUs
	{MicroArchitecture: "HSW", Family: "6", Model: "(50|69|70)", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0},             // Haswell
	{MicroArchitecture: "BDW", Family: "6", Model: "(61|71)", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0},                // Broadwell
	{MicroArchitecture: "SKL", Family: "6", Model: "(78|94)", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0},                // Skylake
	{MicroArchitecture: "KBL", Family: "6", Model: "(142|158)", Stepping: "9", Architecture: "x86_64", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0},             // Kabylake
	{MicroArchitecture: "CFL", Family: "6", Model: "(142|158)", Stepping: "(10|11|12|13)", Architecture: "x86_64", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Coffeelake
	{MicroArchitecture: "RKL", Family: "6", Model: "167", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0},                    // Rocket Lake
	{MicroArchitecture: "TGL", Family: "6", Model: "(140|141)", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0},              // Tiger Lake
	{MicroArchitecture: "ADL", Family: "6", Model: "(151|154)", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0},              // Alder Lake
	{MicroArchitecture: "MTL", Family: "6", Model: "170", Stepping: "4", Architecture: "x86_64", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0},                   // Meteor Lake
	// Intel Xeon CPUs
	{MicroArchitecture: "HSX", Family: "6", Model: "63", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 4, LogicalThreadCount: 2, CacheWayCount: 20},            // Haswell
	{MicroArchitecture: "BDX", Family: "6", Model: "(79|86)", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 4, LogicalThreadCount: 2, CacheWayCount: 20},       // Broadwell
	{MicroArchitecture: "SKX", Family: "6", Model: "85", Stepping: "(0|1|2|3|4)", Architecture: "x86_64", MemoryChannelCount: 6, LogicalThreadCount: 2, CacheWayCount: 11}, // Skylake
	{MicroArchitecture: "CLX", Family: "6", Model: "85", Stepping: "(5|6|7)", Architecture: "x86_64", MemoryChannelCount: 6, LogicalThreadCount: 2, CacheWayCount: 11},     // Cascadelake
	{MicroArchitecture: "CPX", Family: "6", Model: "85", Stepping: "11", Architecture: "x86_64", MemoryChannelCount: 6, LogicalThreadCount: 2, CacheWayCount: 11},          // Cooperlake
	{MicroArchitecture: "ICX", Family: "6", Model: "(106|108)", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 12},     // Icelake
	{MicroArchitecture: "SPR", Family: "6", Model: "143", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15},           // Sapphire Rapids - generic
	{MicroArchitecture: "SPR_MCC", Family: "6", Model: "143", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15},       // Sapphire Rapids - MCC
	{MicroArchitecture: "SPR_XCC", Family: "6", Model: "143", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15},       // Sapphire Rapids - XCC
	{MicroArchitecture: "EMR", Family: "6", Model: "207", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15},           // Emerald Rapids - generic
	{MicroArchitecture: "EMR_MCC", Family: "6", Model: "207", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15},       // Emerald Rapids - MCC
	{MicroArchitecture: "EMR_XCC", Family: "6", Model: "207", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 20},       // Emerald Rapids - XCC
	{MicroArchitecture: "SRF", Family: "6", Model: "175", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 1, CacheWayCount: 12},           // Sierra Forest
	{MicroArchitecture: "GNR", Family: "6", Model: "173", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 0, LogicalThreadCount: 2, CacheWayCount: 16},           // Granite Rapids - generic
	{MicroArchitecture: "GNR_X1", Family: "6", Model: "173", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 16},        // Granite Rapids - SP (MCC/LCC)
	{MicroArchitecture: "GNR_X2", Family: "6", Model: "173", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 16},        // Granite Rapids - SP (XCC)
	{MicroArchitecture: "GNR_X3", Family: "6", Model: "173", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 16},       // Granite Rapids - AP (UCC)
	{MicroArchitecture: "GNR_D", Family: "6", Model: "174", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 0, LogicalThreadCount: 2, CacheWayCount: 16},         // Granite Rapids - D (generic)
	// AMD CPUs
	{MicroArchitecture: "Naples", Family: "23", Model: "1", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 0},                     // Naples
	{MicroArchitecture: "Rome", Family: "23", Model: "49", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 0},                      // Rome
	{MicroArchitecture: "Milan", Family: "25", Model: "1", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 0},                      // Milan
	{MicroArchitecture: "Genoa", Family: "25", Model: "(1[6-9]|2[0-9]|3[01])", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 0}, // Genoa,  model 16-31
	{MicroArchitecture: "Bergamo", Family: "25", Model: "(16[0-9]|17[0-5])", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 0},   // Bergamo, model 160-175
	{MicroArchitecture: "Turin (Zen 5)", Family: "26", Model: "2", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 0},             // Turin (Zen 5)
	{MicroArchitecture: "Turin (Zen 5c)", Family: "26", Model: "17", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 0},           // Turin (Zen 5c)

	// ARM CPUs
	{MicroArchitecture: "Neoverse N1", Family: "", Model: "1", Stepping: "r3p1", Architecture: "arm64", MemoryChannelCount: 8, LogicalThreadCount: 1, CacheWayCount: 0}, // AWS Graviton 2
	{MicroArchitecture: "Neoverse V1", Family: "", Model: "1", Stepping: "r1p1", Architecture: "arm64", MemoryChannelCount: 8, LogicalThreadCount: 1, CacheWayCount: 0}, // AWS Graviton 3
}
