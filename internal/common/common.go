// Package common defines data structures and functions that are used by multiple
// application commands, e.g., report, telemetry, flame, lock.
package common

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"perfspect/internal/progress"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/table"
	"perfspect/internal/target"
	"perfspect/internal/util"
	"strings"
	"syscall"
	"time"

	"slices"

	"github.com/spf13/cobra"
)

// Flag names for flags defined in the root command, but sometimes used in other commands.
const (
	FlagDebugName          = "debug"
	FlagSyslogName         = "syslog"
	FlagLogStdOutName      = "log-stdout"
	FlagOutputDirName      = "output"
	FlagTargetTempRootName = "tempdir"
	FlagNoCheckUpdateName  = "noupdate"
)

var AppName = filepath.Base(os.Args[0])

// AppContext represents the application context that can be accessed from all commands.
type AppContext struct {
	Timestamp      string // Timestamp is the timestamp when the application was started.
	OutputDir      string // OutputDir is the directory where the application will write output files.
	LocalTempDir   string // LocalTempDir is the temp directory on the local host (created by the application).
	LogFilePath    string // LogFilePath is the path to the log file.
	TargetTempRoot string // TargetTempRoot is the path to a directory on the target host where the application can create temporary directories.
	Version        string // Version is the version of the application.
	Debug          bool   // Debug is true if the application is running in debug mode.
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
	Tables        []table.TableDefinition
}

func (tso *TargetScriptOutputs) GetScriptOutputs() map[string]script.ScriptOutput {
	return tso.ScriptOutputs
}

const (
	TableNameInsights  = "Insights"
	TableNamePerfspect = "PerfSpect"
)

type Category struct {
	FlagName     string
	Tables       []table.TableDefinition
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

type SummaryFunc func([]table.TableValues, map[string]script.ScriptOutput) table.TableValues
type InsightsFunc SummaryFunc
type AdhocFunc func(AppContext, map[string]script.ScriptOutput, target.Target, progress.MultiSpinnerUpdateFunc) error

type ReportingCommand struct {
	Cmd                    *cobra.Command
	ReportNamePost         string
	Tables                 []table.TableDefinition
	ScriptParams           map[string]string
	SummaryFunc            SummaryFunc
	SummaryTableName       string // e.g., the benchmark or telemetry summary table
	SummaryBeforeTableName string // the name of the table that the summary table should be placed before in the report
	InsightsFunc           InsightsFunc
	AdhocFunc              AdhocFunc
	SystemSummaryTableName string // Optional: Only affects xlsx format reports. If set, the table with this name will be used as the "Brief" sheet in the xlsx report. If empty or unset, no "Brief" sheet is generated.
}

// Run is the common flow/logic for all reporting commands, i.e., 'report', 'telemetry', 'flame', 'lock'
// The individual commands populate the ReportingCommand struct with the details specific to the command
// and then call this Run function.
func (rc *ReportingCommand) Run() error {
	// appContext is the application context that holds common data and resources.
	appContext := rc.Cmd.Parent().Context().Value(AppContext{}).(AppContext)
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
	if FlagInput != "" {
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
	// if we are debugging, create a tgz archive with the raw reports, formatted reports, and log file
	if appContext.Debug {
		archiveFiles := append(reportFilePaths, rawReports...)
		if len(archiveFiles) > 0 {
			if logFilePath != "" {
				archiveFiles = append(archiveFiles, logFilePath)
			}
			err := util.CreateFlatTGZ(archiveFiles, filepath.Join(outputDir, AppName+"_"+timestamp+".tgz"))
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

func signalProcessOnTarget(t target.Target, pidStr string, sigStr string) error {
	var cmd *exec.Cmd
	// prepend "-" to the signal string if not already present
	if !strings.HasPrefix(sigStr, "-") {
		sigStr = "-" + sigStr
	}
	if !t.IsSuperUser() && t.CanElevatePrivileges() {
		cmd = exec.Command("sudo", "kill", sigStr, pidStr)
	} else {
		cmd = exec.Command("kill", sigStr, pidStr)
	}
	_, _, _, err := t.RunCommandEx(cmd, 5, false, true) // #nosec G204
	return err
}

// configureSignalHandler sets up a signal handler to catch SIGINT and SIGTERM
//
// When perfspect receives ctrl-c while in the shell, the shell propagates the
// signal to all our children. But when perfspect is run in the background or disowned and
// then receives SIGINT, e.g., from a script, we need to send the signal to our children
//
// When running scripts using the controller.sh script, we need to send the signal to the
// controller.sh script on each target so that it can clean up its child processes. This is
// because the controller.sh script is run in its own process group and does not receive the
// signal when perfspect receives it.
//
// Parameters:
//   - myTargets: The list of targets to send the signal to.
//   - statusFunc: A function to update the status of the progress indicator.
func configureSignalHandler(myTargets []target.Target, statusFunc progress.MultiSpinnerUpdateFunc) {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChannel
		slog.Debug("received signal", slog.String("signal", sig.String()))
		// The controller.sh script is run in its own process group, so we need to send the signal
		// directly to the PID of the controller. For every target, look for the primary_collection_script
		// PID file and send SIGINT to it.
		// The controller script is run in its own process group, so we need to send the signal
		// directly to the PID of the controller. For every target, look for the controller
		// PID file and send SIGINT to it.
		for _, t := range myTargets {
			if statusFunc != nil {
				_ = statusFunc(t.GetName(), "Signal received, cleaning up...")
			}
			pidFilePath := filepath.Join(t.GetTempDirectory(), script.ControllerPIDFileName)
			stdout, _, exitcode, err := t.RunCommandEx(exec.Command("cat", pidFilePath), 5, false, true) // #nosec G204
			if err != nil {
				slog.Error("error retrieving target controller PID", slog.String("target", t.GetName()), slog.String("error", err.Error()))
			}
			if exitcode == 0 {
				pidStr := strings.TrimSpace(stdout)
				err = signalProcessOnTarget(t, pidStr, "SIGINT")
				if err != nil {
					slog.Error("error sending SIGINT signal to target controller", slog.String("target", t.GetName()), slog.String("error", err.Error()))
				}
			}
		}
		// now wait until all controller scripts have exited
		slog.Debug("waiting for controller scripts to exit")
		for _, t := range myTargets {
			// create a per-target timeout context
			targetTimeout := 10 * time.Second
			ctx, cancel := context.WithTimeout(context.Background(), targetTimeout)
			timedOut := false
			pidFilePath := filepath.Join(t.GetTempDirectory(), script.ControllerPIDFileName)
			for {
				// read the pid file
				stdout, _, exitcode, err := t.RunCommandEx(exec.Command("cat", pidFilePath), 5, false, true) // #nosec G204
				if err != nil || exitcode != 0 {
					// pid file doesn't exist
					break
				}
				pidStr := strings.TrimSpace(stdout)
				// determine if the process still exists
				_, _, exitcode, err = t.RunCommandEx(exec.Command("ps", "-p", pidStr), 5, false, true) // #nosec G204
				if err != nil || exitcode != 0 {
					break // process no longer exists, script has exited
				}
				// check for timeout
				select {
				case <-ctx.Done():
					timedOut = true
				default:
				}
				if timedOut {
					if statusFunc != nil {
						_ = statusFunc(t.GetName(), "cleanup timeout exceeded, sending kill signal")
					}
					slog.Warn("signal handler cleanup timeout exceeded for target, sending SIGKILL", slog.String("target", t.GetName()))
					err = signalProcessOnTarget(t, pidStr, "SIGKILL")
					if err != nil {
						slog.Error("error sending SIGKILL signal to target controller", slog.String("target", t.GetName()), slog.String("error", err.Error()))
					}
					break
				}
				// sleep for a short time before checking again
				time.Sleep(500 * time.Millisecond)
			}
			cancel()
		}

		// send SIGINT to perfspect's children
		err := util.SignalChildren(syscall.SIGINT)
		if err != nil {
			slog.Error("error sending signal to children", slog.String("error", err.Error()))
		}
	}()
}

// DefaultInsightsFunc returns the insights table values from the table values
func DefaultInsightsFunc(allTableValues []table.TableValues, scriptOutputs map[string]script.ScriptOutput) table.TableValues {
	insightsTableValues := table.TableValues{
		TableDefinition: table.TableDefinition{
			Name:      TableNameInsights,
			HasRows:   true,
			MenuLabel: TableNameInsights,
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
	err := errors.New(msg)
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	fmt.Fprintf(os.Stderr, "See '%s --help' for usage details.\n", cmd.CommandPath())
	cmd.SilenceUsage = true
	return err
}

// createRawReports creates the raw report(s) from the collected data
// returns the list of report files creates or an error if the report creation failed.
func (rc *ReportingCommand) createRawReports(appContext AppContext, orderedTargetScriptOutputs []TargetScriptOutputs) ([]string, error) {
	var reports []string
	for _, targetScriptOutputs := range orderedTargetScriptOutputs {
		reportBytes, err := report.CreateRawReport(rc.Tables, targetScriptOutputs.ScriptOutputs, targetScriptOutputs.TargetName)
		if err != nil {
			err = fmt.Errorf("failed to create raw report: %w", err)
			return reports, err
		}
		post := ""
		if rc.ReportNamePost != "" {
			post = "_" + rc.ReportNamePost
		}
		reportFilename := fmt.Sprintf("%s%s.%s", targetScriptOutputs.TargetName, post, "raw")
		reportPath := filepath.Join(appContext.OutputDir, reportFilename)
		if err = writeReport(reportBytes, reportPath); err != nil {
			err = fmt.Errorf("failed to write report: %w", err)
			return reports, err
		}
		reports = append(reports, reportPath)
	}
	return reports, nil
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
	allTargetsTableValues := make([][]table.TableValues, 0)
	for _, targetScriptOutputs := range orderedTargetScriptOutputs {
		// process the tables, i.e., get field values from script output
		allTableValues, err := table.ProcessTables(targetScriptOutputs.Tables, targetScriptOutputs.ScriptOutputs)
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
						allTableValues = append(allTableValues[:i], append([]table.TableValues{summaryTableValues}, allTableValues[i:]...)...)
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
		allTableValues = append(allTableValues, table.TableValues{
			TableDefinition: table.TableDefinition{
				Name: TableNamePerfspect,
			},
			Fields: []table.Field{
				{Name: "Version", Values: []string{appContext.Version}},
				{Name: "Args", Values: []string{strings.Join(os.Args, " ")}},
				{Name: "OutputDir", Values: []string{appContext.OutputDir}},
			},
		})
		// create the report(s)
		for _, format := range formats {
			reportBytes, err := report.Create(format, allTableValues, targetScriptOutputs.TargetName, rc.SystemSummaryTableName)
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
			reportBytes, err := report.CreateMultiTarget(format, allTargetsTableValues, targetNames, mergedTableNames, rc.SummaryTableName)
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
func extractTableNamesFromValues(allTargetsTableValues [][]table.TableValues) [][]string {
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

func findTableByName(tables []table.TableDefinition, name string) (*table.TableDefinition, error) {
	for _, tbl := range tables {
		if tbl.Name == name {
			return &tbl, nil
		}
	}
	return nil, fmt.Errorf("table [%s] not found", name)
}

// outputsFromInput reads the raw file(s) and returns the data in the order of the raw files
func outputsFromInput(tables []table.TableDefinition, summaryTableName string) ([]TargetScriptOutputs, error) {
	orderedTargetScriptOutputs := []TargetScriptOutputs{}
	includedTables := []table.TableDefinition{}
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
			includedTable, err := findTableByName(tables, tableName)
			if err != nil {
				slog.Warn("table from raw report not found in current tables", slog.String("table", tableName), slog.String("target", rawReport.TargetName))
				continue
			}
			includedTables = append(includedTables, *includedTable)
		}
		orderedTargetScriptOutputs = append(orderedTargetScriptOutputs, TargetScriptOutputs{TargetName: rawReport.TargetName, ScriptOutputs: rawReport.ScriptOutputs, Tables: includedTables})
	}
	return orderedTargetScriptOutputs, nil
}

// outputsFromTargets runs the scripts on the targets and returns the data in the order of the targets
func outputsFromTargets(cmd *cobra.Command, myTargets []target.Target, tables []table.TableDefinition, scriptParams map[string]string, statusUpdate progress.MultiSpinnerUpdateFunc, localTempDir string) ([]TargetScriptOutputs, error) {
	orderedTargetScriptOutputs := []TargetScriptOutputs{}
	channelTargetScriptOutputs := make(chan TargetScriptOutputs)
	channelError := make(chan error)
	// create the list of tables and associated scripts for each target
	targetTables := [][]table.TableDefinition{}
	targetScriptNames := [][]string{}
	for targetIdx, target := range myTargets {
		targetTables = append(targetTables, []table.TableDefinition{})
		targetScriptNames = append(targetScriptNames, []string{})
		for _, tbl := range tables {
			if isTableForTarget(tbl, target, localTempDir) {
				// add table to list of tables to collect
				targetTables[targetIdx] = append(targetTables[targetIdx], tbl)
				// add scripts to list of scripts to run
				for _, scriptName := range tbl.ScriptNames {
					targetScriptNames[targetIdx] = util.UniqueAppend(targetScriptNames[targetIdx], scriptName)
				}
			} else {
				slog.Debug("table not supported for target", slog.String("table", tbl.Name), slog.String("target", target.GetName()))
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
		ctrlCToStop := cmd.Name() == "telemetry" || cmd.Name() == "flamegraph"
		go collectOnTarget(target, scriptsToRunOnTarget, localTempDir, scriptParams["Duration"], ctrlCToStop, channelTargetScriptOutputs, channelError, statusUpdate)
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
				targetScriptOutputs.Tables = targetTables[targetIdx]
				orderedTargetScriptOutputs = append(orderedTargetScriptOutputs, targetScriptOutputs)
				break
			}
		}
	}
	return orderedTargetScriptOutputs, nil
}

// isTableForTarget checks if the given table is applicable for the specified target
func isTableForTarget(tbl table.TableDefinition, t target.Target, localTempDir string) bool {
	if len(tbl.Architectures) > 0 {
		architecture, err := t.GetArchitecture()
		if err != nil {
			slog.Error("failed to get architecture for target", slog.String("target", t.GetName()), slog.String("error", err.Error()))
			return false
		}
		if !slices.Contains(tbl.Architectures, architecture) {
			return false
		}
	}
	if len(tbl.Vendors) > 0 {
		vendor, err := GetTargetVendor(t)
		if err != nil {
			slog.Error("failed to get vendor for target", slog.String("target", t.GetName()), slog.String("error", err.Error()))
			return false
		}
		if !slices.Contains(tbl.Vendors, vendor) {
			return false
		}
	}
	if len(tbl.MicroArchitectures) > 0 {
		uarch, err := GetTargetMicroArchitecture(t, localTempDir, false)
		if err != nil {
			slog.Error("failed to get microarchitecture for target", slog.String("target", t.GetName()), slog.String("error", err.Error()))
		}
		shortUarch := strings.Split(uarch, "_")[0]     // handle EMR_XCC, etc.
		shortUarch = strings.Split(shortUarch, "-")[0] // handle GNR-D
		shortUarch = strings.Split(shortUarch, " ")[0] // handle Turin (Zen 5)
		if !slices.Contains(tbl.MicroArchitectures, uarch) && !slices.Contains(tbl.MicroArchitectures, shortUarch) {
			return false
		}
	}
	return true
}

// elevatedPrivilegesRequired returns true if any of the scripts needed for the tables require elevated privileges
func elevatedPrivilegesRequired(tables []table.TableDefinition) bool {
	for _, tbl := range tables {
		for _, scriptName := range tbl.ScriptNames {
			script := script.GetScriptByName(scriptName)
			if script.Superuser {
				return true
			}
		}
	}
	return false
}

// collectOnTarget runs the scripts on the target and sends the results to the appropriate channels
func collectOnTarget(myTarget target.Target, scriptsToRun []script.ScriptDefinition, localTempDir string, duration string, ctrlCToStop bool, channelTargetScriptOutputs chan TargetScriptOutputs, channelError chan error, statusUpdate progress.MultiSpinnerUpdateFunc) {
	// run the scripts on the target
	status := "collecting data"
	if ctrlCToStop && duration == "0" {
		status += ", press Ctrl+c to stop"
	} else if duration != "0" && duration != "" {
		status += fmt.Sprintf(" for %s seconds", duration)
	}
	scriptOutputs, err := RunScripts(myTarget, scriptsToRun, true, localTempDir, statusUpdate, status, false)
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
