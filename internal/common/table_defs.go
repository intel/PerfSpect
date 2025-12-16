package common

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"perfspect/internal/script"
	"perfspect/internal/table"
	"strings"
)

const BriefSysSummaryTableName = "Brief System Summary"

var TableDefinitions = map[string]table.TableDefinition{
	BriefSysSummaryTableName: {
		Name:      BriefSysSummaryTableName,
		MenuLabel: BriefSysSummaryTableName,
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
		},
		FieldsFunc: briefSummaryTableValues},
}

func briefSummaryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return []table.Field{
		{Name: "Host Name", Values: []string{strings.TrimSpace(outputs[script.HostnameScriptName].Stdout)}},                                                                                   // Hostname
		{Name: "Time", Values: []string{strings.TrimSpace(outputs[script.DateScriptName].Stdout)}},                                                                                            // Date
		{Name: "CPU Model", Values: []string{ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^[Mm]odel name:\s*(.+)$`)}},                                                        // Lscpu
		{Name: "Microarchitecture", Values: []string{UarchFromOutput(outputs)}},                                                                                                               // Lscpu, LspciBits, LspciDevices
		{Name: "TDP", Values: []string{TDPFromOutput(outputs)}},                                                                                                                               // PackagePowerLimit
		{Name: "Sockets", Values: []string{ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)}},                                                            // Lscpu
		{Name: "Cores per Socket", Values: []string{ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(.+)$`)}},                                          // Lscpu
		{Name: "Hyperthreading", Values: []string{HyperthreadingFromOutput(outputs)}},                                                                                                         // Lscpu, LspciBits, LspciDevices
		{Name: "CPUs", Values: []string{ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU\(s\):\s*(.+)$`)}},                                                                  // Lscpu
		{Name: "NUMA Nodes", Values: []string{ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node\(s\):\s*(.+)$`)}},                                                      // Lscpu
		{Name: "Scaling Driver", Values: []string{strings.TrimSpace(outputs[script.ScalingDriverScriptName].Stdout)}},                                                                         // ScalingDriver
		{Name: "Scaling Governor", Values: []string{strings.TrimSpace(outputs[script.ScalingGovernorScriptName].Stdout)}},                                                                     // ScalingGovernor
		{Name: "C-states", Values: []string{CstatesSummaryFromOutput(outputs)}},                                                                                                               // Cstates
		{Name: "Maximum Frequency", Values: []string{MaxFrequencyFromOutput(outputs)}, Description: "The highest speed a single core can reach with Turbo Boost."},                            // MaximumFrequency, SpecCoreFrequencies,
		{Name: "All-core Maximum Frequency", Values: []string{AllCoreMaxFrequencyFromOutput(outputs)}, Description: "The highest speed all cores can reach simultaneously with Turbo Boost."}, // Lscpu, LspciBits, LspciDevices, SpecCoreFrequencies
		{Name: "Energy Performance Bias", Values: []string{EPBFromOutput(outputs)}},                                                                                                           // EpbSource, EpbBIOS, EpbOS
		{Name: "Efficiency Latency Control", Values: []string{ELCSummaryFromOutput(outputs)}},                                                                                                 // Elc
		{Name: "MemTotal", Values: []string{ValFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^MemTotal:\s*(.+?)$`)}},                                                           // Meminfo
		{Name: "NIC", Values: []string{NICSummaryFromOutput(outputs)}},                                                                                                                        // Lshw, NicInfo
		{Name: "Disk", Values: []string{DiskSummaryFromOutput(outputs)}},                                                                                                                      // DiskInfo, Hdparm
		{Name: "OS", Values: []string{OperatingSystemFromOutput(outputs)}},                                                                                                                    // EtcRelease
		{Name: "Kernel", Values: []string{ValFromRegexSubmatch(outputs[script.UnameScriptName].Stdout, `^Linux \S+ (\S+)`)}},                                                                  // Uname
	}
}
