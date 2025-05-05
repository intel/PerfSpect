package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// metrics.go defines a structure and a loading function to hold information about the platform to be
// used during data collection and metric production

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

	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/target"

	"github.com/spf13/cobra"
)

// Metadata is the representation of the platform's state and capabilities
type Metadata struct {
	CoresPerSocket            int
	CPUSocketMap              map[int]int
	UncoreDeviceIDs           map[string][]int
	KernelVersion             string
	Architecture              string
	Vendor                    string
	Microarchitecture         string
	Hostname                  string
	ModelName                 string
	PerfSupportedEvents       string
	PMUDriverVersion          string
	SocketCount               int
	CollectionStartTime       time.Time
	SupportsInstructions      bool
	SupportsFixedCycles       bool
	SupportsFixedInstructions bool
	SupportsFixedTMA          bool
	SupportsRefCycles         bool
	SupportsUncore            bool
	SupportsPEBS              bool
	SupportsOCR               bool
	ThreadsPerCore            int
	TSC                       int
	TSCFrequencyHz            int
	SystemSummaryFields       [][]string // slice of key-value pairs
}

// LoadMetadata - populates and returns a Metadata structure containing state of the
// system.
func LoadMetadata(myTarget target.Target, noRoot bool, noSystemSummary bool, perfPath string, localTempDir string, cmd *cobra.Command) (metadata Metadata, err error) {
	// Hostname
	metadata.Hostname = myTarget.GetName()
	// CPU Info (from /proc/cpuinfo)
	var cpuInfo []map[string]string
	cpuInfo, err = getCPUInfo(myTarget)
	if err != nil || len(cpuInfo) < 1 {
		err = fmt.Errorf("failed to read cpu info: %v", err)
		return
	}
	// Core Count (per socket) (from cpuInfo)
	metadata.CoresPerSocket, err = strconv.Atoi(cpuInfo[0]["cpu cores"])
	if err != nil || metadata.CoresPerSocket == 0 {
		err = fmt.Errorf("failed to retrieve cores per socket: %v", err)
		return
	}
	// Socket Count (from cpuInfo)
	var maxPhysicalID int
	if maxPhysicalID, err = strconv.Atoi(cpuInfo[len(cpuInfo)-1]["physical id"]); err != nil {
		err = fmt.Errorf("failed to retrieve max physical id: %v", err)
		return
	}
	metadata.SocketCount = maxPhysicalID + 1
	// Hyperthreading - threads per core (from cpuInfo)
	if cpuInfo[0]["siblings"] != cpuInfo[0]["cpu cores"] {
		metadata.ThreadsPerCore = 2
	} else {
		metadata.ThreadsPerCore = 1
	}
	// CPUSocketMap (from cpuInfo)
	metadata.CPUSocketMap = createCPUSocketMap(metadata.CoresPerSocket, metadata.SocketCount, metadata.ThreadsPerCore == 2)
	// Model Name (from cpuInfo)
	metadata.ModelName = cpuInfo[0]["model name"]
	// Vendor (from cpuInfo)
	metadata.Vendor = cpuInfo[0]["vendor_id"]
	// CPU microarchitecture (from cpuInfo)
	cpu, err := report.GetCPU(cpuInfo[0]["cpu family"], cpuInfo[0]["model"], cpuInfo[0]["stepping"])
	if err != nil {
		return
	}
	metadata.Microarchitecture = cpu.MicroArchitecture
	// the rest of the metadata is retrieved by running scripts in parallel
	metadataScripts, err := getMetadataScripts(noRoot, perfPath, metadata.Microarchitecture, noSystemSummary)
	if err != nil {
		err = fmt.Errorf("failed to get metadata scripts: %v", err)
		return
	}
	// run the scripts
	scriptOutputs, err := script.RunScripts(myTarget, metadataScripts, true, localTempDir) // nosemgrep
	if err != nil {
		err = fmt.Errorf("failed to run metadata scripts: %v", err)
		return
	}
	// System Summary Values
	if !noSystemSummary {
		if metadata.SystemSummaryFields, err = getSystemSummary(scriptOutputs); err != nil {
			err = fmt.Errorf("failed to get system summary: %w", err)
			return
		}
	} else {
		metadata.SystemSummaryFields = [][]string{{"", "System Info Not Available"}}
	}
	// Architecture
	if metadata.Architecture, err = getArchitecture(scriptOutputs); err != nil {
		err = fmt.Errorf("failed to retrieve architecture: %v", err)
		return
	}
	// PMU Driver Version
	if metadata.PMUDriverVersion, err = getPMUDriverVersion(scriptOutputs); err != nil {
		err = fmt.Errorf("failed to retrieve PMU driver version: %v", err)
		return
	}
	// perf list
	if metadata.PerfSupportedEvents, err = getPerfSupportedEvents(scriptOutputs); err != nil {
		err = fmt.Errorf("failed to load perf list: %v", err)
		return
	}
	// instructions
	var output string
	if metadata.SupportsInstructions, output, err = getSupportsEvent("instructions", scriptOutputs); err != nil {
		slog.Warn("failed to determine if instructions event is supported, assuming not supported", slog.String("error", err.Error()))
		err = nil
	} else {
		if !metadata.SupportsInstructions {
			slog.Warn("instructions event not supported", slog.String("output", output))
		}
	}
	// ref_cycles
	if metadata.SupportsRefCycles, output, err = getSupportsEvent("ref-cycles", scriptOutputs); err != nil {
		slog.Warn("failed to determine if ref_cycles is supported, assuming not supported", slog.String("error", err.Error()))
		err = nil
	} else {
		if !metadata.SupportsRefCycles {
			slog.Warn("ref-cycles not supported", slog.String("output", output))
		}
	}
	// Fixed-counter TMA events
	if metadata.SupportsFixedTMA, output, err = getSupportsFixedTMA(scriptOutputs); err != nil {
		slog.Warn("failed to determine if fixed-counter TMA is supported, assuming not supported", slog.String("error", err.Error()))
		err = nil
	} else {
		if !metadata.SupportsFixedTMA {
			slog.Warn("Fixed-counter TMA events not supported", slog.String("output", output))
		}
	}
	// Fixed-counter cycles events
	if metadata.SupportsFixedCycles, output, err = getSupportsFixedEvent("cpu-cycles", scriptOutputs); err != nil {
		slog.Warn("failed to determine if fixed-counter 'cpu-cycles' is supported, assuming not supported", slog.String("error", err.Error()))
		err = nil
	} else {
		if !metadata.SupportsFixedCycles {
			slog.Warn("Fixed-counter 'cpu-cycles' events not supported", slog.String("output", output))
		}
	}
	// Fixed-counter instructions events
	if metadata.SupportsFixedInstructions, output, err = getSupportsFixedEvent("instructions", scriptOutputs); err != nil {
		slog.Warn("failed to determine if fixed-counter 'instructions' is supported, assuming not supported", slog.String("error", err.Error()))
		err = nil
	} else {
		if !metadata.SupportsFixedInstructions {
			slog.Warn("Fixed-counter 'instructions' events not supported", slog.String("output", output))
		}
	}
	// PEBS
	if metadata.SupportsPEBS, output, err = getSupportsPEBS(scriptOutputs); err != nil {
		slog.Warn("failed to determine if 'PEBS' is supported, assuming not supported", slog.String("error", err.Error()))
		err = nil
	} else {
		if !metadata.SupportsPEBS {
			slog.Warn("'PEBS' events not supported", slog.String("output", output))
		}
	}
	// Offcore response
	if metadata.SupportsOCR, output, err = getSupportsOCR(scriptOutputs); err != nil {
		slog.Warn("failed to determine if 'OCR' is supported, assuming not supported", slog.String("error", err.Error()))
		err = nil
	} else {
		if !metadata.SupportsOCR {
			slog.Warn("'OCR' events not supported", slog.String("output", output))
		}
	}
	// Kernel Version
	if metadata.KernelVersion, err = getKernelVersion(scriptOutputs); err != nil {
		err = fmt.Errorf("failed to retrieve kernel version: %v", err)
		return
	}
	// System TSC Frequency
	if metadata.TSCFrequencyHz, err = getTSCFreqHz(scriptOutputs); err != nil {
		err = fmt.Errorf("failed to retrieve TSC frequency: %v", err)
		return
	} else {
		metadata.TSC = metadata.SocketCount * metadata.CoresPerSocket * metadata.ThreadsPerCore * metadata.TSCFrequencyHz
	}
	// uncore device IDs and uncore support
	isAMDArchitecture := metadata.Vendor == "AuthenticAMD"
	if metadata.UncoreDeviceIDs, err = getUncoreDeviceIDs(isAMDArchitecture, scriptOutputs); err != nil {
		err = fmt.Errorf("failed to retrieve uncore device IDs: %v", err)
		return
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
	err = nil
	return
}

func getMetadataScripts(noRoot bool, perfPath string, uarch string, noSystemSummary bool) (metadataScripts []script.ScriptDefinition, err error) {
	// reduce startup time by running the metadata scripts in parallel
	metadataScriptDefs := []script.ScriptDefinition{
		{
			Name:           "get architecture",
			ScriptTemplate: "uname -m",
			Superuser:      !noRoot,
		},
		{
			Name:           "perf supported events",
			ScriptTemplate: perfPath + " list",
			Superuser:      !noRoot,
		},
		{
			Name:           "list uncore devices",
			ScriptTemplate: "find /sys/bus/event_source/devices/ \\( -name uncore_* -o -name amd_* \\)",
			Superuser:      !noRoot,
		},
		{
			Name:           "perf stat instructions",
			ScriptTemplate: perfPath + " stat -a -e instructions sleep 1",
			Superuser:      !noRoot,
		},
		{
			Name:           "perf stat ref-cycles",
			ScriptTemplate: perfPath + " stat -a -e ref-cycles sleep 1",
			Superuser:      !noRoot,
		},
		{
			Name:           "perf stat pebs",
			ScriptTemplate: perfPath + " stat -a -e cpu/event=0xad,umask=0x40,period=1000003,name='INT_MISC.UNKNOWN_BRANCH_CYCLES'/ sleep 1",
			Superuser:      !noRoot,
		},
		{
			Name:           "perf stat ocr",
			ScriptTemplate: perfPath + " stat -a -e cpu/event=0x2a,umask=0x01,offcore_rsp=0x104004477,name='OCR.READS_TO_CORE.LOCAL_DRAM'/ sleep 1",
			Superuser:      !noRoot,
		},
		{
			Name:           "perf stat tma",
			ScriptTemplate: perfPath + " stat -a -e '{cpu/event=0x00,umask=0x04,period=10000003,name='TOPDOWN.SLOTS'/,cpu/event=0x00,umask=0x81,period=10000003,name='PERF_METRICS.BAD_SPECULATION'/}' sleep 1",
			Superuser:      !noRoot,
		},
		{
			Name:           "perf stat fixed instructions",
			ScriptTemplate: perfPath + " stat -a -e '{{{.InstructionsList}}}' sleep 1",
			Superuser:      !noRoot,
		},
		{
			Name:           "perf stat fixed cpu-cycles",
			ScriptTemplate: perfPath + " stat -a -e '{{{.CpuCyclesList}}}' sleep 1",
			Superuser:      !noRoot,
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
		},
		{
			Name:           "kernel version",
			ScriptTemplate: "uname -r",
			Superuser:      !noRoot,
		},
	}
	// replace script template vars
	numGPCounters, err := getNumGPCounters(uarch)
	if err != nil {
		err = fmt.Errorf("failed to get number of GP counters: %v", err)
		return
	}
	for _, scriptDef := range metadataScriptDefs {
		if scriptDef.Name == "perf stat fixed instructions" {
			var eventList []string
			for range numGPCounters {
				eventList = append(eventList, "instructions")
			}
			scriptDef.ScriptTemplate = strings.Replace(scriptDef.ScriptTemplate, "{{.InstructionsList}}", strings.Join(eventList, ","), -1)
		} else if scriptDef.Name == "perf stat fixed cpu-cycles" {
			var eventList []string
			for range numGPCounters {
				eventList = append(eventList, "cpu-cycles")
			}
			scriptDef.ScriptTemplate = strings.Replace(scriptDef.ScriptTemplate, "{{.CpuCyclesList}}", strings.Join(eventList, ","), -1)
		}
		metadataScripts = append(metadataScripts, scriptDef)
	}
	// add the system summary table scripts to the list
	if !noSystemSummary {
		table := report.GetTableByName(report.BriefSysSummaryTableName)
		for _, scriptName := range table.ScriptNames {
			scriptDef := script.GetScriptByName(scriptName)
			metadataScripts = append(metadataScripts, scriptDef)
		}
	}
	return
}

// String - provides a string representation of the Metadata structure
func (md Metadata) String() string {
	out := fmt.Sprintf(""+
		"Host Name: %s, "+
		"Model Name: %s, "+
		"Architecture: %s, "+
		"Vendor: %s, "+
		"Microarchitecture: %s, "+
		"Socket Count: %d, "+
		"Cores Per Socket: %d, "+
		"Threads per Core: %d, "+
		"TSC Frequency (Hz): %d, "+
		"TSC: %d, "+
		"Instructions event supported: %t, "+
		"Fixed cycles slot supported: %t, "+
		"Fixed instructions slot supported: %t, "+
		"Fixed TMA slot supported: %t, "+
		"ref-cycles supported: %t, "+
		"Uncore supported: %t, "+
		"PEBS supported: %t, "+
		"OCR supported: %t, "+
		"PMU Driver version: %s, "+
		"Kernel version: %s, "+
		"Collection Start Time: %s, ",
		md.Hostname,
		md.ModelName,
		md.Architecture,
		md.Vendor,
		md.Microarchitecture,
		md.SocketCount,
		md.CoresPerSocket,
		md.ThreadsPerCore,
		md.TSCFrequencyHz,
		md.TSC,
		md.SupportsInstructions,
		md.SupportsFixedCycles,
		md.SupportsFixedInstructions,
		md.SupportsFixedTMA,
		md.SupportsRefCycles,
		md.SupportsUncore,
		md.SupportsPEBS,
		md.SupportsOCR,
		md.PMUDriverVersion,
		md.KernelVersion,
		md.CollectionStartTime.Format(time.RFC3339),
	)
	for deviceName, deviceIds := range md.UncoreDeviceIDs {
		var ids []string
		for _, id := range deviceIds {
			ids = append(ids, fmt.Sprintf("%d", id))
		}
		out += fmt.Sprintf("%s: [%s] ", deviceName, strings.Join(ids, ","))
	}
	return out
}

// JSON converts the Metadata struct to a JSON-encoded byte slice.
//
// Returns:
// - out: JSON-encoded byte slice representation of the Metadata.
// - err: error encountered during the marshaling process, if any.
func (md Metadata) JSON() (out []byte, err error) {
	if out, err = json.Marshal(md); err != nil {
		slog.Error("failed to marshal metadata structure", slog.String("error", err.Error()))
		return
	}
	return
}

// WriteJSONToFile writes the metadata structure (minus perf's supported events) to the filename provided
// Note that the file will be truncated.
func (md Metadata) WriteJSONToFile(path string) (err error) {
	var rawFile *os.File
	if rawFile, err = os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644); err != nil {
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
	rawBytes, err = os.ReadFile(path)
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
	var allTableValues []report.TableValues
	allTableValues, err = report.ProcessTables([]string{report.BriefSysSummaryTableName}, scriptOutputs)
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
func getCPUInfo(myTarget target.Target) (cpuInfo []map[string]string, err error) {
	cmd := exec.Command("cat", "/proc/cpuinfo")
	stdout, stderr, exitcode, err := myTarget.RunCommand(cmd, 0, true)
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
// We check for the TOPDOWN.SLOTS and PERF_METRICS.BAD_SPECULATION events as
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
	// example line: "         784333932      TOPDOWN.SLOTS                                                        (59.75%)"
	re := regexp.MustCompile(`\s+(\d+)\s+(\w*\.*\w*)\s+.*`)
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
	topDownSlots := vals["TOPDOWN.SLOTS"]
	badSpeculation := vals["PERF_METRICS.BAD_SPECULATION"]
	supported = topDownSlots != badSpeculation && topDownSlots != 0 && badSpeculation != 0
	return
}

func getNumGPCounters(uarch string) (numGPCounters int, err error) {
	shortUarch := uarch[:3]
	switch shortUarch {
	case "BDX":
		fallthrough
	case "SKX":
		fallthrough
	case "CLX":
		numGPCounters = 4
	case "ICX":
		fallthrough
	case "SPR":
		fallthrough
	case "EMR":
		fallthrough
	case "SRF":
		fallthrough
	case "CWF":
		fallthrough
	case "GNR":
		numGPCounters = 8
	case "Gen":
		fallthrough
	case "Ber":
		fallthrough
	case "Tur":
		numGPCounters = 5
	default:
		err = fmt.Errorf("unsupported uarch: %s", uarch)
		return
	}
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
// It takes a myTarget parameter of type target.Target and returns the frequency
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
// The function takes the number of cores per socket, the number of sockets, and a boolean indicating whether hyperthreading is enabled.
// It returns a map where the key is the logical CPU index and the value is the socket index.
func createCPUSocketMap(coresPerSocket int, sockets int, hyperthreading bool) (cpuSocketMap map[int]int) {
	// Create an empty map
	cpuSocketMap = make(map[int]int)

	// Calculate the total number of logical CPUs
	totalCPUs := coresPerSocket * sockets
	if hyperthreading {
		totalCPUs *= 2 // hyperthreading doubles the number of logical CPUs
	}
	// Assign each CPU to a socket
	for i := range totalCPUs {
		// Assume that the CPUs are evenly distributed between the sockets
		socket := i / coresPerSocket
		if hyperthreading {
			// With non-adjacent hyperthreading, the second logical CPU of each core is in the second half
			if i >= totalCPUs/2 {
				socket = (i - totalCPUs/2) / coresPerSocket
			}
		}
		// Store the mapping
		cpuSocketMap[i] = socket
	}
	return cpuSocketMap
}
