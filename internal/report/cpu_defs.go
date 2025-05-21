package report

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// CPUDefinition - used to lookup micro architecture and channels by family, model, and stepping
//
//	The model and stepping fields will be interpreted as regular expressions
//	An empty stepping field means 'any' stepping
type CPUDefinition struct {
	MicroArchitecture  string
	Family             string
	Model              string
	Stepping           string
	Architecture       string
	MemoryChannelCount int
	LogicalThreadCount int
	CacheWayCount      int
}

// GetCPU retrieves the CPU structure that matches the provided args
func GetCPU(family, model, stepping string) (cpu CPUDefinition, err error) {
	return getCPUExtended(family, model, stepping, "", "")
}

var cpuDefinitions = []CPUDefinition{
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
	{MicroArchitecture: "SRF", Family: "6", Model: "175", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 0, LogicalThreadCount: 1, CacheWayCount: 12},           // Sierra Forest
	{MicroArchitecture: "SRF_SP", Family: "6", Model: "175", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 1, CacheWayCount: 12},        // Sierra Forest
	{MicroArchitecture: "SRF_AP", Family: "6", Model: "175", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 12, LogicalThreadCount: 1, CacheWayCount: 12},       // Sierra Forest
	{MicroArchitecture: "GNR", Family: "6", Model: "173", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 0, LogicalThreadCount: 2, CacheWayCount: 16},           // Granite Rapids - generic
	{MicroArchitecture: "GNR_X1", Family: "6", Model: "173", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 16},        // Granite Rapids - SP (MCC/LCC)
	{MicroArchitecture: "GNR_X2", Family: "6", Model: "173", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 16},        // Granite Rapids - SP (XCC)
	{MicroArchitecture: "GNR_X3", Family: "6", Model: "173", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 16},       // Granite Rapids - AP (UCC)
	{MicroArchitecture: "GNR_D", Family: "6", Model: "174", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 0, LogicalThreadCount: 2, CacheWayCount: 16},         // Granite Rapids - D
	{MicroArchitecture: "CWF", Family: "6", Model: "221", Stepping: "", Architecture: "x86_64", MemoryChannelCount: 12, LogicalThreadCount: 1, CacheWayCount: 0},           // Clearwater Forest - generic
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

// getCPUExtended retrieves the CPU structure that matches the provided args
// capid4 needed to differentiate EMR MCC from EMR XCC
//
//	capid4: $ lspci -s $(lspci | grep 325b | awk 'NR==1{{print $1}}') -xxx |  awk '$1 ~ /^90/{{print $9 $8 $7 $6; exit}}'
//
// devices needed to differentiate GNR X1/2/3
//
//	devices: $ lspci -d 8086:3258 | wc -l
func getCPUExtended(family, model, stepping, capid4, devices string) (cpu CPUDefinition, err error) {
	for _, info := range cpuDefinitions {
		// if family matches
		if info.Family == family {
			var reModel *regexp.Regexp
			reModel, err = regexp.Compile(info.Model)
			if err != nil {
				return
			}
			// if model matches
			if reModel.FindString(model) == model {
				// if there is a stepping
				if info.Stepping != "" {
					var reStepping *regexp.Regexp
					reStepping, err = regexp.Compile(info.Stepping)
					if err != nil {
						return
					}
					// if stepping does NOT match
					if reStepping.FindString(stepping) == "" {
						// no match
						continue
					}
				}
				cpu = info
				if cpu.Family == "6" && (cpu.Model == "143" || cpu.Model == "207" || cpu.Model == "173" || cpu.Model == "175") { // SPR, EMR, GNR, SRF
					cpu, err = getSpecificCPU(family, model, capid4, devices)
				}
				return
			}
		}
	}
	err = fmt.Errorf("CPU match not found for family %s, model %s, stepping %s", family, model, stepping)
	return
}

func GetCPUByMicroArchitecture(uarch string) (cpu CPUDefinition, err error) {
	for _, info := range cpuDefinitions {
		if strings.EqualFold(info.MicroArchitecture, uarch) {
			cpu = info
			return
		}
	}
	err = fmt.Errorf("CPU match not found for uarch %s", uarch)
	return
}

func getSpecificCPU(family, model, capid4, devices string) (cpu CPUDefinition, err error) {
	if family == "6" && model == "143" { // SPR
		cpu, err = getSPRCPU(capid4)
	} else if family == "6" && model == "207" { // EMR
		cpu, err = getEMRCPU(capid4)
	} else if family == "6" && model == "173" { // GNR
		cpu, err = getGNRCPU(devices)
	} else if family == "6" && model == "175" { // SRF
		cpu, err = getSRFCPU(devices)
	}
	return
}

func getSPRCPU(capid4 string) (cpu CPUDefinition, err error) {
	var uarch string
	if capid4 != "" {
		var bits int64
		var capid4Int int64
		capid4Int, err = strconv.ParseInt(capid4, 16, 64)
		if err != nil {
			return
		}
		bits = (capid4Int >> 6) & 0b11
		if bits == 3 {
			uarch = "SPR_XCC"
		} else if bits == 1 {
			uarch = "SPR_MCC"
		}
	}
	if uarch == "" {
		uarch = "SPR"
	}
	for _, info := range cpuDefinitions {
		if info.MicroArchitecture == uarch {
			cpu = info
			return
		}
	}
	err = fmt.Errorf("did not find matching SPR architecture in CPU database: %s", uarch)
	return
}

func getEMRCPU(capid4 string) (cpu CPUDefinition, err error) {
	var uarch string
	if capid4 != "" {
		var bits int64
		var capid4Int int64
		capid4Int, err = strconv.ParseInt(capid4, 16, 64)
		if err != nil {
			return
		}
		bits = (capid4Int >> 6) & 0b11
		if bits == 3 {
			uarch = "EMR_XCC"
		} else if bits == 1 {
			uarch = "EMR_MCC"
		}
	}
	if uarch == "" {
		uarch = "EMR"
	}
	for _, info := range cpuDefinitions {
		if info.MicroArchitecture == uarch {
			cpu = info
			return
		}
	}
	err = fmt.Errorf("did not find matching EMR architecture in CPU database: %s", uarch)
	return
}

func getGNRCPU(devices string) (cpu CPUDefinition, err error) {
	var uarch string
	if devices != "" {
		d, err := strconv.Atoi(devices)
		if err == nil && d != 0 {
			if d%5 == 0 { // device count is multiple of 5
				uarch = "GNR_X3"
			} else if d%4 == 0 { // device count is multiple of 4
				uarch = "GNR_X2"
			} else if d%3 == 0 { // device count is multiple of 3
				uarch = "GNR_X1"
			}
		}
	}
	if uarch == "" {
		uarch = "GNR"
	}
	for _, info := range cpuDefinitions {
		if info.MicroArchitecture == uarch {
			cpu = info
			return
		}
	}
	err = fmt.Errorf("did not find matching GNR architecture in CPU database: %s", uarch)
	return
}

func getSRFCPU(devices string) (cpu CPUDefinition, err error) {
	var uarch string
	if devices != "" {
		d, err := strconv.Atoi(devices)
		if err == nil && d != 0 {
			if d%3 == 0 { // device count is multiple of 3
				uarch = "SRF_SP"
			} else if d%4 == 0 { // device count is multiple of 4
				uarch = "SRF_AP"
			}
		}
	}
	if uarch == "" {
		uarch = "SRF"
	}
	for _, info := range cpuDefinitions {
		if info.MicroArchitecture == uarch {
			cpu = info
			return
		}
	}
	err = fmt.Errorf("did not find matching SRF architecture in CPU database: %s", uarch)
	return
}
