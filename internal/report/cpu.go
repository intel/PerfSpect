// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package report

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"perfspect/internal/cpus"
	"perfspect/internal/script"
	"perfspect/internal/util"
)

// UarchFromOutput returns the architecture of the CPU that matches family, model, stepping,
// capid4, and devices information from the output or an empty string, if no match is found.
func UarchFromOutput(outputs map[string]script.ScriptOutput) string {
	family := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU family:\s*(.+)$`)
	model := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model:\s*(.+)$`)
	stepping := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Stepping:\s*(.+)$`)
	capid4 := valFromRegexSubmatch(outputs[script.LspciBitsScriptName].Stdout, `^([0-9a-fA-F]+)`)
	devices := valFromRegexSubmatch(outputs[script.LspciDevicesScriptName].Stdout, `^([0-9]+)`)
	cpu, err := cpus.GetCPUExtended(family, model, stepping, capid4, devices)
	if err == nil {
		return cpu.MicroArchitecture
	}
	return ""
}

func hyperthreadingFromOutput(outputs map[string]script.ScriptOutput) string {
	family := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU family:\s*(.+)$`)
	model := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model:\s*(.+)$`)
	stepping := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Stepping:\s*(.+)$`)
	sockets := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)
	coresPerSocket := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(.+)$`)
	cpuCount := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU\(.*:\s*(.+?)$`)
	onlineCpus := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^On-line CPU\(s\) list:\s*(.+)$`)
	threadsPerCore := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Thread\(s\) per core:\s*(.+)$`)

	numCPUs, err := strconv.Atoi(cpuCount) // logical CPUs
	if err != nil {
		slog.Error("error parsing cpus from lscpu")
		return ""
	}
	onlineCpusList, err := util.SelectiveIntRangeToIntList(onlineCpus) // logical online CPUs
	numOnlineCpus := len(onlineCpusList)
	if err != nil {
		slog.Error("error parsing online cpus from lscpu")
		numOnlineCpus = 0 // set to 0 to indicate parsing failed, will use numCPUs instead
	}
	numThreadsPerCore, err := strconv.Atoi(threadsPerCore) // logical threads per core
	if err != nil {
		slog.Error("error parsing threads per core from lscpu")
		numThreadsPerCore = 0
	}
	numSockets, err := strconv.Atoi(sockets)
	if err != nil {
		slog.Error("error parsing sockets from lscpu")
		return ""
	}
	numCoresPerSocket, err := strconv.Atoi(coresPerSocket) // physical cores
	if err != nil {
		slog.Error("error parsing cores per sockets from lscpu")
		return ""
	}
	cpu, err := cpus.GetCPUExtended(family, model, stepping, "", "")
	if err != nil {
		return ""
	}
	if numOnlineCpus > 0 && numOnlineCpus < numCPUs {
		// if online CPUs list is available, use it to determine the number of CPUs
		// supersedes lscpu output of numCPUs which counts CPUs on the system, not online CPUs
		numCPUs = numOnlineCpus
	}
	if cpu.LogicalThreadCount < 2 {
		return "N/A"
	} else if numThreadsPerCore == 1 {
		// if threads per core is 1, hyperthreading is disabled
		return "Disabled"
	} else if numThreadsPerCore >= 2 {
		// if threads per core is greater than or equal to 2, hyperthreading is enabled
		return "Enabled"
	} else if numCPUs > numCoresPerSocket*numSockets {
		// if the threads per core attribute is not available, we can still check if hyperthreading is enabled
		// by checking if the number of logical CPUs is greater than the number of physical cores
		return "Enabled"
	} else {
		return "Disabled"
	}
}

func numaCPUListFromOutput(outputs map[string]script.ScriptOutput) string {
	nodeCPUs := valsFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node[0-9] CPU\(.*:\s*(.+?)$`)
	return strings.Join(nodeCPUs, " :: ")
}

func ppinsFromOutput(outputs map[string]script.ScriptOutput) string {
	uniquePpins := []string{}
	for line := range strings.SplitSeq(outputs[script.PPINName].Stdout, "\n") {
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		ppin := strings.TrimSpace(parts[1])
		found := false
		for _, p := range uniquePpins {
			if string(p) == ppin {
				found = true
				break
			}
		}
		if !found && ppin != "" {
			uniquePpins = append(uniquePpins, ppin)
		}
	}
	return strings.Join(uniquePpins, ", ")
}

func channelsFromOutput(outputs map[string]script.ScriptOutput) string {
	family := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU family:\s*(.+)$`)
	model := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model:\s*(.+)$`)
	stepping := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Stepping:\s*(.+)$`)
	capid4 := valFromRegexSubmatch(outputs[script.LspciBitsScriptName].Stdout, `^([0-9a-fA-F]+)`)
	devices := valFromRegexSubmatch(outputs[script.LspciDevicesScriptName].Stdout, `^([0-9]+)`)
	cpu, err := cpus.GetCPUExtended(family, model, stepping, capid4, devices)
	if err != nil {
		slog.Error("error getting CPU from CPUdb", slog.String("error", err.Error()))
		return ""
	}
	return fmt.Sprintf("%d", cpu.MemoryChannelCount)
}

func turboEnabledFromOutput(outputs map[string]script.ScriptOutput) string {
	vendor := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Vendor ID:\s*(.+)$`)
	switch vendor {
	case cpus.IntelVendor:
		val := valFromRegexSubmatch(outputs[script.CpuidScriptName].Stdout, `^Intel Turbo Boost Technology\s*= (.+?)$`)
		if val == "true" {
			return "Enabled"
		}
		if val == "false" {
			return "Disabled"
		}
		return "" // unknown value
	case cpus.AMDVendor:
		val := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Frequency boost.*:\s*(.+?)$`)
		if val != "" {
			return val + " (AMD Frequency Boost)"
		}
	}
	return ""
}

func tdpFromOutput(outputs map[string]script.ScriptOutput) string {
	msrHex := strings.TrimSpace(outputs[script.PackagePowerLimitName].Stdout)
	msr, err := strconv.ParseInt(msrHex, 16, 0)
	if err != nil || msr == 0 {
		return ""
	}
	return fmt.Sprint(msr/8) + "W"
}

func chaCountFromOutput(outputs map[string]script.ScriptOutput) string {
	// output is the result of three rdmsr calls
	// - client cha count
	// - cha count
	// - spr cha count
	// stop when we find a non-zero value
	// note: rdmsr writes to stderr on error so we will likely have fewer than 3 lines in stdout
	for hexCount := range strings.SplitSeq(outputs[script.ChaCountScriptName].Stdout, "\n") {
		if hexCount != "" && hexCount != "0" {
			count, err := strconv.ParseInt(hexCount, 16, 64)
			if err == nil {
				return fmt.Sprintf("%d", count)
			}
		}
	}
	return ""
}

func numaBalancingFromOutput(outputs map[string]script.ScriptOutput) string {
	if strings.Contains(outputs[script.NumaBalancingScriptName].Stdout, "1") {
		return "Enabled"
	} else if strings.Contains(outputs[script.NumaBalancingScriptName].Stdout, "0") {
		return "Disabled"
	}
	return ""
}

func clusteringModeFromOutput(outputs map[string]script.ScriptOutput) string {
	uarch := UarchFromOutput(outputs)
	sockets := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)
	nodes := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node\(s\):\s*(.+)$`)
	if uarch == "" || sockets == "" || nodes == "" {
		return ""
	}
	socketCount, err := strconv.Atoi(sockets)
	if err != nil {
		slog.Error("failed to parse socket count", slog.String("error", err.Error()))
		return ""
	}
	nodeCount, err := strconv.Atoi(nodes)
	if err != nil {
		slog.Error("failed to parse node count", slog.String("error", err.Error()))
		return ""
	}
	if nodeCount == 0 || socketCount == 0 {
		slog.Error("node count or socket count is zero")
		return ""
	}
	nodesPerSocket := nodeCount / socketCount
	switch uarch {
	case "GNR_X1":
		return "All2All"
	case "GNR_X2":
		switch nodesPerSocket {
		case 1:
			return "UMA 4 (Quad)"
		case 2:
			return "SNC 2"
		}
	case "GNR_X3":
		switch nodesPerSocket {
		case 1:
			return "UMA 6 (Hex)"
		case 3:
			return "SNC 3"
		}
	case "SRF_SP":
		return "UMA 2 (Hemi)"
	case "SRF_AP":
		switch nodesPerSocket {
		case 1:
			return "UMA 4 (Quad)"
		case 2:
			return "SNC 2"
		}
	case "CWF":
		switch nodesPerSocket {
		case 1:
			return "UMA 6 (Hex)"
		case 3:
			return "SNC 3"
		}
	}
	return ""
}
