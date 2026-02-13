// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package benchmark

import (
	"fmt"
	"log/slog"
	"strconv"

	"perfspect/internal/report"
	"perfspect/internal/table"
	"perfspect/internal/util"
)

func renderFrequencyTable(tableValues table.TableValues) (out string) {
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
		Type:          "scatter",
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
		points := []report.ScatterPoint{}
		for valIdx := range tableValues.Fields[0].Values {
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
		Type:          "scatter",
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
