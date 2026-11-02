// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package instructions

import (
	htmltemplate "html/template"
	"strings"

	"perfspect/internal/report"
	"perfspect/internal/table"
)

// Tier cell styling. Pressure tiers are the one judgement the table makes, so they are the
// only thing colored; everything else stays neutral so the numbers remain readable.
const (
	tierHighStyle = "background-color:#f8d7da;font-weight:bold"
	tierMedStyle  = "background-color:#fff3cd"
	tierLowStyle  = ""
)

// Columns rendered in a monospace face, because they hold symbol names or assembly.
var monospaceColumns = map[string]bool{
	"Symbol": true,
}

// registerPressureHTMLRenderer renders the per-symbol register-pressure table.
//
// The default row renderer would work, but two things need custom handling: the hot
// stack instructions cell holds newline-separated assembly that must keep its line breaks,
// and the pressure tier is worth highlighting so HIGH rows are findable by eye.
func registerPressureHTMLRenderer(tableValues table.TableValues, targetName string) string {
	if len(tableValues.Fields) == 0 || len(tableValues.Fields[0].Values) == 0 {
		return `<p>` + htmltemplate.HTMLEscapeString(tableValues.NoDataFound) + `</p>`
	}

	headers := make([]string, 0, len(tableValues.Fields))
	descriptions := make([]string, 0, len(tableValues.Fields))
	for _, field := range tableValues.Fields {
		headers = append(headers, field.Name)
		descriptions = append(descriptions, field.Description)
	}

	var (
		values           [][]string
		tableValueStyles [][]string
	)
	for row := range tableValues.Fields[0].Values {
		rowValues := make([]string, 0, len(tableValues.Fields))
		rowStyles := make([]string, 0, len(tableValues.Fields))
		for _, field := range tableValues.Fields {
			value := field.Values[row]
			escaped := htmltemplate.HTMLEscapeString(value)

			style := ""
			switch {
			case field.Name == "PRES":
				style = tierStyle(value)
			case field.Name == "Hot Stack Instructions", field.Name == "DSOs":
				// Stored on one line for the text formats; expand to one instruction per
				// line here. Replacing after escaping is safe: the separator contains no
				// characters that HTML escaping alters.
				escaped = strings.ReplaceAll(escaped, topOpsSeparator, "\n")
				style = "font-family:monospace;white-space:pre-wrap"
			case monospaceColumns[field.Name]:
				style = "font-family:monospace"
			}
			rowValues = append(rowValues, escaped)
			rowStyles = append(rowStyles, style)
		}
		values = append(values, rowValues)
		tableValueStyles = append(tableValueStyles, rowStyles)
	}

	// Descriptions must be passed explicitly: RenderHTMLTable supplies none, which would
	// drop every column's hover text that this table takes care to define.
	return report.RenderHTMLTableWithDescriptions(headers, descriptions, values, "pure-table pure-table-striped", tableValueStyles)
}

func tierStyle(tier string) string {
	switch strings.TrimSpace(tier) {
	case "HIGH":
		return tierHighStyle
	case "MED":
		return tierMedStyle
	default:
		return tierLowStyle
	}
}

// collectionInfoHTMLRenderer renders the collection summary as name/value rows.
//
// It exists only to preserve line breaks. Several of these fields are multi-line - the
// list of unannotatable binaries, and the JIT diagnostics - and the default renderer
// escapes the value into a cell without pre-wrap, so HTML collapses the newlines into one
// unreadable run-on line.
func collectionInfoHTMLRenderer(tableValues table.TableValues, targetName string) string {
	var (
		values           [][]string
		tableValueStyles [][]string
	)
	for _, field := range tableValues.Fields {
		value := ""
		if len(field.Values) > 0 {
			value = field.Values[0]
		}
		values = append(values, []string{
			report.CreateFieldNameWithDescription(field.Name, field.Description),
			htmltemplate.HTMLEscapeString(value),
		})
		tableValueStyles = append(tableValueStyles, []string{
			"font-weight:bold;white-space:nowrap",
			"white-space:pre-wrap;font-family:monospace",
		})
	}
	return report.RenderHTMLTable([]string{}, values, "pure-table pure-table-striped", tableValueStyles)
}
