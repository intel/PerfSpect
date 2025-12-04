package report

import (
	"fmt"
	"log/slog"
	"slices"
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
// Only the Summary and Packages rows are returned, i.e., rows for individual cores/CPUs are ignored.
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
			// Try to parse as HH:MM:SS
			var err error
			timestamp, err = time.Parse("15:04:05", val)
			if err != nil {
				slog.Error("unable to parse time", slog.String("value", val), slog.String("error", err.Error()))
				return nil, fmt.Errorf("unable to parse time: %s", val)
			}
			timeParsed = true
			continue
		}
		// parse the fields in the line
		fields := strings.Fields(line)
		// if this is a header line
		if len(fields) >= 1 && slices.Contains([]string{"package", "die", "node", "core", "cpu"}, strings.ToLower(fields[0])) {
			if len(headers) == 0 {
				headers = fields // first line with a column name is the header
			} else {
				// bump the timestamp to the next interval
				if timeParsed && interval > 0 {
					timestamp = timestamp.Add(time.Duration(interval) * time.Second)
				}
			}
			continue
		}
		if len(headers) == 0 {
			continue // skip data lines before first header
		}
		if len(fields) != len(headers) {
			continue // skip core lines
		}
		row := make(map[string]string, len(headers))
		for i, h := range headers {
			row[h] = fields[i]
		}
		// Add timestamp to row
		if timeParsed && interval > 0 {
			row["timestamp"] = timestamp.Format("15:04:05")
		}
		rows = append(rows, row)
		rowCount++
	}
	return rows, nil
}

// turbostatPlatformRows parses the output of the turbostat script and returns the rows
// for the platform (summary) only, for the specified field names.
// The "platform" rows are those where Package, Die, Core, and CPU are all "-".
// The first column is the sample time, and the rest are the values for the specified fields.
func turbostatPlatformRows(turboStatScriptOutput string, fieldNames []string) ([][]string, error) {
	if len(fieldNames) == 0 {
		err := fmt.Errorf("no field names provided")
		slog.Error(err.Error())
		return nil, err
	}
	rows, err := parseTurbostatOutput(turboStatScriptOutput)
	if err != nil {
		err := fmt.Errorf("unable to parse turbostat output: %w", err)
		return nil, err
	}
	if len(rows) == 0 {
		err := fmt.Errorf("turbostat output is empty")
		return nil, err
	}
	// filter the rows to the summary rows only
	var fieldValues [][]string
	for _, row := range rows {
		if !isPlatformRow(row) {
			continue
		}
		// this is a summary row, extract the values for the specified fields
		rowValues := make([]string, len(fieldNames)+1) // +1 for the sample time
		rowValues[0] = row["timestamp"]                // first column is the sample time
		for i, fieldName := range fieldNames {
			if value, ok := row[fieldName]; ok {
				rowValues[i+1] = value // +1 for the sample time
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

// isPlatformRow returns true if the row represents a platform (summary) row.
// only consider rows where Package, Die, Node, Core, and CPU are "-" (or don't exist), these rows contain the sum of all packages
func isPlatformRow(row map[string]string) bool {
	for _, header := range []string{"Package", "Die", "Node", "Core", "CPU"} {
		if val, ok := row[header]; ok && val != "-" {
			return false
		}
	}
	return true
}

// turbostatPackageRows
// parses the output of the turbostat script and returns the rows
// for each package, for the specified field names.
// The first column is the sample time, and the rest are the values for the specified fields.
func turbostatPackageRows(turboStatScriptOutput string, fieldNames []string) ([][][]string, error) {
	if len(fieldNames) == 0 {
		err := fmt.Errorf("no field names provided")
		return nil, err
	}
	rows, err := parseTurbostatOutput(turboStatScriptOutput)
	if err != nil {
		err := fmt.Errorf("unable to parse turbostat output: %w", err)
		return nil, err
	}
	if len(rows) == 0 {
		err := fmt.Errorf("turbostat output is empty")
		return nil, err
	}
	var packageRows [][][]string
	for _, row := range rows {
		// not all instances of turbostat output include a Package column
		// if it is missing assume 1 package, set it to 0 for rows where CPU is 0
		if _, ok := row["Package"]; !ok {
			if row["CPU"] == "0" {
				row["Package"] = "0"
			} else {
				continue // skip rows that are not package rows
			}
		}
		if !isPackageRow(row) {
			continue
		}
		// this is a package row, extract the values for the specified fields
		rowValues := make([]string, len(fieldNames)+1) // +1 for the sample time
		rowValues[0] = row["timestamp"]                // first column is the sample time
		for i, fieldName := range fieldNames {
			if value, ok := row[fieldName]; ok {
				rowValues[i+1] = value // +1 for the sample time
			} else {
				return nil, fmt.Errorf("field %s not found in turbostat output", fieldName)
			}
		}
		packageNum, err := strconv.Atoi(row["Package"])
		if err != nil {
			return nil, fmt.Errorf("unable to parse package number: %s", row["Package"])
		}
		// if we have a new package, start a new package row
		if len(packageRows) < packageNum+1 {
			packageRows = append(packageRows, [][]string{rowValues})
		} else {
			// append to the associated package row
			packageRows[packageNum] = append(packageRows[packageNum], rowValues)
		}
	}
	if len(packageRows) == 0 {
		err := fmt.Errorf("no data found in turbostat output for fields: %s", fieldNames)
		return nil, err
	}
	return packageRows, nil
}

// isPackageRow returns true if the row represents a package row.
// only consider rows where Package is not "-" or empty
func isPackageRow(row map[string]string) bool {
	if val, ok := row["Package"]; ok && val != "-" {
		return true
	}
	return false
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
