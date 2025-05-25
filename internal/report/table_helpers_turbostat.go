package report

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// parseTurbostatOutput parses turbostat output text into a slice of maps.
// Each map represents a row, with column names as keys and values as strings.
// Adds a "timestamp" key to each row, if TIME and INTERVAL are included in
// the output by the collection script.
func parseTurbostatOutput(output string) ([]map[string]string, error) {
	var (
		headers    []string
		rows       []map[string]string
		interval   float64
		startTime  time.Time
		timeParsed bool
		rowCount   int
	)
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "INTERVAL:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "INTERVAL:"))
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return nil, err
			}
			interval = f
			continue
		}
		if strings.HasPrefix(line, "TIME:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "TIME:"))
			// Try to parse as HH:MM:SS
			t, err := time.Parse("15:04:05", val)
			if err == nil {
				startTime = t
				timeParsed = true
			} else {
				// fallback: try as seconds since epoch
				sec, err := strconv.ParseFloat(val, 64)
				if err == nil {
					startTime = time.Unix(int64(sec), 0)
					timeParsed = true
				}
			}
			continue
		}
		// Only set headers once, even if repeated
		fields := strings.Fields(line)
		if len(fields) > 1 && (fields[0] == "CPU" || fields[0] == "Package" || fields[0] == "Core") {
			if len(headers) == 0 {
				headers = fields
			}
			continue
		}
		if len(headers) == 0 {
			continue // skip data lines before first header
		}
		values := strings.Fields(line)
		if len(values) != len(headers) {
			continue // skip malformed lines
		}
		row := make(map[string]string)
		for i, h := range headers {
			row[h] = values[i]
		}
		// Add timestamp
		if timeParsed && interval > 0 {
			ts := startTime.Add(time.Duration(float64(rowCount)*interval) * time.Second)
			row["timestamp"] = ts.Format("15:04:05")
		}
		rows = append(rows, row)
		rowCount++
	}
	return rows, nil
}

// turbostatSummaryRows parses the output of the turbostat script and returns a slice of rows with the specified field names.
// The first column is the sample time, and the rest are the values for the specified fields.
func turbostatSummaryRows(turboStatScriptOutput string, fieldNames []string) ([][]string, error) {
	if len(fieldNames) == 0 {
		err := fmt.Errorf("no field names provided")
		slog.Error(err.Error())
		return nil, err
	}
	rows, err := parseTurbostatOutput(turboStatScriptOutput)
	if err != nil {
		slog.Error("unable to parse turbostat output", slog.String("output", turboStatScriptOutput), slog.Any("fieldNames", fieldNames), slog.String("error", err.Error()))
		return nil, err
	}
	if len(rows) == 0 {
		err := fmt.Errorf("turbostat output is empty")
		slog.Error("turbostat output is empty", slog.String("output", turboStatScriptOutput), slog.Any("fieldNames", fieldNames), slog.String("error", err.Error()))
		return nil, err
	}
	// filter the rows to the summary rows only
	var fieldValues [][]string
	for _, row := range rows {
		if (row["Package"] != "-" && row["Package"] != "") ||
			(row["Core"] != "-" && row["Core"] != "") ||
			(row["CPU"] != "-" && row["CPU"] != "") {
			continue
		}
		// this is a summary row, extract the values for the specified fields
		rowValues := make([]string, len(fieldNames)+1) // +1 for the sample time
		rowValues[0] = row["timestamp"]                // first column is the sample time
		for i, fieldName := range fieldNames {
			if value, ok := row[fieldName]; ok {
				rowValues[i+1] = value // +1 for the sample time
			} else {
				slog.Error("field not found in turbostat output", slog.String("fieldName", fieldName))
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

// maxTotalPackagePowerFromOutput calculates the maximum total package power from the turbostat output.
func maxTotalPackagePowerFromOutput(turbostatOutput string) string {
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
		// only consider rows where CPU, Package, or Core is "-", these rows contain the sum of all packgaes
		if row["CPU"] != "-" && row["CPU"] != "" ||
			row["Package"] != "-" && row["Package"] != "" ||
			row["Core"] != "-" && row["Core"] != "" {
			continue
		}
		if wattStr, ok := row["PkgWatt"]; ok {
			if !ignoredFirstReading {
				// skip the first reading, it is usually not representative of the system state
				ignoredFirstReading = true
				continue
			}
			watt, err := strconv.ParseFloat(strings.TrimSpace(wattStr), 64)
			if err != nil {
				slog.Warn("unable to parse power value", slog.String("value", wattStr), slog.String("error", err.Error()))
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

// minTotalPackagePowerFromOutput calculates the minimum total package power from the turbostat output.
func minTotalPackagePowerFromOutput(turbostatOutput string) string {
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
		// only consider rows where CPU, Package, or Core is "-", these rows contain the sum of all packgaes
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

// maxPackageTemperatureFromOutput calculates the maximum package temperature from the turbostat output.
func maxPackageTemperatureFromOutput(turbostatOutput string) string {
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
		// only consider rows where CPU, Package, or Core is "-", these rows contain the sum of all packgaes
		if row["CPU"] != "-" && row["CPU"] != "" ||
			row["Package"] != "-" && row["Package"] != "" ||
			row["Core"] != "-" && row["Core"] != "" {
			continue
		}
		if tempStr, ok := row["PkgTmp"]; ok {
			if !ignoredFirstReading {
				// skip the first reading, it is usually not representative of the system state
				ignoredFirstReading = true
				continue
			}
			temp, err := strconv.ParseFloat(strings.TrimSpace(tempStr), 64)
			if err != nil {
				slog.Warn("unable to parse temperature value", slog.String("value", tempStr), slog.String("error", err.Error()))
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
