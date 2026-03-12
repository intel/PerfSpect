// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package benchmark

import (
	"fmt"
	"html"
	"log/slog"
	"strconv"

	"perfspect/internal/report"
	"perfspect/internal/table"
	"perfspect/internal/util"
)

func renderFrequencyTable(tableValues table.TableValues) (out string) {
	if len(tableValues.Fields) < 2 {
		slog.Error("insufficient fields in table, expected at least 2", slog.String("table", tableValues.Name), slog.Int("fields", len(tableValues.Fields)))
		return
	}
	var rows [][]string
	headers := []string{""}
	valuesStyles := [][]string{}
	for i := range tableValues.Fields[0].Values {
		headers = append(headers, fmt.Sprintf("%d", i+1))
	}
	for _, field := range tableValues.Fields[1:] {
		row := append([]string{report.CreateFieldNameWithDescription(field.Name, field.Description)}, field.Values...)
		rows = append(rows, row)
		valuesStyles = append(valuesStyles, []string{"font-weight:bold"})
	}
	out = report.RenderHTMLTable(headers, rows, "pure-table pure-table-striped", valuesStyles)
	return
}

func coreTurboFrequencyTableHTMLRenderer(tableValues table.TableValues) string {
	if len(tableValues.Fields) < 2 {
		slog.Error("insufficient fields in table, expected at least 2", slog.String("table", tableValues.Name), slog.Int("fields", len(tableValues.Fields)))
		return ""
	}
	data := [][]report.ScatterPoint{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []report.ScatterPoint{}
		for i, val := range field.Values {
			if val == "" {
				break
			}
			freq, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing frequency", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, report.ScatterPoint{X: float64(i + 1), Y: freq})
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("turboFrequency%d", util.RandUint(10000)),
		XaxisText:     "Core Count",
		YaxisText:     "Frequency (GHz)",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "4",
		SuggestedMin:  "2",
		SuggestedMax:  "4",
	}
	out := report.RenderScatterChart(data, datasetNames, chartConfig)
	out += "\n"
	out += renderFrequencyTable(tableValues)
	return out
}

func frequencyBenchmarkTableHtmlRenderer(tableValues table.TableValues, targetName string) string {
	return coreTurboFrequencyTableHTMLRenderer(tableValues)
}

func memoryBenchmarkTableHtmlRenderer(tableValues table.TableValues, targetName string) string {
	return memoryBenchmarkTableMultiTargetHtmlRenderer([]table.TableValues{tableValues}, []string{targetName})
}

func memoryBenchmarkTableMultiTargetHtmlRenderer(allTableValues []table.TableValues, targetNames []string) string {
	data := [][]report.ScatterPoint{}
	datasetNames := []string{}
	for targetIdx, tableValues := range allTableValues {
		if len(tableValues.Fields) < 2 {
			slog.Error("insufficient fields in table, expected at least 2", slog.String("table", tableValues.Name), slog.Int("fields", len(tableValues.Fields)))
			continue
		}
		points := []report.ScatterPoint{}
		for valIdx := range tableValues.Fields[0].Values {
			if valIdx >= len(tableValues.Fields[1].Values) {
				slog.Error("field values length mismatch", slog.String("table", tableValues.Name), slog.Int("index", valIdx))
				break
			}
			latency, err := strconv.ParseFloat(tableValues.Fields[0].Values[valIdx], 64)
			if err != nil {
				slog.Error("error parsing latency", slog.String("error", err.Error()))
				return ""
			}
			bandwidth, err := strconv.ParseFloat(tableValues.Fields[1].Values[valIdx], 64)
			if err != nil {
				slog.Error("error parsing bandwidth", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, report.ScatterPoint{X: bandwidth, Y: latency})
		}
		data = append(data, points)
		datasetNames = append(datasetNames, targetNames[targetIdx])
	}
	chartConfig := report.ChartTemplateStruct{
		ID:            fmt.Sprintf("latencyBandwidth%d", util.RandUint(10000)),
		XaxisText:     "Bandwidth (GB/s)",
		YaxisText:     "Latency (ns)",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "4",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return report.RenderScatterChart(data, datasetNames, chartConfig)
}

// renderNUMAMatrixHeatmapTable renders a NUMA matrix (bandwidth or latency) as an HTML table
// with heatmap cell background colors. higherIsBetter true = green for high values (e.g. bandwidth);
// false = green for low values (e.g. latency).
func renderNUMAMatrixHeatmapTable(tableValues table.TableValues, higherIsBetter bool) string {
	if len(tableValues.Fields) < 2 {
		slog.Error("insufficient fields for NUMA matrix", slog.String("table", tableValues.Name), slog.Int("fields", len(tableValues.Fields)))
		return ""
	}
	rows := len(tableValues.Fields[0].Values)
	cols := len(tableValues.Fields)
	// Parse numeric matrix (skip field 0 = row header column)
	type cell struct {
		text string
		val  float64
		ok   bool
	}
	matrix := make([][]cell, rows)
	var minVal, maxVal float64
	first := true
	for r := 0; r < rows; r++ {
		matrix[r] = make([]cell, cols)
		for c := 0; c < cols; c++ {
			matrix[r][c].text = tableValues.Fields[c].Values[r]
			if c == 0 {
				matrix[r][c].ok = false
				continue
			}
			v, err := strconv.ParseFloat(tableValues.Fields[c].Values[r], 64)
			if err != nil {
				matrix[r][c].ok = false
				continue
			}
			matrix[r][c].val = v
			matrix[r][c].ok = true
			if first {
				minVal, maxVal = v, v
				first = false
			} else {
				if v < minVal {
					minVal = v
				}
				if v > maxVal {
					maxVal = v
				}
			}
		}
	}
	// Build headers and rows for RenderHTMLTable
	headers := make([]string, cols)
	for c := 0; c < cols; c++ {
		headers[c] = tableValues.Fields[c].Name
	}
	tableRows := make([][]string, rows)
	valuesStyles := make([][]string, rows)
	span := maxVal - minVal
	if span == 0 {
		span = 1
	}
	for r := 0; r < rows; r++ {
		tableRows[r] = make([]string, cols)
		valuesStyles[r] = make([]string, cols)
		for c := 0; c < cols; c++ {
			tableRows[r][c] = html.EscapeString(matrix[r][c].text)
			if c == 0 {
				valuesStyles[r][c] = "font-weight:bold"
				continue
			}
			if !matrix[r][c].ok {
				continue
			}
			v := matrix[r][c].val
			var t float64
			if higherIsBetter {
				t = (v - minVal) / span
			} else {
				t = (maxVal - v) / span
			}
			valuesStyles[r][c] = heatmapCellStyle(t)
		}
	}
	return report.RenderHTMLTable(headers, tableRows, "pure-table pure-table-striped", valuesStyles)
}

// heatmapCellStyle returns a CSS background-color for a normalized value t in [0,1].
// t=0 -> red, t=1 -> green; interpolates in RGB.
func heatmapCellStyle(t float64) string {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	// Red #e03131 to Green #2f9e44
	r := uint8(224 - (224-47)*t)
	g := uint8(49 + (158-49)*t)
	b := uint8(49 + (68-49)*t)
	return fmt.Sprintf("background-color: rgb(%d,%d,%d)", r, g, b)
}

func memoryNUMABandwidthMatrixTableHtmlRenderer(tableValues table.TableValues, targetName string) string {
	return renderNUMAMatrixHeatmapTable(tableValues, true)
}

func memoryNUMALatencyMatrixTableHtmlRenderer(tableValues table.TableValues, targetName string) string {
	return renderNUMAMatrixHeatmapTable(tableValues, false)
}
