// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"perfspect/internal/script"
	"perfspect/internal/table"
)

// EPPFromOutput gets EPP value from script outputs
func EPPFromOutput(outputs map[string]script.ScriptOutput) string {
	if outputs[script.EppValidScriptName].Exitcode != 0 || len(outputs[script.EppValidScriptName].Stdout) == 0 ||
		outputs[script.EppPackageControlScriptName].Exitcode != 0 || len(outputs[script.EppPackageControlScriptName].Stdout) == 0 ||
		outputs[script.EppPackageScriptName].Exitcode != 0 || len(outputs[script.EppPackageScriptName].Stdout) == 0 {
		slog.Warn("EPP scripts failed or produced no output")
		return ""
	}
	var eppValid string
	for i, line := range strings.Split(outputs[script.EppValidScriptName].Stdout, "\n") {
		if line == "" {
			continue
		}
		lineParts := strings.Split(line, ":")
		if len(lineParts) < 2 {
			continue
		}
		currentEpbValid := strings.TrimSpace(lineParts[1])
		if i == 0 {
			eppValid = currentEpbValid
			continue
		}
		if currentEpbValid != eppValid {
			slog.Warn("EPP valid bit is inconsistent across cores")
			return "inconsistent"
		}
	}
	var eppPkgCtrl string
	for i, line := range strings.Split(outputs[script.EppPackageControlScriptName].Stdout, "\n") {
		if line == "" {
			continue
		}
		lineParts := strings.Split(line, ":")
		if len(lineParts) < 2 {
			continue
		}
		currentEppPkgCtrl := strings.TrimSpace(lineParts[1])
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
		eppPackage := strings.TrimSpace(outputs[script.EppPackageScriptName].Stdout)
		msr, err := strconv.ParseInt(eppPackage, 16, 0)
		if err != nil {
			slog.Error("failed to parse EPP package value", slog.String("error", err.Error()), slog.String("epp", eppPackage))
			return ""
		}
		return eppValToLabel(int(msr))
	} else {
		var epp string
		for i, line := range strings.Split(outputs[script.EppScriptName].Stdout, "\n") {
			if line == "" {
				continue
			}
			lineParts := strings.Split(line, ":")
			if len(lineParts) < 2 {
				continue
			}
			currentEpp := strings.TrimSpace(lineParts[1])
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

// EPBFromOutput gets EPB value from script outputs
func EPBFromOutput(outputs map[string]script.ScriptOutput) string {
	if outputs[script.EpbScriptName].Exitcode != 0 || len(outputs[script.EpbScriptName].Stdout) == 0 {
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

// C6FromOutput returns the C6 C-state status.
func C6FromOutput(outputs map[string]script.ScriptOutput) string {
	cstatesInfo := CstatesFromOutput(outputs)
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

// CstatesSummaryFromOutput returns a summary of all C-state statuses.
func CstatesSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	cstatesInfo := CstatesFromOutput(outputs)
	if cstatesInfo == nil {
		return ""
	}
	summaryParts := []string{}
	for _, cstateInfo := range cstatesInfo {
		summaryParts = append(summaryParts, fmt.Sprintf("%s: %s", cstateInfo.Name, cstateInfo.Status))
	}
	return strings.Join(summaryParts, ", ")
}

// CstateInfo represents a C-state name and status.
type CstateInfo struct {
	Name   string
	Status string
}

// CstatesFromOutput extracts C-state information from script outputs.
func CstatesFromOutput(outputs map[string]script.ScriptOutput) []CstateInfo {
	var cstatesInfo []CstateInfo
	output := outputs[script.CstatesScriptName].Stdout
	for line := range strings.SplitSeq(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			return nil
		}
		cstatesInfo = append(cstatesInfo, CstateInfo{Name: parts[0], Status: parts[1]})
	}
	return cstatesInfo
}

// enum for the column indices in the ELC CSV output
const (
	elcFieldSocketID = iota
	elcFieldInstance
	elcFieldDie
	elcFieldDieType
	elcFieldMinRatio
	elcFieldMaxRatio
	elcFieldELCRatio
	elcFieldELCLowThreshold
	elcFieldELCHighThreshold
	elcFieldELCHighThresholdEnable
	elcFieldMode
)

const (
	ELCModeLatencyOptimized = "Latency Optimized Mode (LOM)"
	ELCModeOptimizedPower   = "Optimized Power Mode (OPM)"
	ELCModeCustom           = "Custom Mode"
)

// ELCFieldValuesFromOutput extracts Efficiency Latency Control field values.
func ELCFieldValuesFromOutput(outputs map[string]script.ScriptOutput) (fieldValues []table.Field) {
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
	// confirm rows have expected number of columns
	for _, row := range rows {
		if len(row) < elcFieldELCHighThresholdEnable+1 {
			slog.Warn("ELC script output has unexpected number of columns", slog.Int("expected", elcFieldELCHighThresholdEnable+1), slog.Int("actual", len(row)))
			return
		}
	}
	for fieldNamesIndex, fieldName := range rows[0] {
		values := []string{}
		for _, row := range rows[1:] {
			values = append(values, row[fieldNamesIndex])
		}
		fieldValues = append(fieldValues, table.Field{Name: fieldName, Values: values})
	}

	modeValues := []string{}
	for _, row := range rows[1:] {
		var mode string
		if row[elcFieldELCRatio] == "0" && row[elcFieldELCLowThreshold] == "0" && row[elcFieldELCHighThreshold] == "0" && row[elcFieldELCHighThresholdEnable] == "1" {
			mode = ELCModeLatencyOptimized
		} else if row[elcFieldELCLowThreshold] == "10" &&
			row[elcFieldELCHighThreshold] == "94" &&
			row[elcFieldELCHighThresholdEnable] == "1" &&
			((row[elcFieldDieType] == "IO" && row[elcFieldELCRatio] == "800") ||
				(row[elcFieldDieType] == "Compute" && row[elcFieldELCRatio] == "1200")) {
			mode = ELCModeOptimizedPower
		} else {
			mode = ELCModeCustom
		}
		modeValues = append(modeValues, mode)
	}
	fieldValues = append(fieldValues, table.Field{Name: "Mode", Values: modeValues})
	return
}

// ELCSummaryFromOutput returns a summary of Efficiency Latency Control settings.
func ELCSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	fieldValues := ELCFieldValuesFromOutput(outputs)
	if len(fieldValues) < elcFieldMode+1 || len(fieldValues[elcFieldMode].Values) == 0 {
		return ""
	}
	summary := fieldValues[elcFieldMode].Values[0]
	for _, value := range fieldValues[elcFieldMode].Values[1:] {
		if value != summary {
			return "mixed"
		}
	}
	return summary
}

// TDPFromOutput returns the TDP (Thermal Design Power) from script outputs.
func TDPFromOutput(outputs map[string]script.ScriptOutput) string {
	msrHex := strings.TrimSpace(outputs[script.PackagePowerLimitName].Stdout)
	msr, err := strconv.ParseInt(msrHex, 16, 0)
	if err != nil {
		slog.Warn("failed to parse TDP value", slog.String("error", err.Error()), slog.String("msrHex", msrHex))
		return ""
	}
	if msr == 0 {
		return "Unknown"
	}
	return fmt.Sprint(msr/8) + "W"
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
