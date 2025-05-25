// Package telemetry is a subcommand of the root command. It collects system telemetry from target(s).
package telemetry

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"perfspect/internal/common"
	"perfspect/internal/report"
	"perfspect/internal/script"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const cmdName = "telemetry"

var examples = []string{
	fmt.Sprintf("  Telemetry from local host:       $ %s %s", common.AppName, cmdName),
	fmt.Sprintf("  Telemetry from remote target:    $ %s %s --target 192.168.1.1 --user fred --key fred_key", common.AppName, cmdName),
	fmt.Sprintf("  Memory telemetry for 60 seconds: $ %s %s --memory --duration 60", common.AppName, cmdName),
	fmt.Sprintf("  Telemetry from multiple targets: $ %s %s --targets targets.yaml", common.AppName, cmdName),
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

	flagCpu         bool
	flagFrequency   bool
	flagIRQRate     bool
	flagMemory      bool
	flagNetwork     bool
	flagStorage     bool
	flagPower       bool
	flagTemperature bool
	flagInstrMix    bool
	flagGaudi       bool

	flagNoSystemSummary bool

	flagInstrMixPid    int
	flagInstrMixFilter []string
)

const (
	flagDurationName = "duration"
	flagIntervalName = "interval"

	flagAllName = "all"

	flagCpuName         = "cpu"
	flagFrequencyName   = "frequency"
	flagIRQRateName     = "irqrate"
	flagMemoryName      = "memory"
	flagNetworkName     = "network"
	flagStorageName     = "storage"
	flagPowerName       = "power"
	flagTemperatureName = "temperature"
	flagInstrMixName    = "instrmix"
	flagGaudiName       = "gaudi"

	flagNoSystemSummaryName = "no-summary"

	flagInstrMixPidName    = "instrmix-pid"
	flagInstrMixFilterName = "instrmix-filter"
)

var telemetrySummaryTableName = "Telemetry Summary"

var categories = []common.Category{
	{FlagName: flagCpuName, FlagVar: &flagCpu, DefaultValue: false, Help: "monitor cpu utilization", TableNames: []string{report.CPUUtilizationTelemetryTableName, report.UtilizationCategoriesTelemetryTableName}},
	{FlagName: flagFrequencyName, FlagVar: &flagFrequency, DefaultValue: false, Help: "monitor cpu frequency", TableNames: []string{report.FrequencyTelemetryTableName}},
	{FlagName: flagPowerName, FlagVar: &flagPower, DefaultValue: false, Help: "monitor power", TableNames: []string{report.PowerTelemetryTableName}},
	{FlagName: flagTemperatureName, FlagVar: &flagTemperature, DefaultValue: false, Help: "monitor temperature", TableNames: []string{report.TemperatureTelemetryTableName}},
	{FlagName: flagMemoryName, FlagVar: &flagMemory, DefaultValue: false, Help: "monitor memory", TableNames: []string{report.MemoryTelemetryTableName}},
	{FlagName: flagNetworkName, FlagVar: &flagNetwork, DefaultValue: false, Help: "monitor network", TableNames: []string{report.NetworkTelemetryTableName}},
	{FlagName: flagStorageName, FlagVar: &flagStorage, DefaultValue: false, Help: "monitor storage", TableNames: []string{report.DriveTelemetryTableName}},
	{FlagName: flagIRQRateName, FlagVar: &flagIRQRate, DefaultValue: false, Help: "monitor IRQ rate", TableNames: []string{report.IRQRateTelemetryTableName}},
	{FlagName: flagInstrMixName, FlagVar: &flagInstrMix, DefaultValue: false, Help: "monitor instruction mix", TableNames: []string{report.InstructionTelemetryTableName}},
	{FlagName: flagGaudiName, FlagVar: &flagGaudi, DefaultValue: false, Help: "monitor gaudi", TableNames: []string{report.GaudiTelemetryTableName}},
}

func init() {
	// set up config category flags
	for _, cat := range categories {
		Cmd.Flags().BoolVar(cat.FlagVar, cat.FlagName, cat.DefaultValue, cat.Help)
	}
	Cmd.Flags().StringVar(&common.FlagInput, common.FlagInputName, "", "")
	Cmd.Flags().BoolVar(&flagAll, flagAllName, false, "")
	Cmd.Flags().StringSliceVar(&common.FlagFormat, common.FlagFormatName, []string{report.FormatAll}, "")
	Cmd.Flags().IntVar(&flagDuration, flagDurationName, 30, "")
	Cmd.Flags().IntVar(&flagInterval, flagIntervalName, 2, "")
	Cmd.Flags().IntVar(&flagInstrMixPid, flagInstrMixPidName, 0, "")
	Cmd.Flags().StringSliceVar(&flagInstrMixFilter, flagInstrMixFilterName, []string{"SSE", "AVX", "AVX2", "AVX512", "AMX_TILE"}, "")
	Cmd.Flags().BoolVar(&flagNoSystemSummary, flagNoSystemSummaryName, false, "")

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
			Help: "collect telemetry for all categories",
		},
	}
	for _, cat := range categories {
		flags = append(flags, common.Flag{
			Name: cat.FlagName,
			Help: cat.Help,
		})
	}
	groups = append(groups, common.FlagGroup{
		GroupName: "Categories",
		Flags:     flags,
	})
	flags = []common.Flag{
		{
			Name: common.FlagFormatName,
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
			Help: "pid to monitor for instruction mix, no pid means all processes",
		},
		{
			Name: flagInstrMixFilterName,
			Help: "filter to apply to instruction mix",
		},
		{
			Name: flagNoSystemSummaryName,
			Help: "do not include system summary table in report",
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
	// set flagAll if all categories are selected or if none are selected
	if !flagAll {
		numCategoriesTrue := 0
		for _, cat := range categories {
			if *cat.FlagVar {
				numCategoriesTrue++
				break
			}
		}
		if numCategoriesTrue == len(categories) || numCategoriesTrue == 0 {
			flagAll = true
		}
	}
	// validate format options
	for _, format := range common.FlagFormat {
		formatOptions := []string{report.FormatAll}
		formatOptions = append(formatOptions, report.FormatOptions...)
		if !slices.Contains(formatOptions, format) {
			return common.FlagValidationError(cmd, fmt.Sprintf("format options are: %s", strings.Join(formatOptions, ", ")))
		}
	}
	if flagInterval < 1 {
		return common.FlagValidationError(cmd, "interval must be 1 or greater")
	}
	if flagDuration < 0 {
		return common.FlagValidationError(cmd, "duration must be 0 or greater")
	}
	target, err := cmd.Flags().GetString("target")
	if err != nil {
		panic("failed to get target flag")
	}
	targets, err := cmd.Flags().GetString("targets")
	if err != nil {
		panic("failed to get targets flag")
	}
	if flagDuration == 0 && (target != "" || targets != "") {
		return common.FlagValidationError(cmd, "duration must be greater than 0 when collecting from a remote target")
	}
	if cmd.Flags().Lookup(flagInstrMixFilterName).Changed {
		re := regexp.MustCompile("^[A-Z0-9_]+$")
		for _, filter := range flagInstrMixFilter {
			if !re.MatchString(filter) {
				return common.FlagValidationError(cmd, fmt.Sprintf("invalid filter: %s, must be uppercase letters, numbers, and underscores", filter))
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
	var tableNames []string
	if !flagNoSystemSummary {
		tableNames = append(tableNames, report.BriefSysSummaryTableName)
	}
	for _, cat := range categories {
		if *cat.FlagVar || flagAll {
			tableNames = append(tableNames, cat.TableNames...)
		}
	}
	// include telemetry summary table if all telemetry options are selected
	var summaryFunc common.SummaryFunc
	if flagAll {
		summaryFunc = summaryFromTableValues
	}
	// include insights table if all categories are selected
	var insightsFunc common.InsightsFunc
	if flagAll {
		insightsFunc = common.DefaultInsightsFunc
	}
	reportingCommand := common.ReportingCommand{
		Cmd:            cmd,
		ReportNamePost: "telem",
		ScriptParams: map[string]string{
			"Interval": strconv.Itoa(flagInterval),
			"Duration": strconv.Itoa(flagDuration),
			"PID":      strconv.Itoa(flagInstrMixPid),
			"Filter":   strings.Join(flagInstrMixFilter, " "),
		},
		TableNames:       tableNames,
		SummaryFunc:      summaryFunc,
		SummaryTableName: telemetrySummaryTableName,
		InsightsFunc:     insightsFunc,
	}
	return reportingCommand.Run()
}

func getTableValues(allTableValues []report.TableValues, tableName string) report.TableValues {
	for _, tv := range allTableValues {
		if tv.Name == tableName {
			return tv
		}
	}
	return report.TableValues{}
}

func summaryFromTableValues(allTableValues []report.TableValues, _ map[string]script.ScriptOutput) report.TableValues {
	cpuUtil := getCPUAveragePercentage(getTableValues(allTableValues, report.UtilizationCategoriesTelemetryTableName), "%idle", true)
	avgCoreFreq := getMetricAverage(getTableValues(allTableValues, report.FrequencyTelemetryTableName), []string{"Core (Avg.)"}, "Time")
	pkgPower := getPkgAveragePower(allTableValues)
	dramPower := getDramAveragePower(allTableValues)
	pkgTemperature := getPkgAverageTemperature(allTableValues)
	coreTemperature := getMetricAverage(getTableValues(allTableValues, report.TemperatureTelemetryTableName), []string{"Core (Avg.)"}, "")
	driveReads := getMetricAverage(getTableValues(allTableValues, report.DriveTelemetryTableName), []string{"kB_read/s"}, "Device")
	driveWrites := getMetricAverage(getTableValues(allTableValues, report.DriveTelemetryTableName), []string{"kB_wrtn/s"}, "Device")
	networkReads := getMetricAverage(getTableValues(allTableValues, report.NetworkTelemetryTableName), []string{"rxkB/s"}, "Time")
	networkWrites := getMetricAverage(getTableValues(allTableValues, report.NetworkTelemetryTableName), []string{"txkB/s"}, "Time")
	memAvail := getMetricAverage(getTableValues(allTableValues, report.MemoryTelemetryTableName), []string{"avail"}, "Time")
	return report.TableValues{
		TableDefinition: report.TableDefinition{
			Name:      telemetrySummaryTableName,
			HasRows:   false,
			MenuLabel: telemetrySummaryTableName,
		},
		Fields: []report.Field{
			{Name: "CPU Utilization (%)", Values: []string{cpuUtil}},
			{Name: "Core Frequency (MHz)", Values: []string{avgCoreFreq}},
			{Name: "Package Power (Watts)", Values: []string{pkgPower}},
			{Name: "DRAM Power (Watts)", Values: []string{dramPower}},
			{Name: "Package Temperature (C)", Values: []string{pkgTemperature}},
			{Name: "Core Temperature (C)", Values: []string{coreTemperature}},
			{Name: "Memory Available (kB)", Values: []string{memAvail}},
			{Name: "Drive Reads (kB/s)", Values: []string{driveReads}},
			{Name: "Drive Writes (kB/s)", Values: []string{driveWrites}},
			{Name: "Network RX (kB/s)", Values: []string{networkReads}},
			{Name: "Network TX (kB/s)", Values: []string{networkWrites}},
		},
	}
}

func getMetricAverage(tableValues report.TableValues, fieldNames []string, separatorFieldName string) (average string) {
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

func getFieldIndex(fields []report.Field, fieldName string) (int, error) {
	for i, field := range fields {
		if field.Name == fieldName {
			return i, nil
		}
	}
	return -1, fmt.Errorf("field not found: %s", fieldName)
}

func getSumOfFields(fields []report.Field, fieldNames []string, separatorFieldName string) (sum float64, numSeparators int, err error) {
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

func getCPUAveragePercentage(tableValues report.TableValues, fieldName string, inverse bool) string {
	if len(tableValues.Fields) == 0 {
		return ""
	}
	var fieldIndex int
	var fv report.Field
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

func getPkgAverageTemperature(allTableValues []report.TableValues) string {
	tableValues := getTableValues(allTableValues, report.TemperatureTelemetryTableName)
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

func getPkgAveragePower(allTableValues []report.TableValues) string {
	tableValues := getTableValues(allTableValues, report.PowerTelemetryTableName)
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

func getDramAveragePower(allTableValues []report.TableValues) string {
	tableValues := getTableValues(allTableValues, report.PowerTelemetryTableName)
	// number of DRAMs can vary, so we need to find the average power across all DRAMs
	if len(tableValues.Fields) == 0 {
		return ""
	}
	dramPowerFieldIndices := make([]int, 0)
	for i, field := range tableValues.Fields {
		if strings.HasPrefix(field.Name, "DRAM") {
			dramPowerFieldIndices = append(dramPowerFieldIndices, i)
		}
	}
	if len(dramPowerFieldIndices) == 0 {
		return ""
	}
	sum := 0.0
	for _, fieldIndex := range dramPowerFieldIndices {
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
		averageFloat := sum / float64(len(dramPowerFieldIndices)) / float64(len(tableValues.Fields[dramPowerFieldIndices[0]].Values))
		return fmt.Sprintf("%0.2f", averageFloat)
	}
	return ""
}
