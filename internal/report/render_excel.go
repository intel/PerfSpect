package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"

	"github.com/xuri/excelize/v2"
)

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
	_ = f.SetCellValue(sheetName, cellName(col, *row), tableValues.Name)
	_ = f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), tableNameStyle)
	*row++
	if len(tableValues.Fields) == 0 || len(tableValues.Fields[0].Values) == 0 {
		msg := noDataFound
		if tableValues.NoDataFound != "" {
			msg = tableValues.NoDataFound
		}
		_ = f.SetCellValue(sheetName, cellName(col, *row), msg)
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

func renderXlsxTableMultiTarget(targetTableValues []TableValues, targetNames []string, f *excelize.File, sheetName string, row *int) {
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

	_ = f.SetCellValue(sheetName, cellName(col, *row), targetTableValues[0].Name)
	_ = f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), tableNameStyle)

	if !targetTableValues[0].HasRows {
		col += 2
		// print the target names
		for _, targetName := range targetNames {
			_ = f.SetCellValue(sheetName, cellName(col, *row), targetName)
			_ = f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), targetNameStyle)
			col++
		}
		*row++

		// print the field names and values from each target
		for fieldIdx, field := range targetTableValues[0].Fields {
			col = 2
			_ = f.SetCellValue(sheetName, cellName(col, *row), field.Name)
			_ = f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), fieldNameStyle)
			col++
			for targetIdx := range targetNames {
				var fieldValue string
				if len(targetTableValues[targetIdx].Fields[fieldIdx].Values) > 0 {
					fieldValue = targetTableValues[targetIdx].Fields[fieldIdx].Values[0]
				}
				_ = f.SetCellValue(sheetName, cellName(col, *row), fieldValue)
				col++
			}
			*row++
		}
	} else {
		for targetIdx, targetName := range targetNames {
			// print the target name
			col = 2
			_ = f.SetCellValue(sheetName, cellName(col, *row), targetName)
			_ = f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), targetNameStyle)
			*row++

			// if no data found, print a message and skip to the next target
			if len(targetTableValues[targetIdx].Fields) == 0 || len(targetTableValues[targetIdx].Fields[0].Values) == 0 {
				msg := noDataFound
				if targetTableValues[targetIdx].NoDataFound != "" {
					msg = targetTableValues[targetIdx].NoDataFound
				}
				_ = f.SetCellValue(sheetName, cellName(col, *row), msg)
				*row += 2
				continue
			}

			// print the field names as column headings across the top of the table
			col = 2
			for _, field := range targetTableValues[targetIdx].Fields {
				_ = f.SetCellValue(sheetName, cellName(col, *row), field.Name)
				_ = f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), fieldNameStyle)
				col++
			}
			*row++
			// print the rows of values
			tableRows := len(targetTableValues[targetIdx].Fields[0].Values)
			for tableRow := range tableRows {
				col = 2
				for _, field := range targetTableValues[targetIdx].Fields {
					value := getValueForCell(field.Values[tableRow])
					_ = f.SetCellValue(sheetName, cellName(col, *row), value)
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
			_ = f.SetCellValue(sheetName, cellName(col, *row), field.Name)
			_ = f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), headerStyle)
			col++
		}
		col = 2
		*row++
		// print the rows
		tableRows := len(tableValues.Fields[0].Values)
		for tableRow := range tableRows {
			for _, field := range tableValues.Fields {
				value := getValueForCell(field.Values[tableRow])
				_ = f.SetCellValue(sheetName, cellName(col, *row), value)
				_ = f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), alignLeft)
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
			_ = f.SetCellValue(sheetName, cellName(col, *row), field.Name)
			col++
			value := getValueForCell(fieldValue)
			_ = f.SetCellValue(sheetName, cellName(col, *row), value)
			_ = f.SetCellStyle(sheetName, cellName(col, *row), cellName(col, *row), alignLeft)
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
	_ = f.SetSheetName("Sheet1", sheetName)
	_ = f.SetColWidth(sheetName, "A", "A", 25)
	_ = f.SetColWidth(sheetName, "B", "L", 25)
	row := 1
	for _, tableValues := range allTableValues {
		if tableValues.Name == SystemSummaryTableName {
			row := 1
			sheetName := XlsxBriefSheetName
			_, _ = f.NewSheet(sheetName)
			_ = f.SetColWidth(sheetName, "A", "L", 25)
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

func createXlsxReportMultiTarget(allTargetsTableValues [][]TableValues, targetNames []string, allTableNames []string) (out []byte, err error) {
	f := excelize.NewFile()
	sheetName := XlsxPrimarySheetName
	_ = f.SetSheetName("Sheet1", sheetName)
	_ = f.SetColWidth(sheetName, "A", "A", 15)
	_ = f.SetColWidth(sheetName, "B", "L", 25)
	row := 1

	// render the tables in the order they were passed in
	for _, tableName := range allTableNames {
		// build list of target names and TableValues for targets that have values for this table
		tableTargets := []string{}
		tableValues := []TableValues{}
		for targetIndex, targetTableValues := range allTargetsTableValues {
			tableIndex := findTableIndex(targetTableValues, tableName)
			if tableIndex == -1 {
				continue
			}
			tableTargets = append(tableTargets, targetNames[targetIndex])
			tableValues = append(tableValues, targetTableValues[tableIndex])
		}
		// render the table, if system summary table put it in a separate sheet
		if tableName == SystemSummaryTableName {
			row = 1
			sheetName := XlsxBriefSheetName
			_, _ = f.NewSheet(sheetName)
			_ = f.SetColWidth(sheetName, "A", "A", 15)
			_ = f.SetColWidth(sheetName, "B", "L", 25)
			renderXlsxTableMultiTarget(tableValues, tableTargets, f, sheetName, &row)
		} else {
			renderXlsxTableMultiTarget(tableValues, tableTargets, f, sheetName, &row)
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

func getValueForCell(value string) (val any) {
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
