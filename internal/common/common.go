// Package common includes functions that are used across the different commands
package common

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"perfspect/internal/progress"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/target"
	"perfspect/internal/util"
	"syscall"

	"slices"

	"github.com/spf13/cobra"
)

var AppName = filepath.Base(os.Args[0])

// AppContext represents the application context that can be accessed from all commands.
type AppContext struct {
	OutputDir      string // OutputDir is the directory where the application will write output files.
	LocalTempDir   string // LocalTempDir is the temp directory on the local host (created by the application).
	TargetTempRoot string // TargetTempRoot is the path to a directory on the target host where the application can create temporary directories.
	Version        string // Version is the version of the application.
}

type Flag struct {
	Name string
	Help string
}
type FlagGroup struct {
	GroupName string
	Flags     []Flag
}

type TargetScriptOutputs struct {
	targetName    string
	scriptOutputs map[string]script.ScriptOutput
	tableNames    []string
}

const (
	TableNameInsights  = "Insights"
	TableNamePerfspect = "PerfSpect Version"
)

type Category struct {
	FlagName     string
	TableNames   []string
	FlagVar      *bool
	DefaultValue bool
	Help         string
}

var (
	FlagInput  string
	FlagFormat []string
)

const (
	FlagInputName  = "input"
	FlagFormatName = "format"
)

type SummaryFunc func([]report.TableValues, map[string]script.ScriptOutput) report.TableValues
type InsightsFunc SummaryFunc

type ReportingCommand struct {
	Cmd              *cobra.Command
	ReportNamePost   string
	TableNames       []string
	ScriptParams     map[string]string
	SummaryFunc      SummaryFunc
	SummaryTableName string
	InsightsFunc     InsightsFunc
}

// Run is the common flow/logic for all reporting commands, i.e., 'report', 'telemetry', 'flame', 'lock'
// The individual commands populate the ReportingCommand struct with the details specific to the command
// and then call this Run function.
func (rc *ReportingCommand) Run() error {
	// appContext is the application context that holds common data and resources.
	appContext := rc.Cmd.Parent().Context().Value(AppContext{}).(AppContext)
	localTempDir := appContext.LocalTempDir
	outputDir := appContext.OutputDir
	// handle signals
	// child processes will exit when the signals are received which will
	// allow this app to exit normally
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChannel
		slog.Info("received signal", slog.String("signal", sig.String()))
		// when perfspect receives ctrl-c while in the shell, the shell makes sure to propogate the
		// signal to all our children. But when perfspect is run in the background or disowned and
		// then receives SIGINT, e.g., from a script, we need to send the signal to our children
		util.SignalChildren(syscall.SIGINT)
	}()
	// get the data we need to generate reports
	orderedTargetScriptOutputs, err := rc.retrieveScriptOutputs(localTempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		rc.Cmd.SilenceUsage = true
		return err
	}
	// we have output data so create the output directory
	err = CreateOutputDir(outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		rc.Cmd.SilenceUsage = true
		return err
	}
	// create the raw report before processing the data, so that we can save the raw data even if there is an error while processing
	err = rc.createRawReports(appContext, orderedTargetScriptOutputs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		rc.Cmd.SilenceUsage = true
		return err
	}
	// check report formats
	formats := FlagFormat
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
	if len(reportFilePaths) > 0 {
		fmt.Println("Report files:")
	}
	for _, reportFilePath := range reportFilePaths {
		fmt.Printf("  %s\n", reportFilePath)
	}
	return nil
}

// CreateOutputDir creates the output directory if it does not exist
func CreateOutputDir(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	return nil
}

// DefaultInsightsFunc returns the insights table values from the table values
func DefaultInsightsFunc(allTableValues []report.TableValues, scriptOutputs map[string]script.ScriptOutput) report.TableValues {
	insightsTableValues := report.TableValues{
		TableDefinition: report.TableDefinition{
			Name:      TableNameInsights,
			HasRows:   true,
			MenuLabel: TableNameInsights,
		},
		Fields: []report.Field{
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

// createRawReports creates the raw report(s) from the collected data
func (rc *ReportingCommand) createRawReports(appContext AppContext, orderedTargetScriptOutputs []TargetScriptOutputs) error {
	for _, targetScriptOutputs := range orderedTargetScriptOutputs {
		reportBytes, err := report.CreateRawReport(rc.TableNames, targetScriptOutputs.scriptOutputs, targetScriptOutputs.targetName)
		if err != nil {
			err = fmt.Errorf("failed to create raw report: %w", err)
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
			slog.Error(err.Error())
			rc.Cmd.SilenceUsage = true
			return err
		}
		post := ""
		if rc.ReportNamePost != "" {
			post = "_" + rc.ReportNamePost
		}
		reportFilename := fmt.Sprintf("%s%s.%s", targetScriptOutputs.targetName, post, "raw")
		reportPath := filepath.Join(appContext.OutputDir, reportFilename)
		if err = report.WriteReport(reportBytes, reportPath); err != nil {
			err = fmt.Errorf("failed to write report: %w", err)
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
			slog.Error(err.Error())
			rc.Cmd.SilenceUsage = true
			return err
		}
	}
	return nil
}

// createReports processes the collected data and creates the requested report(s)
func (rc *ReportingCommand) createReports(appContext AppContext, orderedTargetScriptOutputs []TargetScriptOutputs, formats []string) ([]string, error) {
	reportFilePaths := []string{}
	allTargetsTableValues := make([][]report.TableValues, 0)
	for _, targetScriptOutputs := range orderedTargetScriptOutputs {
		// process the tables, i.e., get field values from script output
		allTableValues, err := report.Process(targetScriptOutputs.tableNames, targetScriptOutputs.scriptOutputs)
		if err != nil {
			err = fmt.Errorf("failed to process collected data: %w", err)
			return nil, err
		}
		// special case - the summary table is built from the post-processed data, i.e., table values
		if rc.SummaryFunc != nil {
			// override the menu label for the System Summary table to avoid conflict with performance summary table added below
			for i, tv := range allTableValues {
				if tv.Name == report.SystemSummaryTableName {
					allTableValues[i].MenuLabel = "System Summary"
				}
			}
			summaryTableValues := rc.SummaryFunc(allTableValues, targetScriptOutputs.scriptOutputs)
			allTableValues = append(allTableValues, summaryTableValues)
		}
		// special case - add tableValues for Insights
		if rc.InsightsFunc != nil {
			insightsTableValues := rc.InsightsFunc(allTableValues, targetScriptOutputs.scriptOutputs)
			allTableValues = append(allTableValues, insightsTableValues)
		}
		// special case - add tableValues for the application version
		allTableValues = append(allTableValues, report.TableValues{
			TableDefinition: report.TableDefinition{
				Name: TableNamePerfspect,
			},
			Fields: []report.Field{
				{Name: "Version", Values: []string{appContext.Version}},
			},
		})
		// create the report(s)
		for _, format := range formats {
			reportBytes, err := report.Create(format, allTableValues, targetScriptOutputs.scriptOutputs, targetScriptOutputs.targetName)
			if err != nil {
				err = fmt.Errorf("failed to create report: %w", err)
				return nil, err
			}
			if len(formats) == 1 && format == report.FormatTxt {
				fmt.Printf("%s:\n", targetScriptOutputs.targetName)
				fmt.Print(string(reportBytes))
			}
			post := ""
			if rc.ReportNamePost != "" {
				post = "_" + rc.ReportNamePost
			}
			reportFilename := fmt.Sprintf("%s%s.%s", targetScriptOutputs.targetName, post, format)
			reportPath := filepath.Join(appContext.OutputDir, reportFilename)
			if err = report.WriteReport(reportBytes, reportPath); err != nil {
				err = fmt.Errorf("failed to write report: %w", err)
				return nil, err
			}
			reportFilePaths = append(reportFilePaths, reportPath)
		}
		// keep all the targets table values for combined reports
		allTargetsTableValues = append(allTargetsTableValues, allTableValues)
	}
	if len(allTargetsTableValues) > 1 {
		// list of target names for the combined report
		// - only those that we received output from
		targetNames := make([]string, 0)
		for _, targetScriptOutputs := range orderedTargetScriptOutputs {
			targetNames = append(targetNames, targetScriptOutputs.targetName)
		}
		multiTargetFormats := []string{report.FormatHtml, report.FormatXlsx}
		for _, format := range multiTargetFormats {
			if !slices.Contains(formats, format) {
				continue
			}
			reportBytes, err := report.CreateMultiTarget(format, allTargetsTableValues, targetNames, rc.TableNames)
			if err != nil {
				err = fmt.Errorf("failed to create multi-target %s report: %w", format, err)
				return nil, err
			}
			reportFilename := fmt.Sprintf("%s.%s", "all_hosts", format)
			reportPath := filepath.Join(appContext.OutputDir, reportFilename)
			if err = report.WriteReport(reportBytes, reportPath); err != nil {
				err = fmt.Errorf("failed to write multi-target %s report: %w", format, err)
				return nil, err
			}
			reportFilePaths = append(reportFilePaths, reportPath)
		}
	}
	return reportFilePaths, nil
}

// retrieveScriptOutputs gets the data from the targets or from the input file(s)
func (rc *ReportingCommand) retrieveScriptOutputs(localTempDir string) ([]TargetScriptOutputs, error) {
	var orderedTargetScriptOutputs []TargetScriptOutputs
	// check if we are reading from a file or running on targets
	if FlagInput != "" {
		var err error
		orderedTargetScriptOutputs, err = outputsFromInput(rc.SummaryTableName)
		if err != nil {
			return nil, err
		}
	} else {
		// get the targets
		myTargets, targetErrs, err := GetTargets(rc.Cmd, elevatedPrivilegesRequired(rc.TableNames), false, localTempDir)
		if err != nil {
			return nil, err
		}
		// schedule the cleanup of the temporary directory on each target (if not debugging)
		if rc.Cmd.Parent().PersistentFlags().Lookup("debug").Value.String() != "true" {
			for _, myTarget := range myTargets {
				if myTarget.GetTempDirectory() != "" {
					defer func() {
						err := myTarget.RemoveTempDirectory()
						if err != nil {
							slog.Error("error removing target temporary directory", slog.String("error", err.Error()))
						}
					}()
				}
			}
		}

		// setup and start the progress indicator
		multiSpinner := progress.NewMultiSpinner()
		for _, target := range myTargets {
			err := multiSpinner.AddSpinner(target.GetName())
			if err != nil {
				return nil, err
			}
		}
		multiSpinner.Start()
		defer multiSpinner.Finish()
		// check for errors in target creation
		for i := range targetErrs {
			if targetErrs[i] != nil {
				_ = multiSpinner.Status(myTargets[i].GetName(), fmt.Sprintf("Error: %v", targetErrs[i]))
				// remove target from targets list
				myTargets = slices.Delete(myTargets, i, i+1)
			}
		}
		// check if we have any remaining targets to run the scripts on
		if len(myTargets) == 0 {
			err := fmt.Errorf("no targets remain")
			return nil, err
		}
		orderedTargetScriptOutputs, err = outputsFromTargets(myTargets, rc, multiSpinner.Status, localTempDir)
		if err != nil {
			return nil, err
		}
		fmt.Println()
	}
	return orderedTargetScriptOutputs, nil
}

// outputsFromInput reads the raw file(s) and returns the data in the order of the raw files
func outputsFromInput(summaryTableName string) ([]TargetScriptOutputs, error) {
	orderedTargetScriptOutputs := []TargetScriptOutputs{}
	tableNames := []string{} // use the table names from the raw files
	// read the raw file(s) as JSON
	rawReports, err := report.ReadRawReports(FlagInput)
	if err != nil {
		err = fmt.Errorf("failed to read raw file(s): %w", err)
		return nil, err
	}
	for _, rawReport := range rawReports {
		for _, tableName := range rawReport.TableNames { // just in case someone tries to use the raw files that were collected with a different set of categories
			// filter out tables that we add after processing
			if tableName == TableNameInsights || tableName == TableNamePerfspect || tableName == summaryTableName {
				continue
			}
			tableNames = util.UniqueAppend(tableNames, tableName)
		}
		orderedTargetScriptOutputs = append(orderedTargetScriptOutputs, TargetScriptOutputs{targetName: rawReport.TargetName, scriptOutputs: rawReport.ScriptOutputs, tableNames: tableNames})
	}
	return orderedTargetScriptOutputs, nil
}

// outputsFromTargets runs the scripts on the targets and returns the data in the order of the targets
func outputsFromTargets(myTargets []target.Target, rc *ReportingCommand, statusUpdate progress.MultiSpinnerUpdateFunc, localTempDir string) ([]TargetScriptOutputs, error) {
	orderedTargetScriptOutputs := []TargetScriptOutputs{}
	channelTargetScriptOutputs := make(chan TargetScriptOutputs)
	channelError := make(chan error)
	// create the list of tables and associated scripts for each target
	targetTableNames := [][]string{}
	targetScriptNames := [][]string{}
	for targetIdx, target := range myTargets {
		targetTableNames = append(targetTableNames, []string{})
		targetScriptNames = append(targetScriptNames, []string{})
		for _, tableName := range rc.TableNames {
			if report.TableForTarget(tableName, target) {
				// add table to list of tables to collect
				targetTableNames[targetIdx] = util.UniqueAppend(targetTableNames[targetIdx], tableName)
				// add scripts to list of scripts to run
				for _, scriptName := range report.GetScriptNamesForTable(tableName) {
					targetScriptNames[targetIdx] = util.UniqueAppend(targetScriptNames[targetIdx], scriptName)
				}
			} else {
				slog.Info("table not supported for target", slog.String("table", tableName), slog.String("target", target.GetName()))
			}
		}
	}
	// run the scripts on the targets
	for targetIdx, target := range myTargets {
		scriptsToRunOnTarget := []script.ScriptDefinition{}
		for _, scriptName := range targetScriptNames[targetIdx] {
			script := script.GetParameterizedScriptByName(scriptName, rc.ScriptParams)
			scriptsToRunOnTarget = append(scriptsToRunOnTarget, script)
		}
		// run the selected scripts on the target
		go collectOnTarget(rc.Cmd, rc.ScriptParams["Duration"], target, scriptsToRunOnTarget, localTempDir, channelTargetScriptOutputs, channelError, statusUpdate)
	}
	// wait for scripts to run on all targets
	var allTargetScriptOutputs []TargetScriptOutputs
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
	for targetIdx, target := range myTargets {
		for _, targetScriptOutputs := range allTargetScriptOutputs {
			if targetScriptOutputs.targetName == target.GetName() {
				targetScriptOutputs.tableNames = targetTableNames[targetIdx]
				orderedTargetScriptOutputs = append(orderedTargetScriptOutputs, targetScriptOutputs)
				break
			}
		}
	}
	return orderedTargetScriptOutputs, nil
}

// elevatedPrivilegesRequired returns true if any of the scripts needed for the tables require elevated privileges
func elevatedPrivilegesRequired(tableNames []string) bool {
	for _, tableName := range tableNames {
		// add scripts to list of scripts to run
		for _, scriptName := range report.GetScriptNamesForTable(tableName) {
			script := script.GetScriptByName(scriptName)
			if script.Superuser {
				return true
			}
		}
	}
	return false
}

// collectOnTarget runs the scripts on the target and sends the results to the appropriate channels
func collectOnTarget(cmd *cobra.Command, duration string, myTarget target.Target, scriptsToRun []script.ScriptDefinition, localTempDir string, channelTargetScriptOutputs chan TargetScriptOutputs, channelError chan error, statusUpdate progress.MultiSpinnerUpdateFunc) {
	// run the scripts on the target
	status := "collecting data"
	if cmd.Name() == "telemetry" && duration == "0" { // telemetry is the only command that uses this common code that can run indefinitely
		status += ", press Ctrl+c to stop"
	} else if duration != "0" {
		status += fmt.Sprintf(" for %s seconds", duration)
	}
	_ = statusUpdate(myTarget.GetName(), status)
	scriptOutputs, err := script.RunScripts(myTarget, scriptsToRun, true, localTempDir)
	if err != nil {
		_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("error collecting data: %v", err))
		err = fmt.Errorf("error running data collection scripts on %s: %v", myTarget.GetName(), err)
		channelError <- err
		return
	}
	_ = statusUpdate(myTarget.GetName(), "collection complete")
	channelTargetScriptOutputs <- TargetScriptOutputs{targetName: myTarget.GetName(), scriptOutputs: scriptOutputs}
}
