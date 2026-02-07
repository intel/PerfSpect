// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Package telemetry is a subcommand of the root command. It collects system telemetry from target(s).
package telemetry

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"

	"perfspect/internal/app"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/table"
	"perfspect/internal/workflow"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const cmdName = "telemetry"

var examples = []string{
	fmt.Sprintf("  Telemetry from local host:       $ %s %s", app.Name, cmdName),
	fmt.Sprintf("  Telemetry from remote target:    $ %s %s --target 192.168.1.1 --user fred --key fred_key", app.Name, cmdName),
	fmt.Sprintf("  Memory telemetry for 60 seconds: $ %s %s --memory --duration 60", app.Name, cmdName),
	fmt.Sprintf("  Telemetry from multiple targets: $ %s %s --targets targets.yaml", app.Name, cmdName),
}

var Cmd = &cobra.Command{
	Use:           cmdName,
	Aliases:       []string{"telem"},
	Short:         "Collect system telemetry from target(s)",
	Long:          "",
	Example:       strings.Join(examples, "\n"),
	RunE:          runCmd,
	PreRunE:       validateFlags,
	GroupID:       "primary",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
}

var (
	flagDuration int
	flagInterval int

	flagAll bool

	flagCPU           bool
	flagFrequency     bool
	flagIPC           bool
	flagC6            bool
	flagIRQRate       bool
	flagMemory        bool
	flagNetwork       bool
	flagStorage       bool
	flagPower         bool
	flagTemperature   bool
	flagInstrMix      bool
	flagVirtualMemory bool
	flagProcess       bool

	flagNoSystemSummary bool

	flagInstrMixPid       int
	flagInstrMixFrequency int
)

const (
	flagDurationName = "duration"
	flagIntervalName = "interval"

	flagAllName = "all"

	flagCPUName           = "cpu"
	flagFrequencyName     = "frequency"
	flagIPCName           = "ipc"
	flagC6Name            = "c6"
	flagIRQRateName       = "irqrate"
	flagMemoryName        = "memory"
	flagNetworkName       = "network"
	flagStorageName       = "storage"
	flagPowerName         = "power"
	flagTemperatureName   = "temperature"
	flagInstrMixName      = "instrmix"
	flagVirtualMemoryName = "virtual-memory"
	flagProcessName       = "process"

	flagNoSystemSummaryName = "no-summary"

	flagInstrMixPidName       = "instrmix-pid"
	flagInstrMixFrequencyName = "instrmix-frequency"
)

var telemetrySummaryTableName = "Telemetry Summary"

var categories = []app.Category{
	{FlagName: flagCPUName, FlagVar: &flagCPU, DefaultValue: false, Help: "monitor cpu utilization", Tables: []table.TableDefinition{tableDefinitions[CPUUtilizationTelemetryTableName], tableDefinitions[UtilizationCategoriesTelemetryTableName]}},
	{FlagName: flagIPCName, FlagVar: &flagIPC, DefaultValue: false, Help: "monitor IPC", Tables: []table.TableDefinition{tableDefinitions[IPCTelemetryTableName]}},
	{FlagName: flagC6Name, FlagVar: &flagC6, DefaultValue: false, Help: "monitor C6 residency", Tables: []table.TableDefinition{tableDefinitions[C6TelemetryTableName]}},
	{FlagName: flagFrequencyName, FlagVar: &flagFrequency, DefaultValue: false, Help: "monitor cpu frequency", Tables: []table.TableDefinition{tableDefinitions[FrequencyTelemetryTableName]}},
	{FlagName: flagPowerName, FlagVar: &flagPower, DefaultValue: false, Help: "monitor power", Tables: []table.TableDefinition{tableDefinitions[PowerTelemetryTableName]}},
	{FlagName: flagTemperatureName, FlagVar: &flagTemperature, DefaultValue: false, Help: "monitor temperature", Tables: []table.TableDefinition{tableDefinitions[TemperatureTelemetryTableName]}},
	{FlagName: flagMemoryName, FlagVar: &flagMemory, DefaultValue: false, Help: "monitor memory", Tables: []table.TableDefinition{tableDefinitions[MemoryTelemetryTableName]}},
	{FlagName: flagNetworkName, FlagVar: &flagNetwork, DefaultValue: false, Help: "monitor network", Tables: []table.TableDefinition{tableDefinitions[NetworkTelemetryTableName]}},
	{FlagName: flagStorageName, FlagVar: &flagStorage, DefaultValue: false, Help: "monitor storage", Tables: []table.TableDefinition{tableDefinitions[DriveTelemetryTableName]}},
	{FlagName: flagIRQRateName, FlagVar: &flagIRQRate, DefaultValue: false, Help: "monitor IRQ rate", Tables: []table.TableDefinition{tableDefinitions[IRQRateTelemetryTableName]}},
	{FlagName: flagInstrMixName, FlagVar: &flagInstrMix, DefaultValue: false, Help: "monitor instruction mix", Tables: []table.TableDefinition{tableDefinitions[InstructionTelemetryTableName]}},
	{FlagName: flagVirtualMemoryName, FlagVar: &flagVirtualMemory, DefaultValue: false, Help: "monitor virtual memory", Tables: []table.TableDefinition{tableDefinitions[VirtualMemoryTelemetryTableName]}},
	{FlagName: flagProcessName, FlagVar: &flagProcess, DefaultValue: false, Help: "monitor process telemetry", Tables: []table.TableDefinition{tableDefinitions[ProcessTelemetryTableName]}},
}

const (
	instrmixFrequencyDefaultSystemWide = 10000000
	instrmixFrequencyDefaultPerPID     = 100000
)

func init() {
	// set up config category flags
	for _, cat := range categories {
		Cmd.Flags().BoolVar(cat.FlagVar, cat.FlagName, cat.DefaultValue, cat.Help)
	}
	Cmd.Flags().StringVar(&app.FlagInput, app.FlagInputName, "", "")
	Cmd.Flags().BoolVar(&flagAll, flagAllName, true, "")
	Cmd.Flags().StringSliceVar(&app.FlagFormat, app.FlagFormatName, []string{report.FormatAll}, "")
	Cmd.Flags().IntVar(&flagDuration, flagDurationName, 0, "")
	Cmd.Flags().IntVar(&flagInterval, flagIntervalName, 2, "")
	Cmd.Flags().IntVar(&flagInstrMixPid, flagInstrMixPidName, 0, "")
	Cmd.Flags().IntVar(&flagInstrMixFrequency, flagInstrMixFrequencyName, instrmixFrequencyDefaultSystemWide, "")
	Cmd.Flags().BoolVar(&flagNoSystemSummary, flagNoSystemSummaryName, false, "")

	workflow.AddTargetFlags(Cmd)

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

func getFlagGroups() []app.FlagGroup {
	var groups []app.FlagGroup
	flags := []app.Flag{
		{
			Name: flagAllName,
			Help: "collect telemetry for all categories",
		},
	}
	for _, cat := range categories {
		flags = append(flags, app.Flag{
			Name: cat.FlagName,
			Help: cat.Help,
		})
	}
	groups = append(groups, app.FlagGroup{
		GroupName: "Categories",
		Flags:     flags,
	})
	flags = []app.Flag{
		{
			Name: app.FlagFormatName,
			Help: fmt.Sprintf("choose output format(s) from: %s", strings.Join(append([]string{report.FormatAll}, report.FormatOptions...), ", ")),
		},
		{
			Name: flagDurationName,
			Help: "number of seconds to run the collection. If 0, the collection will run indefinitely. Ctrl+c to stop.",
		},
		{
			Name: flagIntervalName,
			Help: "number of seconds between each sample",
		},
		{
			Name: flagInstrMixPidName,
			Help: "PID to monitor for instruction mix, no PID means all processes",
		},
		{
			Name: flagInstrMixFrequencyName,
			Help: "number of instructions between samples, default is 10,000,000 when collecting system wide and 100,000 when collecting for a specific PID",
		},
		{
			Name: flagNoSystemSummaryName,
			Help: "do not include system summary table in report",
		},
	}
	groups = append(groups, app.FlagGroup{
		GroupName: "Other Options",
		Flags:     flags,
	})
	groups = append(groups, workflow.GetTargetFlagGroup())
	flags = []app.Flag{
		{
			Name: app.FlagInputName,
			Help: "\".raw\" file, or directory containing \".raw\" files. Will skip data collection and use raw data for reports.",
		},
	}
	groups = append(groups, app.FlagGroup{
		GroupName: "Advanced Options",
		Flags:     flags,
	})
	return groups
}

func validateFlags(cmd *cobra.Command, args []string) error {
	// clear flagAll if any categories are selected
	if flagAll {
		for _, cat := range categories {
			if cat.FlagVar != nil && *cat.FlagVar {
				flagAll = false
				break
			}
		}
	}
	// validate format options
	for _, format := range app.FlagFormat {
		formatOptions := []string{report.FormatAll}
		formatOptions = append(formatOptions, report.FormatOptions...)
		if !slices.Contains(formatOptions, format) {
			return workflow.FlagValidationError(cmd, fmt.Sprintf("format options are: %s", strings.Join(formatOptions, ", ")))
		}
	}
	if flagInterval < 1 {
		return workflow.FlagValidationError(cmd, "interval must be 1 or greater")
	}
	if flagDuration < 0 {
		return workflow.FlagValidationError(cmd, "duration must be 0 or greater")
	}
	if flagInstrMixFrequency < 100000 { // 100,000 instructions is the minimum frequency
		return workflow.FlagValidationError(cmd, "instruction mix frequency must be 100,000 or greater to limit overhead")
	}
	// warn if instruction mix frequency is low when collecting system wide
	if flagInstrMix && flagInstrMixPid == 0 && flagInstrMixFrequency < instrmixFrequencyDefaultSystemWide {
		slog.Warn("instruction mix frequency is set to a value lower than default for system wide collection, consider using a higher frequency to limit collection overhead", slog.Int("frequency", flagInstrMixFrequency))
	}
	// common target flags
	if err := workflow.ValidateTargetFlags(cmd); err != nil {
		return workflow.FlagValidationError(cmd, err.Error())
	}
	return nil
}

func runCmd(cmd *cobra.Command, args []string) error {
	var tables []table.TableDefinition
	// add system summary table if not disabled
	if !flagNoSystemSummary {
		tables = append(tables, app.TableDefinitions[app.SystemSummaryTableName])
	}
	// add category tables
	for _, cat := range categories {
		if *cat.FlagVar || flagAll {
			tables = append(tables, cat.Tables...)
		}
	}
	// confirm proper default for instrmix frequency
	if flagInstrMix {
		if flagInstrMixPid != 0 && !cmd.Flags().Changed(flagInstrMixFrequencyName) {
			// per-PID collection and frequency not changed, set to per-PID default
			flagInstrMixFrequency = instrmixFrequencyDefaultPerPID
		}
	}
	// hidden feature - Gaudi telemetry, only enabled when PERFSPECT_GAUDI_HLSMI_PATH is set
	gaudiHlsmiPath := os.Getenv("PERFSPECT_GAUDI_HLSMI_PATH") // must be full path to hlsmi binary
	if gaudiHlsmiPath != "" {
		slog.Info("Gaudi telemetry enabled", slog.String("hlsmi_path", gaudiHlsmiPath))
		tables = append(tables, tableDefinitions[GaudiTelemetryTableName])
	}
	// hidden feature - PDU telemetry, only enabled when four environment variables are set
	pduHost := os.Getenv("PERFSPECT_PDU_HOST")
	pduUser := os.Getenv("PERFSPECT_PDU_USER")
	pduPassword := os.Getenv("PERFSPECT_PDU_PASSWORD")
	pduOutlet := os.Getenv("PERFSPECT_PDU_OUTLET")
	if pduHost != "" && pduUser != "" && pduPassword != "" && pduOutlet != "" {
		slog.Info("PDU telemetry enabled", slog.String("host", pduHost), slog.String("outlet", pduOutlet))
		tables = append(tables, tableDefinitions[PDUTelemetryTableName])
	}
	// include telemetry summary table if all telemetry options are selected
	var summaryFunc app.SummaryFunc
	if flagAll {
		summaryFunc = summaryFromTableValues
	}
	// include insights table if all categories are selected
	var insightsFunc app.InsightsFunc
	if flagAll {
		insightsFunc = workflow.DefaultInsightsFunc
	}
	reportingCommand := workflow.ReportingCommand{
		Cmd:            cmd,
		ReportNamePost: "telem",
		ScriptParams: map[string]string{
			"Interval":          strconv.Itoa(flagInterval),
			"Duration":          strconv.Itoa(flagDuration),
			"InstrMixPID":       strconv.Itoa(flagInstrMixPid),
			"InstrMixFrequency": strconv.Itoa(flagInstrMixFrequency),
			"GaudiHlsmiPath":    gaudiHlsmiPath,
			"PDUHost":           pduHost,
			"PDUUser":           pduUser,
			"PDUPassword":       pduPassword,
			"PDUOutlet":         pduOutlet,
		},
		Tables:                 tables,
		SummaryFunc:            summaryFunc,
		SummaryTableName:       telemetrySummaryTableName,
		SummaryBeforeTableName: CPUUtilizationTelemetryTableName,
		InsightsFunc:           insightsFunc,
	}

	report.RegisterHTMLRenderer(CPUUtilizationTelemetryTableName, cpuUtilizationTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(UtilizationCategoriesTelemetryTableName, utilizationCategoriesTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(IPCTelemetryTableName, ipcTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(C6TelemetryTableName, c6TelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(FrequencyTelemetryTableName, averageFrequencyTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(IRQRateTelemetryTableName, irqRateTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(DriveTelemetryTableName, driveTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(NetworkTelemetryTableName, networkTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(MemoryTelemetryTableName, memoryTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(PowerTelemetryTableName, powerTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(TemperatureTelemetryTableName, temperatureTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(InstructionTelemetryTableName, instructionTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(GaudiTelemetryTableName, gaudiTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(PDUTelemetryTableName, pduTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(VirtualMemoryTelemetryTableName, virtualMemoryTelemetryTableHTMLRenderer)
	report.RegisterHTMLRenderer(ProcessTelemetryTableName, processTelemetryTableHTMLRenderer)

	return reportingCommand.Run()
}

func getTableValues(allTableValues []table.TableValues, tableName string) table.TableValues {
	for _, tv := range allTableValues {
		if tv.Name == tableName {
			return tv
		}
	}
	return table.TableValues{}
}

func summaryFromTableValues(allTableValues []table.TableValues, _ map[string]script.ScriptOutput) table.TableValues {
	cpuUtil := getCPUAveragePercentage(getTableValues(allTableValues, UtilizationCategoriesTelemetryTableName), "%idle", true)
	ipc := getCPUAveragePercentage(getTableValues(allTableValues, IPCTelemetryTableName), "Core (Avg.)", false)
	c6 := getCPUAveragePercentage(getTableValues(allTableValues, C6TelemetryTableName), "Core (Avg.)", false)
	avgCoreFreq := getMetricAverage(getTableValues(allTableValues, FrequencyTelemetryTableName), []string{"Core (Avg.)"}, "Time")
	pkgPower := getPkgAveragePower(allTableValues)
	pkgTemperature := getPkgAverageTemperature(allTableValues)
	driveReads := getMetricAverage(getTableValues(allTableValues, DriveTelemetryTableName), []string{"kB_read/s"}, "Device")
	driveWrites := getMetricAverage(getTableValues(allTableValues, DriveTelemetryTableName), []string{"kB_wrtn/s"}, "Device")
	networkReads := getMetricAverage(getTableValues(allTableValues, NetworkTelemetryTableName), []string{"rxkB/s"}, "Time")
	networkWrites := getMetricAverage(getTableValues(allTableValues, NetworkTelemetryTableName), []string{"txkB/s"}, "Time")
	memAvail := getMetricAverage(getTableValues(allTableValues, MemoryTelemetryTableName), []string{"avail"}, "Time")
	return table.TableValues{
		TableDefinition: table.TableDefinition{
			Name:      telemetrySummaryTableName,
			HasRows:   false,
			MenuLabel: telemetrySummaryTableName,
		},
		Fields: []table.Field{
			{Name: "CPU Utilization (%)", Values: []string{cpuUtil}},
			{Name: "IPC", Values: []string{ipc}},
			{Name: "C6 Core Residency (%)", Values: []string{c6}},
			{Name: "Core Frequency (MHz)", Values: []string{avgCoreFreq}},
			{Name: "Package Power (Watts)", Values: []string{pkgPower}},
			{Name: "Package Temperature (C)", Values: []string{pkgTemperature}},
			{Name: "Memory Available (kB)", Values: []string{memAvail}},
			{Name: "Drive Reads (kB/s)", Values: []string{driveReads}},
			{Name: "Drive Writes (kB/s)", Values: []string{driveWrites}},
			{Name: "Network RX (kB/s)", Values: []string{networkReads}},
			{Name: "Network TX (kB/s)", Values: []string{networkWrites}},
		},
	}
}

func getMetricAverage(tableValues table.TableValues, fieldNames []string, separatorFieldName string) (average string) {
	if len(tableValues.Fields) == 0 {
		return ""
	}
	sum, seps, err := getSumOfFields(tableValues.Fields, fieldNames, separatorFieldName)
	if err != nil {
		slog.Error("failed to get sum of fields for IO metrics", slog.String("error", err.Error()))
		return
	}
	if len(fieldNames) > 0 && seps > 0 {
		averageFloat := sum / float64(seps/len(fieldNames))
		p := message.NewPrinter(language.English) // use printer to get commas at thousands, e.g., Memory Available (kB)  258,691,376.80
		average = p.Sprintf("%0.2f", averageFloat)
	}
	return
}

func getFieldIndex(fields []table.Field, fieldName string) (int, error) {
	for i, field := range fields {
		if field.Name == fieldName {
			return i, nil
		}
	}
	return -1, fmt.Errorf("field not found: %s", fieldName)
}

func getSumOfFields(fields []table.Field, fieldNames []string, separatorFieldName string) (sum float64, numSeparators int, err error) {
	prevSeparator := ""
	var separatorIdx int
	if separatorFieldName != "" {
		separatorIdx, err = getFieldIndex(fields, separatorFieldName)
		if err != nil {
			return
		}
	}
	for _, fieldName := range fieldNames {
		var fieldIdx int
		fieldIdx, err = getFieldIndex(fields, fieldName)
		if err != nil {
			return
		}
		for i := range fields[fieldIdx].Values {
			valueStr := fields[fieldIdx].Values[i]
			var valueFloat float64
			valueFloat, err = strconv.ParseFloat(valueStr, 64)
			if err != nil {
				return
			}
			if separatorFieldName != "" {
				separator := fields[separatorIdx].Values[i]
				if separator != prevSeparator {
					numSeparators++
					prevSeparator = separator
				}
			} else {
				numSeparators++
			}
			sum += valueFloat
		}
	}
	return
}

func getCPUAveragePercentage(tableValues table.TableValues, fieldName string, inverse bool) string {
	if len(tableValues.Fields) == 0 {
		return ""
	}
	var fieldIndex int
	var fv table.Field
	for fieldIndex, fv = range tableValues.Fields {
		if fv.Name == fieldName {
			break
		}
	}
	sum := 0.0
	for _, value := range tableValues.Fields[fieldIndex].Values {
		valueFloat, err := strconv.ParseFloat(value, 64)
		if err != nil {
			slog.Warn("failed to parse float value", slog.String("value", value), slog.String("error", err.Error()))
			return ""
		}
		sum += valueFloat
	}
	if sum != 0 {
		averageFloat := sum / float64(len(tableValues.Fields[fieldIndex].Values))
		if inverse {
			averageFloat = 100.0 - averageFloat
		}
		return fmt.Sprintf("%0.2f", averageFloat)
	}
	return ""
}

func getPkgAverageTemperature(allTableValues []table.TableValues) string {
	tableValues := getTableValues(allTableValues, TemperatureTelemetryTableName)
	// number of packages can vary, so we need to find the average temperature across all packages
	if len(tableValues.Fields) == 0 {
		return ""
	}
	pkgTempFieldIndices := make([]int, 0)
	for i, field := range tableValues.Fields {
		if strings.HasPrefix(field.Name, "Package") {
			pkgTempFieldIndices = append(pkgTempFieldIndices, i)
		}
	}
	if len(pkgTempFieldIndices) == 0 {
		return ""
	}
	sum := 0.0
	for _, fieldIndex := range pkgTempFieldIndices {
		for _, value := range tableValues.Fields[fieldIndex].Values {
			valueFloat, err := strconv.ParseFloat(value, 64)
			if err != nil {
				slog.Warn("failed to parse float value", slog.String("value", value), slog.String("error", err.Error()))
				return ""
			}
			sum += valueFloat
		}
	}
	if sum != 0 {
		averageFloat := sum / float64(len(pkgTempFieldIndices)) / float64(len(tableValues.Fields[pkgTempFieldIndices[0]].Values))
		return fmt.Sprintf("%0.2f", averageFloat)
	}
	return ""
}

func getPkgAveragePower(allTableValues []table.TableValues) string {
	tableValues := getTableValues(allTableValues, PowerTelemetryTableName)
	// number of packages can vary, so we need to find the average power across all packages
	if len(tableValues.Fields) == 0 {
		return ""
	}
	pkgPowerFieldIndices := make([]int, 0)
	for i, field := range tableValues.Fields {
		if strings.HasPrefix(field.Name, "Package") {
			pkgPowerFieldIndices = append(pkgPowerFieldIndices, i)
		}
	}
	if len(pkgPowerFieldIndices) == 0 {
		return ""
	}
	sum := 0.0
	for _, fieldIndex := range pkgPowerFieldIndices {
		for _, value := range tableValues.Fields[fieldIndex].Values {
			valueFloat, err := strconv.ParseFloat(value, 64)
			if err != nil {
				slog.Warn("failed to parse float value", slog.String("value", value), slog.String("error", err.Error()))
				return ""
			}
			sum += valueFloat
		}
	}
	if sum != 0 {
		averageFloat := sum / float64(len(pkgPowerFieldIndices)) / float64(len(tableValues.Fields[pkgPowerFieldIndices[0]].Values))
		return fmt.Sprintf("%0.2f", averageFloat)
	}
	return ""
}
