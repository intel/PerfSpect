package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"strings"
)

func createTextReport(allTableValues []TableValues) (out []byte, err error) {
	var sb strings.Builder
	for _, tableValues := range allTableValues {
		sb.WriteString(fmt.Sprintf("%s\n", tableValues.Name))
		for range len(tableValues.Name) {
			sb.WriteString("=")
		}
		sb.WriteString("\n")
		if len(tableValues.Fields) == 0 || len(tableValues.Fields[0].Values) == 0 {
			msg := noDataFound
			if tableValues.NoDataFound != "" {
				msg = tableValues.NoDataFound
			}
			sb.WriteString(msg + "\n\n")
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
			for range len(field.Name) {
				underline += "-"
			}
			sb.WriteString(fmt.Sprintf("%-*s", maxFieldLen[field.Name]+columnSpacing, underline))
		}
		sb.WriteString("\n")
		// print the rows
		numRows := len(tableValues.Fields[0].Values)
		for row := range numRows {
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

// configurationTableTextRenderer renders the configuration table for text reports.
// It's similar to the default text table renderer, but uses the Description field
// to show the command line argument for each config item.
// Example output:
// Configuration
// =============
// Cores per Socket:               86          --cores <N>
// L3 Cache:                       336M        --llc <MB>
// Package Power / TDP:            350W        --tdp <Watts>
// All-Core Max Frequency:         3.2GHz      --core-max <GHz>
func configurationTableTextRenderer(tableValues TableValues) string {
	var sb strings.Builder

	// Find the longest field name and value for formatting
	maxFieldNameLen := 0
	maxValueLen := 0
	for _, field := range tableValues.Fields {
		if len(field.Name) > maxFieldNameLen {
			maxFieldNameLen = len(field.Name)
		}
		if len(field.Values) > 0 && len(field.Values[0]) > maxValueLen {
			maxValueLen = len(field.Values[0])
		}
	}

	// Print each field with name, value, and description (command-line arg)
	for _, field := range tableValues.Fields {
		var value string
		if len(field.Values) > 0 {
			value = field.Values[0]
		}
		// Format: "Field Name:      Value       Description"
		sb.WriteString(fmt.Sprintf("%-*s  %-*s  %s\n",
			maxFieldNameLen+1, field.Name+":",
			maxValueLen, value,
			field.Description))
	}

	return sb.String()
}
