package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// metadata_x86.go contains x86_64 (Intel/AMD) metadata collection logic.

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"perfspect/internal/common"
	"perfspect/internal/cpus"
	"perfspect/internal/progress"
	"perfspect/internal/script"
	"perfspect/internal/target"
)

// X86MetadataCollector handles Intel/AMD x86_64 metadata collection.
type X86MetadataCollector struct{}

// CollectMetadata gathers all metadata for x86_64 systems.
func (c *X86MetadataCollector) CollectMetadata(t target.Target, noRoot bool, noSystemSummary bool, localTempDir string, statusUpdate progress.MultiSpinnerUpdateFunc) (Metadata, error) {
	var metadata Metadata
	var err error

	// Hostname
	metadata.Hostname = t.GetName()

	// CPU Info (from /proc/cpuinfo)
	cpuInfo, err := getCPUInfo(t)
	if err != nil || len(cpuInfo) < 1 {
		return Metadata{}, fmt.Errorf("failed to read cpu info: %v", err)
	}

	// Core Count (per socket)
	cpuCoresStr, ok := cpuInfo[0]["cpu cores"]
	if !ok {
		return Metadata{}, fmt.Errorf("'cpu cores' field not found in /proc/cpuinfo")
	}
	metadata.CoresPerSocket, err = strconv.Atoi(cpuCoresStr)
	if err != nil || metadata.CoresPerSocket == 0 {
		return Metadata{}, fmt.Errorf("failed to retrieve cores per socket: %v", err)
	}

	// Socket Count
	physicalIDStr, ok := cpuInfo[len(cpuInfo)-1]["physical id"]
	if !ok {
		return Metadata{}, fmt.Errorf("'physical id' field not found in /proc/cpuinfo")
	}
	var maxPhysicalID int
	if maxPhysicalID, err = strconv.Atoi(physicalIDStr); err != nil {
		return Metadata{}, fmt.Errorf("failed to retrieve max physical id: %v", err)
	}
	metadata.SocketCount = maxPhysicalID + 1

	// Hyperthreading - threads per core
	siblings, hasSiblings := cpuInfo[0]["siblings"]
	cpuCores, hasCpuCores := cpuInfo[0]["cpu cores"]
	if hasSiblings && hasCpuCores && siblings != cpuCores {
		metadata.ThreadsPerCore = 2
	} else {
		metadata.ThreadsPerCore = 1
	}

	// CPUSocketMap
	metadata.CPUSocketMap = createCPUSocketMap(cpuInfo)

	// Model Name
	metadata.ModelName = cpuInfo[0]["model name"] // optional field, empty string if missing

	// Vendor
	metadata.Vendor, ok = cpuInfo[0]["vendor_id"]
	if !ok {
		return Metadata{}, fmt.Errorf("'vendor_id' field not found in /proc/cpuinfo")
	}

	// CPU microarchitecture
	metadata.Microarchitecture, err = common.GetTargetMicroArchitecture(t, localTempDir, noRoot)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get x86 microarchitecture: %v", err)
	}

	// Number of General Purpose Counters
	metadata.NumGeneralPurposeCounters, err = getNumGPCounters(metadata.Microarchitecture)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get number of general purpose counters: %v", err)
	}

	// Run metadata scripts concurrently
	metadataScripts, err := getMetadataScripts(noRoot, noSystemSummary, metadata.NumGeneralPurposeCounters)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get metadata scripts: %v", err)
	}

	scriptOutputs, err := common.RunScripts(t, metadataScripts, true, localTempDir, statusUpdate, "collecting metadata", noRoot) // nosemgrep
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

	// PMU Driver Version
	if metadata.PMUDriverVersion, err = getPMUDriverVersion(scriptOutputs); err != nil {
		return Metadata{}, fmt.Errorf("failed to retrieve PMU driver version: %v", err)
	}

	// perf list
	if metadata.PerfSupportedEvents, err = getPerfSupportedEvents(scriptOutputs); err != nil {
		return Metadata{}, fmt.Errorf("failed to load perf list: %v", err)
	}

	// Check event support
	if err = c.detectEventSupport(&metadata, scriptOutputs); err != nil {
		return Metadata{}, err
	}

	// Kernel Version
	if metadata.KernelVersion, err = getKernelVersion(scriptOutputs); err != nil {
		return Metadata{}, fmt.Errorf("failed to retrieve kernel version: %v", err)
	}

	// System TSC Frequency
	if metadata.TSCFrequencyHz, err = getTSCFreqHz(scriptOutputs); err != nil {
		return Metadata{}, fmt.Errorf("failed to retrieve TSC frequency: %v", err)
	}
	metadata.TSC = metadata.SocketCount * metadata.CoresPerSocket * metadata.ThreadsPerCore * metadata.TSCFrequencyHz

	// Uncore device IDs and support
	isAMDArchitecture := metadata.Vendor == cpus.AMDVendor
	if metadata.UncoreDeviceIDs, err = getUncoreDeviceIDs(isAMDArchitecture, scriptOutputs); err != nil {
		return Metadata{}, fmt.Errorf("failed to retrieve uncore device IDs: %v", err)
	}
	metadata.SupportsUncore = c.checkUncoreSupport(metadata.UncoreDeviceIDs, isAMDArchitecture)

	return metadata, nil
}

// detectEventSupport checks which perf events are supported on the system.
func (c *X86MetadataCollector) detectEventSupport(metadata *Metadata, scriptOutputs map[string]script.ScriptOutput) error {
	var output string
	var err error

	// instructions
	if metadata.SupportsInstructions, output, err = getSupportsEvent("instructions", scriptOutputs); err != nil {
		slog.Warn("failed to determine if instructions event is supported, assuming not supported", slog.String("error", err.Error()))
	} else if !metadata.SupportsInstructions {
		slog.Warn("instructions event not supported", slog.String("output", output))
	}

	// ref_cycles
	if metadata.SupportsRefCycles, output, err = getSupportsEvent("ref-cycles", scriptOutputs); err != nil {
		slog.Warn("failed to determine if ref_cycles is supported, assuming not supported", slog.String("error", err.Error()))
	} else if !metadata.SupportsRefCycles {
		slog.Warn("ref-cycles not supported", slog.String("output", output))
	}

	// Fixed-counter TMA events
	if metadata.SupportsFixedTMA, output, err = getSupportsFixedTMA(scriptOutputs); err != nil {
		slog.Warn("failed to determine if fixed-counter TMA is supported, assuming not supported", slog.String("error", err.Error()))
	} else if !metadata.SupportsFixedTMA {
		slog.Warn("Fixed-counter TMA events not supported", slog.String("output", output))
	}

	// Fixed-counter cycles events
	if metadata.SupportsFixedCycles, output, err = getSupportsFixedEvent("cpu-cycles", scriptOutputs); err != nil {
		slog.Warn("failed to determine if fixed-counter 'cpu-cycles' is supported, assuming not supported", slog.String("error", err.Error()))
	} else if !metadata.SupportsFixedCycles {
		slog.Warn("Fixed-counter 'cpu-cycles' events not supported", slog.String("output", output))
	}

	// Fixed-counter ref-cycles events
	if metadata.SupportsFixedRefCycles, output, err = getSupportsFixedEvent("ref-cycles", scriptOutputs); err != nil {
		slog.Warn("failed to determine if fixed-counter 'ref-cycles' is supported, assuming not supported", slog.String("error", err.Error()))
	} else if !metadata.SupportsFixedRefCycles {
		slog.Warn("Fixed-counter 'ref-cycles' events not supported", slog.String("output", output))
	}

	// Fixed-counter instructions events
	if metadata.SupportsFixedInstructions, output, err = getSupportsFixedEvent("instructions", scriptOutputs); err != nil {
		slog.Warn("failed to determine if fixed-counter 'instructions' is supported, assuming not supported", slog.String("error", err.Error()))
	} else if !metadata.SupportsFixedInstructions {
		slog.Warn("Fixed-counter 'instructions' events not supported", slog.String("output", output))
	}

	// PEBS
	if metadata.SupportsPEBS, output, err = getSupportsPEBS(scriptOutputs); err != nil {
		slog.Warn("failed to determine if 'PEBS' is supported, assuming not supported", slog.String("error", err.Error()))
	} else if !metadata.SupportsPEBS {
		slog.Warn("'PEBS' events not supported", slog.String("output", output))
	}

	// Offcore response
	if metadata.SupportsOCR, output, err = getSupportsOCR(scriptOutputs); err != nil {
		slog.Warn("failed to determine if 'OCR' is supported, assuming not supported", slog.String("error", err.Error()))
	} else if !metadata.SupportsOCR {
		slog.Warn("'OCR' events not supported", slog.String("output", output))
	}

	return nil
}

// checkUncoreSupport determines if uncore events are supported based on detected devices.
func (c *X86MetadataCollector) checkUncoreSupport(uncoreDeviceIDs map[string][]int, isAMD bool) bool {
	for uncoreDeviceName := range uncoreDeviceIDs {
		if !isAMD && uncoreDeviceName == "cha" {
			return true
		} else if isAMD && (uncoreDeviceName == "l3" || uncoreDeviceName == "df") {
			return true
		}
	}
	slog.Warn("Uncore devices not supported")
	return false
}

// --- x86-specific helper functions ---

// getUncoreDeviceIDs returns a map of device type to list of device indices.
// e.g., "upi" -> [0,1,2,3]
func getUncoreDeviceIDs(isAMDArchitecture bool, scriptOutputs map[string]script.ScriptOutput) (IDs map[string][]int, err error) {
	if scriptOutputs[scriptListUncoreDevices].Exitcode != 0 {
		err = fmt.Errorf("failed to list uncore devices: %s", scriptOutputs[scriptListUncoreDevices].Stderr)
		return
	}
	fileNames := strings.Split(scriptOutputs[scriptListUncoreDevices].Stdout, "\n")
	IDs = make(map[string][]int)
	re := regexp.MustCompile(`(?:uncore_|amd_)(.*)_(\d+)`)
	if isAMDArchitecture {
		re = regexp.MustCompile(`(?:uncore_|amd_)(.*?)(?:_(\d+))?$`)
	}
	for _, fileName := range fileNames {
		match := re.FindStringSubmatch(fileName)
		if match == nil {
			continue
		}
		var id int
		// match[2] will never be empty for Intel Architecture due to regex filtering
		if match[2] != "" {
			if id, err = strconv.Atoi(match[2]); err != nil {
				return
			}
		}
		IDs[match[1]] = append(IDs[match[1]], id)
	}
	return
}

// getPMUDriverVersion returns the version of the Intel PMU driver.
func getPMUDriverVersion(scriptOutputs map[string]script.ScriptOutput) (version string, err error) {
	if scriptOutputs[scriptPMUDriverVersion].Exitcode != 0 {
		err = fmt.Errorf("failed to retrieve PMU driver version: %s", scriptOutputs[scriptPMUDriverVersion].Stderr)
		return
	}
	version = strings.TrimSpace(scriptOutputs[scriptPMUDriverVersion].Stdout)
	return
}

// getTSCFreqHz returns the frequency of the Time Stamp Counter (TSC) in hertz.
func getTSCFreqHz(scriptOutputs map[string]script.ScriptOutput) (freqHz int, err error) {
	if scriptOutputs[scriptTSC].Exitcode != 0 {
		err = fmt.Errorf("failed to retrieve TSC frequency: %s", scriptOutputs[scriptTSC].Stderr)
		return
	}
	freqMhz, err := strconv.Atoi(strings.TrimSpace(scriptOutputs[scriptTSC].Stdout))
	if err != nil {
		return
	}
	// convert MHz to Hz
	freqHz = freqMhz * 1000000
	return
}

// getSupportsPEBS checks if PEBS events are supported on the target.
// On some VMs (e.g. GCP C4), PEBS events are not supported and perf returns '<not supported>'.
func getSupportsPEBS(scriptOutputs map[string]script.ScriptOutput) (supported bool, output string, err error) {
	output = scriptOutputs[scriptPerfStatPEBS].Stderr
	if scriptOutputs[scriptPerfStatPEBS].Exitcode != 0 {
		err = fmt.Errorf("failed to determine if pebs is supported: %s", output)
		return
	}
	supported = !strings.Contains(output, "<not supported>")
	return
}

// getSupportsOCR checks if offcore response events are supported on the target.
func getSupportsOCR(scriptOutputs map[string]script.ScriptOutput) (supported bool, output string, err error) {
	output = scriptOutputs[scriptPerfStatOCR].Stderr
	if scriptOutputs[scriptPerfStatOCR].Exitcode != 0 {
		supported = false
		err = nil
		return
	}
	supported = !strings.Contains(output, "<not supported>")
	return
}

// getSupportsFixedTMA checks if fixed TMA counter events are supported by perf.
//
// We check for the topdown.slots and topdown-bad-spec events as an indicator
// of support for fixed TMA counter support. At the time of writing, these
// events are not supported on AWS m7i VMs or AWS m6i VMs.
func getSupportsFixedTMA(scriptOutputs map[string]script.ScriptOutput) (supported bool, output string, err error) {
	output = scriptOutputs[scriptPerfStatTMA].Stderr
	if scriptOutputs[scriptPerfStatTMA].Exitcode != 0 {
		supported = false
		err = nil
		return
	}
	// event values being zero or equal to each other indicates these events are not (properly) supported
	vals := make(map[string]float64)
	lines := strings.Split(output, "\n")
	re := regexp.MustCompile(`^\s*([0-9]+)\s+(topdown[\w.\-]+)`)
	for _, line := range lines {
		// count may include commas as thousands separators, remove them
		line = strings.ReplaceAll(line, ",", "")
		match := re.FindStringSubmatch(line)
		if match != nil {
			vals[match[2]], err = strconv.ParseFloat(match[1], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse topdown value: %v", err)
				return
			}
		}
	}
	topDownSlots := vals["topdown.slots"]
	badSpeculation := vals["topdown-bad-spec"]
	supported = topDownSlots != badSpeculation && topDownSlots != 0 && badSpeculation != 0
	return
}

// getSupportsFixedEvent checks if a fixed counter event is supported.
func getSupportsFixedEvent(event string, scriptOutputs map[string]script.ScriptOutput) (supported bool, output string, err error) {
	scriptKey := scriptPerfStatFixedPrefix + event
	output = scriptOutputs[scriptKey].Stderr
	if scriptOutputs[scriptKey].Exitcode != 0 {
		supported = false
		return
	}
	// on some VMs we see "<not counted>" or "<not supported>" in the perf output
	if strings.Contains(output, "<not counted>") || strings.Contains(output, "<not supported") {
		supported = false
		return
	}
	// on some VMs we get a count of 0
	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
		tokens := strings.Fields(line)
		if len(tokens) == 2 && tokens[0] == "0" {
			supported = false
			return
		}
	}
	supported = true
	return
}
