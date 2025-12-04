// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package report

import (
	"fmt"
	"strings"
	"time"

	"perfspect/internal/cpus"
	"perfspect/internal/script"
)

func operatingSystemFromOutput(outputs map[string]script.ScriptOutput) string {
	os := valFromRegexSubmatch(outputs[script.EtcReleaseScriptName].Stdout, `^PRETTY_NAME=\"(.+?)\"`)
	centos := valFromRegexSubmatch(outputs[script.EtcReleaseScriptName].Stdout, `^(CentOS Linux release .*)`)
	if centos != "" {
		os = centos
	}
	return os
}

func systemSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	// BASELINE: 1-node, 2x Intel® Xeon® <SKU, processor>, xx cores, 100W TDP, HT On/Off?, Turbo On/Off?, Total Memory xxx GB (xx slots/ xx GB/ xxxx MHz [run @ xxxx MHz] ), <BIOS version>, <ucode version>, <OS Version>, <kernel version>. Test by Intel as of <mm/dd/yy>.
	template := "1-node, %s, %sx %s, %s cores, %s TDP, %s %s, %s %s, Total Memory %s, BIOS %s, microcode %s, %s, %s, %s, %s. Test by Intel as of %s."
	var systemType, socketCount, cpuModel, coreCount, tdp, htLabel, htOnOff, turboLabel, turboOnOff, installedMem, biosVersion, uCodeVersion, nics, disks, operatingSystem, kernelVersion, date string

	// system type
	systemType = valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Manufacturer:\s*(.+?)$`) + " " + valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "1", `^Product Name:\s*(.+?)$`)
	// socket count
	socketCount = valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(\d+)$`)
	// CPU model
	cpuModel = valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model name:\s*(.+?)$`)
	// core count
	coreCount = valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(\d+)$`)
	// TDP
	tdp = tdpFromOutput(outputs)
	if tdp == "" {
		tdp = "?"
	}
	vendor := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Vendor ID:\s*(.+)$`)
	// hyperthreading
	htLabel = "HT"
	if vendor == cpus.AMDVendor {
		htLabel = "SMT"
	}
	htOnOff = hyperthreadingFromOutput(outputs)
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
	biosVersion = valFromRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, `^Version:\s*(.+?)$`)
	// microcode
	uCodeVersion = valFromRegexSubmatch(outputs[script.ProcCpuinfoScriptName].Stdout, `^microcode.*:\s*(.+?)$`)
	// NICs
	nics = nicSummaryFromOutput(outputs)
	// disks
	disks = diskSummaryFromOutput(outputs)
	// OS
	operatingSystem = operatingSystemFromOutput(outputs)
	// kernel
	kernelVersion = valFromRegexSubmatch(outputs[script.UnameScriptName].Stdout, `^Linux \S+ (\S+)`)
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
