package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// table_defs.go defines the tables used for generating reports

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"perfspect/internal/script"

	"github.com/xuri/excelize/v2"
)

// Field represents the values for a field in a table
type Field struct {
	Name   string
	Values []string
}

// TableValues combines the table definition with the resulting fields and their values
type TableValues struct {
	TableDefinition
	Fields   []Field
	Insights []Insight
}

// Insight represents an insight about the data in a table
type Insight struct {
	Recommendation string
	Justification  string
}

type FieldsRetriever func(map[string]script.ScriptOutput) []Field
type InsightsRetriever func(map[string]script.ScriptOutput, TableValues) []Insight
type HTMLTableRenderer func(TableValues, string) string
type HTMLMultiTargetTableRenderer func([]TableValues, []string) string
type TextTableRenderer func(TableValues) string
type XlsxTableRenderer func(TableValues, *excelize.File, string, *int)

// TableDefinition defines the structure of a table in the report
type TableDefinition struct {
	Name          string
	ScriptNames   []string
	Architectures []string // architectures, i.e., x86_64, aarch64. If empty, it will be present for all architectures.
	Vendors       []string // vendors, e.g., GenuineIntel, AuthenticAMD. If empty, it will be present for all vendors.
	Families      []string // families, e.g., 6, 7. If empty, it will be present for all families.
	Models        []string // models, e.g., 62, 63. If empty, it will be present for all models.
	// Fields function is called to retrieve field values from the script outputs
	FieldsFunc  FieldsRetriever
	MenuLabel   string // add to tables that will be displayed in the menu
	HasRows     bool   // table is meant to be displayed in row form, i.e., a field may have multiple values
	NoDataFound string // message to display when no data is found
	// render functions are used to override the default rendering behavior
	HTMLTableRendererFunc            HTMLTableRenderer
	HTMLMultiTargetTableRendererFunc HTMLMultiTargetTableRenderer
	TextTableRendererFunc            TextTableRenderer
	XlsxTableRendererFunc            XlsxTableRenderer
	// insights function is used to retrieve insights about the data in the table
	InsightsFunc InsightsRetriever
}

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
	NICTableName               = "NIC"
	NetworkIRQMappingTableName = "Network IRQ Mapping"
	NetworkConfigTableName     = "Network Configuration"
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
	// benchmark table names
	SpeedBenchmarkTableName       = "Speed Benchmark"
	PowerBenchmarkTableName       = "Power Benchmark"
	TemperatureBenchmarkTableName = "Temperature Benchmark"
	FrequencyBenchmarkTableName   = "Frequency Benchmark"
	MemoryBenchmarkTableName      = "Memory Benchmark"
	NUMABenchmarkTableName        = "NUMA Benchmark"
	StorageBenchmarkTableName     = "Storage Benchmark"
	// telemetry table names
	CPUUtilizationTelemetryTableName        = "CPU Utilization Telemetry"
	UtilizationCategoriesTelemetryTableName = "Utilization Categories Telemetry"
	IPCTelemetryTableName                   = "IPC Telemetry"
	C6TelemetryTableName                    = "C6 Telemetry"
	FrequencyTelemetryTableName             = "Frequency Telemetry"
	IRQRateTelemetryTableName               = "IRQ Rate Telemetry"
	InstructionTelemetryTableName           = "Instruction Telemetry"
	DriveTelemetryTableName                 = "Drive Telemetry"
	NetworkTelemetryTableName               = "Network Telemetry"
	MemoryTelemetryTableName                = "Memory Telemetry"
	PowerTelemetryTableName                 = "Power Telemetry"
	TemperatureTelemetryTableName           = "Temperature Telemetry"
	GaudiTelemetryTableName                 = "Gaudi Telemetry"
	// config  table names
	ConfigurationTableName = "Configuration"
	// flamegraph table names
	CallStackFrequencyTableName = "Call Stack Frequency"
	// lock table names
	KernelLockAnalysisTableName = "Kernel Lock Analysis"
	// common table names
	BriefSysSummaryTableName = "Brief System Summary"
)

// menu labels
const (
	// telemetry table menu labels
	CPUUtilizationTelemetryMenuLabel        = "CPU Utilization"
	UtilizationCategoriesTelemetryMenuLabel = "Utilization Categories"
	IPCTelemetryMenuLabel                   = "IPC"
	C6TelemetryMenuLabel                    = "C6"
	FrequencyTelemetryMenuLabel             = "Frequency"
	IRQRateTelemetryMenuLabel               = "IRQ Rate"
	InstructionTelemetryMenuLabel           = "Instruction"
	DriveTelemetryMenuLabel                 = "Drive"
	NetworkTelemetryMenuLabel               = "Network"
	MemoryTelemetryMenuLabel                = "Memory"
	PowerTelemetryMenuLabel                 = "Power"
	TemperatureTelemetryMenuLabel           = "Temperature"
	GaudiTelemetryMenuLabel                 = "Gaudi"
)

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

var tableDefinitions = map[string]TableDefinition{
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
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
			script.CpuidScriptName,
			script.BaseFrequencyScriptName,
			script.MaximumFrequencyScriptName,
			script.SpecCoreFrequenciesScriptName,
			script.PPINName,
			script.L3CacheWayEnabledName},
		FieldsFunc:   cpuTableValues,
		InsightsFunc: cpuTableInsights},
	PrefetcherTableName: {
		Name:    PrefetcherTableName,
		HasRows: true,
		Vendors: []string{"GenuineIntel"},
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
		Architectures: []string{"x86_64"},
		ScriptNames:   []string{script.CpuidScriptName},
		FieldsFunc:    isaTableValues},
	AcceleratorTableName: {
		Name:    AcceleratorTableName,
		Vendors: []string{"GenuineIntel"},
		Models:  []string{"143", "207", "173", "174", "175", "221"}, // Sapphire Rapids, Emerald Rapids, Granite Rapids, Granite Rapids D, Sierra Forest, Clearwater Forest
		HasRows: true,
		ScriptNames: []string{
			script.LshwScriptName,
			script.IaaDevicesScriptName,
			script.DsaDevicesScriptName},
		FieldsFunc:   acceleratorTableValues,
		InsightsFunc: acceleratorTableInsights},
	PowerTableName: {
		Name:      PowerTableName,
		Vendors:   []string{"GenuineIntel"},
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
		Vendors: []string{"GenuineIntel"},
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
		Vendors: []string{"GenuineIntel"},
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
		Name:     ElcTableName,
		Families: []string{"6"},                        // Intel CPUs only
		Models:   []string{"173", "174", "175", "221"}, // Granite Rapids, Granite Rapids D, Sierra Forest, Clearwater Forest
		HasRows:  true,
		ScriptNames: []string{
			script.ElcScriptName,
		},
		FieldsFunc:   elcTableValues,
		InsightsFunc: elcTableInsights},
	SSTTFHPTableName: {
		Name:     SSTTFHPTableName,
		Families: []string{"6"},                        // Intel CPUs only
		Models:   []string{"173", "174", "175", "221"}, // Granite Rapids, Granite Rapids D, Sierra Forest, Clearwater Forest
		HasRows:  true,
		ScriptNames: []string{
			script.SSTTFHPScriptName,
		},
		FieldsFunc: sstTFHPTableValues},
	SSTTFLPTableName: {
		Name:     SSTTFLPTableName,
		Families: []string{"6"},                        // Intel CPUs only
		Models:   []string{"173", "174", "175", "221"}, // Granite Rapids, Granite Rapids D, Sierra Forest, Clearwater Forest
		HasRows:  true,
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
			script.TmeScriptName},
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
		},
		FieldsFunc:            dimmTableValues,
		InsightsFunc:          dimmTableInsights,
		HTMLTableRendererFunc: dimmTableHTMLRenderer},
	NICTableName: {
		Name:      NICTableName,
		HasRows:   true,
		MenuLabel: NetworkMenuLabel,
		ScriptNames: []string{
			script.NicInfoScriptName,
		},
		FieldsFunc: nicTableValues},
	NetworkConfigTableName: {
		Name:    NetworkConfigTableName,
		HasRows: false,
		ScriptNames: []string{
			script.SysctlScriptName,
		},
		FieldsFunc: networkConfigTableValues},
	NetworkIRQMappingTableName: {
		Name:    NetworkIRQMappingTableName,
		HasRows: true,
		ScriptNames: []string{
			script.NicInfoScriptName,
		},
		FieldsFunc: networkIRQMappingTableValues},
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
		Architectures: []string{"x86_64"},
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
		Vendors: []string{"GenuineIntel"},
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
		},
		FieldsFunc: systemSummaryTableValues},
	BriefSysSummaryTableName: {
		Name:      BriefSysSummaryTableName,
		MenuLabel: BriefSysSummaryTableName,
		HasRows:   false,
		ScriptNames: []string{
			script.HostnameScriptName,
			script.DateScriptName,
			script.LscpuScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
			script.MaximumFrequencyScriptName,
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
		},
		FieldsFunc: briefSummaryTableValues},
	//
	// configuration set table
	//
	ConfigurationTableName: {
		Name:    ConfigurationTableName,
		Vendors: []string{"GenuineIntel"},
		HasRows: false,
		ScriptNames: []string{
			script.LscpuScriptName,
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
		},
		FieldsFunc: configurationTableValues},
	//
	// benchmarking tables
	//
	SpeedBenchmarkTableName: {
		Name:      SpeedBenchmarkTableName,
		MenuLabel: SpeedBenchmarkTableName,
		HasRows:   false,
		ScriptNames: []string{
			script.SpeedBenchmarkScriptName,
		},
		FieldsFunc: speedBenchmarkTableValues},
	PowerBenchmarkTableName: {
		Name:          PowerBenchmarkTableName,
		MenuLabel:     PowerBenchmarkTableName,
		Architectures: []string{"x86_64"},
		HasRows:       false,
		ScriptNames: []string{
			script.IdlePowerBenchmarkScriptName,
			script.PowerBenchmarkScriptName,
		},
		FieldsFunc: powerBenchmarkTableValues},
	TemperatureBenchmarkTableName: {
		Name:          TemperatureBenchmarkTableName,
		MenuLabel:     TemperatureBenchmarkTableName,
		Architectures: []string{"x86_64"},
		HasRows:       false,
		ScriptNames: []string{
			script.PowerBenchmarkScriptName,
		},
		FieldsFunc: temperatureBenchmarkTableValues},
	FrequencyBenchmarkTableName: {
		Name:          FrequencyBenchmarkTableName,
		MenuLabel:     FrequencyBenchmarkTableName,
		Architectures: []string{"x86_64"},
		HasRows:       true,
		ScriptNames: []string{
			script.SpecCoreFrequenciesScriptName,
			script.LscpuScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
			script.FrequencyBenchmarkScriptName,
		},
		FieldsFunc:            frequencyBenchmarkTableValues,
		HTMLTableRendererFunc: frequencyBenchmarkTableHtmlRenderer},
	MemoryBenchmarkTableName: {
		Name:          MemoryBenchmarkTableName,
		MenuLabel:     MemoryBenchmarkTableName,
		Architectures: []string{"x86_64"},
		HasRows:       true,
		ScriptNames: []string{
			script.MemoryBenchmarkScriptName,
		},
		NoDataFound:                      "No memory benchmark data found. Please see the GitHub repository README for instructions on how to install Intel Memory Latency Checker (mlc).",
		FieldsFunc:                       memoryBenchmarkTableValues,
		HTMLTableRendererFunc:            memoryBenchmarkTableHtmlRenderer,
		HTMLMultiTargetTableRendererFunc: memoryBenchmarkTableMultiTargetHtmlRenderer},
	NUMABenchmarkTableName: {
		Name:          NUMABenchmarkTableName,
		MenuLabel:     NUMABenchmarkTableName,
		Architectures: []string{"x86_64"},
		HasRows:       true,
		ScriptNames: []string{
			script.NumaBenchmarkScriptName,
		},
		NoDataFound: "No NUMA benchmark data found. Please see the GitHub repository README for instructions on how to install Intel Memory Latency Checker (mlc).",
		FieldsFunc:  numaBenchmarkTableValues},
	StorageBenchmarkTableName: {
		Name:      StorageBenchmarkTableName,
		MenuLabel: StorageBenchmarkTableName,
		HasRows:   false,
		ScriptNames: []string{
			script.StorageBenchmarkScriptName,
		},
		FieldsFunc: storageBenchmarkTableValues},
	//
	// telemetry tables
	//
	CPUUtilizationTelemetryTableName: {
		Name:      CPUUtilizationTelemetryTableName,
		MenuLabel: CPUUtilizationTelemetryMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.MpstatTelemetryScriptName,
		},
		FieldsFunc:            cpuUtilizationTelemetryTableValues,
		HTMLTableRendererFunc: cpuUtilizationTelemetryTableHTMLRenderer},
	UtilizationCategoriesTelemetryTableName: {
		Name:      UtilizationCategoriesTelemetryTableName,
		MenuLabel: UtilizationCategoriesTelemetryMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.MpstatTelemetryScriptName,
		},
		FieldsFunc:            utilizationCategoriesTelemetryTableValues,
		HTMLTableRendererFunc: utilizationCategoriesTelemetryTableHTMLRenderer},
	IPCTelemetryTableName: {
		Name:          IPCTelemetryTableName,
		MenuLabel:     IPCTelemetryMenuLabel,
		Architectures: []string{"x86_64"},
		HasRows:       true,
		ScriptNames: []string{
			script.TurbostatTelemetryScriptName,
		},
		FieldsFunc:            ipcTelemetryTableValues,
		HTMLTableRendererFunc: ipcTelemetryTableHTMLRenderer},
	C6TelemetryTableName: {
		Name:          C6TelemetryTableName,
		MenuLabel:     C6TelemetryMenuLabel,
		Architectures: []string{"x86_64"},
		HasRows:       true,
		ScriptNames: []string{
			script.TurbostatTelemetryScriptName,
		},
		FieldsFunc:            c6TelemetryTableValues,
		HTMLTableRendererFunc: c6TelemetryTableHTMLRenderer},
	FrequencyTelemetryTableName: {
		Name:          FrequencyTelemetryTableName,
		MenuLabel:     FrequencyTelemetryMenuLabel,
		Architectures: []string{"x86_64"},
		HasRows:       true,
		ScriptNames: []string{
			script.TurbostatTelemetryScriptName,
		},
		FieldsFunc:            frequencyTelemetryTableValues,
		HTMLTableRendererFunc: averageFrequencyTelemetryTableHTMLRenderer},
	IRQRateTelemetryTableName: {
		Name:      IRQRateTelemetryTableName,
		MenuLabel: IRQRateTelemetryMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.MpstatTelemetryScriptName,
		},
		FieldsFunc:            irqRateTelemetryTableValues,
		HTMLTableRendererFunc: irqRateTelemetryTableHTMLRenderer},
	DriveTelemetryTableName: {
		Name:      DriveTelemetryTableName,
		MenuLabel: DriveTelemetryMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.IostatTelemetryScriptName,
		},
		FieldsFunc:            driveTelemetryTableValues,
		HTMLTableRendererFunc: driveTelemetryTableHTMLRenderer},
	NetworkTelemetryTableName: {
		Name:      NetworkTelemetryTableName,
		MenuLabel: NetworkTelemetryMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.NetworkTelemetryScriptName,
		},
		FieldsFunc:            networkTelemetryTableValues,
		HTMLTableRendererFunc: networkTelemetryTableHTMLRenderer},
	MemoryTelemetryTableName: {
		Name:      MemoryTelemetryTableName,
		MenuLabel: MemoryTelemetryMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.MemoryTelemetryScriptName,
		},
		FieldsFunc:            memoryTelemetryTableValues,
		HTMLTableRendererFunc: memoryTelemetryTableHTMLRenderer},
	PowerTelemetryTableName: {
		Name:          PowerTelemetryTableName,
		MenuLabel:     PowerTelemetryMenuLabel,
		Architectures: []string{"x86_64"},
		HasRows:       true,
		ScriptNames: []string{
			script.TurbostatTelemetryScriptName,
		},
		FieldsFunc:            powerTelemetryTableValues,
		HTMLTableRendererFunc: powerTelemetryTableHTMLRenderer},
	TemperatureTelemetryTableName: {
		Name:          TemperatureTelemetryTableName,
		MenuLabel:     TemperatureTelemetryMenuLabel,
		Architectures: []string{"x86_64"},
		HasRows:       true,
		ScriptNames: []string{
			script.TurbostatTelemetryScriptName,
		},
		FieldsFunc:            temperatureTelemetryTableValues,
		HTMLTableRendererFunc: temperatureTelemetryTableHTMLRenderer},
	InstructionTelemetryTableName: {
		Name:          InstructionTelemetryTableName,
		MenuLabel:     InstructionTelemetryMenuLabel,
		Architectures: []string{"x86_64"},
		HasRows:       true,
		ScriptNames: []string{
			script.InstructionTelemetryScriptName,
		},
		FieldsFunc:            instructionTelemetryTableValues,
		HTMLTableRendererFunc: instructionTelemetryTableHTMLRenderer},
	GaudiTelemetryTableName: {
		Name:          GaudiTelemetryTableName,
		MenuLabel:     GaudiTelemetryMenuLabel,
		Architectures: []string{"x86_64"},
		HasRows:       true,
		ScriptNames: []string{
			script.GaudiTelemetryScriptName,
		},
		NoDataFound:           "No Gaudi telemetry found. Gaudi devices and the hl-smi tool must be installed on the target system to collect Gaudi stats.",
		FieldsFunc:            gaudiTelemetryTableValues,
		HTMLTableRendererFunc: gaudiTelemetryTableHTMLRenderer},
	//
	// flamegraph tables
	//
	CallStackFrequencyTableName: {
		Name:      CallStackFrequencyTableName,
		MenuLabel: CallStackFrequencyTableName,
		ScriptNames: []string{
			script.CollapsedCallStacksScriptName,
		},
		FieldsFunc:            callStackFrequencyTableValues,
		HTMLTableRendererFunc: callStackFrequencyTableHTMLRenderer},
	//
	// kernel lock analysis tables
	//
	KernelLockAnalysisTableName: {
		Name:      KernelLockAnalysisTableName,
		MenuLabel: KernelLockAnalysisTableName,
		ScriptNames: []string{
			script.ProfileKernelLockScriptName,
		},
		FieldsFunc:            kernelLockAnalysisTableValues,
		HTMLTableRendererFunc: kernelLockAnalysisHTMLRenderer,
	},
}

// GetScriptNamesForTable returns the script names required to generate the table with the given name
func GetScriptNamesForTable(name string) []string {
	if _, ok := tableDefinitions[name]; !ok {
		panic(fmt.Sprintf("table not found: %s", name))
	}
	return tableDefinitions[name].ScriptNames
}

// GetValuesForTable returns the fields and their values for the table with the given name
func GetValuesForTable(name string, outputs map[string]script.ScriptOutput) TableValues {
	// if table with given name doesn't exist, panic
	if _, ok := tableDefinitions[name]; !ok {
		panic(fmt.Sprintf("table not found: %s", name))
	}
	table := tableDefinitions[name]
	// ValuesFunc can't be nil
	if table.FieldsFunc == nil {
		panic(fmt.Sprintf("table %s, ValuesFunc cannot be nil", name))
	}
	// call the table's FieldsFunc to get the table's fields and values
	fields := table.FieldsFunc(outputs)
	tableValues := TableValues{
		TableDefinition: tableDefinitions[name],
		Fields:          fields,
	}
	// sanity check
	if err := validateTableValues(tableValues); err != nil {
		slog.Error("table validation failed", "table", name, "error", err)
		return TableValues{
			TableDefinition: tableDefinitions[name],
			Fields:          []Field{},
		}
	}
	// call the table's InsightsFunc to get insights about the data in the table
	if table.InsightsFunc != nil {
		tableValues.Insights = table.InsightsFunc(outputs, tableValues)
	}
	return tableValues
}

func getFieldIndex(fieldName string, tableValues TableValues) (int, error) {
	for i, field := range tableValues.Fields {
		if field.Name == fieldName {
			if len(field.Values) == 0 {
				return -1, fmt.Errorf("field [%s] does not have associated value(s)", field.Name)
			}
			return i, nil
		}
	}
	return -1, fmt.Errorf("field [%s] not found in table [%s]", fieldName, tableValues.Name)
}

func validateTableValues(tableValues TableValues) error {
	if tableValues.Name == "" {
		return fmt.Errorf("table name cannot be empty")
	}
	// no field values is a valid state
	if len(tableValues.Fields) == 0 {
		return nil
	}
	// field names cannot be empty
	for i, field := range tableValues.Fields {
		if field.Name == "" {
			return fmt.Errorf("table %s, field %d, name cannot be empty", tableValues.Name, i)
		}
	}
	// the number of entries in each field must be the same
	numEntries := len(tableValues.Fields[0].Values)
	for i, field := range tableValues.Fields {
		if len(field.Values) != numEntries {
			return fmt.Errorf("table %s, field %d, %s, number of entries must be the same for all fields, expected %d, got %d", tableValues.Name, i, field.Name, numEntries, len(field.Values))
		}
	}
	return nil
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

func hostTableValues(outputs map[string]script.ScriptOutput) []Field {
	hostName := strings.TrimSpace(outputs[script.HostnameScriptName].Stdout)
	time := strings.TrimSpace(outputs[script.DateScriptName].Stdout)
	system := valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Manufacturer:\s*(.+?)$`) +
		" " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Product Name:\s*(.+?)$`) +
		", " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Version:\s*(.+?)$`)
	baseboard := valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Manufacturer:\s*(.+?)$`) +
		" " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Product Name:\s*(.+?)$`) +
		", " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Version:\s*(.+?)$`)
	chassis := valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Manufacturer:\s*(.+?)$`) +
		" " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Type:\s*(.+?)$`) +
		", " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Version:\s*(.+?)$`)
	return []Field{
		{Name: "Host Name", Values: []string{hostName}},
		{Name: "Time", Values: []string{time}},
		{Name: "System", Values: []string{system}},
		{Name: "Baseboard", Values: []string{baseboard}},
		{Name: "Chassis", Values: []string{chassis}},
	}
}

func pcieSlotsTableValues(outputs map[string]script.ScriptOutput) []Field {
	fieldValues := valsArrayFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "9",
		[]string{
			`^Designation:\s*(.+?)$`,
			`^Type:\s*(.+?)$`,
			`^Length:\s*(.+?)$`,
			`^Bus Address:\s*(.+?)$`,
			`^Current Usage:\s*(.+?)$`,
		}...,
	)
	if len(fieldValues) == 0 {
		return []Field{}
	}
	fields := []Field{
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

func biosTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Vendor"},
		{Name: "Version"},
		{Name: "Release Date"},
	}
	fieldValues := valsArrayFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "0",
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

func operatingSystemTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "OS", Values: []string{operatingSystemFromOutput(outputs)}},
		{Name: "Kernel", Values: []string{valFromRegexSubmatch(outputs[script.UnameScriptName].Stdout, `^Linux \S+ (\S+)`)}},
		{Name: "Boot Parameters", Values: []string{strings.TrimSpace(outputs[script.ProcCmdlineScriptName].Stdout)}},
		{Name: "Microcode", Values: []string{valFromRegexSubmatch(outputs[script.ProcCpuinfoScriptName].Stdout, `^microcode.*:\s*(.+?)$`)}},
	}
}

func softwareVersionTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "GCC", Values: []string{valFromRegexSubmatch(outputs[script.GccVersionScriptName].Stdout, `^(gcc .*)$`)}},
		{Name: "GLIBC", Values: []string{valFromRegexSubmatch(outputs[script.GlibcVersionScriptName].Stdout, `^(ldd .*)`)}},
		{Name: "Binutils", Values: []string{valFromRegexSubmatch(outputs[script.BinutilsVersionScriptName].Stdout, `^(GNU ld .*)$`)}},
		{Name: "Python", Values: []string{valFromRegexSubmatch(outputs[script.PythonVersionScriptName].Stdout, `^(Python .*)$`)}},
		{Name: "Python3", Values: []string{valFromRegexSubmatch(outputs[script.Python3VersionScriptName].Stdout, `^(Python 3.*)$`)}},
		{Name: "Java", Values: []string{valFromRegexSubmatch(outputs[script.JavaVersionScriptName].Stdout, `^(openjdk .*)$`)}},
		{Name: "OpenSSL", Values: []string{valFromRegexSubmatch(outputs[script.OpensslVersionScriptName].Stdout, `^(OpenSSL .*)$`)}},
	}
}

func cpuTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "CPU Model", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^[Mm]odel name:\s*(.+)$`)}},
		{Name: "Architecture", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Architecture:\s*(.+)$`)}},
		{Name: "Microarchitecture", Values: []string{UarchFromOutput(outputs)}},
		{Name: "Family", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU family:\s*(.+)$`)}},
		{Name: "Model", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model:\s*(.+)$`)}},
		{Name: "Stepping", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Stepping:\s*(.+)$`)}},
		{Name: "Base Frequency", Values: []string{baseFrequencyFromOutput(outputs)}},
		{Name: "Maximum Frequency", Values: []string{maxFrequencyFromOutput(outputs)}},
		{Name: "All-core Maximum Frequency", Values: []string{allCoreMaxFrequencyFromOutput(outputs)}},
		{Name: "CPUs", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU\(s\):\s*(.+)$`)}},
		{Name: "On-line CPU List", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^On-line CPU\(s\) list:\s*(.+)$`)}},
		{Name: "Hyperthreading", Values: []string{hyperthreadingFromOutput(outputs)}},
		{Name: "Cores per Socket", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(.+)$`)}},
		{Name: "Sockets", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)}},
		{Name: "NUMA Nodes", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node\(s\):\s*(.+)$`)}},
		{Name: "NUMA CPU List", Values: []string{numaCPUListFromOutput(outputs)}},
		{Name: "L1d Cache", Values: []string{l1dFromOutput(outputs)}},
		{Name: "L1i Cache", Values: []string{l1iFromOutput(outputs)}},
		{Name: "L2 Cache", Values: []string{l2FromOutput(outputs)}},
		{Name: "L3 Cache", Values: []string{l3FromOutput(outputs)}},
		{Name: "L3 per Core", Values: []string{l3PerCoreFromOutput(outputs)}},
		{Name: "Memory Channels", Values: []string{channelsFromOutput(outputs)}},
		{Name: "Intel Turbo Boost", Values: []string{turboEnabledFromOutput(outputs)}},
		{Name: "Virtualization", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Virtualization:\s*(.+)$`)}},
		{Name: "PPINs", Values: []string{ppinsFromOutput(outputs)}},
	}
}

func prefetcherTableValues(outputs map[string]script.ScriptOutput) []Field {
	prefetchers := prefetchersFromOutput(outputs)
	if len(prefetchers) == 0 {
		return []Field{}
	}
	fields := []Field{
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

func cpuTableInsights(outputs map[string]script.ScriptOutput, tableValues TableValues) []Insight {
	insights := []Insight{}
	addInsightFunc := func(fieldName, bestValue string) {
		fieldIndex, err := getFieldIndex(fieldName, tableValues)
		if err != nil {
			slog.Warn(err.Error())
		} else {
			fieldValue := tableValues.Fields[fieldIndex].Values[0]
			if fieldValue != "" && fieldValue != "N/A" && fieldValue != bestValue {
				insights = append(insights, Insight{
					Recommendation: fmt.Sprintf("Consider enabling %s.", fieldName),
					Justification:  fmt.Sprintf("%s is not enabled.", fieldName),
				})
			}
		}
	}
	addInsightFunc("Hyperthreading", "Enabled")
	addInsightFunc("Intel Turbo Boost", "Enabled")
	// Xeon Generation
	familyIndex, err := getFieldIndex("Family", tableValues)
	if err != nil {
		slog.Warn(err.Error())
	} else {
		family := tableValues.Fields[familyIndex].Values[0]
		if family == "6" { // Intel
			uarchIndex, err := getFieldIndex("Microarchitecture", tableValues)
			if err != nil {
				slog.Warn(err.Error())
			} else {
				xeonGens := map[string]int{
					"HSX": 1,
					"BDX": 2,
					"SKX": 3,
					"CLX": 4,
					"ICX": 5,
					"SPR": 6,
					"EMR": 7,
					"SRF": 8,
					"CWF": 8,
					"GNR": 8,
				}
				uarch := tableValues.Fields[uarchIndex].Values[0]
				if len(uarch) >= 3 {
					xeonGen, ok := xeonGens[uarch[:3]]
					if ok {
						if xeonGen < xeonGens["SPR"] {
							insights = append(insights, Insight{
								Recommendation: "Consider upgrading to the latest generation Intel(r) Xeon(r) CPU.",
								Justification:  "The CPU is 2 or more generations behind the latest Intel(r) Xeon(r) CPU.",
							})
						}
					}
				}
			}
		} else {
			insights = append(insights, Insight{
				Recommendation: "Consider upgrading to an Intel(r) Xeon(r) CPU.",
				Justification:  "The current CPU is not an Intel(r) Xeon(r) CPU.",
			})
		}
	}
	return insights
}

func isaTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{}
	supported := isaSupportedFromOutput(outputs)
	for i, isa := range isaFullNames() {
		fields = append(fields, Field{
			Name:   isa,
			Values: []string{supported[i]},
		})
	}
	return fields
}

func acceleratorTableValues(outputs map[string]script.ScriptOutput) []Field {
	names := acceleratorNames()
	if len(names) == 0 {
		return []Field{}
	}
	return []Field{
		{Name: "Name", Values: names},
		{Name: "Count", Values: acceleratorCountsFromOutput(outputs)},
		{Name: "Work Queues", Values: acceleratorWorkQueuesFromOutput(outputs)},
		{Name: "Full Name", Values: acceleratorFullNamesFromYaml()},
		{Name: "Description", Values: acceleratorDescriptionsFromYaml()},
	}
}

func acceleratorTableInsights(outputs map[string]script.ScriptOutput, tableValues TableValues) []Insight {
	insights := []Insight{}
	nameFieldIndex, err := getFieldIndex("Name", tableValues)
	if err != nil {
		slog.Warn(err.Error())
		return insights
	}
	countFieldIndex, err := getFieldIndex("Count", tableValues)
	if err != nil {
		slog.Warn(err.Error())
		return insights
	}
	queuesFieldIndex, err := getFieldIndex("Work Queues", tableValues)
	if err != nil {
		slog.Warn(err.Error())
		return insights
	}
	for i, count := range tableValues.Fields[countFieldIndex].Values {
		name := tableValues.Fields[nameFieldIndex].Values[i]
		queues := tableValues.Fields[queuesFieldIndex].Values[i]
		if name == "DSA" && count != "0" && queues == "None" {
			insights = append(insights, Insight{
				Recommendation: "Consider configuring DSA to allow accelerated data copy and transformation in DSA-enabled software.",
				Justification:  "No work queues are configured for DSA accelerator(s).",
			})
		}
		if name == "IAA" && count != "0" && queues == "None" {
			insights = append(insights, Insight{
				Recommendation: "Consider configuring IAA to allow accelerated compression and decompression in IAA-enabled software.",
				Justification:  "No work queues are configured for IAA accelerator(s).",
			})
		}
	}
	return insights
}

func powerTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "TDP", Values: []string{tdpFromOutput(outputs)}},
		{Name: "Energy Performance Bias", Values: []string{epbFromOutput(outputs)}},
		{Name: "Energy Performance Preference", Values: []string{eppFromOutput(outputs)}},
		{Name: "Scaling Governor", Values: []string{strings.TrimSpace(outputs[script.ScalingGovernorScriptName].Stdout)}},
		{Name: "Scaling Driver", Values: []string{strings.TrimSpace(outputs[script.ScalingDriverScriptName].Stdout)}},
	}
}

func powerTableInsights(outputs map[string]script.ScriptOutput, tableValues TableValues) []Insight {
	insights := []Insight{}
	addInsightFunc := func(fieldName, bestValue string) {
		fieldIndex, err := getFieldIndex(fieldName, tableValues)
		if err != nil {
			slog.Warn(err.Error())
		} else {
			fieldValue := tableValues.Fields[fieldIndex].Values[0]
			if fieldValue != "" && fieldValue != bestValue {
				insights = append(insights, Insight{
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

func cstateTableValues(outputs map[string]script.ScriptOutput) []Field {
	cstates := cstatesFromOutput(outputs)
	if len(cstates) == 0 {
		return []Field{}
	}
	fields := []Field{
		{Name: "Name"},
		{Name: "Status"}, // enabled/disabled
	}
	for _, cstateInfo := range cstates {
		fields[0].Values = append(fields[0].Values, cstateInfo.Name)
		fields[1].Values = append(fields[1].Values, cstateInfo.Status)
	}
	return fields
}

func uncoreTableValues(outputs map[string]script.ScriptOutput) []Field {
	uarch := UarchFromOutput(outputs)
	if uarch == "" {
		slog.Error("failed to get uarch from script outputs")
		return []Field{}
	}
	if strings.Contains(uarch, "SRF") || strings.Contains(uarch, "GNR") || strings.Contains(uarch, "CWF") {
		return []Field{
			{Name: "Min Frequency (Compute)", Values: []string{uncoreMinMaxDieFrequencyFromOutput(false, true, outputs)}},
			{Name: "Min Frequency (I/O)", Values: []string{uncoreMinMaxDieFrequencyFromOutput(false, false, outputs)}},
			{Name: "Max Frequency (Compute)", Values: []string{uncoreMinMaxDieFrequencyFromOutput(true, true, outputs)}},
			{Name: "Max Frequency (I/O)", Values: []string{uncoreMinMaxDieFrequencyFromOutput(true, false, outputs)}},
			{Name: "CHA Count", Values: []string{chaCountFromOutput(outputs)}},
		}
	} else { // field counts need to match for the all_hosts reports to work properly
		return []Field{
			{Name: "Min Frequency", Values: []string{uncoreMinFrequencyFromOutput(outputs)}},
			{Name: "N/A", Values: []string{""}},
			{Name: "Max Frequency", Values: []string{uncoreMaxFrequencyFromOutput(outputs)}},
			{Name: "N/A", Values: []string{""}},
			{Name: "CHA Count", Values: []string{chaCountFromOutput(outputs)}},
		}
	}
}

func elcTableValues(outputs map[string]script.ScriptOutput) []Field {
	return elcFieldValuesFromOutput(outputs)
}

func elcTableInsights(outputs map[string]script.ScriptOutput, tableValues TableValues) []Insight {
	insights := []Insight{}
	modeFieldIndex, err := getFieldIndex("Mode", tableValues)
	if err != nil {
		slog.Warn(err.Error())
	} else {
		// warn if ELC mode is not set to 'Latency Optimized' or 'Default' consistently across all dies
		firstMode := tableValues.Fields[modeFieldIndex].Values[0]
		for _, mode := range tableValues.Fields[modeFieldIndex].Values[1:] {
			if mode != firstMode {
				insights = append(insights, Insight{
					Recommendation: "Consider setting Efficiency Latency Control mode consistently across all dies.",
					Justification:  "ELC mode is not set consistently across all dies.",
				})
				break
			}
		}
		// suggest setting ELC mode to 'Latency Optimized' or 'Default' based on the current setting
		for _, mode := range tableValues.Fields[modeFieldIndex].Values {
			if mode != "" && mode != "Latency Optimized" {
				insights = append(insights, Insight{
					Recommendation: "Consider setting Efficiency Latency Control mode to 'Latency Optimized' when workload is highly sensitive to memory latency.",
					Justification:  fmt.Sprintf("ELC mode is set to '%s' on at least one die.", mode),
				})
				break
			}
		}
		for _, mode := range tableValues.Fields[modeFieldIndex].Values {
			if mode != "" && mode != "Default" {
				insights = append(insights, Insight{
					Recommendation: "Consider setting Efficiency Latency Control mode to 'Default' to balance uncore performance and power utilization.",
					Justification:  fmt.Sprintf("ELC mode is set to '%s' on at least one die.", mode),
				})
				break
			}
		}
		// if epb is not set to 'Performance (0)' and ELC mode is set to 'Latency Optimized', suggest setting epb to 'Performance (0)'
		epb := epbFromOutput(outputs)
		if epb != "" && epb != "Performance (0)" && firstMode == "Latency Optimized" {
			insights = append(insights, Insight{
				Recommendation: "Consider setting Energy Performance Bias to 'Performance (0)' to allow Latency Optimized mode to operate as designed.",
				Justification:  fmt.Sprintf("Energy Performance Bias is set to '%s' and ELC Mode is set to '%s'.", epb, firstMode),
			})
		}
	}
	return insights
}

func maximumFrequencyTableValues(outputs map[string]script.ScriptOutput) []Field {
	frequencyBuckets, err := getSpecFrequencyBuckets(outputs)
	if err != nil {
		slog.Warn("unable to get spec core frequencies", slog.String("error", err.Error()))
		return []Field{}
	}
	var fields []Field
	for i, row := range frequencyBuckets {
		// first row is field names
		if i == 0 {
			for _, fieldName := range row {
				fields = append(fields, Field{Name: fieldName})
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

func sstTFHPTableValues(outputs map[string]script.ScriptOutput) []Field {
	output := outputs[script.SSTTFHPScriptName].Stdout
	if len(output) == 0 {
		return []Field{}
	}
	lines := strings.Split(output, "\n")
	if len(lines) >= 1 && (strings.Contains(lines[0], "not supported") || strings.Contains(lines[0], "not enabled")) {
		return []Field{}
	}
	// lines should contain CSV formatted data
	fields := []Field{}
	for i, line := range lines {
		// field names are in the header
		if i == 0 {
			fieldNames := strings.Split(line, ",")
			for j, fieldName := range fieldNames {
				if j > 1 {
					fieldName = fieldName + " (MHz)"
				}
				fields = append(fields, Field{Name: fieldName})
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
				return []Field{}
			}
			if j > 1 {
				value = value + "00"
			}
			fields[j].Values = append(fields[j].Values, value)
		}
	}
	return fields
}

func sstTFLPTableValues(outputs map[string]script.ScriptOutput) []Field {
	output := outputs[script.SSTTFLPScriptName].Stdout
	if len(output) == 0 {
		return []Field{}
	}
	lines := strings.Split(output, "\n")
	if len(lines) >= 1 && (strings.Contains(lines[0], "not supported") || strings.Contains(lines[0], "not enabled")) {
		return []Field{}
	}
	// lines should contain CSV formatted data
	fields := []Field{}
	for i, line := range lines {
		// field names are in the header
		if i == 0 {
			for fieldName := range strings.SplitSeq(line, ",") {
				fields = append(fields, Field{Name: fieldName + " (MHz)"})
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
				return []Field{}
			}
			fields[j].Values = append(fields[j].Values, value+"00")
		}
	}
	return fields
}

func memoryTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "Installed Memory", Values: []string{installedMemoryFromOutput(outputs)}},
		{Name: "MemTotal", Values: []string{valFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^MemTotal:\s*(.+?)$`)}},
		{Name: "MemFree", Values: []string{valFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^MemFree:\s*(.+?)$`)}},
		{Name: "MemAvailable", Values: []string{valFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^MemAvailable:\s*(.+?)$`)}},
		{Name: "Buffers", Values: []string{valFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^Buffers:\s*(.+?)$`)}},
		{Name: "Cached", Values: []string{valFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^Cached:\s*(.+?)$`)}},
		{Name: "HugePages_Total", Values: []string{valFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^HugePages_Total:\s*(.+?)$`)}},
		{Name: "Hugepagesize", Values: []string{valFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^Hugepagesize:\s*(.+?)$`)}},
		{Name: "Transparent Huge Pages", Values: []string{valFromRegexSubmatch(outputs[script.TransparentHugePagesScriptName].Stdout, `.*\[(.*)\].*`)}},
		{Name: "Automatic NUMA Balancing", Values: []string{numaBalancingFromOutput(outputs)}},
		{Name: "Populated Memory Channels", Values: []string{populatedChannelsFromOutput(outputs)}},
		{Name: "Total Memory Encryption (TME)", Values: []string{strings.TrimSpace(outputs[script.TmeScriptName].Stdout)}},
		{Name: "Clustering Mode", Values: []string{clusteringModeFromOutput(outputs)}},
	}
}

func memoryTableInsights(outputs map[string]script.ScriptOutput, tableValues TableValues) []Insight {
	insights := []Insight{}
	// check if memory is not fully populated
	populatedChannelsIndex, err := getFieldIndex("Populated Memory Channels", tableValues)
	if err != nil {
		slog.Warn(err.Error())
	} else {
		populatedChannels := tableValues.Fields[populatedChannelsIndex].Values[0]
		if populatedChannels != "" {
			uarch := UarchFromOutput(outputs)
			if uarch != "" {
				cpu, err := GetCPUByMicroArchitecture(uarch)
				if err != nil {
					slog.Warn(err.Error())
				} else {
					sockets := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)
					socketCount, err := strconv.Atoi(sockets)
					if err != nil {
						slog.Warn(err.Error())
					} else {
						totalMemoryChannels := socketCount * cpu.MemoryChannelCount
						if populatedChannels != strconv.Itoa(totalMemoryChannels) {
							insights = append(insights, Insight{
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
	nodes := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node\(s\):\s*(.+)$`)
	nodeCount, err := strconv.Atoi(nodes)
	if err != nil {
		slog.Warn(err.Error())
	} else {
		if nodeCount > 1 {
			numaBalancingIndex, err := getFieldIndex("Automatic NUMA Balancing", tableValues)
			if err != nil {
				slog.Warn(err.Error())
			} else {
				numaBalancing := tableValues.Fields[numaBalancingIndex].Values[0]
				if numaBalancing != "" && numaBalancing != "Enabled" {
					insights = append(insights, Insight{
						Recommendation: "Consider enabling Automatic NUMA Balancing.",
						Justification:  "Automatic NUMA Balancing is not enabled.",
					})
				}
			}
		}
	}

	return insights
}

func dimmTableValues(outputs map[string]script.ScriptOutput) []Field {
	dimmFieldValues := valsArrayFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "17",
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
		return []Field{}
	}
	fields := []Field{
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

func dimmTableInsights(outputs map[string]script.ScriptOutput, tableValues TableValues) []Insight {
	insights := []Insight{}
	// check if are configured for their maximum speed
	SpeedIndex, err := getFieldIndex("Speed", tableValues)
	if err != nil {
		slog.Warn(err.Error())
	} else {
		ConfiguredSpeedIndex, err := getFieldIndex("Configured Speed", tableValues)
		if err != nil {
			slog.Warn(err.Error())
		} else {
			for i, speed := range tableValues.Fields[SpeedIndex].Values {
				configuredSpeed := tableValues.Fields[ConfiguredSpeedIndex].Values[i]
				if speed != "" && configuredSpeed != "" && speed != "Unknown" && configuredSpeed != "Unknown" {
					speedVal, err := strconv.Atoi(strings.Split(speed, " ")[0])
					if err != nil {
						slog.Warn(err.Error())
					} else {
						configuredSpeedVal, err := strconv.Atoi(strings.Split(configuredSpeed, " ")[0])
						if err != nil {
							slog.Warn(err.Error())
						} else {
							if speedVal < configuredSpeedVal {
								insights = append(insights, Insight{
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
	return insights
}

func nicTableValues(outputs map[string]script.ScriptOutput) []Field {
	allNicsInfo := parseNicInfo(outputs[script.NicInfoScriptName].Stdout)
	if len(allNicsInfo) == 0 {
		return []Field{}
	}
	fields := []Field{
		{Name: "Name"},
		{Name: "Vendor (ID)"},
		{Name: "Model (ID)"},
		{Name: "Speed"},
		{Name: "Link"},
		{Name: "Bus"},
		{Name: "Driver"},
		{Name: "Driver Version"},
		{Name: "Firmware Version"},
		{Name: "MAC Address"},
		{Name: "NUMA Node"},
		{Name: "IRQBalance"},
	}
	for _, nicInfo := range allNicsInfo {
		fields[0].Values = append(fields[0].Values, nicInfo.Name)
		fields[1].Values = append(fields[1].Values, nicInfo.Vendor)
		if nicInfo.VendorID != "" {
			fields[1].Values[len(fields[1].Values)-1] += fmt.Sprintf(" (%s)", nicInfo.VendorID)
		}
		fields[2].Values = append(fields[2].Values, nicInfo.Model)
		if nicInfo.ModelID != "" {
			fields[2].Values[len(fields[2].Values)-1] += fmt.Sprintf(" (%s)", nicInfo.ModelID)
		}
		fields[3].Values = append(fields[3].Values, nicInfo.Speed)
		fields[4].Values = append(fields[4].Values, nicInfo.Link)
		fields[5].Values = append(fields[5].Values, nicInfo.Bus)
		fields[6].Values = append(fields[6].Values, nicInfo.Driver)
		fields[7].Values = append(fields[7].Values, nicInfo.DriverVersion)
		fields[8].Values = append(fields[8].Values, nicInfo.FirmwareVersion)
		fields[9].Values = append(fields[9].Values, nicInfo.MACAddress)
		fields[10].Values = append(fields[10].Values, nicInfo.NUMANode)
		fields[11].Values = append(fields[11].Values, nicInfo.IRQBalance)
	}
	return fields
}

func networkIRQMappingTableValues(outputs map[string]script.ScriptOutput) []Field {
	nicIRQMappings := nicIRQMappingsFromOutput(outputs)
	if len(nicIRQMappings) == 0 {
		return []Field{}
	}
	fields := []Field{
		{Name: "Interface"},
		{Name: "IRQ:CPU | IRQ:CPU | ..."},
	}
	for _, nicIRQMapping := range nicIRQMappings {
		fields[0].Values = append(fields[0].Values, nicIRQMapping[0])
		fields[1].Values = append(fields[1].Values, nicIRQMapping[1])
	}
	return fields
}

func networkConfigTableValues(outputs map[string]script.ScriptOutput) []Field {
	// these are the fields we want to display
	fields := []Field{
		{Name: "net.ipv4.tcp_rmem"},
		{Name: "net.ipv4.tcp_wmem"},
		{Name: "net.core.rmem_max"},
		{Name: "net.core.wmem_max"},
		{Name: "net.core.netdev_max_backlog"},
		{Name: "net.ipv4.tcp_max_syn_backlog"},
		{Name: "net.core.somaxconn"},
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
	for i := range fields {
		if val, ok := sysctlParams[fields[i].Name]; ok {
			fields[i].Values = append(fields[i].Values, val)
		} else {
			fields[i].Values = append(fields[i].Values, "")
		}
	}
	return fields
}

func diskTableValues(outputs map[string]script.ScriptOutput) []Field {
	allDisksInfo := diskInfoFromOutput(outputs)
	if len(allDisksInfo) == 0 {
		return []Field{}
	}
	fields := []Field{
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

func filesystemTableValues(outputs map[string]script.ScriptOutput) []Field {
	return filesystemFieldValuesFromOutput(outputs)
}

func filesystemTableInsights(outputs map[string]script.ScriptOutput, tableValues TableValues) []Insight {
	insights := []Insight{}
	mountOptionsIndex, err := getFieldIndex("Mount Options", tableValues)
	if err != nil {
		slog.Warn(err.Error())
	} else {
		for i, options := range tableValues.Fields[mountOptionsIndex].Values {
			if strings.Contains(options, "discard") {
				insights = append(insights, Insight{
					Recommendation: fmt.Sprintf("Consider mounting the '%s' file system without the 'discard' option and instead configure periodic TRIM for SSDs, if used for I/O intensive workloads.", tableValues.Fields[0].Values[i]),
					Justification:  fmt.Sprintf("The '%s' filesystem is mounted with 'discard' option.", tableValues.Fields[0].Values[i]),
				})
			}
		}
	}
	return insights
}

func gpuTableValues(outputs map[string]script.ScriptOutput) []Field {
	gpuInfos := gpuInfoFromOutput(outputs)
	if len(gpuInfos) == 0 {
		return []Field{}
	}
	fields := []Field{
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

func gaudiTableValues(outputs map[string]script.ScriptOutput) []Field {
	gaudiInfos := gaudiInfoFromOutput(outputs)
	if len(gaudiInfos) == 0 {
		return []Field{}
	}
	fields := []Field{
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

func cxlTableValues(outputs map[string]script.ScriptOutput) []Field {
	cxlDevices := getPCIDevices("CXL", outputs)
	if len(cxlDevices) == 0 {
		return []Field{}
	}
	fields := []Field{
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

func cveTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{}
	cves := cveInfoFromOutput(outputs)
	for _, cve := range cves {
		fields = append(fields, Field{Name: cve[0], Values: []string{cve[1]}})
	}
	return fields
}

func cveTableInsights(outputs map[string]script.ScriptOutput, tableValues TableValues) []Insight {
	insights := []Insight{}
	for _, field := range tableValues.Fields {
		if strings.HasPrefix(field.Values[0], "VULN") {
			insights = append(insights, Insight{
				Recommendation: fmt.Sprintf("Consider applying the security patch for %s.", field.Name),
				Justification:  fmt.Sprintf("The system is vulnerable to %s.", field.Name),
			})
		}
	}
	return insights
}

func processTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{}
	for i, line := range strings.Split(outputs[script.ProcessListScriptName].Stdout, "\n") {
		tokens := strings.Fields(line)
		if i == 0 { // header -- defines fields in table
			for _, token := range tokens {
				fields = append(fields, Field{Name: token})
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

func sensorTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
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
		return []Field{}
	}
	return fields
}

func chassisStatusTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{}
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
		fields = append(fields, Field{Name: fieldName, Values: []string{fieldValue}})
	}
	return fields
}

func systemEventLogTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
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
		return []Field{}
	}
	return fields
}

func systemEventLogTableInsights(outputs map[string]script.ScriptOutput, tableValues TableValues) []Insight {
	insights := []Insight{}
	sensorFieldIndex, err := getFieldIndex("Sensor", tableValues)
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
			insights = append(insights, Insight{
				Recommendation: "Consider reviewing the System Event Log table.",
				Justification:  fmt.Sprintf("Detected '%d' temperature-related service action(s) in the System Event Log.", temperatureEvents),
			})
		}
	}
	return insights
}

func kernelLogTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "Entries", Values: strings.Split(outputs[script.KernelLogScriptName].Stdout, "\n")},
	}
}

func pmuTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "PMU Driver Version", Values: []string{strings.TrimSpace(outputs[script.PMUDriverVersionScriptName].Stdout)}},
		{Name: "cpu_cycles", Values: []string{valFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0x30a (.*)$`)}},
		{Name: "instructions", Values: []string{valFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0x309 (.*)$`)}},
		{Name: "ref_cycles", Values: []string{valFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0x30b (.*)$`)}},
		{Name: "topdown_slots", Values: []string{valFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0x30c (.*)$`)}},
		{Name: "gen_programmable_1", Values: []string{valFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc1 (.*)$`)}},
		{Name: "gen_programmable_2", Values: []string{valFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc2 (.*)$`)}},
		{Name: "gen_programmable_3", Values: []string{valFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc3 (.*)$`)}},
		{Name: "gen_programmable_4", Values: []string{valFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc4 (.*)$`)}},
		{Name: "gen_programmable_5", Values: []string{valFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc5 (.*)$`)}},
		{Name: "gen_programmable_6", Values: []string{valFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc6 (.*)$`)}},
		{Name: "gen_programmable_7", Values: []string{valFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc7 (.*)$`)}},
		{Name: "gen_programmable_8", Values: []string{valFromRegexSubmatch(outputs[script.PMUBusyScriptName].Stdout, `^0xc8 (.*)$`)}},
	}
}

func systemSummaryTableValues(outputs map[string]script.ScriptOutput) []Field {
	system := valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Manufacturer:\s*(.+?)$`) +
		" " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Product Name:\s*(.+?)$`) +
		", " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Version:\s*(.+?)$`)
	baseboard := valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Manufacturer:\s*(.+?)$`) +
		" " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Product Name:\s*(.+?)$`) +
		", " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Version:\s*(.+?)$`)
	chassis := valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Manufacturer:\s*(.+?)$`) +
		" " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Type:\s*(.+?)$`) +
		", " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Version:\s*(.+?)$`)

	return []Field{
		{Name: "Host Name", Values: []string{strings.TrimSpace(outputs[script.HostnameScriptName].Stdout)}},
		{Name: "Time", Values: []string{strings.TrimSpace(outputs[script.DateScriptName].Stdout)}},
		{Name: "System", Values: []string{system}},
		{Name: "Baseboard", Values: []string{baseboard}},
		{Name: "Chassis", Values: []string{chassis}},
		{Name: "CPU Model", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^[Mm]odel name:\s*(.+)$`)}},
		{Name: "Architecture", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Architecture:\s*(.+)$`)}},
		{Name: "Microarchitecture", Values: []string{UarchFromOutput(outputs)}},
		{Name: "L3 Cache", Values: []string{l3FromOutput(outputs)}},
		{Name: "Cores per Socket", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(.+)$`)}},
		{Name: "Sockets", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)}},
		{Name: "Hyperthreading", Values: []string{hyperthreadingFromOutput(outputs)}},
		{Name: "CPUs", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU\(s\):\s*(.+)$`)}},
		{Name: "Intel Turbo Boost", Values: []string{turboEnabledFromOutput(outputs)}},
		{Name: "Base Frequency", Values: []string{baseFrequencyFromOutput(outputs)}},
		{Name: "All-core Maximum Frequency", Values: []string{allCoreMaxFrequencyFromOutput(outputs)}},
		{Name: "Maximum Frequency", Values: []string{maxFrequencyFromOutput(outputs)}},
		{Name: "NUMA Nodes", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node\(s\):\s*(.+)$`)}},
		{Name: "Prefetchers", Values: []string{prefetchersSummaryFromOutput(outputs)}},
		{Name: "PPINs", Values: []string{ppinsFromOutput(outputs)}},
		{Name: "Accelerators Available [used]", Values: []string{acceleratorSummaryFromOutput(outputs)}},
		{Name: "Installed Memory", Values: []string{installedMemoryFromOutput(outputs)}},
		{Name: "Hugepagesize", Values: []string{valFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^Hugepagesize:\s*(.+?)$`)}},
		{Name: "Transparent Huge Pages", Values: []string{valFromRegexSubmatch(outputs[script.TransparentHugePagesScriptName].Stdout, `.*\[(.*)\].*`)}},
		{Name: "Automatic NUMA Balancing", Values: []string{numaBalancingFromOutput(outputs)}},
		{Name: "NIC", Values: []string{nicSummaryFromOutput(outputs)}},
		{Name: "Disk", Values: []string{diskSummaryFromOutput(outputs)}},
		{Name: "BIOS", Values: []string{valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "0", `^Version:\s*(.+?)$`)}},
		{Name: "Microcode", Values: []string{valFromRegexSubmatch(outputs[script.ProcCpuinfoScriptName].Stdout, `^microcode.*:\s*(.+?)$`)}},
		{Name: "OS", Values: []string{operatingSystemFromOutput(outputs)}},
		{Name: "Kernel", Values: []string{valFromRegexSubmatch(outputs[script.UnameScriptName].Stdout, `^Linux \S+ (\S+)`)}},
		{Name: "TDP", Values: []string{tdpFromOutput(outputs)}},
		{Name: "Energy Performance Bias", Values: []string{epbFromOutput(outputs)}},
		{Name: "Scaling Governor", Values: []string{strings.TrimSpace(outputs[script.ScalingGovernorScriptName].Stdout)}},
		{Name: "Scaling Driver", Values: []string{strings.TrimSpace(outputs[script.ScalingDriverScriptName].Stdout)}},
		{Name: "C-states", Values: []string{cstatesSummaryFromOutput(outputs)}},
		{Name: "Efficiency Latency Control", Values: []string{elcSummaryFromOutput(outputs)}},
		{Name: "CVEs", Values: []string{cveSummaryFromOutput(outputs)}},
		{Name: "System Summary", Values: []string{systemSummaryFromOutput(outputs)}},
	}
}

func briefSummaryTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "Host Name", Values: []string{strings.TrimSpace(outputs[script.HostnameScriptName].Stdout)}},                                          // Hostname
		{Name: "Time", Values: []string{strings.TrimSpace(outputs[script.DateScriptName].Stdout)}},                                                   // Date
		{Name: "CPU Model", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^[Mm]odel name:\s*(.+)$`)}},               // Lscpu
		{Name: "Microarchitecture", Values: []string{UarchFromOutput(outputs)}},                                                                      // Lscpu, LspciBits, LspciDevices
		{Name: "TDP", Values: []string{tdpFromOutput(outputs)}},                                                                                      // PackagePowerLimit
		{Name: "Sockets", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)}},                   // Lscpu
		{Name: "Cores per Socket", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(.+)$`)}}, // Lscpu
		{Name: "Hyperthreading", Values: []string{hyperthreadingFromOutput(outputs)}},                                                                // Lscpu, LspciBits, LspciDevices
		{Name: "CPUs", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU\(s\):\s*(.+)$`)}},                         // Lscpu
		{Name: "NUMA Nodes", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node\(s\):\s*(.+)$`)}},             // Lscpu
		{Name: "Scaling Driver", Values: []string{strings.TrimSpace(outputs[script.ScalingDriverScriptName].Stdout)}},                                // ScalingDriver
		{Name: "Scaling Governor", Values: []string{strings.TrimSpace(outputs[script.ScalingGovernorScriptName].Stdout)}},                            // ScalingGovernor
		{Name: "C-states", Values: []string{cstatesSummaryFromOutput(outputs)}},                                                                      // Cstates
		{Name: "Maximum Frequency", Values: []string{maxFrequencyFromOutput(outputs)}},                                                               // MaximumFrequency, SpecCoreFrequencies,
		{Name: "All-core Maximum Frequency", Values: []string{allCoreMaxFrequencyFromOutput(outputs)}},                                               // Lscpu, LspciBits, LspciDevices, SpecCoreFrequencies
		{Name: "Energy Performance Bias", Values: []string{epbFromOutput(outputs)}},                                                                  // EpbSource, EpbBIOS, EpbOS
		{Name: "Efficiency Latency Control", Values: []string{elcSummaryFromOutput(outputs)}},                                                        // Elc
		{Name: "MemTotal", Values: []string{valFromRegexSubmatch(outputs[script.MeminfoScriptName].Stdout, `^MemTotal:\s*(.+?)$`)}},                  // Meminfo
		{Name: "NIC", Values: []string{nicSummaryFromOutput(outputs)}},                                                                               // Lshw, NicInfo
		{Name: "Disk", Values: []string{diskSummaryFromOutput(outputs)}},                                                                             // DiskInfo, Hdparm
		{Name: "OS", Values: []string{operatingSystemFromOutput(outputs)}},                                                                           // EtcRelease
		{Name: "Kernel", Values: []string{valFromRegexSubmatch(outputs[script.UnameScriptName].Stdout, `^Linux \S+ (\S+)`)}},                         // Uname
	}
}

func configurationTableValues(outputs map[string]script.ScriptOutput) []Field {
	uarch := UarchFromOutput(outputs)
	if uarch == "" {
		slog.Error("failed to get uarch from script outputs")
		return []Field{}
	}

	fields := []Field{
		{Name: "Cores per Socket", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(.+)$`)}},
		{Name: "L3 Cache", Values: []string{l3FromOutput(outputs)}},
		{Name: "Package Power / TDP", Values: []string{tdpFromOutput(outputs)}},
		{Name: "All-Core Max Frequency", Values: []string{allCoreMaxFrequencyFromOutput(outputs)}},
	}
	if strings.Contains(uarch, "SRF") || strings.Contains(uarch, "GNR") || strings.Contains(uarch, "CWF") {
		fields = append(fields, []Field{
			{Name: "Uncore Min Frequency (Compute)", Values: []string{uncoreMinMaxDieFrequencyFromOutput(false, true, outputs)}},
			{Name: "Uncore Min Frequency (I/O)", Values: []string{uncoreMinMaxDieFrequencyFromOutput(false, false, outputs)}},
			{Name: "Uncore Max Frequency (Compute)", Values: []string{uncoreMinMaxDieFrequencyFromOutput(true, true, outputs)}},
			{Name: "Uncore Max Frequency (I/O)", Values: []string{uncoreMinMaxDieFrequencyFromOutput(true, false, outputs)}},
		}...)
	} else {
		fields = append(fields, []Field{
			{Name: "Uncore Max Frequency (GHz)", Values: []string{uncoreMaxFrequencyFromOutput(outputs)}},
			{Name: "Uncore Min Frequency (GHz)", Values: []string{uncoreMinFrequencyFromOutput(outputs)}},
		}...)
	}
	fields = append(fields, []Field{
		{Name: "Energy Performance Bias", Values: []string{epbFromOutput(outputs)}},
		{Name: "Energy Performance Preference", Values: []string{eppFromOutput(outputs)}},
		{Name: "Scaling Governor", Values: []string{strings.TrimSpace(outputs[script.ScalingGovernorScriptName].Stdout)}},
	}...)
	// add ELC (for SRF, CWF and GNR only)
	if strings.Contains(uarch, "SRF") || strings.Contains(uarch, "GNR") || strings.Contains(uarch, "CWF") {
		fields = append(fields, Field{Name: "Efficiency Latency Control", Values: []string{elcSummaryFromOutput(outputs)}})
	}
	// add prefetchers
	for _, pf := range prefetcherDefinitions {
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
			msrVal := valFromRegexSubmatch(outputs[scriptName].Stdout, `^([0-9a-fA-F]+)`)
			var enabledDisabled string
			enabled, err := isPrefetcherEnabled(msrVal, pf.Bit)
			if err != nil {
				slog.Warn("error checking prefetcher enabled status", slog.String("error", err.Error()))
				continue
			}
			if enabled {
				enabledDisabled = "Enabled"
			} else {
				enabledDisabled = "Disabled"
			}
			fields = append(fields, Field{Name: pf.ShortName + " prefetcher", Values: []string{enabledDisabled}})
		}
	}
	// add C-states
	cstates := cstatesSummaryFromOutput(outputs)
	if cstates != "" {
		fields = append(fields, Field{Name: "C-states", Values: []string{cstates}})
	}
	// add C1 Demotion
	c1Demotion := strings.TrimSpace(outputs[script.C1DemotionScriptName].Stdout)
	if c1Demotion != "" {
		fields = append(fields, Field{Name: "C1 Demotion", Values: []string{c1Demotion}})
	}
	return fields
}

// benchmarking

func speedBenchmarkTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "Ops/s", Values: []string{cpuSpeedFromOutput(outputs)}},
	}
}

func powerBenchmarkTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "Maximum Power", Values: []string{maxTotalPackagePowerFromOutput(outputs[script.PowerBenchmarkScriptName].Stdout)}},
		{Name: "Minimum Power", Values: []string{minTotalPackagePowerFromOutput(outputs[script.IdlePowerBenchmarkScriptName].Stdout)}},
	}
}

func temperatureBenchmarkTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "Maximum Temperature", Values: []string{maxPackageTemperatureFromOutput(outputs[script.PowerBenchmarkScriptName].Stdout)}},
	}
}

func frequencyBenchmarkTableValues(outputs map[string]script.ScriptOutput) []Field {
	// get the sse, avx256, and avx512 frequencies from the avx-turbo output
	instructionFreqs, err := avxTurboFrequenciesFromOutput(outputs[script.FrequencyBenchmarkScriptName].Stdout)
	if err != nil {
		slog.Error("unable to get avx turbo frequencies", slog.String("error", err.Error()))
		return []Field{}
	}
	// we're expecting scalar_iadd, avx256_fma, avx512_fma
	scalarIaddFreqs := instructionFreqs["scalar_iadd"]
	avx256FmaFreqs := instructionFreqs["avx256_fma"]
	avx512FmaFreqs := instructionFreqs["avx512_fma"]
	// stop if we don't have any scalar_iadd frequencies
	if len(scalarIaddFreqs) == 0 {
		slog.Error("no scalar_iadd frequencies found")
		return []Field{}
	}
	// get the spec core frequencies from the spec output
	var specSSEFreqs []string
	frequencyBuckets, err := getSpecFrequencyBuckets(outputs)
	if err == nil && len(frequencyBuckets) >= 2 {
		// get the frequencies from the buckets
		specSSEFreqs, err = expandTurboFrequencies(frequencyBuckets, "sse")
		if err != nil {
			slog.Error("unable to convert buckets to counts", slog.String("error", err.Error()))
			return []Field{}
		}
		// trim the spec frequencies to the length of the scalar_iadd frequencies
		// this can happen when the actual core count is less than the number of cores in the spec
		if len(scalarIaddFreqs) < len(specSSEFreqs) {
			specSSEFreqs = specSSEFreqs[:len(scalarIaddFreqs)]
		}
		// pad the spec frequencies with the last value if they are shorter than the scalar_iadd frequencies
		// this can happen when the first die has fewer cores than other dies
		if len(specSSEFreqs) < len(scalarIaddFreqs) {
			diff := len(scalarIaddFreqs) - len(specSSEFreqs)
			for range diff {
				specSSEFreqs = append(specSSEFreqs, specSSEFreqs[len(specSSEFreqs)-1])
			}
		}
	}
	// create the fields
	fields := []Field{
		{Name: "cores"},
	}
	coresIdx := 0 // always the first field
	var specSSEFieldIdx int
	var scalarIaddFieldIdx int
	var avx2FieldIdx int
	var avx512FieldIdx int
	if len(specSSEFreqs) > 0 {
		fields = append(fields, Field{Name: "SSE (expected)"})
		specSSEFieldIdx = len(fields) - 1
	}
	if len(scalarIaddFreqs) > 0 {
		fields = append(fields, Field{Name: "SSE"})
		scalarIaddFieldIdx = len(fields) - 1
	}
	if len(avx256FmaFreqs) > 0 {
		fields = append(fields, Field{Name: "AVX2"})
		avx2FieldIdx = len(fields) - 1
	}
	if len(avx512FmaFreqs) > 0 {
		fields = append(fields, Field{Name: "AVX512"})
		avx512FieldIdx = len(fields) - 1
	}
	// add the data to the fields
	for i := range scalarIaddFreqs { // scalarIaddFreqs is required
		fields[coresIdx].Values = append(fields[coresIdx].Values, fmt.Sprintf("%d", i+1))
		if specSSEFieldIdx > 0 {
			if len(specSSEFreqs) > i {
				fields[specSSEFieldIdx].Values = append(fields[specSSEFieldIdx].Values, specSSEFreqs[i])
			} else {
				fields[specSSEFieldIdx].Values = append(fields[specSSEFieldIdx].Values, "")
			}
		}
		if scalarIaddFieldIdx > 0 {
			if len(scalarIaddFreqs) > i {
				fields[scalarIaddFieldIdx].Values = append(fields[scalarIaddFieldIdx].Values, fmt.Sprintf("%.1f", scalarIaddFreqs[i]))
			} else {
				fields[scalarIaddFieldIdx].Values = append(fields[scalarIaddFieldIdx].Values, "")
			}
		}
		if avx2FieldIdx > 0 {
			if len(avx256FmaFreqs) > i {
				fields[avx2FieldIdx].Values = append(fields[avx2FieldIdx].Values, fmt.Sprintf("%.1f", avx256FmaFreqs[i]))
			} else {
				fields[avx2FieldIdx].Values = append(fields[avx2FieldIdx].Values, "")
			}
		}
		if avx512FieldIdx > 0 {
			if len(avx512FmaFreqs) > i {
				fields[avx512FieldIdx].Values = append(fields[avx512FieldIdx].Values, fmt.Sprintf("%.1f", avx512FmaFreqs[i]))
			} else {
				fields[avx512FieldIdx].Values = append(fields[avx512FieldIdx].Values, "")
			}
		}
	}
	return fields
}

func memoryBenchmarkTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Latency (ns)"},
		{Name: "Bandwidth (GB/s)"},
	}
	/* MLC Output:
	Inject	Latency	Bandwidth
	Delay	(ns)	MB/sec
	==========================
	 00000	261.65	 225060.9
	 00002	261.63	 225040.5
	 00008	261.54	 225073.3
	 ...
	*/
	latencyBandwidthPairs := valsArrayFromRegexSubmatch(outputs[script.MemoryBenchmarkScriptName].Stdout, `\s*[0-9]*\s*([0-9]*\.[0-9]+)\s*([0-9]*\.[0-9]+)`)
	for _, latencyBandwidth := range latencyBandwidthPairs {
		latency := latencyBandwidth[0]
		bandwidth, err := strconv.ParseFloat(latencyBandwidth[1], 32)
		if err != nil {
			slog.Error(fmt.Sprintf("Unable to convert bandwidth to float: %s", latencyBandwidth[1]))
			continue
		}
		// insert into beginning of list
		fields[0].Values = append([]string{latency}, fields[0].Values...)
		fields[1].Values = append([]string{fmt.Sprintf("%.1f", bandwidth/1000)}, fields[1].Values...)
	}
	if len(fields[0].Values) == 0 {
		return []Field{}
	}
	return fields
}

func numaBenchmarkTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Node"},
	}
	/* MLC Output:
			Numa node
	Numa node	     0	     1
	       0	175610.3	 55579.7
	       1	 55575.2	175656.7
	*/
	nodeBandwidthsPairs := valsArrayFromRegexSubmatch(outputs[script.NumaBenchmarkScriptName].Stdout, `^\s+(\d)\s+(\d.*)$`)
	// add 1 field per numa node
	for _, nodeBandwidthsPair := range nodeBandwidthsPairs {
		fields = append(fields, Field{Name: nodeBandwidthsPair[0]})
	}
	// add rows
	for _, nodeBandwidthsPair := range nodeBandwidthsPairs {
		fields[0].Values = append(fields[0].Values, nodeBandwidthsPair[0])
		bandwidths := strings.Split(strings.TrimSpace(nodeBandwidthsPair[1]), "\t")
		if len(bandwidths) != len(nodeBandwidthsPairs) {
			slog.Warn(fmt.Sprintf("Mismatched number of bandwidths for numa node %s, %s", nodeBandwidthsPair[0], nodeBandwidthsPair[1]))
			return []Field{}
		}
		for i, bw := range bandwidths {
			bw = strings.TrimSpace(bw)
			val, err := strconv.ParseFloat(bw, 64)
			if err != nil {
				slog.Error(fmt.Sprintf("Unable to convert bandwidth to float: %s", bw))
				continue
			}
			fields[i+1].Values = append(fields[i+1].Values, fmt.Sprintf("%.1f", val/1000))
		}
	}
	if len(fields[0].Values) == 0 {
		return []Field{}
	}
	return fields
}

func storageBenchmarkTableValues(outputs map[string]script.ScriptOutput) []Field {
	readBW, writeBW := storagePerfFromOutput(outputs)
	if readBW == "" && writeBW == "" {
		return []Field{}
	}
	return []Field{
		{Name: "Single-Thread Read Bandwidth", Values: []string{readBW}},
		{Name: "Single-Thread Write Bandwidth", Values: []string{writeBW}},
	}
}

// telemetry

func cpuUtilizationTelemetryTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Time"},
		{Name: "CPU"},
		{Name: "CORE"},
		{Name: "SOCK"},
		{Name: "NODE"},
		{Name: "%usr"},
		{Name: "%nice"},
		{Name: "%sys"},
		{Name: "%iowait"},
		{Name: "%irq"},
		{Name: "%soft"},
		{Name: "%steal"},
		{Name: "%guest"},
		{Name: "%gnice"},
		{Name: "%idle"},
	}
	reStat := regexp.MustCompile(`^(\d\d:\d\d:\d\d)\s+(\d+)\s+(\d+)\s+(\d+)\s+(-*\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)$`)
	for line := range strings.SplitSeq(outputs[script.MpstatTelemetryScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	if len(fields[0].Values) == 0 {
		return []Field{}
	}
	return fields
}

func utilizationCategoriesTelemetryTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Time"},
		{Name: "%usr"},
		{Name: "%nice"},
		{Name: "%sys"},
		{Name: "%iowait"},
		{Name: "%irq"},
		{Name: "%soft"},
		{Name: "%steal"},
		{Name: "%guest"},
		{Name: "%gnice"},
		{Name: "%idle"},
	}
	reStat := regexp.MustCompile(`^(\d\d:\d\d:\d\d)\s+all\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)$`)
	for line := range strings.SplitSeq(outputs[script.MpstatTelemetryScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	if len(fields[0].Values) == 0 {
		return []Field{}
	}
	return fields
}

func irqRateTelemetryTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Time"},
		{Name: "CPU"},
		{Name: "HI/s"},
		{Name: "TIMER/s"},
		{Name: "NET_TX/s"},
		{Name: "NET_RX/s"},
		{Name: "BLOCK/s"},
		{Name: "IRQ_POLL/s"},
		{Name: "TASKLET/s"},
		{Name: "SCHED/s"},
		{Name: "HRTIMER/s"},
		{Name: "RCU/s"},
	}
	reStat := regexp.MustCompile(`^(\d\d:\d\d:\d\d)\s+(\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)$`)
	for line := range strings.SplitSeq(outputs[script.MpstatTelemetryScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	if len(fields[0].Values) == 0 {
		return []Field{}
	}
	return fields
}

func driveTelemetryTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Time"},
		{Name: "Device"},
		{Name: "tps"},
		{Name: "kB_read/s"},
		{Name: "kB_wrtn/s"},
		{Name: "kB_dscd/s"},
	}
	// the time is on its own line, so we need to keep track of it
	reTime := regexp.MustCompile(`^\d\d\d\d-\d\d-\d\dT(\d\d:\d\d:\d\d)`)
	// don't capture the last three vals: "kB_read","kB_wrtn","kB_dscd" -- they aren't the same scale as the others
	reStat := regexp.MustCompile(`^(\w+)\s*(\d+.\d+)\s*(\d+.\d+)\s*(\d+.\d+)\s*(\d+.\d+)\s*\d+\s*\d+\s*\d+$`)
	var time string
	for line := range strings.SplitSeq(outputs[script.IostatTelemetryScriptName].Stdout, "\n") {
		match := reTime.FindStringSubmatch(line)
		if len(match) > 0 {
			time = match[1]
			continue
		}
		match = reStat.FindStringSubmatch(line)
		if len(match) > 0 {
			fields[0].Values = append(fields[0].Values, time)
			for i := range fields[1:] {
				fields[i+1].Values = append(fields[i+1].Values, match[i+1])
			}
		}
	}
	if len(fields[0].Values) == 0 {
		return []Field{}
	}
	return fields
}

func networkTelemetryTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Time"},
		{Name: "IFACE"},
		{Name: "rxpck/s"},
		{Name: "txpck/s"},
		{Name: "rxkB/s"},
		{Name: "txkB/s"},
	}
	// don't capture the last four vals: "rxcmp/s","txcmp/s","rxcmt/s","%ifutil" -- obscure more important vals
	reStat := regexp.MustCompile(`^(\d+:\d+:\d+)\s*(\w*)\s*(\d+.\d+)\s*(\d+.\d+)\s*(\d+.\d+)\s*(\d+.\d+)\s*\d+.\d+\s*\d+.\d+\s*\d+.\d+\s*\d+.\d+$`)
	for line := range strings.SplitSeq(outputs[script.NetworkTelemetryScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	if len(fields[0].Values) == 0 {
		return []Field{}
	}
	return fields
}

func memoryTelemetryTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Time"},
		{Name: "free"},
		{Name: "avail"},
		{Name: "used"},
		{Name: "buffers"},
		{Name: "cache"},
		{Name: "commit"},
		{Name: "active"},
		{Name: "inactive"},
		{Name: "dirty"},
	}
	reStat := regexp.MustCompile(`^(\d+:\d+:\d+)\s*(\d+)\s*(\d+)\s*(\d+)\s*\d+\.\d+\s*(\d+)\s*(\d+)\s*(\d+)\s*\d+\.\d+\s*(\d+)\s*(\d+)\s*(\d+)$`)
	for line := range strings.SplitSeq(outputs[script.MemoryTelemetryScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	if len(fields[0].Values) == 0 {
		return []Field{}
	}
	return fields
}

func powerTelemetryTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Time"},
	}
	packageRows, err := turbostatPackageRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"PkgWatt", "RAMWatt"})
	if err != nil {
		slog.Error(err.Error())
		return []Field{}
	}
	for i := range packageRows {
		fields = append(fields, Field{Name: fmt.Sprintf("Package %d", i)})
		fields = append(fields, Field{Name: fmt.Sprintf("DRAM %d", i)})
	}
	// for each package
	numPackages := len(packageRows)
	for i := range packageRows {
		// traverse the rows
		for _, row := range packageRows[i] {
			if i == 0 {
				fields[0].Values = append(fields[0].Values, row[0]) // Timestamp
			}
			// append the package power and DRAM power to the fields
			fields[i*numPackages+1].Values = append(fields[i*numPackages+1].Values, row[1]) // Package power
			fields[i*numPackages+2].Values = append(fields[i*numPackages+2].Values, row[2]) // DRAM power
		}
	}
	if len(fields[0].Values) == 0 {
		return []Field{}
	}
	return fields
}

func temperatureTelemetryTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Time"},
		{Name: "Core (Avg.)"},
	}
	platformRows, err := turbostatPlatformRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"CoreTmp"})
	if err != nil {
		slog.Error(err.Error())
		return []Field{}
	}
	packageRows, err := turbostatPackageRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"PkgTmp"})
	if err != nil {
		// not an error, just means no package rows (package temperature)
		slog.Warn(err.Error())
	}
	// add the package rows to the fields
	for i := range packageRows {
		fields = append(fields, Field{Name: fmt.Sprintf("Package %d", i)})
	}
	// for each platform row
	for i := range platformRows {
		// append the timestamp to the fields
		fields[0].Values = append(fields[0].Values, platformRows[i][0]) // Timestamp
		// append the core temperature values to the fields
		fields[1].Values = append(fields[1].Values, platformRows[i][1]) // Core temperature
	}
	// for each package
	for i := range packageRows {
		// traverse the rows
		for _, row := range packageRows[i] {
			// append the package temperature to the fields
			fields[i+2].Values = append(fields[i+2].Values, row[1]) // Package temperature
		}
	}
	if len(fields[0].Values) == 0 {
		return []Field{}
	}
	return fields
}

func frequencyTelemetryTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Time"},
		{Name: "Core (Avg.)"},
	}
	platformRows, err := turbostatPlatformRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"Bzy_MHz"})
	if err != nil {
		slog.Error(err.Error())
		return []Field{}
	}
	packageRows, err := turbostatPackageRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"UncMHz"})
	if err != nil {
		// not an error, just means no package rows (uncore frequency)
		slog.Warn(err.Error())
	}
	// add the package rows to the fields
	for i := range packageRows {
		fields = append(fields, Field{Name: fmt.Sprintf("Uncore Package %d", i)})
	}
	// for each platform row
	for i := range platformRows {
		// append the timestamp to the fields
		fields[0].Values = append(fields[0].Values, platformRows[i][0]) // Timestamp
		// append the core frequency values to the fields
		fields[1].Values = append(fields[1].Values, platformRows[i][1]) // Core frequency
	}
	// for each package
	for i := range packageRows {
		// traverse the rows
		for _, row := range packageRows[i] {
			// append the package frequency to the fields
			fields[i+2].Values = append(fields[i+2].Values, row[1]) // Package frequency
		}
	}
	if len(fields[0].Values) == 0 {
		return []Field{}
	}
	return fields
}

func ipcTelemetryTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Time"},
		{Name: "Core (Avg.)"},
	}
	platformRows, err := turbostatPlatformRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"IPC"})
	if err != nil {
		slog.Error(err.Error())
		return []Field{}
	}
	if len(platformRows) == 0 {
		slog.Warn("no platform rows found in turbostat telemetry output")
		return []Field{}
	}
	// for each platform row
	for i := range platformRows {
		// append the timestamp to the fields
		fields[0].Values = append(fields[0].Values, platformRows[i][0]) // Timestamp
		// append the core IPC values to the fields
		fields[1].Values = append(fields[1].Values, platformRows[i][1]) // Core IPC
	}
	return fields
}

func c6TelemetryTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Time"},
		{Name: "Package (Avg.)"},
		{Name: "Core (Avg.)"},
	}
	platformRows, err := turbostatPlatformRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"C6%", "CPU%c6"})
	if err != nil {
		slog.Error(err.Error())
		return []Field{}
	}
	if len(platformRows) == 0 {
		slog.Warn("no platform rows found in turbostat telemetry output")
		return []Field{}
	}
	// for each platform row
	for i := range platformRows {
		// append the timestamp to the fields
		fields[0].Values = append(fields[0].Values, platformRows[i][0]) // Timestamp
		// append the C6 residency values to the fields
		fields[1].Values = append(fields[1].Values, platformRows[i][1]) // C6%
		// append the CPU C6 residency values to the fields
		fields[2].Values = append(fields[2].Values, platformRows[i][2]) // CPU%c6
	}
	return fields
}

func gaudiTelemetryTableValues(outputs map[string]script.ScriptOutput) []Field {
	// parse the CSV output
	csvOutput := outputs[script.GaudiTelemetryScriptName].Stdout
	if csvOutput == "" {
		return []Field{}
	}
	r := csv.NewReader(strings.NewReader(csvOutput))
	rows, err := r.ReadAll()
	if err != nil {
		slog.Error(err.Error())
		return []Field{}
	}
	if len(rows) < 2 {
		slog.Error("gaudi stats output is not in expected format")
		return []Field{}
	}
	// build fields to match CSV output from hl_smi tool
	fields := []Field{}
	// first row is the header, extract field names
	for _, fieldName := range rows[0] {
		fields = append(fields, Field{Name: strings.TrimSpace(fieldName)})
	}
	// values start in 2nd row
	for _, row := range rows[1:] {
		for i := range fields {
			// reformat the timestamp field to only include the time
			if i == 0 {
				// parse the timestamp field's value
				rowTime, err := time.Parse("Mon Jan 2 15:04:05 MST 2006", row[i])
				if err != nil {
					err = fmt.Errorf("unable to parse Gaudi telemetry timestamp: %s", row[i])
					slog.Error(err.Error())
					return []Field{}
				}
				// reformat the timestamp field's value to include time only
				timestamp := rowTime.Format("15:04:05")
				fields[i].Values = append(fields[i].Values, timestamp)
			} else {
				fields[i].Values = append(fields[i].Values, strings.TrimSpace(row[i]))
			}
		}
	}
	return fields
}

func callStackFrequencyTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Native Stacks", Values: []string{nativeFoldedFromOutput(outputs)}},
		{Name: "Java Stacks", Values: []string{javaFoldedFromOutput(outputs)}},
		{Name: "Maximum Render Depth", Values: []string{maxRenderDepthFromOutput(outputs)}},
	}
	return fields
}

func kernelLockAnalysisTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Hotspot without Callstack", Values: []string{sectionValueFromOutput(outputs[script.ProfileKernelLockScriptName].Stdout, "perf_hotspot_no_children")}},
		{Name: "Hotspot with Callstack", Values: []string{sectionValueFromOutput(outputs[script.ProfileKernelLockScriptName].Stdout, "perf_hotspot_callgraph")}},
		{Name: "Cache2Cache without Callstack", Values: []string{sectionValueFromOutput(outputs[script.ProfileKernelLockScriptName].Stdout, "perf_c2c_no_children")}},
		{Name: "Cache2Cache with CallStack", Values: []string{sectionValueFromOutput(outputs[script.ProfileKernelLockScriptName].Stdout, "perf_c2c_callgraph")}},
		{Name: "Lock Contention", Values: []string{sectionValueFromOutput(outputs[script.ProfileKernelLockScriptName].Stdout, "perf_lock_contention")}},
		{Name: "Perf Package Path", Values: []string{strings.TrimSpace(sectionValueFromOutput(outputs[script.ProfileKernelLockScriptName].Stdout, "perf_package_path"))}},
	}
	return fields
}

func instructionTelemetryTableValues(outputs map[string]script.ScriptOutput) []Field {
	// first two lines are not part of the CSV output, they are the start time and interval
	var startTime time.Time
	var interval int
	lines := strings.Split(outputs[script.InstructionTelemetryScriptName].Stdout, "\n")
	if len(lines) < 4 {
		slog.Warn("no data found in instruction mix output")
		return []Field{}
	}
	// TIME
	line := lines[0]
	if !strings.HasPrefix(line, "TIME") {
		slog.Error("instruction mix output is not in expected format, missing TIME")
		return []Field{}
	} else {
		val := strings.Split(line, " ")[1]
		var err error
		startTime, err = time.Parse("15:04:05", val)
		if err != nil {
			slog.Error(fmt.Sprintf("unable to parse instruction mix start time: %s", val))
			return []Field{}
		}
	}
	// INTERVAL
	line = lines[1]
	if !strings.HasPrefix(line, "INTERVAL") {
		slog.Error("instruction mix output is not in expected format, missing INTERVAL")
		return []Field{}
	} else {
		val := strings.Split(line, " ")[1]
		var err error
		interval, err = strconv.Atoi(val)
		if err != nil {
			slog.Error(fmt.Sprintf("unable to convert instruction mix interval to int: %s", val))
			return []Field{}
		}
	}
	// remove blank lines that occur throughout the remaining lines
	csvLines := []string{}
	for _, line := range lines[2:] { // skip the TIME and INTERVAL lines
		if line != "" {
			csvLines = append(csvLines, line)
		}
	}
	if len(csvLines) < 2 {
		slog.Error("instruction mix CSV output is not in expected format, missing header and data")
		return []Field{}
	}
	// if processwatch was killed, it may print a partial output line at the end
	// check if the last line is a partial line by comparing the number of fields in the last line to the number of fields in the header
	if len(strings.Split(csvLines[len(csvLines)-1], ",")) != len(strings.Split(csvLines[0], ",")) {
		slog.Debug("removing partial line from instruction mix output", "line", csvLines[len(csvLines)-1], "lineNo", len(csvLines)-1)
		csvLines = csvLines[:len(csvLines)-1] // remove the last line
	}
	// CSV
	r := csv.NewReader(strings.NewReader(strings.Join(csvLines, "\n")))
	rows, err := r.ReadAll()
	if err != nil {
		slog.Error(err.Error())
		return []Field{}
	}
	if len(rows) < 2 {
		slog.Error("instruction mix CSV output is not in expected format")
		return []Field{}
	}
	fields := []Field{{Name: "Time"}}
	// first row is the header, extract field names, skip the first three fields (interval, pid, name)
	if len(rows[0]) < 3 {
		slog.Error("not enough headers in instruction mix CSV output", slog.Any("headers", rows[0]))
		return []Field{}
	}
	for _, field := range rows[0][3:] {
		fields = append(fields, Field{Name: field})
	}
	sample := -1
	// values start in 2nd row, we're only interested in the first row of the sample
	for _, row := range rows[1:] {
		if len(row) < 2+len(fields) {
			continue
		}
		rowSample, err := strconv.Atoi(row[0])
		if err != nil {
			slog.Error(fmt.Sprintf("unable to convert instruction mix sample to int: %s", row[0]))
			continue
		}
		if rowSample != sample { // new sample
			sample = rowSample
			for i := range fields {
				if i == 0 {
					fields[i].Values = append(fields[i].Values, startTime.Add(time.Duration(interval+(sample*interval))*time.Second).Format("15:04:05"))
				} else {
					fields[i].Values = append(fields[i].Values, row[i+2])
				}
			}
		}
	}
	return fields
}
