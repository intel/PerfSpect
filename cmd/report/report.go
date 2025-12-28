// Package report is a subcommand of the root command. It generates a configuration report for target(s).
package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"perfspect/internal/app"
	"perfspect/internal/report"
	"perfspect/internal/table"
	"perfspect/internal/workflow"
)

const cmdName = "report"

var examples = []string{
	fmt.Sprintf("  Data from local host:          $ %s %s", app.Name, cmdName),
	fmt.Sprintf("  Specific data from local host: $ %s %s --bios --os --cpu --format html,json", app.Name, cmdName),
	fmt.Sprintf("  All data from remote target:   $ %s %s --target 192.168.1.1 --user fred --key fred_key", app.Name, cmdName),
	fmt.Sprintf("  Data from multiple targets:    $ %s %s --targets targets.yaml", app.Name, cmdName),
}

var Cmd = &cobra.Command{
	Use:           cmdName,
	Short:         "Collect configuration data from target(s)",
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
	// categories
	flagSystemSummary  bool
	flagHost           bool
	flagPcie           bool
	flagBios           bool
	flagOs             bool
	flagSoftware       bool
	flagCpu            bool
	flagPrefetcher     bool
	flagIsa            bool
	flagAccelerator    bool
	flagPower          bool
	flagCstates        bool
	flagFrequency      bool
	flagUncore         bool
	flagElc            bool
	flagSST            bool
	flagMemory         bool
	flagDimm           bool
	flagNic            bool
	flagNetConfig      bool
	flagDisk           bool
	flagFilesystem     bool
	flagGpu            bool
	flagGaudi          bool
	flagCxl            bool
	flagCve            bool
	flagProcess        bool
	flagSensor         bool
	flagChassisStatus  bool
	flagPmu            bool
	flagSystemEventLog bool
	flagKernelLog      bool
)

// flag names
const (
	flagAllName = "all"
	// categories
	flagSystemSummaryName  = "system-summary"
	flagHostName           = "host"
	flagPcieName           = "pcie"
	flagBiosName           = "bios"
	flagOsName             = "os"
	flagSoftwareName       = "software"
	flagCpuName            = "cpu"
	flagPrefetcherName     = "prefetcher"
	flagIsaName            = "isa"
	flagAcceleratorName    = "accelerator"
	flagPowerName          = "power"
	flagCstatesName        = "cstates"
	flagFrequencyName      = "frequency"
	flagUncoreName         = "uncore"
	flagElcName            = "elc"
	flagSSTName            = "sst"
	flagMemoryName         = "memory"
	flagDimmName           = "dimm"
	flagNetConfigName      = "netconfig"
	flagNicName            = "nic"
	flagDiskName           = "disk"
	flagFilesystemName     = "filesystem"
	flagGpuName            = "gpu"
	flagGaudiName          = "gaudi"
	flagCxlName            = "cxl"
	flagCveName            = "cve"
	flagProcessName        = "process"
	flagSensorName         = "sensor"
	flagChassisStatusName  = "chassisstatus"
	flagPmuName            = "pmu"
	flagSystemEventLogName = "sel"
	flagKernelLogName      = "kernellog"
)

// categories maps flag names to tables that will be included in report
var categories = []app.Category{
	{FlagName: flagSystemSummaryName, FlagVar: &flagSystemSummary, Help: "System Summary", Tables: []table.TableDefinition{tableDefinitions[SystemSummaryTableName]}},
	{FlagName: flagHostName, FlagVar: &flagHost, Help: "Host", Tables: []table.TableDefinition{tableDefinitions[HostTableName]}},
	{FlagName: flagBiosName, FlagVar: &flagBios, Help: "BIOS", Tables: []table.TableDefinition{tableDefinitions[BIOSTableName]}},
	{FlagName: flagOsName, FlagVar: &flagOs, Help: "Operating System", Tables: []table.TableDefinition{tableDefinitions[OperatingSystemTableName]}},
	{FlagName: flagSoftwareName, FlagVar: &flagSoftware, Help: "Software Versions", Tables: []table.TableDefinition{tableDefinitions[SoftwareVersionTableName]}},
	{FlagName: flagCpuName, FlagVar: &flagCpu, Help: "Processor Details", Tables: []table.TableDefinition{tableDefinitions[CPUTableName]}},
	{FlagName: flagPrefetcherName, FlagVar: &flagPrefetcher, Help: "Prefetchers", Tables: []table.TableDefinition{tableDefinitions[PrefetcherTableName]}},
	{FlagName: flagIsaName, FlagVar: &flagIsa, Help: "Instruction Sets", Tables: []table.TableDefinition{tableDefinitions[ISATableName]}},
	{FlagName: flagAcceleratorName, FlagVar: &flagAccelerator, Help: "On-board Accelerators", Tables: []table.TableDefinition{tableDefinitions[AcceleratorTableName]}},
	{FlagName: flagPowerName, FlagVar: &flagPower, Help: "Power Settings", Tables: []table.TableDefinition{tableDefinitions[PowerTableName]}},
	{FlagName: flagCstatesName, FlagVar: &flagCstates, Help: "C-states", Tables: []table.TableDefinition{tableDefinitions[CstateTableName]}},
	{FlagName: flagFrequencyName, FlagVar: &flagFrequency, Help: "Maximum Frequencies", Tables: []table.TableDefinition{tableDefinitions[MaximumFrequencyTableName]}},
	{FlagName: flagSSTName, FlagVar: &flagSST, Help: "Speed Select Technology Settings", Tables: []table.TableDefinition{tableDefinitions[SSTTFHPTableName], tableDefinitions[SSTTFLPTableName]}},
	{FlagName: flagUncoreName, FlagVar: &flagUncore, Help: "Uncore Configuration", Tables: []table.TableDefinition{tableDefinitions[UncoreTableName]}},
	{FlagName: flagElcName, FlagVar: &flagElc, Help: "Efficiency Latency Control Settings", Tables: []table.TableDefinition{tableDefinitions[ElcTableName]}},
	{FlagName: flagMemoryName, FlagVar: &flagMemory, Help: "Memory Configuration", Tables: []table.TableDefinition{tableDefinitions[MemoryTableName]}},
	{FlagName: flagDimmName, FlagVar: &flagDimm, Help: "DIMM Population", Tables: []table.TableDefinition{tableDefinitions[DIMMTableName]}},
	{FlagName: flagNetConfigName, FlagVar: &flagNetConfig, Help: "Network Configuration", Tables: []table.TableDefinition{tableDefinitions[NetworkConfigTableName]}},
	{FlagName: flagNicName, FlagVar: &flagNic, Help: "Network Cards", Tables: []table.TableDefinition{tableDefinitions[NICTableName], tableDefinitions[NICCpuAffinityTableName], tableDefinitions[NICPacketSteeringTableName]}},
	{FlagName: flagDiskName, FlagVar: &flagDisk, Help: "Storage Devices", Tables: []table.TableDefinition{tableDefinitions[DiskTableName]}},
	{FlagName: flagFilesystemName, FlagVar: &flagFilesystem, Help: "File Systems", Tables: []table.TableDefinition{tableDefinitions[FilesystemTableName]}},
	{FlagName: flagGpuName, FlagVar: &flagGpu, Help: "GPUs", Tables: []table.TableDefinition{tableDefinitions[GPUTableName]}},
	{FlagName: flagGaudiName, FlagVar: &flagGaudi, Help: "Gaudi Devices", Tables: []table.TableDefinition{tableDefinitions[GaudiTableName]}},
	{FlagName: flagCxlName, FlagVar: &flagCxl, Help: "CXL Devices", Tables: []table.TableDefinition{tableDefinitions[CXLTableName]}},
	{FlagName: flagPcieName, FlagVar: &flagPcie, Help: "PCIE Slots", Tables: []table.TableDefinition{tableDefinitions[PCIeTableName]}},
	{FlagName: flagCveName, FlagVar: &flagCve, Help: "Vulnerabilities", Tables: []table.TableDefinition{tableDefinitions[CVETableName]}},
	{FlagName: flagProcessName, FlagVar: &flagProcess, Help: "Process List", Tables: []table.TableDefinition{tableDefinitions[ProcessTableName]}},
	{FlagName: flagSensorName, FlagVar: &flagSensor, Help: "Sensor Status", Tables: []table.TableDefinition{tableDefinitions[SensorTableName]}},
	{FlagName: flagChassisStatusName, FlagVar: &flagChassisStatus, Help: "Chassis Status", Tables: []table.TableDefinition{tableDefinitions[ChassisStatusTableName]}},
	{FlagName: flagPmuName, FlagVar: &flagPmu, Help: "Performance Monitoring Unit Status", Tables: []table.TableDefinition{tableDefinitions[PMUTableName]}},
	{FlagName: flagSystemEventLogName, FlagVar: &flagSystemEventLog, Help: "System Event Log", Tables: []table.TableDefinition{tableDefinitions[SystemEventLogTableName]}},
	{FlagName: flagKernelLogName, FlagVar: &flagKernelLog, Help: "Kernel Log", Tables: []table.TableDefinition{tableDefinitions[KernelLogTableName]}},
}

func init() {
	// set up category flags
	for _, cat := range categories {
		Cmd.Flags().BoolVar(cat.FlagVar, cat.FlagName, cat.DefaultValue, cat.Help)
	}
	// set up other flags
	Cmd.Flags().StringVar(&app.FlagInput, app.FlagInputName, "", "")
	Cmd.Flags().BoolVar(&flagAll, flagAllName, true, "")
	Cmd.Flags().StringSliceVar(&app.FlagFormat, app.FlagFormatName, []string{report.FormatAll}, "")

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
			Help: "report configuration for all categories",
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
		formatOptions := append([]string{report.FormatAll}, report.FormatOptions...)
		if !slices.Contains(formatOptions, format) {
			return workflow.FlagValidationError(cmd, fmt.Sprintf("format options are: %s", strings.Join(formatOptions, ", ")))
		}
	}
	// common target flags
	if err := workflow.ValidateTargetFlags(cmd); err != nil {
		return workflow.FlagValidationError(cmd, err.Error())
	}
	return nil
}

func runCmd(cmd *cobra.Command, args []string) error {
	tables := []table.TableDefinition{}
	// add category tables
	for _, cat := range categories {
		if *cat.FlagVar || flagAll {
			tables = append(tables, cat.Tables...)
		}
	}
	// include insights table if all categories are selected
	var insightsFunc app.InsightsFunc
	if flagAll {
		insightsFunc = workflow.DefaultInsightsFunc
	}
	reportingCommand := workflow.ReportingCommand{
		Cmd:                    cmd,
		Tables:                 tables,
		InsightsFunc:           insightsFunc,
		SystemSummaryTableName: SystemSummaryTableName,
	}

	report.RegisterHTMLRenderer(DIMMTableName, dimmTableHTMLRenderer)

	return reportingCommand.Run()
}
