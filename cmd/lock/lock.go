// Package lock is a subcommand of the root command. It is used to collect kernel lock related perf information from target(s).
package lock

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"path/filepath"
	"perfspect/internal/common"
	"perfspect/internal/progress"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/table"
	"perfspect/internal/target"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const cmdName = "lock"

var examples = []string{
	fmt.Sprintf("  Lock inspect from local host:       $ %s %s", common.AppName, cmdName),
	fmt.Sprintf("  Lock inspect from remote target:    $ %s %s --target 192.168.1.1 --user fred --key fred_key", common.AppName, cmdName),
	fmt.Sprintf("  Lock inspect from multiple targets: $ %s %s --targets targets.yaml", common.AppName, cmdName),
}

var Cmd = &cobra.Command{
	Use:           cmdName,
	Short:         "Collect system information for kernel lock analysis from target(s)",
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
	flagPackage         bool
	flagFormat          []string
	flagNoSystemSummary bool
)

const (
	flagDurationName        = "duration"
	flagFrequencyName       = "frequency"
	flagPackageName         = "package"
	flagNoSystemSummaryName = "no-summary"
)

func init() {
	Cmd.Flags().StringVar(&common.FlagInput, common.FlagInputName, "", "")
	Cmd.Flags().StringSliceVar(&flagFormat, common.FlagFormatName, []string{report.FormatAll}, "")
	Cmd.Flags().IntVar(&flagDuration, flagDurationName, 10, "")
	Cmd.Flags().IntVar(&flagFrequency, flagFrequencyName, 11, "")
	Cmd.PersistentFlags().BoolVar(&flagPackage, flagPackageName, false, "")
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
			Name: flagDurationName,
			Help: "number of seconds to run the collection",
		},
		{
			Name: flagFrequencyName,
			Help: "number of samples taken per second",
		},
		{
			Name: flagPackageName,
			Help: "create raw data package",
		},
		{
			Name: common.FlagFormatName,
			Help: fmt.Sprintf("choose output format(s) from: %s", strings.Join(append([]string{report.FormatAll}, report.FormatHtml, report.FormatTxt), ", ")),
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

	return groups
}

func validateFlags(cmd *cobra.Command, args []string) error {
	// validate format options
	formatOptions := append([]string{report.FormatAll}, report.FormatHtml, report.FormatTxt)
	for _, format := range flagFormat {
		if !slices.Contains(formatOptions, format) {
			return common.FlagValidationError(cmd, fmt.Sprintf("format options are: %s", strings.Join(formatOptions, ", ")))
		}
	}
	if flagDuration <= 0 {
		return common.FlagValidationError(cmd, "duration must be greater than 0")
	}
	if flagFrequency <= 0 {
		return common.FlagValidationError(cmd, "frequency must be greater than 0")
	}
	// common target flags
	if err := common.ValidateTargetFlags(cmd); err != nil {
		return common.FlagValidationError(cmd, err.Error())
	}
	return nil
}

func formalizeOutputFormat(outputFormat []string) []string {
	result := []string{}
	for _, format := range outputFormat {
		switch format {
		case report.FormatAll:
			return []string{report.FormatHtml, report.FormatTxt}
		case report.FormatTxt:
			result = append(result, report.FormatTxt)
		case report.FormatHtml:
			result = append(result, report.FormatHtml)
		}
	}

	return result
}

func pullDataFiles(appContext common.AppContext, scriptOutputs map[string]script.ScriptOutput, myTarget target.Target, statusUpdate progress.MultiSpinnerUpdateFunc) error {
	localOutputDir := appContext.OutputDir
	tableValues := table.GetValuesForTable(table.KernelLockAnalysisTableName, scriptOutputs)
	found := false
	for _, field := range tableValues.Fields {
		if field.Name == "Perf Package Path" {
			for _, remoteFile := range field.Values {
				if remoteFile == "" {
					continue
				}
				found = true
				_ = statusUpdate(myTarget.GetName(), "retrieving lock package")
				err := myTarget.PullFile(remoteFile, localOutputDir)
				if err != nil {
					_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("failed to retrieve lock package: %v", err))
					return err
				}
				_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("retrieved lock package (%s)", filepath.Base(remoteFile)))
			}
			break
		}
	}
	if !found {
		_ = statusUpdate(myTarget.GetName(), "no lock package found")
	}
	return nil
}

func runCmd(cmd *cobra.Command, args []string) error {
	var tableNames []string
	if !flagNoSystemSummary {
		tableNames = append(tableNames, table.BriefSysSummaryTableName)
	}
	tableNames = append(tableNames, table.KernelLockAnalysisTableName)
	reportingCommand := common.ReportingCommand{
		Cmd:            cmd,
		ReportNamePost: "lock",
		ScriptParams: map[string]string{
			"Frequency": strconv.Itoa(flagFrequency),
			"Duration":  strconv.Itoa(flagDuration),
			"Package":   strconv.FormatBool(flagPackage),
		},
		TableNames: tableNames,
	}

	// only try to download package when option specified
	if flagPackage {
		reportingCommand.AdhocFunc = pullDataFiles
	}

	// The common.FlagFormat designed to hold the output formats, but as a global variable,
	// it would be overwrite by other command's initialization function. So the current
	// workaround is to make an assignment to ensure the current command's output format
	// flag takes effect as expected.
	common.FlagFormat = formalizeOutputFormat(flagFormat)
	return reportingCommand.Run()
}
