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

	"github.com/spf13/cobra"
)

var AppName = filepath.Base(os.Args[0])

// AppContext represents the application context that can be accessed from all commands.
type AppContext struct {
	OutputDir string // OutputDir is the directory where the application will write output files.
	TempDir   string // TempDir is the local host's temp directory.
	Version   string // Version is the version of the application.
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

func CreateOutputDir(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	return nil
}

type SummaryFunc func([]report.TableValues, map[string]script.ScriptOutput) report.TableValues
type InsightsFunc SummaryFunc

type ReportingCommand struct {
	Cmd              *cobra.Command
	ReportNamePost   string
	TableNames       []string
	Duration         int
	Interval         int
	Frequency        int
	SummaryFunc      SummaryFunc
	SummaryTableName string
	InsightsFunc     InsightsFunc
}

func (rc *ReportingCommand) Run() error {
	// appContext is the application context that holds common data and resources.
	appContext := rc.Cmd.Context().Value(AppContext{}).(AppContext)
	localTempDir := appContext.TempDir
	outputDir := appContext.OutputDir
	// handle signals
	// child processes will exit when the signals are received which will
	// allow this app to exit normally
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChannel
		slog.Info("received signal", slog.String("signal", sig.String()))
	}()
	// get the data we need to generate reports
	var orderedTargetScriptOutputs []TargetScriptOutputs
	if FlagInput != "" {
		// read the raw file(s) as JSON
		rawReports, err := report.ReadRawReports(FlagInput)
		if err != nil {
			err = fmt.Errorf("failed to read raw file(s): %w", err)
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
			slog.Error(err.Error())
			rc.Cmd.SilenceUsage = true
			return err
		}
		rc.TableNames = []string{} // use the table names from the raw files
		for _, rawReport := range rawReports {
			for _, tableName := range rawReport.TableNames { // just in case someone tries to use the raw files that were collected with a different set of categories
				// filter out tables that we add after processing
				if tableName == TableNameInsights || tableName == TableNamePerfspect || tableName == rc.SummaryTableName {
					continue
				}
				rc.TableNames = util.UniqueAppend(rc.TableNames, tableName)
			}
			orderedTargetScriptOutputs = append(orderedTargetScriptOutputs, TargetScriptOutputs{targetName: rawReport.TargetName, scriptOutputs: rawReport.ScriptOutputs})
		}
	} else {
		// get the list of unique scripts to run and tables we're interested in
		var scriptNames []string
		for _, tableName := range rc.TableNames {
			// add scripts to list of scripts to run
			for _, scriptName := range report.GetScriptNamesForTable(tableName) {
				scriptNames = util.UniqueAppend(scriptNames, scriptName)
			}
		}
		// make a list of unique script definitions
		var scriptsToRun []script.ScriptDefinition
		for _, scriptName := range scriptNames {
			scriptsToRun = append(scriptsToRun, script.GetTimedScriptByName(scriptName, rc.Duration, rc.Interval, rc.Frequency))
		}
		// do any of the scripts require elevated privileges?
		elevated := false
		for _, script := range scriptsToRun {
			if script.Superuser {
				elevated = true
				break
			}
		}
		// get the targets
		myTargets, err := GetTargets(rc.Cmd, elevated, false, localTempDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			rc.Cmd.SilenceUsage = true
			return err
		}
		if len(myTargets) == 0 {
			err := fmt.Errorf("no targets specified")
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			rc.Cmd.SilenceUsage = true
			return err
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
		// run the scripts on the targets
		channelTargetScriptOutputs := make(chan TargetScriptOutputs)
		channelError := make(chan error)
		for _, target := range myTargets {
			go collectOnTarget(rc.Cmd, target, scriptsToRun, localTempDir, channelTargetScriptOutputs, channelError, multiSpinner.Status)
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
		for _, target := range myTargets {
			for _, targetScriptOutputs := range allTargetScriptOutputs {
				if targetScriptOutputs.targetName == target.GetName() {
					orderedTargetScriptOutputs = append(orderedTargetScriptOutputs, targetScriptOutputs)
					break
				}
			}
		}
		multiSpinner.Finish()
		fmt.Println()
	}
	err := CreateOutputDir(outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		rc.Cmd.SilenceUsage = true
		return err
	}
	// create the raw report before processing the data, so that we can save the raw data even if there is an error while processing
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
	// check report formats
	formats := FlagFormat
	if util.StringInList(report.FormatAll, formats) {
		formats = report.FormatOptions
	}
	// process the collected data and create the requested report(s)
	allTargetsTableValues := make([][]report.TableValues, 0)
	var reportFilePaths []string
	for _, targetScriptOutputs := range orderedTargetScriptOutputs {
		scriptOutputs := targetScriptOutputs.scriptOutputs
		// process the tables, i.e., get field values from script output
		allTableValues, err := report.Process(rc.TableNames, scriptOutputs)
		if err != nil {
			err = fmt.Errorf("failed to process collected data: %w", err)
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
			slog.Error(err.Error())
			rc.Cmd.SilenceUsage = true
			return err
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
			reportBytes, err := report.Create(format, allTableValues, scriptOutputs, targetScriptOutputs.targetName)
			if err != nil {
				err = fmt.Errorf("failed to create report: %w", err)
				fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
				slog.Error(err.Error())
				rc.Cmd.SilenceUsage = true
				return err
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
				fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
				slog.Error(err.Error())
				rc.Cmd.SilenceUsage = true
				return err
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
			if !util.StringInList(format, formats) {
				continue
			}
			reportBytes, err := report.CreateMultiTarget(format, allTargetsTableValues, targetNames)
			if err != nil {
				err = fmt.Errorf("failed to create multi-target %s report: %w", format, err)
				fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
				slog.Error(err.Error())
				rc.Cmd.SilenceUsage = true
				return err
			}
			reportFilename := fmt.Sprintf("%s.%s", "all_hosts", format)
			reportPath := filepath.Join(appContext.OutputDir, reportFilename)
			if err = report.WriteReport(reportBytes, reportPath); err != nil {
				err = fmt.Errorf("failed to write multi-target %s report: %w", format, err)
				fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
				slog.Error(err.Error())
				rc.Cmd.SilenceUsage = true
				return err
			}
			reportFilePaths = append(reportFilePaths, reportPath)
		}
	}
	if len(reportFilePaths) > 0 {
		fmt.Println("Report files:")
	}
	for _, reportFilePath := range reportFilePaths {
		fmt.Printf("  %s\n", reportFilePath)
	}
	return nil

}

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

func collectOnTarget(cmd *cobra.Command, myTarget target.Target, scriptsToRun []script.ScriptDefinition, localTempDir string, channelTargetScriptOutputs chan TargetScriptOutputs, channelError chan error, statusUpdate progress.MultiSpinnerUpdateFunc) {
	// create a temporary directory on the target
	var targetTempDir string
	var err error
	if statusUpdateErr := statusUpdate(myTarget.GetName(), "creating temporary directory"); statusUpdateErr != nil {
		slog.Error("failed to set status", slog.String("target", myTarget.GetName()), slog.String("error", statusUpdateErr.Error()))
	}
	targetTempRoot, _ := cmd.Flags().GetString(FlagTargetTempDirName)
	if targetTempDir, err = myTarget.CreateTempDirectory(targetTempRoot); err != nil {
		if statusUpdateErr := statusUpdate(myTarget.GetName(), fmt.Sprintf("error creating temporary directory: %v", err)); statusUpdateErr != nil {
			slog.Error("failed to set status", slog.String("target", myTarget.GetName()), slog.String("error", statusUpdateErr.Error()))
		}
		err = fmt.Errorf("error creating temporary directory on %s: %v", myTarget.GetName(), err)
		channelError <- err
		return
	}
	// don't remove the directory if we're debugging
	if cmd.Parent().PersistentFlags().Lookup("debug").Value.String() != "true" {
		defer func() {
			err := myTarget.RemoveDirectory(targetTempDir)
			if err != nil {
				slog.Error("error removing target temporary directory", slog.String("error", err.Error()))
			}
		}()
	}
	// run the scripts on the target
	if statusUpdateErr := statusUpdate(myTarget.GetName(), "collecting data"); statusUpdateErr != nil {
		slog.Error("failed to set status", slog.String("target", myTarget.GetName()), slog.String("error", statusUpdateErr.Error()))
	}
	scriptOutputs, err := script.RunScripts(myTarget, scriptsToRun, true, localTempDir)
	if err != nil {
		if statusUpdateErr := statusUpdate(myTarget.GetName(), fmt.Sprintf("error collecting data: %v", err)); statusUpdateErr != nil {
			slog.Error("failed to set status", slog.String("target", myTarget.GetName()), slog.String("error", statusUpdateErr.Error()))
		}
		err = fmt.Errorf("error running data collection scripts on %s: %v", myTarget.GetName(), err)
		channelError <- err
		return
	}
	if statusUpdateErr := statusUpdate(myTarget.GetName(), "collection complete"); statusUpdateErr != nil {
		slog.Error("failed to set status", slog.String("target", myTarget.GetName()), slog.String("error", statusUpdateErr.Error()))
	}
	channelTargetScriptOutputs <- TargetScriptOutputs{targetName: myTarget.GetName(), scriptOutputs: scriptOutputs}
}
