// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Package flamegraph is a subcommand of the root command. It is used to generate flamegraphs from target(s).
package flamegraph

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"perfspect/internal/app"
	"perfspect/internal/report"
	"perfspect/internal/table"
	"perfspect/internal/util"
	"perfspect/internal/workflow"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const cmdName = "flamegraph"

var examples = []string{
	fmt.Sprintf("  Flamegraph from local host:       $ %s %s", app.Name, cmdName),
	fmt.Sprintf("  Flamegraph from remote target:    $ %s %s --target 192.168.1.1 --user fred --key fred_key", app.Name, cmdName),
	fmt.Sprintf("  Flamegraph from multiple targets: $ %s %s --targets targets.yaml", app.Name, cmdName),
	fmt.Sprintf("  Flamegraph for cache misses:      $ %s %s --perf-event cache-misses", app.Name, cmdName),
}

var Cmd = &cobra.Command{
	Use:           cmdName,
	Aliases:       []string{"flame"},
	Short:         "Collect flamegraph data from target(s)",
	Long:          "",
	Example:       strings.Join(examples, "\n"),
	RunE:          runCmd,
	PreRunE:       validateFlags,
	GroupID:       "primary",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
}

var (
	flagInput           string
	flagFormat          []string
	flagDuration        int
	flagFrequency       int
	flagPids            []int
	flagNoSystemSummary bool
	flagMaxDepth        int
	flagPerfEvent       string
)

const (
	flagDurationName        = "duration"
	flagFrequencyName       = "frequency"
	flagPidsName            = "pids"
	flagNoSystemSummaryName = "no-summary"
	flagMaxDepthName        = "max-depth"
	flagPerfEventName       = "perf-event"
)

func init() {
	Cmd.Flags().StringVar(&flagInput, app.FlagInputName, "", "")
	Cmd.Flags().StringSliceVar(&flagFormat, app.FlagFormatName, []string{report.FormatHtml}, "")
	Cmd.Flags().IntVar(&flagDuration, flagDurationName, 0, "")
	Cmd.Flags().IntVar(&flagFrequency, flagFrequencyName, 11, "")
	Cmd.Flags().IntSliceVar(&flagPids, flagPidsName, nil, "")
	Cmd.Flags().BoolVar(&flagNoSystemSummary, flagNoSystemSummaryName, false, "")
	Cmd.Flags().IntVar(&flagMaxDepth, flagMaxDepthName, 0, "")
	Cmd.Flags().StringVar(&flagPerfEvent, flagPerfEventName, "cycles:P", "")

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
			Name: flagDurationName,
			Help: "number of seconds to run the collection. If 0, the collection will run indefinitely. Ctrl+c to stop.",
		},
		{
			Name: flagFrequencyName,
			Help: "number of samples taken per second",
		},
		{
			Name: flagPidsName,
			Help: "comma separated list of PIDs. If not specified, all PIDs will be collected",
		},
		{
			Name: flagPerfEventName,
			Help: "perf event to use for native sampling (e.g., cpu-cycles, instructions, cache-misses, branches, context-switches, mem-loads, mem-stores, etc.)",
		},
		{
			Name: flagMaxDepthName,
			Help: "maximum render depth of call stack in flamegraph (0 = no limit)",
		},
		{
			Name: app.FlagFormatName,
			Help: fmt.Sprintf("choose output format(s) from: %s", strings.Join(append([]string{report.FormatAll}, report.FormatHtml, report.FormatTxt, report.FormatJson), ", ")),
		},
		{
			Name: flagNoSystemSummaryName,
			Help: "do not include system summary table in report",
		},
	}
	groups = append(groups, app.FlagGroup{
		GroupName: "Options",
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
	// validate format options
	for _, format := range flagFormat {
		formatOptions := append([]string{report.FormatAll}, report.FormatHtml, report.FormatTxt, report.FormatJson)
		if !slices.Contains(formatOptions, format) {
			return workflow.FlagValidationError(cmd, fmt.Sprintf("format options are: %s", strings.Join(formatOptions, ", ")))
		}
	}
	// validate input file
	if flagInput != "" {
		if _, err := os.Stat(flagInput); os.IsNotExist(err) {
			return workflow.FlagValidationError(cmd, fmt.Sprintf("input file %s does not exist", flagInput))
		}
	}
	if flagDuration < 0 {
		return workflow.FlagValidationError(cmd, "duration must be 0 or greater")
	}
	if flagFrequency <= 0 {
		return workflow.FlagValidationError(cmd, "frequency must be 1 or greater")
	}
	for _, pid := range flagPids {
		if pid < 0 {
			return workflow.FlagValidationError(cmd, "PID must be 0 or greater")
		}
	}
	if flagMaxDepth < 0 {
		return workflow.FlagValidationError(cmd, "max depth must be 0 or greater")
	}
	// common target flags
	if err := workflow.ValidateTargetFlags(cmd); err != nil {
		return workflow.FlagValidationError(cmd, err.Error())
	}
	return nil
}

func runCmd(cmd *cobra.Command, args []string) error {
	var tables []table.TableDefinition
	if !flagNoSystemSummary {
		tables = append(tables, app.TableDefinitions[app.SystemSummaryTableName])
	}
	tables = append(tables, tableDefinitions[FlameGraphTableName])
	reportingCommand := workflow.ReportingCommand{
		Cmd:            cmd,
		ReportNamePost: "flame",
		ScriptParams: map[string]string{
			"Frequency": strconv.Itoa(flagFrequency),
			"Duration":  strconv.Itoa(flagDuration),
			"PIDs":      strings.Join(util.IntSliceToStringSlice(flagPids), ","),
			"MaxDepth":  strconv.Itoa(flagMaxDepth),
			"PerfEvent": flagPerfEvent,
		},
		Tables:  tables,
		Input:   flagInput,
		Formats: flagFormat,
	}

	report.RegisterHTMLRenderer(FlameGraphTableName, callStackFrequencyTableHTMLRenderer)

	return reportingCommand.Run()
}
