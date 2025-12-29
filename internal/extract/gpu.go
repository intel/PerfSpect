// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"perfspect/internal/script"
)

// Intel Discrete GPUs (sorted by devid)
// references:
//   https://pci-ids.ucw.cz/read/PC/8086
//   https://dgpu-docs.intel.com/devices/hardware-table.html
//
//   The devid field will be interpreted as a regular expression.

// GPUDefinition represents an Intel GPU device definition.
type GPUDefinition struct {
	Model string
	MfgID string
	DevID string
}

// GPUDefinitions contains all known Intel GPU definitions.
var GPUDefinitions = []GPUDefinition{
	{
		Model: "ATS-P",
		MfgID: "8086",
		DevID: "201",
	},
	{
		Model: "Ponte Vecchio 2T",
		MfgID: "8086",
		DevID: "BD0",
	},
	{
		Model: "Ponte Vecchio 1T",
		MfgID: "8086",
		DevID: "BD5",
	},
	{
		Model: "Intel® Iris® Xe MAX Graphics (DG1)",
		MfgID: "8086",
		DevID: "4905",
	},
	{
		Model: "Intel® Iris® Xe Pod (DG1)",
		MfgID: "8086",
		DevID: "4906",
	},
	{
		Model: "SG1",
		MfgID: "8086",
		DevID: "4907",
	},
	{
		Model: "Intel® Iris® Xe Graphics (DG1)",
		MfgID: "8086",
		DevID: "4908",
	},
	{
		Model: "Intel® Iris® Xe MAX 100 (DG1)",
		MfgID: "8086",
		DevID: "4909",
	},
	{
		Model: "DG2",
		MfgID: "8086",
		DevID: "(4F80|4F81|4F82)",
	},
	{
		Model: "Intel® Arc ™ A770M Graphics",
		MfgID: "8086",
		DevID: "5690",
	},
	{
		Model: "Intel® Arc ™ A730M Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "5691",
	},
	{
		Model: "Intel® Arc ™ A550M Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "5692",
	},
	{
		Model: "Intel® Arc ™ A370M Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "5693",
	},
	{
		Model: "Intel® Arc ™ A350M Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "5694",
	},
	{
		Model: "Intel® Arc ™ A770 Graphics",
		MfgID: "8086",
		DevID: "56A0",
	},
	{
		Model: "Intel® Arc ™ A750 Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "56A1",
	},
	{
		Model: "Intel® Arc ™ A380 Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "56A5",
	},
	{
		Model: "Intel® Arc ™ A310 Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "56A6",
	},
	{
		Model: "Intel® Data Center GPU Flex 170",
		MfgID: "8086",
		DevID: "56C0",
	},
	{
		Model: "Intel® Data Center GPU Flex 140",
		MfgID: "8086",
		DevID: "56C1",
	},
	{
		Model: "Intel® Data Center GPU Flex 170V",
		MfgID: "8086",
		DevID: "56C2",
	},
}

// GPU represents a graphics processing unit found in the system.
type GPU struct {
	Manufacturer string
	Model        string
	PCIID        string
}

// GPUInfoFromOutput returns GPU information from lshw output.
func GPUInfoFromOutput(outputs map[string]script.ScriptOutput) []GPU {
	gpus := []GPU{}
	gpusLshw := ValsArrayFromRegexSubmatch(outputs[script.LshwScriptName].Stdout, `^pci.*?\s+display\s+(\w+).*?\s+\[(\w+):(\w+)]$`)
	idxMfgName := 0
	idxMfgID := 1
	idxDevID := 2
	for _, gpu := range gpusLshw {
		// Find GPU in GPU defs, note the model
		var model string
		for _, intelGPU := range GPUDefinitions {
			if gpu[idxMfgID] == intelGPU.MfgID {
				model = intelGPU.Model
				break
			}
			re := regexp.MustCompile(intelGPU.DevID)
			if re.FindString(gpu[idxDevID]) != "" {
				model = intelGPU.Model
				break
			}
		}
		if model == "" {
			if gpu[idxMfgID] == "8086" {
				model = "Unknown Intel"
			} else {
				model = "Unknown"
			}
		}
		gpus = append(gpus, GPU{Manufacturer: gpu[idxMfgName], Model: model, PCIID: gpu[idxMfgID] + ":" + gpu[idxDevID]})
	}
	return gpus
}

// Gaudi represents an Intel Gaudi accelerator.
type Gaudi struct {
	ModuleID          string
	Microarchitecture string
	SerialNumber      string
	BusID             string
	DriverVersion     string
	EROM              string
	CPLD              string
	SPI               string
	NUMA              string
}

// GaudiInfoFromOutput returns Gaudi accelerator information from script output.
func GaudiInfoFromOutput(outputs map[string]script.ScriptOutput) []Gaudi {
	gaudis := []Gaudi{}
	for i, line := range strings.Split(outputs[script.GaudiInfoScriptName].Stdout, "\n") {
		if line == "" || i == 0 { // skip blank lines and header
			continue
		}
		fields := strings.Split(line, ", ")
		if len(fields) != 4 {
			slog.Error("unexpected number of fields in gaudi info output", slog.String("line", line))
			continue
		}
		gaudis = append(gaudis, Gaudi{ModuleID: fields[0], SerialNumber: fields[1], BusID: fields[2], DriverVersion: fields[3]})
	}
	// sort the gaudis by module ID
	sort.Slice(gaudis, func(i, j int) bool {
		return gaudis[i].ModuleID < gaudis[j].ModuleID
	})
	// set microarchitecture (assumes same arch for all gaudi devices)
	for i := range gaudis {
		gaudis[i].Microarchitecture = strings.TrimSpace(outputs[script.GaudiArchitectureScriptName].Stdout)
	}
	// get NUMA affinity
	numaAffinities := ValsArrayFromRegexSubmatch(outputs[script.GaudiNumaScriptName].Stdout, `^(\d+)\s+(\d+)\s+$`)
	if len(numaAffinities) != len(gaudis) {
		slog.Error("number of gaudis in gaudi info and numa output do not match", slog.Int("gaudis", len(gaudis)), slog.Int("numaAffinities", len(numaAffinities)))
		return nil
	}
	for i, numaAffinity := range numaAffinities {
		gaudis[i].NUMA = numaAffinity[1]
	}
	// get firmware versions
	reDevice := regexp.MustCompile(`^\[(\d+)] AIP \(accel\d+\) (.*)$`)
	reErom := regexp.MustCompile(`\s+erom$`)
	reCpld := regexp.MustCompile(`\s+cpld$`)
	rePreboot := regexp.MustCompile(`\s+preboot$`)
	reComponent := regexp.MustCompile(`^\s+component\s+:\s+hl-gaudi\d-(.*)-sec-\d+`)
	reCpldComponent := regexp.MustCompile(`^\s+component\s+:\s+(0x[0-9a-fA-F]+\.[0-9a-fA-F]+)$`)
	deviceIdx := -1
	state := -1
	for line := range strings.SplitSeq(outputs[script.GaudiFirmwareScriptName].Stdout, "\n") {
		if line == "" {
			continue
		}
		match := reDevice.FindStringSubmatch(line)
		if match != nil {
			var err error
			deviceIdx, err = strconv.Atoi(match[1])
			if err != nil {
				slog.Error("failed to parse device index", slog.String("deviceIdx", match[1]))
				return nil
			}
			if deviceIdx >= len(gaudis) {
				slog.Error("device index out of range", slog.Int("deviceIdx", deviceIdx), slog.Int("gaudis", len(gaudis)))
				return nil
			}
			continue
		}
		if deviceIdx == -1 {
			continue
		}
		if reErom.FindString(line) != "" {
			state = 0
			continue
		}
		if reCpld.FindString(line) != "" {
			state = 1
			continue
		}
		if rePreboot.FindString(line) != "" {
			state = 2
			continue
		}
		if state != -1 {
			switch state {
			case 0:
				match := reComponent.FindStringSubmatch(line)
				if match != nil {
					gaudis[deviceIdx].EROM = match[1]
				}
			case 1:
				match := reCpldComponent.FindStringSubmatch(line)
				if match != nil {
					gaudis[deviceIdx].CPLD = match[1]
				}
			case 2:
				match := reComponent.FindStringSubmatch(line)
				if match != nil {
					gaudis[deviceIdx].SPI = match[1]
				}
			}
			state = -1
		}
	}
	return gaudis
}

// GetPCIDevices returns all PCI Devices of specified class from lspci output.
func GetPCIDevices(class string, outputs map[string]script.ScriptOutput) (devices []map[string]string) {
	device := make(map[string]string)
	re := regexp.MustCompile(`^(\w+):\s+(.*)$`)
	for line := range strings.SplitSeq(outputs[script.LspciVmmScriptName].Stdout, "\n") {
		if line == "" { // end of device
			if devClass, ok := device["Class"]; ok {
				if devClass == class {
					devices = append(devices, device)
				}
			}
			device = make(map[string]string)
			continue
		}
		match := re.FindStringSubmatch(line)
		if len(match) > 0 {
			key := match[1]
			value := match[2]
			device[key] = value
		}
	}
	return
}
