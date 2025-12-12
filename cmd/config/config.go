// Package config is a subcommand of the root command. It sets system configuration items on target platform(s).
package config

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"os"
	"perfspect/internal/common"
	"perfspect/internal/cpus"
	"perfspect/internal/progress"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/table"
	"perfspect/internal/target"
	"perfspect/internal/util"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

const cmdName = "config"

var examples = []string{
	fmt.Sprintf("  Set core count on local host:            $ %s %s --cores 32", common.AppName, cmdName),
	fmt.Sprintf("  Set multiple config items on local host: $ %s %s --core-max 3.0 --uncore-max 2.1 --tdp 120", common.AppName, cmdName),
	fmt.Sprintf("  Record config to file before changes:    $ %s %s --c6 disable --epb 0 --record", common.AppName, cmdName),
	fmt.Sprintf("  Restore config from file:                $ %s %s restore gnr_config.txt", common.AppName, cmdName),
	fmt.Sprintf("  Set core count on remote target:         $ %s %s --cores 32 --target 192.168.1.1 --user fred --key fred_key", common.AppName, cmdName),
	fmt.Sprintf("  View current config on remote target:    $ %s %s --target 192.168.1.1 --user fred --key fred_key", common.AppName, cmdName),
	fmt.Sprintf("  Set governor on remote targets:          $ %s %s --gov performance --targets targets.yaml", common.AppName, cmdName),
}

var Cmd = &cobra.Command{
	Use:   cmdName,
	Short: "Modify target(s) system configuration",
	Long: `Sets system configuration items on target platform(s).

USE CAUTION! Target may become unstable. It is up to the user to ensure that the requested configuration is valid for the target. There is not an automated way to revert the configuration changes. If all else fails, reboot the target.`,
	Example:       strings.Join(examples, "\n"),
	RunE:          runCmd,
	PreRunE:       validateFlags,
	GroupID:       "primary",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
}

func init() {
	initializeFlags(Cmd)
}

func runCmd(cmd *cobra.Command, args []string) error {
	// appContext is the application context that holds common data and resources.
	appContext := cmd.Parent().Context().Value(common.AppContext{}).(common.AppContext)
	localTempDir := appContext.LocalTempDir
	outputDir := appContext.OutputDir

	flagRecord := cmd.Flags().Lookup(flagRecordName).Value.String() == "true"
	flagNoSummary := cmd.Flags().Lookup(flagNoSummaryName).Value.String() == "true"

	// create output directory if we are recording the configuration
	if flagRecord {
		err := util.CreateDirectoryIfNotExists(outputDir, 0755) // #nosec G301
		if err != nil {
			err = fmt.Errorf("failed to create output directory: %w", err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
	}
	// get the targets
	myTargets, targetErrs, err := common.GetTargets(cmd, true, true, localTempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	// schedule the removal of the temp directory on each target (if the debug flag is not set)
	if cmd.Parent().PersistentFlags().Lookup("debug").Value.String() != "true" {
		for _, myTarget := range myTargets {
			if myTarget.GetTempDirectory() != "" {
				deferTarget := myTarget // create a new variable to capture the current value
				defer func(deferTarget target.Target) {
					err = myTarget.RemoveTempDirectory()
					if err != nil {
						fmt.Fprintf(os.Stderr, "Failed to remove target temp directory: %+v\n", err)
						slog.Error(err.Error())
					}
				}(deferTarget)
			}
		}
	}
	// check for errors in target creation
	for i := range targetErrs {
		if targetErrs[i] != nil {
			fmt.Fprintf(os.Stderr, "Error: target: %s, %v\n", myTargets[i].GetName(), targetErrs[i])
			slog.Error(targetErrs[i].Error())
			// remove target from targets list
			myTargets = slices.Delete(myTargets, i, i+1)
		}
	}
	if len(myTargets) == 0 {
		err := fmt.Errorf("no targets remain")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	// collect and print and/or record the configuration before making changes
	if !flagNoSummary || flagRecord {
		config, err := getConfig(myTargets, localTempDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
		reports, err := processConfig(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
		filesWritten, err := printConfig(reports, !flagNoSummary, flagRecord, outputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
		if len(filesWritten) > 0 {
			message := "Configuration"
			if len(filesWritten) > 1 {
				message = "Configurations"
			}
			fmt.Printf("%s recorded:\n", message)
			for _, fileWritten := range filesWritten {
				fmt.Printf("  %s\n", fileWritten)
			}
			fmt.Println()
		}
	}
	// if no changes were requested, print a message and return
	var changeRequested bool
	for _, group := range flagGroups {
		for _, flag := range group.flags {
			if flag.HasSetFunc() && cmd.Flags().Lookup(flag.GetName()).Changed {
				changeRequested = true
				break
			}
		}
		if changeRequested {
			break
		}
	}
	if !changeRequested {
		fmt.Println("No changes requested.")
		return nil
	}
	// make requested changes on all targets
	channelError := make(chan error)
	multiSpinner := progress.NewMultiSpinner()
	multiSpinner.Start()
	for _, myTarget := range myTargets {
		err = multiSpinner.AddSpinner(myTarget.GetName())
		if err != nil {
			err = fmt.Errorf("failed to add spinner: %v", err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
		go setOnTarget(cmd, myTarget, flagGroups, localTempDir, channelError, multiSpinner.Status)
	}
	// wait for all targets to finish
	var setOnTargetErr error
	for range myTargets {
		setOnTargetErr = <-channelError
	}
	multiSpinner.Finish()
	fmt.Println() // blank line
	// collect and print the configuration before making changes
	if !flagNoSummary {
		config, err := getConfig(myTargets, localTempDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
		reports, err := processConfig(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
		_, err = printConfig(reports, !flagNoSummary, false, outputDir) // print, don't record
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
	}
	if setOnTargetErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", setOnTargetErr)
		slog.Error(setOnTargetErr.Error())
		cmd.SilenceUsage = true
		return setOnTargetErr
	}
	return nil
}

// prepareTarget prepares the target for configuration changes
// almost all set scripts require the msr kernel module to be loaded and
// use wrmsr and rdmsr, so we do that here so that the goroutines for the
// set scripts can run in parallel without conflicts
func prepareTarget(myTarget target.Target, localTempDir string) (err error) {
	prepareScript := script.ScriptDefinition{
		Name:           "prepare-target",
		ScriptTemplate: "exit 0",
		Superuser:      true,
		Vendors:        []string{cpus.IntelVendor},
		Depends:        []string{"wrmsr", "rdmsr"},
		Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, prepareScript, localTempDir)
	return err
}

func setOnTarget(cmd *cobra.Command, myTarget target.Target, flagGroups []flagGroup, localTempDir string, channelError chan error, statusUpdate progress.MultiSpinnerUpdateFunc) {
	// prepare the target for configuration changes
	_ = statusUpdate(myTarget.GetName(), "preparing target for configuration changes")
	if err := prepareTarget(myTarget, localTempDir); err != nil {
		_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("error preparing target: %v", err))
		slog.Error(fmt.Sprintf("error preparing target %s: %v", myTarget.GetName(), err))
		channelError <- nil
		return
	}
	var statusMessages []string
	_ = statusUpdate(myTarget.GetName(), "updating configuration")
	var setErrs []error // collect errors but continue setting other flags
	for _, group := range flagGroups {
		for _, flag := range group.flags {
			if flag.HasSetFunc() && cmd.Flags().Lookup(flag.GetName()).Changed {
				successMessage := fmt.Sprintf("set %s to %s", flag.GetName(), flag.GetValueAsString())
				errorMessage := fmt.Sprintf("failed to set %s to %s", flag.GetName(), flag.GetValueAsString())
				var setErr error
				switch flag.GetType() {
				case "int":
					if flag.intSetFunc != nil {
						value, _ := cmd.Flags().GetInt(flag.GetName())
						setErr = flag.intSetFunc(value, myTarget, localTempDir)
					}
				case "float64":
					if flag.floatSetFunc != nil {
						value, _ := cmd.Flags().GetFloat64(flag.GetName())
						setErr = flag.floatSetFunc(value, myTarget, localTempDir)
					}
				case "string":
					if flag.stringSetFunc != nil {
						value, _ := cmd.Flags().GetString(flag.GetName())
						setErr = flag.stringSetFunc(value, myTarget, localTempDir)
					}
				case "bool":
					if flag.boolSetFunc != nil {
						value, _ := cmd.Flags().GetBool(flag.GetName())
						setErr = flag.boolSetFunc(value, myTarget, localTempDir)
					}
				}
				if setErr != nil {
					setErrs = append(setErrs, setErr)
					slog.Error(setErr.Error())
					statusMessages = append(statusMessages, errorMessage)
				} else {
					statusMessages = append(statusMessages, successMessage)
				}
			}
		}
	}
	statusMessage := fmt.Sprintf("configuration update complete: %s", strings.Join(statusMessages, ", "))
	slog.Info(statusMessage, slog.String("target", myTarget.GetName()))
	_ = statusUpdate(myTarget.GetName(), statusMessage)
	// aggregate setErrs and send to channel
	if len(setErrs) > 0 {
		aggregateErrMessages := []string{}
		for _, setErr := range setErrs {
			aggregateErrMessages = append(aggregateErrMessages, setErr.Error())
		}
		channelError <- fmt.Errorf("errors setting configuration on target %s: %s", myTarget.GetName(), strings.Join(aggregateErrMessages, "; "))
		return
	}
	channelError <- nil
}

// getConfig collects the configuration data from the target(s)
func getConfig(myTargets []target.Target, localTempDir string) ([]common.TargetScriptOutputs, error) {

	var scriptsToRun []script.ScriptDefinition
	for _, scriptName := range tableDefinitions[ConfigurationTableName].ScriptNames {
		scriptsToRun = append(scriptsToRun, script.GetScriptByName(scriptName))
	}
	multiSpinner := progress.NewMultiSpinner()
	multiSpinner.Start()
	orderedTargetScriptOutputs := []common.TargetScriptOutputs{}
	channelTargetScriptOutputs := make(chan common.TargetScriptOutputs)
	channelError := make(chan error)
	for _, myTarget := range myTargets {
		err := multiSpinner.AddSpinner(myTarget.GetName())
		if err != nil {
			err = fmt.Errorf("failed to add spinner: %v", err)
			return nil, err
		}
		// run the selected scripts on the target
		go collectOnTarget(myTarget, scriptsToRun, localTempDir, channelTargetScriptOutputs, channelError, multiSpinner.Status)
	}
	// wait for scripts to run on all targets
	var allTargetScriptOutputs []common.TargetScriptOutputs
	for range myTargets {
		select {
		case scriptOutputs := <-channelTargetScriptOutputs:
			allTargetScriptOutputs = append(allTargetScriptOutputs, scriptOutputs)
		case err := <-channelError:
			slog.Error(err.Error())
		}
	}
	// allTargetScriptOutputs is in the order of data collection completion
	// reorder to match order of myTargets
	for _, target := range myTargets {
		for _, targetScriptOutputs := range allTargetScriptOutputs {
			if targetScriptOutputs.TargetName == target.GetName() {
				orderedTargetScriptOutputs = append(orderedTargetScriptOutputs, targetScriptOutputs)
				break
			}
		}
	}
	multiSpinner.Finish()
	return orderedTargetScriptOutputs, nil
}

// processConfig processes the collected configuration data and creates text reports
func processConfig(targetScriptOutputs []common.TargetScriptOutputs) (map[string][]byte, error) {
	reports := make(map[string][]byte)
	var err error
	for _, targetScriptOutput := range targetScriptOutputs {
		// process the tables, i.e., get field values from raw script output
		tables := []table.TableDefinition{tableDefinitions[ConfigurationTableName]}
		var tableValues []table.TableValues
		if tableValues, err = table.ProcessTables(tables, targetScriptOutput.ScriptOutputs); err != nil {
			err = fmt.Errorf("failed to process collected data: %v", err)
			return nil, err
		}
		// create the report for this single table
		var reportBytes []byte
		report.RegisterTextRenderer(ConfigurationTableName, configurationTableTextRenderer)

		if reportBytes, err = report.Create("txt", tableValues, targetScriptOutput.TargetName, ""); err != nil {
			err = fmt.Errorf("failed to create report: %v", err)
			return nil, err
		}
		// append the report to the list
		reports[targetScriptOutput.TargetName] = reportBytes
	}
	return reports, nil
}

// printConfig prints and/or saves the configuration reports
func printConfig(reports map[string][]byte, toStdout bool, toFile bool, outputDir string) ([]string, error) {
	filesWritten := []string{}
	for targetName, reportBytes := range reports {
		if toStdout {
			// print the report to stdout
			if len(reports) > 1 {
				fmt.Printf("%s\n", targetName)
			}
			fmt.Print(string(reportBytes))
		}
		if toFile {
			outputFilePath := fmt.Sprintf("%s/%s_config.txt", outputDir, targetName)
			err := os.WriteFile(outputFilePath, reportBytes, 0644) // #nosec G306
			if err != nil {
				err = fmt.Errorf("failed to write configuration report to file: %v", err)
				return filesWritten, err
			}
			filesWritten = append(filesWritten, outputFilePath)
		}
	}
	return filesWritten, nil
}

// collectOnTarget runs the scripts on the target and sends the results to the appropriate channels
func collectOnTarget(myTarget target.Target, scriptsToRun []script.ScriptDefinition, localTempDir string, channelTargetScriptOutputs chan common.TargetScriptOutputs, channelError chan error, statusUpdate progress.MultiSpinnerUpdateFunc) {
	// run the scripts on the target
	scriptOutputs, err := common.RunScripts(myTarget, scriptsToRun, true, localTempDir, statusUpdate, "collecting configuration", false)
	if err != nil {
		if statusUpdate != nil {
			_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("error collecting configuration: %v", err))
		}
		err = fmt.Errorf("error running data collection scripts on %s: %v", myTarget.GetName(), err)
		channelError <- err
		return
	}
	if statusUpdate != nil {
		_ = statusUpdate(myTarget.GetName(), "configuration collection complete")
	}
	channelTargetScriptOutputs <- common.TargetScriptOutputs{TargetName: myTarget.GetName(), ScriptOutputs: scriptOutputs}
}
