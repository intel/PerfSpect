package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"encoding/json"
	"fmt"
	"os"
	"perfspect/internal/script"
	"strings"
)

// RawReport represents a raw report containing the target name, table names, and script outputs.
type RawReport struct {
	TargetName    string                         // json:"target_name"
	TableNames    []string                       // json:"table_names"
	ScriptOutputs map[string]script.ScriptOutput // json:"script_outputs"
}

// CreateRawReport creates a raw report with the specified table names, script outputs, and target name.
// It marshals the report into a JSON format with indentation for readability.
// The function returns the JSON byte slice and any error encountered during the process.
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
