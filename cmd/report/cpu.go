// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package report

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"perfspect/internal/common"
	"perfspect/internal/cpus"
	"perfspect/internal/script"
)

func numaCPUListFromOutput(outputs map[string]script.ScriptOutput) string {
	nodeCPUs := common.ValsFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node[0-9] CPU\(.*:\s*(.+?)$`)
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
	family := common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU family:\s*(.+)$`)
	model := common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model:\s*(.+)$`)
	stepping := common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Stepping:\s*(.+)$`)
	capid4 := common.ValFromRegexSubmatch(outputs[script.LspciBitsScriptName].Stdout, `^([0-9a-fA-F]+)`)
	devices := common.ValFromRegexSubmatch(outputs[script.LspciDevicesScriptName].Stdout, `^([0-9]+)`)
	implementer := strings.TrimSpace(outputs[script.ArmImplementerScriptName].Stdout)
	part := strings.TrimSpace(outputs[script.ArmPartScriptName].Stdout)
	dmidecodePart := strings.TrimSpace(outputs[script.ArmDmidecodePartScriptName].Stdout)
	cpu, err := cpus.GetCPU(cpus.NewCPUIdentifier(family, model, stepping, capid4, devices, implementer, part, dmidecodePart, ""))
	if err != nil {
		slog.Error("error getting CPU characteristics", slog.String("error", err.Error()))
		return ""
	}
	return fmt.Sprintf("%d", cpu.MemoryChannelCount)
}

func turboEnabledFromOutput(outputs map[string]script.ScriptOutput) string {
	vendor := common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Vendor ID:\s*(.+)$`)
	switch vendor {
	case cpus.IntelVendor:
		val := common.ValFromRegexSubmatch(outputs[script.CpuidScriptName].Stdout, `^Intel Turbo Boost Technology\s*= (.+?)$`)
		if val == "true" {
			return "Enabled"
		}
		if val == "false" {
			return "Disabled"
		}
		return "" // unknown value
	case cpus.AMDVendor:
		val := common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Frequency boost.*:\s*(.+?)$`)
		if val != "" {
			return val + " (AMD Frequency Boost)"
		}
	}
	return ""
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
	uarch := common.UarchFromOutput(outputs)
	sockets := common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)
	nodes := common.ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node\(s\):\s*(.+)$`)
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
