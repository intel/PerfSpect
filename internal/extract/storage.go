// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"perfspect/internal/script"
	"perfspect/internal/table"
)

// DiskInfo represents disk/storage device information.
type DiskInfo struct {
	Name             string
	Model            string
	Size             string
	MountPoint       string
	Type             string
	RequestQueueSize string
	MinIOSize        string
	FirmwareVersion  string
	PCIeAddress      string
	NUMANode         string
	LinkSpeed        string
	LinkWidth        string
	MaxLinkSpeed     string
	MaxLinkWidth     string
}

// DiskInfoFromOutput extracts disk information from script outputs.
func DiskInfoFromOutput(outputs map[string]script.ScriptOutput) []DiskInfo {
	diskInfos := []DiskInfo{}
	for i, line := range strings.Split(outputs[script.DiskInfoScriptName].Stdout, "\n") {
		// first line is the header
		if i == 0 {
			continue
		}
		if line == "" {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) != 14 {
			slog.Error("unexpected number of fields in disk info output", slog.String("line", line))
			return nil
		}
		// clean up the model name
		fields[1] = strings.TrimSpace(fields[1])
		// if we don't have a firmware version, try to get it from another source
		if fields[7] == "" {
			reFwRev := regexp.MustCompile(`FwRev=(\w+)`)
			reDev := regexp.MustCompile(fmt.Sprintf(`/dev/%s:`, fields[0]))
			devFound := false
			for line := range strings.SplitSeq(outputs[script.HdparmScriptName].Stdout, "\n") {
				if !devFound {
					if reDev.FindString(line) != "" {
						devFound = true
						continue
					}
				} else {
					match := reFwRev.FindStringSubmatch(line)
					if match != nil {
						fields[7] = match[1]
						break
					}
				}
			}
		}
		diskInfos = append(diskInfos, DiskInfo{fields[0], fields[1], fields[2], fields[3], fields[4], fields[5], fields[6], fields[7], fields[8], fields[9], fields[10], fields[11], fields[12], fields[13]})
	}
	return diskInfos
}

// DiskSummaryFromOutput returns a summary of installed disks.
func DiskSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	disks := DiskInfoFromOutput(outputs)
	if len(disks) == 0 {
		return "N/A"
	}
	type ModelSize struct {
		model string
		size  string
	}
	modelSizeCount := make(map[ModelSize]int)
	for _, disk := range disks {
		if disk.Model == "" {
			continue
		}
		modelSize := ModelSize{model: disk.Model, size: disk.Size}
		modelSizeCount[modelSize]++
	}
	var summary []string
	for modelSize, count := range modelSizeCount {
		summary = append(summary, fmt.Sprintf("%dx %s %s", count, modelSize.size, modelSize.model))
	}
	return strings.Join(summary, ", ")
}

// FilesystemFieldValuesFromOutput returns filesystem information as table fields.
func FilesystemFieldValuesFromOutput(outputs map[string]script.ScriptOutput) []table.Field {
	fieldValues := []table.Field{}
	reFindmnt := regexp.MustCompile(`(.*)\s(.*)\s(.*)\s(.*)`)
	for i, line := range strings.Split(outputs[script.DfScriptName].Stdout, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		// "Mounted On" gets split into two fields, rejoin
		if i == 0 && len(fields) >= 2 && fields[len(fields)-2] == "Mounted" && fields[len(fields)-1] == "on" {
			fields[len(fields)-2] = "Mounted on"
			fields = fields[:len(fields)-1]
			for _, field := range fields {
				fieldValues = append(fieldValues, table.Field{Name: field, Values: []string{}})
			}
			// add an additional field
			fieldValues = append(fieldValues, table.Field{Name: "Mount Options", Values: []string{}})
			continue
		}
		if len(fields) != len(fieldValues)-1 {
			slog.Error("unexpected number of fields in df output", slog.String("line", line))
			return nil
		}
		for i, field := range fields {
			fieldValues[i].Values = append(fieldValues[i].Values, field)
		}
		// get mount options for the current file system
		var options string
		for i, line := range strings.Split(outputs[script.FindMntScriptName].Stdout, "\n") {
			if i == 0 {
				continue
			}
			match := reFindmnt.FindStringSubmatch(line)
			if match != nil && len(fields) > 5 {
				target := match[1]
				source := match[2]
				if fields[0] == source && fields[5] == target {
					options = match[4]
					break
				}
			}
		}
		fieldValues[len(fieldValues)-1].Values = append(fieldValues[len(fieldValues)-1].Values, options)
	}
	return fieldValues
}
