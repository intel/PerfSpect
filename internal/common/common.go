// Package common defines data structures and functions that are used by multiple
// application commands, e.g., report, telemetry, flame, lock.
package common

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"errors"
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
	"strings"
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
	TargetName    string
	ScriptOutputs map[string]script.ScriptOutput
	TableNames    []string
}

func (tso *TargetScriptOutputs) GetScriptOutputs() map[string]script.ScriptOutput {
	return tso.ScriptOutputs
}
func (tso *TargetScriptOutputs) GetTableNames() []string {
	return tso.TableNames
}

const (
	TableNameInsights  = "Insights"
	TableNamePerfspect = "PerfSpect"
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
type AdhocFunc func(AppContext, map[string]script.ScriptOutput, target.Target, progress.MultiSpinnerUpdateFunc) error

type ReportingCommand struct {
	Cmd                    *cobra.Command
	ReportNamePost         string
	TableNames             []string
	ScriptParams           map[string]string
	SummaryFunc            SummaryFunc
	SummaryTableName       string
	SummaryBeforeTableName string // the name of the table that the summary table should be placed before in the report
	InsightsFunc           InsightsFunc
	AdhocFunc              AdhocFunc
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
		err := util.SignalChildren(syscall.SIGINT)
		if err != nil {
			slog.Error("error sending signal to children", slog.String("error", err.Error()))
		}
	}()

	var orderedTargetScriptOutputs []TargetScriptOutputs
	var myTargets []target.Target
	if FlagInput != "" {
		var err error
		orderedTargetScriptOutputs, err = outputsFromInput(rc.SummaryTableName)
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
		myTargets, targetErrs, err = GetTargets(rc.Cmd, elevatedPrivilegesRequired(rc.TableNames), false, localTempDir)
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
		// collect data from targets
		orderedTargetScriptOutputs, err = outputsFromTargets(rc.Cmd, myTargets, rc.TableNames, rc.ScriptParams, multiSpinner.Status, localTempDir)
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
	// we have output data so create the output directory
	err := CreateOutputDir(outputDir)
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

// CreateOutputDir creates the output directory if it does not exist
func CreateOutputDir(outputDir string) error {
	err := os.MkdirAll(outputDir, 0755) // #nosec G301
	if err != nil {
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

// FlagValidationError is used to report an error with a flag
func FlagValidationError(cmd *cobra.Command, msg string) error {
	err := errors.New(msg)
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	fmt.Fprintf(os.Stderr, "See '%s --help' for usage details.\n", cmd.CommandPath())
	cmd.SilenceUsage = true
	return err
}

// createRawReports creates the raw report(s) from the collected data
func (rc *ReportingCommand) createRawReports(appContext AppContext, orderedTargetScriptOutputs []TargetScriptOutputs) error {
	for _, targetScriptOutputs := range orderedTargetScriptOutputs {
		reportBytes, err := report.CreateRawReport(rc.TableNames, targetScriptOutputs.ScriptOutputs, targetScriptOutputs.TargetName)
		if err != nil {
			err = fmt.Errorf("failed to create raw report: %w", err)
			return err
		}
		post := ""
		if rc.ReportNamePost != "" {
			post = "_" + rc.ReportNamePost
		}
		reportFilename := fmt.Sprintf("%s%s.%s", targetScriptOutputs.TargetName, post, "raw")
		reportPath := filepath.Join(appContext.OutputDir, reportFilename)
		if err = writeReport(reportBytes, reportPath); err != nil {
			err = fmt.Errorf("failed to write report: %w", err)
			return err
		}
	}
	return nil
}

// writeReport writes the report bytes to the specified path.
func writeReport(reportBytes []byte, reportPath string) error {
	err := os.WriteFile(reportPath, reportBytes, 0644) // #nosec G306
	if err != nil {
		err = fmt.Errorf("failed to write report file: %v", err)
		fmt.Fprintln(os.Stderr, err)
		slog.Error(err.Error())
		return err
	}
	return nil
}

// createReports processes the collected data and creates the requested report(s)
func (rc *ReportingCommand) createReports(appContext AppContext, orderedTargetScriptOutputs []TargetScriptOutputs, formats []string) ([]string, error) {
	reportFilePaths := []string{}
	allTargetsTableValues := make([][]report.TableValues, 0)
	for _, targetScriptOutputs := range orderedTargetScriptOutputs {
		// process the tables, i.e., get field values from script output
		allTableValues, err := report.ProcessTables(targetScriptOutputs.TableNames, targetScriptOutputs.ScriptOutputs)
		if err != nil {
			err = fmt.Errorf("failed to process collected data: %w", err)
			return nil, err
		}
		// special case - the summary table is built from the post-processed data, i.e., table values
		if rc.SummaryFunc != nil {
			summaryTableValues := rc.SummaryFunc(allTableValues, targetScriptOutputs.ScriptOutputs)
			// insert the summary table before the table specified by SummaryBeforeTableName, otherwise append it at the end
			summaryBeforeTableFound := false
			if rc.SummaryBeforeTableName != "" {
				for i, tableValues := range allTableValues {
					if tableValues.TableDefinition.Name == rc.SummaryBeforeTableName {
						summaryBeforeTableFound = true
						// insert the summary table before this table
						allTableValues = append(allTableValues[:i], append([]report.TableValues{summaryTableValues}, allTableValues[i:]...)...)
						break
					}
				}
			}
			if !summaryBeforeTableFound {
				// append the summary table at the end
				allTableValues = append(allTableValues, summaryTableValues)
			}
		}
		// special case - add tableValues for Insights
		if rc.InsightsFunc != nil {
			insightsTableValues := rc.InsightsFunc(allTableValues, targetScriptOutputs.ScriptOutputs)
			allTableValues = append(allTableValues, insightsTableValues)
		}
		// special case - add tableValues for the application version
		allTableValues = append(allTableValues, report.TableValues{
			TableDefinition: report.TableDefinition{
				Name: TableNamePerfspect,
			},
			Fields: []report.Field{
				{Name: "Version", Values: []string{appContext.Version}},
				{Name: "Args", Values: []string{strings.Join(os.Args, " ")}},
				{Name: "OutputDir", Values: []string{appContext.OutputDir}},
			},
		})
		// create the report(s)
		for _, format := range formats {
			reportBytes, err := report.Create(format, allTableValues, targetScriptOutputs.ScriptOutputs, targetScriptOutputs.TargetName)
			if err != nil {
				err = fmt.Errorf("failed to create report: %w", err)
				return nil, err
			}
			if len(formats) == 1 && format == report.FormatTxt {
				fmt.Printf("%s:\n", targetScriptOutputs.TargetName)
				fmt.Print(string(reportBytes))
			}
			post := ""
			if rc.ReportNamePost != "" {
				post = "_" + rc.ReportNamePost
			}
			reportFilename := fmt.Sprintf("%s%s.%s", targetScriptOutputs.TargetName, post, format)
			reportPath := filepath.Join(appContext.OutputDir, reportFilename)
			if err = writeReport(reportBytes, reportPath); err != nil {
				err = fmt.Errorf("failed to write report: %w", err)
				return nil, err
			}
			reportFilePaths = append(reportFilePaths, reportPath)
		}
		// keep all the targets table values for combined reports
		allTargetsTableValues = append(allTargetsTableValues, allTableValues)
	}
	if len(allTargetsTableValues) > 1 && len(orderedTargetScriptOutputs) > 1 {
		// list of target names for the combined report
		// - only those that we received output from
		targetNames := make([]string, 0)
		for _, targetScriptOutputs := range orderedTargetScriptOutputs {
			targetNames = append(targetNames, targetScriptOutputs.TargetName)
		}
		// merge table names from all targets maintaining the order of the tables
		mergedTableNames := util.MergeOrderedUnique(extractTableNamesFromValues(allTargetsTableValues))
		multiTargetFormats := []string{report.FormatHtml, report.FormatXlsx}
		for _, format := range multiTargetFormats {
			if !slices.Contains(formats, format) {
				continue
			}
			reportBytes, err := report.CreateMultiTarget(format, allTargetsTableValues, targetNames, mergedTableNames)
			if err != nil {
				err = fmt.Errorf("failed to create multi-target %s report: %w", format, err)
				return nil, err
			}
			reportFilename := fmt.Sprintf("%s.%s", "all_hosts", format)
			reportPath := filepath.Join(appContext.OutputDir, reportFilename)
			if err = writeReport(reportBytes, reportPath); err != nil {
				err = fmt.Errorf("failed to write multi-target %s report: %w", format, err)
				return nil, err
			}
			reportFilePaths = append(reportFilePaths, reportPath)
		}
	}
	return reportFilePaths, nil
}

// extractTableNamesFromValues extracts the table names from the processed table values for each target.
// It returns a slice of slices, where each inner slice contains the table names for a target.
func extractTableNamesFromValues(allTargetsTableValues [][]report.TableValues) [][]string {
	targetTableNames := make([][]string, 0, len(allTargetsTableValues))
	for _, tableValues := range allTargetsTableValues {
		names := make([]string, 0, len(tableValues))
		for _, tv := range tableValues {
			names = append(names, tv.TableDefinition.Name)
		}
		targetTableNames = append(targetTableNames, names)
	}
	return targetTableNames
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
		orderedTargetScriptOutputs = append(orderedTargetScriptOutputs, TargetScriptOutputs{TargetName: rawReport.TargetName, ScriptOutputs: rawReport.ScriptOutputs, TableNames: tableNames})
	}
	return orderedTargetScriptOutputs, nil
}

// outputsFromTargets runs the scripts on the targets and returns the data in the order of the targets
func outputsFromTargets(cmd *cobra.Command, myTargets []target.Target, tableNames []string, scriptParams map[string]string, statusUpdate progress.MultiSpinnerUpdateFunc, localTempDir string) ([]TargetScriptOutputs, error) {
	orderedTargetScriptOutputs := []TargetScriptOutputs{}
	channelTargetScriptOutputs := make(chan TargetScriptOutputs)
	channelError := make(chan error)
	// create the list of tables and associated scripts for each target
	targetTableNames := [][]string{}
	targetScriptNames := [][]string{}
	for targetIdx, target := range myTargets {
		targetTableNames = append(targetTableNames, []string{})
		targetScriptNames = append(targetScriptNames, []string{})
		for _, tableName := range tableNames {
			if report.IsTableForTarget(tableName, target) {
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
			script := script.GetParameterizedScriptByName(scriptName, scriptParams)
			scriptsToRunOnTarget = append(scriptsToRunOnTarget, script)
		}
		// run the selected scripts on the target
		go collectOnTarget(target, scriptsToRunOnTarget, localTempDir, scriptParams["Duration"], cmd.Name() == "telemetry", channelTargetScriptOutputs, channelError, statusUpdate)
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
			if targetScriptOutputs.TargetName == target.GetName() {
				targetScriptOutputs.TableNames = targetTableNames[targetIdx]
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
func collectOnTarget(myTarget target.Target, scriptsToRun []script.ScriptDefinition, localTempDir string, duration string, isTelemetry bool, channelTargetScriptOutputs chan TargetScriptOutputs, channelError chan error, statusUpdate progress.MultiSpinnerUpdateFunc) {
	// run the scripts on the target
	status := "collecting data"
	if isTelemetry && duration == "0" { // telemetry is the only command that uses this common code that can run indefinitely
		status += ", press Ctrl+c to stop"
	} else if duration != "0" && duration != "" {
		status += fmt.Sprintf(" for %s seconds", duration)
	}
	if statusUpdate != nil {
		_ = statusUpdate(myTarget.GetName(), status)
	}
	scriptOutputs, err := script.RunScripts(myTarget, scriptsToRun, true, localTempDir)
	if err != nil {
		if statusUpdate != nil {
			_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("error collecting data: %v", err))
		}
		err = fmt.Errorf("error running data collection scripts on %s: %v", myTarget.GetName(), err)
		channelError <- err
		return
	}
	if statusUpdate != nil {
		_ = statusUpdate(myTarget.GetName(), "collection complete")
	}
	channelTargetScriptOutputs <- TargetScriptOutputs{TargetName: myTarget.GetName(), ScriptOutputs: scriptOutputs}
}
