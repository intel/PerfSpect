package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"perfspect/internal/table"
	"strings"
)

// Package-level map for custom text renderers
var customTextRenderers = map[string]table.TextTableRenderer{}

// getCustomTextRenderer returns the custom text renderer for a table, or nil if no custom renderer exists
func getCustomTextRenderer(tableName string) table.TextTableRenderer {
	return customTextRenderers[tableName]
}

// RegisterTextRenderer allows external packages to register custom text renderers for specific tables
func RegisterTextRenderer(tableName string, renderer table.TextTableRenderer) {
	customTextRenderers[tableName] = renderer
}

func createTextReport(allTableValues []table.TableValues) (out []byte, err error) {
	var sb strings.Builder
	for _, tableValues := range allTableValues {
		sb.WriteString(fmt.Sprintf("%s\n", tableValues.Name))
		for range len(tableValues.Name) {
			sb.WriteString("=")
		}
		sb.WriteString("\n")
		if len(tableValues.Fields) == 0 || len(tableValues.Fields[0].Values) == 0 {
			msg := NoDataFound
			if tableValues.NoDataFound != "" {
				msg = tableValues.NoDataFound
			}
			sb.WriteString(msg + "\n\n")
			continue
		}
		// custom renderer defined?
		if renderer := getCustomTextRenderer(tableValues.Name); renderer != nil {
			sb.WriteString(renderer(tableValues))
		} else {
			sb.WriteString(DefaultTextTableRendererFunc(tableValues))
		}
		sb.WriteString("\n")
	}
	out = []byte(sb.String())
	return
}

func DefaultTextTableRendererFunc(tableValues table.TableValues) string {
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
