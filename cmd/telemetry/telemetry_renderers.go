// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package telemetry

import (
	"fmt"
	"log/slog"
	"perfspect/internal/report"
	"perfspect/internal/table"
	"perfspect/internal/util"
	"slices"
	"sort"
	"strconv"
	"strings"
)

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
	return report.RenderChart("stackedBar", allFormattedPoints, datasetNames, xAxisLabels, config, datasetHiddenFlags)
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
		SuggestedMax:  "100", // TODO AG: confirm that this suggested max makes sense
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

func kernelTelemetryTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
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
		YaxisText:     "count per second",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig, nil)
}
