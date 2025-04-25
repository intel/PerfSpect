package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

func printMetricsJSON(metricFrames []MetricFrame, targetName string, collectionStartTime time.Time, printToStdout bool, printToFile bool, outputDir string) (outputFilename string, err error) {
	if !printToStdout && !printToFile {
		return
	}
	filename := outputDir + "/" + targetName + "_" + "metrics.json"
	for _, metricFrame := range metricFrames {
		// can't Marshal NaN or Inf values in JSON, so no need to set them to a specific value
		filteredMetricFrame := metricFrame
		filteredMetricFrame.Metrics = make([]Metric, 0, len(metricFrame.Metrics))
		filteredMetricFrame.Timestamp = float64(collectionStartTime.Unix() + int64(metricFrame.Timestamp))
		for _, metric := range metricFrame.Metrics {
			if math.IsNaN(metric.Value) || math.IsInf(metric.Value, 0) {
				filteredMetricFrame.Metrics = append(filteredMetricFrame.Metrics, Metric{Name: metric.Name, Value: -1})
			} else {
				filteredMetricFrame.Metrics = append(filteredMetricFrame.Metrics, metric)
			}
		}
		var jsonBytes []byte
		jsonBytes, err = json.Marshal(filteredMetricFrame)
		if err != nil {
			return
		}
		if printToStdout {
			fmt.Println(string(jsonBytes))
		}
		if printToFile {
			var file *os.File
			file, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return
			}
			defer file.Close()
			_, err = file.WriteString(string(jsonBytes) + "\n")
			if err != nil {
				return
			}
		}
	}
	outputFilename = filename
	return
}

func printMetricsCSV(metricFrames []MetricFrame, frameCount int, targetName string, collectionStartTime time.Time, printToStdout bool, printToFile bool, outputDir string) (outputFilename string, err error) {
	if !printToStdout && !printToFile {
		return
	}
	filename := outputDir + "/" + targetName + "_" + "metrics.csv"
	var file *os.File
	if printToFile {
		// open file for writing/appending
		file, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer file.Close()
	}
	for idx, metricFrame := range metricFrames {
		if idx == 0 && frameCount == 1 {
			contextHeaders := "TS,SKT,CPU,CID,"
			if printToStdout {
				fmt.Print(contextHeaders)
			}
			if printToFile {
				_, err = file.WriteString(contextHeaders)
				if err != nil {
					return
				}
			}
			names := make([]string, 0, len(metricFrame.Metrics))
			for _, metric := range metricFrame.Metrics {
				names = append(names, metric.Name)
			}
			metricNames := strings.Join(names, ",")
			if printToStdout {
				fmt.Println(metricNames)
			}
			if printToFile {
				_, err = file.WriteString(metricNames + "\n")
				if err != nil {
					return
				}
			}
		}
		metricContext := fmt.Sprintf("%d,%s,%s,%s,", collectionStartTime.Unix()+int64(metricFrame.Timestamp), metricFrame.Socket, metricFrame.CPU, metricFrame.Cgroup)
		values := make([]string, 0, len(metricFrame.Metrics))
		for _, metric := range metricFrame.Metrics {
			values = append(values, strconv.FormatFloat(metric.Value, 'g', 8, 64))
		}
		metricValues := strings.ReplaceAll(strings.Join(values, ","), "NaN", "")
		if printToStdout {
			fmt.Println(metricContext + metricValues)
		}
		if printToFile {
			_, err = file.WriteString(metricContext + metricValues + "\n")
			if err != nil {
				return
			}
		}
	}
	outputFilename = filename
	return
}

func printMetricsWide(metricFrames []MetricFrame, frameCount int, targetName string, collectionStartTime time.Time, printToStdout bool, printToFile bool, outputDir string) (outputFilename string, err error) {
	if !printToStdout && !printToFile {
		return
	}
	filename := outputDir + "/" + targetName + "_" + "metrics_wide.txt"
	var file *os.File
	if printToFile {
		// open file for writing/appending
		file, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer file.Close()
	}
	for idx, metricFrame := range metricFrames {
		var names []string
		var values []float64
		for _, metric := range metricFrame.Metrics {
			names = append(names, metric.Name)
			values = append(values, metric.Value)
		}
		minColWidth := 6
		colSpacing := 3
		if idx == 0 && frameCount == 1 { // print headers
			header := "Timestamp    " // 10 + 3
			if metricFrame.PID != "" {
				header += "PID       "         // 7 + 3
				header += "Command           " // 15 + 3
			} else if metricFrame.Cgroup != "" {
				header += "CID       "
			}
			if metricFrame.CPU != "" {
				header += "CPU   " // 3 + 3
			} else if metricFrame.Socket != "" {
				header += "SKT   " // 3 + 3
			}
			for _, name := range names {
				extend := 0
				if len(name) < minColWidth {
					extend = minColWidth - len(name)
				}
				header += fmt.Sprintf("%s%*s%*s", name, extend, "", colSpacing, "")
			}
			if printToStdout {
				fmt.Println(header)
			}
			if printToFile {
				_, err = file.WriteString(header + "\n")
				if err != nil {
					return
				}
			}
		}
		// handle values
		TimestampColWidth := 10
		formattedTimestamp := fmt.Sprintf("%d", collectionStartTime.Unix()+int64(metricFrame.Timestamp))
		row := fmt.Sprintf("%s%*s%*s", formattedTimestamp, TimestampColWidth-len(formattedTimestamp), "", colSpacing, "")
		if metricFrame.PID != "" {
			PIDColWidth := 7
			commandColWidth := 15
			row += fmt.Sprintf("%s%*s%*s", metricFrame.PID, PIDColWidth-len(metricFrame.PID), "", colSpacing, "")
			var command string
			if len(metricFrame.Cmd) <= commandColWidth {
				command = metricFrame.Cmd
			} else {
				command = metricFrame.Cmd[:commandColWidth]
			}
			row += fmt.Sprintf("%s%*s%*s", command, commandColWidth-len(command), "", colSpacing, "")
		} else if metricFrame.Cgroup != "" {
			CIDColWidth := 7
			row += fmt.Sprintf("%s%*s%*s", metricFrame.Cgroup, CIDColWidth-len(metricFrame.Cgroup), "", colSpacing, "")
		}
		if metricFrame.CPU != "" {
			CPUColWidth := 3
			row += fmt.Sprintf("%s%*s%*s", metricFrame.CPU, CPUColWidth-len(metricFrame.CPU), "", colSpacing, "")
		} else if metricFrame.Socket != "" {
			SKTColWidth := 3
			row += fmt.Sprintf("%s%*s%*s", metricFrame.Socket, SKTColWidth-len(metricFrame.Socket), "", colSpacing, "")
		}
		// handle the metric values
		for i, value := range values {
			colWidth := max(len(names[i]), minColWidth)
			formattedVal := fmt.Sprintf("%.2f", value)
			row += fmt.Sprintf("%s%*s%*s", formattedVal, colWidth-len(formattedVal), "", colSpacing, "")
		}
		if printToStdout {
			fmt.Println(row)
		}
		if printToFile {
			_, err = file.WriteString(row + "\n")
			if err != nil {
				return
			}
		}
	}
	outputFilename = filename
	return
}

func printMetricsTxt(metricFrames []MetricFrame, targetName string, collectionStartTime time.Time, printToStdout bool, printToFile bool, outputDir string) (outputFilename string, err error) {
	if !printToStdout && !printToFile {
		return
	}
	var outputLines []string
	if len(metricFrames) > 0 && metricFrames[0].Socket != "" {
		outputLines = append(outputLines, "--------------------------------------------------------------------------------------")
		outputLines = append(outputLines, fmt.Sprintf("- Metrics captured at %s", collectionStartTime.Add(time.Second*time.Duration(int(metricFrames[0].Timestamp))).UTC()))
		outputLines = append(outputLines, "--------------------------------------------------------------------------------------")
		line := fmt.Sprintf("%-70s ", "metric")
		for i := range len(metricFrames) {
			line += fmt.Sprintf("%15s", fmt.Sprintf("skt %s val", metricFrames[i].Socket))
		}
		outputLines = append(outputLines, line)
		line = fmt.Sprintf("%-70s ", "------------------------")
		for range len(metricFrames) {
			line += fmt.Sprintf("%15s", "----------")
		}
		outputLines = append(outputLines, line)
		for i := range metricFrames[0].Metrics {
			line = fmt.Sprintf("%-70s ", metricFrames[0].Metrics[i].Name)
			for _, metricFrame := range metricFrames {
				line += fmt.Sprintf("%15s", strconv.FormatFloat(metricFrame.Metrics[i].Value, 'g', 4, 64))
			}
			outputLines = append(outputLines, line)
		}
	} else {
		for _, metricFrame := range metricFrames {
			outputLines = append(outputLines, "--------------------------------------------------------------------------------------")
			outputLines = append(outputLines, fmt.Sprintf("- Metrics captured at %s", collectionStartTime.Add(time.Second*time.Duration(int(metricFrame.Timestamp))).UTC()))
			if metricFrame.PID != "" {
				outputLines = append(outputLines, fmt.Sprintf("- PID: %s", metricFrame.PID))
				outputLines = append(outputLines, fmt.Sprintf("- CMD: %s", metricFrame.Cmd))
			} else if metricFrame.Cgroup != "" {
				outputLines = append(outputLines, fmt.Sprintf("- CID: %s", metricFrame.Cgroup))
			}
			if metricFrame.CPU != "" {
				outputLines = append(outputLines, fmt.Sprintf("- CPU: %s", metricFrame.CPU))
			} else if metricFrame.Socket != "" {
				outputLines = append(outputLines, fmt.Sprintf("- Socket: %s", metricFrame.Socket)) // TODO: remove this, it shouldn't happen
			}
			outputLines = append(outputLines, "--------------------------------------------------------------------------------------")
			outputLines = append(outputLines, fmt.Sprintf("%-70s %15s", "metric", "value"))
			outputLines = append(outputLines, fmt.Sprintf("%-70s %15s", "------------------------", "----------"))
			for _, metric := range metricFrame.Metrics {
				outputLines = append(outputLines, fmt.Sprintf("%-70s %15s", metric.Name, strconv.FormatFloat(metric.Value, 'g', 4, 64)))
			}
		}
	}
	if printToStdout {
		fmt.Println(strings.Join(outputLines, "\n"))
	}
	if printToFile {
		// open file for writing/appending
		var file *os.File
		file, err = os.OpenFile(outputDir+"/"+targetName+"_"+"metrics.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer file.Close()
		_, err = file.WriteString(strings.Join(outputLines, "\n") + "\n")
		if err != nil {
			return
		}
		outputFilename = file.Name()
	}
	return
}
