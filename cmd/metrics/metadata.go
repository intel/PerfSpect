package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// metrics.go defines a structure and a loading function to hold information about the platform to be
// used during data collection and metric production

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"perfspect/internal/common"
	"perfspect/internal/cpus"
	"perfspect/internal/progress"
	"perfspect/internal/script"
	"perfspect/internal/table"
	"perfspect/internal/target"
)

// CommonMetadata -- common to all architectures
type CommonMetadata struct {
	NumGeneralPurposeCounters int
	SocketCount               int
	CoresPerSocket            int
	ThreadsPerCore            int
	CPUSocketMap              map[int]int
	KernelVersion             string
	Architecture              string
	Vendor                    string
	Microarchitecture         string
	Hostname                  string
	ModelName                 string
	PerfSupportedEvents       string
	SystemSummaryFields       [][]string // slice of key-value pairs
	SupportsInstructions      bool
}

// X86Metadata -- x86_64 specific
type X86Metadata struct {
	PMUDriverVersion          string
	UncoreDeviceIDs           map[string][]int
	SupportsFixedCycles       bool
	SupportsFixedInstructions bool
	SupportsFixedTMA          bool
	SupportsFixedRefCycles    bool
	SupportsRefCycles         bool
	SupportsUncore            bool
	SupportsPEBS              bool
	SupportsOCR               bool
	TSC                       int
	TSCFrequencyHz            int
}

// ARMMetadata -- aarch64 specific
type ARMMetadata struct {
	ARMSlots int
	ARMCPUID string
}

// Metadata -- representation of the platform's state and capabilities
type Metadata struct {
	CommonMetadata
	X86Metadata
	ARMMetadata
	// below are not loaded by LoadMetadata, but are set by the caller (should these be here at all?)
	CollectionStartTime time.Time
	PerfSpectVersion    string
	WithWorkload        bool // true if metrics were collected with a user-provided workload application
}

// LoadMetadata - populates and returns a Metadata structure containing state of the
// system.
func LoadMetadata(t target.Target, noRoot bool, noSystemSummary bool, localTempDir string, statusUpdate progress.MultiSpinnerUpdateFunc) (Metadata, error) {
	uarch, err := common.GetTargetArchitecture(t)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get target architecture: %v", err)
	}
	collector, err := NewMetadataCollector(uarch)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to create metadata collector: %v", err)
	}
	return collector.CollectMetadata(t, noRoot, noSystemSummary, localTempDir, statusUpdate)
}

type MetadataCollector interface {
	CollectMetadata(t target.Target, noRoot bool, noSystemSummary bool, localTempDir string, statusUpdate progress.MultiSpinnerUpdateFunc) (Metadata, error)
}

func NewMetadataCollector(architecture string) (MetadataCollector, error) {
	switch architecture {
	case cpus.X86Architecture:
		return &X86MetadataCollector{}, nil
	case cpus.ARMArchitecture:
		return &ARMMetadataCollector{}, nil
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", architecture)
	}
}

// X86MetadataCollector handles Intel/AMD x86_64 metadata collection
type X86MetadataCollector struct {
}

// ARMMetadataCollector handles ARM metadata collection
type ARMMetadataCollector struct {
}

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
	// Core Count (per socket) (from cpuInfo)
	metadata.CoresPerSocket, err = strconv.Atoi(cpuInfo[0]["cpu cores"])
	if err != nil || metadata.CoresPerSocket == 0 {
		return Metadata{}, fmt.Errorf("failed to retrieve cores per socket: %v", err)
	}
	// Socket Count (from cpuInfo)
	var maxPhysicalID int
	if maxPhysicalID, err = strconv.Atoi(cpuInfo[len(cpuInfo)-1]["physical id"]); err != nil {
		return Metadata{}, fmt.Errorf("failed to retrieve max physical id: %v", err)
	}
	metadata.SocketCount = maxPhysicalID + 1
	// Hyperthreading - threads per core (from cpuInfo)
	if cpuInfo[0]["siblings"] != cpuInfo[0]["cpu cores"] {
		metadata.ThreadsPerCore = 2
	} else {
		metadata.ThreadsPerCore = 1
	}
	// CPUSocketMap (from cpuInfo)
	metadata.CPUSocketMap = createCPUSocketMap(cpuInfo)
	// Model Name (from cpuInfo)
	metadata.ModelName = cpuInfo[0]["model name"]
	// Vendor (from cpuInfo)
	metadata.Vendor = cpuInfo[0]["vendor_id"]
	// CPU microarchitecture (from cpuInfo)
	metadata.Microarchitecture, err = common.GetTargetMicroArchitecture(t, localTempDir, noRoot)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get x86 microarchitecture: %v", err)
	}
	// Number of General Purpose Counters
	metadata.NumGeneralPurposeCounters, err = getNumGPCounters(metadata.Microarchitecture)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get number of general purpose counters: %v", err)
	}
	// the rest of the metadata is retrieved by running scripts in parallel
	metadataScripts, err := getMetadataScripts(noRoot, noSystemSummary, metadata.NumGeneralPurposeCounters)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get metadata scripts: %v", err)
	}
	// run the scripts
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
		metadata.SystemSummaryFields = [][]string{{"", "System Info Not Available"}}
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
	// instructions
	var output string
	if metadata.SupportsInstructions, output, err = getSupportsEvent("instructions", scriptOutputs); err != nil {
		slog.Warn("failed to determine if instructions event is supported, assuming not supported", slog.String("error", err.Error()))
	} else {
		if !metadata.SupportsInstructions {
			slog.Warn("instructions event not supported", slog.String("output", output))
		}
	}
	// ref_cycles
	if metadata.SupportsRefCycles, output, err = getSupportsEvent("ref-cycles", scriptOutputs); err != nil {
		slog.Warn("failed to determine if ref_cycles is supported, assuming not supported", slog.String("error", err.Error()))
	} else {
		if !metadata.SupportsRefCycles {
			slog.Warn("ref-cycles not supported", slog.String("output", output))
		}
	}
	// Fixed-counter TMA events
	if metadata.SupportsFixedTMA, output, err = getSupportsFixedTMA(scriptOutputs); err != nil {
		slog.Warn("failed to determine if fixed-counter TMA is supported, assuming not supported", slog.String("error", err.Error()))
	} else {
		if !metadata.SupportsFixedTMA {
			slog.Warn("Fixed-counter TMA events not supported", slog.String("output", output))
		}
	}
	// Fixed-counter cycles events
	if metadata.SupportsFixedCycles, output, err = getSupportsFixedEvent("cpu-cycles", scriptOutputs); err != nil {
		slog.Warn("failed to determine if fixed-counter 'cpu-cycles' is supported, assuming not supported", slog.String("error", err.Error()))
	} else {
		if !metadata.SupportsFixedCycles {
			slog.Warn("Fixed-counter 'cpu-cycles' events not supported", slog.String("output", output))
		}
	}
	// Fixed-counter ref-cycles events
	if metadata.SupportsFixedRefCycles, output, err = getSupportsFixedEvent("ref-cycles", scriptOutputs); err != nil {
		slog.Warn("failed to determine if fixed-counter 'ref-cycles' is supported, assuming not supported", slog.String("error", err.Error()))
	} else {
		if !metadata.SupportsFixedRefCycles {
			slog.Warn("Fixed-counter 'ref-cycles' events not supported", slog.String("output", output))
		}
	}
	// Fixed-counter instructions events
	if metadata.SupportsFixedInstructions, output, err = getSupportsFixedEvent("instructions", scriptOutputs); err != nil {
		slog.Warn("failed to determine if fixed-counter 'instructions' is supported, assuming not supported", slog.String("error", err.Error()))
	} else {
		if !metadata.SupportsFixedInstructions {
			slog.Warn("Fixed-counter 'instructions' events not supported", slog.String("output", output))
		}
	}
	// PEBS
	if metadata.SupportsPEBS, output, err = getSupportsPEBS(scriptOutputs); err != nil {
		slog.Warn("failed to determine if 'PEBS' is supported, assuming not supported", slog.String("error", err.Error()))
	} else {
		if !metadata.SupportsPEBS {
			slog.Warn("'PEBS' events not supported", slog.String("output", output))
		}
	}
	// Offcore response
	if metadata.SupportsOCR, output, err = getSupportsOCR(scriptOutputs); err != nil {
		slog.Warn("failed to determine if 'OCR' is supported, assuming not supported", slog.String("error", err.Error()))
	} else {
		if !metadata.SupportsOCR {
			slog.Warn("'OCR' events not supported", slog.String("output", output))
		}
	}
	// Kernel Version
	if metadata.KernelVersion, err = getKernelVersion(scriptOutputs); err != nil {
		return Metadata{}, fmt.Errorf("failed to retrieve kernel version: %v", err)
	}
	// System TSC Frequency
	if metadata.TSCFrequencyHz, err = getTSCFreqHz(scriptOutputs); err != nil {
		return Metadata{}, fmt.Errorf("failed to retrieve TSC frequency: %v", err)
	} else {
		metadata.TSC = metadata.SocketCount * metadata.CoresPerSocket * metadata.ThreadsPerCore * metadata.TSCFrequencyHz
	}
	// uncore device IDs and uncore support
	isAMDArchitecture := metadata.Vendor == cpus.AMDVendor
	if metadata.UncoreDeviceIDs, err = getUncoreDeviceIDs(isAMDArchitecture, scriptOutputs); err != nil {
		return Metadata{}, fmt.Errorf("failed to retrieve uncore device IDs: %v", err)
	} else {
		for uncoreDeviceName := range metadata.UncoreDeviceIDs {
			if !isAMDArchitecture && uncoreDeviceName == "cha" { // could be any uncore device
				metadata.SupportsUncore = true
				break
			} else if isAMDArchitecture && (uncoreDeviceName == "l3" || uncoreDeviceName == "df") { // could be any uncore device
				metadata.SupportsUncore = true
				break
			}
		}
		if !metadata.SupportsUncore {
			slog.Warn("Uncore devices not supported")
		}
	}
	return metadata, nil
}

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
	// TODO: this currently assumes one socket
	metadata.CPUSocketMap = make(map[int]int)
	for i := range metadata.CoresPerSocket {
		metadata.CPUSocketMap[i] = 0
	}
	metadata.Microarchitecture, err = common.GetTargetMicroArchitecture(t, localTempDir, noRoot)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get ARM microarchitecture: %v", err)
	}
	metadata.NumGeneralPurposeCounters, err = getNumGPCountersARM(t, localTempDir, noRoot)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get number of general purpose counters: %v", err)
	}
	// the rest of the metadata is retrieved by running scripts in parallel and then parsing the output
	metadataScripts, err := getMetadataScripts(noRoot, noSystemSummary, metadata.NumGeneralPurposeCounters)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get metadata scripts: %v", err)
	}
	// run the scripts
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
		metadata.SystemSummaryFields = [][]string{{"", "System Info Not Available"}}
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
	// instructions
	var output string
	if metadata.SupportsInstructions, output, err = getSupportsEvent("instructions", scriptOutputs); err != nil {
		slog.Warn("failed to determine if instructions event is supported, assuming not supported", slog.String("error", err.Error()))
	} else {
		if !metadata.SupportsInstructions {
			slog.Warn("instructions event not supported", slog.String("output", output))
		}
	}
	return metadata, nil
}

func getMetadataScripts(noRoot bool, noSystemSummary bool, numGPCounters int) (metadataScripts []script.ScriptDefinition, err error) {
	// reduce startup time by running the metadata scripts in parallel
	metadataScriptDefs := []script.ScriptDefinition{
		{
			Name:           "get architecture",
			ScriptTemplate: "uname -m",
			Superuser:      !noRoot,
		},
		{
			Name:           "perf supported events",
			ScriptTemplate: "perf list",
			Superuser:      !noRoot,
			Depends:        []string{"perf"},
		},
		{
			Name:           "list uncore devices",
			ScriptTemplate: "find /sys/bus/event_source/devices/ \\( -name uncore_* -o -name amd_* \\)",
			Superuser:      !noRoot,
			Architectures:  []string{cpus.X86Architecture},
		},
		{
			Name:           "perf stat instructions",
			ScriptTemplate: "perf stat -a -e instructions sleep 1",
			Superuser:      !noRoot,
			Depends:        []string{"perf"},
		},
		{
			Name:           "perf stat ref-cycles",
			ScriptTemplate: "perf stat -a -e ref-cycles sleep 1",
			Superuser:      !noRoot,
			Depends:        []string{"perf"},
		},
		{
			Name:           "perf stat pebs",
			ScriptTemplate: "perf stat -a -e INT_MISC.UNKNOWN_BRANCH_CYCLES sleep 1",
			Superuser:      !noRoot,
			Architectures:  []string{cpus.X86Architecture},
			Depends:        []string{"perf"},
		},
		{
			Name:           "perf stat ocr",
			ScriptTemplate: "perf stat -a -e OCR.READS_TO_CORE.LOCAL_DRAM sleep 1",
			Superuser:      !noRoot,
			Architectures:  []string{cpus.X86Architecture},
			Depends:        []string{"perf"},
		},
		{
			Name:           "perf stat tma",
			ScriptTemplate: "perf stat -a -e '{topdown.slots, topdown-bad-spec}' sleep 1",
			Superuser:      !noRoot,
			Architectures:  []string{cpus.X86Architecture},
			Depends:        []string{"perf"},
		},
		{
			Name:           "perf stat fixed instructions",
			ScriptTemplate: "perf stat -a -e '{{{.InstructionsList}}}' sleep 1",
			Superuser:      !noRoot,
			Depends:        []string{"perf"},
		},
		{
			Name:           "perf stat fixed cpu-cycles",
			ScriptTemplate: "perf stat -a -e '{{{.CpuCyclesList}}}' sleep 1",
			Superuser:      !noRoot,
			Depends:        []string{"perf"},
		},
		{
			Name:           "perf stat fixed ref-cycles",
			ScriptTemplate: "perf stat -a -e '{{{.RefCyclesList}}}' sleep 1",
			Superuser:      !noRoot,
			Depends:        []string{"perf"},
		},
		{
			Name:           "pmu driver version",
			ScriptTemplate: "dmesg | grep -A 1 \"Intel PMU driver\" | tail -1 | awk '{print $NF}'",
			Superuser:      !noRoot,
		},
		{
			Name:           "tsc",
			ScriptTemplate: "tsc && echo",
			Depends:        []string{"tsc"},
			Superuser:      !noRoot,
			Architectures:  []string{cpus.X86Architecture},
		},
		{
			Name:           "kernel version",
			ScriptTemplate: "uname -r",
			Superuser:      !noRoot,
		},
		{
			Name:           "arm slots",
			ScriptTemplate: "cat /sys/bus/event_source/devices/armv8_pmuv3_0/caps/slots",
			Superuser:      !noRoot,
			Architectures:  []string{cpus.ARMArchitecture},
		},
		{
			Name:           "arm cpuid",
			ScriptTemplate: "cat /sys/devices/system/cpu/cpu0/regs/identification/midr_el1",
			Superuser:      !noRoot,
			Architectures:  []string{cpus.ARMArchitecture},
		},
	}
	// replace script template vars
	for _, scriptDef := range metadataScriptDefs {
		switch scriptDef.Name {
		case "perf stat fixed instructions":
			var eventList []string
			for range numGPCounters + 1 {
				eventList = append(eventList, "instructions")
			}
			scriptDef.ScriptTemplate = strings.Replace(scriptDef.ScriptTemplate, "{{.InstructionsList}}", strings.Join(eventList, ","), -1)
		case "perf stat fixed cpu-cycles":
			var eventList []string
			for range numGPCounters + 1 {
				eventList = append(eventList, "cpu-cycles")
			}
			scriptDef.ScriptTemplate = strings.Replace(scriptDef.ScriptTemplate, "{{.CpuCyclesList}}", strings.Join(eventList, ","), -1)
		case "perf stat fixed ref-cycles":
			var eventList []string
			for range numGPCounters + 1 {
				eventList = append(eventList, "ref-cycles")
			}
			scriptDef.ScriptTemplate = strings.Replace(scriptDef.ScriptTemplate, "{{.RefCyclesList}}", strings.Join(eventList, ","), -1)
		}
		metadataScripts = append(metadataScripts, scriptDef)
	}
	// add the system summary table scripts to the list
	if !noSystemSummary {
		for _, scriptName := range common.TableDefinitions[common.BriefSysSummaryTableName].ScriptNames {
			scriptDef := script.GetScriptByName(scriptName)
			metadataScripts = append(metadataScripts, scriptDef)
		}
	}
	return
}

// String - provides a string representation of the Metadata structure
func (md Metadata) String() string {
	// Create a copy of the metadata without the PerfSupportedEvents field
	mdCopy := md
	mdCopy.PerfSupportedEvents = "" // Set to empty to reduce log size

	// Marshal the copy to JSON (no indentation)
	jsonData, err := json.Marshal(mdCopy)
	if err != nil {
		return fmt.Sprintf("Error marshaling metadata to JSON: %v", err)
	}

	return string(jsonData)
}

func (md Metadata) Initialized() bool {
	return md.SocketCount != 0 && md.CoresPerSocket != 0
}

// JSON converts the Metadata struct to a JSON-encoded byte slice.
//
// Returns:
// - out: JSON-encoded byte slice representation of the Metadata.
// - err: error encountered during the marshaling process, if any.
func (md Metadata) JSON() (out []byte, err error) {
	if !md.Initialized() {
		return []byte("null"), nil
	}
	if out, err = json.Marshal(md); err != nil {
		slog.Error("failed to marshal metadata structure", slog.String("error", err.Error()))
		return
	}
	return
}

// WriteJSONToFile writes the metadata structure to the filename provided
// Note that the file will be truncated.
func (md Metadata) WriteJSONToFile(path string) (err error) {
	rawFile, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644) // #nosec G304 G302
	if err != nil {
		slog.Error("failed to open raw file for writing", slog.String("error", err.Error()))
		return
	}
	defer rawFile.Close()
	var out []byte
	if out, err = md.JSON(); err != nil {
		return
	}
	out = append(out, []byte("\n")...)
	if _, err = rawFile.Write(out); err != nil {
		slog.Error("failed to write metadata json to file", slog.String("error", err.Error()))
		return
	}
	return
}

// ReadJSONFromFile reads the metadata structure from the filename provided
func ReadJSONFromFile(path string) (md Metadata, err error) {
	// read the file
	var rawBytes []byte
	rawBytes, err = os.ReadFile(path) // #nosec G304
	if err != nil {
		slog.Error("failed to read metadata file", slog.String("error", err.Error()))
		return
	}
	if err = json.Unmarshal(rawBytes, &md); err != nil {
		slog.Error("failed to unmarshal metadata json", slog.String("error", err.Error()))
		return
	}
	return
}

// getSystemSummary - retrieves the system summary from the target
func getSystemSummary(scriptOutputs map[string]script.ScriptOutput) (summaryFields [][]string, err error) {
	allTableValues, err := table.ProcessTables([]table.TableDefinition{common.TableDefinitions[common.BriefSysSummaryTableName]}, scriptOutputs)
	if err != nil {
		err = fmt.Errorf("failed to process script outputs: %w", err)
		return
	} else {
		for _, field := range allTableValues[0].Fields {
			summaryFields = append(summaryFields, []string{field.Name, field.Values[0]})
		}
	}
	return
}

// getArchitecture - retrieves the architecture from the target
func getArchitecture(scriptOutputs map[string]script.ScriptOutput) (arch string, err error) {
	if scriptOutputs["get architecture"].Exitcode != 0 {
		err = fmt.Errorf("failed to retrieve architecture: %s", scriptOutputs["get architecture"].Stderr)
		return
	}
	arch = strings.TrimSpace(scriptOutputs["get architecture"].Stdout)
	return
}

// getUncoreDeviceIDs - returns a map of device type to list of device indices
// e.g., "upi" -> [0,1,2,3],
func getUncoreDeviceIDs(isAMDArchitecture bool, scriptOutputs map[string]script.ScriptOutput) (IDs map[string][]int, err error) {
	if scriptOutputs["list uncore devices"].Exitcode != 0 {
		err = fmt.Errorf("failed to list uncore devices: %s", scriptOutputs["list uncore devices"].Stderr)
		return
	}
	fileNames := strings.Split(scriptOutputs["list uncore devices"].Stdout, "\n")
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
		//match[2] will never be empty for Intel Architecture due to regex filtering
		if match[2] != "" {
			if id, err = strconv.Atoi(match[2]); err != nil {
				return
			}
		}
		IDs[match[1]] = append(IDs[match[1]], id)
	}
	return
}

// getCPUInfo - reads and returns all data from /proc/cpuinfo
func getCPUInfo(t target.Target) (cpuInfo []map[string]string, err error) {
	cmd := exec.Command("cat", "/proc/cpuinfo")
	stdout, stderr, exitcode, err := t.RunCommand(cmd, 0, true)
	if err != nil {
		err = fmt.Errorf("failed to get cpuinfo: %s, %d, %v", stderr, exitcode, err)
		return
	}
	oneCPUInfo := make(map[string]string)
	for line := range strings.SplitSeq(stdout, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			if len(oneCPUInfo) > 0 {
				cpuInfo = append(cpuInfo, oneCPUInfo)
				oneCPUInfo = make(map[string]string)
				continue
			} else {
				break
			}
		}
		oneCPUInfo[strings.TrimSpace(fields[0])] = strings.TrimSpace(fields[1])
	}
	return
}

// getLscpu - runs lscpu on the target and returns the output
func getLscpu(t target.Target) (output string, err error) {
	cmd := exec.Command("lscpu")
	output, stderr, exitcode, err := t.RunCommand(cmd, 0, true)
	if err != nil || exitcode != 0 {
		err = fmt.Errorf("failed to run lscpu: %s, %d, %v", stderr, exitcode, err)
		return
	}
	return
}

func parseLscpuIntField(lscpu string, pattern string) (int, error) {
	re := regexp.MustCompile(pattern)
	for line := range strings.SplitSeq(lscpu, "\n") {
		match := re.FindStringSubmatch(line)
		if match != nil {
			value, err := strconv.Atoi(strings.TrimSpace(match[1]))
			if err != nil {
				return 0, fmt.Errorf("failed to parse integer from lscpu field: %v", err)
			}
			return value, nil
		}
	}
	return 0, fmt.Errorf("lscpu field not found")
}
func parseLscpuStringField(lscpu string, pattern string) (string, error) {
	re := regexp.MustCompile(pattern)
	for line := range strings.SplitSeq(lscpu, "\n") {
		match := re.FindStringSubmatch(line)
		if match != nil {
			return strings.TrimSpace(match[1]), nil
		}
	}
	return "", fmt.Errorf("lscpu field not found")
}

// getPerfSupportedEvents - returns a string containing the output from
// 'perf list'
func getPerfSupportedEvents(scriptOutputs map[string]script.ScriptOutput) (supportedEvents string, err error) {
	supportedEvents = scriptOutputs["perf supported events"].Stdout
	if scriptOutputs["perf supported events"].Exitcode != 0 {
		err = fmt.Errorf("failed to get perf supported events: %s", scriptOutputs["perf supported events"].Stderr)
		return
	}
	return
}

// getSupportsEvent() - checks if the event is supported by perf
func getSupportsEvent(event string, scriptOutputs map[string]script.ScriptOutput) (supported bool, output string, err error) {
	output = scriptOutputs["perf stat "+event].Stderr
	if scriptOutputs["perf stat "+event].Exitcode != 0 {
		err = fmt.Errorf("failed to determine if %s is supported: %s", event, output)
		return
	}
	supported = !strings.Contains(output, "<not supported>")
	return
}

// getSupportsPEBS() - checks if the PEBS events are supported on the target
// On some VMs, e.g. GCP C4, PEBS events are not supported and perf returns '<not supported>'
// Events that use MSR 0x3F7 are PEBS events. We use the INT_MISC.UNKNOWN_BRANCH_CYCLES event since
// it is a PEBS event that we used in EMR metrics.
func getSupportsPEBS(scriptOutputs map[string]script.ScriptOutput) (supported bool, output string, err error) {
	output = scriptOutputs["perf stat pebs"].Stderr
	if scriptOutputs["perf stat pebs"].Exitcode != 0 {
		err = fmt.Errorf("failed to determine if pebs is supported: %s", output)
		return
	}
	supported = !strings.Contains(output, "<not supported>")
	return
}

// getSupportsOCR() - checks if the offcore response events are supported on the target
// On some VMs, e.g. GCP C4, offcore response events are not supported and perf returns '<not supported>'
func getSupportsOCR(scriptOutputs map[string]script.ScriptOutput) (supported bool, output string, err error) {
	output = scriptOutputs["perf stat ocr"].Stderr
	if scriptOutputs["perf stat ocr"].Exitcode != 0 {
		supported = false
		err = nil
		return
	}
	supported = !strings.Contains(output, "<not supported>")
	return
}

// getSupportsFixedTMA - checks if the fixed TMA counter events are
// supported by perf.
//
// We check for the topdown.slots and topdown-bad-spec events as
// an indicator of support for fixed TMA counter support. At the time of
// writing, these events are not supported on AWS m7i VMs or AWS m6i VMs.  On
// AWS m7i VMs, we get an error from the perf stat command below. On AWS m6i
// VMs, the values of the events equal to each other.
// In some other situations (need to find/document) the event count values are
// zero.
// All three of these failure modes are checked for in this function.
func getSupportsFixedTMA(scriptOutputs map[string]script.ScriptOutput) (supported bool, output string, err error) {
	output = scriptOutputs["perf stat tma"].Stderr
	if scriptOutputs["perf stat tma"].Exitcode != 0 {
		supported = false
		err = nil
		return
	}
	// event values being zero or equal to each other is 2nd indication that these events are not (properly) supported
	vals := make(map[string]float64)
	lines := strings.Split(output, "\n")
	// example lines:
	// "     1078623236      topdown.slots                                                           (34.40%)"
	// "        83572327       topdown-bad-spec                                                       (34.40%)"
	re := regexp.MustCompile(`^\s*([0-9]+)\s+(topdown[\w.\-]+)`)
	for _, line := range lines {
		// count may include commas as thousands separators, remove them
		line = strings.ReplaceAll(line, ",", "")
		match := re.FindStringSubmatch(line)
		if match != nil {
			vals[match[2]], err = strconv.ParseFloat(match[1], 64)
			if err != nil {
				// this should never happen
				panic("failed to parse float")
			}
		}
	}
	topDownSlots := vals["topdown.slots"]
	badSpeculation := vals["topdown-bad-spec"]
	supported = topDownSlots != badSpeculation && topDownSlots != 0 && badSpeculation != 0
	return
}

func getNumGPCounters(uarch string) (numGPCounters int, err error) {
	shortUarch := uarch[:3]
	switch shortUarch {
	case cpus.UarchBDX, cpus.UarchSKX, cpus.UarchCLX:
		numGPCounters = 4
	case cpus.UarchICX, cpus.UarchSPR, cpus.UarchEMR, cpus.UarchSRF, cpus.UarchCWF, cpus.UarchGNR:
		numGPCounters = 8
	case "Gen", "Ber", "Tur":
		numGPCounters = 5
	default:
		err = fmt.Errorf("unsupported uarch: %s", uarch)
		return
	}
	return
}

// getNumGPCountersARM - returns the number of general purpose counters on ARM systems
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

// getARMSlots - returns the number of ARM slots available
// Copyright 2025 Google LLC.
// SPDX-License-Identifier: BSD-3-Clause
// Contributed by Edwin Chiu
func getARMSlots(scriptOutputs map[string]script.ScriptOutput) (slots int, err error) {
	if scriptOutputs["arm slots"].Exitcode != 0 {
		slog.Warn("failed to retrieve ARM slots", slog.Any("script", scriptOutputs["arm slots"]))
		err = fmt.Errorf("failed to retrieve ARM slots: %s", scriptOutputs["arm slots"].Stderr)
		return
	}
	hexString := strings.TrimSpace(string(scriptOutputs["arm slots"].Stdout))
	hexString = strings.TrimPrefix(hexString, "0x")
	parsedValue, err := strconv.ParseInt(hexString, 16, 64)
	if err != nil {
		slog.Warn("Failed to parse ARM slots value", slog.String("value", hexString), slog.Any("error", err))
		err = fmt.Errorf("failed to parse ARM slots value (%s): %w", hexString, err)
		return
	}
	if parsedValue <= math.MinInt32 || parsedValue > math.MaxInt32 {
		slog.Warn("Parsed ARM slots value out of range", slog.Int64("value", parsedValue))
		err = fmt.Errorf("parsed ARM slots value out of range: %d", parsedValue)
		return
	}
	slots = int(parsedValue)
	slog.Debug("Successfully read ARM slots value", slog.Int("slots", slots))
	return
}

// getARMSlotsByArchitecture - returns the number of ARM slots based on the microarchitecture
// Used as a fallback when we cannot read the slots from sysfs
func getARMSlotsByArchitecture(uarch string) (slots int, err error) {
	switch uarch {
	case cpus.UarchGraviton4, cpus.UarchAxion:
		slots = 8
	case cpus.UarchGraviton2, cpus.UarchGraviton3:
		slots = 6
	case cpus.UarchAmpereOneAC04, cpus.UarchAmpereOneAC04_1:
		slots = 10
	default:
		err = fmt.Errorf("unsupported ARM uarch: %s", uarch)
		return
	}
	return
}

// getARMCPUID retrieves the ARM CPUID from the script outputs.
// script output will have a hex value like 0x00000000410fd4f1
func getARMCPUID(scriptOutputs map[string]script.ScriptOutput) (cpuid string, err error) {
	output, ok := scriptOutputs["arm cpuid"]
	if !ok || output.Exitcode != 0 {
		err = fmt.Errorf("failed to retrieve ARM CPUID: %s", output.Stderr)
		return
	}
	cpuid = strings.TrimSpace(output.Stdout)
	return
}

func getSupportsFixedEvent(event string, scriptOutputs map[string]script.ScriptOutput) (supported bool, output string, err error) {
	output = scriptOutputs["perf stat fixed "+event].Stderr
	if scriptOutputs["perf stat fixed "+event].Exitcode != 0 {
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

// getPMUDriverVersion - returns the version of the Intel PMU driver
func getPMUDriverVersion(scriptOutputs map[string]script.ScriptOutput) (version string, err error) {
	if scriptOutputs["pmu driver version"].Exitcode != 0 {
		err = fmt.Errorf("failed to retrieve PMU driver version: %s", scriptOutputs["pmu driver version"].Stderr)
		return
	}
	version = strings.TrimSpace(scriptOutputs["pmu driver version"].Stdout)
	return
}

// getTSCFreqHz returns the frequency of the Time Stamp Counter (TSC) in hertz.
// It takes a t parameter of type target.Target and returns the frequency
// in hertz and an error if any.
func getTSCFreqHz(scriptOutputs map[string]script.ScriptOutput) (freqHz int, err error) {
	if scriptOutputs["tsc"].Exitcode != 0 {
		err = fmt.Errorf("failed to retrieve TSC frequency: %s", scriptOutputs["tsc"].Stderr)
		return
	}
	freqMhz, err := strconv.Atoi(strings.TrimSpace(scriptOutputs["tsc"].Stdout))
	if err != nil {
		return
	}
	// convert MHz to Hz
	freqHz = freqMhz * 1000000
	return
}

// getKernelVersion returns the kernel version of the system.
func getKernelVersion(scriptOutputs map[string]script.ScriptOutput) (version string, err error) {
	if scriptOutputs["kernel version"].Exitcode != 0 {
		err = fmt.Errorf("failed to retrieve kernel version: %s", scriptOutputs["kernel version"].Stderr)
		return
	}
	version = strings.TrimSpace(scriptOutputs["kernel version"].Stdout)
	return
}

// createCPUSocketMap creates a mapping of logical CPUs to their corresponding sockets.
// The function traverses the output of /proc/cpuinfo and examines the "processor" and "physical id" fields.
// It returns a map where the key is the logical CPU index and the value is the socket index.
func createCPUSocketMap(cpuInfo []map[string]string) (cpuSocketMap map[int]int) {
	// Create an empty map
	cpuSocketMap = make(map[int]int)

	// Iterate over the CPU info to create the mapping
	for idx := range cpuInfo {
		procID, _ := strconv.Atoi(cpuInfo[idx]["processor"])
		physID, _ := strconv.Atoi(cpuInfo[idx]["physical id"])
		cpuSocketMap[procID] = physID
	}

	return cpuSocketMap
}
