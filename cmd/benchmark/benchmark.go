// Package benchmark is a subcommand of the root command. It runs performance benchmarks on target(s).
package benchmark

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/xuri/excelize/v2"

	"perfspect/internal/common"
	"perfspect/internal/cpus"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/table"
	"perfspect/internal/util"
)

const cmdName = "benchmark"

var examples = []string{
	fmt.Sprintf("  Run all benchmarks:        $ %s %s", common.AppName, cmdName),
	fmt.Sprintf("  Run specific benchmarks:   $ %s %s --speed --power", common.AppName, cmdName),
	fmt.Sprintf("  Benchmark remote target:   $ %s %s --target 192.168.1.1 --user fred --key fred_key", common.AppName, cmdName),
	fmt.Sprintf("  Benchmark multiple targets:$ %s %s --targets targets.yaml", common.AppName, cmdName),
}

var Cmd = &cobra.Command{
	Use:           cmdName,
	Aliases:       []string{"bench"},
	Short:         "Run performance benchmarks on target(s)",
	Example:       strings.Join(examples, "\n"),
	RunE:          runCmd,
	PreRunE:       validateFlags,
	GroupID:       "primary",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
}

// flag vars
var (
	flagAll bool

	flagSpeed       bool
	flagPower       bool
	flagTemperature bool
	flagFrequency   bool
	flagMemory      bool
	flagNuma        bool
	flagStorage     bool

	flagNoSystemSummary bool

	flagStorageDir string
)

// flag names
const (
	flagAllName = "all"

	flagSpeedName       = "speed"
	flagPowerName       = "power"
	flagTemperatureName = "temperature"
	flagFrequencyName   = "frequency"
	flagMemoryName      = "memory"
	flagNumaName        = "numa"
	flagStorageName     = "storage"

	flagNoSystemSummaryName = "no-summary"

	flagStorageDirName = "storage-dir"
)

var benchmarkSummaryTableName = "Benchmark Summary"

var categories = []common.Category{
	{FlagName: flagSpeedName, FlagVar: &flagSpeed, DefaultValue: false, Help: "CPU speed benchmark", Tables: []table.TableDefinition{tableDefinitions[SpeedBenchmarkTableName]}},
	{FlagName: flagPowerName, FlagVar: &flagPower, DefaultValue: false, Help: "power consumption benchmark", Tables: []table.TableDefinition{tableDefinitions[PowerBenchmarkTableName]}},
	{FlagName: flagTemperatureName, FlagVar: &flagTemperature, DefaultValue: false, Help: "temperature benchmark", Tables: []table.TableDefinition{tableDefinitions[TemperatureBenchmarkTableName]}},
	{FlagName: flagFrequencyName, FlagVar: &flagFrequency, DefaultValue: false, Help: "turbo frequency benchmark", Tables: []table.TableDefinition{tableDefinitions[FrequencyBenchmarkTableName]}},
	{FlagName: flagMemoryName, FlagVar: &flagMemory, DefaultValue: false, Help: "memory latency and bandwidth benchmark", Tables: []table.TableDefinition{tableDefinitions[MemoryBenchmarkTableName]}},
	{FlagName: flagNumaName, FlagVar: &flagNuma, DefaultValue: false, Help: "NUMA bandwidth matrix benchmark", Tables: []table.TableDefinition{tableDefinitions[NUMABenchmarkTableName]}},
	{FlagName: flagStorageName, FlagVar: &flagStorage, DefaultValue: false, Help: "storage performance benchmark", Tables: []table.TableDefinition{tableDefinitions[StorageBenchmarkTableName]}},
}

func init() {
	// set up benchmark flags
	for _, benchmark := range categories {
		Cmd.Flags().BoolVar(benchmark.FlagVar, benchmark.FlagName, benchmark.DefaultValue, benchmark.Help)
	}
	// set up other flags
	Cmd.Flags().StringVar(&common.FlagInput, common.FlagInputName, "", "")
	Cmd.Flags().BoolVar(&flagAll, flagAllName, true, "")
	Cmd.Flags().StringSliceVar(&common.FlagFormat, common.FlagFormatName, []string{report.FormatAll}, "")
	Cmd.Flags().BoolVar(&flagNoSystemSummary, flagNoSystemSummaryName, false, "")
	Cmd.Flags().StringVar(&flagStorageDir, flagStorageDirName, "/tmp", "")

	common.AddTargetFlags(Cmd)

	Cmd.SetUsageFunc(usageFunc)
}

func usageFunc(cmd *cobra.Command) error {
	cmd.Printf("Usage: %s [flags]\n\n", cmd.CommandPath())
	cmd.Printf("Examples:\n%s\n\n", cmd.Example)
	cmd.Println("Flags:")
	for _, group := range getFlagGroups() {
		cmd.Printf("  %s:\n", group.GroupName)
		for _, flag := range group.Flags {
			flagDefault := ""
			if cmd.Flags().Lookup(flag.Name).DefValue != "" {
				flagDefault = fmt.Sprintf(" (default: %s)", cmd.Flags().Lookup(flag.Name).DefValue)
			}
			cmd.Printf("    --%-20s %s%s\n", flag.Name, flag.Help, flagDefault)
		}
	}
	cmd.Println("\nGlobal Flags:")
	cmd.Parent().PersistentFlags().VisitAll(func(pf *pflag.Flag) {
		flagDefault := ""
		if cmd.Parent().PersistentFlags().Lookup(pf.Name).DefValue != "" {
			flagDefault = fmt.Sprintf(" (default: %s)", cmd.Flags().Lookup(pf.Name).DefValue)
		}
		cmd.Printf("  --%-20s %s%s\n", pf.Name, pf.Usage, flagDefault)
	})
	return nil
}

func getFlagGroups() []common.FlagGroup {
	var groups []common.FlagGroup
	flags := []common.Flag{
		{
			Name: flagAllName,
			Help: "run all benchmarks",
		},
	}
	for _, benchmark := range categories {
		flags = append(flags, common.Flag{
			Name: benchmark.FlagName,
			Help: benchmark.Help,
		})
	}
	groups = append(groups, common.FlagGroup{
		GroupName: "Benchmark Options",
		Flags:     flags,
	})
	flags = []common.Flag{
		{
			Name: flagNoSystemSummaryName,
			Help: "do not include system summary in output",
		},
		{
			Name: flagStorageDirName,
			Help: "existing directory where storage performance benchmark data will be temporarily stored",
		},
		{
			Name: common.FlagFormatName,
			Help: fmt.Sprintf("choose output format(s) from: %s", strings.Join(append([]string{report.FormatAll}, report.FormatOptions...), ", ")),
		},
	}
	groups = append(groups, common.FlagGroup{
		GroupName: "Other Options",
		Flags:     flags,
	})
	groups = append(groups, common.GetTargetFlagGroup())
	flags = []common.Flag{
		{
			Name: common.FlagInputName,
			Help: "\".raw\" file, or directory containing \".raw\" files. Will skip data collection and use raw data for reports.",
		},
	}
	groups = append(groups, common.FlagGroup{
		GroupName: "Advanced Options",
		Flags:     flags,
	})
	return groups
}

func validateFlags(cmd *cobra.Command, args []string) error {
	// clear flagAll if any benchmarks are selected
	if flagAll {
		for _, benchmark := range categories {
			if benchmark.FlagVar != nil && *benchmark.FlagVar {
				flagAll = false
				break
			}
		}
	}
	// validate format options
	for _, format := range common.FlagFormat {
		formatOptions := append([]string{report.FormatAll}, report.FormatOptions...)
		if !slices.Contains(formatOptions, format) {
			return common.FlagValidationError(cmd, fmt.Sprintf("format options are: %s", strings.Join(formatOptions, ", ")))
		}
	}
	// validate storage dir
	if flagStorageDir != "" {
		if !util.IsValidDirectoryName(flagStorageDir) {
			return common.FlagValidationError(cmd, fmt.Sprintf("invalid storage directory name: %s", flagStorageDir))
		}
		// if no target is specified, i.e., we have a local target only, check if the directory exists
		if !cmd.Flags().Lookup("targets").Changed && !cmd.Flags().Lookup("target").Changed {
			if _, err := os.Stat(flagStorageDir); os.IsNotExist(err) {
				return common.FlagValidationError(cmd, fmt.Sprintf("storage dir does not exist: %s", flagStorageDir))
			}
		}
	}
	// common target flags
	if err := common.ValidateTargetFlags(cmd); err != nil {
		return common.FlagValidationError(cmd, err.Error())
	}
	return nil
}

func runCmd(cmd *cobra.Command, args []string) error {
	var tables []table.TableDefinition
	// add system summary table if not disabled
	if !flagNoSystemSummary {
		tables = append(tables, common.TableDefinitions[common.BriefSysSummaryTableName])
	}
	// add benchmark tables
	selectedBenchmarkCount := 0
	for _, benchmark := range categories {
		if *benchmark.FlagVar || flagAll {
			tables = append(tables, benchmark.Tables...)
			selectedBenchmarkCount++
		}
	}
	// include benchmark summary table if all benchmarks are selected
	var summaryFunc common.SummaryFunc
	if selectedBenchmarkCount == len(categories) {
		summaryFunc = benchmarkSummaryFromTableValues
	}

	reportingCommand := common.ReportingCommand{
		Cmd:                    cmd,
		ScriptParams:           map[string]string{"StorageDir": flagStorageDir},
		Tables:                 tables,
		SummaryFunc:            summaryFunc,
		SummaryTableName:       benchmarkSummaryTableName,
		SummaryBeforeTableName: SpeedBenchmarkTableName,
		InsightsFunc:           nil,
	}

	report.RegisterHTMLRenderer(FrequencyBenchmarkTableName, frequencyBenchmarkTableHtmlRenderer)
	report.RegisterHTMLRenderer(MemoryBenchmarkTableName, memoryBenchmarkTableHtmlRenderer)

	report.RegisterHTMLMultiTargetRenderer(MemoryBenchmarkTableName, memoryBenchmarkTableMultiTargetHtmlRenderer)

	return reportingCommand.Run()
}

func benchmarkSummaryFromTableValues(allTableValues []table.TableValues, outputs map[string]script.ScriptOutput) table.TableValues {
	maxFreq := getValueFromTableValues(getTableValues(allTableValues, FrequencyBenchmarkTableName), "SSE", 0)
	if maxFreq != "" {
		maxFreq = maxFreq + " GHz"
	}
	allCoreMaxFreq := getValueFromTableValues(getTableValues(allTableValues, FrequencyBenchmarkTableName), "SSE", -1)
	if allCoreMaxFreq != "" {
		allCoreMaxFreq = allCoreMaxFreq + " GHz"
	}
	// get the maximum memory bandwidth from the memory latency table
	memLatTableValues := getTableValues(allTableValues, MemoryBenchmarkTableName)
	var bandwidthValues []string
	if len(memLatTableValues.Fields) > 1 {
		bandwidthValues = memLatTableValues.Fields[1].Values
	}
	maxBandwidth := 0.0
	for _, bandwidthValue := range bandwidthValues {
		bandwidth, err := strconv.ParseFloat(bandwidthValue, 64)
		if err != nil {
			slog.Error("unexpected value in memory bandwidth", slog.String("error", err.Error()), slog.Float64("value", bandwidth))
			break
		}
		if bandwidth > maxBandwidth {
			maxBandwidth = bandwidth
		}
	}
	maxMemBW := ""
	if maxBandwidth != 0 {
		maxMemBW = fmt.Sprintf("%.1f GB/s", maxBandwidth)
	}
	// get the minimum memory latency
	minLatency := getValueFromTableValues(getTableValues(allTableValues, MemoryBenchmarkTableName), "Latency (ns)", 0)
	if minLatency != "" {
		minLatency = minLatency + " ns"
	}

	report.RegisterHTMLRenderer(benchmarkSummaryTableName, summaryHTMLTableRenderer)
	report.RegisterTextRenderer(benchmarkSummaryTableName, summaryTextTableRenderer)
	report.RegisterXlsxRenderer(benchmarkSummaryTableName, summaryXlsxTableRenderer)

	return table.TableValues{
		TableDefinition: table.TableDefinition{
			Name:      benchmarkSummaryTableName,
			HasRows:   false,
			MenuLabel: benchmarkSummaryTableName,
		},
		Fields: []table.Field{
			{Name: "CPU Speed", Values: []string{getValueFromTableValues(getTableValues(allTableValues, SpeedBenchmarkTableName), "Ops/s", 0) + " Ops/s"}},
			{Name: "Single-core Maximum frequency", Values: []string{maxFreq}},
			{Name: "All-core Maximum frequency", Values: []string{allCoreMaxFreq}},
			{Name: "Maximum Power", Values: []string{getValueFromTableValues(getTableValues(allTableValues, PowerBenchmarkTableName), "Maximum Power", 0)}},
			{Name: "Maximum Temperature", Values: []string{getValueFromTableValues(getTableValues(allTableValues, TemperatureBenchmarkTableName), "Maximum Temperature", 0)}},
			{Name: "Minimum Power", Values: []string{getValueFromTableValues(getTableValues(allTableValues, PowerBenchmarkTableName), "Minimum Power", 0)}},
			{Name: "Memory Peak Bandwidth", Values: []string{maxMemBW}},
			{Name: "Memory Minimum Latency", Values: []string{minLatency}},
			{Name: "Microarchitecture", Values: []string{common.UarchFromOutput(outputs)}},
			{Name: "Sockets", Values: []string{common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)}},
		},
	}
}

// getTableValues returns the table values for a table with a given name
func getTableValues(allTableValues []table.TableValues, tableName string) table.TableValues {
	for _, tv := range allTableValues {
		if tv.Name == tableName {
			return tv
		}
	}
	return table.TableValues{}
}

// getValueFromTableValues returns the value of a field in a table
// if row is -1, it returns the last value
func getValueFromTableValues(tv table.TableValues, fieldName string, row int) string {
	for _, fv := range tv.Fields {
		if fv.Name == fieldName {
			if row == -1 { // return the last value
				if len(fv.Values) == 0 {
					return ""
				}
				return fv.Values[len(fv.Values)-1]
			}
			if len(fv.Values) > row {
				return fv.Values[row]
			}
			break
		}
	}
	return ""
}

// ReferenceData is a struct that holds reference data for a microarchitecture
type ReferenceData struct {
	Description      string
	CPUSpeed         float64
	SingleCoreFreq   float64
	AllCoreFreq      float64
	MaxPower         float64
	MaxTemp          float64
	MinPower         float64
	MemPeakBandwidth float64
	MemMinLatency    float64
}

// ReferenceDataKey is a struct that holds the key for reference data
type ReferenceDataKey struct {
	Microarchitecture string
	Sockets           string
}

// referenceData is a map of reference data for microarchitectures
var referenceData = map[ReferenceDataKey]ReferenceData{
	{cpus.UarchBDX, "2"}:     {Description: "Reference (Intel 2S Xeon E5-2699 v4)", CPUSpeed: 403415, SingleCoreFreq: 3509, AllCoreFreq: 2980, MaxPower: 289.9, MaxTemp: 0, MinPower: 0, MemPeakBandwidth: 138.1, MemMinLatency: 78},
	{cpus.UarchSKX, "2"}:     {Description: "Reference (Intel 2S Xeon 8180)", CPUSpeed: 585157, SingleCoreFreq: 3758, AllCoreFreq: 3107, MaxPower: 429.07, MaxTemp: 0, MinPower: 0, MemPeakBandwidth: 225.1, MemMinLatency: 71},
	{cpus.UarchCLX, "2"}:     {Description: "Reference (Intel 2S Xeon 8280)", CPUSpeed: 548644, SingleCoreFreq: 3928, AllCoreFreq: 3926, MaxPower: 415.93, MaxTemp: 0, MinPower: 0, MemPeakBandwidth: 223.9, MemMinLatency: 72},
	{cpus.UarchICX, "2"}:     {Description: "Reference (Intel 2S Xeon 8380)", CPUSpeed: 933644, SingleCoreFreq: 3334, AllCoreFreq: 2950, MaxPower: 552, MaxTemp: 0, MinPower: 175.38, MemPeakBandwidth: 350.7, MemMinLatency: 70},
	{cpus.UarchSPR_XCC, "2"}: {Description: "Reference (Intel 2S Xeon 8480+)", CPUSpeed: 1678712, SingleCoreFreq: 3776, AllCoreFreq: 2996, MaxPower: 698.35, MaxTemp: 0, MinPower: 249.21, MemPeakBandwidth: 524.6, MemMinLatency: 111.8},
	{cpus.UarchSPR_XCC, "1"}: {Description: "Reference (Intel 1S Xeon 8480+)", CPUSpeed: 845743, SingleCoreFreq: 3783, AllCoreFreq: 2999, MaxPower: 334.68, MaxTemp: 0, MinPower: 163.79, MemPeakBandwidth: 264.0, MemMinLatency: 112.2},
	{cpus.UarchEMR_XCC, "2"}: {Description: "Reference (Intel 2S Xeon 8592V)", CPUSpeed: 1789534, SingleCoreFreq: 3862, AllCoreFreq: 2898, MaxPower: 664.4, MaxTemp: 0, MinPower: 166.36, MemPeakBandwidth: 553.5, MemMinLatency: 92.0},
	{cpus.UarchSRF_SP, "2"}:  {Description: "Reference (Intel 2S Xeon 6780E)", CPUSpeed: 3022446, SingleCoreFreq: 3001, AllCoreFreq: 3001, MaxPower: 583.97, MaxTemp: 0, MinPower: 123.34, MemPeakBandwidth: 534.3, MemMinLatency: 129.25},
	{cpus.UarchGNR_X2, "2"}:  {Description: "Reference (Intel 2S Xeon 6787P)", CPUSpeed: 3178562, SingleCoreFreq: 3797, AllCoreFreq: 3199, MaxPower: 679, MaxTemp: 0, MinPower: 248.49, MemPeakBandwidth: 749.6, MemMinLatency: 117.51},
}

// getFieldIndex returns the index of a field in a list of fields
func getFieldIndex(fields []table.Field, fieldName string) (int, error) {
	for i, field := range fields {
		if field.Name == fieldName {
			return i, nil
		}
	}
	return -1, fmt.Errorf("field not found: %s", fieldName)
}

// summaryHTMLTableRenderer is a custom HTML table renderer for the summary table
// it removes the Microarchitecture and Sockets fields and adds a reference table
func summaryHTMLTableRenderer(tv table.TableValues, targetName string) string {
	uarchFieldIdx, err := getFieldIndex(tv.Fields, "Microarchitecture")
	if err != nil {
		panic(err)
	}
	socketsFieldIdx, err := getFieldIndex(tv.Fields, "Sockets")
	if err != nil {
		panic(err)
	}
	// if we have reference data that matches the microarchitecture and sockets, use it
	if len(tv.Fields[uarchFieldIdx].Values) > 0 && len(tv.Fields[socketsFieldIdx].Values) > 0 {
		if refData, ok := referenceData[ReferenceDataKey{tv.Fields[uarchFieldIdx].Values[0], tv.Fields[socketsFieldIdx].Values[0]}]; ok {
			// remove microarchitecture and sockets fields
			fields := tv.Fields[:len(tv.Fields)-2]
			refTableValues := table.TableValues{
				Fields: []table.Field{
					{Name: "CPU Speed", Values: []string{fmt.Sprintf("%.0f Ops/s", refData.CPUSpeed)}},
					{Name: "Single-core Maximum frequency", Values: []string{fmt.Sprintf("%.0f MHz", refData.SingleCoreFreq)}},
					{Name: "All-core Maximum frequency", Values: []string{fmt.Sprintf("%.0f MHz", refData.AllCoreFreq)}},
					{Name: "Maximum Power", Values: []string{fmt.Sprintf("%.0f W", refData.MaxPower)}},
					{Name: "Maximum Temperature", Values: []string{fmt.Sprintf("%.0f C", refData.MaxTemp)}},
					{Name: "Minimum Power", Values: []string{fmt.Sprintf("%.0f W", refData.MinPower)}},
					{Name: "Memory Peak Bandwidth", Values: []string{fmt.Sprintf("%.0f GB/s", refData.MemPeakBandwidth)}},
					{Name: "Memory Minimum Latency", Values: []string{fmt.Sprintf("%.0f ns", refData.MemMinLatency)}},
				},
			}
			return report.RenderMultiTargetTableValuesAsHTML([]table.TableValues{{TableDefinition: tv.TableDefinition, Fields: fields}, refTableValues}, []string{targetName, refData.Description})
		}
	}
	// remove microarchitecture and sockets fields
	fields := tv.Fields[:len(tv.Fields)-2]
	return report.DefaultHTMLTableRendererFunc(table.TableValues{TableDefinition: tv.TableDefinition, Fields: fields})
}

func summaryXlsxTableRenderer(tv table.TableValues, f *excelize.File, targetName string, row *int) {
	// remove microarchitecture and sockets fields
	fields := tv.Fields[:len(tv.Fields)-2]
	report.DefaultXlsxTableRendererFunc(table.TableValues{TableDefinition: tv.TableDefinition, Fields: fields}, f, report.XlsxPrimarySheetName, row)
}

func summaryTextTableRenderer(tv table.TableValues) string {
	// remove microarchitecture and sockets fields
	fields := tv.Fields[:len(tv.Fields)-2]
	return report.DefaultTextTableRendererFunc(table.TableValues{TableDefinition: tv.TableDefinition, Fields: fields})
}
