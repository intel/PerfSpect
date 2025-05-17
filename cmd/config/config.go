// Package config is a subcommand of the root command. It sets system configuration items on target platform(s).
package config

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"os"
	"perfspect/internal/common"
	"perfspect/internal/progress"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/target"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

const cmdName = "config"

var examples = []string{
	fmt.Sprintf("  Set core count on local host:            $ %s %s --cores 32", common.AppName, cmdName),
	fmt.Sprintf("  Set multiple config items on local host: $ %s %s --core-max 3.0 --uncore-max 2.1 --tdp 120", common.AppName, cmdName),
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
	// print config prior to changes, optionally
	if !cmd.Flags().Lookup(flagNoSummaryName).Changed {
		if err := printConfig(myTargets, localTempDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
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
	for range myTargets {
		<-channelError
	}
	multiSpinner.Finish()
	fmt.Println() // blank line
	// print config after making changes
	if !cmd.Flags().Lookup(flagNoSummaryName).Changed {
		if err := printConfig(myTargets, localTempDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
	}
	return nil
}

func setOnTarget(cmd *cobra.Command, myTarget target.Target, flagGroups []flagGroup, localTempDir string, channelError chan error, statusUpdate progress.MultiSpinnerUpdateFunc) {
	channelSetComplete := make(chan setOutput)
	var successMessages []string
	var errorMessages []string
	_ = statusUpdate(myTarget.GetName(), "updating configuration")
	for _, group := range flagGroups {
		for _, flag := range group.flags {
			if flag.HasSetFunc() && cmd.Flags().Lookup(flag.GetName()).Changed {
				successMessages = append(successMessages, fmt.Sprintf("set %s to %s", flag.GetName(), flag.GetValueAsString()))
				errorMessages = append(errorMessages, fmt.Sprintf("failed to set %s to %s", flag.GetName(), flag.GetValueAsString()))
				switch flag.GetType() {
				case "uint":
					if flag.uintSetFunc != nil {
						value, _ := cmd.Flags().GetUint(flag.GetName())
						go flag.uintSetFunc(value, myTarget, localTempDir, channelSetComplete, len(successMessages)-1)
					}
				case "int":
					if flag.intSetFunc != nil {
						value, _ := cmd.Flags().GetInt(flag.GetName())
						go flag.intSetFunc(value, myTarget, localTempDir, channelSetComplete, len(successMessages)-1)
					}
				case "float64":
					if flag.floatSetFunc != nil {
						value, _ := cmd.Flags().GetFloat64(flag.GetName())
						go flag.floatSetFunc(value, myTarget, localTempDir, channelSetComplete, len(successMessages)-1)
					}
				case "string":
					if flag.stringSetFunc != nil {
						value, _ := cmd.Flags().GetString(flag.GetName())
						go flag.stringSetFunc(value, myTarget, localTempDir, channelSetComplete, len(successMessages)-1)
					}
				case "bool":
					if flag.boolSetFunc != nil {
						value, _ := cmd.Flags().GetBool(flag.GetName())
						go flag.boolSetFunc(value, myTarget, localTempDir, channelSetComplete, len(successMessages)-1)
					}
				}
			}
		}
	}
	// wait for all set goroutines to finish
	statusMessages := []string{}
	for range successMessages {
		out := <-channelSetComplete
		if out.err != nil {
			slog.Error(out.err.Error())
			statusMessages = append(statusMessages, errorMessages[out.goRoutineID])
		} else {
			statusMessages = append(statusMessages, successMessages[out.goRoutineID])
		}
	}
	statusMessage := fmt.Sprintf("configuration update complete: %s", strings.Join(statusMessages, ", "))
	slog.Info(statusMessage, slog.String("target", myTarget.GetName()))
	_ = statusUpdate(myTarget.GetName(), statusMessage)
	channelError <- nil
}

func printConfig(myTargets []target.Target, localTempDir string) (err error) {
	scriptNames := report.GetScriptNamesForTable(report.ConfigurationTableName)
	var scriptsToRun []script.ScriptDefinition
	for _, scriptName := range scriptNames {
		scriptsToRun = append(scriptsToRun, script.GetScriptByName(scriptName))
	}
	multiSpinner := progress.NewMultiSpinner()
	multiSpinner.Start()
	orderedTargetScriptOutputs := []common.TargetScriptOutputs{}
	channelTargetScriptOutputs := make(chan common.TargetScriptOutputs)
	channelError := make(chan error)
	for _, myTarget := range myTargets {
		err = multiSpinner.AddSpinner(myTarget.GetName())
		if err != nil {
			err = fmt.Errorf("failed to add spinner: %v", err)
			return
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
				targetScriptOutputs.TableNames = []string{report.ConfigurationTableName}
				orderedTargetScriptOutputs = append(orderedTargetScriptOutputs, targetScriptOutputs)
				break
			}
		}
	}
	multiSpinner.Finish()
	// process and print the table for each target
	for _, targetScriptOutputs := range orderedTargetScriptOutputs {
		// process the tables, i.e., get field values from raw script output
		tableNames := []string{report.ConfigurationTableName}
		var tableValues []report.TableValues
		if tableValues, err = report.ProcessTables(tableNames, targetScriptOutputs.ScriptOutputs); err != nil {
			err = fmt.Errorf("failed to process collected data: %v", err)
			return
		}
		// create the report for this single table
		var reportBytes []byte
		if reportBytes, err = report.Create("txt", tableValues, targetScriptOutputs.ScriptOutputs, targetScriptOutputs.TargetName); err != nil {
			err = fmt.Errorf("failed to create report: %v", err)
			return
		}
		// print the report
		if len(orderedTargetScriptOutputs) > 1 {
			fmt.Printf("%s\n", targetScriptOutputs.TargetName)
		}
		fmt.Print(string(reportBytes))
	}
	return
}

// collectOnTarget runs the scripts on the target and sends the results to the appropriate channels
func collectOnTarget(myTarget target.Target, scriptsToRun []script.ScriptDefinition, localTempDir string, channelTargetScriptOutputs chan common.TargetScriptOutputs, channelError chan error, statusUpdate progress.MultiSpinnerUpdateFunc) {
	// run the scripts on the target
	if statusUpdate != nil {
		_ = statusUpdate(myTarget.GetName(), "collecting configuration")
	}
	scriptOutputs, err := script.RunScripts(myTarget, scriptsToRun, true, localTempDir)
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
