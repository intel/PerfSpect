// Package flame is a subcommand of the root command. It is used to generate flamegraphs from target(s).
package flame

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"os"
	"perfspect/internal/common"
	"perfspect/internal/report"
	"perfspect/internal/table"
	"perfspect/internal/util"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const cmdName = "flame"

var examples = []string{
	fmt.Sprintf("  Flamegraph from local host:       $ %s %s", common.AppName, cmdName),
	fmt.Sprintf("  Flamegraph from remote target:    $ %s %s --target 192.168.1.1 --user fred --key fred_key", common.AppName, cmdName),
	fmt.Sprintf("  Flamegraph from multiple targets: $ %s %s --targets targets.yaml", common.AppName, cmdName),
}

var Cmd = &cobra.Command{
	Use:           cmdName,
	Short:         "Generate flamegraphs from target(s)",
	Long:          "",
	Example:       strings.Join(examples, "\n"),
	RunE:          runCmd,
	PreRunE:       validateFlags,
	GroupID:       "primary",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
}

var (
	flagDuration        int
	flagFrequency       int
	flagPids            []int
	flagNoSystemSummary bool
	flagMaxDepth        int
)

const (
	flagDurationName        = "duration"
	flagFrequencyName       = "frequency"
	flagPidsName            = "pids"
	flagNoSystemSummaryName = "no-summary"
	flagMaxDepthName        = "max-depth"
)

func init() {
	Cmd.Flags().StringVar(&common.FlagInput, common.FlagInputName, "", "")
	Cmd.Flags().StringSliceVar(&common.FlagFormat, common.FlagFormatName, []string{report.FormatAll}, "")
	Cmd.Flags().IntVar(&flagDuration, flagDurationName, 30, "")
	Cmd.Flags().IntVar(&flagFrequency, flagFrequencyName, 11, "")
	Cmd.Flags().IntSliceVar(&flagPids, flagPidsName, nil, "")
	Cmd.Flags().BoolVar(&flagNoSystemSummary, flagNoSystemSummaryName, false, "")
	Cmd.Flags().IntVar(&flagMaxDepth, flagMaxDepthName, 0, "")

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
			Name: flagDurationName,
			Help: "number of seconds to run the collection",
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
			Name: common.FlagFormatName,
			Help: fmt.Sprintf("choose output format(s) from: %s", strings.Join(append([]string{report.FormatAll}, report.FormatHtml, report.FormatTxt, report.FormatJson), ", ")),
		},
		{
			Name: flagMaxDepthName,
			Help: "maximum render depth of call stack in flamegraph (0 = no limit)",
		},
		{
			Name: flagNoSystemSummaryName,
			Help: "do not include system summary table in report",
		},
	}
	groups = append(groups, common.FlagGroup{
		GroupName: "Options",
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
	// validate format options
	for _, format := range common.FlagFormat {
		formatOptions := append([]string{report.FormatAll}, report.FormatHtml, report.FormatTxt, report.FormatJson)
		if !slices.Contains(formatOptions, format) {
			return common.FlagValidationError(cmd, fmt.Sprintf("format options are: %s", strings.Join(formatOptions, ", ")))
		}
	}
	// validate input file
	if common.FlagInput != "" {
		if _, err := os.Stat(common.FlagInput); os.IsNotExist(err) {
			return common.FlagValidationError(cmd, fmt.Sprintf("input file %s does not exist", common.FlagInput))
		}
	}
	if flagDuration <= 0 {
		return common.FlagValidationError(cmd, "duration must be greater than 0")
	}
	if flagFrequency <= 0 {
		return common.FlagValidationError(cmd, "frequency must be greater than 0")
	}
	for _, pid := range flagPids {
		if pid < 0 {
			return common.FlagValidationError(cmd, "PID must be greater than or equal to 0")
		}
	}
	if flagMaxDepth < 0 {
		return common.FlagValidationError(cmd, "max depth must be greater than or equal to 0")
	}
	// common target flags
	if err := common.ValidateTargetFlags(cmd); err != nil {
		return common.FlagValidationError(cmd, err.Error())
	}
	return nil
}

func runCmd(cmd *cobra.Command, args []string) error {
	var tables []table.TableDefinition
	if !flagNoSystemSummary {
		tables = append(tables, common.TableDefinitions[common.BriefSysSummaryTableName])
	}
	tables = append(tables, tableDefinitions[CallStackFrequencyTableName])
	reportingCommand := common.ReportingCommand{
		Cmd:            cmd,
		ReportNamePost: "flame",
		ScriptParams: map[string]string{
			"Frequency": strconv.Itoa(flagFrequency),
			"Duration":  strconv.Itoa(flagDuration),
			"PIDs":      strings.Join(util.IntSliceToStringSlice(flagPids), ","),
			"MaxDepth":  strconv.Itoa(flagMaxDepth),
		},
		Tables: tables,
	}

	report.RegisterHTMLRenderer(CallStackFrequencyTableName, callStackFrequencyTableHTMLRenderer)

	return reportingCommand.Run()
}
