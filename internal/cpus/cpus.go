// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Package cpus provides CPU definitions and lookup utilities for microarchitecture,
// family, model, and stepping, supporting both x86 and ARM architectures.
package cpus

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const IntelVendor = "GenuineIntel"
const AMDVendor = "AuthenticAMD"

const X86Architecture = "x86_64"
const ARMArchitecture = "aarch64"

var IntelFamilies = []int{6, 19}

// Microarchitecture constants
const (
	// Intel Core CPUs
	UarchHSW = "HSW"
	UarchBDW = "BDW"
	UarchSKL = "SKL"
	UarchKBL = "KBL"
	UarchCFL = "CFL"
	UarchRKL = "RKL"
	UarchTGL = "TGL"
	UarchADL = "ADL"
	UarchMTL = "MTL"
	UarchARL = "ARL"
	// Intel Xeon CPUs
	UarchHSX     = "HSX"
	UarchBDX     = "BDX"
	UarchSKX     = "SKX"
	UarchCLX     = "CLX"
	UarchCPX     = "CPX"
	UarchICX     = "ICX"
	UarchSPR     = "SPR"
	UarchSPR_MCC = "SPR_MCC" //lint:ignore ST1003 microarchitecture names use underscores to match Intel specifications
	UarchSPR_XCC = "SPR_XCC" //lint:ignore ST1003 microarchitecture names use underscores to match Intel specifications
	UarchEMR     = "EMR"
	UarchEMR_MCC = "EMR_MCC" //lint:ignore ST1003 microarchitecture names use underscores to match Intel specifications
	UarchEMR_XCC = "EMR_XCC" //lint:ignore ST1003 microarchitecture names use underscores to match Intel specifications
	UarchSRF     = "SRF"
	UarchSRF_SP  = "SRF_SP" //lint:ignore ST1003 microarchitecture names use underscores to match Intel specifications
	UarchSRF_AP  = "SRF_AP" //lint:ignore ST1003 microarchitecture names use underscores to match Intel specifications
	UarchGNR     = "GNR"
	UarchGNR_X1  = "GNR_X1" //lint:ignore ST1003 microarchitecture names use underscores to match Intel specifications
	UarchGNR_X2  = "GNR_X2" //lint:ignore ST1003 microarchitecture names use underscores to match Intel specifications
	UarchGNR_X3  = "GNR_X3" //lint:ignore ST1003 microarchitecture names use underscores to match Intel specifications
	UarchGNR_D   = "GNR-D"  //lint:ignore ST1003 microarchitecture names use underscores to match Intel specifications
	UarchCWF     = "CWF"
	UarchDMR     = "DMR"
	// AMD CPUs
	UarchNaples     = "Naples"
	UarchRome       = "Rome"
	UarchMilan      = "Milan"
	UarchGenoa      = "Genoa"
	UarchBergamo    = "Bergamo"
	UarchTurinZen5  = "Turin (Zen 5)"
	UarchTurinZen5c = "Turin (Zen 5c)"
	// ARM CPUs
	UarchGraviton2       = "Graviton2"
	UarchGraviton3       = "Graviton3"
	UarchGraviton4       = "Graviton4"
	UarchAxion           = "Axion"
	UarchAltraFamily     = "Altra Family"
	UarchAmpereOneAC03   = "AmpereOne AC03"
	UarchAmpereOneAC04   = "AmpereOne AC04"
	UarchAmpereOneAC04_1 = "AmpereOne AC04_1"
)

type CPUCharacteristics struct {
	MicroArchitecture  string
	MemoryChannelCount int
	LogicalThreadCount int
	CacheWayCount      int
}

type CPUIdentifierX86 struct {
	Family   string // from lscpu
	Model    string // from lscpu -- regex match
	Stepping string // from lscpu -- empty field means 'any' stepping, otherwise regex match
	Capid4   string // from lspci -- used to differentiate some CPUs
	Devices  string // from lspci -- used to differentiate some CPUs
}

type CPUIdentifierARM struct {
	Implementer   string // from /proc/cpuinfo
	Part          string // from /proc/cpuinfo
	DmidecodePart string // from dmidecode -- processor part number
}

// CPUIdentifier is a unified type that can hold either x86 or ARM identification
type CPUIdentifier struct {
	CPUIdentifierX86
	CPUIdentifierARM

	// Architecture hint (optional, can be auto-detected)
	Architecture string
}

// cpuCharacteristicsMap maps microarchitecture name to CPU characteristics
var cpuCharacteristicsMap = map[string]CPUCharacteristics{
	// Intel Core CPUs
	UarchHSW: {MicroArchitecture: UarchHSW, MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Haswell
	UarchBDW: {MicroArchitecture: UarchBDW, MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Broadwell
	UarchSKL: {MicroArchitecture: UarchSKL, MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Skylake
	UarchKBL: {MicroArchitecture: UarchKBL, MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Kabylake
	UarchCFL: {MicroArchitecture: UarchCFL, MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Coffeelake
	UarchRKL: {MicroArchitecture: UarchRKL, MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Rocket Lake
	UarchTGL: {MicroArchitecture: UarchTGL, MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Tiger Lake
	UarchADL: {MicroArchitecture: UarchADL, MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Alder Lake
	UarchMTL: {MicroArchitecture: UarchMTL, MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Meteor Lake
	UarchARL: {MicroArchitecture: UarchARL, MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Arrow Lake
	// Intel Xeon CPUs
	UarchHSX:     {MicroArchitecture: UarchHSX, MemoryChannelCount: 4, LogicalThreadCount: 2, CacheWayCount: 20},     // Haswell
	UarchBDX:     {MicroArchitecture: UarchBDX, MemoryChannelCount: 4, LogicalThreadCount: 2, CacheWayCount: 20},     // Broadwell
	UarchSKX:     {MicroArchitecture: UarchSKX, MemoryChannelCount: 6, LogicalThreadCount: 2, CacheWayCount: 11},     // Skylake
	UarchCLX:     {MicroArchitecture: UarchCLX, MemoryChannelCount: 6, LogicalThreadCount: 2, CacheWayCount: 11},     // Cascadelake
	UarchCPX:     {MicroArchitecture: UarchCPX, MemoryChannelCount: 6, LogicalThreadCount: 2, CacheWayCount: 11},     // Cooperlake
	UarchICX:     {MicroArchitecture: UarchICX, MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 12},     // Icelake
	UarchSPR:     {MicroArchitecture: UarchSPR, MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15},     // Sapphire Rapids - generic
	UarchSPR_MCC: {MicroArchitecture: UarchSPR_MCC, MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15}, // Sapphire Rapids - MCC
	UarchSPR_XCC: {MicroArchitecture: UarchSPR_XCC, MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15}, // Sapphire Rapids - XCC
	UarchEMR:     {MicroArchitecture: UarchEMR, MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15},     // Emerald Rapids - generic
	UarchEMR_MCC: {MicroArchitecture: UarchEMR_MCC, MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15}, // Emerald Rapids - MCC
	UarchEMR_XCC: {MicroArchitecture: UarchEMR_XCC, MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 20}, // Emerald Rapids - XCC
	UarchSRF:     {MicroArchitecture: UarchSRF, MemoryChannelCount: 0, LogicalThreadCount: 1, CacheWayCount: 12},     // Sierra Forest
	UarchSRF_SP:  {MicroArchitecture: UarchSRF_SP, MemoryChannelCount: 8, LogicalThreadCount: 1, CacheWayCount: 12},  // Sierra Forest
	UarchSRF_AP:  {MicroArchitecture: UarchSRF_AP, MemoryChannelCount: 12, LogicalThreadCount: 1, CacheWayCount: 12}, // Sierra Forest
	UarchGNR:     {MicroArchitecture: UarchGNR, MemoryChannelCount: 0, LogicalThreadCount: 2, CacheWayCount: 16},     // Granite Rapids - generic
	UarchGNR_X1:  {MicroArchitecture: UarchGNR_X1, MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 16},  // Granite Rapids - SP (MCC/LCC)
	UarchGNR_X2:  {MicroArchitecture: UarchGNR_X2, MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 16},  // Granite Rapids - SP (XCC)
	UarchGNR_X3:  {MicroArchitecture: UarchGNR_X3, MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 16}, // Granite Rapids - AP (UCC)
	UarchGNR_D:   {MicroArchitecture: UarchGNR_D, MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 16},   // Granite Rapids - D
	UarchCWF:     {MicroArchitecture: UarchCWF, MemoryChannelCount: 12, LogicalThreadCount: 1, CacheWayCount: 0},     // Clearwater Forest - generic
	UarchDMR:     {MicroArchitecture: UarchDMR, MemoryChannelCount: 16, LogicalThreadCount: 1, CacheWayCount: 0},     // Diamond Rapids
	// AMD CPUs
	UarchNaples:     {MicroArchitecture: UarchNaples, MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 0},      // Naples
	UarchRome:       {MicroArchitecture: UarchRome, MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 0},        // Rome
	UarchMilan:      {MicroArchitecture: UarchMilan, MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 0},       // Milan
	UarchGenoa:      {MicroArchitecture: UarchGenoa, MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 0},      // Genoa
	UarchBergamo:    {MicroArchitecture: UarchBergamo, MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 0},    // Bergamo
	UarchTurinZen5:  {MicroArchitecture: UarchTurinZen5, MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 0},  // Turin (Zen 5)
	UarchTurinZen5c: {MicroArchitecture: UarchTurinZen5c, MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 0}, // Turin (Zen 5c)
	// ARM CPUs
	UarchGraviton2:       {MicroArchitecture: UarchGraviton2, MemoryChannelCount: 8, LogicalThreadCount: 1},        // AWS Graviton 2 ([m|c|r]6g) Neoverse-N1
	UarchGraviton3:       {MicroArchitecture: UarchGraviton3, MemoryChannelCount: 8, LogicalThreadCount: 1},        // AWS Graviton 3 ([m|c|r]7g) Neoverse-V1
	UarchGraviton4:       {MicroArchitecture: UarchGraviton4, MemoryChannelCount: 12, LogicalThreadCount: 1},       // AWS Graviton 4 ([m|c|r]8g) Neoverse-V2
	UarchAxion:           {MicroArchitecture: UarchAxion, MemoryChannelCount: 12, LogicalThreadCount: 1},           // GCP Axion (c4a) Neoverse-V2
	UarchAltraFamily:     {MicroArchitecture: UarchAltraFamily, MemoryChannelCount: 8, LogicalThreadCount: 1},      // Ampere Altra
	UarchAmpereOneAC03:   {MicroArchitecture: UarchAmpereOneAC03, MemoryChannelCount: 8, LogicalThreadCount: 1},    // AmpereOne AC03
	UarchAmpereOneAC04:   {MicroArchitecture: UarchAmpereOneAC04, MemoryChannelCount: 8, LogicalThreadCount: 1},    // AmpereOne AC04
	UarchAmpereOneAC04_1: {MicroArchitecture: UarchAmpereOneAC04_1, MemoryChannelCount: 12, LogicalThreadCount: 1}, // AmpereOne AC04_1
}

// cpuIdentifiersX86 maps x86 CPU identification to microarchitecture names
var cpuIdentifiersX86 = []struct {
	Identifier        CPUIdentifierX86
	MicroArchitecture string
}{
	// Intel Core CPUs
	{CPUIdentifierX86{Family: "6", Model: "(50|69|70)", Stepping: "", Capid4: "", Devices: ""}, UarchHSW},             // Haswell
	{CPUIdentifierX86{Family: "6", Model: "(61|71)", Stepping: "", Capid4: "", Devices: ""}, UarchBDW},                // Broadwell
	{CPUIdentifierX86{Family: "6", Model: "(78|94)", Stepping: "", Capid4: "", Devices: ""}, UarchSKL},                // Skylake
	{CPUIdentifierX86{Family: "6", Model: "(142|158)", Stepping: "9", Capid4: "", Devices: ""}, UarchKBL},             // Kabylake
	{CPUIdentifierX86{Family: "6", Model: "(142|158)", Stepping: "(10|11|12|13)", Capid4: "", Devices: ""}, UarchCFL}, // Coffeelake
	{CPUIdentifierX86{Family: "6", Model: "167", Stepping: "", Capid4: "", Devices: ""}, UarchRKL},                    // Rocket Lake
	{CPUIdentifierX86{Family: "6", Model: "(140|141)", Stepping: "", Capid4: "", Devices: ""}, UarchTGL},              // Tiger Lake
	{CPUIdentifierX86{Family: "6", Model: "(151|154)", Stepping: "", Capid4: "", Devices: ""}, UarchADL},              // Alder Lake
	{CPUIdentifierX86{Family: "6", Model: "170", Stepping: "4", Capid4: "", Devices: ""}, UarchMTL},                   // Meteor Lake
	{CPUIdentifierX86{Family: "6", Model: "197", Stepping: "2", Capid4: "", Devices: ""}, UarchARL},                   // Arrow Lake
	// Intel Xeon CPUs
	{CPUIdentifierX86{Family: "6", Model: "63", Stepping: "", Capid4: "", Devices: ""}, UarchHSX},            // Haswell
	{CPUIdentifierX86{Family: "6", Model: "(79|86)", Stepping: "", Capid4: "", Devices: ""}, UarchBDX},       // Broadwell
	{CPUIdentifierX86{Family: "6", Model: "85", Stepping: "(0|1|2|3|4)", Capid4: "", Devices: ""}, UarchSKX}, // Skylake
	{CPUIdentifierX86{Family: "6", Model: "85", Stepping: "(5|6|7)", Capid4: "", Devices: ""}, UarchCLX},     // Cascadelake
	{CPUIdentifierX86{Family: "6", Model: "85", Stepping: "11", Capid4: "", Devices: ""}, UarchCPX},          // Cooperlake
	{CPUIdentifierX86{Family: "6", Model: "(106|108)", Stepping: "", Capid4: "", Devices: ""}, UarchICX},     // Icelake
	{CPUIdentifierX86{Family: "6", Model: "143", Stepping: "", Capid4: "", Devices: ""}, UarchSPR},           // Sapphire Rapids
	{CPUIdentifierX86{Family: "6", Model: "207", Stepping: "", Capid4: "", Devices: ""}, UarchEMR},           // Emerald Rapids
	{CPUIdentifierX86{Family: "6", Model: "175", Stepping: "", Capid4: "", Devices: ""}, UarchSRF},           // Sierra Forest
	{CPUIdentifierX86{Family: "6", Model: "173", Stepping: "", Capid4: "", Devices: ""}, UarchGNR},           // Granite Rapids
	{CPUIdentifierX86{Family: "6", Model: "174", Stepping: "", Capid4: "", Devices: ""}, UarchGNR_D},         // Granite Rapids - D
	{CPUIdentifierX86{Family: "6", Model: "221", Stepping: "", Capid4: "", Devices: ""}, UarchCWF},           // Clearwater Forest
	{CPUIdentifierX86{Family: "19", Model: "1", Stepping: "", Capid4: "", Devices: ""}, UarchDMR},            // Diamond Rapids
	// AMD CPUs
	{CPUIdentifierX86{Family: "23", Model: "1", Stepping: "", Capid4: "", Devices: ""}, UarchNaples},                    // Naples
	{CPUIdentifierX86{Family: "23", Model: "49", Stepping: "", Capid4: "", Devices: ""}, UarchRome},                     // Rome
	{CPUIdentifierX86{Family: "25", Model: "1", Stepping: "", Capid4: "", Devices: ""}, UarchMilan},                     // Milan
	{CPUIdentifierX86{Family: "25", Model: "(1[6-9]|2[0-9]|3[01])", Stepping: "", Capid4: "", Devices: ""}, UarchGenoa}, // Genoa, model 16-31
	{CPUIdentifierX86{Family: "25", Model: "(16[0-9]|17[0-5])", Stepping: "", Capid4: "", Devices: ""}, UarchBergamo},   // Bergamo, model 160-175
	{CPUIdentifierX86{Family: "26", Model: "2", Stepping: "", Capid4: "", Devices: ""}, UarchTurinZen5},                 // Turin (Zen 5)
	{CPUIdentifierX86{Family: "26", Model: "17", Stepping: "", Capid4: "", Devices: ""}, UarchTurinZen5c},               // Turin (Zen 5c)
}

// cpuIdentifiersARM maps ARM CPU identification to microarchitecture names
var cpuIdentifiersARM = []struct {
	Identifier        CPUIdentifierARM
	MicroArchitecture string
}{
	{CPUIdentifierARM{Implementer: "0x41", Part: "0xd0c", DmidecodePart: "AWS Graviton2"}, UarchGraviton2},   // AWS Graviton 2 ([m|c|r]6g) Neoverse-N1
	{CPUIdentifierARM{Implementer: "0x41", Part: "0xd40", DmidecodePart: "AWS Graviton3"}, UarchGraviton3},   // AWS Graviton 3 ([m|c|r]7g) Neoverse-V1
	{CPUIdentifierARM{Implementer: "0x41", Part: "0xd4f", DmidecodePart: "AWS Graviton4"}, UarchGraviton4},   // AWS Graviton 4 ([m|c|r]8g) Neoverse-V2
	{CPUIdentifierARM{Implementer: "0x41", Part: "0xd4f", DmidecodePart: "Not Specified"}, UarchAxion},       // GCP Axion (c4a) Neoverse-V2
	{CPUIdentifierARM{Implementer: "0x41", Part: "0xd0c", DmidecodePart: "Not Specified"}, UarchAltraFamily}, // Ampere Altra
	{CPUIdentifierARM{Implementer: "0xc0", Part: "0xac3", DmidecodePart: ""}, UarchAmpereOneAC03},            // AmpereOne AC03
	{CPUIdentifierARM{Implementer: "0xc0", Part: "0xac4", DmidecodePart: "X"}, UarchAmpereOneAC04},           // AmpereOne AC04
	{CPUIdentifierARM{Implementer: "0xc0", Part: "0xac4", DmidecodePart: "M"}, UarchAmpereOneAC04_1},         // AmpereOne AC04_1
}

// NewCPUIdentifier creates a CPUIdentifier with all data elements
func NewCPUIdentifier(family, model, stepping, capid4, devices, implementer, part, dmidecodePart, architecture string) CPUIdentifier {
	return CPUIdentifier{
		CPUIdentifierX86: CPUIdentifierX86{
			Family:   family,
			Model:    model,
			Stepping: stepping,
			Capid4:   capid4,
			Devices:  devices,
		},
		CPUIdentifierARM: CPUIdentifierARM{
			Implementer:   implementer,
			Part:          part,
			DmidecodePart: dmidecodePart,
		},
		Architecture: architecture,
	}
}

// NewX86Identifier creates a CPUIdentifier for x86/AMD CPUs with extended parameters
func NewX86Identifier(family, model, stepping, capid4, devices string) CPUIdentifier {
	return CPUIdentifier{
		CPUIdentifierX86: CPUIdentifierX86{
			Family:   family,
			Model:    model,
			Stepping: stepping,
			Capid4:   capid4,
			Devices:  devices,
		},
		Architecture: X86Architecture,
	}
}

// NewARMIdentifier creates a CPUIdentifier for ARM CPUs
func NewARMIdentifier(implementer, part, dmidecodePart string) CPUIdentifier {
	return CPUIdentifier{
		CPUIdentifierARM: CPUIdentifierARM{
			Implementer:   implementer,
			Part:          part,
			DmidecodePart: dmidecodePart,
		},
		Architecture: ARMArchitecture,
	}
}

// GetCPU is a unified function that retrieves CPU characteristics for both x86 and ARM
func GetCPU(id CPUIdentifier) (CPUCharacteristics, error) {
	// Auto-detect architecture if not specified
	arch := id.Architecture
	if arch == "" {
		if id.Implementer == "" && id.Family != "" && id.Model != "" {
			arch = X86Architecture
		} else if id.Implementer != "" || id.Part != "" {
			arch = ARMArchitecture
		} else {
			return CPUCharacteristics{}, fmt.Errorf("unable to determine CPU architecture")
		}
	}

	// Route to appropriate handler
	switch arch {
	case X86Architecture:
		return getCPUX86(id.Family, id.Model, id.Stepping, id.Capid4, id.Devices)
	case ARMArchitecture:
		return getCPUARM(id.Implementer, id.Part, id.DmidecodePart)
	}

	return CPUCharacteristics{}, fmt.Errorf("unsupported architecture: %s", arch)
}

// getCPUARM is an internal helper for ARM CPU lookup
func getCPUARM(implementer, part, dmidecodePart string) (cpu CPUCharacteristics, err error) {
	for _, entry := range cpuIdentifiersARM {
		id := entry.Identifier
		// any value specified in the definition must match
		if id.Implementer != "" && id.Implementer != implementer {
			continue
		}
		if id.Part != "" && id.Part != part {
			continue
		}
		if id.DmidecodePart != "" && id.DmidecodePart != dmidecodePart {
			continue
		}
		// Found matching identifier, look up characteristics
		uarch := entry.MicroArchitecture
		var ok bool
		cpu, ok = cpuCharacteristicsMap[uarch]
		if !ok {
			err = fmt.Errorf("CPU characteristics not found for microarchitecture %s", uarch)
			return
		}
		return
	}
	err = fmt.Errorf("CPU match not found for implementer %s, part %s, dmidecode part %s", implementer, part, dmidecodePart)
	return
}

// getCPUX86 is an internal helper for x86/AMD CPU lookup
// capid4 needed to differentiate EMR MCC from EMR XCC
//
//	capid4: $ lspci -s $(lspci | grep 325b | awk 'NR==1{{print $1}}') -xxx |  awk '$1 ~ /^90/{{print $9 $8 $7 $6; exit}}'
//
// devices needed to differentiate GNR X1/2/3
//
//	devices: $ lspci -d 8086:3258 | wc -l
func getCPUX86(family, model, stepping, capid4, devices string) (cpu CPUCharacteristics, err error) {
	for _, entry := range cpuIdentifiersX86 {
		id := entry.Identifier
		// if family matches
		if id.Family == family {
			var reModel *regexp.Regexp
			reModel, err = regexp.Compile(id.Model)
			if err != nil {
				return
			}
			// if model matches
			if reModel.FindString(model) == model {
				// if there is a stepping
				if id.Stepping != "" {
					var reStepping *regexp.Regexp
					reStepping, err = regexp.Compile(id.Stepping)
					if err != nil {
						return
					}
					// if stepping does NOT match
					if reStepping.FindString(stepping) == "" {
						// no match
						continue
					}
				}
				// Found matching identifier
				uarch := entry.MicroArchitecture
				if family == "6" && (model == "143" || model == "207" || model == "173" || model == "175") { // SPR, EMR, GNR, SRF
					uarch, err = getSpecificMicroArchitecture(family, model, capid4, devices)
					if err != nil {
						return
					}
				}
				// Look up characteristics
				var ok bool
				cpu, ok = cpuCharacteristicsMap[uarch]
				if !ok {
					err = fmt.Errorf("CPU characteristics not found for microarchitecture %s", uarch)
					return
				}
				return
			}
		}
	}
	err = fmt.Errorf("CPU match not found for family %s, model %s, stepping %s", family, model, stepping)
	return
}

func GetCPUByMicroArchitecture(uarch string) (cpu CPUCharacteristics, err error) {
	// Try exact match first
	if chars, ok := cpuCharacteristicsMap[uarch]; ok {
		cpu = chars
		return
	}
	// Try case-insensitive match
	for key, chars := range cpuCharacteristicsMap {
		if strings.EqualFold(key, uarch) {
			cpu = chars
			return
		}
	}
	err = fmt.Errorf("CPU match not found for uarch %s", uarch)
	return
}

// IsIntelCPUFamily checks if the CPU family corresponds to Intel CPUs.
func IsIntelCPUFamily(family int) bool {
	return slices.Contains(IntelFamilies, family)
}

// IsIntelCPUFamilyStr checks if the CPU family string corresponds to Intel CPUs.
func IsIntelCPUFamilyStr(familyStr string) bool {
	family, err := strconv.Atoi(familyStr)
	if err != nil {
		return false
	}
	return IsIntelCPUFamily(family)
}

func getSpecificMicroArchitecture(family, model, capid4, devices string) (uarch string, err error) {
	if family == "6" && model == "143" { // SPR
		uarch, err = getSPRMicroArchitecture(capid4)
	} else if family == "6" && model == "207" { // EMR
		uarch, err = getEMRMicroArchitecture(capid4)
	} else if family == "6" && model == "173" { // GNR
		uarch, err = getGNRMicroArchitecture(devices)
	} else if family == "6" && model == "175" { // SRF
		uarch, err = getSRFMicroArchitecture(devices)
	}
	return
}

func getSPRMicroArchitecture(capid4 string) (uarch string, err error) {
	if capid4 != "" {
		var bits int64
		var capid4Int int64
		capid4Int, err = strconv.ParseInt(capid4, 16, 64)
		if err != nil {
			return
		}
		bits = (capid4Int >> 6) & 0b11
		switch bits {
		case 3:
			uarch = UarchSPR_XCC
		case 1:
			uarch = UarchSPR_MCC
		}
	}
	if uarch == "" {
		uarch = UarchSPR
	}
	return
}

func getEMRMicroArchitecture(capid4 string) (uarch string, err error) {
	if capid4 != "" {
		var bits int64
		var capid4Int int64
		capid4Int, err = strconv.ParseInt(capid4, 16, 64)
		if err != nil {
			return
		}
		bits = (capid4Int >> 6) & 0b11
		switch bits {
		case 3:
			uarch = UarchEMR_XCC
		case 1:
			uarch = UarchEMR_MCC
		}
	}
	if uarch == "" {
		uarch = UarchEMR
	}
	return
}

func getGNRMicroArchitecture(devices string) (uarch string, err error) {
	if devices != "" {
		d, err := strconv.Atoi(devices)
		if err == nil && d != 0 {
			if d%5 == 0 { // device count is multiple of 5
				uarch = UarchGNR_X3
			} else if d%4 == 0 { // device count is multiple of 4
				uarch = UarchGNR_X2
			} else if d%3 == 0 { // device count is multiple of 3
				uarch = UarchGNR_X1
			}
		}
	}
	if uarch == "" {
		uarch = UarchGNR
	}
	return
}

func getSRFMicroArchitecture(devices string) (uarch string, err error) {
	if devices != "" {
		d, err := strconv.Atoi(devices)
		if err == nil && d != 0 {
			if d%3 == 0 { // device count is multiple of 3
				uarch = UarchSRF_SP
			} else if d%4 == 0 { // device count is multiple of 4
				uarch = UarchSRF_AP
			}
		}
	}
	if uarch == "" {
		uarch = UarchSRF
	}
	return
}
