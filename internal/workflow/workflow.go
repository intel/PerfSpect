// Package workflow implements the common flow/logic for reporting commands
// (report, telemetry, flamegraph, lock). It handles target management,
// script execution, and report generation.
package workflow

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"slices"

	"perfspect/internal/app"
	"perfspect/internal/progress"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/table"
	"perfspect/internal/target"
	"perfspect/internal/util"

	"github.com/spf13/cobra"
)

// TargetScriptOutputs holds the script outputs and tables for a target.
type TargetScriptOutputs struct {
	TargetName    string
	ScriptOutputs map[string]script.ScriptOutput
	Tables        []table.TableDefinition
}

// GetScriptOutputs returns the script outputs for the target.
func (tso *TargetScriptOutputs) GetScriptOutputs() map[string]script.ScriptOutput {
	return tso.ScriptOutputs
}

// AdhocFunc is a function type for running ad-hoc actions after report generation.
type AdhocFunc func(app.Context, map[string]script.ScriptOutput, target.Target, progress.MultiSpinnerUpdateFunc) error

// ReportingCommand represents a command that generates reports from collected data.
type ReportingCommand struct {
	Cmd                    *cobra.Command
	ReportNamePost         string
	Tables                 []table.TableDefinition
	ScriptParams           map[string]string
	SummaryFunc            app.SummaryFunc
	SummaryTableName       string // e.g., the benchmark or telemetry summary table
	SummaryBeforeTableName string // the name of the table that the summary table should be placed before in the report
	InsightsFunc           app.InsightsFunc
	AdhocFunc              AdhocFunc
	SystemSummaryTableName string // Optional: Only affects xlsx format reports. If set, the table with this name will be used as the "Brief" sheet in the xlsx report. If empty or unset, no "Brief" sheet is generated.
}

// Run is the common flow/logic for all reporting commands, i.e., 'report', 'telemetry', 'flame', 'lock'
// The individual commands populate the ReportingCommand struct with the details specific to the command
// and then call this Run function.
func (rc *ReportingCommand) Run() error {
	// appContext is the application context that holds common data and resources.
	appContext := rc.Cmd.Parent().Context().Value(app.Context{}).(app.Context)
	timestamp := appContext.Timestamp
	localTempDir := appContext.LocalTempDir
	outputDir := appContext.OutputDir
	logFilePath := appContext.LogFilePath
	// create output directory
	err := util.CreateDirectoryIfNotExists(outputDir, 0755) // #nosec G301
	if err != nil {
		err = fmt.Errorf("failed to create output directory: %w", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		rc.Cmd.SilenceUsage = true
		return err
	}

	var myTargets []target.Target
	var orderedTargetScriptOutputs []TargetScriptOutputs
	if app.FlagInput != "" {
		var err error
		orderedTargetScriptOutputs, err = outputsFromInput(rc.Tables, rc.SummaryTableName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			rc.Cmd.SilenceUsage = true
			return err
		}
	} else {
		// get the targets
		var targetErrs []error
		var err error
		myTargets, targetErrs, err = GetTargets(rc.Cmd, elevatedPrivilegesRequired(rc.Tables), false, localTempDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			rc.Cmd.SilenceUsage = true
			return err
		}
		// schedule the cleanup of the temporary directory on each target (if not debugging)
		if rc.Cmd.Parent().PersistentFlags().Lookup("debug").Value.String() != "true" {
			for _, myTarget := range myTargets {
				if myTarget.GetTempDirectory() != "" {
					deferTarget := myTarget // create a new variable to capture the current value
					defer func(deferTarget target.Target) {
						err := deferTarget.RemoveTempDirectory()
						if err != nil {
							slog.Error("error removing target temporary directory", slog.String("error", err.Error()))
						}
					}(deferTarget)
				}
			}
		}
		// setup and start the progress indicator
		multiSpinner := progress.NewMultiSpinner()
		for _, target := range myTargets {
			err := multiSpinner.AddSpinner(target.GetName())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				slog.Error(err.Error())
				rc.Cmd.SilenceUsage = true
				return err
			}
		}
		multiSpinner.Start()
		// remove targets that had errors
		var indicesToRemove []int
		for i := range targetErrs {
			if targetErrs[i] != nil {
				_ = multiSpinner.Status(myTargets[i].GetName(), fmt.Sprintf("Error: %v", targetErrs[i]))
				indicesToRemove = append(indicesToRemove, i)
			}
		}
		for i := len(indicesToRemove) - 1; i >= 0; i-- {
			myTargets = slices.Delete(myTargets, indicesToRemove[i], indicesToRemove[i]+1)
		}
		// set up signal handler to help with cleaning up child processes on ctrl-c/SIGINT or SIGTERM
		configureSignalHandler(myTargets, multiSpinner.Status)
		// collect data from targets
		orderedTargetScriptOutputs, err = outputsFromTargets(rc.Cmd, myTargets, rc.Tables, rc.ScriptParams, multiSpinner.Status, localTempDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			rc.Cmd.SilenceUsage = true
			return err
		}
		// stop the progress indicator
		multiSpinner.Finish()
		fmt.Println()
		// exit with error if no targets remain
		if len(myTargets) == 0 {
			err := fmt.Errorf("no successful targets found")
			slog.Error(err.Error())
			rc.Cmd.SilenceUsage = true
			return err
		}
	}
	// create the raw report before processing the data, so that we can save the raw data even if there is an error while processing
	var rawReports []string
	rawReports, err = rc.createRawReports(appContext, orderedTargetScriptOutputs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		rc.Cmd.SilenceUsage = true
		return err
	}
	// check report formats
	formats := app.FlagFormat
	if slices.Contains(formats, report.FormatAll) {
		formats = report.FormatOptions
	}
	// process the collected data and create the requested report(s)
	reportFilePaths, err := rc.createReports(appContext, orderedTargetScriptOutputs, formats)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		rc.Cmd.SilenceUsage = true
		return err
	}
	// if we are debugging, create a tgz archive with the raw reports, formatted reports, and log file
	if appContext.Debug {
		archiveFiles := append(reportFilePaths, rawReports...)
		if len(archiveFiles) > 0 {
			if logFilePath != "" {
				archiveFiles = append(archiveFiles, logFilePath)
			}
			err := util.CreateFlatTGZ(archiveFiles, filepath.Join(outputDir, app.Name+"_"+timestamp+".tgz"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				slog.Error(err.Error())
				rc.Cmd.SilenceUsage = true
				return err
			}
		}
	}
	if len(reportFilePaths) > 0 {
		fmt.Println("Report files:")
	}
	for _, reportFilePath := range reportFilePaths {
		fmt.Printf("  %s\n", reportFilePath)
	}
	// lastly, run any adhoc actions
	if rc.AdhocFunc != nil {
		fmt.Println()
		// setup and start the progress indicator
		multiSpinner := progress.NewMultiSpinner()
		for _, target := range myTargets {
			err := multiSpinner.AddSpinner(target.GetName())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				slog.Error(err.Error())
				rc.Cmd.SilenceUsage = true
				return err
			}
		}
		multiSpinner.Start()
		adhocErrorChannel := make(chan error)
		for i, t := range myTargets {
			go func(target target.Target, i int) {
				err := rc.AdhocFunc(appContext, orderedTargetScriptOutputs[i].ScriptOutputs, target, multiSpinner.Status)
				adhocErrorChannel <- err
			}(t, i)
		}
		// wait for all adhoc actions to complete, errors were reported by the AdhocFunc
		for range myTargets {
			<-adhocErrorChannel
		}
		// stop the progress indicator
		multiSpinner.Finish()
		fmt.Println()
	}
	return nil
}

// DefaultInsightsFunc returns the insights table values from the table values
func DefaultInsightsFunc(allTableValues []table.TableValues, scriptOutputs map[string]script.ScriptOutput) table.TableValues {
	insightsTableValues := table.TableValues{
		TableDefinition: table.TableDefinition{
			Name:      app.TableNameInsights,
			HasRows:   true,
			MenuLabel: app.TableNameInsights,
		},
		Fields: []table.Field{
			{Name: "Recommendation", Values: []string{}},
			{Name: "Justification", Values: []string{}},
		},
	}
	for _, tableValues := range allTableValues {
		for _, insight := range tableValues.Insights {
			insightsTableValues.Fields[0].Values = append(insightsTableValues.Fields[0].Values, insight.Recommendation)
			insightsTableValues.Fields[1].Values = append(insightsTableValues.Fields[1].Values, insight.Justification)
		}
	}
	return insightsTableValues
}

// FlagValidationError is used to report an error with a flag
func FlagValidationError(cmd *cobra.Command, msg string) error {
	err := fmt.Errorf("%s", msg)
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	fmt.Fprintf(os.Stderr, "See '%s --help' for usage details.\n", cmd.CommandPath())
	cmd.SilenceUsage = true
	return err
}
