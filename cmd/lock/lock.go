// Package lock is a subcommand of the root command. It is used to collect kernel lock related perf information from target(s).
package lock

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"os"
	"perfspect/internal/common"
	"perfspect/internal/report"
	"perfspect/internal/script"
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
	flagDuration  int
	flagFrequency int
)

const (
	flagDurationName  = "duration"
	flagFrequencyName = "frequency"
)

func init() {
	Cmd.Flags().StringVar(&common.FlagInput, common.FlagInputName, "", "")
	Cmd.Flags().StringSliceVar(&common.FlagFormat, common.FlagFormatName, []string{report.FormatAll}, "")
	Cmd.Flags().IntVar(&flagDuration, flagDurationName, 10, "")
	Cmd.Flags().IntVar(&flagFrequency, flagFrequencyName, 11, "")

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
	}
	groups = append(groups, common.FlagGroup{
		GroupName: "Options",
		Flags:     flags,
	})
	groups = append(groups, common.GetTargetFlagGroup())

	return groups
}

func validateFlags(cmd *cobra.Command, args []string) error {
	if flagDuration <= 0 {
		err := fmt.Errorf("duration must be greater than 0")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// common target flags
	if err := common.ValidateTargetFlags(cmd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	return nil
}

func runCmd(cmd *cobra.Command, args []string) error {
	reportingCommand := common.ReportingCommand{
		Cmd:            cmd,
		ReportNamePost: "lock",
		ScriptParams:   script.ScriptParams{Frequency: flagFrequency, Duration: flagDuration},
		TableNames:     []string{report.KernelLockAnalysisTableName},
	}
	return reportingCommand.Run()
}
