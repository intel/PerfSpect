// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"
)

// parseTurbostatOutput parses turbostat output text into a slice of maps.
func parseTurbostatOutput(output string) ([]map[string]string, error) {
	var (
		headers    []string
		rows       []map[string]string
		interval   float64
		timestamp  time.Time
		timeParsed bool
		rowCount   int
	)
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if val, found := strings.CutPrefix(line, "INTERVAL:"); found {
			val = strings.TrimSpace(val)
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return nil, err
			}
			interval = f
			continue
		}
		if val, found := strings.CutPrefix(line, "TIME:"); found {
			val = strings.TrimSpace(val)
			var err error
			timestamp, err = time.Parse("15:04:05", val)
			if err != nil {
				slog.Error("unable to parse time", slog.String("value", val), slog.String("error", err.Error()))
				return nil, fmt.Errorf("unable to parse time: %s", val)
			}
			timeParsed = true
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 1 && slices.Contains([]string{"package", "die", "node", "core", "cpu"}, strings.ToLower(fields[0])) {
			if len(headers) == 0 {
				headers = fields
			} else {
				if timeParsed && interval > 0 {
					timestamp = timestamp.Add(time.Duration(interval) * time.Second)
				}
			}
			continue
		}
		if len(headers) == 0 {
			continue
		}
		if len(fields) != len(headers) {
			continue
		}
		row := make(map[string]string, len(headers))
		for i, h := range headers {
			row[h] = fields[i]
		}
		if timeParsed && interval > 0 {
			row["timestamp"] = timestamp.Format("15:04:05")
		}
		rows = append(rows, row)
		rowCount++
	}
	return rows, nil
}

// TurbostatPlatformRows parses the output of the turbostat script and returns the rows
// for the platform (summary) only.
func TurbostatPlatformRows(turboStatScriptOutput string, fieldNames []string) ([][]string, error) {
	if turboStatScriptOutput == "" {
		return nil, fmt.Errorf("turbostat output is empty")
	}
	if len(fieldNames) == 0 {
		return nil, fmt.Errorf("no field names provided")
	}
	rows, err := parseTurbostatOutput(turboStatScriptOutput)
	if err != nil {
		return nil, fmt.Errorf("unable to parse turbostat output: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no platform rows found in turbostat output")
	}
	var fieldValues [][]string
	for _, row := range rows {
		if !isPlatformRow(row) {
			continue
		}
		rowValues := make([]string, len(fieldNames)+1)
		rowValues[0] = row["timestamp"]
		for i, fieldName := range fieldNames {
			if value, ok := row[fieldName]; ok {
				rowValues[i+1] = value
			} else {
				return nil, fmt.Errorf("field %s not found in turbostat output", fieldName)
			}
		}
		fieldValues = append(fieldValues, rowValues)
	}
	if len(fieldValues) == 0 {
		err := fmt.Errorf("no data found in turbostat output for fields: %s", fieldNames)
		return nil, err
	}
	return fieldValues, nil
}

func isPlatformRow(row map[string]string) bool {
	for _, header := range []string{"Package", "Die", "Node", "Core", "CPU"} {
		if val, ok := row[header]; ok && val != "-" {
			return false
		}
	}
	return true
}

// TurbostatPackageRows parses the output of the turbostat script and returns the rows
// for each package.
func TurbostatPackageRows(turboStatScriptOutput string, fieldNames []string) ([][][]string, error) {
	if turboStatScriptOutput == "" {
		return nil, fmt.Errorf("turbostat output is empty")
	}
	if len(fieldNames) == 0 {
		return nil, fmt.Errorf("no field names provided")
	}
	rows, err := parseTurbostatOutput(turboStatScriptOutput)
	if err != nil {
		return nil, fmt.Errorf("unable to parse turbostat output: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no package rows found in turbostat output")
	}
	var packageRows [][][]string
	for _, row := range rows {
		if _, ok := row["Package"]; !ok {
			if row["CPU"] == "0" {
				row["Package"] = "0"
			} else {
				continue
			}
		}
		if !isPackageRow(row) {
			continue
		}
		rowValues := make([]string, len(fieldNames)+1)
		rowValues[0] = row["timestamp"]
		for i, fieldName := range fieldNames {
			if value, ok := row[fieldName]; ok {
				rowValues[i+1] = value
			} else {
				return nil, fmt.Errorf("field %s not found in turbostat output", fieldName)
			}
		}
		packageNum, err := strconv.Atoi(row["Package"])
		if err != nil {
			return nil, fmt.Errorf("unable to parse package number: %s", row["Package"])
		}
		if len(packageRows) < packageNum+1 {
			packageRows = append(packageRows, [][]string{rowValues})
		} else {
			packageRows[packageNum] = append(packageRows[packageNum], rowValues)
		}
	}
	if len(packageRows) == 0 {
		return nil, fmt.Errorf("no data found in turbostat output for fields: %s", fieldNames)
	}
	return packageRows, nil
}

func isPackageRow(row map[string]string) bool {
	if val, ok := row["Package"]; ok && val != "-" {
		return true
	}
	return false
}

// MaxTotalPackagePowerFromOutput calculates the maximum total package power from the turbostat output.
func MaxTotalPackagePowerFromOutput(turbostatOutput string) string {
	rows, err := parseTurbostatOutput(turbostatOutput)
	if err != nil {
		slog.Error("unable to parse turbostat output", slog.String("error", err.Error()))
		return ""
	}
	if len(rows) == 0 {
		return ""
	}
	var maxPower float64
	var ignoredFirstReading bool
	for _, row := range rows {
		if row["CPU"] != "-" && row["CPU"] != "" ||
			row["Package"] != "-" && row["Package"] != "" ||
			row["Core"] != "-" && row["Core"] != "" {
			continue
		}
		if wattStr, ok := row["PkgWatt"]; ok {
			if !ignoredFirstReading {
				ignoredFirstReading = true
				continue
			}
			watt, err := strconv.ParseFloat(strings.TrimSpace(wattStr), 64)
			if err != nil {
				slog.Warn("unable to parse power value", slog.String("value", wattStr), slog.String("error", err.Error()))
				continue
			}
			if watt > 10000 {
				slog.Warn("ignoring anomalous high power reading", slog.String("value", wattStr))
				continue
			}
			if watt > maxPower {
				maxPower = watt
			}
		}
	}
	if maxPower == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f Watts", maxPower)
}

// MinTotalPackagePowerFromOutput calculates the minimum total package power from the turbostat output.
func MinTotalPackagePowerFromOutput(turbostatOutput string) string {
	rows, err := parseTurbostatOutput(turbostatOutput)
	if err != nil {
		slog.Error("unable to parse turbostat output", slog.String("error", err.Error()))
		return ""
	}
	if len(rows) == 0 {
		return ""
	}
	var minPower float64
	for _, row := range rows {
		if row["CPU"] != "-" && row["CPU"] != "" ||
			row["Package"] != "-" && row["Package"] != "" ||
			row["Core"] != "-" && row["Core"] != "" {
			continue
		}
		if wattStr, ok := row["PkgWatt"]; ok {
			watt, err := strconv.ParseFloat(strings.TrimSpace(wattStr), 64)
			if err != nil {
				slog.Warn("unable to parse power value", slog.String("value", wattStr), slog.String("error", err.Error()))
				continue
			}
			if minPower == 0 || watt < minPower {
				minPower = watt
			}
		}
	}
	if minPower == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f Watts", minPower)
}

// MaxPackageTemperatureFromOutput calculates the maximum package temperature from the turbostat output.
func MaxPackageTemperatureFromOutput(turbostatOutput string) string {
	rows, err := parseTurbostatOutput(turbostatOutput)
	if err != nil {
		slog.Error("unable to parse turbostat output", slog.String("error", err.Error()))
		return ""
	}
	if len(rows) == 0 {
		return ""
	}
	var maxTemp float64
	var ignoredFirstReading bool
	for _, row := range rows {
		if row["CPU"] != "-" && row["CPU"] != "" ||
			row["Package"] != "-" && row["Package"] != "" ||
			row["Core"] != "-" && row["Core"] != "" {
			continue
		}
		if tempStr, ok := row["PkgTmp"]; ok {
			if !ignoredFirstReading {
				ignoredFirstReading = true
				continue
			}
			temp, err := strconv.ParseFloat(strings.TrimSpace(tempStr), 64)
			if err != nil {
				slog.Warn("unable to parse temperature value", slog.String("value", tempStr), slog.String("error", err.Error()))
				continue
			}
			if temp > 200 {
				slog.Warn("ignoring anomalous high temperature reading", slog.String("value", tempStr))
				continue
			}
			if temp > maxTemp {
				maxTemp = temp
			}
		}
	}
	if maxTemp == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f C", maxTemp)
}
