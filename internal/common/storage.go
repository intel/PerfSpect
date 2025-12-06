// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package common

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"perfspect/internal/script"
)

type diskInfo struct {
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

func DiskInfoFromOutput(outputs map[string]script.ScriptOutput) []diskInfo {
	diskInfos := []diskInfo{}
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
		diskInfos = append(diskInfos, diskInfo{fields[0], fields[1], fields[2], fields[3], fields[4], fields[5], fields[6], fields[7], fields[8], fields[9], fields[10], fields[11], fields[12], fields[13]})
	}
	return diskInfos
}

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
