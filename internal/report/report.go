// Package report provides functions to generate reports in various formats such as txt, json, html, xlsx.
package report

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"perfspect/internal/script"

	"github.com/xuri/excelize/v2"
)

const (
	FormatHtml = "html"
	FormatXlsx = "xlsx"
	FormatJson = "json"
	FormatTxt  = "txt"
	FormatRaw  = "raw"
	FormatAll  = "all"
)

const noDataFound = "No data found."

var FormatOptions = []string{FormatHtml, FormatXlsx, FormatJson, FormatTxt}

// Process processes the given tables and script outputs to generate table values.
// It collects values for each field in the tables and returns a slice of TableValues.
// If any error occurs during processing, it is returned along with the table values.

func Process(tableNames []string, scriptOutputs map[string]script.ScriptOutput) (allTableValues []TableValues, err error) {
	for _, tableName := range tableNames {
		allTableValues = append(allTableValues, GetValuesForTable(tableName, scriptOutputs))
	}
	return
}

// Create generates a report in the specified format based on the provided tables, table values, and script outputs.
// The function ensures that all fields have the same number of values before generating the report.
// It supports formats such as txt, json, html, xlsx.
// If the format is not supported, the function panics with an error message.
//
// Parameters:
// - format: The desired format of the report (txt, json, html, xlsx, raw).
// - tableValues: The values for each field in each table.
// - scriptOutputs: The outputs of any scripts used in the report.
// - targetName: The name of the target for which the report is being generated.
//
// Returns:
// - out: The generated report as a byte slice.
// - err: An error, if any occurred during report generation.
func Create(format string, allTableValues []TableValues, scriptOutputs map[string]script.ScriptOutput, targetName string) (out []byte, err error) {
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
		return createXlsxReport(allTableValues)
	}
	panic(fmt.Sprintf("expected one of %s, got %s", strings.Join(FormatOptions, ", "), format))
}

func CreateMultiTarget(format string, allTargetsTableValues [][]TableValues, targetNames []string) (out []byte, err error) {
	switch format {
	case "html":
		return createHtmlReportMultiTarget(allTargetsTableValues, targetNames)
	case "xlsx":
		return createXlsxReportMultiTarget(allTargetsTableValues, targetNames)
	}
	panic("only HTML and XLSX multi-target report supported currently")
}

func createTextReport(allTableValues []TableValues) (out []byte, err error) {
	var sb strings.Builder
	for _, tableValues := range allTableValues {
		sb.WriteString(fmt.Sprintf("%s\n", tableValues.Name))
		for i := 0; i < len(tableValues.Name); i++ {
			sb.WriteString("=")
		}
		sb.WriteString("\n")
		if len(tableValues.Fields) == 0 || len(tableValues.Fields[0].Values) == 0 {
			sb.WriteString(noDataFound + "\n\n")
			continue
		}
		// custom renderer defined?
		if tableValues.TextTableRendererFunc != nil {
			sb.WriteString(tableValues.TextTableRendererFunc(tableValues))
		} else {
			sb.WriteString(DefaultTextTableRendererFunc(tableValues))
		}
		sb.WriteString("\n")
	}
	out = []byte(sb.String())
	return
}

func DefaultTextTableRendererFunc(tableValues TableValues) string {
	var sb strings.Builder
	if tableValues.HasRows { // print the field names as column headings across the top of the table
		// find the longest item per column -- can be the field name (column header) or a value
		maxFieldLen := make(map[string]int)
		for i, field := range tableValues.Fields {
			// the last column shouldn't occupy more space than the value
			if i == len(tableValues.Fields)-1 {
				maxFieldLen[field.Name] = 0
				continue
			}
			// other columns should occupy the larger of the field name or the longest value
			maxFieldLen[field.Name] = len(field.Name)
			for _, val := range field.Values {
				if len(val) > maxFieldLen[field.Name] {
					maxFieldLen[field.Name] = len(val)
				}
			}
		}
		columnSpacing := 3
		// print the field names
		for _, field := range tableValues.Fields {
			sb.WriteString(fmt.Sprintf("%-*s", maxFieldLen[field.Name]+columnSpacing, field.Name))
		}
		sb.WriteString("\n")
		// underline the field names
		for _, field := range tableValues.Fields {
			underline := ""
			for i := 0; i < len(field.Name); i++ {
				underline += "-"
			}
			sb.WriteString(fmt.Sprintf("%-*s", maxFieldLen[field.Name]+columnSpacing, underline))
		}
		sb.WriteString("\n")
		// print the rows
		numRows := len(tableValues.Fields[0].Values)
		for row := 0; row < numRows; row++ {
			for fieldIdx, field := range tableValues.Fields {
				sb.WriteString(fmt.Sprintf("%-*s", maxFieldLen[field.Name]+columnSpacing, tableValues.Fields[fieldIdx].Values[row]))
			}
			sb.WriteString("\n")
		}
	} else {
		// get the longest field name to format the table nicely
		maxFieldNameLen := 0
		for _, field := range tableValues.Fields {
			if len(field.Name) > maxFieldNameLen {
				maxFieldNameLen = len(field.Name)
			}
		}
		// print the field names followed by their value
		for _, field := range tableValues.Fields {
			var value string
			if len(field.Values) > 0 {
				value = field.Values[0]
			}
			sb.WriteString(fmt.Sprintf("%s%-*s %s\n", field.Name, maxFieldNameLen-len(field.Name)+1, ":", value))
		}
	}
	return sb.String()
}

func createJsonReport(allTableValues []TableValues) (out []byte, err error) {
	type outRecord map[string]string
	type outTable []outRecord
	type outReport map[string]outTable
	oReport := make(outReport)
	for _, tableValues := range allTableValues {
		var oTable outTable
		if len(tableValues.Fields) == 0 {
			oReport[tableValues.Name] = oTable
			continue
		}
		numRecords := len(tableValues.Fields[0].Values)
		if numRecords > 0 {
			for recordIdx := 0; recordIdx < numRecords; recordIdx++ {
				oRecord := make(outRecord)
				for _, field := range tableValues.Fields {
					oRecord[field.Name] = field.Values[recordIdx]
				}
				oTable = append(oTable, oRecord)
			}
		} else {
			// insert an empty record
			oRecord := make(outRecord)
			for _, field := range tableValues.Fields {
				oRecord[field.Name] = ""
			}
			oTable = append(oTable, oRecord)
		}
		oReport[tableValues.Name] = oTable
	}
	return json.MarshalIndent(oReport, "", " ")
}

func cellName(col int, row int) (name string) {
	columnName, err := excelize.ColumnNumberToName(col)
	if err != nil {
		return
	}
	name, err = excelize.JoinCellName(columnName, row)
	if err != nil {
		return
	}
	return
}

func renderXlsxTable(tableValues TableValues, f *excelize.File, sheetName string, row *int) {
	col := 1
	// print the table name
	tableNameStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
	})
	f.SetCellValue(sheetName, cellName(col, *row), tableValues.Name)
	f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), tableNameStyle)
	*row++
	if len(tableValues.Fields) == 0 || len(tableValues.Fields[0].Values) == 0 {
		f.SetCellValue(sheetName, cellName(col, *row), noDataFound)
		*row += 2
		return
	}
	if tableValues.XlsxTableRendererFunc != nil {
		tableValues.XlsxTableRendererFunc(tableValues, f, sheetName, row)
	} else {
		DefaultXlsxTableRendererFunc(tableValues, f, sheetName, row)
	}
	*row++
}

func renderXlsxTableMultiTarget(tableIdx int, allTargetsTableValues [][]TableValues, targetNames []string, f *excelize.File, sheetName string, row *int) {
	col := 1
	// print the table name
	tableNameStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
	})
	targetNameStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
	})
	fieldNameStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
	})

	f.SetCellValue(sheetName, cellName(col, *row), allTargetsTableValues[0][tableIdx].Name)
	f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), tableNameStyle)

	if !allTargetsTableValues[0][tableIdx].HasRows {
		col += 2
		// print the target names
		for _, targetName := range targetNames {
			f.SetCellValue(sheetName, cellName(col, *row), targetName)
			f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), targetNameStyle)
			col++
		}
		*row++

		// print the field names and values from each target
		for fieldIdx, field := range allTargetsTableValues[0][tableIdx].Fields {
			col = 2
			f.SetCellValue(sheetName, cellName(col, *row), field.Name)
			f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), fieldNameStyle)
			col++
			for targetIdx := 0; targetIdx < len(targetNames); targetIdx++ {
				var fieldValue string
				if len(allTargetsTableValues[targetIdx][tableIdx].Fields[fieldIdx].Values) > 0 {
					fieldValue = allTargetsTableValues[targetIdx][tableIdx].Fields[fieldIdx].Values[0]
				}
				f.SetCellValue(sheetName, cellName(col, *row), fieldValue)
				col++
			}
			*row++
		}
	} else {
		for targetIdx, targetName := range targetNames {
			// print the target name
			col = 2
			f.SetCellValue(sheetName, cellName(col, *row), targetName)
			f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), targetNameStyle)
			*row++

			// if no data found, print a message and skip to the next target
			if len(allTargetsTableValues[targetIdx][tableIdx].Fields) == 0 || len(allTargetsTableValues[targetIdx][tableIdx].Fields[0].Values) == 0 {
				f.SetCellValue(sheetName, cellName(col, *row), noDataFound)
				*row += 2
				continue
			}

			// print the field names as column headings across the top of the table
			col = 2
			for _, field := range allTargetsTableValues[targetIdx][tableIdx].Fields {
				f.SetCellValue(sheetName, cellName(col, *row), field.Name)
				f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), fieldNameStyle)
				col++
			}
			*row++
			// print the rows of values
			tableRows := len(allTargetsTableValues[targetIdx][tableIdx].Fields[0].Values)
			for tableRow := 0; tableRow < tableRows; tableRow++ {
				col = 2
				for _, field := range allTargetsTableValues[targetIdx][tableIdx].Fields {
					value := getValueForCell(field.Values[tableRow])
					f.SetCellValue(sheetName, cellName(col, *row), value)
					col++
				}
				*row++
			}
			*row++
		}
	}
	*row++

}

func DefaultXlsxTableRendererFunc(tableValues TableValues, f *excelize.File, sheetName string, row *int) {
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
	})
	alignLeft, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "left",
		},
	})
	if tableValues.HasRows {
		// print the field names as column headings across the top of the table
		col := 2
		for _, field := range tableValues.Fields {
			f.SetCellValue(sheetName, cellName(col, *row), field.Name)
			f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), headerStyle)
			col++
		}
		col = 2
		*row++
		// print the rows
		tableRows := len(tableValues.Fields[0].Values)
		for tableRow := 0; tableRow < tableRows; tableRow++ {
			for _, field := range tableValues.Fields {
				value := getValueForCell(field.Values[tableRow])
				f.SetCellValue(sheetName, cellName(col, *row), value)
				f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), alignLeft)
				col++
			}
			col = 2
			*row++
		}
	} else {
		// print the field name followed by its value
		col := 1
		for _, field := range tableValues.Fields {
			var fieldValue string
			if len(tableValues.Fields[0].Values) > 0 {
				fieldValue = field.Values[0]
			}
			f.SetCellValue(sheetName, cellName(col, *row), field.Name)
			col++
			value := getValueForCell(fieldValue)
			f.SetCellValue(sheetName, cellName(col, *row), value)
			f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), alignLeft)
			col = 1
			*row++
		}
	}
}

const (
	XlsxPrimarySheetName = "Report"
	XlsxBriefSheetName   = "Brief"
)

func createXlsxReport(allTableValues []TableValues) (out []byte, err error) {
	f := excelize.NewFile()
	sheetName := XlsxPrimarySheetName
	f.SetSheetName("Sheet1", sheetName)
	f.SetColWidth(sheetName, "A", "A", 25)
	f.SetColWidth(sheetName, "B", "L", 25)
	row := 1
	for _, tableValues := range allTableValues {
		if tableValues.Name == SystemSummaryTableName {
			row := 1
			sheetName := XlsxBriefSheetName
			f.NewSheet(sheetName)
			f.SetColWidth(sheetName, "A", "L", 25)
			renderXlsxTable(tableValues, f, sheetName, &row)
		} else {
			renderXlsxTable(tableValues, f, sheetName, &row)
		}
	}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	_, err = f.WriteTo(w)
	if err != nil {
		err = fmt.Errorf("failed to write xlsx report to buffer: %v", err)
		return
	}
	out = buf.Bytes()
	return
}

func createXlsxReportMultiTarget(allTargetsTableValues [][]TableValues, targetNames []string) (out []byte, err error) {
	f := excelize.NewFile()
	sheetName := XlsxPrimarySheetName
	f.SetSheetName("Sheet1", sheetName)
	f.SetColWidth(sheetName, "A", "A", 15)
	f.SetColWidth(sheetName, "B", "L", 25)
	row := 1
	for tableIdx, tableValues := range allTargetsTableValues[0] {
		if tableValues.Name == SystemSummaryTableName {
			row := 1
			sheetName := XlsxBriefSheetName
			f.NewSheet(sheetName)
			f.SetColWidth(sheetName, "A", "A", 15)
			f.SetColWidth(sheetName, "B", "L", 25)
			renderXlsxTableMultiTarget(tableIdx, allTargetsTableValues, targetNames, f, sheetName, &row)
		} else {
			renderXlsxTableMultiTarget(tableIdx, allTargetsTableValues, targetNames, f, sheetName, &row)
		}
	}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	_, err = f.WriteTo(w)
	if err != nil {
		err = fmt.Errorf("failed to write multi-target xlsx report to buffer: %v", err)
		return
	}
	out = buf.Bytes()
	return
}

func getValueForCell(value string) (val interface{}) {
	intValue, err := strconv.Atoi(value)
	if err == nil {
		val = intValue
		return
	}
	floatValue, err := strconv.ParseFloat(value, 64)
	if err == nil {
		val = floatValue
		return
	}
	val = value
	return
}

// RawReport represents a raw report containing the target name, table names, and script outputs.
type RawReport struct {
	TargetName    string                         // json:"target_name"
	TableNames    []string                       // json:"table_names"
	ScriptOutputs map[string]script.ScriptOutput // json:"script_outputs"
}

func CreateRawReport(tableNames []string, scriptOutputs map[string]script.ScriptOutput, targetName string) (out []byte, err error) {
	report := RawReport{
		TargetName:    targetName,
		TableNames:    tableNames,
		ScriptOutputs: scriptOutputs,
	}
	out, err = json.MarshalIndent(report, "", " ")
	return
}

// ReadRawReports reads raw reports from the specified path.
// It reads all .raw files in the directory and returns a slice of RawReport.
// If the path is a file, it reads the single raw report and returns it.
func ReadRawReports(path string) (reports []RawReport, err error) {
	// path may be a directory or a file
	fileInfo, err := os.Stat(path)
	if err != nil {
		err = fmt.Errorf("failed to get file info: %v", err)
		return
	}
	allRawPaths := []string{}
	if fileInfo.IsDir() {
		var files []os.DirEntry
		files, err = os.ReadDir(path)
		if err != nil {
			err = fmt.Errorf("failed to read raw report directory: %v", err)
			return
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			if strings.HasSuffix(file.Name(), ".raw") {
				allRawPaths = append(allRawPaths, path+"/"+file.Name())
			}
		}
	} else {
		allRawPaths = append(allRawPaths, path)
	}
	for _, rawPath := range allRawPaths {
		var report RawReport
		report, err = readRawReport(rawPath)
		if err != nil {
			return
		}
		reports = append(reports, report)
	}
	return
}

func readRawReport(rawReportPath string) (report RawReport, err error) {
	reportBytes, err := os.ReadFile(rawReportPath)
	if err != nil {
		err = fmt.Errorf("failed to read raw report file: %v", err)
		return
	}
	err = json.Unmarshal(reportBytes, &report)
	return
}

// WriteReport writes the report bytes to the specified path.
func WriteReport(reportBytes []byte, reportPath string) error {
	err := os.WriteFile(reportPath, reportBytes, 0644)
	if err != nil {
		err = fmt.Errorf("failed to write report file: %v", err)
		fmt.Fprintln(os.Stderr, err)
		slog.Error(err.Error())
		return err
	}
	return nil
}
