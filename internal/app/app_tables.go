// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package app

// This file contains common table definitions used across multiple commands.

import (
	"strings"

	"perfspect/internal/extract"
	"perfspect/internal/script"
	"perfspect/internal/table"
)

// SystemSummaryTableName is the name of the system summary table.
const SystemSummaryTableName = "System Summary"

// TableDefinitions contains table definitions used across multiple commands.
var TableDefinitions = map[string]table.TableDefinition{
	SystemSummaryTableName: {
		Name:      SystemSummaryTableName,
		MenuLabel: SystemSummaryTableName,
		HasRows:   false,
		ScriptNames: []string{
			script.HostnameScriptName,
			script.DateScriptName,
			script.LscpuScriptName,
			script.LscpuCacheScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
			script.SpecCoreFrequenciesScriptName,
			script.MeminfoScriptName,
			script.NicInfoScriptName,
			script.DiskInfoScriptName,
			script.UnameScriptName,
			script.EtcReleaseScriptName,
			script.PackagePowerLimitName,
			script.EpbScriptName,
			script.ScalingDriverScriptName,
			script.ScalingGovernorScriptName,
			script.CstatesScriptName,
			script.ElcScriptName,
			script.ArmImplementerScriptName,
			script.ArmPartScriptName,
			script.ArmDmidecodePartScriptName,
			script.DmidecodeScriptName,
		},
		FieldsFunc: BriefSummaryTableValues},
}

// BriefSummaryTableValues returns the field values for the system summary table.
func BriefSummaryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	memory := extract.InstalledMemoryFromOutput(outputs)
	if memory == "" {
		memory = extract.ValFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^MemTotal:\s*(.+?)$`)
	}
	return []table.Field{
		{Name: "Host Name", Values: []string{strings.TrimSpace(outputs[script.HostnameScriptName].Stdout)}},
		{Name: "Time", Values: []string{strings.TrimSpace(outputs[script.DateScriptName].Stdout)}},
		{Name: "CPU Model", Values: []string{extract.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^[Mm]odel name:\s*(.+)$`)}},
		{Name: "Microarchitecture", Values: []string{extract.UarchFromOutput(outputs)}},
		{Name: "TDP", Values: []string{extract.TDPFromOutput(outputs)}},
		{Name: "Sockets", Values: []string{extract.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)}},
		{Name: "Cores per Socket", Values: []string{extract.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(.+)$`)}},
		{Name: "Hyperthreading", Values: []string{extract.HyperthreadingFromOutput(outputs)}},
		{Name: "CPUs", Values: []string{extract.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU\(s\):\s*(.+)$`)}},
		{Name: "NUMA Nodes", Values: []string{extract.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node\(s\):\s*(.+)$`)}},
		{Name: "Scaling Driver", Values: []string{strings.TrimSpace(outputs[script.ScalingDriverScriptName].Stdout)}},
		{Name: "Scaling Governor", Values: []string{strings.TrimSpace(outputs[script.ScalingGovernorScriptName].Stdout)}},
		{Name: "C-states", Values: []string{extract.CstatesSummaryFromOutput(outputs)}},
		{Name: "Maximum Frequency", Values: []string{extract.MaxFrequencyFromOutput(outputs)}, Description: "The highest speed a single core can reach with Turbo Boost."},
		{Name: "All-core Maximum Frequency", Values: []string{extract.AllCoreMaxFrequencyFromOutput(outputs)}, Description: "The highest speed all cores can reach simultaneously with Turbo Boost."},
		{Name: "Energy Performance Bias", Values: []string{extract.EPBFromOutput(outputs)}},
		{Name: "Efficiency Latency Control", Values: []string{extract.ELCSummaryFromOutput(outputs)}},
		{Name: "Memory", Values: []string{memory}},
		{Name: "NIC", Values: []string{extract.NICSummaryFromOutput(outputs)}},
		{Name: "Disk", Values: []string{extract.DiskSummaryFromOutput(outputs)}},
		{Name: "OS", Values: []string{extract.OperatingSystemFromOutput(outputs)}},
		{Name: "Kernel", Values: []string{extract.ValFromRegexSubmatch(outputs[script.UnameScriptName].Stdout, `^Linux \S+ (\S+)`)}},
	}
}
