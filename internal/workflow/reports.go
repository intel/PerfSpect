// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package workflow

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"slices"

	"perfspect/internal/app"
	"perfspect/internal/report"
	"perfspect/internal/table"
	"perfspect/internal/util"
)

// createRawReports creates the raw report(s) from the collected data
// returns the list of report files creates or an error if the report creation failed.
func (rc *ReportingCommand) createRawReports(appContext app.Context, orderedTargetScriptOutputs []TargetScriptOutputs) ([]string, error) {
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
func (rc *ReportingCommand) createReports(appContext app.Context, orderedTargetScriptOutputs []TargetScriptOutputs, formats []string) ([]string, error) {
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
				Name: app.TableNamePerfspect,
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
			post := ""
			if rc.ReportNamePost != "" {
				post = "_" + rc.ReportNamePost
			}
			reportFilename := fmt.Sprintf("%s.%s", "all_hosts"+post, format)
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
