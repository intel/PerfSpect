// Package config is a subcommand of the root command. It sets system configuration items on target platform(s).
package config

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
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
		err = fmt.Errorf("failed to parse configuration file: %v", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
		err = fmt.Errorf("failed to get executable path: %v", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
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

	// always add --no-summary to avoid printing config summary before and after changes
	cmdArgs = append(cmdArgs, "--no-summary")

	// show the command that will be executed
	fmt.Printf("Command: %s %s\n\n", executable, strings.Join(cmdArgs, " "))

	// prompt for confirmation unless --yes was specified
	if !flagRestoreYes {
		fmt.Print("Apply these configuration changes? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			err = fmt.Errorf("failed to read user input: %v", err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
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
	execCmd.Stdin = os.Stdin

	// capture stderr for parsing
	var stderrBuf bytes.Buffer
	stderrWriter := io.MultiWriter(os.Stderr, &stderrBuf)
	execCmd.Stderr = stderrWriter

	err = execCmd.Run()

	// parse stderr output and present results in flag order
	parseAndPresentResults(stderrBuf.String(), flagValues)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			err = fmt.Errorf("config command failed with exit code %d", exitErr.ExitCode())
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
		err = fmt.Errorf("failed to execute config command: %v", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
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

// parseAndPresentResults parses the stderr output from the config command and presents
// successes and errors in the same order as the config flags were specified
// example: "configuration update complete: set gov to powersave, set c1-demotion to disable, set tdp to 350, set c6 to enable, set epb to 0, set core-max to 3.2, set cores to 86, set elc to default, failed to set pref-l2hw to enable, set pref-dcuhw to enable, set pref-llc to disable, set pref-aop to enable, set pref-l2adj to enable, set uncore-max-compute to 2.2, failed to set llc to 336, set pref-dcunp to enable, set pref-homeless to enable, set pref-amp to enable, set pref-dcuip to enable, set pref-llcpp to enable, set uncore-max-io to 2.5, set uncore-min-compute to 0.8, set uncore-min-io to 0.8"
func parseAndPresentResults(stderrOutput string, flagValues []flagValue) {
	if stderrOutput == "" {
		return
	}

	// Parse stderr for success and error messages
	// Looking for patterns like:
	// - "set <flag> to <value>"
	// - "failed to set <flag> to <value>"
	// - "error: ..." messages related to flags

	// Build a map of flag names to their results
	flagResults := make(map[string]string)

	// Regex patterns to match success and error messages
	// Flag names can contain hyphens, so use [\w-]+ instead of \S+
	successPattern := regexp.MustCompile(`set ([\w-]+) to ([^,]+)`)
	errorPattern := regexp.MustCompile(`failed to set ([\w-]+) to ([^,]+)`)

	// Parse stderr line by line
	lines := strings.Split(stderrOutput, "\n")
	for _, line := range lines {
		// Check for success messages - use FindAllStringSubmatch to find all matches on the line
		successMatches := successPattern.FindAllStringSubmatch(line, -1)
		for _, matches := range successMatches {
			if len(matches) >= 3 {
				flagName := matches[1]
				value := strings.TrimSpace(matches[2])
				flagResults[flagName] = fmt.Sprintf("✓ Set %s to %s", flagName, value)
			}
		}

		// Check for error messages - use FindAllStringSubmatch to find all matches on the line
		errorMatches := errorPattern.FindAllStringSubmatch(line, -1)
		for _, matches := range errorMatches {
			if len(matches) >= 3 {
				flagName := matches[1]
				value := strings.TrimSpace(matches[2])
				flagResults[flagName] = fmt.Sprintf("✗ Failed to set %s to %s", flagName, value)
			}
		}
	}

	// Present results in the order of flagValues
	if len(flagValues) > 0 {
		fmt.Println("\nConfiguration Results:")
		for _, fv := range flagValues {
			if result, found := flagResults[fv.flagName]; found {
				fmt.Printf("  %s\n", result)
			} else {
				// If no explicit success or error was found, show unknown status
				fmt.Printf("  ? %s: status unknown\n", fv.flagName)
			}
		}
		fmt.Println()
	}
}
