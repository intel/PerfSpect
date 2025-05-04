package config

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"os"
	"perfspect/internal/common"
	"perfspect/internal/report"
	"perfspect/internal/target"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagGroup - structure to hold a group of flags
// groups are used to organize the flags for display in the help message
type flagGroup struct {
	name  string
	flags []flagDefinition
}

// flagGroups - list of flag groups
// initialized by initializeFlags
// and used by the config command
var flagGroups = []flagGroup{}

// flag group names
const (
	flagGroupGeneralName         = "General Options"
	flagGroupUncoreFrequencyName = "Uncore Frequency Options"
	flagGroupPrefetcherName      = "Prefetcher Options"
)

// general flag names
const (
	flagCoreCountName           = "cores"
	flagLLCSizeName             = "llc"
	flagAllCoreMaxFrequencyName = "core-max"
	flagTDPName                 = "tdp"
	flagEPBName                 = "epb"
	flagEPPName                 = "epp"
	flagGovernorName            = "gov"
	flagELCName                 = "elc"
)

// uncore frequency flag names
const (
	flagUncoreMaxFrequencyName        = "uncore-max"
	flagUncoreMinFrequencyName        = "uncore-min"
	flagUncoreMaxComputeFrequencyName = "uncore-max-compute"
	flagUncoreMinComputeFrequencyName = "uncore-min-compute"
	flagUncoreMaxIOFrequencyName      = "uncore-max-io"
	flagUncoreMinIOFrequencyName      = "uncore-min-io"
)

// prefetcher flag names
const (
	flagPrefetcherL2HWName     = "pref-l2hw"
	flagPrefetcherL2AdjName    = "pref-l2adj"
	flagPrefetcherDCUHWName    = "pref-dcuhw"
	flagPrefetcherDCUIPName    = "pref-dcuip"
	flagPrefetcherDCUNPName    = "pref-dcunp"
	flagPrefetcherAMPName      = "pref-amp"
	flagPrefetcherLLCPPName    = "pref-llcpp"
	flagPrefetcherAOPName      = "pref-aop"
	flagPrefetcherHomelessName = "pref-homeless"
	flagPrefetcherLLCName      = "pref-llc"
)

// other flag names
const (
	flagNoSummaryName = "no-summary"
)

// governorOptions - list of valid governor options
var governorOptions = []string{"performance", "powersave"}

// elcOptions - list of valid elc options
var elcOptions = []string{"latency-optimized", "default"}

// prefetcherOptions - list of valid prefetcher options
var prefetcherOptions = []string{"enable", "disable"}

// initializeFlags initializes the command line flags for the config command
// the global flagGroups variable is used to store the flags
func initializeFlags(cmd *cobra.Command) {
	// general options
	group := flagGroup{name: flagGroupGeneralName, flags: []flagDefinition{}}
	group.flags = append(group.flags,
		newIntFlag(cmd, flagCoreCountName, 0, setCoreCount, "number of physical cores per processor", "greater than 0",
			func(cmd *cobra.Command) bool { value, _ := cmd.Flags().GetInt(flagCoreCountName); return value > 0 }),
		newFloat64Flag(cmd, flagLLCSizeName, 0, setLlcSize, "LLC size in MB", "greater than 0",
			func(cmd *cobra.Command) bool { value, _ := cmd.Flags().GetFloat64(flagLLCSizeName); return value > 0 }),
		newFloat64Flag(cmd, flagAllCoreMaxFrequencyName, 0, setCoreFrequency, "all-core max frequency in GHz", "greater than 0.1",
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetFloat64(flagAllCoreMaxFrequencyName)
				return value > 0.1
			}),
		newIntFlag(cmd, flagTDPName, 0, setTDP, "maximum power per processor in Watts", "greater than 0",
			func(cmd *cobra.Command) bool { value, _ := cmd.Flags().GetInt(flagTDPName); return value > 0 }),
		newIntFlag(cmd, flagEPBName, 0, setEPB, "energy perf bias from best performance (0) to most power savings (15)", "0-15",
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetInt(flagEPBName)
				return value >= 0 && value <= 15
			}),
		newIntFlag(cmd, flagEPPName, 0, setEPP, "energy perf profile from best performance (0) to most power savings (255)", "0-255",
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetInt(flagEPPName)
				return value >= 0 && value <= 255
			}),
		newStringFlag(cmd, flagGovernorName, "", setGovernor, "CPU scaling governor ("+strings.Join(governorOptions, ", ")+")", strings.Join(governorOptions, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagGovernorName)
				return slices.Contains(governorOptions, value)
			}),
		newStringFlag(cmd, flagELCName, "", setELC, "Efficiency Latency Control ("+strings.Join(elcOptions, ", ")+") [SRF+]", strings.Join(elcOptions, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagELCName)
				return slices.Contains(elcOptions, value)
			}))
	flagGroups = append(flagGroups, group)
	// uncore frequency options
	group = flagGroup{name: flagGroupUncoreFrequencyName, flags: []flagDefinition{}}
	group.flags = append(group.flags,
		newFloat64Flag(cmd, flagUncoreMaxFrequencyName, 0,
			func(value float64, myTarget target.Target, localTempDir string) error {
				return setUncoreFrequency(true, value, myTarget, localTempDir)
			},
			"maximum uncore frequency in GHz [EMR-]", "greater than 0.1",
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetFloat64(flagUncoreMaxFrequencyName)
				return value > 0.1
			}),
		newFloat64Flag(cmd, flagUncoreMinFrequencyName, 0,
			func(value float64, myTarget target.Target, localTempDir string) error {
				return setUncoreFrequency(false, value, myTarget, localTempDir)
			},
			"minimum uncore frequency in GHz [EMR-]", "greater than 0.1",
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetFloat64(flagUncoreMinFrequencyName)
				return value > 0.1
			}),
		newFloat64Flag(cmd, flagUncoreMaxComputeFrequencyName, 0,
			func(value float64, myTarget target.Target, localTempDir string) error {
				return setUncoreDieFrequency(true, true, value, myTarget, localTempDir)
			},
			"maximum uncore compute die frequency in GHz [SRF+]", "greater than 0.1",
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetFloat64(flagUncoreMaxComputeFrequencyName)
				return value > 0.1
			}),
		newFloat64Flag(cmd, flagUncoreMinComputeFrequencyName, 0,
			func(value float64, myTarget target.Target, localTempDir string) error {
				return setUncoreDieFrequency(false, true, value, myTarget, localTempDir)
			},
			"minimum uncore compute die frequency in GHz [SRF+]", "greater than 0.1",
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetFloat64(flagUncoreMinComputeFrequencyName)
				return value > 0.1
			}),
		newFloat64Flag(cmd, flagUncoreMaxIOFrequencyName, 0,
			func(value float64, myTarget target.Target, localTempDir string) error {
				return setUncoreDieFrequency(true, false, value, myTarget, localTempDir)
			},
			"maximum uncore IO die frequency in GHz [SRF+]", "greater than 0.1",
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetFloat64(flagUncoreMaxIOFrequencyName)
				return value > 0.1
			}),
		newFloat64Flag(cmd, flagUncoreMinIOFrequencyName, 0,
			func(value float64, myTarget target.Target, localTempDir string) error {
				return setUncoreDieFrequency(false, false, value, myTarget, localTempDir)
			},
			"minimum uncore IO die frequency in GHz [SRF+]", "greater than 0.1",
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetFloat64(flagUncoreMinIOFrequencyName)
				return value > 0.1
			}))
	flagGroups = append(flagGroups, group)
	// prefetcher options
	group = flagGroup{name: flagGroupPrefetcherName, flags: []flagDefinition{}}
	group.flags = append(group.flags,
		newStringFlag(cmd, flagPrefetcherL2HWName, "",
			func(value string, myTarget target.Target, localTempDir string) error {
				return setPrefetcher(value, myTarget, localTempDir, report.PrefetcherL2HWName)
			},
			"L2 hardware prefetcher ("+strings.Join(prefetcherOptions, ", ")+")", strings.Join(prefetcherOptions, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagPrefetcherL2HWName)
				return slices.Contains(prefetcherOptions, value)
			}),
		newStringFlag(cmd, flagPrefetcherL2AdjName, "",
			func(value string, myTarget target.Target, localTempDir string) error {
				return setPrefetcher(value, myTarget, localTempDir, report.PrefetcherL2AdjName)
			},
			"L2 adjacent cache line prefetcher ("+strings.Join(prefetcherOptions, ", ")+")", strings.Join(prefetcherOptions, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagPrefetcherL2AdjName)
				return slices.Contains(prefetcherOptions, value)
			}),
		newStringFlag(cmd, flagPrefetcherDCUHWName, "",
			func(value string, myTarget target.Target, localTempDir string) error {
				return setPrefetcher(value, myTarget, localTempDir, report.PrefetcherDCUHWName)
			},
			"DCU hardware prefetcher ("+strings.Join(prefetcherOptions, ", ")+")", strings.Join(prefetcherOptions, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagPrefetcherDCUHWName)
				return slices.Contains(prefetcherOptions, value)
			}),
		newStringFlag(cmd, flagPrefetcherDCUIPName, "",
			func(value string, myTarget target.Target, localTempDir string) error {
				return setPrefetcher(value, myTarget, localTempDir, report.PrefetcherDCUIPName)
			},
			"DCU instruction pointer prefetcher ("+strings.Join(prefetcherOptions, ", ")+")", strings.Join(prefetcherOptions, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagPrefetcherDCUIPName)
				return slices.Contains(prefetcherOptions, value)
			}),
		newStringFlag(cmd, flagPrefetcherDCUNPName, "",
			func(value string, myTarget target.Target, localTempDir string) error {
				return setPrefetcher(value, myTarget, localTempDir, report.PrefetcherDCUNPName)
			},
			"DCU next page prefetcher ("+strings.Join(prefetcherOptions, ", ")+")", strings.Join(prefetcherOptions, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagPrefetcherDCUNPName)
				return slices.Contains(prefetcherOptions, value)
			}),
		newStringFlag(cmd, flagPrefetcherAMPName, "",
			func(value string, myTarget target.Target, localTempDir string) error {
				return setPrefetcher(value, myTarget, localTempDir, report.PrefetcherAMPName)
			},
			"Adaptive multipath probability prefetcher ("+strings.Join(prefetcherOptions, ", ")+") [SPR,EMR,GNR]", strings.Join(prefetcherOptions, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagPrefetcherAMPName)
				return slices.Contains(prefetcherOptions, value)
			}),
		newStringFlag(cmd, flagPrefetcherLLCPPName, "",
			func(value string, myTarget target.Target, localTempDir string) error {
				return setPrefetcher(value, myTarget, localTempDir, report.PrefetcherLLCPPName)
			},
			"LLC page prefetcher ("+strings.Join(prefetcherOptions, ", ")+") [GNR]", strings.Join(prefetcherOptions, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagPrefetcherLLCPPName)
				return slices.Contains(prefetcherOptions, value)
			}),
		newStringFlag(cmd, flagPrefetcherAOPName, "",
			func(value string, myTarget target.Target, localTempDir string) error {
				return setPrefetcher(value, myTarget, localTempDir, report.PrefetcherAOPName)
			},
			"Array of pointers prefetcher ("+strings.Join(prefetcherOptions, ", ")+") [GNR]", strings.Join(prefetcherOptions, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagPrefetcherAOPName)
				return slices.Contains(prefetcherOptions, value)
			}),
		newStringFlag(cmd, flagPrefetcherHomelessName, "",
			func(value string, myTarget target.Target, localTempDir string) error {
				return setPrefetcher(value, myTarget, localTempDir, report.PrefetcherHomelessName)
			},
			"Homeless prefetcher ("+strings.Join(prefetcherOptions, ", ")+") [SPR,EMR,GNR]", strings.Join(prefetcherOptions, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagPrefetcherHomelessName)
				return slices.Contains(prefetcherOptions, value)
			}),
		newStringFlag(cmd, flagPrefetcherLLCName, "",
			func(value string, myTarget target.Target, localTempDir string) error {
				return setPrefetcher(value, myTarget, localTempDir, report.PrefetcherLLCName)
			},
			"Last level cache prefetcher ("+strings.Join(prefetcherOptions, ", ")+") [SPR,EMR,GNR]", strings.Join(prefetcherOptions, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagPrefetcherLLCName)
				return slices.Contains(prefetcherOptions, value)
			}))
	flagGroups = append(flagGroups, group)
	// other options
	group = flagGroup{name: "Other Options", flags: []flagDefinition{}}
	group.flags = append(group.flags,
		newBoolFlag(cmd, flagNoSummaryName, false, nil, "do not print configuration summary", "", nil),
	)
	flagGroups = append(flagGroups, group)

	common.AddTargetFlags(Cmd)
	Cmd.SetUsageFunc(usageFunc)
}

// usageFunc prints the usage information for the command
func usageFunc(cmd *cobra.Command) error {
	cmd.Printf("Usage: %s [flags]\n\n", cmd.CommandPath())
	cmd.Printf("Examples:\n%s\n\n", cmd.Example)
	cmd.Println("Flags:")
	for _, group := range flagGroups {
		cmd.Printf("  %s:\n", group.name)
		for _, flag := range group.flags {
			cmd.Printf("    --%-20s %s\n", flag.GetName(), flag.pflag.Usage)
		}
	}

	targetFlagGroup := common.GetTargetFlagGroup()
	cmd.Printf("  %s:\n", targetFlagGroup.GroupName)
	for _, flag := range targetFlagGroup.Flags {
		cmd.Printf("    --%-20s %s\n", flag.Name, flag.Help)
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

// validateFlags validates the command line flags for the config command
// operates on the global flagGroups variable
func validateFlags(cmd *cobra.Command, args []string) error {
	for _, group := range flagGroups {
		for _, flag := range group.flags {
			if cmd.Flags().Lookup(flag.GetName()).Changed && flag.validationFunc != nil {
				if !flag.validationFunc(cmd) {
					err := fmt.Errorf("invalid flag value, --%s %s, valid values are %s", flag.GetName(), flag.GetValueAsString(), flag.validationDescription)
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					cmd.SilenceUsage = true
					return err
				}
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
