package report

import (
	"fmt"
	"perfspect/internal/script"
	"regexp"
	"strings"
)

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

func acceleratorNames() []string {
	var names []string
	for _, accel := range acceleratorDefinitions {
		names = append(names, accel.Name)
	}
	return names
}

func acceleratorCountsFromOutput(outputs map[string]script.ScriptOutput) []string {
	var counts []string
	lshw := outputs[script.LshwScriptName].Stdout
	for _, accel := range acceleratorDefinitions {
		regex := fmt.Sprintf("%s:%s", accel.MfgID, accel.DevID)
		re := regexp.MustCompile(regex)
		count := len(re.FindAllString(lshw, -1))
		counts = append(counts, fmt.Sprintf("%d", count))
	}
	return counts
}

func acceleratorWorkQueuesFromOutput(outputs map[string]script.ScriptOutput) []string {
	var queues []string
	for _, accel := range acceleratorDefinitions {
		if accel.Name == "IAA" || accel.Name == "DSA" {
			var scriptName string
			if accel.Name == "IAA" {
				scriptName = script.IaaDevicesScriptName
			} else {
				scriptName = script.DsaDevicesScriptName
			}
			devices := outputs[scriptName].Stdout
			lines := strings.Split(devices, "\n")
			// get non-empty lines
			var nonEmptyLines []string
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					nonEmptyLines = append(nonEmptyLines, line)
				}
			}
			if len(nonEmptyLines) == 0 {
				queues = append(queues, "None")
			} else {
				queues = append(queues, strings.Join(nonEmptyLines, ", "))
			}
		} else {
			queues = append(queues, "N/A")
		}
	}
	return queues
}

func acceleratorFullNamesFromYaml() []string {
	var fullNames []string
	for _, accel := range acceleratorDefinitions {
		fullNames = append(fullNames, accel.FullName)
	}
	return fullNames
}

func acceleratorDescriptionsFromYaml() []string {
	var descriptions []string
	for _, accel := range acceleratorDefinitions {
		descriptions = append(descriptions, accel.Description)
	}
	return descriptions
}

func acceleratorSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	var summary []string
	accelerators := acceleratorNames()
	counts := acceleratorCountsFromOutput(outputs)
	for i, name := range accelerators {
		if strings.Contains(name, "chipset") { // skip "QAT (on chipset)" in this table
			continue
		} else if strings.Contains(name, "CPU") { // rename "QAT (on CPU) to simply "QAT"
			name = "QAT"
		}
		summary = append(summary, fmt.Sprintf("%s %s [0]", name, counts[i]))
	}
	return strings.Join(summary, ", ")
}
