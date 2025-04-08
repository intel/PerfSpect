// Package telemetry is a subcommand of the root command. It collects system telemetry from target(s).
package telemetry

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"os"
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

	flagCpu      bool
	flagMemory   bool
	flagNetwork  bool
	flagStorage  bool
	flagPower    bool
	flagInstrMix bool
	flagGaudi    bool

	flagInstrMixPid    int
	flagInstrMixFilter []string
)

const (
	flagDurationName = "duration"
	flagIntervalName = "interval"

	flagAllName = "all"

	flagCpuName      = "cpu"
	flagMemoryName   = "memory"
	flagNetworkName  = "network"
	flagStorageName  = "storage"
	flagPowerName    = "power"
	flagInstrMixName = "instrmix"
	flagGaudiName    = "gaudi"

	flagInstrMixPidName    = "instrmix-pid"
	flagInstrMixFilterName = "instrmix-filter"
)

var telemetrySummaryTableName = "Telemetry Summary"

var categories = []common.Category{
	{FlagName: flagCpuName, FlagVar: &flagCpu, DefaultValue: false, Help: "monitor cpu", TableNames: []string{report.CPUUtilizationTableName, report.SummaryCPUUtilizationTableName, report.SummaryCpuFreqTelemetryTableName, report.IRQRateTableName}},
	{FlagName: flagMemoryName, FlagVar: &flagMemory, DefaultValue: false, Help: "monitor memory", TableNames: []string{report.MemoryStatsTableName}},
	{FlagName: flagNetworkName, FlagVar: &flagNetwork, DefaultValue: false, Help: "monitor network", TableNames: []string{report.NetworkStatsTableName}},
	{FlagName: flagStorageName, FlagVar: &flagStorage, DefaultValue: false, Help: "monitor storage", TableNames: []string{report.DriveStatsTableName}},
	{FlagName: flagPowerName, FlagVar: &flagPower, DefaultValue: false, Help: "monitor power", TableNames: []string{report.PowerStatsTableName}},
	{FlagName: flagInstrMixName, FlagVar: &flagInstrMix, DefaultValue: false, Help: "monitor instruction mix", TableNames: []string{report.InstructionMixTableName}},
	{FlagName: flagGaudiName, FlagVar: &flagGaudi, DefaultValue: false, Help: "monitor gaudi", TableNames: []string{report.GaudiStatsTableName}},
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
	}
	groups = append(groups, common.FlagGroup{
		GroupName: "Others Options",
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
			err := fmt.Errorf("format options are: %s", strings.Join(formatOptions, ", "))
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
	}
	if flagDuration < 0 {
		err := fmt.Errorf("duration must be 0 or greater")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
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
		err := fmt.Errorf("duration must be greater than 0 when collecting from a remote target")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	if cmd.Flags().Lookup(flagInstrMixFilterName).Changed {
		re := regexp.MustCompile("^[A-Z0-9_]+$")
		for _, filter := range flagInstrMixFilter {
			if !re.MatchString(filter) {
				err := fmt.Errorf("invalid filter: %s, must be uppercase letters, numbers, and underscores", filter)
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}
		}
	}
	// common target flags
	if err := common.ValidateTargetFlags(cmd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	return nil
}

func runCmd(cmd *cobra.Command, args []string) error {
	var tableNames []string
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
	cpuUtil := getCPUAveragePercentage(getTableValues(allTableValues, report.SummaryCPUUtilizationTableName), "%idle", true)
	cpuFreq := getMetricAverage(getTableValues(allTableValues, report.SummaryCpuFreqTelemetryTableName), []string{"Frequency"}, "Time")
	pkgPower := getMetricAverage(getTableValues(allTableValues, report.PowerStatsTableName), []string{"Package"}, "")
	driveReads := getMetricAverage(getTableValues(allTableValues, report.DriveStatsTableName), []string{"kB_read/s"}, "Device")
	driveWrites := getMetricAverage(getTableValues(allTableValues, report.DriveStatsTableName), []string{"kB_wrtn/s"}, "Device")
	networkReads := getMetricAverage(getTableValues(allTableValues, report.NetworkStatsTableName), []string{"rxkB/s"}, "Time")
	networkWrites := getMetricAverage(getTableValues(allTableValues, report.NetworkStatsTableName), []string{"txkB/s"}, "Time")
	memAvail := getMetricAverage(getTableValues(allTableValues, report.MemoryStatsTableName), []string{"avail"}, "Time")
	return report.TableValues{
		TableDefinition: report.TableDefinition{
			Name:      telemetrySummaryTableName,
			HasRows:   false,
			MenuLabel: telemetrySummaryTableName,
		},
		Fields: []report.Field{
			{Name: "CPU Utilization (%)", Values: []string{cpuUtil}},
			{Name: "CPU Frequency (MHz)", Values: []string{cpuFreq}},
			{Name: "Package Power (Watts)", Values: []string{pkgPower}},
			{Name: "Drive Reads (kB/s)", Values: []string{driveReads}},
			{Name: "Drive Writes (kB/s)", Values: []string{driveWrites}},
			{Name: "Network RX (kB/s)", Values: []string{networkReads}},
			{Name: "Network TX (kB/s)", Values: []string{networkWrites}},
			{Name: "Memory Available (kB)", Values: []string{memAvail}},
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
