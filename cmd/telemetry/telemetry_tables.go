package telemetry

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"perfspect/internal/common"
	"perfspect/internal/cpus"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/table"
	"perfspect/internal/util"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// telemetry table names
const (
	CPUUtilizationTelemetryTableName        = "CPU Utilization Telemetry"
	UtilizationCategoriesTelemetryTableName = "Utilization Categories Telemetry"
	IPCTelemetryTableName                   = "IPC Telemetry"
	C6TelemetryTableName                    = "C6 Telemetry"
	FrequencyTelemetryTableName             = "Frequency Telemetry"
	IRQRateTelemetryTableName               = "IRQ Rate Telemetry"
	InstructionTelemetryTableName           = "Instruction Telemetry"
	DriveTelemetryTableName                 = "Drive Telemetry"
	NetworkTelemetryTableName               = "Network Telemetry"
	MemoryTelemetryTableName                = "Memory Telemetry"
	PowerTelemetryTableName                 = "Power Telemetry"
	TemperatureTelemetryTableName           = "Temperature Telemetry"
	GaudiTelemetryTableName                 = "Gaudi Telemetry"
	PDUTelemetryTableName                   = "PDU Telemetry"
)

// telemetry table menu labels
const (
	CPUUtilizationTelemetryMenuLabel        = "CPU Utilization"
	UtilizationCategoriesTelemetryMenuLabel = "Utilization Categories"
	IPCTelemetryMenuLabel                   = "IPC"
	C6TelemetryMenuLabel                    = "C6"
	FrequencyTelemetryMenuLabel             = "Frequency"
	IRQRateTelemetryMenuLabel               = "IRQ Rate"
	InstructionTelemetryMenuLabel           = "Instruction"
	DriveTelemetryMenuLabel                 = "Drive"
	NetworkTelemetryMenuLabel               = "Network"
	MemoryTelemetryMenuLabel                = "Memory"
	PowerTelemetryMenuLabel                 = "Power"
	TemperatureTelemetryMenuLabel           = "Temperature"
	GaudiTelemetryMenuLabel                 = "Gaudi"
	PDUTelemetryMenuLabel                   = "PDU"
)

var tableDefinitions = map[string]table.TableDefinition{
	//
	// telemetry tables
	//
	CPUUtilizationTelemetryTableName: {
		Name:      CPUUtilizationTelemetryTableName,
		MenuLabel: CPUUtilizationTelemetryMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.MpstatTelemetryScriptName,
		},
		FieldsFunc: cpuUtilizationTelemetryTableValues},
	UtilizationCategoriesTelemetryTableName: {
		Name:      UtilizationCategoriesTelemetryTableName,
		MenuLabel: UtilizationCategoriesTelemetryMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.MpstatTelemetryScriptName,
		},
		FieldsFunc: utilizationCategoriesTelemetryTableValues},
	IPCTelemetryTableName: {
		Name:          IPCTelemetryTableName,
		MenuLabel:     IPCTelemetryMenuLabel,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.TurbostatTelemetryScriptName,
		},
		FieldsFunc: ipcTelemetryTableValues},
	C6TelemetryTableName: {
		Name:          C6TelemetryTableName,
		MenuLabel:     C6TelemetryMenuLabel,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.TurbostatTelemetryScriptName,
		},
		FieldsFunc: c6TelemetryTableValues},
	FrequencyTelemetryTableName: {
		Name:          FrequencyTelemetryTableName,
		MenuLabel:     FrequencyTelemetryMenuLabel,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.TurbostatTelemetryScriptName,
		},
		FieldsFunc: frequencyTelemetryTableValues},
	IRQRateTelemetryTableName: {
		Name:      IRQRateTelemetryTableName,
		MenuLabel: IRQRateTelemetryMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.MpstatTelemetryScriptName,
		},
		FieldsFunc: irqRateTelemetryTableValues},
	DriveTelemetryTableName: {
		Name:      DriveTelemetryTableName,
		MenuLabel: DriveTelemetryMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.IostatTelemetryScriptName,
		},
		FieldsFunc: driveTelemetryTableValues},
	NetworkTelemetryTableName: {
		Name:      NetworkTelemetryTableName,
		MenuLabel: NetworkTelemetryMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.NetworkTelemetryScriptName,
		},
		FieldsFunc: networkTelemetryTableValues},
	MemoryTelemetryTableName: {
		Name:      MemoryTelemetryTableName,
		MenuLabel: MemoryTelemetryMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.MemoryTelemetryScriptName,
		},
		FieldsFunc: memoryTelemetryTableValues},
	PowerTelemetryTableName: {
		Name:          PowerTelemetryTableName,
		MenuLabel:     PowerTelemetryMenuLabel,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.TurbostatTelemetryScriptName,
		},
		FieldsFunc: powerTelemetryTableValues},
	TemperatureTelemetryTableName: {
		Name:          TemperatureTelemetryTableName,
		MenuLabel:     TemperatureTelemetryMenuLabel,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.TurbostatTelemetryScriptName,
		},
		FieldsFunc: temperatureTelemetryTableValues},
	InstructionTelemetryTableName: {
		Name:          InstructionTelemetryTableName,
		MenuLabel:     InstructionTelemetryMenuLabel,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.InstructionTelemetryScriptName,
		},
		FieldsFunc: instructionTelemetryTableValues},
	GaudiTelemetryTableName: {
		Name:          GaudiTelemetryTableName,
		MenuLabel:     GaudiTelemetryMenuLabel,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.GaudiTelemetryScriptName,
		},
		NoDataFound: "No Gaudi telemetry found. Gaudi devices and the hl-smi tool must be installed on the target system to collect Gaudi stats.",
		FieldsFunc:  gaudiTelemetryTableValues},
	PDUTelemetryTableName: {
		Name:      PDUTelemetryTableName,
		MenuLabel: PDUTelemetryMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.PDUTelemetryScriptName,
		},
		FieldsFunc: pduTelemetryTableValues},
}

func cpuUtilizationTelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Time"},
		{Name: "CPU"},
		{Name: "CORE"},
		{Name: "SOCK"},
		{Name: "NODE"},
		{Name: "%usr"},
		{Name: "%nice"},
		{Name: "%sys"},
		{Name: "%iowait"},
		{Name: "%irq"},
		{Name: "%soft"},
		{Name: "%steal"},
		{Name: "%guest"},
		{Name: "%gnice"},
		{Name: "%idle"},
	}
	reStat := regexp.MustCompile(`^(\d\d:\d\d:\d\d)\s+(\d+)\s+(\d+)\s+(\d+)\s+(-*\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)$`)
	for line := range strings.SplitSeq(outputs[script.MpstatTelemetryScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func utilizationCategoriesTelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Time"},
		{Name: "%usr"},
		{Name: "%nice"},
		{Name: "%sys"},
		{Name: "%iowait"},
		{Name: "%irq"},
		{Name: "%soft"},
		{Name: "%steal"},
		{Name: "%guest"},
		{Name: "%gnice"},
		{Name: "%idle"},
	}
	reStat := regexp.MustCompile(`^(\d\d:\d\d:\d\d)\s+all\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)$`)
	for line := range strings.SplitSeq(outputs[script.MpstatTelemetryScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func irqRateTelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Time"},
		{Name: "CPU"},
		{Name: "HI/s"},
		{Name: "TIMER/s"},
		{Name: "NET_TX/s"},
		{Name: "NET_RX/s"},
		{Name: "BLOCK/s"},
		{Name: "IRQ_POLL/s"},
		{Name: "TASKLET/s"},
		{Name: "SCHED/s"},
		{Name: "HRTIMER/s"},
		{Name: "RCU/s"},
	}
	reStat := regexp.MustCompile(`^(\d\d:\d\d:\d\d)\s+(\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)$`)
	for line := range strings.SplitSeq(outputs[script.MpstatTelemetryScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func driveTelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Time"},
		{Name: "Device"},
		{Name: "tps"},
		{Name: "kB_read/s"},
		{Name: "kB_wrtn/s"},
		{Name: "kB_dscd/s"},
	}
	// the time is on its own line, so we need to keep track of it
	reTime := regexp.MustCompile(`^\d\d\d\d-\d\d-\d\dT(\d\d:\d\d:\d\d)`)
	// don't capture the last three vals: "kB_read","kB_wrtn","kB_dscd" -- they aren't the same scale as the others
	reStat := regexp.MustCompile(`^(\w+)\s*(\d+.\d+)\s*(\d+.\d+)\s*(\d+.\d+)\s*(\d+.\d+)\s*\d+\s*\d+\s*\d+$`)
	var time string
	for line := range strings.SplitSeq(outputs[script.IostatTelemetryScriptName].Stdout, "\n") {
		match := reTime.FindStringSubmatch(line)
		if len(match) > 0 {
			time = match[1]
			continue
		}
		match = reStat.FindStringSubmatch(line)
		if len(match) > 0 {
			fields[0].Values = append(fields[0].Values, time)
			for i := range fields[1:] {
				fields[i+1].Values = append(fields[i+1].Values, match[i+1])
			}
		}
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func networkTelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Time"},
		{Name: "IFACE"},
		{Name: "rxpck/s"},
		{Name: "txpck/s"},
		{Name: "rxkB/s"},
		{Name: "txkB/s"},
	}
	// don't capture the last four vals: "rxcmp/s","txcmp/s","rxcmt/s","%ifutil" -- obscure more important vals
	reStat := regexp.MustCompile(`^(\d+:\d+:\d+)\s*(\w*)\s*(\d+.\d+)\s*(\d+.\d+)\s*(\d+.\d+)\s*(\d+.\d+)\s*\d+.\d+\s*\d+.\d+\s*\d+.\d+\s*\d+.\d+$`)
	for line := range strings.SplitSeq(outputs[script.NetworkTelemetryScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func memoryTelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Time"},
		{Name: "free"},
		{Name: "avail"},
		{Name: "used"},
		{Name: "buffers"},
		{Name: "cache"},
		{Name: "commit"},
		{Name: "active"},
		{Name: "inactive"},
		{Name: "dirty"},
	}
	reStat := regexp.MustCompile(`^(\d+:\d+:\d+)\s*(\d+)\s*(\d+)\s*(\d+)\s*\d+\.\d+\s*(\d+)\s*(\d+)\s*(\d+)\s*\d+\.\d+\s*(\d+)\s*(\d+)\s*(\d+)$`)
	for line := range strings.SplitSeq(outputs[script.MemoryTelemetryScriptName].Stdout, "\n") {
		match := reStat.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		for i := range fields {
			fields[i].Values = append(fields[i].Values, match[i+1])
		}
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func powerTelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Time"},
	}
	packageRows, err := common.TurbostatPackageRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"PkgWatt", "RAMWatt"})
	if err != nil {
		slog.Warn(err.Error())
		return []table.Field{}
	}
	for i := range packageRows {
		fields = append(fields, table.Field{Name: fmt.Sprintf("Package %d", i)})
		fields = append(fields, table.Field{Name: fmt.Sprintf("DRAM %d", i)})
	}
	// for each package
	numPackages := len(packageRows)
	for i := range packageRows {
		// traverse the rows
		for _, row := range packageRows[i] {
			if i == 0 {
				fields[0].Values = append(fields[0].Values, row[0]) // Timestamp
			}
			// append the package power and DRAM power to the fields
			fields[i*numPackages+1].Values = append(fields[i*numPackages+1].Values, row[1]) // Package power
			fields[i*numPackages+2].Values = append(fields[i*numPackages+2].Values, row[2]) // DRAM power
		}
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func temperatureTelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Time"},
		{Name: "Core (Avg.)"},
	}
	platformRows, err := common.TurbostatPlatformRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"CoreTmp"})
	if err != nil {
		slog.Warn(err.Error()) // not all systems report core temperature, e.g., cloud VMs
		return []table.Field{}
	}
	packageRows, err := common.TurbostatPackageRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"PkgTmp"})
	if err != nil {
		// not an error, just means no package rows (package temperature)
		slog.Warn(err.Error())
	}
	// add the package rows to the fields
	for i := range packageRows {
		fields = append(fields, table.Field{Name: fmt.Sprintf("Package %d", i)})
	}
	// for each platform row
	for i := range platformRows {
		// append the timestamp to the fields
		fields[0].Values = append(fields[0].Values, platformRows[i][0]) // Timestamp
		// append the core temperature values to the fields
		fields[1].Values = append(fields[1].Values, platformRows[i][1]) // Core temperature
	}
	// for each package
	for i := range packageRows {
		// traverse the rows
		for _, row := range packageRows[i] {
			// append the package temperature to the fields
			fields[i+2].Values = append(fields[i+2].Values, row[1]) // Package temperature
		}
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func frequencyTelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Time"},
		{Name: "Core (Avg.)"},
	}
	platformRows, err := common.TurbostatPlatformRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"Bzy_MHz"})
	if err != nil {
		slog.Warn(err.Error())
		return []table.Field{}
	}
	packageRows, err := common.TurbostatPackageRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"UncMHz"})
	if err != nil {
		// not an error, just means no package rows (uncore frequency)
		slog.Warn(err.Error())
	}
	// add the package rows to the fields
	for i := range packageRows {
		fields = append(fields, table.Field{Name: fmt.Sprintf("Uncore Package %d", i)})
	}
	// for each platform row
	for i := range platformRows {
		// append the timestamp to the fields
		fields[0].Values = append(fields[0].Values, platformRows[i][0]) // Timestamp
		// append the core frequency values to the fields
		fields[1].Values = append(fields[1].Values, platformRows[i][1]) // Core frequency
	}
	// for each package
	for i := range packageRows {
		// traverse the rows
		for _, row := range packageRows[i] {
			// append the package frequency to the fields
			fields[i+2].Values = append(fields[i+2].Values, row[1]) // Package frequency
		}
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func ipcTelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Time"},
		{Name: "Core (Avg.)"},
	}
	platformRows, err := common.TurbostatPlatformRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"IPC"})
	if err != nil {
		slog.Warn(err.Error())
		return []table.Field{}
	}
	if len(platformRows) == 0 {
		slog.Warn("no platform rows found in turbostat telemetry output")
		return []table.Field{}
	}
	// for each platform row
	for i := range platformRows {
		// append the timestamp to the fields
		fields[0].Values = append(fields[0].Values, platformRows[i][0]) // Timestamp
		// append the core IPC values to the fields
		fields[1].Values = append(fields[1].Values, platformRows[i][1]) // Core IPC
	}
	return fields
}

func c6TelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Time"},
		{Name: "Package (Avg.)"},
		{Name: "Core (Avg.)"},
	}
	platformRows, err := common.TurbostatPlatformRows(outputs[script.TurbostatTelemetryScriptName].Stdout, []string{"C6%", "CPU%c6"})
	if err != nil {
		slog.Warn(err.Error())
		return []table.Field{}
	}
	if len(platformRows) == 0 {
		slog.Warn("no platform rows found in turbostat telemetry output")
		return []table.Field{}
	}
	// for each platform row
	for i := range platformRows {
		// append the timestamp to the fields
		fields[0].Values = append(fields[0].Values, platformRows[i][0]) // Timestamp
		// append the C6 residency values to the fields
		fields[1].Values = append(fields[1].Values, platformRows[i][1]) // C6%
		// append the CPU C6 residency values to the fields
		fields[2].Values = append(fields[2].Values, platformRows[i][2]) // CPU%c6
	}
	return fields
}

func gaudiTelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	// parse the CSV output
	csvOutput := outputs[script.GaudiTelemetryScriptName].Stdout
	if csvOutput == "" {
		return []table.Field{}
	}
	r := csv.NewReader(strings.NewReader(csvOutput))
	rows, err := r.ReadAll()
	if err != nil {
		slog.Error(err.Error())
		return []table.Field{}
	}
	if len(rows) < 2 {
		slog.Error("gaudi stats output is not in expected format")
		return []table.Field{}
	}
	// build fields to match CSV output from hl_smi tool
	fields := []table.Field{}
	// first row is the header, extract field names
	for _, fieldName := range rows[0] {
		fields = append(fields, table.Field{Name: strings.TrimSpace(fieldName)})
	}
	// values start in 2nd row
	for _, row := range rows[1:] {
		for i := range fields {
			// reformat the timestamp field to only include the time
			if i == 0 {
				// parse the timestamp field's value
				rowTime, err := time.Parse("Mon Jan 2 15:04:05 MST 2006", row[i])
				if err != nil {
					err = fmt.Errorf("unable to parse Gaudi telemetry timestamp: %s", row[i])
					slog.Error(err.Error())
					return []table.Field{}
				}
				// reformat the timestamp field's value to include time only
				timestamp := rowTime.Format("15:04:05")
				fields[i].Values = append(fields[i].Values, timestamp)
			} else {
				fields[i].Values = append(fields[i].Values, strings.TrimSpace(row[i]))
			}
		}
	}
	return fields
}

func pduTelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	// extract PDU fields and their values from PDU telemetry script output
	// output is CSV formatted:
	//   Timestamp,ActivePower(W)
	//   18:32:38,123.45
	//   18:32:40,124.10
	//   ...
	fields := []table.Field{}
	reader := csv.NewReader(strings.NewReader(outputs[script.PDUTelemetryScriptName].Stdout))
	records, err := reader.ReadAll()
	if err != nil {
		slog.Error("failed to read PDU telemetry CSV output", slog.String("error", err.Error()))
		return []table.Field{}
	}
	if len(records) == 0 {
		return []table.Field{}
	}
	// first row is the header
	for _, header := range records[0] {
		fields = append(fields, table.Field{Name: header, Values: []string{}})
	}
	// subsequent rows are data
	for _, record := range records[1:] {
		if len(record) != len(fields) {
			slog.Error("unexpected number of fields in PDU telemetry output", slog.Int("expected", len(fields)), slog.Int("got", len(record)))
			return []table.Field{}
		}
		for i, value := range record {
			fields[i].Values = append(fields[i].Values, value)
		}
	}
	return fields
}

func instructionTelemetryTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	// first two lines are not part of the CSV output, they are the start time and interval
	var startTime time.Time
	var interval int
	lines := strings.Split(outputs[script.InstructionTelemetryScriptName].Stdout, "\n")
	if len(lines) < 4 {
		slog.Warn("no data found in instruction mix output")
		return []table.Field{}
	}
	// TIME
	line := lines[0]
	if !strings.HasPrefix(line, "TIME") {
		slog.Error("instruction mix output is not in expected format, missing TIME")
		return []table.Field{}
	} else {
		val := strings.Split(line, " ")[1]
		var err error
		startTime, err = time.Parse("15:04:05", val)
		if err != nil {
			slog.Error(fmt.Sprintf("unable to parse instruction mix start time: %s", val))
			return []table.Field{}
		}
	}
	// INTERVAL
	line = lines[1]
	if !strings.HasPrefix(line, "INTERVAL") {
		slog.Error("instruction mix output is not in expected format, missing INTERVAL")
		return []table.Field{}
	} else {
		val := strings.Split(line, " ")[1]
		var err error
		interval, err = strconv.Atoi(val)
		if err != nil {
			slog.Error(fmt.Sprintf("unable to convert instruction mix interval to int: %s", val))
			return []table.Field{}
		}
	}
	// remove blank lines that occur throughout the remaining lines
	csvLines := []string{}
	for _, line := range lines[2:] { // skip the TIME and INTERVAL lines
		if line != "" {
			csvLines = append(csvLines, line)
		}
	}
	if len(csvLines) < 2 {
		slog.Error("instruction mix CSV output is not in expected format, missing header and data")
		return []table.Field{}
	}
	// if processwatch was killed, it may print a partial output line at the end
	// check if the last line is a partial line by comparing the number of fields in the last line to the number of fields in the header
	if len(strings.Split(csvLines[len(csvLines)-1], ",")) != len(strings.Split(csvLines[0], ",")) {
		slog.Debug("removing partial line from instruction mix output", "line", csvLines[len(csvLines)-1], "lineNo", len(csvLines)-1)
		csvLines = csvLines[:len(csvLines)-1] // remove the last line
	}
	// CSV
	r := csv.NewReader(strings.NewReader(strings.Join(csvLines, "\n")))
	rows, err := r.ReadAll()
	if err != nil {
		slog.Error(err.Error())
		return []table.Field{}
	}
	if len(rows) < 2 {
		slog.Error("instruction mix CSV output is not in expected format")
		return []table.Field{}
	}
	fields := []table.Field{{Name: "Time"}}
	// first row is the header, extract field names, skip the first three fields (interval, pid, name)
	if len(rows[0]) < 3 {
		slog.Error("not enough headers in instruction mix CSV output", slog.Any("headers", rows[0]))
		return []table.Field{}
	}
	for _, field := range rows[0][3:] {
		fields = append(fields, table.Field{Name: field})
	}
	sample := -1
	// values start in 2nd row, we're only interested in the first row of the sample
	for _, row := range rows[1:] {
		if len(row) < 2+len(fields) {
			continue
		}
		rowSample, err := strconv.Atoi(row[0])
		if err != nil {
			slog.Error(fmt.Sprintf("unable to convert instruction mix sample to int: %s", row[0]))
			continue
		}
		if rowSample != sample { // new sample
			sample = rowSample
			for i := range fields {
				if i == 0 {
					fields[i].Values = append(fields[i].Values, startTime.Add(time.Duration(interval+(sample*interval))*time.Second).Format("15:04:05"))
				} else {
					fields[i].Values = append(fields[i].Values, row[i+2])
				}
			}
		}
	}
	return fields
}

func telemetryTableHTMLRenderer(tableValues table.TableValues, data [][]float64, datasetNames []string, chartConfig report.ChartTemplateStruct, datasetHiddenFlags []bool) string {
	tsFieldIdx := 0
	var timestamps []string
	for i := range tableValues.Fields[0].Values {
		timestamp := tableValues.Fields[tsFieldIdx].Values[i]
		if !slices.Contains(timestamps, timestamp) { // could be slow if list is long
			timestamps = append(timestamps, timestamp)
		}
	}
	return renderLineChart(timestamps, data, datasetNames, chartConfig, datasetHiddenFlags)
}

// renderLineChart generates an HTML string for a line chart using the provided data and configuration.
//
// Parameters:
//
//	xAxisLabels        - Slice of strings representing the labels for the X axis.
//	data               - 2D slice of float64 values, where each inner slice represents a dataset's data points.
//	datasetNames       - Slice of strings representing the names of each dataset.
//	config             - chartTemplateStruct containing chart configuration options.
//	datasetHiddenFlags - Slice of booleans indicating whether each dataset should be hidden initially.
//
// Returns:
//
//	A string containing the rendered HTML for the line chart.
func renderLineChart(xAxisLabels []string, data [][]float64, datasetNames []string, config report.ChartTemplateStruct, datasetHiddenFlags []bool) string {
	allFormattedPoints := []string{}
	for dataIdx := range data {
		formattedPoints := []string{}
		for _, point := range data[dataIdx] {
			formattedPoints = append(formattedPoints, fmt.Sprintf("%f", point))
		}
		allFormattedPoints = append(allFormattedPoints, strings.Join(formattedPoints, ","))
	}
	return report.RenderChart("line", allFormattedPoints, datasetNames, xAxisLabels, config, datasetHiddenFlags)
}

func cpuUtilizationTelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	// collect the busy (100 - idle) values for each CPU
	cpuBusyStats := make(map[int][]float64)
	idleFieldIdx := len(tableValues.Fields) - 1
	cpuFieldIdx := 1
	for i := range tableValues.Fields[0].Values {
		idle, err := strconv.ParseFloat(tableValues.Fields[idleFieldIdx].Values[i], 64)
		if err != nil {
			continue
		}
		busy := 100 - idle
		cpu, err := strconv.Atoi(tableValues.Fields[cpuFieldIdx].Values[i])
		if err != nil {
			continue
		}
		if _, ok := cpuBusyStats[cpu]; !ok {
			cpuBusyStats[cpu] = []float64{}
		}
		cpuBusyStats[cpu] = append(cpuBusyStats[cpu], busy)
	}
	// sort map keys by cpu number
	var keys []int
	for cpu := range cpuBusyStats {
		keys = append(keys, cpu)
	}
	sort.Ints(keys)
	// build the data
	for _, cpu := range keys {
		if len(cpuBusyStats[cpu]) > 0 {
			data = append(data, cpuBusyStats[cpu])
			datasetNames = append(datasetNames, fmt.Sprintf("CPU %d", cpu))
		}
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
		XaxisText:     "Time",
		YaxisText:     "% Utilization",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "false",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "100",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil)
}

func utilizationCategoriesTelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []float64{}
		for _, val := range field.Values {
			if val == "" {
				break
			}
			util, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing percentage", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, util)
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
		XaxisText:     "Time",
		YaxisText:     "% Utilization",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "100",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil)
}

func irqRateTelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[2:] { // 1 data set per field, e.g., %usr, %nice, etc., skip Time and CPU fields
		datasetNames = append(datasetNames, field.Name)
		// sum the values in the field per timestamp, store the sum as a point
		timeStamp := tableValues.Fields[0].Values[0]
		points := []float64{}
		total := 0.0
		for i := range field.Values {
			if tableValues.Fields[0].Values[i] != timeStamp { // new timestamp?
				points = append(points, total)
				total = 0.0
				timeStamp = tableValues.Fields[0].Values[i]
			}
			val, err := strconv.ParseFloat(field.Values[i], 64)
			if err != nil {
				slog.Error("error parsing value", slog.String("error", err.Error()))
				return ""
			}
			total += val
		}
		points = append(points, total) // add the point for the last timestamp
		// save the points in the data slice
		data = append(data, points)
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
		XaxisText:     "Time",
		YaxisText:     "IRQ/s",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil)
}

// driveTelemetryTableHTMLRenderer renders charts of drive statistics
// - one scatter chart per drive, showing the drive's utilization over time
// - each drive stat is a separate dataset within the chart
func driveTelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	var out strings.Builder
	driveStats := make(map[string][][]string)
	for i := range tableValues.Fields[0].Values {
		drive := tableValues.Fields[1].Values[i]
		if _, ok := driveStats[drive]; !ok {
			driveStats[drive] = make([][]string, len(tableValues.Fields)-2)
		}
		for j := range len(tableValues.Fields) - 2 {
			driveStats[drive][j] = append(driveStats[drive][j], tableValues.Fields[j+2].Values[i])
		}
	}
	var keys []string
	for drive := range driveStats {
		keys = append(keys, drive)
	}
	sort.Strings(keys)
	for _, drive := range keys {
		data := [][]float64{}
		datasetNames := []string{}
		for i, statVals := range driveStats[drive] {
			points := []float64{}
			for i, val := range statVals {
				if val == "" {
					slog.Error("empty stat value", slog.String("drive", drive), slog.Int("index", i))
					return ""
				}
				util, err := strconv.ParseFloat(val, 64)
				if err != nil {
					slog.Error("error parsing stat", slog.String("error", err.Error()))
					return ""
				}
				points = append(points, util)
			}
			if len(points) > 0 {
				data = append(data, points)
				datasetNames = append(datasetNames, tableValues.Fields[i+2].Name)
			}
		}
		chartConfig := report.ChartTemplateStruct{
			ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
			XaxisText:     "Time",
			YaxisText:     "",
			TitleText:     drive,
			DisplayTitle:  "true",
			DisplayLegend: "true",
			AspectRatio:   "2",
			SuggestedMin:  "0",
			SuggestedMax:  "0",
		}
		out.WriteString(telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil))
	}
	return out.String()
}

// networkTelemetryTableHTMLRenderer renders charts of network device statistics
// - one scatter chart per network device, showing the device's utilization over time
// - each network stat is a separate dataset within the chart
func networkTelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	var out strings.Builder
	nicStats := make(map[string][][]string)
	for i := range tableValues.Fields[0].Values {
		drive := tableValues.Fields[1].Values[i]
		if _, ok := nicStats[drive]; !ok {
			nicStats[drive] = make([][]string, len(tableValues.Fields)-2)
		}
		for j := range len(tableValues.Fields) - 2 {
			nicStats[drive][j] = append(nicStats[drive][j], tableValues.Fields[j+2].Values[i])
		}
	}
	var keys []string
	for drive := range nicStats {
		keys = append(keys, drive)
	}
	sort.Strings(keys)
	for _, nic := range keys {
		data := [][]float64{}
		datasetNames := []string{}
		for i, statVals := range nicStats[nic] {
			points := []float64{}
			for i, val := range statVals {
				if val == "" {
					slog.Error("empty stat value", slog.String("nic", nic), slog.Int("index", i))
					return ""
				}
				util, err := strconv.ParseFloat(val, 64)
				if err != nil {
					slog.Error("error parsing stat", slog.String("error", err.Error()))
					return ""
				}
				points = append(points, util)
			}
			if len(points) > 0 {
				data = append(data, points)
				datasetNames = append(datasetNames, tableValues.Fields[i+2].Name)
			}
		}
		chartConfig := report.ChartTemplateStruct{
			ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
			XaxisText:     "Time",
			YaxisText:     "",
			TitleText:     nic,
			DisplayTitle:  "true",
			DisplayLegend: "true",
			AspectRatio:   "2",
			SuggestedMin:  "0",
			SuggestedMax:  "0",
		}
		out.WriteString(telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil))
	}
	return out.String()
}

func memoryTelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []float64{}
		for _, val := range field.Values {
			if val == "" {
				break
			}
			stat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing stat", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, stat)
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
		XaxisText:     "Time",
		YaxisText:     "kilobytes",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil)
}

func averageFrequencyTelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []float64{}
		for _, val := range field.Values {
			if val == "" {
				break
			}
			stat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing stat", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, stat)
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
		XaxisText:     "Time",
		YaxisText:     "MHz",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil)
}

func powerTelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []float64{}
		for _, val := range field.Values {
			if val == "" {
				break
			}
			stat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing stat", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, stat)
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
		XaxisText:     "Time",
		YaxisText:     "Watts",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil)
}

func temperatureTelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []float64{}
		for _, val := range field.Values {
			if val == "" {
				break
			}
			stat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing stat", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, stat)
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
		XaxisText:     "Time",
		YaxisText:     "Celsius",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil)
}

func ipcTelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []float64{}
		for _, val := range field.Values {
			if val == "" {
				break
			}
			stat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing stat", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, stat)
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
		XaxisText:     "Time",
		YaxisText:     "IPC",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil)
}

func c6TelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []float64{}
		for _, val := range field.Values {
			if val == "" {
				break
			}
			stat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing stat", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, stat)
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
		XaxisText:     "Time",
		YaxisText:     "% C6 Residency",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil)
}

// instructionTelemetryTableHTMLRenderer renders instruction set usage statistics.
// Each category is a separate dataset within the chart.
// Categories with zero total usage are hidden by default.
// Categories are sorted in two tiers: first, all non-zero categories are sorted alphabetically;
// then, all zero-sum categories are sorted alphabetically and placed after the non-zero categories.
func instructionTelemetryTableHTMLRenderer(tableValues table.TableValues, targetname string) string {
	// Collect entries with their sums so we can sort per requirements
	type instrEntry struct {
		name   string
		points []float64
		sum    float64
	}
	entries := []instrEntry{}
	for _, field := range tableValues.Fields[1:] { // skip timestamp field
		points := []float64{}
		sum := 0.0
		for _, val := range field.Values {
			if val == "" { // end of data for this category
				break
			}
			stat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing stat", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, stat)
			sum += stat
		}
		if len(points) > 0 { // only include categories with at least one point
			entries = append(entries, instrEntry{name: field.Name, points: points, sum: sum})
		}
	}
	// Partition into non-zero and zero-sum groups
	nonZero := []instrEntry{}
	zero := []instrEntry{}
	for _, e := range entries {
		if e.sum > 0 {
			nonZero = append(nonZero, e)
		} else {
			zero = append(zero, e)
		}
	}
	sort.Slice(nonZero, func(i, j int) bool { return nonZero[i].name < nonZero[j].name })
	sort.Slice(zero, func(i, j int) bool { return zero[i].name < zero[j].name })
	ordered := append(nonZero, zero...)
	data := make([][]float64, 0, len(ordered))
	datasetNames := make([]string, 0, len(ordered))
	hiddenFlags := make([]bool, 0, len(ordered))
	for _, e := range ordered {
		data = append(data, e.points)
		datasetNames = append(datasetNames, e.name)
		// hide zero-sum categories by default
		hiddenFlags = append(hiddenFlags, e.sum == 0)
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
		XaxisText:     "Time",
		YaxisText:     "% Samples",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "1", // extra tall due to large number of data sets
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, hiddenFlags)
}

func renderGaudiStatsChart(tableValues table.TableValues, chartStatFieldName string, titleText string, yAxisText string, suggestedMax string) string {
	data := [][]float64{}
	datasetNames := []string{}
	// timestamp is in the first field
	// find the module_id field index
	moduleIdFieldIdx, err := table.GetFieldIndex("module_id", tableValues)
	if err != nil {
		slog.Error("no gaudi module_id field found")
		return ""
	}
	// find the chartStatFieldName field index
	chartStatFieldIndex, err := table.GetFieldIndex(chartStatFieldName, tableValues)
	if err != nil {
		slog.Error("no gaudi chartStatFieldName field found")
		return ""
	}
	// group the data points by module_id
	moduleStat := make(map[string][]float64)
	for i := range tableValues.Fields[0].Values {
		moduleId := tableValues.Fields[moduleIdFieldIdx].Values[i]
		val, err := strconv.ParseFloat(tableValues.Fields[chartStatFieldIndex].Values[i], 64)
		if err != nil {
			slog.Error("error parsing utilization", slog.String("error", err.Error()))
			return ""
		}
		if _, ok := moduleStat[moduleId]; !ok {
			moduleStat[moduleId] = []float64{}
		}
		moduleStat[moduleId] = append(moduleStat[moduleId], val)
	}
	// sort the module ids
	var moduleIds []string
	for moduleId := range moduleStat {
		moduleIds = append(moduleIds, moduleId)
	}
	sort.Strings(moduleIds)
	// build the data
	for _, moduleId := range moduleIds {
		if len(moduleStat[moduleId]) > 0 {
			data = append(data, moduleStat[moduleId])
			datasetNames = append(datasetNames, "module "+moduleId)
		}
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
		XaxisText:     "Time",
		YaxisText:     yAxisText,
		TitleText:     titleText,
		DisplayTitle:  "true",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  suggestedMax,
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil)
}

func gaudiTelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	out := ""
	out += renderGaudiStatsChart(tableValues, "utilization.aip [%]", "Utilization", "% Utilization", "100")
	out += renderGaudiStatsChart(tableValues, "memory.free [MiB]", "Memory Free", "Memory (MiB)", "0")
	out += renderGaudiStatsChart(tableValues, "memory.used [MiB]", "Memory Used", "Memory (MiB)", "0")
	out += renderGaudiStatsChart(tableValues, "power.draw [W]", "Power", "Watts", "0")
	out += renderGaudiStatsChart(tableValues, "temperature.aip [C]", "Temperature", "Temperature (C)", "0")
	return out
}

func pduTelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	data := [][]float64{}
	for _, field := range tableValues.Fields[1:] {
		points := []float64{}
		for _, val := range field.Values {
			if val == "" {
				break
			}
			stat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing stat", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, stat)
		}
		if len(points) > 0 {
			data = append(data, points)
		}
	}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		datasetNames = append(datasetNames, field.Name)
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, util.RandUint(10000)),
		XaxisText:     "Time",
		YaxisText:     "Watts",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil)
}
