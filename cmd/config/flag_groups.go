package config

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"perfspect/internal/common"
	"perfspect/internal/report"
	"perfspect/internal/target"
	"regexp"
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
	flagGroupCstateName          = "C-State Options"
	flagGroupOtherName           = "Other Options"
)

// general flag names
const (
	flagCoreCountName              = "cores"
	flagLLCSizeName                = "llc"
	flagTDPName                    = "tdp"
	flagSSEFrequencyName           = "core-max"
	flagSSEFrequencyAllBucketsName = "core-max-buckets"
	flagEPBName                    = "epb"
	flagEPPName                    = "epp"
	flagGovernorName               = "gov"
	flagELCName                    = "elc"
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

const (
	flagC6Name         = "c6"
	flagC1DemotionName = "c1-demotion"
)

// other flag names
const (
	flagNoSummaryName = "no-summary"
	flagRecordName    = "record"
)

// governorOptions - list of valid governor options
var governorOptions = []string{"performance", "powersave"}

// elcOptions - list of valid elc options
var elcOptions = []string{"latency-optimized", "default"}

// prefetcherOptions - list of valid prefetcher options
var prefetcherOptions = []string{"enable", "disable"}

// c6Options - list of valid c-state options
var c6Options = []string{"enable", "disable"}

// c1DemotionOptions - list of valid c1 demotion options
var c1DemotionOptions = []string{"enable", "disable"}

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
		newIntFlag(cmd, flagTDPName, 0, setTDP, "maximum power per processor in Watts", "greater than 0",
			func(cmd *cobra.Command) bool { value, _ := cmd.Flags().GetInt(flagTDPName); return value > 0 }),
		newFloat64Flag(cmd, flagSSEFrequencyName, 0, setSSEFrequency, "SSE frequency in GHz", "greater than 0.1",
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetFloat64(flagSSEFrequencyName)
				return value > 0.1
			}),
		newStringFlag(cmd, flagSSEFrequencyAllBucketsName, "", setSSEFrequencies, "SSE frequencies for all core buckets in GHz (e.g., 1-40/3.5, 41-60/3.4, 61-86/3.2)", "correct format",
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagSSEFrequencyAllBucketsName)
				// Regex pattern: 1-8 buckets in format "start-end/freq", comma-separated
				// Example: "1-40/3.5, 41-60/3.4, 61-86/3.2"
				pattern := `^\d+-\d+/\d+(\.\d+)?(, \d+-\d+/\d+(\.\d+)?){0,7}$`
				matched, _ := regexp.MatchString(pattern, value)
				return matched
			}),
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
		newStringFlag(cmd, flagELCName, "", setELC, "efficiency latency control ("+strings.Join(elcOptions, ", ")+") [SRF+]", strings.Join(elcOptions, ", "),
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
	for _, pref := range report.GetPrefetcherDefinitions() {
		group.flags = append(group.flags,
			newStringFlag(cmd,
				// flag name
				func() string {
					return "pref-" + strings.ReplaceAll(strings.ToLower(pref.ShortName), " ", "")
				}(),
				// flag default value
				"",
				// flag value setter function
				func(value string, myTarget target.Target, localTempDir string) error {
					return setPrefetcher(value, myTarget, localTempDir, pref.ShortName)
				},
				// flag help
				func() string {
					return pref.ShortName + " [" + strings.Join(pref.Uarchs, ",") + "]" + " (" + strings.Join(prefetcherOptions, ", ") + ")"
				}(),
				// flag validation description
				strings.Join(prefetcherOptions, ", "),
				// flag validation function
				func(cmd *cobra.Command) bool {
					value, _ := cmd.Flags().GetString("pref-" + strings.ReplaceAll(strings.ToLower(pref.ShortName), " ", ""))
					return slices.Contains(prefetcherOptions, value)
				},
			),
		)
	}
	flagGroups = append(flagGroups, group)
	// c-state options
	group = flagGroup{name: flagGroupCstateName, flags: []flagDefinition{}}
	group.flags = append(group.flags,
		newStringFlag(cmd, flagC6Name, "", setC6,
			"C6 ("+strings.Join(c6Options, ", ")+")", strings.Join(c6Options, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagC6Name)
				return slices.Contains(c6Options, value)
			}),
		newStringFlag(cmd, flagC1DemotionName, "", setC1Demotion,
			"C1 Demotion ("+strings.Join(c1DemotionOptions, ", ")+")", strings.Join(c1DemotionOptions, ", "),
			func(cmd *cobra.Command) bool {
				value, _ := cmd.Flags().GetString(flagC1DemotionName)
				return slices.Contains(c1DemotionOptions, value)
			}),
	)
	flagGroups = append(flagGroups, group)
	// other options
	group = flagGroup{name: flagGroupOtherName, flags: []flagDefinition{}}
	group.flags = append(group.flags,
		newBoolFlag(cmd, flagNoSummaryName, false, nil, "do not print configuration summary", "", nil),
	)
	group.flags = append(group.flags,
		newBoolFlag(cmd, flagRecordName, false, nil, "record the current configuration to a file to be restored later", "", nil),
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

	cmd.Printf("\nSubcommands:\n")
	for _, subCmd := range cmd.Commands() {
		cmd.Printf("  %s: %s\n", subCmd.Name(), subCmd.Short)
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
					return common.FlagValidationError(cmd, fmt.Sprintf("invalid flag value, --%s %s, valid values are %s", flag.GetName(), flag.GetValueAsString(), flag.validationDescription))
				}
			}
		}
	}
	// common target flags
	if err := common.ValidateTargetFlags(cmd); err != nil {
		return common.FlagValidationError(cmd, err.Error())
	}
	return nil
}
