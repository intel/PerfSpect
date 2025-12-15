package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// table_defs.go defines the tables used for generating reports

import (
	"fmt"
	htmltemplate "html/template"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"

	"perfspect/internal/common"
	"perfspect/internal/cpus"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/table"
)

const (
	// report table names
	HostTableName              = "Host"
	SystemTableName            = "System"
	BaseboardTableName         = "Baseboard"
	ChassisTableName           = "Chassis"
	BIOSTableName              = "BIOS"
	OperatingSystemTableName   = "Operating System"
	SoftwareVersionTableName   = "Software Version"
	CPUTableName               = "CPU"
	PrefetcherTableName        = "Prefetcher"
	ISATableName               = "ISA"
	AcceleratorTableName       = "Accelerator"
	PowerTableName             = "Power"
	CstateTableName            = "C-state"
	MaximumFrequencyTableName  = "Maximum Frequency"
	SSTTFHPTableName           = "Speed Select Turbo Frequency - High Priority"
	SSTTFLPTableName           = "Speed Select Turbo Frequency - Low Priority"
	UncoreTableName            = "Uncore"
	ElcTableName               = "Efficiency Latency Control"
	MemoryTableName            = "Memory"
	DIMMTableName              = "DIMM"
	NetworkConfigTableName     = "Network Configuration"
	NICTableName               = "NIC"
	NICCpuAffinityTableName    = "NIC CPU Affinity"
	NICPacketSteeringTableName = "NIC Packet Steering"
	DiskTableName              = "Disk"
	FilesystemTableName        = "Filesystem"
	GPUTableName               = "GPU"
	GaudiTableName             = "Gaudi"
	CXLTableName               = "CXL"
	PCIeTableName              = "PCIe"
	CVETableName               = "CVE"
	ProcessTableName           = "Process"
	SensorTableName            = "Sensor"
	ChassisStatusTableName     = "Chassis Status"
	PMUTableName               = "PMU"
	SystemEventLogTableName    = "System Event Log"
	KernelLogTableName         = "Kernel Log"
	SystemSummaryTableName     = "System Summary"
)

// menu labels

const (
	// menu labels
	HostMenuLabel          = "Host"
	SoftwareMenuLabel      = "Software"
	CPUMenuLabel           = "CPU"
	PowerMenuLabel         = "Power"
	MemoryMenuLabel        = "Memory"
	NetworkMenuLabel       = "Network"
	StorageMenuLabel       = "Storage"
	DevicesMenuLabel       = "Devices"
	SecurityMenuLabel      = "Security"
	StatusMenuLabel        = "Status"
	LogsMenuLabel          = "Logs"
	SystemSummaryMenuLabel = "System Summary"
)

var tableDefinitions = map[string]table.TableDefinition{
	//
	// configuration tables
	//
	HostTableName: {
		Name:      HostTableName,
		HasRows:   false,
		MenuLabel: HostMenuLabel,
		ScriptNames: []string{
			script.HostnameScriptName,
			script.DateScriptName,
			script.DmidecodeScriptName},
		FieldsFunc: hostTableValues},
	BIOSTableName: {
		Name:      BIOSTableName,
		HasRows:   false,
		MenuLabel: SoftwareMenuLabel,
		ScriptNames: []string{
			script.DmidecodeScriptName,
		},
		FieldsFunc: biosTableValues},
	OperatingSystemTableName: {
		Name:    OperatingSystemTableName,
		HasRows: false,
		ScriptNames: []string{
			script.EtcReleaseScriptName,
			script.UnameScriptName,
			script.ProcCmdlineScriptName,
			script.ProcCpuinfoScriptName},
		FieldsFunc: operatingSystemTableValues},
	SoftwareVersionTableName: {
		Name:    SoftwareVersionTableName,
		HasRows: false,
		ScriptNames: []string{
			script.GccVersionScriptName,
			script.GlibcVersionScriptName,
			script.BinutilsVersionScriptName,
			script.PythonVersionScriptName,
			script.Python3VersionScriptName,
			script.JavaVersionScriptName,
			script.OpensslVersionScriptName},
		FieldsFunc: softwareVersionTableValues},
	CPUTableName: {
		Name:      CPUTableName,
		HasRows:   false,
		MenuLabel: CPUMenuLabel,
		ScriptNames: []string{
			script.LscpuScriptName,
			script.LscpuCacheScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
			script.CpuidScriptName,
			script.BaseFrequencyScriptName,
			script.MaximumFrequencyScriptName,
			script.SpecCoreFrequenciesScriptName,
			script.PPINName,
			script.L3CacheWayEnabledName,
			script.ArmImplementerScriptName,
			script.ArmPartScriptName,
			script.ArmDmidecodePartScriptName},
		FieldsFunc:   cpuTableValues,
		InsightsFunc: cpuTableInsights},
	PrefetcherTableName: {
		Name:    PrefetcherTableName,
		HasRows: true,
		Vendors: []string{cpus.IntelVendor},
		ScriptNames: []string{
			script.LscpuScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
			script.PrefetchControlName,
			script.PrefetchersName,
			script.PrefetchersAtomName,
		},
		FieldsFunc: prefetcherTableValues},
	ISATableName: {
		Name:          ISATableName,
		Architectures: []string{cpus.X86Architecture},
		ScriptNames:   []string{script.CpuidScriptName},
		FieldsFunc:    isaTableValues},
	AcceleratorTableName: {
		Name:               AcceleratorTableName,
		Vendors:            []string{cpus.IntelVendor},
		MicroArchitectures: []string{cpus.UarchSPR, cpus.UarchEMR, cpus.UarchGNR, cpus.UarchSRF, cpus.UarchCWF, cpus.UarchDMR},
		HasRows:            true,
		ScriptNames: []string{
			script.LshwScriptName,
			script.IaaDevicesScriptName,
			script.DsaDevicesScriptName},
		FieldsFunc:   acceleratorTableValues,
		InsightsFunc: acceleratorTableInsights},
	PowerTableName: {
		Name:      PowerTableName,
		Vendors:   []string{cpus.IntelVendor},
		HasRows:   false,
		MenuLabel: PowerMenuLabel,
		ScriptNames: []string{
			script.PackagePowerLimitName,
			script.EpbScriptName,
			script.EppScriptName,
			script.EppValidScriptName,
			script.EppPackageControlScriptName,
			script.EppPackageScriptName,
			script.ScalingDriverScriptName,
			script.ScalingGovernorScriptName},
		FieldsFunc:   powerTableValues,
		InsightsFunc: powerTableInsights},
	CstateTableName: {
		Name:    CstateTableName,
		HasRows: true,
		ScriptNames: []string{
			script.CstatesScriptName,
		},
		FieldsFunc: cstateTableValues},
	MaximumFrequencyTableName: {
		Name:    MaximumFrequencyTableName,
		Vendors: []string{cpus.IntelVendor},
		HasRows: true,
		ScriptNames: []string{
			script.SpecCoreFrequenciesScriptName,
			script.LscpuScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
		},
		FieldsFunc: maximumFrequencyTableValues},
	UncoreTableName: {
		Name:    UncoreTableName,
		Vendors: []string{cpus.IntelVendor},
		HasRows: false,
		ScriptNames: []string{
			script.UncoreMaxFromMSRScriptName,
			script.UncoreMinFromMSRScriptName,
			script.UncoreMaxFromTPMIScriptName,
			script.UncoreMinFromTPMIScriptName,
			script.UncoreDieTypesFromTPMIScriptName,
			script.ChaCountScriptName,
			script.LscpuScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName},
		FieldsFunc: uncoreTableValues},
	ElcTableName: {
		Name:               ElcTableName,
		MicroArchitectures: []string{cpus.UarchGNR, cpus.UarchSRF, cpus.UarchCWF, cpus.UarchDMR},
		HasRows:            true,
		ScriptNames: []string{
			script.ElcScriptName,
		},
		FieldsFunc:   elcTableValues,
		InsightsFunc: elcTableInsights},
	SSTTFHPTableName: {
		Name:               SSTTFHPTableName,
		MicroArchitectures: []string{cpus.UarchGNR, cpus.UarchSRF, cpus.UarchCWF, cpus.UarchDMR},
		HasRows:            true,
		ScriptNames: []string{
			script.SSTTFHPScriptName,
		},
		FieldsFunc: sstTFHPTableValues},
	SSTTFLPTableName: {
		Name:               SSTTFLPTableName,
		MicroArchitectures: []string{cpus.UarchGNR, cpus.UarchSRF, cpus.UarchCWF, cpus.UarchDMR},
		HasRows:            true,
		ScriptNames: []string{
			script.SSTTFLPScriptName,
		},
		FieldsFunc: sstTFLPTableValues},
	MemoryTableName: {
		Name:      MemoryTableName,
		HasRows:   false,
		MenuLabel: MemoryMenuLabel,
		ScriptNames: []string{
			script.DmidecodeScriptName,
			script.MeminfoScriptName,
			script.TransparentHugePagesScriptName,
			script.NumaBalancingScriptName,
			script.LscpuScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
			script.TmeScriptName,
			script.ArmImplementerScriptName,
			script.ArmPartScriptName,
			script.ArmDmidecodePartScriptName,
		},
		FieldsFunc:   memoryTableValues,
		InsightsFunc: memoryTableInsights},
	DIMMTableName: {
		Name:    DIMMTableName,
		HasRows: true,
		ScriptNames: []string{
			script.DmidecodeScriptName,
			script.LscpuScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
			script.ArmImplementerScriptName,
			script.ArmPartScriptName,
			script.ArmDmidecodePartScriptName,
		},
		FieldsFunc:   dimmTableValues,
		InsightsFunc: dimmTableInsights},
	NetworkConfigTableName: {
		Name:      NetworkConfigTableName,
		HasRows:   false,
		MenuLabel: NetworkMenuLabel,
		ScriptNames: []string{
			script.SysctlScriptName,
			script.IRQBalanceScriptName,
		},
		FieldsFunc: networkConfigTableValues},
	NICTableName: {
		Name:    NICTableName,
		HasRows: true,
		ScriptNames: []string{
			script.NicInfoScriptName,
		},
		FieldsFunc: nicTableValues},
	NICCpuAffinityTableName: {
		Name:    NICCpuAffinityTableName,
		HasRows: true,
		ScriptNames: []string{
			script.NicInfoScriptName,
		},
		FieldsFunc: nicCpuAffinityTableValues},
	NICPacketSteeringTableName: {
		Name:    NICPacketSteeringTableName,
		HasRows: true,
		ScriptNames: []string{
			script.NicInfoScriptName,
		},
		FieldsFunc: nicPacketSteeringTableValues},
	DiskTableName: {
		Name:      DiskTableName,
		HasRows:   true,
		MenuLabel: StorageMenuLabel,
		ScriptNames: []string{
			script.DiskInfoScriptName,
			script.HdparmScriptName,
		},
		FieldsFunc: diskTableValues},
	FilesystemTableName: {
		Name:    FilesystemTableName,
		HasRows: true,
		ScriptNames: []string{
			script.DfScriptName,
			script.FindMntScriptName,
		},
		FieldsFunc:   filesystemTableValues,
		InsightsFunc: filesystemTableInsights},
	GPUTableName: {
		Name:      GPUTableName,
		HasRows:   true,
		MenuLabel: DevicesMenuLabel,
		ScriptNames: []string{
			script.LshwScriptName,
		},
		FieldsFunc: gpuTableValues},
	GaudiTableName: {
		Name:          GaudiTableName,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.GaudiInfoScriptName,
			script.GaudiFirmwareScriptName,
			script.GaudiNumaScriptName,
			script.GaudiArchitectureScriptName,
		},
		FieldsFunc: gaudiTableValues},
	CXLTableName: {
		Name:    CXLTableName,
		HasRows: true,
		ScriptNames: []string{
			script.LspciVmmScriptName,
		},
		FieldsFunc: cxlTableValues},
	PCIeTableName: {
		Name:    PCIeTableName,
		HasRows: true,
		ScriptNames: []string{
			script.DmidecodeScriptName,
		},
		FieldsFunc: pcieSlotsTableValues},
	CVETableName: {
		Name:      CVETableName,
		MenuLabel: SecurityMenuLabel,
		ScriptNames: []string{
			script.CveScriptName,
		},
		FieldsFunc:   cveTableValues,
		InsightsFunc: cveTableInsights},
	ProcessTableName: {
		Name:      ProcessTableName,
		HasRows:   true,
		MenuLabel: StatusMenuLabel,
		ScriptNames: []string{
			script.ProcessListScriptName,
		},
		FieldsFunc: processTableValues},
	SensorTableName: {
		Name:    SensorTableName,
		HasRows: true,
		ScriptNames: []string{
			script.IpmitoolSensorsScriptName,
		},
		FieldsFunc: sensorTableValues},
	ChassisStatusTableName: {
		Name:    ChassisStatusTableName,
		HasRows: false,
		ScriptNames: []string{
			script.IpmitoolChassisScriptName,
		},
		FieldsFunc: chassisStatusTableValues},
	PMUTableName: {
		Name:    PMUTableName,
		Vendors: []string{cpus.IntelVendor},
		HasRows: false,
		ScriptNames: []string{
			script.PMUBusyScriptName,
			script.PMUDriverVersionScriptName,
		},
		FieldsFunc: pmuTableValues},
	SystemEventLogTableName: {
		Name:      SystemEventLogTableName,
		HasRows:   true,
		MenuLabel: LogsMenuLabel,
		ScriptNames: []string{
			script.IpmitoolEventsScriptName,
		},
		FieldsFunc:   systemEventLogTableValues,
		InsightsFunc: systemEventLogTableInsights},
	KernelLogTableName: {
		Name:    KernelLogTableName,
		HasRows: true,
		ScriptNames: []string{
			script.KernelLogScriptName,
		},
		FieldsFunc: kernelLogTableValues},
	SystemSummaryTableName: {
		Name:      SystemSummaryTableName,
		HasRows:   false,
		MenuLabel: SystemSummaryMenuLabel,
		ScriptNames: []string{
			script.HostnameScriptName,
			script.DateScriptName,
			script.DmidecodeScriptName,
			script.LscpuScriptName,
			script.LscpuCacheScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
			script.L3CacheWayEnabledName,
			script.CpuidScriptName,
			script.BaseFrequencyScriptName,
			script.SpecCoreFrequenciesScriptName,
			script.PrefetchControlName,
			script.PrefetchersName,
			script.PrefetchersAtomName,
			script.PPINName,
			script.MeminfoScriptName,
			script.TransparentHugePagesScriptName,
			script.NumaBalancingScriptName,
			script.NicInfoScriptName,
			script.DiskInfoScriptName,
			script.ProcCpuinfoScriptName,
			script.UnameScriptName,
			script.EtcReleaseScriptName,
			script.PackagePowerLimitName,
			script.EpbScriptName,
			script.ScalingDriverScriptName,
			script.ScalingGovernorScriptName,
			script.CstatesScriptName,
			script.ElcScriptName,
			script.CveScriptName,
			script.ArmImplementerScriptName,
			script.ArmPartScriptName,
			script.ArmDmidecodePartScriptName,
		},
		FieldsFunc: systemSummaryTableValues},
}

//
// define the fieldsFunc for each table
//

// Tables without rows have a fixed set of fields with a single value each.
// If no data is found for a field, the value is an empty string.
//
// Tables with rows have a variable number of fields and values
// depending on the system configuration. If no data is found for a table with rows,
// the FieldsFunc should return an empty slice of fields.

func hostTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	hostName := strings.TrimSpace(outputs[script.HostnameScriptName].Stdout)
	time := strings.TrimSpace(outputs[script.DateScriptName].Stdout)
	system := common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Manufacturer:\s*(.+?)$`) +
		" " + common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Product Name:\s*(.+?)$`) +
		", " + common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Version:\s*(.+?)$`)
	baseboard := common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Manufacturer:\s*(.+?)$`) +
		" " + common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Product Name:\s*(.+?)$`) +
		", " + common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Version:\s*(.+?)$`)
	chassis := common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Manufacturer:\s*(.+?)$`) +
		" " + common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Type:\s*(.+?)$`) +
		", " + common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Version:\s*(.+?)$`)
	return []table.Field{
		{Name: "Host Name", Values: []string{hostName}},
		{Name: "Time", Values: []string{time}},
		{Name: "System", Values: []string{system}},
		{Name: "Baseboard", Values: []string{baseboard}},
		{Name: "Chassis", Values: []string{chassis}},
	}
}

func pcieSlotsTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fieldValues := common.ValsArrayFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "9",
		[]string{
			`^Designation:\s*(.+?)$`,
			`^Type:\s*(.+?)$`,
			`^Length:\s*(.+?)$`,
			`^Bus Address:\s*(.+?)$`,
			`^Current Usage:\s*(.+?)$`,
		}...,
	)
	if len(fieldValues) == 0 {
		return []table.Field{}
	}
	fields := []table.Field{
		{Name: "Designation"},
		{Name: "Type"},
		{Name: "Length"},
		{Name: "Bus Address"},
		{Name: "Current Usage"},
	}
	for i := range fields {
		for j := range fieldValues {
			fields[i].Values = append(fields[i].Values, fieldValues[j][i])
		}
	}
	return fields
}

func biosTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Vendor"},
		{Name: "Version"},
		{Name: "Release Date"},
	}
	fieldValues := common.ValsArrayFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "0",
		[]string{
			`^Vendor:\s*(.+?)$`,
			`^Version:\s*(.+?)$`,
			`^Release Date:\s*(.+?)$`,
		}...,
	)
	for i := range fields {
		if len(fieldValues) > 0 {
			for j := range fieldValues {
				fields[i].Values = append(fields[i].Values, fieldValues[j][i])
			}
		} else {
			fields[i].Values = append(fields[i].Values, "")
		}
	}
	return fields
}

func operatingSystemTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return []table.Field{
		{Name: "OS", Values: []string{common.OperatingSystemFromOutput(outputs)}},
		{Name: "Kernel", Values: []string{common.ValFromRegexSubmatch(outputs[script.UnameScriptName].Stdout, `^Linux \S+ (\S+)`)}},
		{Name: "Boot Parameters", Values: []string{strings.TrimSpace(outputs[script.ProcCmdlineScriptName].Stdout)}},
		{Name: "Microcode", Values: []string{common.ValFromRegexSubmatch(outputs[script.ProcCpuinfoScriptName].Stdout, `^microcode.*:\s*(.+?)$`)}},
	}
}

func softwareVersionTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return []table.Field{
		{Name: "GCC", Values: []string{common.ValFromRegexSubmatch(outputs[script.GccVersionScriptName].Stdout, `^(gcc .*)$`)}},
		{Name: "GLIBC", Values: []string{common.ValFromRegexSubmatch(outputs[script.GlibcVersionScriptName].Stdout, `^(ldd .*)`)}},
		{Name: "Binutils", Values: []string{common.ValFromRegexSubmatch(outputs[script.BinutilsVersionScriptName].Stdout, `^(GNU ld .*)$`)}},
		{Name: "Python", Values: []string{common.ValFromRegexSubmatch(outputs[script.PythonVersionScriptName].Stdout, `^(Python .*)$`)}},
		{Name: "Python3", Values: []string{common.ValFromRegexSubmatch(outputs[script.Python3VersionScriptName].Stdout, `^(Python 3.*)$`)}},
		{Name: "Java", Values: []string{common.ValFromRegexSubmatch(outputs[script.JavaVersionScriptName].Stdout, `^(openjdk .*)$`)}},
		{Name: "OpenSSL", Values: []string{common.ValFromRegexSubmatch(outputs[script.OpensslVersionScriptName].Stdout, `^(OpenSSL .*)$`)}},
	}
}

func cpuTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	var l1d, l1i, l2 string
	lscpuCache, err := common.ParseLscpuCacheOutput(outputs[script.LscpuCacheScriptName].Stdout)
	if err != nil {
		slog.Warn("failed to parse lscpu cache output", "error", err)
	} else {
		if _, ok := lscpuCache["L1d"]; ok {
			l1d = common.L1l2CacheSizeFromLscpuCache(lscpuCache["L1d"])
		}
		if _, ok := lscpuCache["L1i"]; ok {
			l1i = common.L1l2CacheSizeFromLscpuCache(lscpuCache["L1i"])
		}
		if _, ok := lscpuCache["L2"]; ok {
			l2 = common.L1l2CacheSizeFromLscpuCache(lscpuCache["L2"])
		}
	}
	return []table.Field{
		{Name: "CPU Model", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^[Mm]odel name:\s*(.+)$`)}},
		{Name: "Architecture", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Architecture:\s*(.+)$`)}},
		{Name: "Microarchitecture", Values: []string{common.UarchFromOutput(outputs)}},
		{Name: "Family", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU family:\s*(.+)$`)}},
		{Name: "Model", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model:\s*(.+)$`)}},
		{Name: "Stepping", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Stepping:\s*(.+)$`)}},
		{Name: "Base Frequency", Values: []string{common.BaseFrequencyFromOutput(outputs)}, Description: "The minimum guaranteed speed of a single core under standard conditions."},
		{Name: "Maximum Frequency", Values: []string{common.MaxFrequencyFromOutput(outputs)}, Description: "The highest speed a single core can reach with Turbo Boost."},
		{Name: "All-core Maximum Frequency", Values: []string{common.AllCoreMaxFrequencyFromOutput(outputs)}, Description: "The highest speed all cores can reach simultaneously with Turbo Boost."},
		{Name: "CPUs", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU\(s\):\s*(.+)$`)}},
		{Name: "On-line CPU List", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^On-line CPU\(s\) list:\s*(.+)$`)}},
		{Name: "Hyperthreading", Values: []string{common.HyperthreadingFromOutput(outputs)}},
		{Name: "Cores per Socket", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(.+)$`)}},
		{Name: "Sockets", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)}},
		{Name: "NUMA Nodes", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node\(s\):\s*(.+)$`)}},
		{Name: "NUMA CPU List", Values: []string{numaCPUListFromOutput(outputs)}},
		{Name: "L1d Cache", Values: []string{l1d}, Description: "The size of the L1 data cache for one core."},
		{Name: "L1i Cache", Values: []string{l1i}, Description: "The size of the L1 instruction cache for one core."},
		{Name: "L2 Cache", Values: []string{l2}, Description: "The size of the L2 cache for one core."},
		{Name: "L3 Cache (instance/total)", Values: []string{common.L3FromOutput(outputs)}, Description: "The size of one L3 cache instance and the total L3 cache size for the system."},
		{Name: "L3 per Core", Values: []string{common.L3PerCoreFromOutput(outputs)}, Description: "The L3 cache size per core."},
		{Name: "Memory Channels", Values: []string{channelsFromOutput(outputs)}},
		{Name: "Intel Turbo Boost", Values: []string{turboEnabledFromOutput(outputs)}},
		{Name: "Virtualization", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Virtualization:\s*(.+)$`)}},
		{Name: "PPINs", Values: []string{ppinsFromOutput(outputs)}},
	}
}

func prefetcherTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	prefetchers := common.PrefetchersFromOutput(outputs)
	if len(prefetchers) == 0 {
		return []table.Field{}
	}
	fields := []table.Field{
		{Name: "Prefetcher"},
		{Name: "Description"},
		{Name: "MSR"},
		{Name: "Bit"},
		{Name: "Status"},
	}
	for _, pref := range prefetchers {
		for i := range pref {
			if i < len(fields) {
				fields[i].Values = append(fields[i].Values, pref[i])
			}
		}
	}
	return fields
}

func cpuTableInsights(outputs map[string]script.ScriptOutput, tableValues table.TableValues) []table.Insight {
	insights := []table.Insight{}
	addInsightFunc := func(fieldName, bestValue string) {
		fieldIndex, err := table.GetFieldIndex(fieldName, tableValues)
		if err != nil {
			slog.Warn(err.Error())
		} else {
			fieldValue := tableValues.Fields[fieldIndex].Values[0]
			if fieldValue != "" && fieldValue != "N/A" && fieldValue != bestValue {
				insights = append(insights, table.Insight{
					Recommendation: fmt.Sprintf("Consider enabling %s.", fieldName),
					Justification:  fmt.Sprintf("%s is not enabled.", fieldName),
				})
			}
		}
	}
	addInsightFunc("Hyperthreading", "Enabled")
	addInsightFunc("Intel Turbo Boost", "Enabled")
	// Xeon Generation
	familyIndex, err := table.GetFieldIndex("Family", tableValues)
	if err != nil {
		slog.Warn(err.Error())
	} else {
		family := tableValues.Fields[familyIndex].Values[0]
		if cpus.IsIntelCPUFamilyStr(family) { // Intel
			uarchIndex, err := table.GetFieldIndex("Microarchitecture", tableValues)
			if err != nil {
				slog.Warn(err.Error())
			} else {
				xeonGens := map[string]int{
					cpus.UarchHSX: 1,
					cpus.UarchBDX: 2,
					cpus.UarchSKX: 3,
					cpus.UarchCLX: 4,
					cpus.UarchICX: 5,
					cpus.UarchSPR: 6,
					cpus.UarchEMR: 7,
					cpus.UarchSRF: 8,
					cpus.UarchCWF: 8,
					cpus.UarchGNR: 8,
					cpus.UarchDMR: 9,
				}
				uarch := tableValues.Fields[uarchIndex].Values[0]
				if len(uarch) >= 3 {
					xeonGen, ok := xeonGens[uarch[:3]]
					if ok {
						if xeonGen < xeonGens[cpus.UarchSPR] {
							insights = append(insights, table.Insight{
								Recommendation: "Consider upgrading to the latest generation Intel(r) Xeon(r) CPU.",
								Justification:  "The CPU is 2 or more generations behind the latest Intel(r) Xeon(r) CPU.",
							})
						}
					}
				}
			}
		} else {
			insights = append(insights, table.Insight{
				Recommendation: "Consider upgrading to an Intel(r) Xeon(r) CPU.",
				Justification:  "The current CPU is not an Intel(r) Xeon(r) CPU.",
			})
		}
	}
	return insights
}

func isaTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{}
	supported := isaSupportedFromOutput(outputs)
	for i, isa := range isaFullNames() {
		fields = append(fields, table.Field{
			Name:   isa,
			Values: []string{supported[i]},
		})
	}
	return fields
}

func acceleratorTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	names := acceleratorNames()
	if len(names) == 0 {
		return []table.Field{}
	}
	return []table.Field{
		{Name: "Name", Values: names},
		{Name: "Count", Values: acceleratorCountsFromOutput(outputs)},
		{Name: "Work Queues", Values: acceleratorWorkQueuesFromOutput(outputs)},
		{Name: "Full Name", Values: acceleratorFullNamesFromYaml()},
		{Name: "Description", Values: acceleratorDescriptionsFromYaml()},
	}
}

func acceleratorTableInsights(outputs map[string]script.ScriptOutput, tableValues table.TableValues) []table.Insight {
	insights := []table.Insight{}
	nameFieldIndex, err := table.GetFieldIndex("Name", tableValues)
	if err != nil {
		slog.Warn(err.Error())
		return insights
	}
	countFieldIndex, err := table.GetFieldIndex("Count", tableValues)
	if err != nil {
		slog.Warn(err.Error())
		return insights
	}
	queuesFieldIndex, err := table.GetFieldIndex("Work Queues", tableValues)
	if err != nil {
		slog.Warn(err.Error())
		return insights
	}
	for i, count := range tableValues.Fields[countFieldIndex].Values {
		name := tableValues.Fields[nameFieldIndex].Values[i]
		queues := tableValues.Fields[queuesFieldIndex].Values[i]
		if name == "DSA" && count != "0" && queues == "None" {
			insights = append(insights, table.Insight{
				Recommendation: "Consider configuring DSA to allow accelerated data copy and transformation in DSA-enabled software.",
				Justification:  "No work queues are configured for DSA accelerator(s).",
			})
		}
		if name == "IAA" && count != "0" && queues == "None" {
			insights = append(insights, table.Insight{
				Recommendation: "Consider configuring IAA to allow accelerated compression and decompression in IAA-enabled software.",
				Justification:  "No work queues are configured for IAA accelerator(s).",
			})
		}
	}
	return insights
}

func powerTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return []table.Field{
		{Name: "TDP", Values: []string{common.TDPFromOutput(outputs)}},
		{Name: "Energy Performance Bias", Values: []string{common.EPBFromOutput(outputs)}},
		{Name: "Energy Performance Preference", Values: []string{common.EPPFromOutput(outputs)}},
		{Name: "Scaling Governor", Values: []string{strings.TrimSpace(outputs[script.ScalingGovernorScriptName].Stdout)}},
		{Name: "Scaling Driver", Values: []string{strings.TrimSpace(outputs[script.ScalingDriverScriptName].Stdout)}},
	}
}

func powerTableInsights(outputs map[string]script.ScriptOutput, tableValues table.TableValues) []table.Insight {
	insights := []table.Insight{}
	addInsightFunc := func(fieldName, bestValue string) {
		fieldIndex, err := table.GetFieldIndex(fieldName, tableValues)
		if err != nil {
			slog.Warn(err.Error())
		} else {
			fieldValue := tableValues.Fields[fieldIndex].Values[0]
			if fieldValue != "" && fieldValue != bestValue {
				insights = append(insights, table.Insight{
					Recommendation: fmt.Sprintf("Consider setting %s to '%s'.", fieldName, bestValue),
					Justification:  fmt.Sprintf("%s is set to '%s'", fieldName, fieldValue),
				})
			}
		}
	}
	addInsightFunc("Scaling Governor", "performance")
	addInsightFunc("Scaling Driver", "intel_pstate")
	addInsightFunc("Energy Performance Bias", "Performance (0)")
	addInsightFunc("Energy Performance Preference", "Performance (0)")
	return insights
}

func cstateTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	cstates := common.CstatesFromOutput(outputs)
	if len(cstates) == 0 {
		return []table.Field{}
	}
	fields := []table.Field{
		{Name: "Name"},
		{Name: "Status"}, // enabled/disabled
	}
	for _, cstateInfo := range cstates {
		fields[0].Values = append(fields[0].Values, cstateInfo.Name)
		fields[1].Values = append(fields[1].Values, cstateInfo.Status)
	}
	return fields
}

func uncoreTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	uarch := common.UarchFromOutput(outputs)
	if uarch == "" {
		slog.Error("failed to get uarch from script outputs")
		return []table.Field{}
	}
	if strings.Contains(uarch, cpus.UarchSRF) || strings.Contains(uarch, cpus.UarchGNR) || strings.Contains(uarch, cpus.UarchCWF) {
		return []table.Field{
			{Name: "Min Frequency (Compute)", Values: []string{common.UncoreMinMaxDieFrequencyFromOutput(false, true, outputs)}},
			{Name: "Min Frequency (I/O)", Values: []string{common.UncoreMinMaxDieFrequencyFromOutput(false, false, outputs)}},
			{Name: "Max Frequency (Compute)", Values: []string{common.UncoreMinMaxDieFrequencyFromOutput(true, true, outputs)}},
			{Name: "Max Frequency (I/O)", Values: []string{common.UncoreMinMaxDieFrequencyFromOutput(true, false, outputs)}},
			{Name: "CHA Count", Values: []string{chaCountFromOutput(outputs)}},
		}
	} else { // field counts need to match for the all_hosts reports to work properly
		return []table.Field{
			{Name: "Min Frequency", Values: []string{common.UncoreMinFrequencyFromOutput(outputs)}},
			{Name: "N/A", Values: []string{""}},
			{Name: "Max Frequency", Values: []string{common.UncoreMaxFrequencyFromOutput(outputs)}},
			{Name: "N/A", Values: []string{""}},
			{Name: "CHA Count", Values: []string{chaCountFromOutput(outputs)}},
		}
	}
}

func elcTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return common.ELCFieldValuesFromOutput(outputs)
}

func elcTableInsights(outputs map[string]script.ScriptOutput, tableValues table.TableValues) []table.Insight {
	insights := []table.Insight{}
	modeFieldIndex, err := table.GetFieldIndex("Mode", tableValues)
	if err != nil {
		slog.Warn(err.Error())
	} else {
		// warn if ELC mode is not set to 'Latency Optimized' or 'Default' consistently across all dies
		firstMode := tableValues.Fields[modeFieldIndex].Values[0]
		for _, mode := range tableValues.Fields[modeFieldIndex].Values[1:] {
			if mode != firstMode {
				insights = append(insights, table.Insight{
					Recommendation: "Consider setting Efficiency Latency Control mode consistently across all dies.",
					Justification:  "ELC mode is not set consistently across all dies.",
				})
				break
			}
		}
		// suggest setting ELC mode to 'Latency Optimized' or 'Default' based on the current setting
		for _, mode := range tableValues.Fields[modeFieldIndex].Values {
			if mode != "" && mode != "Latency Optimized" {
				insights = append(insights, table.Insight{
					Recommendation: "Consider setting Efficiency Latency Control mode to 'Latency Optimized' when workload is highly sensitive to memory latency.",
					Justification:  fmt.Sprintf("ELC mode is set to '%s' on at least one die.", mode),
				})
				break
			}
		}
		for _, mode := range tableValues.Fields[modeFieldIndex].Values {
			if mode != "" && mode != "Default" {
				insights = append(insights, table.Insight{
					Recommendation: "Consider setting Efficiency Latency Control mode to 'Default' to balance uncore performance and power utilization.",
					Justification:  fmt.Sprintf("ELC mode is set to '%s' on at least one die.", mode),
				})
				break
			}
		}
		// if epb is not set to 'Performance (0)' and ELC mode is set to 'Latency Optimized', suggest setting epb to 'Performance (0)'
		epb := common.EPBFromOutput(outputs)
		if epb != "" && epb != "Performance (0)" && firstMode == "Latency Optimized" {
			insights = append(insights, table.Insight{
				Recommendation: "Consider setting Energy Performance Bias to 'Performance (0)' to allow Latency Optimized mode to operate as designed.",
				Justification:  fmt.Sprintf("Energy Performance Bias is set to '%s' and ELC Mode is set to '%s'.", epb, firstMode),
			})
		}
	}
	return insights
}

func maximumFrequencyTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	frequencyBuckets, err := common.GetSpecFrequencyBuckets(outputs)
	if err != nil {
		slog.Warn("unable to get spec core frequencies", slog.String("error", err.Error()))
		return []table.Field{}
	}
	var fields []table.Field
	for i, row := range frequencyBuckets {
		// first row is field names
		if i == 0 {
			for _, fieldName := range row {
				fields = append(fields, table.Field{Name: fieldName})
			}
			continue
		}
		// following rows are field values
		for i, fieldValue := range row {
			fields[i].Values = append(fields[i].Values, fieldValue)
		}
	}
	return fields
}

func sstTFHPTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	output := outputs[script.SSTTFHPScriptName].Stdout
	if len(output) == 0 {
		return []table.Field{}
	}
	lines := strings.Split(output, "\n")
	if len(lines) >= 1 && (strings.Contains(lines[0], "not supported") || strings.Contains(lines[0], "not enabled")) {
		return []table.Field{}
	}
	// lines should contain CSV formatted data
	fields := []table.Field{}
	for i, line := range lines {
		// field names are in the header
		if i == 0 {
			fieldNames := strings.Split(line, ",")
			for j, fieldName := range fieldNames {
				if j > 1 {
					fieldName = fieldName + " (MHz)"
				}
				fields = append(fields, table.Field{Name: fieldName})
			}
			continue
		}
		// skip empty lines
		if line == "" {
			continue
		}
		values := strings.Split(line, ",")
		if len(values) != len(fields) {
			slog.Warn("unexpected number of values in line", slog.String("line", line))
			continue
		}
		for j, value := range values {
			// confirm value is a number
			if _, err := strconv.Atoi(value); err != nil {
				slog.Warn("unexpected non-numeric value in line", slog.String("line", line), slog.String("value", value))
				return []table.Field{}
			}
			if j > 1 {
				value = value + "00"
			}
			fields[j].Values = append(fields[j].Values, value)
		}
	}
	return fields
}

func sstTFLPTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	output := outputs[script.SSTTFLPScriptName].Stdout
	if len(output) == 0 {
		return []table.Field{}
	}
	lines := strings.Split(output, "\n")
	if len(lines) >= 1 && (strings.Contains(lines[0], "not supported") || strings.Contains(lines[0], "not enabled")) {
		return []table.Field{}
	}
	// lines should contain CSV formatted data
	fields := []table.Field{}
	for i, line := range lines {
		// field names are in the header
		if i == 0 {
			for fieldName := range strings.SplitSeq(line, ",") {
				fields = append(fields, table.Field{Name: fieldName + " (MHz)"})
			}
			continue
		}
		// skip empty lines
		if line == "" {
			continue
		}
		values := strings.Split(line, ",")
		if len(values) != len(fields) {
			slog.Warn("unexpected number of values in line", slog.String("line", line))
			continue
		}
		for j, value := range values {
			// confirm value is a number
			if _, err := strconv.Atoi(value); err != nil {
				slog.Warn("unexpected non-numeric value in line", slog.String("line", line), slog.String("value", value))
				return []table.Field{}
			}
			fields[j].Values = append(fields[j].Values, value+"00")
		}
	}
	return fields
}

func memoryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return []table.Field{
		{Name: "Installed Memory", Values: []string{installedMemoryFromOutput(outputs)}},
		{Name: "MemTotal", Values: []string{common.ValFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^MemTotal:\s*(.+?)$`)}},
		{Name: "MemFree", Values: []string{common.ValFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^MemFree:\s*(.+?)$`)}},
		{Name: "MemAvailable", Values: []string{common.ValFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^MemAvailable:\s*(.+?)$`)}},
		{Name: "Buffers", Values: []string{common.ValFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^Buffers:\s*(.+?)$`)}},
		{Name: "Cached", Values: []string{common.ValFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^Cached:\s*(.+?)$`)}},
		{Name: "HugePages_Total", Values: []string{common.ValFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^HugePages_Total:\s*(.+?)$`)}},
		{Name: "Hugepagesize", Values: []string{common.ValFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^Hugepagesize:\s*(.+?)$`)}},
		{Name: "Transparent Huge Pages", Values: []string{common.ValFromRegexSubmatch(outputs[script.TransparentHugePagesScriptName].Stdout, `.*\[(.*)\].*`)}},
		{Name: "Automatic NUMA Balancing", Values: []string{numaBalancingFromOutput(outputs)}},
		{Name: "Populated Memory Channels", Values: []string{populatedChannelsFromOutput(outputs)}},
		{Name: "Total Memory Encryption (TME)", Values: []string{strings.TrimSpace(outputs[script.TmeScriptName].Stdout)}},
		{Name: "Clustering Mode", Values: []string{clusteringModeFromOutput(outputs)}},
	}
}

func memoryTableInsights(outputs map[string]script.ScriptOutput, tableValues table.TableValues) []table.Insight {
	insights := []table.Insight{}
	// check if memory is not fully populated
	populatedChannelsIndex, err := table.GetFieldIndex("Populated Memory Channels", tableValues)
	if err != nil {
		slog.Warn(err.Error())
	} else {
		populatedChannels := tableValues.Fields[populatedChannelsIndex].Values[0]
		if populatedChannels != "" {
			uarch := common.UarchFromOutput(outputs)
			if uarch != "" {
				cpu, err := cpus.GetCPUByMicroArchitecture(uarch)
				if err != nil {
					slog.Warn(err.Error())
				} else {
					sockets := common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)
					socketCount, err := strconv.Atoi(sockets)
					if err != nil {
						slog.Warn(err.Error())
					} else {
						totalMemoryChannels := socketCount * cpu.MemoryChannelCount
						if populatedChannels != strconv.Itoa(totalMemoryChannels) {
							insights = append(insights, table.Insight{
								Recommendation: fmt.Sprintf("Consider populating all (%d) memory channels.", totalMemoryChannels),
								Justification:  fmt.Sprintf("%s memory channels are populated.", populatedChannels),
							})
						}
					}
				}
			}
		}
	}
	// check if NUMA balancing is not enabled (when there are multiple NUMA nodes)
	nodes := common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node\(s\):\s*(.+)$`)
	nodeCount, err := strconv.Atoi(nodes)
	if err != nil {
		slog.Warn(err.Error())
	} else {
		if nodeCount > 1 {
			numaBalancingIndex, err := table.GetFieldIndex("Automatic NUMA Balancing", tableValues)
			if err != nil {
				slog.Warn(err.Error())
			} else {
				numaBalancing := tableValues.Fields[numaBalancingIndex].Values[0]
				if numaBalancing != "" && numaBalancing != "Enabled" {
					insights = append(insights, table.Insight{
						Recommendation: "Consider enabling Automatic NUMA Balancing.",
						Justification:  "Automatic NUMA Balancing is not enabled.",
					})
				}
			}
		}
	}

	return insights
}

func dimmTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	dimmFieldValues := common.ValsArrayFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "17",
		[]string{
			`^Bank Locator:\s*(.+?)$`,
			`^Locator:\s*(.+?)$`,
			`^Manufacturer:\s*(.+?)$`,
			`^Part Number:\s*(.+?)\s*$`,
			`^Serial Number:\s*(.+?)\s*$`,
			`^Size:\s*(.+?)$`,
			`^Type:\s*(.+?)$`,
			`^Type Detail:\s*(.+?)$`,
			`^Speed:\s*(.+?)$`,
			`^Rank:\s*(.+?)$`,
			`^Configured.*Speed:\s*(.+?)$`,
		}...,
	)
	if len(dimmFieldValues) == 0 {
		return []table.Field{}
	}
	fields := []table.Field{
		{Name: "Bank Locator"},
		{Name: "Locator"},
		{Name: "Manufacturer"},
		{Name: "Part"},
		{Name: "Serial"},
		{Name: "Size"},
		{Name: "Type"},
		{Name: "Detail"},
		{Name: "Speed"},
		{Name: "Rank"},
		{Name: "Configured Speed"},
		{Name: "Socket"},
		{Name: "Channel"},
		{Name: "Slot"},
	}
	for dimmIndex := range dimmFieldValues {
		for fieldIndex := 0; fieldIndex <= 10; fieldIndex++ {
			fields[fieldIndex].Values = append(fields[fieldIndex].Values, dimmFieldValues[dimmIndex][fieldIndex])
		}
	}
	derivedDimmFieldValues := derivedDimmsFieldFromOutput(outputs)
	if len(dimmFieldValues) != len(derivedDimmFieldValues) {
		slog.Warn("unable to derive socket, channel, and slot for all DIMMs")
		// fill with empty strings
		fields[11].Values = append(fields[11].Values, make([]string, len(dimmFieldValues))...)
		fields[12].Values = append(fields[12].Values, make([]string, len(dimmFieldValues))...)
		fields[13].Values = append(fields[13].Values, make([]string, len(dimmFieldValues))...)
	} else {
		for i := range derivedDimmFieldValues {
			fields[11].Values = append(fields[11].Values, derivedDimmFieldValues[i].socket)
			fields[12].Values = append(fields[12].Values, derivedDimmFieldValues[i].channel)
			fields[13].Values = append(fields[13].Values, derivedDimmFieldValues[i].slot)
		}
	}
	return fields
}

func dimmTableInsights(outputs map[string]script.ScriptOutput, tableValues table.TableValues) []table.Insight {
	insights := []table.Insight{}
	// check if are configured for their maximum speed
	SpeedIndex, err := table.GetFieldIndex("Speed", tableValues)
	if err != nil {
		slog.Warn(err.Error())
	} else {
		ConfiguredSpeedIndex, err := table.GetFieldIndex("Configured Speed", tableValues)
		if err != nil {
			slog.Warn(err.Error())
		} else {
			for i, speed := range tableValues.Fields[SpeedIndex].Values {
				configuredSpeed := tableValues.Fields[ConfiguredSpeedIndex].Values[i]
				if speed != "" && configuredSpeed != "" && speed != "Unknown" && configuredSpeed != "Unknown" {
					speedParts := strings.Split(speed, " ")
					configuredSpeedParts := strings.Split(configuredSpeed, " ")
					if len(speedParts) > 0 && len(configuredSpeedParts) > 0 {
						speedVal, err := strconv.Atoi(speedParts[0])
						if err != nil {
							slog.Warn(err.Error())
						} else {
							configuredSpeedVal, err := strconv.Atoi(configuredSpeedParts[0])
							if err != nil {
								slog.Warn(err.Error())
							} else {
								if speedVal < configuredSpeedVal {
									insights = append(insights, table.Insight{
										Recommendation: "Consider configuring DIMMs for their maximum speed.",
										Justification:  fmt.Sprintf("DIMMs configured for %s when their maximum speed is %s.", configuredSpeed, speed),
									})
								}
							}
						}
					}
				}
			}
		}
	}
	return insights
}

func nicTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	allNicsInfo := common.ParseNicInfo(outputs[script.NicInfoScriptName].Stdout)
	if len(allNicsInfo) == 0 {
		return []table.Field{}
	}
	fields := []table.Field{
		{Name: "Name"},
		{Name: "Vendor (ID)"},
		{Name: "Model (ID)"},
		{Name: "MAC Address"},
		{Name: "Speed"},
		{Name: "Link"},
		{Name: "Bus"},
		{Name: "Card / Port"},
		{Name: "NUMA Node"},
		{Name: "Driver"},
		{Name: "Driver Version"},
		{Name: "Firmware Version"},
		{Name: "MTU", Description: "Maximum Transmission Unit. The largest size packet or frame, specified in octets (eight-bit bytes), that can be sent in a packet- or frame-based network such as the Internet."},
		{Name: "TX Queues"},
		{Name: "RX Queues"},
		{Name: "Adaptive RX", Description: "Enables dynamic adjustment of receive interrupt coalescing based on traffic patterns."},
		{Name: "Adaptive TX", Description: "Enables dynamic adjustment of transmit interrupt coalescing based on traffic patterns."},
		{Name: "rx-usecs", Description: "Sets the delay, in microseconds, before an interrupt is generated after receiving a packet. Higher values reduce CPU usage (by batching packets), but increase latency. Lower values reduce latency, but increase interrupt rate and CPU load."},
		{Name: "tx-usecs", Description: "Sets the delay, in microseconds, before an interrupt is generated after transmitting a packet. Higher values reduce CPU usage (by batching packets), but increase latency. Lower values reduce latency, but increase interrupt rate and CPU load."},
	}
	for _, nicInfo := range allNicsInfo {
		// Annotate interface name with (virtual) if it's a virtual function
		nicName := nicInfo.Name
		if nicInfo.IsVirtual {
			nicName += " (virtual)"
		}
		fields[0].Values = append(fields[0].Values, nicName)
		fields[1].Values = append(fields[1].Values, nicInfo.Vendor)
		if nicInfo.VendorID != "" {
			fields[1].Values[len(fields[1].Values)-1] += fmt.Sprintf(" (%s)", nicInfo.VendorID)
		}
		fields[2].Values = append(fields[2].Values, nicInfo.Model)
		if nicInfo.ModelID != "" {
			fields[2].Values[len(fields[2].Values)-1] += fmt.Sprintf(" (%s)", nicInfo.ModelID)
		}
		fields[3].Values = append(fields[3].Values, nicInfo.MACAddress)
		fields[4].Values = append(fields[4].Values, nicInfo.Speed)
		fields[5].Values = append(fields[5].Values, nicInfo.Link)
		fields[6].Values = append(fields[6].Values, nicInfo.Bus)
		// Add Card / Port column
		cardPort := ""
		if nicInfo.Card != "" && nicInfo.Port != "" {
			cardPort = nicInfo.Card + " / " + nicInfo.Port
		}
		fields[7].Values = append(fields[7].Values, cardPort)
		fields[8].Values = append(fields[8].Values, nicInfo.NUMANode)
		fields[9].Values = append(fields[9].Values, nicInfo.Driver)
		fields[10].Values = append(fields[10].Values, nicInfo.DriverVersion)
		fields[11].Values = append(fields[11].Values, nicInfo.FirmwareVersion)
		fields[12].Values = append(fields[12].Values, nicInfo.MTU)
		fields[13].Values = append(fields[13].Values, nicInfo.TXQueues)
		fields[14].Values = append(fields[14].Values, nicInfo.RXQueues)
		fields[15].Values = append(fields[15].Values, nicInfo.AdaptiveRX)
		fields[16].Values = append(fields[16].Values, nicInfo.AdaptiveTX)
		fields[17].Values = append(fields[17].Values, nicInfo.RxUsecs)
		fields[18].Values = append(fields[18].Values, nicInfo.TxUsecs)
	}
	return fields
}

func nicPacketSteeringTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	allNicsInfo := common.ParseNicInfo(outputs[script.NicInfoScriptName].Stdout)
	if len(allNicsInfo) == 0 {
		return []table.Field{}
	}

	fields := []table.Field{
		{Name: "Interface"},
		{Name: "Type", Description: "XPS (Transmit Packet Steering) and RPS (Receive Packet Steering) are software-based mechanisms that allow the selection of a specific logical CPU core to handle the transmission or processing of network packets for a given queue."},
		{Name: "Queue:CPU(s) | Queue|CPU(s) | ..."},
	}

	for _, nicInfo := range allNicsInfo {
		// XPS row
		if nicInfo.TXQueues != "0" {
			fields[0].Values = append(fields[0].Values, nicInfo.Name)
			fields[1].Values = append(fields[1].Values, "XPS")
			fields[2].Values = append(fields[2].Values, formatQueueCPUMappings(nicInfo.XPSCPUs, "tx-"))
		}

		// RPS row
		if nicInfo.RXQueues != "0" {
			fields[0].Values = append(fields[0].Values, nicInfo.Name)
			fields[1].Values = append(fields[1].Values, "RPS")
			fields[2].Values = append(fields[2].Values, formatQueueCPUMappings(nicInfo.RPSCPUs, "rx-"))
		}
	}

	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func formatQueueCPUMappings(mappings map[string]string, prefix string) string {
	var queueMappings []string

	// Extract and sort queue numbers to ensure consistent output
	var queues []int
	for queueStr := range mappings {
		queueNum, err := strconv.Atoi(strings.TrimPrefix(queueStr, prefix))
		if err == nil {
			queues = append(queues, queueNum)
		}
	}
	sort.Ints(queues)

	for _, queueNum := range queues {
		queueStr := fmt.Sprintf("%s%d", prefix, queueNum)
		cpus := mappings[queueStr]
		// a nil value can be returned from the map if the key does not exist, so check for that
		if cpus != "" {
			queueMappings = append(queueMappings, fmt.Sprintf("%d:%s", queueNum, cpus))
		}
	}

	if len(queueMappings) == 0 {
		return ""
	}
	return strings.Join(queueMappings, " | ")
}

func nicCpuAffinityTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	nicIRQMappings := common.NICIrqMappingsFromOutput(outputs)
	if len(nicIRQMappings) == 0 {
		return []table.Field{}
	}
	fields := []table.Field{
		{Name: "Interface"},
		{Name: "IRQ:CPU | IRQ:CPU | ..."},
	}
	for _, nicIRQMapping := range nicIRQMappings {
		fields[0].Values = append(fields[0].Values, nicIRQMapping[0])
		fields[1].Values = append(fields[1].Values, nicIRQMapping[1])
	}
	return fields
}

func networkConfigTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	// these are the fields we want to display
	fields := []table.Field{
		{Name: "net.ipv4.tcp_rmem"},
		{Name: "net.ipv4.tcp_wmem"},
		{Name: "net.core.rmem_max"},
		{Name: "net.core.wmem_max"},
		{Name: "net.core.netdev_max_backlog"},
		{Name: "net.ipv4.tcp_max_syn_backlog"},
		{Name: "net.core.somaxconn"},
		{Name: "IRQ Balance"},
	}
	// load the params into a map so we can easily look them up
	sysctlParams := make(map[string]string)
	for line := range strings.SplitSeq(outputs[script.SysctlScriptName].Stdout, "\n") {
		parts := strings.SplitN(line, " = ", 2)
		if len(parts) != 2 {
			continue
		}
		// if the param name is already in the map, append the value
		if val, ok := sysctlParams[parts[0]]; ok {
			sysctlParams[parts[0]] = val + ", " + parts[1]
		} else {
			sysctlParams[parts[0]] = parts[1]
		}
	}
	// add the values to the fields
	for i := range fields[:len(fields)-1] {
		if val, ok := sysctlParams[fields[i].Name]; ok {
			fields[i].Values = append(fields[i].Values, val)
		} else {
			fields[i].Values = append(fields[i].Values, "")
		}
	}
	fields[len(fields)-1].Values = append(fields[len(fields)-1].Values, strings.TrimSpace(outputs[script.IRQBalanceScriptName].Stdout))
	return fields
}

func diskTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	allDisksInfo := common.DiskInfoFromOutput(outputs)
	if len(allDisksInfo) == 0 {
		return []table.Field{}
	}
	fields := []table.Field{
		{Name: "Name"},
		{Name: "Model"},
		{Name: "Size"},
		{Name: "Mount Point"},
		{Name: "Type"},
		{Name: "Request Queue Size"},
		{Name: "Minimum I/O Size"},
		{Name: "Firmware Version"},
		{Name: "PCIe Address"},
		{Name: "NUMA Node"},
		{Name: "Link Speed"},
		{Name: "Link Width"},
		{Name: "Max Link Speed"},
		{Name: "Max Link Width"},
	}
	for _, diskInfo := range allDisksInfo {
		fields[0].Values = append(fields[0].Values, diskInfo.Name)
		fields[1].Values = append(fields[1].Values, diskInfo.Model)
		fields[2].Values = append(fields[2].Values, diskInfo.Size)
		fields[3].Values = append(fields[3].Values, diskInfo.MountPoint)
		fields[4].Values = append(fields[4].Values, diskInfo.Type)
		fields[5].Values = append(fields[5].Values, diskInfo.RequestQueueSize)
		fields[6].Values = append(fields[6].Values, diskInfo.MinIOSize)
		fields[7].Values = append(fields[7].Values, diskInfo.FirmwareVersion)
		fields[8].Values = append(fields[8].Values, diskInfo.PCIeAddress)
		fields[9].Values = append(fields[9].Values, diskInfo.NUMANode)
		fields[10].Values = append(fields[10].Values, diskInfo.LinkSpeed)
		fields[11].Values = append(fields[11].Values, diskInfo.LinkWidth)
		fields[12].Values = append(fields[12].Values, diskInfo.MaxLinkSpeed)
		fields[13].Values = append(fields[13].Values, diskInfo.MaxLinkWidth)
	}
	return fields
}

func filesystemTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return filesystemFieldValuesFromOutput(outputs)
}

func filesystemTableInsights(outputs map[string]script.ScriptOutput, tableValues table.TableValues) []table.Insight {
	insights := []table.Insight{}
	mountOptionsIndex, err := table.GetFieldIndex("Mount Options", tableValues)
	if err != nil {
		slog.Warn(err.Error())
	} else {
		for i, options := range tableValues.Fields[mountOptionsIndex].Values {
			if strings.Contains(options, "discard") {
				insights = append(insights, table.Insight{
					Recommendation: fmt.Sprintf("Consider mounting the '%s' file system without the 'discard' option and instead configure periodic TRIM for SSDs, if used for I/O intensive workloads.", tableValues.Fields[0].Values[i]),
					Justification:  fmt.Sprintf("The '%s' filesystem is mounted with 'discard' option.", tableValues.Fields[0].Values[i]),
				})
			}
		}
	}
	return insights
}

func gpuTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	gpuInfos := gpuInfoFromOutput(outputs)
	if len(gpuInfos) == 0 {
		return []table.Field{}
	}
	fields := []table.Field{
		{Name: "Manufacturer"},
		{Name: "Model"},
		{Name: "PCI ID"},
	}
	for _, gpuInfo := range gpuInfos {
		fields[0].Values = append(fields[0].Values, gpuInfo.Manufacturer)
		fields[1].Values = append(fields[1].Values, gpuInfo.Model)
		fields[2].Values = append(fields[2].Values, gpuInfo.PCIID)
	}
	return fields
}

func gaudiTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	gaudiInfos := gaudiInfoFromOutput(outputs)
	if len(gaudiInfos) == 0 {
		return []table.Field{}
	}
	fields := []table.Field{
		{Name: "Module ID"},
		{Name: "Microarchitecture"},
		{Name: "Serial Number"},
		{Name: "Bus ID"},
		{Name: "Driver Version"},
		{Name: "EROM"},
		{Name: "CPLD"},
		{Name: "SPI"},
		{Name: "NUMA"},
	}
	for _, gaudiInfo := range gaudiInfos {
		fields[0].Values = append(fields[0].Values, gaudiInfo.ModuleID)
		fields[1].Values = append(fields[1].Values, gaudiInfo.Microarchitecture)
		fields[2].Values = append(fields[2].Values, gaudiInfo.SerialNumber)
		fields[3].Values = append(fields[3].Values, gaudiInfo.BusID)
		fields[4].Values = append(fields[4].Values, gaudiInfo.DriverVersion)
		fields[5].Values = append(fields[5].Values, gaudiInfo.EROM)
		fields[6].Values = append(fields[6].Values, gaudiInfo.CPLD)
		fields[7].Values = append(fields[7].Values, gaudiInfo.SPI)
		fields[8].Values = append(fields[8].Values, gaudiInfo.NUMA)
	}
	return fields
}

func cxlTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	cxlDevices := getPCIDevices("CXL", outputs)
	if len(cxlDevices) == 0 {
		return []table.Field{}
	}
	fields := []table.Field{
		{Name: "Slot"},
		{Name: "Class"},
		{Name: "Vendor"},
		{Name: "Device"},
		{Name: "Rev"},
		{Name: "ProgIf"},
		{Name: "NUMANode"},
		{Name: "IOMMUGroup"},
	}
	for _, cxlDevice := range cxlDevices {
		for fieldIdx, field := range fields {
			if value, ok := cxlDevice[field.Name]; ok {
				fields[fieldIdx].Values = append(fields[fieldIdx].Values, value)
			} else {
				fields[fieldIdx].Values = append(fields[fieldIdx].Values, "")
			}
		}
	}
	return fields
}

func cveTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{}
	cves := cveInfoFromOutput(outputs)
	for _, cve := range cves {
		fields = append(fields, table.Field{Name: cve[0], Values: []string{cve[1]}})
	}
	return fields
}

func cveTableInsights(outputs map[string]script.ScriptOutput, tableValues table.TableValues) []table.Insight {
	insights := []table.Insight{}
	for _, field := range tableValues.Fields {
		if strings.HasPrefix(field.Values[0], "VULN") {
			insights = append(insights, table.Insight{
				Recommendation: fmt.Sprintf("Consider applying the security patch for %s.", field.Name),
				Justification:  fmt.Sprintf("The system is vulnerable to %s.", field.Name),
			})
		}
	}
	return insights
}

func processTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{}
	for i, line := range strings.Split(outputs[script.ProcessListScriptName].Stdout, "\n") {
		tokens := strings.Fields(line)
		if i == 0 { // header -- defines fields in table
			for _, token := range tokens {
				fields = append(fields, table.Field{Name: token})
			}
			continue
		}
		// combine trailing fields
		if len(tokens) > len(fields) {
			tokens[len(fields)-1] = strings.Join(tokens[len(fields)-1:], " ")
			tokens = tokens[:len(fields)]
		}
		for j, token := range tokens {
			fields[j].Values = append(fields[j].Values, token)
		}
	}
	return fields
}

func sensorTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Sensor"},
		{Name: "Reading"},
		{Name: "Status"},
	}
	for line := range strings.SplitSeq(outputs[script.IpmitoolSensorsScriptName].Stdout, "\n") {
		tokens := strings.Split(line, " | ")
		if len(tokens) < len(fields) {
			continue
		}
		fields[0].Values = append(fields[0].Values, tokens[0])
		fields[1].Values = append(fields[1].Values, tokens[1])
		fields[2].Values = append(fields[2].Values, tokens[2])
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func chassisStatusTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{}
	for line := range strings.SplitSeq(outputs[script.IpmitoolChassisScriptName].Stdout, "\n") {
		tokens := strings.Split(line, ":")
		if len(tokens) != 2 {
			continue
		}
		fieldName := strings.TrimSpace(tokens[0])
		fieldValue := strings.TrimSpace(tokens[1])
		if strings.Contains(fieldName, "Button") { // skip button status
			continue
		}
		fields = append(fields, table.Field{Name: fieldName, Values: []string{fieldValue}})
	}
	return fields
}

func systemEventLogTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Date"},
		{Name: "Time"},
		{Name: "Sensor"},
		{Name: "Status"},
		{Name: "Event"},
	}
	for line := range strings.SplitSeq(outputs[script.IpmitoolEventsScriptName].Stdout, "\n") {
		tokens := strings.Split(line, " | ")
		if len(tokens) < len(fields) {
			continue
		}
		fields[0].Values = append(fields[0].Values, tokens[0])
		fields[1].Values = append(fields[1].Values, tokens[1])
		fields[2].Values = append(fields[2].Values, tokens[2])
		fields[3].Values = append(fields[3].Values, tokens[3])
		fields[4].Values = append(fields[4].Values, tokens[4])
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func systemEventLogTableInsights(outputs map[string]script.ScriptOutput, tableValues table.TableValues) []table.Insight {
	insights := []table.Insight{}
	sensorFieldIndex, err := table.GetFieldIndex("Sensor", tableValues)
	if err != nil {
		slog.Warn(err.Error())
	} else {
		temperatureEvents := 0
		for _, sensor := range tableValues.Fields[sensorFieldIndex].Values {
			if strings.Contains(sensor, "Temperature") {
				temperatureEvents++
			}
		}
		if temperatureEvents > 0 {
			insights = append(insights, table.Insight{
				Recommendation: "Consider reviewing the System Event Log table.",
				Justification:  fmt.Sprintf("Detected '%d' temperature-related service action(s) in the System Event Log.", temperatureEvents),
			})
		}
	}
	return insights
}

func kernelLogTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return []table.Field{
		{Name: "Entries", Values: strings.Split(outputs[script.KernelLogScriptName].Stdout, "\n")},
	}
}

func pmuTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return []table.Field{
		{Name: "PMU Driver Version", Values: []string{strings.TrimSpace(outputs[script.PMUDriverVersionScriptName].Stdout)}},
		{Name: "cpu_cycles", Values: []string{common.ValFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0x30a (.*)$`)}},
		{Name: "instructions", Values: []string{common.ValFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0x309 (.*)$`)}},
		{Name: "ref_cycles", Values: []string{common.ValFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0x30b (.*)$`)}},
		{Name: "topdown_slots", Values: []string{common.ValFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0x30c (.*)$`)}},
		{Name: "gen_programmable_1", Values: []string{common.ValFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc1 (.*)$`)}},
		{Name: "gen_programmable_2", Values: []string{common.ValFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc2 (.*)$`)}},
		{Name: "gen_programmable_3", Values: []string{common.ValFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc3 (.*)$`)}},
		{Name: "gen_programmable_4", Values: []string{common.ValFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc4 (.*)$`)}},
		{Name: "gen_programmable_5", Values: []string{common.ValFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc5 (.*)$`)}},
		{Name: "gen_programmable_6", Values: []string{common.ValFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc6 (.*)$`)}},
		{Name: "gen_programmable_7", Values: []string{common.ValFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc7 (.*)$`)}},
		{Name: "gen_programmable_8", Values: []string{common.ValFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc8 (.*)$`)}},
	}
}

func systemSummaryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	system := common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Manufacturer:\s*(.+?)$`) +
		" " + common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Product Name:\s*(.+?)$`) +
		", " + common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Version:\s*(.+?)$`)
	baseboard := common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Manufacturer:\s*(.+?)$`) +
		" " + common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Product Name:\s*(.+?)$`) +
		", " + common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Version:\s*(.+?)$`)
	chassis := common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Manufacturer:\s*(.+?)$`) +
		" " + common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Type:\s*(.+?)$`) +
		", " + common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Version:\s*(.+?)$`)

	return []table.Field{
		{Name: "Host Name", Values: []string{strings.TrimSpace(outputs[script.HostnameScriptName].Stdout)}},
		{Name: "Time", Values: []string{strings.TrimSpace(outputs[script.DateScriptName].Stdout)}},
		{Name: "System", Values: []string{system}},
		{Name: "Baseboard", Values: []string{baseboard}},
		{Name: "Chassis", Values: []string{chassis}},
		{Name: "CPU Model", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^[Mm]odel name:\s*(.+)$`)}},
		{Name: "Architecture", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Architecture:\s*(.+)$`)}},
		{Name: "Microarchitecture", Values: []string{common.UarchFromOutput(outputs)}},
		{Name: "L3 Cache (instance/total)", Values: []string{common.L3FromOutput(outputs)}, Description: "The size of one L3 cache instance and the total L3 cache size for the system."},
		{Name: "Cores per Socket", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(.+)$`)}},
		{Name: "Sockets", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)}},
		{Name: "Hyperthreading", Values: []string{common.HyperthreadingFromOutput(outputs)}},
		{Name: "CPUs", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU\(s\):\s*(.+)$`)}},
		{Name: "Intel Turbo Boost", Values: []string{turboEnabledFromOutput(outputs)}},
		{Name: "Base Frequency", Values: []string{common.BaseFrequencyFromOutput(outputs)}, Description: "The minimum guaranteed speed of a single core under standard conditions."},
		{Name: "Maximum Frequency", Values: []string{common.MaxFrequencyFromOutput(outputs)}, Description: "The highest speed a single core can reach with Turbo Boost."},
		{Name: "All-core Maximum Frequency", Values: []string{common.AllCoreMaxFrequencyFromOutput(outputs)}, Description: "The highest speed all cores can reach simultaneously with Turbo Boost."},
		{Name: "NUMA Nodes", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node\(s\):\s*(.+)$`)}},
		{Name: "Prefetchers", Values: []string{common.PrefetchersSummaryFromOutput(outputs)}},
		{Name: "PPINs", Values: []string{ppinsFromOutput(outputs)}},
		{Name: "Accelerators Available [used]", Values: []string{acceleratorSummaryFromOutput(outputs)}},
		{Name: "Installed Memory", Values: []string{installedMemoryFromOutput(outputs)}},
		{Name: "Hugepagesize", Values: []string{common.ValFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^Hugepagesize:\s*(.+?)$`)}},
		{Name: "Transparent Huge Pages", Values: []string{common.ValFromRegexSubmatch(outputs[script.TransparentHugePagesScriptName].Stdout, `.*\[(.*)\].*`)}},
		{Name: "Automatic NUMA Balancing", Values: []string{numaBalancingFromOutput(outputs)}},
		{Name: "NIC", Values: []string{common.NICSummaryFromOutput(outputs)}},
		{Name: "Disk", Values: []string{common.DiskSummaryFromOutput(outputs)}},
		{Name: "BIOS", Values: []string{common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "0", `^Version:\s*(.+?)$`)}},
		{Name: "Microcode", Values: []string{common.ValFromRegexSubmatch(outputs[script.ProcCpuinfoScriptName].Stdout, `^microcode.*:\s*(.+?)$`)}},
		{Name: "OS", Values: []string{common.OperatingSystemFromOutput(outputs)}},
		{Name: "Kernel", Values: []string{common.ValFromRegexSubmatch(outputs[script.UnameScriptName].Stdout, `^Linux \S+ (\S+)`)}},
		{Name: "TDP", Values: []string{common.TDPFromOutput(outputs)}},
		{Name: "Energy Performance Bias", Values: []string{common.EPBFromOutput(outputs)}},
		{Name: "Scaling Governor", Values: []string{strings.TrimSpace(outputs[script.ScalingGovernorScriptName].Stdout)}},
		{Name: "Scaling Driver", Values: []string{strings.TrimSpace(outputs[script.ScalingDriverScriptName].Stdout)}},
		{Name: "C-states", Values: []string{common.CstatesSummaryFromOutput(outputs)}},
		{Name: "Efficiency Latency Control", Values: []string{common.ELCSummaryFromOutput(outputs)}},
		{Name: "CVEs", Values: []string{cveSummaryFromOutput(outputs)}},
		{Name: "System Summary", Values: []string{systemSummaryFromOutput(outputs)}},
	}
}
func dimmDetails(dimm []string) (details string) {
	if strings.Contains(dimm[SizeIdx], "No") {
		details = "No Module Installed"
	} else {
		// Intel PMEM modules may have serial number appended to end of part number...
		// strip that off so it doesn't mess with color selection later
		partNumber := dimm[PartIdx]
		if strings.Contains(dimm[DetailIdx], "Synchronous Non-Volatile") &&
			dimm[ManufacturerIdx] == "Intel" &&
			strings.HasSuffix(dimm[PartIdx], dimm[SerialIdx]) {
			partNumber = dimm[PartIdx][:len(dimm[PartIdx])-len(dimm[SerialIdx])]
		}
		// example: "64GB DDR5 R2 Synchronous Registered (Buffered) Micron Technology MTC78ASF4G72PZ-2G6E1 6400 MT/s [6000 MT/s]"
		details = fmt.Sprintf("%s %s %s R%s %s %s %s [%s]",
			strings.ReplaceAll(dimm[SizeIdx], " ", ""),
			dimm[TypeIdx],
			dimm[DetailIdx],
			dimm[RankIdx],
			dimm[ManufacturerIdx],
			partNumber,
			strings.ReplaceAll(dimm[SpeedIdx], " ", ""),
			strings.ReplaceAll(dimm[ConfiguredSpeedIdx], " ", ""))
	}
	return
}

func dimmTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	if len(tableValues.Fields) <= max(DerivedSocketIdx, DerivedChannelIdx, DerivedSlotIdx) ||
		len(tableValues.Fields[DerivedSocketIdx].Values) == 0 ||
		len(tableValues.Fields[DerivedChannelIdx].Values) == 0 ||
		len(tableValues.Fields[DerivedSlotIdx].Values) == 0 ||
		tableValues.Fields[DerivedSocketIdx].Values[0] == "" ||
		tableValues.Fields[DerivedChannelIdx].Values[0] == "" ||
		tableValues.Fields[DerivedSlotIdx].Values[0] == "" {
		return report.DefaultHTMLTableRendererFunc(tableValues)
	}
	htmlColors := []string{"lightgreen", "orange", "aqua", "lime", "yellow", "beige", "magenta", "violet", "salmon", "pink"}
	var slotColorIndices = make(map[string]int)
	// socket -> channel -> slot -> dimm details
	var dimms = map[string]map[string]map[string]string{}
	for dimmIdx := range tableValues.Fields[DerivedSocketIdx].Values {
		if _, ok := dimms[tableValues.Fields[DerivedSocketIdx].Values[dimmIdx]]; !ok {
			dimms[tableValues.Fields[DerivedSocketIdx].Values[dimmIdx]] = make(map[string]map[string]string)
		}
		if _, ok := dimms[tableValues.Fields[DerivedSocketIdx].Values[dimmIdx]][tableValues.Fields[DerivedChannelIdx].Values[dimmIdx]]; !ok {
			dimms[tableValues.Fields[DerivedSocketIdx].Values[dimmIdx]][tableValues.Fields[DerivedChannelIdx].Values[dimmIdx]] = make(map[string]string)
		}
		dimmValues := []string{}
		for _, field := range tableValues.Fields {
			dimmValues = append(dimmValues, field.Values[dimmIdx])
		}
		dimms[tableValues.Fields[DerivedSocketIdx].Values[dimmIdx]][tableValues.Fields[DerivedChannelIdx].Values[dimmIdx]][tableValues.Fields[DerivedSlotIdx].Values[dimmIdx]] = dimmDetails(dimmValues)
	}

	var socketTableHeaders = []string{"Socket", ""}
	var socketTableValues [][]string
	var socketKeys []string
	for k := range dimms {
		socketKeys = append(socketKeys, k)
	}
	sort.Strings(socketKeys)
	for _, socket := range socketKeys {
		socketMap := dimms[socket]
		socketTableValues = append(socketTableValues, []string{})
		var channelTableHeaders = []string{"Channel", "Slots"}
		var channelTableValues [][]string
		var channelKeys []int
		for k := range socketMap {
			channel, err := strconv.Atoi(k)
			if err != nil {
				slog.Error("failed to convert channel to int", slog.String("error", err.Error()))
				return ""
			}
			channelKeys = append(channelKeys, channel)
		}
		sort.Ints(channelKeys)
		for _, channel := range channelKeys {
			channelMap := socketMap[strconv.Itoa(channel)]
			channelTableValues = append(channelTableValues, []string{})
			var slotTableHeaders []string
			var slotTableValues [][]string
			var slotTableValuesStyles [][]string
			var slotKeys []string
			for k := range channelMap {
				slotKeys = append(slotKeys, k)
			}
			sort.Strings(slotKeys)
			slotTableValues = append(slotTableValues, []string{})
			slotTableValuesStyles = append(slotTableValuesStyles, []string{})
			for _, slot := range slotKeys {
				dimmDetails := channelMap[slot]
				slotTableValues[0] = append(slotTableValues[0], htmltemplate.HTMLEscapeString(dimmDetails))
				var slotColor string
				if dimmDetails == "No Module Installed" {
					slotColor = "background-color:silver"
				} else {
					if _, ok := slotColorIndices[dimmDetails]; !ok {
						slotColorIndices[dimmDetails] = int(math.Min(float64(len(slotColorIndices)), float64(len(htmlColors)-1)))
					}
					slotColor = "background-color:" + htmlColors[slotColorIndices[dimmDetails]]
				}
				slotTableValuesStyles[0] = append(slotTableValuesStyles[0], slotColor)
			}
			slotTable := report.RenderHTMLTable(slotTableHeaders, slotTableValues, "pure-table pure-table-bordered", slotTableValuesStyles)
			// channel number
			channelTableValues[len(channelTableValues)-1] = append(channelTableValues[len(channelTableValues)-1], strconv.Itoa(channel))
			// slot table
			channelTableValues[len(channelTableValues)-1] = append(channelTableValues[len(channelTableValues)-1], slotTable)
			// style
		}
		channelTable := report.RenderHTMLTable(channelTableHeaders, channelTableValues, "pure-table pure-table-bordered", [][]string{})
		// socket number
		socketTableValues[len(socketTableValues)-1] = append(socketTableValues[len(socketTableValues)-1], socket)
		// channel table
		socketTableValues[len(socketTableValues)-1] = append(socketTableValues[len(socketTableValues)-1], channelTable)
	}
	return report.RenderHTMLTable(socketTableHeaders, socketTableValues, "pure-table pure-table-bordered", [][]string{})
}
