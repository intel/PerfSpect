package report

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// table_defs.go defines the tables used for generating reports

import (
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"strings"

	"perfspect/internal/cpudb"
	"perfspect/internal/script"

	"github.com/xuri/excelize/v2"
)

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

type TableDefinition struct {
	Name        string
	ScriptNames []string
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

const (
	// report table names
	HostTableName               = "Host"
	SystemTableName             = "System"
	BaseboardTableName          = "Baseboard"
	ChassisTableName            = "Chassis"
	PCIeSlotsTableName          = "PCIe Slots"
	BIOSTableName               = "BIOS"
	OperatingSystemTableName    = "Operating System"
	SoftwareVersionTableName    = "Software Version"
	CPUTableName                = "CPU"
	ISATableName                = "ISA"
	AcceleratorTableName        = "Accelerator"
	PowerTableName              = "Power"
	CstateTableName             = "C-states"
	CoreTurboFrequencyTableName = "Core Turbo Frequency"
	UncoreTableName             = "Uncore"
	ElcTableName                = "Efficiency Latency Control"
	MemoryTableName             = "Memory"
	DIMMTableName               = "DIMM"
	NICTableName                = "NIC"
	NetworkIRQMappingTableName  = "Network IRQ Mapping"
	DiskTableName               = "Disk"
	FilesystemTableName         = "Filesystem"
	GPUTableName                = "GPU"
	GaudiTableName              = "Gaudi"
	CXLDeviceTableName          = "CXL Device"
	CVETableName                = "CVE"
	ProcessTableName            = "Process"
	SensorTableName             = "Sensor"
	ChassisStatusTableName      = "Chassis Status"
	PMUTableName                = "PMU"
	SystemEventLogTableName     = "System Event Log"
	KernelLogTableName          = "Kernel Log"
	SystemSummaryTableName      = "System Summary"
	// benchmark table names
	CPUSpeedTableName       = "CPU Speed"
	CPUPowerTableName       = "CPU Power"
	CPUTemperatureTableName = "CPU Temperature"
	CPUFrequencyTableName   = "CPU Frequency"
	MemoryLatencyTableName  = "Memory Latency"
	NUMABandwidthTableName  = "NUMA Bandwidth"
	// telemetry table names
	CPUUtilizationTableName        = "CPU Utilization"
	AverageCPUUtilizationTableName = "Average CPU Utilization"
	IRQRateTableName               = "IRQ Rate"
	DriveStatsTableName            = "Drive Stats"
	NetworkStatsTableName          = "Network Stats"
	MemoryStatsTableName           = "Memory Stats"
	PowerStatsTableName            = "Power Stats"
	// config  table names
	ConfigurationTableName = "Configuration"
	// flamegraph table names
	CodePathFrequencyTableName = "Code Path Frequency"
	// lock table names
	KernelLockAnalysisTableName = "Kernel Lock Analysis "
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
			script.SpecTurboFrequenciesScriptName,
			script.SpecTurboCoresScriptName,
			script.PPINName,
			script.PrefetchControlName,
			script.PrefetchersName,
			script.L3WaySizeName},
		FieldsFunc:   cpuTableValues,
		InsightsFunc: cpuTableInsights},
	ISATableName: {
		Name:        ISATableName,
		ScriptNames: []string{script.CpuidScriptName},
		FieldsFunc:  isaTableValues},
	AcceleratorTableName: {
		Name:    AcceleratorTableName,
		HasRows: true,
		ScriptNames: []string{
			script.LshwScriptName,
			script.IaaDevicesScriptName,
			script.DsaDevicesScriptName},
		FieldsFunc:   acceleratorTableValues,
		InsightsFunc: acceleratorTableInsights},
	PowerTableName: {
		Name:      PowerTableName,
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
	CoreTurboFrequencyTableName: {
		Name:    CoreTurboFrequencyTableName,
		HasRows: true,
		ScriptNames: []string{
			script.SpecTurboFrequenciesScriptName,
			script.SpecTurboCoresScriptName,
			script.LscpuScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
		},
		FieldsFunc:            coreTurboFrequencyTableValues,
		HTMLTableRendererFunc: coreTurboFrequencyTableHTMLRenderer},
	UncoreTableName: {
		Name:    UncoreTableName,
		HasRows: false,
		ScriptNames: []string{
			script.UncoreMaxFromMSRScriptName,
			script.UncoreMinFromMSRScriptName,
			script.UncoreMaxFromTPMIScriptName,
			script.UncoreMinFromTPMIScriptName,
			script.ChaCountScriptName,
			script.LscpuScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName},
		FieldsFunc: uncoreTableValues},
	ElcTableName: {
		Name:    ElcTableName,
		HasRows: true,
		ScriptNames: []string{
			script.ElcScriptName,
		},
		FieldsFunc:   elcTableValues,
		InsightsFunc: elcTableInsights},
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
			script.LspciDevicesScriptName},
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
			script.LshwScriptName,
		},
		FieldsFunc: nicTableValues},
	NetworkIRQMappingTableName: {
		Name:    NetworkIRQMappingTableName,
		HasRows: true,
		ScriptNames: []string{
			script.LshwScriptName,
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
		Name:    GaudiTableName,
		HasRows: true,
		ScriptNames: []string{
			script.GaudiInfoScriptName,
			script.GaudiFirmwareScriptName,
			script.GaudiNumaScriptName,
		},
		FieldsFunc: gaudiTableValues},
	CXLDeviceTableName: {
		Name:    CXLDeviceTableName,
		HasRows: true,
		ScriptNames: []string{
			script.LspciVmmScriptName,
		},
		FieldsFunc: cxlDeviceTableValues},
	PCIeSlotsTableName: {
		Name:    PCIeSlotsTableName,
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
			script.IpmitoolEventTimeScriptName,
		},
		FieldsFunc: chassisStatusTableValues},
	PMUTableName: {
		Name:    PMUTableName,
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
			script.L3WaySizeName,
			script.CpuidScriptName,
			script.BaseFrequencyScriptName,
			script.SpecTurboCoresScriptName,
			script.SpecTurboFrequenciesScriptName,
			script.PrefetchControlName,
			script.PrefetchersName,
			script.PPINName,
			script.LshwScriptName,
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
	//
	// configuration set table
	//
	ConfigurationTableName: {
		Name:    ConfigurationTableName,
		HasRows: false,
		ScriptNames: []string{
			script.LscpuScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
			script.L3WaySizeName,
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
			script.SpecTurboFrequenciesScriptName,
			script.SpecTurboCoresScriptName,
			script.ElcScriptName,
		},
		FieldsFunc: configurationTableValues},
	//
	// benchmarking tables
	//
	CPUSpeedTableName: {
		Name:      CPUSpeedTableName,
		MenuLabel: CPUSpeedTableName,
		HasRows:   false,
		ScriptNames: []string{
			script.CpuSpeedScriptName,
		},
		FieldsFunc: cpuSpeedTableValues},
	CPUPowerTableName: {
		Name:      CPUPowerTableName,
		MenuLabel: CPUPowerTableName,
		HasRows:   false,
		ScriptNames: []string{
			script.IdlePowerScriptName,
			script.TurboFrequencyPowerAndTemperatureScriptName,
		},
		FieldsFunc: cpuPowerTableValues},
	CPUTemperatureTableName: {
		Name:      CPUTemperatureTableName,
		MenuLabel: CPUTemperatureTableName,
		HasRows:   false,
		ScriptNames: []string{
			script.TurboFrequencyPowerAndTemperatureScriptName,
		},
		FieldsFunc: cpuTemperatureTableValues},
	CPUFrequencyTableName: {
		Name:      CPUFrequencyTableName,
		MenuLabel: CPUFrequencyTableName,
		HasRows:   true,
		ScriptNames: []string{
			script.SpecTurboFrequenciesScriptName,
			script.SpecTurboCoresScriptName,
			script.LscpuScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
			script.TurboFrequenciesScriptName,
		},
		FieldsFunc:            cpuFrequencyTableValues,
		HTMLTableRendererFunc: cpuFrequencyTableHtmlRenderer},
	MemoryLatencyTableName: {
		Name:      MemoryLatencyTableName,
		MenuLabel: MemoryLatencyTableName,
		HasRows:   true,
		ScriptNames: []string{
			script.MemoryBandwidthAndLatencyScriptName,
		},
		NoDataFound:                      "No memory latency data found. Please see the GitHub repository README for instructions on how to install Intel Memory Latency Checker (mlc).",
		FieldsFunc:                       memoryLatencyTableValues,
		HTMLTableRendererFunc:            memoryLatencyTableHtmlRenderer,
		HTMLMultiTargetTableRendererFunc: memoryLatencyTableMultiTargetHtmlRenderer},
	NUMABandwidthTableName: {
		Name:      NUMABandwidthTableName,
		MenuLabel: NUMABandwidthTableName,
		HasRows:   true,
		ScriptNames: []string{
			script.NumaBandwidthScriptName,
		},
		NoDataFound: "No NUMA bandwidth data found. Please see the GitHub repository README for instructions on how to install Intel Memory Latency Checker (mlc).",
		FieldsFunc:  numaBandwidthTableValues},
	//
	// telemetry tables
	//
	CPUUtilizationTableName: {
		Name:      CPUUtilizationTableName,
		MenuLabel: CPUUtilizationTableName,
		HasRows:   true,
		ScriptNames: []string{
			script.MpstatScriptName,
		},
		FieldsFunc:            cpuUtilizationTableValues,
		HTMLTableRendererFunc: cpuUtilizationTableHTMLRenderer},
	AverageCPUUtilizationTableName: {
		Name:      AverageCPUUtilizationTableName,
		MenuLabel: AverageCPUUtilizationTableName,
		HasRows:   true,
		ScriptNames: []string{
			script.MpstatScriptName,
		},
		FieldsFunc:            averageCPUUtilizationTableValues,
		HTMLTableRendererFunc: averageCPUUtilizationTableHTMLRenderer},
	IRQRateTableName: {
		Name:      IRQRateTableName,
		MenuLabel: IRQRateTableName,
		HasRows:   true,
		ScriptNames: []string{
			script.MpstatScriptName,
		},
		FieldsFunc:            irqRateTableValues,
		HTMLTableRendererFunc: irqRateTableHTMLRenderer},
	DriveStatsTableName: {
		Name:      DriveStatsTableName,
		MenuLabel: DriveStatsTableName,
		HasRows:   true,
		ScriptNames: []string{
			script.IostatScriptName,
		},
		FieldsFunc:            driveStatsTableValues,
		HTMLTableRendererFunc: driveStatsTableHTMLRenderer},
	NetworkStatsTableName: {
		Name:      NetworkStatsTableName,
		MenuLabel: NetworkStatsTableName,
		HasRows:   true,
		ScriptNames: []string{
			script.SarNetworkScriptName,
		},
		FieldsFunc:            networkStatsTableValues,
		HTMLTableRendererFunc: networkStatsTableHTMLRenderer},
	MemoryStatsTableName: {
		Name:      MemoryStatsTableName,
		MenuLabel: MemoryStatsTableName,
		HasRows:   true,
		ScriptNames: []string{
			script.SarMemoryScriptName,
		},
		FieldsFunc:            memoryStatsTableValues,
		HTMLTableRendererFunc: memoryStatsTableHTMLRenderer},
	PowerStatsTableName: {
		Name:      PowerStatsTableName,
		MenuLabel: PowerStatsTableName,
		HasRows:   true,
		ScriptNames: []string{
			script.TurbostatScriptName,
		},
		FieldsFunc:            powerStatsTableValues,
		HTMLTableRendererFunc: powerStatsTableHTMLRenderer},
	//
	// flamegraph tables
	//
	CodePathFrequencyTableName: {
		Name: CodePathFrequencyTableName,
		ScriptNames: []string{
			script.ProfileJavaScriptName,
			script.ProfileSystemScriptName,
		},
		FieldsFunc:            codePathFrequencyTableValues,
		HTMLTableRendererFunc: codePathFrequencyTableHTMLRenderer},
	//
	// kernel lock analysis tables
	//
	KernelLockAnalysisTableName: {
		Name: KernelLockAnalysisTableName,
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
	validateTableValues(tableValues)
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

func validateTableValues(tableValues TableValues) {
	if tableValues.Name == "" {
		panic("table name cannot be empty")
	}
	// no field values is a valid state
	if len(tableValues.Fields) == 0 {
		return
	}
	// field names cannot be empty
	for i, field := range tableValues.Fields {
		if field.Name == "" {
			panic(fmt.Sprintf("table %s, field %d, name cannot be empty", tableValues.Name, i))
		}
	}
	// the number of entries in each field must be the same
	numEntries := len(tableValues.Fields[0].Values)
	for i, field := range tableValues.Fields {
		if len(field.Values) != numEntries {
			panic(fmt.Sprintf("table %s, field %d, %s, number of entries must be the same for all fields", tableValues.Name, i, field.Name))
		}
	}
}

//
// define the fieldsFunc for each table
//

func hostTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "Host Name", Values: []string{strings.TrimSpace(outputs[script.HostnameScriptName].Stdout)}},
		{Name: "Time", Values: []string{strings.TrimSpace(outputs[script.DateScriptName].Stdout)}},
		{Name: "System", Values: []string{valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Manufacturer:\s*(.+?)$`) + " " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Product Name:\s*(.+?)$`)}},
		{Name: "Baseboard", Values: []string{valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Manufacturer:\s*(.+?)$`) + " " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Product Name:\s*(.+?)$`)}},
		{Name: "Chassis", Values: []string{valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Manufacturer:\s*(.+?)$`) + " " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Type:\s*(.+?)$`)}},
	}
}

func pcieSlotsTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Designation"},
		{Name: "Type"},
		{Name: "Length"},
		{Name: "Bus Address"},
		{Name: "Current Usage"},
	}
	fieldValues := valsArrayFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "9",
		[]string{
			`^Designation:\s*(.+?)$`,
			`^Type:\s*(.+?)$`,
			`^Length:\s*(.+?)$`,
			`^Bus Address:\s*(.+?)$`,
			`^Current Usage:\s*(.+?)$`,
		}...,
	)
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
		for j := range fieldValues {
			fields[i].Values = append(fields[i].Values, fieldValues[j][i])
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
		{Name: "Microarchitecture", Values: []string{uarchFromOutput(outputs)}},
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
		{Name: "L1d Cache", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^L1d cache:\s*(.+)$`)}},
		{Name: "L1i Cache", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^L1i cache:\s*(.+)$`)}},
		{Name: "L2 Cache", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^L2 cache:\s*(.+)$`)}},
		{Name: "L3 Cache", Values: []string{l3FromOutput(outputs)}},
		{Name: "L3 per Core", Values: []string{l3PerCoreFromOutput(outputs)}},
		{Name: "Memory Channels", Values: []string{channelsFromOutput(outputs)}},
		{Name: "Prefetchers", Values: []string{prefetchersFromOutput(outputs)}},
		{Name: "Intel Turbo Boost", Values: []string{turboEnabledFromOutput(outputs)}},
		{Name: "Virtualization", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Virtualization:\s*(.+)$`)}},
		{Name: "PPINs", Values: []string{ppinsFromOutput(outputs)}},
	}
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
	return []Field{
		{Name: "Name", Values: acceleratorNames()},
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
		if name == "DSA" && count != "0" && queues != "None" {
			insights = append(insights, Insight{
				Recommendation: "Consider configuring DSA to allow accelerated data copy and transformation in DSA-enabled software.",
				Justification:  "No work queues are configured for DSA accelerator(s).",
			})
		}
		if name == "IAA" && count != "0" && queues != "None" {
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
	fields := []Field{
		{Name: "Name"},
		{Name: "Status"}, // enabled/disabled
	}
	cstates := cstatesFromOutput(outputs)
	for _, cstateInfo := range cstates {
		fields[0].Values = append(fields[0].Values, cstateInfo.Name)
		fields[1].Values = append(fields[1].Values, cstateInfo.Status)
	}
	return fields
}

func uncoreTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "Min Frequency", Values: []string{uncoreMinFrequencyFromOutput(outputs)}},
		{Name: "Max Frequency", Values: []string{uncoreMaxFrequencyFromOutput(outputs)}},
		{Name: "CHA Count", Values: []string{chaCountFromOutput(outputs)}},
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
	}
	return insights
}

func coreTurboFrequencyTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Active Cores", Values: []string{}},
		{Name: "non-avx:spec", Values: []string{}},
	}
	bins, err := getSpecCountFrequencies(outputs)
	if err != nil {
		slog.Warn("unable to get spec count frequencies", slog.String("error", err.Error()))
		return fields
	}
	coresPerSocket := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(\d+)$`)
	coreCount, err := strconv.Atoi(coresPerSocket)
	if err != nil {
		slog.Warn("unable to get core count", slog.String("error", err.Error()))
		return fields
	}
	counts := []string{}
	frequencies := []string{}
	for i := 1; i <= coreCount; i++ {
		counts = append(counts, strconv.Itoa(i))
	}
	beginInt := 0
	for _, bin := range bins {
		count := bin[0]
		endInt, err := strconv.Atoi(count)
		if err != nil {
			slog.Warn("unable to convert count to int", slog.String("count", count), slog.String("error", err.Error()))
			return fields
		}
		endInt -= 1
		endInt = int(math.Min(float64(endInt), float64(coreCount-1)))
		for i := beginInt; i <= endInt; i++ {
			frequencies = append(frequencies, bin[1])
		}
		beginInt = endInt + 1
	}
	fields[0].Values = counts
	fields[1].Values = frequencies
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
			uarch := uarchFromOutput(outputs)
			if uarch != "" {
				CPUdb := cpudb.NewCPUDB()
				cpu, err := CPUdb.GetCPUByMicroArchitecture(uarch)
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
	// check if NUMA balancing is not enabled
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
	return insights
}

func dimmTableValues(outputs map[string]script.ScriptOutput) []Field {
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
	fields := []Field{
		{Name: "Name"},
		{Name: "Model"},
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
	allNicsInfo := nicInfoFromOutput(outputs)
	for _, nicInfo := range allNicsInfo {
		fields[0].Values = append(fields[0].Values, nicInfo.Name)
		fields[1].Values = append(fields[1].Values, nicInfo.Model)
		fields[2].Values = append(fields[2].Values, nicInfo.Speed)
		fields[3].Values = append(fields[3].Values, nicInfo.Link)
		fields[4].Values = append(fields[4].Values, nicInfo.Bus)
		fields[5].Values = append(fields[5].Values, nicInfo.Driver)
		fields[6].Values = append(fields[6].Values, nicInfo.DriverVersion)
		fields[7].Values = append(fields[7].Values, nicInfo.FirmwareVersion)
		fields[8].Values = append(fields[8].Values, nicInfo.MACAddress)
		fields[9].Values = append(fields[9].Values, nicInfo.NUMANode)
		fields[10].Values = append(fields[10].Values, nicInfo.IRQBalance)
	}
	return fields
}

func networkIRQMappingTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Interface"},
		{Name: "CPU:IRQs CPU:IRQs ..."},
	}
	nicIRQMappings := nicIRQMappingsFromOutput(outputs)
	for _, nicIRQMapping := range nicIRQMappings {
		fields[0].Values = append(fields[0].Values, nicIRQMapping[0])
		fields[1].Values = append(fields[1].Values, nicIRQMapping[1])
	}
	return fields
}

func diskTableValues(outputs map[string]script.ScriptOutput) []Field {
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
	allDisksInfo := diskInfoFromOutput(outputs)
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
	fields := []Field{
		{Name: "Manufacturer"},
		{Name: "Model"},
		{Name: "PCI ID"},
	}
	gpuInfos := gpuInfoFromOutput(outputs)
	for _, gpuInfo := range gpuInfos {
		fields[0].Values = append(fields[0].Values, gpuInfo.Manufacturer)
		fields[1].Values = append(fields[1].Values, gpuInfo.Model)
		fields[2].Values = append(fields[2].Values, gpuInfo.PCIID)
	}
	return fields
}

func gaudiTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Module ID"},
		{Name: "Serial Number"},
		{Name: "Bus ID"},
		{Name: "Driver Version"},
		{Name: "EROM"},
		{Name: "CPLD"},
		{Name: "SPI"},
		{Name: "NUMA"},
	}
	gaudiInfos := gaudiInfoFromOutput(outputs)
	for _, gaudiInfo := range gaudiInfos {
		fields[0].Values = append(fields[0].Values, gaudiInfo.ModuleID)
		fields[1].Values = append(fields[1].Values, gaudiInfo.SerialNumber)
		fields[2].Values = append(fields[2].Values, gaudiInfo.BusID)
		fields[3].Values = append(fields[3].Values, gaudiInfo.DriverVersion)
		fields[4].Values = append(fields[4].Values, gaudiInfo.EROM)
		fields[5].Values = append(fields[5].Values, gaudiInfo.CPLD)
		fields[6].Values = append(fields[6].Values, gaudiInfo.SPI)
		fields[7].Values = append(fields[7].Values, gaudiInfo.NUMA)
	}
	return fields
}

func cxlDeviceTableValues(outputs map[string]script.ScriptOutput) []Field {
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
	cxlDevices := getPCIDevices("CXL", outputs)
	for _, cxlDevice := range cxlDevices {
		for _, field := range fields {
			if value, ok := cxlDevice[field.Name]; ok {
				field.Values = append(field.Values, value)
			} else {
				field.Values = append(field.Values, "")
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
	for _, line := range strings.Split(outputs[script.IpmitoolSensorsScriptName].Stdout, "\n") {
		tokens := strings.Split(line, " | ")
		if len(tokens) < len(fields) {
			continue
		}
		fields[0].Values = append(fields[0].Values, tokens[0])
		fields[1].Values = append(fields[1].Values, tokens[1])
		fields[2].Values = append(fields[2].Values, tokens[2])
	}
	return fields
}

func chassisStatusTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "Last Power Event", Values: []string{valFromRegexSubmatch(outputs[script.IpmitoolChassisScriptName].Stdout, `^Last Power Event\s*: (.+?)$`)}},
		{Name: "Power Overload", Values: []string{valFromRegexSubmatch(outputs[script.IpmitoolChassisScriptName].Stdout, `^Power Overload\s*: (.+?)$`)}},
		{Name: "Main Power Fault", Values: []string{valFromRegexSubmatch(outputs[script.IpmitoolChassisScriptName].Stdout, `^Main Power Fault\s*: (.+?)$`)}},
		{Name: "Power Restore Policy", Values: []string{valFromRegexSubmatch(outputs[script.IpmitoolChassisScriptName].Stdout, `^Power Restore Policy\s*: (.+?)$`)}},
		{Name: "Drive Fault", Values: []string{valFromRegexSubmatch(outputs[script.IpmitoolChassisScriptName].Stdout, `^Drive Fault\s*: (.+?)$`)}},
		{Name: "Cooling/Fan Fault", Values: []string{valFromRegexSubmatch(outputs[script.IpmitoolChassisScriptName].Stdout, `^Cooling/Fan Fault\s*: (.+?)$`)}},
		{Name: "System Time", Values: []string{strings.TrimSpace(outputs[script.IpmitoolEventTimeScriptName].Stdout)}},
	}
}

func systemEventLogTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Date"},
		{Name: "Time"},
		{Name: "Sensor"},
		{Name: "Status"},
		{Name: "Event"},
	}
	for _, line := range strings.Split(outputs[script.IpmitoolEventsScriptName].Stdout, "\n") {
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
	return []Field{
		{Name: "Host Name", Values: []string{strings.TrimSpace(outputs[script.HostnameScriptName].Stdout)}},
		{Name: "Time", Values: []string{strings.TrimSpace(outputs[script.DateScriptName].Stdout)}},
		{Name: "System", Values: []string{valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Manufacturer:\s*(.+?)$`) + " " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Product Name:\s*(.+?)$`)}},
		{Name: "Baseboard", Values: []string{valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Manufacturer:\s*(.+?)$`) + " " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "2", `^Product Name:\s*(.+?)$`)}},
		{Name: "Chassis", Values: []string{valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Manufacturer:\s*(.+?)$`) + " " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "3", `^Type:\s*(.+?)$`)}},
		{Name: "CPU Model", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^[Mm]odel name:\s*(.+)$`)}},
		{Name: "Architecture", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Architecture:\s*(.+)$`)}},
		{Name: "Microarchitecture", Values: []string{uarchFromOutput(outputs)}},
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
		{Name: "Prefetchers", Values: []string{prefetchersFromOutput(outputs)}},
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

func configurationTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "Cores per Socket", Values: []string{valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(.+)$`)}},
		{Name: "L3 Cache", Values: []string{l3FromOutput(outputs)}},
		{Name: "Package Power / TDP (Watts)", Values: []string{tdpFromOutput(outputs)}},
		{Name: "All-Core Max Frequency (GHz)", Values: []string{allCoreMaxFrequencyFromOutput(outputs)}},
		{Name: "Uncore Max Frequency (GHz)", Values: []string{uncoreMaxFrequencyFromOutput(outputs)}},
		{Name: "Uncore Min Frequency (GHz)", Values: []string{uncoreMinFrequencyFromOutput(outputs)}},
		{Name: "Energy Performance Bias", Values: []string{epbFromOutput(outputs)}},
		{Name: "Energy Performance Preference", Values: []string{eppFromOutput(outputs)}},
		{Name: "Scaling Governor", Values: []string{strings.TrimSpace(outputs[script.ScalingGovernorScriptName].Stdout)}},
		{Name: "Efficiency Latency Control", Values: []string{elcSummaryFromOutput(outputs)}},
	}
}

// benchmarking

func cpuSpeedTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "Ops/s", Values: []string{cpuSpeedFromOutput(outputs)}},
	}
}

func cpuPowerTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "Maximum Power", Values: []string{maxPowerFromOutput(outputs)}},
		{Name: "Minimum Power", Values: []string{minPowerFromOutput(outputs)}},
	}
}

func cpuTemperatureTableValues(outputs map[string]script.ScriptOutput) []Field {
	return []Field{
		{Name: "Maximum Temperature", Values: []string{maxTemperatureFromOutput(outputs)}},
	}
}

func cpuFrequencyTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := coreTurboFrequencyTableValues(outputs) // may result in no values within the two fields
	if len(fields) != 2 {
		panic("coreTurboFrequencyTableValues must return 2 fields")
	}
	fields = append(fields, []Field{
		{Name: "non-avx"},
		{Name: "avx128"},
		{Name: "avx256"},
		{Name: "avx512"},
	}...)
	nonavxFreqs, avx128Freqs, avx256Freqs, avx512Freqs, err := avxTurboFrequenciesFromOutput(outputs[script.TurboFrequenciesScriptName].Stdout)
	if err != nil {
		slog.Warn(err.Error())
		fields[0].Values = []string{}
		fields[1].Values = []string{}
		return fields
	} else {
		// pad core counts and spec frequency (field 0 and 1)
		for i := 0; i < 2; i++ {
			for j := len(fields[i].Values); j < len(nonavxFreqs); j++ {
				pad := ""
				if i == 0 {
					pad = strconv.Itoa(j + 1)
				}
				fields[i].Values = append(fields[i].Values, pad)
			}
		}
		for i := 0; i < len(nonavxFreqs); i++ {
			fields[2].Values = append(fields[2].Values, fmt.Sprintf("%.1f", nonavxFreqs[i]))
		}
		for i := 0; i < len(avx128Freqs); i++ {
			fields[3].Values = append(fields[3].Values, fmt.Sprintf("%.1f", avx128Freqs[i]))
		}
		for i := 0; i < len(avx256Freqs); i++ {
			fields[4].Values = append(fields[4].Values, fmt.Sprintf("%.1f", avx256Freqs[i]))
		}
		for i := 0; i < len(avx512Freqs); i++ {
			fields[5].Values = append(fields[5].Values, fmt.Sprintf("%.1f", avx512Freqs[i]))
		}
	}
	// pad frequency fields with empty string
	for i := 2; i < len(fields); i++ {
		for j := len(fields[i].Values); j < len(fields[0].Values); j++ {
			fields[i].Values = append(fields[i].Values, "")
		}
	}
	return fields
}

func memoryLatencyTableValues(outputs map[string]script.ScriptOutput) []Field {
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
	latencyBandwidthPairs := valsArrayFromRegexSubmatch(outputs[script.MemoryBandwidthAndLatencyScriptName].Stdout, `\s*[0-9]*\s*([0-9]*\.[0-9]+)\s*([0-9]*\.[0-9]+)`)
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
	return fields
}

func numaBandwidthTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Node"},
	}
	/* MLC Output:
			Numa node
	Numa node	     0	     1
	       0	175610.3	55579.7
	       1	55575.2	175656.7
	*/
	nodeBandwidthsPairs := valsArrayFromRegexSubmatch(outputs[script.NumaBandwidthScriptName].Stdout, `^\s+(\d)\s+(\d.*)$`)
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
			val, err := strconv.ParseFloat(bw, 64)
			if err != nil {
				slog.Error(fmt.Sprintf("Unable to convert bandwidth to float: %s", bw))
				continue
			}
			fields[i+1].Values = append(fields[i+1].Values, fmt.Sprintf("%.1f", val/1000))
		}
	}
	return fields
}

// telemetry

func cpuUtilizationTableValues(outputs map[string]script.ScriptOutput) []Field {
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
	for _, line := range strings.Split(outputs[script.MpstatScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	return fields
}

func averageCPUUtilizationTableValues(outputs map[string]script.ScriptOutput) []Field {
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
	for _, line := range strings.Split(outputs[script.MpstatScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	return fields
}

func irqRateTableValues(outputs map[string]script.ScriptOutput) []Field {
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
	for _, line := range strings.Split(outputs[script.MpstatScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	return fields
}

func driveStatsTableValues(outputs map[string]script.ScriptOutput) []Field {
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
	for _, line := range strings.Split(outputs[script.IostatScriptName].Stdout, "\n") {
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
	return fields
}

func networkStatsTableValues(outputs map[string]script.ScriptOutput) []Field {
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
	for _, line := range strings.Split(outputs[script.SarNetworkScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	return fields
}

func memoryStatsTableValues(outputs map[string]script.ScriptOutput) []Field {
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
	for _, line := range strings.Split(outputs[script.SarMemoryScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	return fields
}

func powerStatsTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Time"},
		{Name: "Package"},
		{Name: "DRAM"},
	}
	reStat := regexp.MustCompile(`^(\d\d:\d\d:\d\d)\s*(\d+\.\d+)\s*(\d+\.\d+)$`)
	for _, line := range strings.Split(outputs[script.TurbostatScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	return fields
}

func codePathFrequencyTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "System Paths", Values: []string{systemFoldedFromOutput(outputs)}},
		{Name: "Java Paths", Values: []string{javaFoldedFromOutput(outputs)}},
	}
	return fields
}

func kernelLockAnalysisTableValues(outputs map[string]script.ScriptOutput) []Field {
	fields := []Field{
		{Name: "Hotspot without Callstack", Values: []string{sectionValueFromOutput(outputs, "perf_hotspot_no_children")}},
		{Name: "Hotspot with Callstack", Values: []string{sectionValueFromOutput(outputs, "perf_hotspot_callgraph")}},
		{Name: "Cache2Cache without Callstack", Values: []string{sectionValueFromOutput(outputs, "perf_c2c_no_children")}},
		{Name: "Cache2Cache with CallStack", Values: []string{sectionValueFromOutput(outputs, "perf_c2c_callgraph")}},
		{Name: "Lock Contention", Values: []string{sectionValueFromOutput(outputs, "perf_lock_contention")}},
	}
	return fields
}
