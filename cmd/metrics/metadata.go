// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

// metadata.go defines structures and functions to hold information about the platform
// to be used during data collection and metric production.
//
// Architecture-specific collectors are in metadata_x86.go and metadata_aarch.go.

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"perfspect/internal/app"
	"perfspect/internal/cpus"
	"perfspect/internal/progress"
	"perfspect/internal/script"
	"perfspect/internal/table"
	"perfspect/internal/target"
	"perfspect/internal/workflow"
)

// Script name constants - used as map keys when retrieving script outputs.
// Using constants prevents silent failures from typos.
const (
	scriptGetArchitecture        = "get architecture"
	scriptPerfSupportedEvents    = "perf supported events"
	scriptListUncoreDevices      = "list uncore devices"
	scriptPerfStatInstructions   = "perf stat instructions"
	scriptPerfStatRefCycles      = "perf stat ref-cycles"
	scriptPerfStatPEBS           = "perf stat pebs"
	scriptPerfStatOCR            = "perf stat ocr"
	scriptPerfStatTMA            = "perf stat tma"
	scriptPerfStatFixedPrefix    = "perf stat fixed "
	scriptPerfStatFixedInstr     = scriptPerfStatFixedPrefix + "instructions"
	scriptPerfStatFixedCycles    = scriptPerfStatFixedPrefix + "cpu-cycles"
	scriptPerfStatFixedRefCycles = scriptPerfStatFixedPrefix + "ref-cycles"
	scriptPMUDriverVersion       = "pmu driver version"
	scriptTSC                    = "tsc"
	scriptKernelVersion          = "kernel version"
	scriptARMSlots               = "arm slots"
	scriptARMCPUID               = "arm cpuid"
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

// MetadataCollector defines the interface for architecture-specific metadata collection.
type MetadataCollector interface {
	CollectMetadata(t target.Target, noRoot bool, noSystemSummary bool, localTempDir string, statusUpdate progress.MultiSpinnerUpdateFunc) (Metadata, error)
}

// LoadMetadata populates and returns a Metadata structure containing state of the system.
func LoadMetadata(t target.Target, noRoot bool, noSystemSummary bool, localTempDir string, statusUpdate progress.MultiSpinnerUpdateFunc) (Metadata, error) {
	uarch, err := workflow.GetTargetArchitecture(t)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to get target architecture: %v", err)
	}
	collector, err := NewMetadataCollector(uarch)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to create metadata collector: %v", err)
	}
	return collector.CollectMetadata(t, noRoot, noSystemSummary, localTempDir, statusUpdate)
}

// NewMetadataCollector creates the appropriate collector for the given architecture.
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

// Base script definitions for metadata collection.
// These are copied and parameterized by getMetadataScripts().
var baseMetadataScripts = []script.ScriptDefinition{
	{
		Name:           scriptGetArchitecture,
		ScriptTemplate: "uname -m",
	},
	{
		Name: scriptPerfSupportedEvents,
		ScriptTemplate: `# Parse perf list JSON output to extract Hardware events and cstate/power events
perf list --json 2>/dev/null | awk '
BEGIN {
    in_hardware_event = 0
    event_name = ""
}

# Capture EventName
/"EventName":/ {
    # Extract the value between quotes after "EventName":
    line = $0
    sub(/.*"EventName": "/, "", line)
    sub(/".*/, "", line)
    event_name = line
}

# Check if EventType is Hardware event
/"EventType": "Hardware event"/ {
    in_hardware_event = 1
}

# At end of object (closing brace), check if we should print
/^}/ {
    if (in_hardware_event ||
        event_name ~ /^cstate_core\// ||
        event_name ~ /^cstate_pkg\// ||
        event_name ~ /^power\//) {
        if (event_name != "") {
            print event_name
        }
    }
    # Reset for next object
    in_hardware_event = 0
    event_name = ""
}
' # end of awk
`,
		Depends: []string{"perf"},
	},
	{
		Name:           scriptListUncoreDevices,
		ScriptTemplate: "find /sys/bus/event_source/devices/ \\( -name uncore_* -o -name amd_* \\)",
		Architectures:  []string{cpus.X86Architecture},
	},
	{
		Name:           scriptPerfStatInstructions,
		ScriptTemplate: "perf stat -a -e instructions sleep 1",
		Depends:        []string{"perf"},
	},
	{
		Name:           scriptPerfStatRefCycles,
		ScriptTemplate: "perf stat -a -e ref-cycles sleep 1",
		Depends:        []string{"perf"},
	},
	{
		Name:           scriptPerfStatPEBS,
		ScriptTemplate: "perf stat -a -e INT_MISC.UNKNOWN_BRANCH_CYCLES sleep 1",
		Architectures:  []string{cpus.X86Architecture},
		Depends:        []string{"perf"},
	},
	{
		Name:           scriptPerfStatOCR,
		ScriptTemplate: "perf stat -a -e OCR.READS_TO_CORE.LOCAL_DRAM sleep 1",
		Architectures:  []string{cpus.X86Architecture},
		Depends:        []string{"perf"},
	},
	{
		Name:           scriptPerfStatTMA,
		ScriptTemplate: "perf stat -a -e '{topdown.slots, topdown-bad-spec}' sleep 1",
		Architectures:  []string{cpus.X86Architecture},
		Depends:        []string{"perf"},
	},
	{
		Name:           scriptPerfStatFixedInstr,
		ScriptTemplate: "perf stat -a -e '{{{.InstructionsList}}}' sleep 1",
		Depends:        []string{"perf"},
	},
	{
		Name:           scriptPerfStatFixedCycles,
		ScriptTemplate: "perf stat -a -e '{{{.CpuCyclesList}}}' sleep 1",
		Depends:        []string{"perf"},
	},
	{
		Name:           scriptPerfStatFixedRefCycles,
		ScriptTemplate: "perf stat -a -e '{{{.RefCyclesList}}}' sleep 1",
		Depends:        []string{"perf"},
	},
	{
		Name:           scriptPMUDriverVersion,
		ScriptTemplate: "dmesg | grep -A 1 \"Intel PMU driver\" | tail -1 | awk '{print $NF}'",
	},
	{
		Name:           scriptTSC,
		ScriptTemplate: "tsc && echo",
		Depends:        []string{"tsc"},
		Architectures:  []string{cpus.X86Architecture},
	},
	{
		Name:           scriptKernelVersion,
		ScriptTemplate: "uname -r",
	},
	{
		Name:           scriptARMSlots,
		ScriptTemplate: "cat /sys/bus/event_source/devices/armv8_pmuv3_0/caps/slots",
		Architectures:  []string{cpus.ARMArchitecture},
	},
	{
		Name:           scriptARMCPUID,
		ScriptTemplate: "cat /sys/devices/system/cpu/cpu0/regs/identification/midr_el1",
		Architectures:  []string{cpus.ARMArchitecture},
	},
}

// getMetadataScripts returns the list of scripts to run for metadata collection.
// It copies the base definitions and applies template replacements and privilege settings.
func getMetadataScripts(noRoot bool, noSystemSummary bool, numGPCounters int) ([]script.ScriptDefinition, error) {
	metadataScripts := make([]script.ScriptDefinition, 0, len(baseMetadataScripts))

	// Copy base scripts and apply settings
	for _, baseDef := range baseMetadataScripts {
		scriptDef := baseDef
		scriptDef.Superuser = !noRoot

		// Apply template replacements for fixed counter scripts
		switch scriptDef.Name {
		case scriptPerfStatFixedInstr:
			eventList := make([]string, numGPCounters+1)
			for i := range eventList {
				eventList[i] = "instructions"
			}
			scriptDef.ScriptTemplate = strings.Replace(scriptDef.ScriptTemplate, "{{.InstructionsList}}", strings.Join(eventList, ","), -1)
		case scriptPerfStatFixedCycles:
			eventList := make([]string, numGPCounters+1)
			for i := range eventList {
				eventList[i] = "cpu-cycles"
			}
			scriptDef.ScriptTemplate = strings.Replace(scriptDef.ScriptTemplate, "{{.CpuCyclesList}}", strings.Join(eventList, ","), -1)
		case scriptPerfStatFixedRefCycles:
			eventList := make([]string, numGPCounters+1)
			for i := range eventList {
				eventList[i] = "ref-cycles"
			}
			scriptDef.ScriptTemplate = strings.Replace(scriptDef.ScriptTemplate, "{{.RefCyclesList}}", strings.Join(eventList, ","), -1)
		}

		metadataScripts = append(metadataScripts, scriptDef)
	}

	// Add the system summary table scripts
	if !noSystemSummary {
		for _, scriptName := range app.TableDefinitions[app.SystemSummaryTableName].ScriptNames {
			scriptDef := script.GetScriptByName(scriptName)
			metadataScripts = append(metadataScripts, scriptDef)
		}
	}

	return metadataScripts, nil
}

// String provides a string representation of the Metadata structure.
func (md Metadata) String() string {
	// Create a copy without PerfSupportedEvents to reduce log size
	mdCopy := md
	mdCopy.PerfSupportedEvents = ""

	jsonData, err := json.Marshal(mdCopy)
	if err != nil {
		return fmt.Sprintf("Error marshaling metadata to JSON: %v", err)
	}

	return string(jsonData)
}

// Initialized returns true if the metadata has been populated.
func (md Metadata) Initialized() bool {
	return md.SocketCount != 0 && md.CoresPerSocket != 0
}

// JSON converts the Metadata struct to a JSON-encoded byte slice.
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

// WriteJSONToFile writes the metadata structure to the filename provided.
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

// ReadJSONFromFile reads the metadata structure from the filename provided.
func ReadJSONFromFile(path string) (md Metadata, err error) {
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

// --- Common helper functions used by both X86 and ARM collectors ---

// getSystemSummary retrieves the system summary from script outputs.
func getSystemSummary(scriptOutputs map[string]script.ScriptOutput) (summaryFields [][]string, err error) {
	allTableValues, err := table.ProcessTables([]table.TableDefinition{app.TableDefinitions[app.SystemSummaryTableName]}, scriptOutputs)
	if err != nil {
		err = fmt.Errorf("failed to process script outputs: %w", err)
		return
	}
	for _, field := range allTableValues[0].Fields {
		summaryFields = append(summaryFields, []string{field.Name, field.Values[0]})
	}
	return
}

// getArchitecture retrieves the architecture from script outputs.
func getArchitecture(scriptOutputs map[string]script.ScriptOutput) (arch string, err error) {
	if scriptOutputs[scriptGetArchitecture].Exitcode != 0 {
		err = fmt.Errorf("failed to retrieve architecture: %s", scriptOutputs[scriptGetArchitecture].Stderr)
		return
	}
	arch = strings.TrimSpace(scriptOutputs[scriptGetArchitecture].Stdout)
	return
}

// getPerfSupportedEvents returns the output from 'perf list'.
func getPerfSupportedEvents(scriptOutputs map[string]script.ScriptOutput) (supportedEvents string, err error) {
	supportedEvents = scriptOutputs[scriptPerfSupportedEvents].Stdout
	if scriptOutputs[scriptPerfSupportedEvents].Exitcode != 0 {
		err = fmt.Errorf("failed to get perf supported events: %s", scriptOutputs[scriptPerfSupportedEvents].Stderr)
		return
	}
	return
}

// getKernelVersion returns the kernel version of the system.
func getKernelVersion(scriptOutputs map[string]script.ScriptOutput) (version string, err error) {
	if scriptOutputs[scriptKernelVersion].Exitcode != 0 {
		err = fmt.Errorf("failed to retrieve kernel version: %s", scriptOutputs[scriptKernelVersion].Stderr)
		return
	}
	version = strings.TrimSpace(scriptOutputs[scriptKernelVersion].Stdout)
	return
}

// getSupportsEvent checks if the event is supported by perf.
func getSupportsEvent(event string, scriptOutputs map[string]script.ScriptOutput) (supported bool, output string, err error) {
	output = scriptOutputs["perf stat "+event].Stderr
	if scriptOutputs["perf stat "+event].Exitcode != 0 {
		err = fmt.Errorf("failed to determine if %s is supported: %s", event, output)
		return
	}
	supported = !strings.Contains(output, "<not supported>")
	return
}

// getCPUInfo reads and returns all data from /proc/cpuinfo.
func getCPUInfo(t target.Target) (cpuInfo []map[string]string, err error) {
	cmd := exec.Command("cat", "/proc/cpuinfo")
	stdout, stderr, exitcode, err := t.RunCommand(cmd)
	if err != nil {
		err = fmt.Errorf("failed to execute cat command: %v", err)
		return
	}
	if exitcode != 0 {
		err = fmt.Errorf("failed to get cpuinfo: %s, exit code %d", stderr, exitcode)
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

// createCPUSocketMap creates a mapping of logical CPUs to their corresponding sockets.
func createCPUSocketMap(cpuInfo []map[string]string) (cpuSocketMap map[int]int) {
	cpuSocketMap = make(map[int]int)
	for idx := range cpuInfo {
		procID, _ := strconv.Atoi(cpuInfo[idx]["processor"])
		physID, _ := strconv.Atoi(cpuInfo[idx]["physical id"])
		cpuSocketMap[procID] = physID
	}
	return cpuSocketMap
}

// getNumGPCounters returns the number of general purpose counters for a given microarchitecture.
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

// getLscpu runs lscpu on the target and returns the output.
func getLscpu(t target.Target) (output string, err error) {
	cmd := exec.Command("lscpu")
	output, stderr, exitcode, err := t.RunCommand(cmd)
	if err != nil {
		err = fmt.Errorf("failed to execute lscpu command: %v", err)
		return
	}
	if exitcode != 0 {
		err = fmt.Errorf("failed to run lscpu: %s, exit code %d", stderr, exitcode)
		return
	}
	return
}

// parseLscpuIntField parses an integer field from lscpu output.
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

// parseLscpuStringField parses a string field from lscpu output.
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
