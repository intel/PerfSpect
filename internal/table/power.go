// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package table

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"perfspect/internal/script"
)

func elcFieldValuesFromOutput(outputs map[string]script.ScriptOutput) (fieldValues []Field) {
	if outputs[script.ElcScriptName].Stdout == "" {
		return
	}
	r := csv.NewReader(strings.NewReader(outputs[script.ElcScriptName].Stdout))
	rows, err := r.ReadAll()
	if err != nil {
		return
	}
	if len(rows) < 2 {
		return
	}
	// first row is headers
	for fieldNamesIndex, fieldName := range rows[0] {
		values := []string{}
		// value rows
		for _, row := range rows[1:] {
			values = append(values, row[fieldNamesIndex])
		}
		fieldValues = append(fieldValues, Field{Name: fieldName, Values: values})
	}

	// let's add an interpretation of the values in an additional column
	values := []string{}
	// value rows
	for _, row := range rows[1:] {
		var mode string
		if row[2] == "IO" {
			if row[5] == "0" && row[6] == "0" && row[7] == "0" {
				mode = "Latency Optimized"
			} else if row[5] == "800" && row[6] == "10" && row[7] == "94" {
				mode = "Default"
			} else {
				mode = "Custom"
			}
		} else { // COMPUTE
			switch row[5] {
			case "0":
				mode = "Latency Optimized"
			case "1200":
				mode = "Default"
			default:
				mode = "Custom"
			}
		}
		values = append(values, mode)
	}
	fieldValues = append(fieldValues, Field{Name: "Mode", Values: values})
	return
}

func elcSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	fieldValues := elcFieldValuesFromOutput(outputs)
	if len(fieldValues) == 0 {
		return ""
	}
	if len(fieldValues) < 10 {
		return ""
	}
	if len(fieldValues[9].Values) == 0 {
		return ""
	}
	summary := fieldValues[9].Values[0]
	for _, value := range fieldValues[9].Values[1:] {
		if value != summary {
			return "mixed"
		}
	}
	return summary
}

// epbFromOutput gets EPB value from script outputs
func epbFromOutput(outputs map[string]script.ScriptOutput) string {
	if outputs[script.EpbScriptName].Exitcode != 0 || len(outputs[script.EpbScriptName].Stdout) == 0 {
		slog.Warn("EPB scripts failed or produced no output")
		return ""
	}
	epb := strings.TrimSpace(outputs[script.EpbScriptName].Stdout)
	msr, err := strconv.ParseInt(epb, 16, 0)
	if err != nil {
		slog.Error("failed to parse EPB value", slog.String("error", err.Error()), slog.String("epb", epb))
		return ""
	}
	return epbValToLabel(int(msr))
}

func epbValToLabel(msr int) string {
	var val string
	if msr >= 0 && msr <= 3 {
		val = "Performance"
	} else if msr >= 4 && msr <= 7 {
		val = "Balanced Performance"
	} else if msr >= 8 && msr <= 11 {
		val = "Balanced Energy"
	} else if msr >= 12 {
		val = "Energy Efficient"
	}
	return fmt.Sprintf("%s (%d)", val, msr)
}

func eppValToLabel(msr int) string {
	var val string
	if msr == 128 {
		val = "Normal"
	} else if msr < 128 && msr > 64 {
		val = "Balanced Performance"
	} else if msr <= 64 {
		val = "Performance"
	} else if msr > 128 && msr < 192 {
		val = "Balanced Powersave"
	} else {
		val = "Powersave"
	}
	return fmt.Sprintf("%s (%d)", val, msr)
}

// eppFromOutput gets EPP value from script outputs
// IF 0x774[42] is '1' AND 0x774[60] is '0'
// THEN
//
//	get EPP from 0x772 (package)
//
// ELSE
//
//	get EPP from 0x774 (per core)
func eppFromOutput(outputs map[string]script.ScriptOutput) string {
	// if we couldn't get the EPP values, return empty string
	if outputs[script.EppValidScriptName].Exitcode != 0 || len(outputs[script.EppValidScriptName].Stdout) == 0 ||
		outputs[script.EppPackageControlScriptName].Exitcode != 0 || len(outputs[script.EppPackageControlScriptName].Stdout) == 0 ||
		outputs[script.EppPackageScriptName].Exitcode != 0 || len(outputs[script.EppPackageScriptName].Stdout) == 0 {
		slog.Warn("EPP scripts failed or produced no output")
		return ""
	}
	// check if the epp valid bit is set and consistent across all cores
	var eppValid string
	for i, line := range strings.Split(outputs[script.EppValidScriptName].Stdout, "\n") { // MSR 0x774, bit 60
		if line == "" {
			continue
		}
		currentEpbValid := strings.TrimSpace(strings.Split(line, ":")[1])
		if i == 0 {
			eppValid = currentEpbValid
			continue
		}
		if currentEpbValid != eppValid {
			slog.Warn("EPP valid bit is inconsistent across cores")
			return "inconsistent"
		}
	}
	// check if epp package control bit is set and consistent across all cores
	var eppPkgCtrl string
	for i, line := range strings.Split(outputs[script.EppPackageControlScriptName].Stdout, "\n") { // MSR 0x774, bit 42
		if line == "" {
			continue
		}
		currentEppPkgCtrl := strings.TrimSpace(strings.Split(line, ":")[1])
		if i == 0 {
			eppPkgCtrl = currentEppPkgCtrl
			continue
		}
		if currentEppPkgCtrl != eppPkgCtrl {
			slog.Warn("EPP package control bit is inconsistent across cores")
			return "inconsistent"
		}
	}
	if eppPkgCtrl == "1" && eppValid == "0" {
		eppPackage := strings.TrimSpace(outputs[script.EppPackageScriptName].Stdout) // MSR 0x772, bits 24-31  (package)
		msr, err := strconv.ParseInt(eppPackage, 16, 0)
		if err != nil {
			slog.Error("failed to parse EPP package value", slog.String("error", err.Error()), slog.String("epp", eppPackage))
			return ""
		}
		return eppValToLabel(int(msr))
	} else {
		var epp string
		for i, line := range strings.Split(outputs[script.EppScriptName].Stdout, "\n") { // MSR 0x774, bits 24-31 (per-core)
			if line == "" {
				continue
			}
			currentEpp := strings.TrimSpace(strings.Split(line, ":")[1])
			if i == 0 {
				epp = currentEpp
				continue
			}
			if currentEpp != epp {
				slog.Warn("EPP is inconsistent across cores")
				return "inconsistent"
			}
		}
		msr, err := strconv.ParseInt(epp, 16, 0)
		if err != nil {
			slog.Error("failed to parse EPP value", slog.String("error", err.Error()), slog.String("epp", epp))
			return ""
		}
		return eppValToLabel(int(msr))
	}
}

type cstateInfo struct {
	Name   string
	Status string
}

func c6FromOutput(outputs map[string]script.ScriptOutput) string {
	cstatesInfo := cstatesFromOutput(outputs)
	if cstatesInfo == nil {
		return ""
	}
	for _, cstateInfo := range cstatesInfo {
		if cstateInfo.Name == "C6" {
			return cstateInfo.Status
		}
	}
	return ""
}

func cstatesFromOutput(outputs map[string]script.ScriptOutput) []cstateInfo {
	var cstatesInfo []cstateInfo
	output := outputs[script.CstatesScriptName].Stdout
	for line := range strings.SplitSeq(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			return nil
		}
		cstatesInfo = append(cstatesInfo, cstateInfo{Name: parts[0], Status: parts[1]})
	}
	return cstatesInfo
}

func cstatesSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	cstatesInfo := cstatesFromOutput(outputs)
	if cstatesInfo == nil {
		return ""
	}
	summaryParts := []string{}
	for _, cstateInfo := range cstatesInfo {
		summaryParts = append(summaryParts, fmt.Sprintf("%s: %s", cstateInfo.Name, cstateInfo.Status))
	}
	return strings.Join(summaryParts, ", ")
}
