// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"perfspect/internal/cpus"
	"perfspect/internal/script"
)

// MSR addresses for prefetcher control
const (
	MsrPrefetchControl = 0x1a4
	MsrPrefetchers     = 0x6d
	MsrAtomPrefTuning1 = 0x1320
)

// Prefetcher short names
const (
	PrefetcherL2HWName      = "L2 HW"
	PrefetcherL2AdjName     = "L2 Adj"
	PrefetcherDCUHWName     = "DCU HW"
	PrefetcherDCUIPName     = "DCU IP"
	PrefetcherDCUNPName     = "DCU NP"
	PrefetcherAMPName       = "AMP"
	PrefetcherLLCPPName     = "LLCPP"
	PrefetcherAOPName       = "AOP"
	PrefetcherHomelessName  = "Homeless"
	PrefetcherLLCName       = "LLC"
	PrefetcherLLCStreamName = "LLC Stream"
)

// PrefetcherDefinition represents a prefetcher configuration.
type PrefetcherDefinition struct {
	ShortName   string
	Description string
	Msr         int
	Bit         int
	Uarchs      []string
}

// PrefetcherDefinitions contains all known prefetcher definitions.
var PrefetcherDefinitions = []PrefetcherDefinition{
	{
		ShortName:   PrefetcherL2HWName,
		Description: "L2 Hardware (MLC Streamer) fetches additional lines of code or data into the L2 cache.",
		Msr:         MsrPrefetchControl,
		Bit:         0,
		Uarchs:      []string{"all"},
	},
	{
		ShortName:   PrefetcherL2AdjName,
		Description: "L2 Adjacent Cache Line (MLC Spatial) fetches the cache line that comprises a cache line pair.",
		Msr:         MsrPrefetchControl,
		Bit:         1,
		Uarchs:      []string{"all"},
	},
	{
		ShortName:   PrefetcherDCUHWName,
		Description: "DCU Hardware (DCU Streamer) fetches the next cache line into the L1 cache.",
		Msr:         MsrPrefetchControl,
		Bit:         2,
		Uarchs:      []string{"all"},
	},
	{
		ShortName:   PrefetcherDCUIPName,
		Description: "DCU Instruction Pointer prefetcher uses sequential load history to determine the cache lines to prefetch.",
		Msr:         MsrPrefetchControl,
		Bit:         3,
		Uarchs:      []string{"all"},
	},
	{
		ShortName:   PrefetcherDCUNPName,
		Description: "DCU Next Page is an L1 data cache prefetcher.",
		Msr:         MsrPrefetchControl,
		Bit:         4,
		Uarchs:      []string{"all"},
	},
	{
		ShortName:   PrefetcherAMPName,
		Description: "Adaptive Multipath Probability (MLC AMP) predicts access patterns based on previous patterns and fetches the corresponding cache lines into the L2 cache.",
		Msr:         MsrPrefetchControl,
		Bit:         5,
		Uarchs:      []string{cpus.UarchSPR, cpus.UarchEMR, cpus.UarchGNR},
	},
	{
		ShortName:   PrefetcherLLCPPName,
		Description: "Last Level Cache Page (MLC LLC Page) Prefetcher",
		Msr:         MsrPrefetchControl,
		Bit:         6,
		Uarchs:      []string{cpus.UarchGNR},
	},
	{
		ShortName:   PrefetcherAOPName,
		Description: "L2 Array of Pointers (DCU AOP) Prefetcher",
		Msr:         MsrPrefetchControl,
		Bit:         7,
		Uarchs:      []string{cpus.UarchGNR},
	},
	{
		ShortName:   PrefetcherHomelessName,
		Description: "Homeless prefetch allows early fetch of the demand miss into the MLC when we don't have enough resources to track this demand in the L1 cache.",
		Msr:         MsrPrefetchers,
		Bit:         14,
		Uarchs:      []string{cpus.UarchSPR, cpus.UarchEMR, cpus.UarchGNR},
	},
	{
		ShortName:   PrefetcherLLCName,
		Description: "Last level cache gives the core prefetcher the ability to prefetch data directly into the LLC without necessarily filling into the L1 and L2 caches first.",
		Msr:         MsrPrefetchers,
		Bit:         42,
		Uarchs:      []string{cpus.UarchSPR, cpus.UarchEMR, cpus.UarchGNR},
	},
	{
		ShortName:   PrefetcherLLCStreamName,
		Description: "Last level cache stream prefetcher.",
		Msr:         MsrAtomPrefTuning1,
		Bit:         43,
		Uarchs:      []string{cpus.UarchSRF, cpus.UarchCWF},
	},
}

// GetPrefetcherDefByName returns the Prefetcher definition by its short name.
func GetPrefetcherDefByName(name string) (PrefetcherDefinition, error) {
	for _, p := range PrefetcherDefinitions {
		if p.ShortName == name {
			return p, nil
		}
	}
	return PrefetcherDefinition{}, fmt.Errorf("prefetcher %s not found", name)
}

// GetPrefetcherDefinitions returns all Prefetcher definitions.
func GetPrefetcherDefinitions() []PrefetcherDefinition {
	return PrefetcherDefinitions
}

// IsPrefetcherEnabled checks if a prefetcher is enabled based on MSR value and bit position.
func IsPrefetcherEnabled(msrValue string, bit int) (bool, error) {
	if msrValue == "" {
		return false, fmt.Errorf("msrValue is empty")
	}
	msrInt, err := strconv.ParseInt(msrValue, 16, 64)
	if err != nil {
		return false, fmt.Errorf("failed to parse msrValue: %s, %v", msrValue, err)
	}
	bitMask := int64(1) << bit
	// enabled if bit is zero
	return bitMask&msrInt == 0, nil
}

// PrefetchersFromOutput extracts prefetcher status from script outputs.
func PrefetchersFromOutput(outputs map[string]script.ScriptOutput) [][]string {
	out := make([][]string, 0)
	uarch := UarchFromOutput(outputs)
	if uarch == "" {
		return [][]string{}
	}
	for _, pf := range PrefetcherDefinitions {
		if slices.Contains(pf.Uarchs, "all") || slices.Contains(pf.Uarchs, uarch[:3]) {
			var scriptName string
			switch pf.Msr {
			case MsrPrefetchControl:
				scriptName = script.PrefetchControlName
			case MsrPrefetchers:
				scriptName = script.PrefetchersName
			case MsrAtomPrefTuning1:
				scriptName = script.PrefetchersAtomName
			default:
				slog.Error("unknown msr for prefetcher", slog.String("msr", fmt.Sprintf("0x%x", pf.Msr)))
				continue
			}
			msrVal := ValFromRegexSubmatch(outputs[scriptName].Stdout, `^([0-9a-fA-F]+)`)
			if msrVal == "" {
				continue
			}
			var enabledDisabled string
			enabled, err := IsPrefetcherEnabled(msrVal, pf.Bit)
			if err != nil {
				slog.Warn("error checking prefetcher enabled status", slog.String("error", err.Error()))
				continue
			}
			if enabled {
				enabledDisabled = "Enabled"
			} else {
				enabledDisabled = "Disabled"
			}
			out = append(out, []string{pf.ShortName, pf.Description, fmt.Sprintf("0x%04X", pf.Msr), strconv.Itoa(pf.Bit), enabledDisabled})
		}
	}
	return out
}

// PrefetchersSummaryFromOutput returns a summary of all prefetcher statuses.
func PrefetchersSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	uarch := UarchFromOutput(outputs)
	if uarch == "" {
		return ""
	}
	var prefList []string
	for _, pf := range PrefetcherDefinitions {
		if slices.Contains(pf.Uarchs, "all") || slices.Contains(pf.Uarchs, uarch[:3]) {
			var scriptName string
			switch pf.Msr {
			case MsrPrefetchControl:
				scriptName = script.PrefetchControlName
			case MsrPrefetchers:
				scriptName = script.PrefetchersName
			case MsrAtomPrefTuning1:
				scriptName = script.PrefetchersAtomName
			default:
				slog.Error("unknown msr for prefetcher", slog.String("msr", fmt.Sprintf("0x%x", pf.Msr)))
				continue
			}
			msrVal := ValFromRegexSubmatch(outputs[scriptName].Stdout, `^([0-9a-fA-F]+)`)
			if msrVal == "" {
				continue
			}
			var enabledDisabled string
			enabled, err := IsPrefetcherEnabled(msrVal, pf.Bit)
			if err != nil {
				slog.Warn("error checking prefetcher enabled status", slog.String("error", err.Error()))
				continue
			}
			if enabled {
				enabledDisabled = "Enabled"
			} else {
				enabledDisabled = "Disabled"
			}
			prefList = append(prefList, fmt.Sprintf("%s: %s", pf.ShortName, enabledDisabled))
		}
	}
	if len(prefList) > 0 {
		return strings.Join(prefList, ", ")
	}
	return "None"
}
