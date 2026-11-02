// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Package instructions is a subcommand of the root command. It analyzes the
// sample-weighted disassembly of hot code on target(s) to report register pressure and
// other instruction-level characteristics.
package instructions

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

const cmdName = "instructions"

var examples = []string{
	fmt.Sprintf("  Instruction analysis of local host:    $ %s %s", app.Name, cmdName),
	fmt.Sprintf("  Instruction analysis of remote target: $ %s %s --target 192.168.1.1 --user fred --key fred_key", app.Name, cmdName),
	fmt.Sprintf("  Analyze specific processes:            $ %s %s --pids 1234,5678", app.Name, cmdName),
	fmt.Sprintf("  Register pressure only:                $ %s %s --register-pressure", app.Name, cmdName),
	fmt.Sprintf("  With an Intel APX benefit estimate:    $ %s %s --apx-estimate", app.Name, cmdName),
	fmt.Sprintf("  Java / JIT-compiled code:              $ %s %s --jit --pids $(pgrep -x java)", app.Name, cmdName),
}

var Cmd = &cobra.Command{
	Use:     cmdName,
	Aliases: []string{"insn"},
	Short:   "Analyze instructions in hot code on target(s)",
	Long: `Analyze the instructions in hot code on target(s).

Samples the target with perf, then disassembles the hottest symbols and classifies each
instruction. Reports register spill and reload traffic, split by general-purpose versus
vector registers.

Requires symbols in the profiled binaries. Stripped binaries cannot be annotated and are
reported as such rather than analyzed.`,
	Example:       strings.Join(examples, "\n"),
	RunE:          runCmd,
	PreRunE:       validateFlags,
	GroupID:       "primary",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
}

var (
	flagInput            string
	flagFormat           []string
	flagDuration         int
	flagFrequency        int
	flagPids             []int
	flagTopSymbols       int
	flagJobs             int
	flagIncludeKernel    bool
	flagJIT              bool
	flagSymfs            string
	flagNoSystemSummary  bool
	flagRegisterPressure bool
	flagAPXEstimate      bool
	flagAll              bool
)

const (
	flagDurationName         = "duration"
	flagFrequencyName        = "frequency"
	flagPidsName             = "pids"
	flagTopSymbolsName       = "top"
	flagJobsName             = "jobs"
	flagIncludeKernelName    = "kernel"
	flagJITName              = "jit"
	flagSymfsName            = "symfs"
	flagNoSystemSummaryName  = "no-summary"
	flagRegisterPressureName = "register-pressure"
	flagAPXEstimateName      = "apx-estimate"
	flagAllName              = "all"
)

// analyses maps a selection flag to the tables it enables, mirroring how the report
// command groups its categories.
var analyses = []app.Category{
	{
		FlagName: flagRegisterPressureName,
		FlagVar:  &flagRegisterPressure,
		Help:     "Register spill/reload pressure in hot symbols",
		Tables: []table.TableDefinition{
			tableDefinitions[RegisterPressureTableName],
		},
	},
}

func init() {
	for _, a := range analyses {
		Cmd.Flags().BoolVar(a.FlagVar, a.FlagName, a.DefaultValue, a.Help)
	}
	// Not part of --all: this is a projection from a heuristic model rather than an
	// observation, so it is opt-in, matching the reference tool's default.
	Cmd.Flags().BoolVar(&flagAPXEstimate, flagAPXEstimateName, false, "")
	Cmd.Flags().BoolVar(&flagAll, flagAllName, true, "")
	Cmd.Flags().StringVar(&flagInput, app.FlagInputName, "", "")
	Cmd.Flags().StringSliceVar(&flagFormat, app.FlagFormatName, []string{report.FormatAll}, "")
	Cmd.Flags().IntVar(&flagDuration, flagDurationName, 10, "")
	// 500 Hz follows the reference implementation. Precise attribution matters here: too
	// sparse a profile makes the per-symbol instruction weights unreliable.
	Cmd.Flags().IntVar(&flagFrequency, flagFrequencyName, 500, "")
	Cmd.Flags().IntSliceVar(&flagPids, flagPidsName, nil, "")
	Cmd.Flags().IntVar(&flagTopSymbols, flagTopSymbolsName, 50, "")
	Cmd.Flags().IntVar(&flagJobs, flagJobsName, 4, "")
	Cmd.Flags().BoolVar(&flagIncludeKernel, flagIncludeKernelName, false, "")
	Cmd.Flags().BoolVar(&flagJIT, flagJITName, false, "")
	Cmd.Flags().StringVar(&flagSymfs, flagSymfsName, "", "")
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

	analysisFlags := []app.Flag{{Name: flagAllName, Help: "run all analyses"}}
	for _, a := range analyses {
		analysisFlags = append(analysisFlags, app.Flag{Name: a.FlagName, Help: a.Help})
	}
	analysisFlags = append(analysisFlags, app.Flag{
		Name: flagAPXEstimateName,
		Help: "estimate the benefit of Intel APX (heuristic projection, not a measurement)",
	})
	groups = append(groups, app.FlagGroup{GroupName: "Analyses", Flags: analysisFlags})

	groups = append(groups, app.FlagGroup{
		GroupName: "Options",
		Flags: []app.Flag{
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
				Help: "comma separated list of PIDs. If not specified, the whole system is sampled",
			},
			{
				Name: flagTopSymbolsName,
				Help: "number of hottest symbols to disassemble and analyze",
			},
			{
				Name: flagJobsName,
				Help: "number of parallel disassembly workers",
			},
			{
				Name: flagIncludeKernelName,
				Help: "include kernel symbols (userspace only by default)",
			},
			{
				Name: flagJITName,
				Help: "analyze JIT-compiled code (Java, Node.js). The runtime must emit a jitdump; for the JVM add -agentpath:<path>/libperf-jvmti.so",
			},
			{
				Name: app.FlagFormatName,
				Help: fmt.Sprintf("choose output format(s) from: %s", strings.Join(append([]string{report.FormatAll}, report.FormatHtml, report.FormatTxt, report.FormatJson), ", ")),
			},
			{
				Name: flagNoSystemSummaryName,
				Help: "do not include system summary table in report",
			},
		},
	})
	groups = append(groups, workflow.GetTargetFlagGroup())
	groups = append(groups, app.FlagGroup{
		GroupName: "Advanced Options",
		Flags: []app.Flag{
			{
				Name: flagSymfsName,
				Help: "filesystem root to resolve binaries under. Needed when profiling containers from the host, where the recorded paths do not exist: pass the container's merged overlay directory",
			},
			{
				Name: app.FlagInputName,
				Help: "\".raw\" file, or directory containing \".raw\" files. Will skip data collection and use raw data for reports.",
			},
		},
	})
	return groups
}

func validateFlags(cmd *cobra.Command, args []string) error {
	formatOptions := append([]string{report.FormatAll}, report.FormatHtml, report.FormatTxt, report.FormatJson)
	for _, format := range flagFormat {
		if !slices.Contains(formatOptions, format) {
			return workflow.FlagValidationError(cmd, fmt.Sprintf("format options are: %s", strings.Join(formatOptions, ", ")))
		}
	}
	if flagInput != "" {
		if _, err := os.Stat(flagInput); os.IsNotExist(err) {
			return workflow.FlagValidationError(cmd, fmt.Sprintf("input file %s does not exist", flagInput))
		}
	}
	if flagDuration <= 0 {
		return workflow.FlagValidationError(cmd, "duration must be greater than 0")
	}
	if flagFrequency <= 0 {
		return workflow.FlagValidationError(cmd, "frequency must be greater than 0")
	}
	if flagTopSymbols <= 0 {
		return workflow.FlagValidationError(cmd, "top must be greater than 0")
	}
	if flagJobs <= 0 {
		return workflow.FlagValidationError(cmd, "jobs must be greater than 0")
	}
	for _, pid := range flagPids {
		if pid < 0 {
			return workflow.FlagValidationError(cmd, "PID must be 0 or greater")
		}
	}
	if err := workflow.ValidateTargetFlags(cmd); err != nil {
		return workflow.FlagValidationError(cmd, err.Error())
	}
	return nil
}

// selectedTables resolves the analysis flags to the tables to include. Naming any analysis
// explicitly turns off the implicit --all.
func selectedTables() []table.TableDefinition {
	var explicit bool
	for _, a := range analyses {
		if *a.FlagVar {
			explicit = true
			break
		}
	}

	var tables []table.TableDefinition
	if !flagNoSystemSummary {
		tables = append(tables, app.TableDefinitions[app.SystemSummaryTableName])
	}
	for _, a := range analyses {
		if !explicit || *a.FlagVar {
			tables = append(tables, a.Tables...)
		}
	}
	if flagAPXEstimate {
		tables = append(tables, tableDefinitions[APXEstimateTableName])
	}
	tables = append(tables, tableDefinitions[CollectionInfoTableName])
	return tables
}

func runCmd(cmd *cobra.Command, args []string) error {
	reportingCommand := workflow.ReportingCommand{
		Cmd:            cmd,
		ReportNamePost: "insn",
		ScriptParams: map[string]string{
			"Duration":      strconv.Itoa(flagDuration),
			"Frequency":     strconv.Itoa(flagFrequency),
			"PIDs":          strings.Join(util.IntSliceToStringSlice(flagPids), ","),
			"TopSymbols":    strconv.Itoa(flagTopSymbols),
			"Jobs":          strconv.Itoa(flagJobs),
			"IncludeKernel": strconv.FormatBool(flagIncludeKernel),
			"JIT":           strconv.FormatBool(flagJIT),
			"Symfs":         flagSymfs,
		},
		Tables:  selectedTables(),
		Input:   flagInput,
		Formats: flagFormat,
	}

	report.RegisterHTMLRenderer(RegisterPressureTableName, registerPressureHTMLRenderer)

	return reportingCommand.Run()
}
