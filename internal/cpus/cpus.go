// Package cpus provides CPU definitions and lookup utilities for microarchitecture,
// family, model, and stepping, supporting both x86 and ARM architectures.
package cpus

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

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
	"HSW": {MicroArchitecture: "HSW", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Haswell
	"BDW": {MicroArchitecture: "BDW", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Broadwell
	"SKL": {MicroArchitecture: "SKL", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Skylake
	"KBL": {MicroArchitecture: "KBL", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Kabylake
	"CFL": {MicroArchitecture: "CFL", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Coffeelake
	"RKL": {MicroArchitecture: "RKL", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Rocket Lake
	"TGL": {MicroArchitecture: "TGL", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Tiger Lake
	"ADL": {MicroArchitecture: "ADL", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Alder Lake
	"MTL": {MicroArchitecture: "MTL", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Meteor Lake
	"ARL": {MicroArchitecture: "ARL", MemoryChannelCount: 2, LogicalThreadCount: 2, CacheWayCount: 0}, // Arrow Lake
	// Intel Xeon CPUs
	"HSX":     {MicroArchitecture: "HSX", MemoryChannelCount: 4, LogicalThreadCount: 2, CacheWayCount: 20},     // Haswell
	"BDX":     {MicroArchitecture: "BDX", MemoryChannelCount: 4, LogicalThreadCount: 2, CacheWayCount: 20},     // Broadwell
	"SKX":     {MicroArchitecture: "SKX", MemoryChannelCount: 6, LogicalThreadCount: 2, CacheWayCount: 11},     // Skylake
	"CLX":     {MicroArchitecture: "CLX", MemoryChannelCount: 6, LogicalThreadCount: 2, CacheWayCount: 11},     // Cascadelake
	"CPX":     {MicroArchitecture: "CPX", MemoryChannelCount: 6, LogicalThreadCount: 2, CacheWayCount: 11},     // Cooperlake
	"ICX":     {MicroArchitecture: "ICX", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 12},     // Icelake
	"SPR":     {MicroArchitecture: "SPR", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15},     // Sapphire Rapids - generic
	"SPR_MCC": {MicroArchitecture: "SPR_MCC", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15}, // Sapphire Rapids - MCC
	"SPR_XCC": {MicroArchitecture: "SPR_XCC", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15}, // Sapphire Rapids - XCC
	"EMR":     {MicroArchitecture: "EMR", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15},     // Emerald Rapids - generic
	"EMR_MCC": {MicroArchitecture: "EMR_MCC", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 15}, // Emerald Rapids - MCC
	"EMR_XCC": {MicroArchitecture: "EMR_XCC", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 20}, // Emerald Rapids - XCC
	"SRF":     {MicroArchitecture: "SRF", MemoryChannelCount: 0, LogicalThreadCount: 1, CacheWayCount: 12},     // Sierra Forest
	"SRF_SP":  {MicroArchitecture: "SRF_SP", MemoryChannelCount: 8, LogicalThreadCount: 1, CacheWayCount: 12},  // Sierra Forest
	"SRF_AP":  {MicroArchitecture: "SRF_AP", MemoryChannelCount: 12, LogicalThreadCount: 1, CacheWayCount: 12}, // Sierra Forest
	"GNR":     {MicroArchitecture: "GNR", MemoryChannelCount: 0, LogicalThreadCount: 2, CacheWayCount: 16},     // Granite Rapids - generic
	"GNR_X1":  {MicroArchitecture: "GNR_X1", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 16},  // Granite Rapids - SP (MCC/LCC)
	"GNR_X2":  {MicroArchitecture: "GNR_X2", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 16},  // Granite Rapids - SP (XCC)
	"GNR_X3":  {MicroArchitecture: "GNR_X3", MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 16}, // Granite Rapids - AP (UCC)
	"GNR-D":   {MicroArchitecture: "GNR-D", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 16},   // Granite Rapids - D
	"CWF":     {MicroArchitecture: "CWF", MemoryChannelCount: 12, LogicalThreadCount: 1, CacheWayCount: 0},     // Clearwater Forest - generic
	"DMR":     {MicroArchitecture: "DMR", MemoryChannelCount: 16, LogicalThreadCount: 1, CacheWayCount: 0},     // Diamond Rapids
	// AMD CPUs
	"Naples":         {MicroArchitecture: "Naples", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 0},          // Naples
	"Rome":           {MicroArchitecture: "Rome", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 0},            // Rome
	"Milan":          {MicroArchitecture: "Milan", MemoryChannelCount: 8, LogicalThreadCount: 2, CacheWayCount: 0},           // Milan
	"Genoa":          {MicroArchitecture: "Genoa", MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 0},          // Genoa
	"Bergamo":        {MicroArchitecture: "Bergamo", MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 0},        // Bergamo
	"Turin (Zen 5)":  {MicroArchitecture: "Turin (Zen 5)", MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 0},  // Turin (Zen 5)
	"Turin (Zen 5c)": {MicroArchitecture: "Turin (Zen 5c)", MemoryChannelCount: 12, LogicalThreadCount: 2, CacheWayCount: 0}, // Turin (Zen 5c)
	// ARM CPUs
	"Graviton2":        {MicroArchitecture: "Graviton2", MemoryChannelCount: 8, LogicalThreadCount: 1},         // AWS Graviton 2 ([m|c|r]6g) Neoverse-N1
	"Graviton3":        {MicroArchitecture: "Graviton3", MemoryChannelCount: 8, LogicalThreadCount: 1},         // AWS Graviton 3 ([m|c|r]7g) Neoverse-V1
	"Graviton4":        {MicroArchitecture: "Graviton4", MemoryChannelCount: 12, LogicalThreadCount: 1},        // AWS Graviton 4 ([m|c|r]8g) Neoverse-V2
	"Axion":            {MicroArchitecture: "Axion", MemoryChannelCount: 12, LogicalThreadCount: 1},            // GCP Axion (c4a) Neoverse-V2
	"Altra Family":     {MicroArchitecture: "Altra Family", MemoryChannelCount: 8, LogicalThreadCount: 1},      // Ampere Altra
	"AmpereOne AC03":   {MicroArchitecture: "AmpereOne AC03", MemoryChannelCount: 8, LogicalThreadCount: 1},    // AmpereOne AC03
	"AmpereOne AC04":   {MicroArchitecture: "AmpereOne AC04", MemoryChannelCount: 8, LogicalThreadCount: 1},    // AmpereOne AC04
	"AmpereOne AC04_1": {MicroArchitecture: "AmpereOne AC04_1", MemoryChannelCount: 12, LogicalThreadCount: 1}, // AmpereOne AC04_1
}

// cpuIdentifiersX86 maps x86 CPU identification to microarchitecture names
var cpuIdentifiersX86 = []struct {
	Identifier        CPUIdentifierX86
	MicroArchitecture string
}{
	// Intel Core CPUs
	{CPUIdentifierX86{Family: "6", Model: "(50|69|70)", Stepping: "", Capid4: "", Devices: ""}, "HSW"},             // Haswell
	{CPUIdentifierX86{Family: "6", Model: "(61|71)", Stepping: "", Capid4: "", Devices: ""}, "BDW"},                // Broadwell
	{CPUIdentifierX86{Family: "6", Model: "(78|94)", Stepping: "", Capid4: "", Devices: ""}, "SKL"},                // Skylake
	{CPUIdentifierX86{Family: "6", Model: "(142|158)", Stepping: "9", Capid4: "", Devices: ""}, "KBL"},             // Kabylake
	{CPUIdentifierX86{Family: "6", Model: "(142|158)", Stepping: "(10|11|12|13)", Capid4: "", Devices: ""}, "CFL"}, // Coffeelake
	{CPUIdentifierX86{Family: "6", Model: "167", Stepping: "", Capid4: "", Devices: ""}, "RKL"},                    // Rocket Lake
	{CPUIdentifierX86{Family: "6", Model: "(140|141)", Stepping: "", Capid4: "", Devices: ""}, "TGL"},              // Tiger Lake
	{CPUIdentifierX86{Family: "6", Model: "(151|154)", Stepping: "", Capid4: "", Devices: ""}, "ADL"},              // Alder Lake
	{CPUIdentifierX86{Family: "6", Model: "170", Stepping: "4", Capid4: "", Devices: ""}, "MTL"},                   // Meteor Lake
	{CPUIdentifierX86{Family: "6", Model: "197", Stepping: "2", Capid4: "", Devices: ""}, "ARL"},                   // Arrow Lake
	// Intel Xeon CPUs
	{CPUIdentifierX86{Family: "6", Model: "63", Stepping: "", Capid4: "", Devices: ""}, "HSX"},            // Haswell
	{CPUIdentifierX86{Family: "6", Model: "(79|86)", Stepping: "", Capid4: "", Devices: ""}, "BDX"},       // Broadwell
	{CPUIdentifierX86{Family: "6", Model: "85", Stepping: "(0|1|2|3|4)", Capid4: "", Devices: ""}, "SKX"}, // Skylake
	{CPUIdentifierX86{Family: "6", Model: "85", Stepping: "(5|6|7)", Capid4: "", Devices: ""}, "CLX"},     // Cascadelake
	{CPUIdentifierX86{Family: "6", Model: "85", Stepping: "11", Capid4: "", Devices: ""}, "CPX"},          // Cooperlake
	{CPUIdentifierX86{Family: "6", Model: "(106|108)", Stepping: "", Capid4: "", Devices: ""}, "ICX"},     // Icelake
	{CPUIdentifierX86{Family: "6", Model: "143", Stepping: "", Capid4: "", Devices: ""}, "SPR"},           // Sapphire Rapids
	{CPUIdentifierX86{Family: "6", Model: "207", Stepping: "", Capid4: "", Devices: ""}, "EMR"},           // Emerald Rapids
	{CPUIdentifierX86{Family: "6", Model: "175", Stepping: "", Capid4: "", Devices: ""}, "SRF"},           // Sierra Forest
	{CPUIdentifierX86{Family: "6", Model: "173", Stepping: "", Capid4: "", Devices: ""}, "GNR"},           // Granite Rapids
	{CPUIdentifierX86{Family: "6", Model: "174", Stepping: "", Capid4: "", Devices: ""}, "GNR-D"},         // Granite Rapids - D
	{CPUIdentifierX86{Family: "6", Model: "221", Stepping: "", Capid4: "", Devices: ""}, "CWF"},           // Clearwater Forest
	{CPUIdentifierX86{Family: "19", Model: "1", Stepping: "", Capid4: "", Devices: ""}, "DMR"},            // Diamond Rapids
	// AMD CPUs
	{CPUIdentifierX86{Family: "23", Model: "1", Stepping: "", Capid4: "", Devices: ""}, "Naples"},                    // Naples
	{CPUIdentifierX86{Family: "23", Model: "49", Stepping: "", Capid4: "", Devices: ""}, "Rome"},                     // Rome
	{CPUIdentifierX86{Family: "25", Model: "1", Stepping: "", Capid4: "", Devices: ""}, "Milan"},                     // Milan
	{CPUIdentifierX86{Family: "25", Model: "(1[6-9]|2[0-9]|3[01])", Stepping: "", Capid4: "", Devices: ""}, "Genoa"}, // Genoa, model 16-31
	{CPUIdentifierX86{Family: "25", Model: "(16[0-9]|17[0-5])", Stepping: "", Capid4: "", Devices: ""}, "Bergamo"},   // Bergamo, model 160-175
	{CPUIdentifierX86{Family: "26", Model: "2", Stepping: "", Capid4: "", Devices: ""}, "Turin (Zen 5)"},             // Turin (Zen 5)
	{CPUIdentifierX86{Family: "26", Model: "17", Stepping: "", Capid4: "", Devices: ""}, "Turin (Zen 5c)"},           // Turin (Zen 5c)
}

// cpuIdentifiersARM maps ARM CPU identification to microarchitecture names
var cpuIdentifiersARM = []struct {
	Identifier        CPUIdentifierARM
	MicroArchitecture string
}{
	{CPUIdentifierARM{Implementer: "0x41", Part: "0xd0c", DmidecodePart: "AWS Graviton2"}, "Graviton2"}, // AWS Graviton 2 ([m|c|r]6g) Neoverse-N1
	{CPUIdentifierARM{Implementer: "0x41", Part: "0xd40", DmidecodePart: "AWS Graviton3"}, "Graviton3"}, // AWS Graviton 3 ([m|c|r]7g) Neoverse-V1
	{CPUIdentifierARM{Implementer: "0x41", Part: "0xd4f", DmidecodePart: "AWS Graviton4"}, "Graviton4"}, // AWS Graviton 4 ([m|c|r]8g) Neoverse-V2
	{CPUIdentifierARM{Implementer: "0x41", Part: "0xd4f", DmidecodePart: "Not Specified"}, "Axion"},     // GCP Axion (c4a) Neoverse-V2
	{CPUIdentifierARM{Implementer: "0x41", Part: "0xd0c", DmidecodePart: ""}, "Altra Family"},           // Ampere Altra
	{CPUIdentifierARM{Implementer: "0xc0", Part: "0xac3", DmidecodePart: ""}, "AmpereOne AC03"},         // AmpereOne AC03
	{CPUIdentifierARM{Implementer: "0xc0", Part: "0xac4", DmidecodePart: "X"}, "AmpereOne AC04"},        // AmpereOne AC04
	{CPUIdentifierARM{Implementer: "0xc0", Part: "0xac4", DmidecodePart: "M"}, "AmpereOne AC04_1"},      // AmpereOne AC04_1
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
		if id.Family != "" || id.Model != "" {
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
			uarch = "SPR_XCC"
		case 1:
			uarch = "SPR_MCC"
		}
	}
	if uarch == "" {
		uarch = "SPR"
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
			uarch = "EMR_XCC"
		case 1:
			uarch = "EMR_MCC"
		}
	}
	if uarch == "" {
		uarch = "EMR"
	}
	return
}

func getGNRMicroArchitecture(devices string) (uarch string, err error) {
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
	return
}

func getSRFMicroArchitecture(devices string) (uarch string, err error) {
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
	return
}
