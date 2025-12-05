// Package report provides functions to generate reports in various formats such as txt, json, html, xlsx.
package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"perfspect/internal/table"
	"strings"
)

const (
	FormatHtml = "html"
	FormatXlsx = "xlsx"
	FormatJson = "json"
	FormatTxt  = "txt"
	FormatRaw  = "raw"
	FormatAll  = "all"
)

const NoDataFound = "No data found."

var FormatOptions = []string{FormatHtml, FormatXlsx, FormatJson, FormatTxt}

// Create generates a report in the specified format based on the provided tables, table values, and script outputs.
// The function ensures that all fields have the same number of values before generating the report.
// It supports formats such as txt, json, html, xlsx.
// If the format is not supported, the function panics with an error message.
//
// Parameters:
// - format: The desired format of the report (txt, json, html, xlsx, raw).
// - tableValues: The values for each field in each table.
// - targetName: The name of the target for which the report is being generated.
//
// Returns:
// - out: The generated report as a byte slice.
// - err: An error, if any occurred during report generation.
func Create(format string, allTableValues []table.TableValues, targetName string, systemSummaryTableName string) (out []byte, err error) {
	// make sure that all fields have the same number of values
	for _, tableValue := range allTableValues {
		numRows := -1
		for _, fieldValues := range tableValue.Fields {
			if numRows == -1 {
				numRows = len(fieldValues.Values)
				continue
			}
			if len(fieldValues.Values) != numRows {
				return nil, fmt.Errorf("expected %d value(s) for field, found %d", numRows, len(fieldValues.Values))
			}
		}
	}
	// create the report based on the specified format
	switch format {
	case FormatTxt:
		return createTextReport(allTableValues)
	case FormatJson:
		return createJsonReport(allTableValues)
	case FormatHtml:
		return createHtmlReport(allTableValues, targetName)
	case FormatXlsx:
		return createXlsxReport(allTableValues, systemSummaryTableName)
	}
	panic(fmt.Sprintf("expected one of %s, got %s", strings.Join(FormatOptions, ", "), format))
}

// CreateMultiTarget generates a report in the specified format for multiple targets.
// It supports "html" and "xlsx" formats. The function takes the following parameters:
//
// - format: A string specifying the desired report format ("html" or "xlsx").
// - allTargetsTableValues: A 2D slice of TableValues containing data for all targets.
// - targetNames: A slice of strings representing the names of the targets.
// - allTableNames: A slice of strings representing the names of the tables.
//
// Returns:
// - out: A byte slice containing the generated report.
// - err: An error if the report generation fails.
//
// Note: If an unsupported format is provided, the function will panic.
func CreateMultiTarget(format string, allTargetsTableValues [][]table.TableValues, targetNames []string, allTableNames []string, systemSummaryTableName string) (out []byte, err error) {
	switch format {
	case "html":
		return createHtmlReportMultiTarget(allTargetsTableValues, targetNames, allTableNames)
	case "xlsx":
		return createXlsxReportMultiTarget(allTargetsTableValues, targetNames, allTableNames, systemSummaryTableName)
	}
	panic("only HTML and XLSX multi-target report supported currently")
}
