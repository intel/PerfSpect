package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// metadata_aarch.go contains ARM/aarch64 metadata collection logic.

import (
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"perfspect/internal/common"
	"perfspect/internal/cpus"
	"perfspect/internal/progress"
	"perfspect/internal/script"
	"perfspect/internal/target"
)

// ARMMetadataCollector handles ARM metadata collection.
type ARMMetadataCollector struct{}

// CollectMetadata gathers all metadata for ARM/aarch64 systems.
func (c *ARMMetadataCollector) CollectMetadata(t target.Target, noRoot bool, noSystemSummary bool, localTempDir string, statusUpdate progress.MultiSpinnerUpdateFunc) (Metadata, error) {
	var metadata Metadata

	// Hostname
	metadata.Hostname = t.GetName()

	// CPU Info (from /proc/cpuinfo)
	cpuInfo, err := getCPUInfo(t)
	if err != nil || len(cpuInfo) < 1 {
		return Metadata{}, fmt.Errorf("failed to read cpu info: %v", err)
	}

	// lscpu output will be used for several metadata fields
	lscpu, err := getLscpu(t)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get lscpu output: %v", err)
	}

	// Vendor
	metadata.Vendor, err = parseLscpuStringField(lscpu, `^Vendor ID:\s*(.+)$`)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to parse vendor ID: %v", err)
	}

	// Model Name
	metadata.ModelName, err = parseLscpuStringField(lscpu, `^Model name:\s*(.+)$`)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to parse model name: %v", err)
	}

	// Sockets
	metadata.SocketCount, err = parseLscpuIntField(lscpu, `^Socket\(s\):\s*(.+)$`)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to parse socket count: %v", err)
	}

	// Cores per socket
	metadata.CoresPerSocket, err = parseLscpuIntField(lscpu, `^Core\(s\) per socket:\s*(.+)$`)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to parse cores per socket: %v", err)
	}

	// Threads per core
	metadata.ThreadsPerCore, err = parseLscpuIntField(lscpu, `^Thread\(s\) per core:\s*(.+)$`)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to parse threads per core: %v", err)
	}

	// CPUSocketMap - create a map of CPU to socket ID
	metadata.CPUSocketMap, err = getCPUSocketMapFromSysfs(t, metadata.SocketCount, metadata.CoresPerSocket, metadata.ThreadsPerCore, localTempDir)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to build CPU socket map: %v", err)
	}

	// Microarchitecture
	metadata.Microarchitecture, err = common.GetTargetMicroArchitecture(t, localTempDir, noRoot)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get ARM microarchitecture: %v", err)
	}

	// Number of General Purpose Counters
	metadata.NumGeneralPurposeCounters, err = getNumGPCountersARM(t, localTempDir, noRoot)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get number of general purpose counters: %v", err)
	}

	// Run metadata scripts concurrently
	metadataScripts, err := getMetadataScripts(noRoot, noSystemSummary, metadata.NumGeneralPurposeCounters)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get metadata scripts: %v", err)
	}

	scriptOutputs, err := common.RunScripts(t, metadataScripts, true, localTempDir, nil, "", noRoot) // nosemgrep
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to run metadata scripts: %v", err)
	}

	// System Summary Values
	if !noSystemSummary {
		if metadata.SystemSummaryFields, err = getSystemSummary(scriptOutputs); err != nil {
			return Metadata{}, fmt.Errorf("failed to get system summary: %w", err)
		}
	} else {
		metadata.SystemSummaryFields = [][]string{{"", "System Summary Not Available"}}
	}

	// Architecture
	if metadata.Architecture, err = getArchitecture(scriptOutputs); err != nil {
		return Metadata{}, fmt.Errorf("failed to retrieve architecture: %v", err)
	}

	// perf list
	if metadata.PerfSupportedEvents, err = getPerfSupportedEvents(scriptOutputs); err != nil {
		return Metadata{}, fmt.Errorf("failed to load perf list: %v", err)
	}

	// Kernel Version
	if metadata.KernelVersion, err = getKernelVersion(scriptOutputs); err != nil {
		return Metadata{}, fmt.Errorf("failed to retrieve kernel version: %v", err)
	}

	// ARM slots
	if metadata.ARMSlots, err = getARMSlots(scriptOutputs); err != nil {
		return Metadata{}, fmt.Errorf("failed to retrieve ARM slots: %v", err)
	}
	if metadata.ARMSlots == 0 { // can't retrieve slots on EC2 VMs
		metadata.ARMSlots, err = getARMSlotsByArchitecture(metadata.Microarchitecture)
		if err != nil {
			return Metadata{}, fmt.Errorf("failed to determine ARM slots by architecture: %v", err)
		}
	}

	// ARM CPUID
	if metadata.ARMCPUID, err = getARMCPUID(scriptOutputs); err != nil {
		return Metadata{}, fmt.Errorf("failed to retrieve ARM CPUID: %v", err)
	}

	// instructions support
	var output string
	if metadata.SupportsInstructions, output, err = getSupportsEvent("instructions", scriptOutputs); err != nil {
		slog.Warn("failed to determine if instructions event is supported, assuming not supported", slog.String("error", err.Error()))
	} else if !metadata.SupportsInstructions {
		slog.Warn("instructions event not supported", slog.String("output", output))
	}

	return metadata, nil
}

// --- ARM-specific helper functions ---

// getNumGPCountersARM returns the number of general purpose counters on ARM systems.
// Copyright 2025 Google LLC.
// SPDX-License-Identifier: BSD-3-Clause
// Contributed by Edwin Chiu
func getNumGPCountersARM(t target.Target, localTempDir string, noRoot bool) (numGPCounters int, err error) {
	getScript := script.ScriptDefinition{
		Name:           "get pmu driver version line",
		ScriptTemplate: "dmesg | grep -i \"PMU Driver\"",
		Superuser:      !noRoot,
		Architectures:  []string{cpus.ARMArchitecture},
	}
	scriptOutput, err := common.RunScript(t, getScript, localTempDir, noRoot)
	if err != nil {
		err = fmt.Errorf("failed to run pmu driver version script: %v", err)
		return
	}
	lines := strings.Split(strings.TrimSpace(scriptOutput.Stdout), "\n")
	if len(lines) == 0 {
		err = fmt.Errorf("no output from pmu driver version script")
		return
	}
	stdout := lines[0]
	// examples:
	//   [    1.339550] hw perfevents: enabled with armv8_pmuv3_0 PMU driver, 5 counters available
	//   [    3.663956] hw perfevents: enabled with armv8_pmuv3_0 PMU driver, 6 (0,8000001f) counters available
	// regex to match both "5 counters available" and "6 (0,8000001f) counters available"
	counterRegex := regexp.MustCompile(`(\d+)\s*(?:\([^)]+\))?\s+counters available`)
	matches := counterRegex.FindStringSubmatch(stdout)
	if len(matches) > 1 {
		numberStr := matches[1]
		numGPCounters, err = strconv.Atoi(numberStr)
		if err != nil {
			err = fmt.Errorf("error converting string to int: %v", err)
			return
		}
		numGPCounters-- // for ARM, there is a fixed counter for cycles and the driver includes it
		slog.Debug("getNumGPCountersArm", slog.Int("numGPCounters", numGPCounters))
	} else {
		err = fmt.Errorf("no match for number of counters on line: %s", stdout)
		return
	}
	return
}

// getARMSlots returns the number of ARM slots available.
// Copyright 2025 Google LLC.
// SPDX-License-Identifier: BSD-3-Clause
// Contributed by Edwin Chiu
func getARMSlots(scriptOutputs map[string]script.ScriptOutput) (slots int, err error) {
	if scriptOutputs[scriptARMSlots].Exitcode != 0 {
		err = fmt.Errorf("failed to retrieve ARM slots: %s", scriptOutputs[scriptARMSlots].Stderr)
		return
	}
	hexString := strings.TrimSpace(string(scriptOutputs[scriptARMSlots].Stdout))
	hexString = strings.TrimPrefix(hexString, "0x")
	parsedValue, err := strconv.ParseInt(hexString, 16, 64)
	if err != nil {
		err = fmt.Errorf("failed to parse ARM slots value (%s): %w", hexString, err)
		return
	}
	if parsedValue <= math.MinInt32 || parsedValue > math.MaxInt32 {
		err = fmt.Errorf("parsed ARM slots value out of range: %d", parsedValue)
		return
	}
	slots = int(parsedValue)
	slog.Debug("Successfully read ARM slots value", slog.Int("slots", slots))
	return
}

// getARMSlotsByArchitecture returns the number of ARM slots based on the microarchitecture.
// Used as a fallback when we cannot read the slots from sysfs.
func getARMSlotsByArchitecture(uarch string) (slots int, err error) {
	switch uarch {
	case cpus.UarchGraviton4, cpus.UarchAxion:
		slots = 8
	case cpus.UarchGraviton2, cpus.UarchGraviton3:
		slots = 6
	case cpus.UarchAmpereOneAC03:
		slots = 6
	case cpus.UarchAmpereOneAC04, cpus.UarchAmpereOneAC04_1:
		slots = 10
	case cpus.UarchAltraFamily:
		slots = 6
	default:
		err = fmt.Errorf("unsupported ARM uarch: %s", uarch)
		return
	}
	return
}

// getARMCPUID retrieves the ARM CPUID from the script outputs.
// Script output will have a hex value like 0x00000000410fd4f1.
func getARMCPUID(scriptOutputs map[string]script.ScriptOutput) (cpuid string, err error) {
	output, ok := scriptOutputs[scriptARMCPUID]
	if !ok || output.Exitcode != 0 {
		err = fmt.Errorf("failed to retrieve ARM CPUID: %s", output.Stderr)
		return
	}
	cpuid = strings.TrimSpace(output.Stdout)
	return
}

// getCPUSocketMapFromSysfs builds a CPU to socket map by reading from sysfs.
// This handles multi-socket ARM systems correctly.
func getCPUSocketMapFromSysfs(t target.Target, socketCount, coresPerSocket, threadsPerCore int, localTempDir string) (map[int]int, error) {
	totalCPUs := socketCount * coresPerSocket * threadsPerCore
	cpuSocketMap := make(map[int]int, totalCPUs)

	// Read physical_package_id for each CPU from sysfs
	// Output format: "cpuN socketID" per line (e.g., "0 60", "10 60")
	// This avoids relying on glob order which is lexicographic, not numeric
	getScript := script.ScriptDefinition{
		Name:           "get cpu socket map",
		ScriptTemplate: `for cpu in /sys/devices/system/cpu/cpu[0-9]*; do cpunum="${cpu##*cpu}"; echo "$cpunum $(cat $cpu/topology/physical_package_id 2>/dev/null)"; done`,
		Superuser:      false,
		Architectures:  []string{cpus.ARMArchitecture},
	}
	scriptOutput, err := common.RunScript(t, getScript, localTempDir, false)
	if err != nil || scriptOutput.Exitcode != 0 {
		// Fallback: assume single socket if sysfs read fails
		slog.Debug("failed to read CPU topology from sysfs, assuming single socket", slog.String("stderr", scriptOutput.Stderr))
		for i := range totalCPUs {
			cpuSocketMap[i] = 0
		}
		return cpuSocketMap, nil
	}

	lines := strings.Split(strings.TrimSpace(scriptOutput.Stdout), "\n")

	// First pass: parse "cpuID socketID" pairs and collect unique socket IDs
	rawSocketMap := make(map[int]int, len(lines))
	uniqueSockets := make(map[int]struct{})
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		cpuID, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		socketID, err := strconv.Atoi(fields[1])
		if err != nil {
			socketID = 0
		}
		rawSocketMap[cpuID] = socketID
		uniqueSockets[socketID] = struct{}{}
	}

	// Build a mapping from raw socket IDs to normalized (0-indexed) IDs
	// This handles ARM platforms that report non-zero socket IDs (e.g., 60 instead of 0)
	sortedSockets := make([]int, 0, len(uniqueSockets))
	for s := range uniqueSockets {
		sortedSockets = append(sortedSockets, s)
	}
	sort.Ints(sortedSockets)

	socketNormalize := make(map[int]int, len(sortedSockets))
	for i, s := range sortedSockets {
		socketNormalize[s] = i
	}

	// Second pass: assign normalized socket IDs
	for cpuID, rawSocket := range rawSocketMap {
		cpuSocketMap[cpuID] = socketNormalize[rawSocket]
	}

	return cpuSocketMap, nil
}
