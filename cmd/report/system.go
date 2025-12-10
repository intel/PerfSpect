// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package report

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"perfspect/internal/common"
	"perfspect/internal/cpus"
	"perfspect/internal/script"
	"perfspect/internal/table"
)

func systemSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	// BASELINE: 1-node, 2x Intel® Xeon® <SKU, processor>, xx cores, 100W TDP, HT On/Off?, Turbo On/Off?, Total Memory xxx GB (xx slots/ xx GB/ xxxx MHz [run @ xxxx MHz] ), <BIOS version>, <ucode version>, <OS Version>, <kernel version>. Test by Intel as of <mm/dd/yy>.
	template := "1-node, %s, %sx %s, %s cores, %s TDP, %s %s, %s %s, Total Memory %s, BIOS %s, microcode %s, %s, %s, %s, %s. Test by Intel as of %s."
	var systemType, socketCount, cpuModel, coreCount, tdp, htLabel, htOnOff, turboLabel, turboOnOff, installedMem, biosVersion, uCodeVersion, nics, disks, operatingSystem, kernelVersion, date string

	// system type
	systemType = common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Manufacturer:\s*(.+?)$`) + " " + common.ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Product Name:\s*(.+?)$`)
	// socket count
	socketCount = common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(\d+)$`)
	// CPU model
	cpuModel = common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model name:\s*(.+?)$`)
	// core count
	coreCount = common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(\d+)$`)
	// TDP
	tdp = common.TDPFromOutput(outputs)
	if tdp == "" {
		tdp = "?"
	}
	vendor := common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Vendor ID:\s*(.+)$`)
	// hyperthreading
	htLabel = "HT"
	if vendor == cpus.AMDVendor {
		htLabel = "SMT"
	}
	htOnOff = common.HyperthreadingFromOutput(outputs)
	switch htOnOff {
	case "Enabled":
		htOnOff = "On"
	case "Disabled":
		htOnOff = "Off"
	case "N/A":
		htOnOff = "N/A"
	default:
		htOnOff = "?"
	}
	// turbo
	turboLabel = "Turbo"
	if vendor == cpus.AMDVendor {
		turboLabel = "Boost"
	}
	turboOnOff = turboEnabledFromOutput(outputs)
	if strings.Contains(strings.ToLower(turboOnOff), "enabled") {
		turboOnOff = "On"
	} else if strings.Contains(strings.ToLower(turboOnOff), "disabled") {
		turboOnOff = "Off"
	} else {
		turboOnOff = "?"
	}
	// memory
	installedMem = installedMemoryFromOutput(outputs)
	// BIOS
	biosVersion = common.ValFromRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, `^Version:\s*(.+?)$`)
	// microcode
	uCodeVersion = common.ValFromRegexSubmatch(outputs[script.ProcCpuinfoScriptName].Stdout, `^microcode.*:\s*(.+?)$`)
	// NICs
	nics = common.NICSummaryFromOutput(outputs)
	// disks
	disks = common.DiskSummaryFromOutput(outputs)
	// OS
	operatingSystem = common.OperatingSystemFromOutput(outputs)
	// kernel
	kernelVersion = common.ValFromRegexSubmatch(outputs[script.UnameScriptName].Stdout, `^Linux \S+ (\S+)`)
	// date
	date = strings.TrimSpace(outputs[script.DateScriptName].Stdout)
	// parse date so that we can format it
	parsedTime, err := time.Parse("Mon Jan 2 15:04:05 MST 2006", date) // without AM/PM
	if err != nil {
		parsedTime, err = time.Parse("Mon Jan 2 15:04:05 AM MST 2006", date) // with AM/PM
	}
	if err == nil {
		date = parsedTime.Format("January 2 2006")
	}

	// put it all together
	return fmt.Sprintf(template, systemType, socketCount, cpuModel, coreCount, tdp, htLabel, htOnOff, turboLabel, turboOnOff, installedMem, biosVersion, uCodeVersion, nics, disks, operatingSystem, kernelVersion, date)
}

func filesystemFieldValuesFromOutput(outputs map[string]script.ScriptOutput) []table.Field {
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
			if match != nil && len(match) > 4 && len(fields) > 5 {
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
