package lock

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	htmltemplate "html/template"
	"perfspect/internal/common"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/table"
	"strings"
)

// lock table names
const (
	KernelLockAnalysisTableName = "Kernel Lock Analysis"
)

// kernel lock analysis tables
var tableDefinitions = map[string]table.TableDefinition{
	KernelLockAnalysisTableName: {
		Name:      KernelLockAnalysisTableName,
		MenuLabel: KernelLockAnalysisTableName,
		ScriptNames: []string{
			script.ProfileKernelLockScriptName,
		},
		FieldsFunc: kernelLockAnalysisTableValues},
}

func kernelLockAnalysisTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Hotspot without Callstack", Values: []string{common.SectionValueFromOutput(outputs[script.ProfileKernelLockScriptName].Stdout, "perf_hotspot_no_children")}},
		{Name: "Hotspot with Callstack", Values: []string{common.SectionValueFromOutput(outputs[script.ProfileKernelLockScriptName].Stdout, "perf_hotspot_callgraph")}},
		{Name: "Cache2Cache without Callstack", Values: []string{common.SectionValueFromOutput(outputs[script.ProfileKernelLockScriptName].Stdout, "perf_c2c_no_children")}},
		{Name: "Cache2Cache with CallStack", Values: []string{common.SectionValueFromOutput(outputs[script.ProfileKernelLockScriptName].Stdout, "perf_c2c_callgraph")}},
		{Name: "Lock Contention", Values: []string{common.SectionValueFromOutput(outputs[script.ProfileKernelLockScriptName].Stdout, "perf_lock_contention")}},
		{Name: "Perf Package Path", Values: []string{strings.TrimSpace(common.SectionValueFromOutput(outputs[script.ProfileKernelLockScriptName].Stdout, "perf_package_path"))}},
	}
	return fields
}

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
