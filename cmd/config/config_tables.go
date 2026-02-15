// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package config

import (
	"fmt"
	"log/slog"
	"perfspect/internal/extract"

	"perfspect/internal/cpus"
	"perfspect/internal/script"
	"perfspect/internal/table"
	"slices"
	"strings"
)

const (
	ConfigurationTableName = "Configuration"
)

var tableDefinitions = map[string]table.TableDefinition{
	ConfigurationTableName: {
		Name:    ConfigurationTableName,
		Vendors: []string{cpus.IntelVendor},
		HasRows: false,
		ScriptNames: []string{
			script.LscpuScriptName,
			script.LscpuCacheScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
			script.L3CacheWayEnabledName,
			script.PackagePowerLimitName,
			script.EpbScriptName,
			script.EppScriptName,
			script.EppValidScriptName,
			script.EppPackageControlScriptName,
			script.EppPackageScriptName,
			script.ScalingGovernorScriptName,
			script.UncoreMaxFromMSRScriptName,
			script.UncoreMinFromMSRScriptName,
			script.UncoreMaxFromTPMIScriptName,
			script.UncoreMinFromTPMIScriptName,
			script.UncoreDieTypesFromTPMIScriptName,
			script.SpecCoreFrequenciesScriptName,
			script.ElcScriptName,
			script.PrefetchControlName,
			script.PrefetchersName,
			script.PrefetchersAtomName,
			script.CstatesScriptName,
			script.C1DemotionScriptName,
			script.ArmImplementerScriptName,
			script.ArmPartScriptName,
			script.ArmDmidecodePartScriptName,
		},
		FieldsFunc: configurationTableValues},
}

func configurationTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	uarch := extract.UarchFromOutput(outputs)
	if uarch == "" {
		slog.Error("failed to get uarch from script outputs")
		return []table.Field{}
	}
	// This table is only shown in text mode on stdout for the config command. The config
	// command implements its own print logic and uses the Description field to show the command line
	// argument for each config item.
	fields := []table.Field{
		{Name: "Cores per Socket", Description: "--cores <N>", Values: []string{extract.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(.+)$`)}},
		{Name: "L3 Cache", Description: "--llc <MB>", Values: []string{l3InstanceFromOutput(outputs)}},
		{Name: "Package Power / TDP", Description: "--tdp <Watts>", Values: []string{extract.TDPFromOutput(outputs)}},
		{Name: "Core SSE Frequency", Description: "--core-max <GHz>", Values: []string{sseFrequenciesFromOutput(outputs)}},
	}
	if strings.Contains(uarch, cpus.UarchSRF) || strings.Contains(uarch, cpus.UarchGNR) || strings.Contains(uarch, cpus.UarchCWF) || strings.Contains(uarch, cpus.UarchDMR) {
		fields = append(fields, []table.Field{
			{Name: "Uncore Max Frequency (Compute)", Description: "--uncore-max-compute <GHz>", Values: []string{extract.UncoreMinMaxDieFrequencyFromOutput(true, true, outputs)}},
			{Name: "Uncore Min Frequency (Compute)", Description: "--uncore-min-compute <GHz>", Values: []string{extract.UncoreMinMaxDieFrequencyFromOutput(false, true, outputs)}},
			{Name: "Uncore Max Frequency (I/O)", Description: "--uncore-max-io <GHz>", Values: []string{extract.UncoreMinMaxDieFrequencyFromOutput(true, false, outputs)}},
			{Name: "Uncore Min Frequency (I/O)", Description: "--uncore-min-io <GHz>", Values: []string{extract.UncoreMinMaxDieFrequencyFromOutput(false, false, outputs)}},
		}...)
	} else {
		fields = append(fields, []table.Field{
			{Name: "Uncore Max Frequency", Description: "--uncore-max <GHz>", Values: []string{extract.UncoreMaxFrequencyFromOutput(outputs)}},
			{Name: "Uncore Min Frequency", Description: "--uncore-min <GHz>", Values: []string{extract.UncoreMinFrequencyFromOutput(outputs)}},
		}...)
	}
	fields = append(fields, []table.Field{
		{Name: "Energy Performance Bias", Description: "--epb <0-15>", Values: []string{extract.EPBFromOutput(outputs)}},
		{Name: "Energy Performance Preference", Description: "--epp <0-255>", Values: []string{extract.EPPFromOutput(outputs)}},
		{Name: "Scaling Governor", Description: "--gov <" + strings.Join(governorOptions, "|") + ">", Values: []string{strings.TrimSpace(outputs[script.ScalingGovernorScriptName].Stdout)}},
	}...)
	// add ELC (for SRF, CWF, GNR, and DMR only)
	if strings.Contains(uarch, cpus.UarchSRF) || strings.Contains(uarch, cpus.UarchGNR) || strings.Contains(uarch, cpus.UarchCWF) || strings.Contains(uarch, cpus.UarchDMR) {
		fields = append(fields, table.Field{Name: "Efficiency Latency Control", Description: "--elc <" + strings.Join(elcOptions, "|") + ">", Values: []string{extract.ELCSummaryFromOutput(outputs)}})
	}
	// add prefetchers
	for _, pf := range extract.PrefetcherDefinitions {
		if slices.Contains(pf.Uarchs, "all") || slices.Contains(pf.Uarchs, uarch[:3]) {
			var scriptName string
			switch pf.Msr {
			case extract.MsrPrefetchControl:
				scriptName = script.PrefetchControlName
			case extract.MsrPrefetchers:
				scriptName = script.PrefetchersName
			case extract.MsrAtomPrefTuning1:
				scriptName = script.PrefetchersAtomName
			default:
				slog.Error("unknown msr for prefetcher", slog.String("msr", fmt.Sprintf("0x%x", pf.Msr)))
				continue
			}
			msrVal := extract.ValFromRegexSubmatch(outputs[scriptName].Stdout, `^([0-9a-fA-F]+)`)
			var enabledDisabled string
			enabled, err := extract.IsPrefetcherEnabled(msrVal, pf.Bit)
			if err != nil {
				slog.Warn("error checking prefetcher enabled status", slog.String("error", err.Error()))
				continue
			}
			if enabled {
				enabledDisabled = "Enabled"
			} else {
				enabledDisabled = "Disabled"
			}
			fields = append(fields,
				table.Field{
					Name:        pf.ShortName + " prefetcher",
					Description: "--" + "pref-" + strings.ReplaceAll(strings.ToLower(pf.ShortName), " ", "") + " <" + strings.Join(prefetcherOptions, "|") + ">",
					Values:      []string{enabledDisabled}},
			)
		}
	}
	// add C6
	c6 := extract.C6FromOutput(outputs)
	if c6 != "" {
		fields = append(fields, table.Field{Name: "C6", Description: "--c6 <" + strings.Join(c6Options, "|") + ">", Values: []string{c6}})
	}
	// add C1 Demotion
	c1Demotion := strings.TrimSpace(outputs[script.C1DemotionScriptName].Stdout)
	if c1Demotion != "" {
		fields = append(fields, table.Field{Name: "C1 Demotion", Description: "--c1-demotion <" + strings.Join(c1DemotionOptions, "|") + ">", Values: []string{c1Demotion}})
	}
	return fields
}

// l3InstanceFromOutput retrieves the L3 cache size per instance (per socket on Intel) in megabytes
func l3InstanceFromOutput(outputs map[string]script.ScriptOutput) string {
	l3InstanceMB, _, err := extract.GetL3MSRMB(outputs)
	if err != nil {
		slog.Debug("Could not get L3 size from MSR, falling back to lscpu", slog.String("error", err.Error()))
		l3InstanceMB, _, err = extract.GetL3LscpuMB(outputs)
		if err != nil {
			slog.Warn("Could not get L3 size from lscpu", slog.String("error", err.Error()))
			return ""
		}
	}
	return extract.FormatCacheSizeMB(l3InstanceMB)
}

// sseFrequenciesFromOutput gets the bucketed SSE frequencies from the output
// and returns a compact string representation with consolidated ranges, e.g.:
// "1-40/3.5, 41-60/3.4, 61-86/3.2"
func sseFrequenciesFromOutput(outputs map[string]script.ScriptOutput) string {
	specCoreFrequencies, err := extract.GetSpecFrequencyBuckets(outputs)
	if err != nil {
		return ""
	}
	sseFreqs := extract.GetSSEFreqsFromBuckets(specCoreFrequencies)
	if len(sseFreqs) < 1 {
		return ""
	}

	var result []string
	i := 1
	for i < len(specCoreFrequencies) {
		startIdx := i
		currentFreq := sseFreqs[i-1]

		// Find consecutive buckets with the same frequency
		for i < len(specCoreFrequencies) && sseFreqs[i-1] == currentFreq {
			i++
		}
		endIdx := i - 1

		// Extract start and end core numbers from the ranges
		startRange := strings.Split(specCoreFrequencies[startIdx][0], "-")[0]
		endRange := strings.Split(specCoreFrequencies[endIdx][0], "-")[1]

		// Format the consolidated range
		if startRange == endRange {
			result = append(result, fmt.Sprintf("%s/%s", startRange, currentFreq))
		} else {
			result = append(result, fmt.Sprintf("%s-%s/%s", startRange, endRange, currentFreq))
		}
	}

	return strings.Join(result, ", ")
}

// configurationTableTextRenderer renders the configuration table for text reports.
// It's similar to the default text table renderer, but uses the Description field
// to show the command line argument for each config item.
// Example output:
// Configuration
// =============
// Cores per Socket:               86          --cores <N>
// L3 Cache:                       336M        --llc <MB>
// Package Power / TDP:            350W        --tdp <Watts>
// All-Core Max Frequency:         3.2GHz      --core-max <GHz>
func configurationTableTextRenderer(tableValues table.TableValues) string {
	var sb strings.Builder

	// Find the longest field name and value for formatting
	maxFieldNameLen := 0
	maxValueLen := 0
	for _, field := range tableValues.Fields {
		if len(field.Name) > maxFieldNameLen {
			maxFieldNameLen = len(field.Name)
		}
		if len(field.Values) > 0 && len(field.Values[0]) > maxValueLen {
			maxValueLen = len(field.Values[0])
		}
	}

	// Print each field with name, value, and description (command-line arg)
	for _, field := range tableValues.Fields {
		var value string
		if len(field.Values) > 0 {
			value = field.Values[0]
		}
		// Format: "Field Name:      Value       Description"
		fmt.Fprintf(&sb, "%-*s  %-*s  %s\n",
			maxFieldNameLen+1, field.Name+":",
			maxValueLen, value,
			field.Description)
	}

	return sb.String()
}
