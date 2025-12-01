// Package config is a subcommand of the root command. It sets system configuration items on target platform(s).
package config

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"perfspect/internal/common"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const restoreCmdName = "restore"

// flagValue represents a single flag name and value pair, preserving order
type flagValue struct {
	flagName string
	value    string
}

var restoreExamples = []string{
	fmt.Sprintf("  Restore config from file on local host:     $ %s %s %s gnr_config.txt", common.AppName, cmdName, restoreCmdName),
	fmt.Sprintf("  Restore config on remote target:            $ %s %s %s gnr_config.txt --target 192.168.1.1 --user fred --key fred_key", common.AppName, cmdName, restoreCmdName),
	fmt.Sprintf("  Restore config without confirmation:        $ %s %s %s gnr_config.txt --yes", common.AppName, cmdName, restoreCmdName),
}

var RestoreCmd = &cobra.Command{
	Use:   restoreCmdName + " <file>",
	Short: "Restore system configuration from a previously recorded file",
	Long: `Restores system configuration from a file that was previously recorded using the --record flag.

The restore command will parse the configuration file, validate the settings against the target system,
and apply the configuration changes. By default, you will be prompted to confirm before applying changes.`,
	Example:       strings.Join(restoreExamples, "\n"),
	RunE:          runRestoreCmd,
	PreRunE:       validateRestoreFlags,
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
}

var (
	flagRestoreYes bool
)

const (
	flagRestoreYesName = "yes"
)

func init() {
	Cmd.AddCommand(RestoreCmd)

	RestoreCmd.Flags().BoolVar(&flagRestoreYes, flagRestoreYesName, false, "skip confirmation prompt")

	common.AddTargetFlags(RestoreCmd)

	RestoreCmd.SetUsageFunc(restoreUsageFunc)
}

func restoreUsageFunc(cmd *cobra.Command) error {
	cmd.Printf("Usage: %s <file> [flags]\n\n", cmd.CommandPath())
	cmd.Printf("Examples:\n%s\n\n", cmd.Example)
	cmd.Println("Arguments:")
	cmd.Printf("  file: path to the configuration file to restore\n\n")
	cmd.Println("Flags:")
	cmd.Print("  General Options:\n")
	cmd.Printf("    --%-20s %s\n", flagRestoreYesName, "skip confirmation prompt")

	targetFlagGroup := common.GetTargetFlagGroup()
	cmd.Printf("  %s:\n", targetFlagGroup.GroupName)
	for _, flag := range targetFlagGroup.Flags {
		cmd.Printf("    --%-20s %s\n", flag.Name, flag.Help)
	}

	cmd.Println("\nGlobal Flags:")
	cmd.Root().PersistentFlags().VisitAll(func(pf *pflag.Flag) {
		flagDefault := ""
		if cmd.Root().PersistentFlags().Lookup(pf.Name).DefValue != "" {
			flagDefault = fmt.Sprintf(" (default: %s)", cmd.Root().PersistentFlags().Lookup(pf.Name).DefValue)
		}
		cmd.Printf("  --%-20s %s%s\n", pf.Name, pf.Usage, flagDefault)
	})
	return nil
}

func validateRestoreFlags(cmd *cobra.Command, args []string) error {
	// validate that the file exists
	if len(args) != 1 {
		return common.FlagValidationError(cmd, "restore requires exactly one argument: the path to the configuration file")
	}
	filePath := args[0]
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return common.FlagValidationError(cmd, fmt.Sprintf("configuration file does not exist: %s", filePath))
	}
	// validate common target flags
	if err := common.ValidateTargetFlags(cmd); err != nil {
		return common.FlagValidationError(cmd, err.Error())
	}
	return nil
}

func runRestoreCmd(cmd *cobra.Command, args []string) error {
	configFilePath := args[0]

	// parse the configuration file
	flagValues, err := parseConfigFile(configFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse configuration file: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}

	if len(flagValues) == 0 {
		fmt.Println("No configuration settings found in file.")
		return nil
	}

	// show what will be restored
	fmt.Printf("Configuration settings to restore from %s:\n", configFilePath)
	for _, fv := range flagValues {
		fmt.Printf("  --%s %s\n", fv.flagName, fv.value)
	}
	fmt.Println()

	// build the command to execute
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// build arguments: perfspect config --target ... --flag1 value1 --flag2 value2 ...
	cmdArgs := []string{"config"}

	// copy target flags from restore command first
	targetFlags := []string{"target", "targets", "user", "key", "keystring", "port", "password"}
	for _, flagName := range targetFlags {
		if flag := cmd.Flags().Lookup(flagName); flag != nil && flag.Changed {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", flagName), flag.Value.String())
		}
	}

	// copy relevant global flags from root command next
	globalFlags := []string{"debug", "output", "tempdir", "syslog", "log-stdout"}
	for _, flagName := range globalFlags {
		if flag := cmd.Root().PersistentFlags().Lookup(flagName); flag != nil && flag.Changed {
			if flag.Value.Type() == "bool" {
				// for bool flags, only add if true
				if flag.Value.String() == "true" {
					cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", flagName))
				}
			} else {
				cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", flagName), flag.Value.String())
			}
		}
	}

	// add config flags last
	for _, fv := range flagValues {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", fv.flagName), fv.value)
	}

	// show the command that will be executed
	fmt.Printf("Command: %s %s\n\n", executable, strings.Join(cmdArgs, " "))

	// prompt for confirmation unless --yes was specified
	if !flagRestoreYes {
		fmt.Print("Apply these configuration changes? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read user input: %v", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Restore cancelled.")
			return nil
		}
	}

	// execute the command
	slog.Info("executing perfspect config", slog.String("command", executable), slog.String("args", strings.Join(cmdArgs, " ")))
	fmt.Println() // blank line before config output

	execCmd := exec.Command(executable, cmdArgs...)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	err = execCmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("config command failed with exit code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("failed to execute config command: %v", err)
	}

	return nil
}

// parseConfigFile parses a recorded configuration file and extracts flag names and values
// Returns a slice of flagValue structs in the order they appear in the file
func parseConfigFile(filePath string) ([]flagValue, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	var flagValues []flagValue
	scanner := bufio.NewScanner(file)

	// regex to match lines with flag syntax: --flag-name <value>
	// Format: "Field Name:  Value  --flag-name <value_type>"
	flagLineRegex := regexp.MustCompile(`^\s*(.+?):\s+(.+?)\s+(--\S+)\s+<.+>$`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := flagLineRegex.FindStringSubmatch(line)
		if len(matches) == 4 {
			// matches[1] = field name (not used)
			rawValue := strings.TrimSpace(matches[2])
			flagStr := matches[3]

			// extract flag name (remove the leading --)
			flagName := strings.TrimPrefix(flagStr, "--")

			// convert the raw value to the appropriate format
			convertedValue, err := convertValue(flagName, rawValue)
			if err != nil {
				slog.Warn(fmt.Sprintf("skipping flag %s: %v", flagName, err))
				continue
			}

			flagValues = append(flagValues, flagValue{
				flagName: flagName,
				value:    convertedValue,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return flagValues, nil
}

// convertValue converts a raw value string from the config file to the appropriate format for the flag
func convertValue(flagName string, rawValue string) (string, error) {
	// handle "inconsistent" values - skip these
	if strings.Contains(strings.ToLower(rawValue), "inconsistent") {
		return "", fmt.Errorf("value is inconsistent, cannot restore")
	}

	// handle numeric values with units
	switch flagName {
	case flagCoreCountName:
		// "86" -> "86" (just validate it's a number)
		if _, err := strconv.Atoi(rawValue); err != nil {
			return "", fmt.Errorf("invalid integer value: %s", rawValue)
		}
		return rawValue, nil
	case flagLLCSizeName:
		// "336M" -> "336" (MB is assumed)
		return parseNumericWithUnit(rawValue, "M", "MB")
	case flagTDPName:
		// "350W" -> "350" (Watts is assumed)
		return parseNumericWithUnit(rawValue, "W")
	case flagAllCoreMaxFrequencyName, flagUncoreMaxFrequencyName, flagUncoreMinFrequencyName,
		flagUncoreMaxComputeFrequencyName, flagUncoreMinComputeFrequencyName,
		flagUncoreMaxIOFrequencyName, flagUncoreMinIOFrequencyName:
		// "3.2GHz" -> "3.2" (GHz is assumed)
		return parseNumericWithUnit(rawValue, "GHz")
	case flagEPBName, flagEPPName:
		// "Performance (0)" -> "0"
		// "inconsistent" -> error
		return parseParenthesizedNumber(rawValue)
	case flagGovernorName:
		// "performance" or "powersave"
		return parseEnableDisableOrOption(rawValue, governorOptions)
	case flagELCName:
		// "Default" -> "default"
		// "Latency-Optimized" -> "latency-optimized"
		rawValueLower := strings.ToLower(rawValue)
		if slices.Contains(elcOptions, rawValueLower) {
			return rawValueLower, nil
		}
		return "", fmt.Errorf("invalid elc value: %s", rawValue)
	case flagC6Name:
		return parseEnableDisableOrOption(rawValue, c6Options)
	case flagC1DemotionName:
		return parseEnableDisableOrOption(rawValue, c1DemotionOptions)
	default:
		// check if it's a prefetcher flag
		if strings.HasPrefix(flagName, "pref-") {
			// "Enabled" or "Disabled" -> "enable" or "disable"
			return parseEnableDisableOrOption(rawValue, prefetcherOptions)
		}
	}

	return "", fmt.Errorf("unknown flag: %s", flagName)
}

// parseNumericWithUnit extracts numeric value from strings like "3.2GHz", "336M", "350W"
func parseNumericWithUnit(value string, units ...string) (string, error) {
	// trim whitespace
	value = strings.TrimSpace(value)

	// try to remove each unit suffix
	for _, unit := range units {
		if strings.HasSuffix(value, unit) {
			numStr := strings.TrimSuffix(value, unit)
			// validate it's a valid number
			if _, err := strconv.ParseFloat(numStr, 64); err != nil {
				return "", fmt.Errorf("invalid numeric value: %s", value)
			}
			return numStr, nil
		}
	}

	// if no unit found, check if it's already a valid number
	if _, err := strconv.ParseFloat(value, 64); err != nil {
		return "", fmt.Errorf("value missing expected unit (%s): %s", strings.Join(units, ", "), value)
	}
	return value, nil
}

// parseParenthesizedNumber extracts number from strings like "Performance (0)" or "Best Performance (0)"
func parseParenthesizedNumber(value string) (string, error) {
	// look for pattern "text (number)"
	parenRegex := regexp.MustCompile(`\((\d+)\)`)
	matches := parenRegex.FindStringSubmatch(value)
	if len(matches) == 2 {
		return matches[1], nil
	}
	return "", fmt.Errorf("could not extract number from: %s", value)
}

// parseEnableDisableOrOption converts "Enabled"/"Disabled" to "enable"/"disable" or validates against option list
func parseEnableDisableOrOption(value string, validOptions []string) (string, error) {
	// normalize: trim and lowercase
	normalized := strings.ToLower(strings.TrimSpace(value))

	// check direct match with valid options
	if slices.Contains(validOptions, normalized) {
		return normalized, nil
	}

	// special case: "Enabled" -> "enable", "Disabled" -> "disable"
	switch normalized {
	case "enabled":
		normalized = "enable"
	case "disabled":
		normalized = "disable"
	}

	// check if normalized value is in valid options
	if slices.Contains(validOptions, normalized) {
		return normalized, nil
	}

	return "", fmt.Errorf("invalid value '%s', valid options are: %s", value, strings.Join(validOptions, ", "))
}
