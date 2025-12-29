// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package lock

import (
	htmltemplate "html/template"
	"perfspect/internal/report"
	"perfspect/internal/table"
)

func kernelLockAnalysisHTMLRenderer(tableValues table.TableValues, targetName string) string {
	values := [][]string{}
	var tableValueStyles [][]string
	for _, field := range tableValues.Fields {
		rowValues := []string{}
		rowValues = append(rowValues, field.Name)
		rowValues = append(rowValues, htmltemplate.HTMLEscapeString(field.Values[0]))
		values = append(values, rowValues)
		rowStyles := []string{}
		rowStyles = append(rowStyles, "font-weight:bold")
		rowStyles = append(rowStyles, "white-space: pre-wrap")
		tableValueStyles = append(tableValueStyles, rowStyles)
	}
	return report.RenderHTMLTable([]string{}, values, "pure-table pure-table-striped", tableValueStyles)
}
