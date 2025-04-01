package metrics

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// defines a structure and a loading funciton to hold information about the platform to be
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

	"perfspect/internal/cpudb"
	"perfspect/internal/script"
	"perfspect/internal/target"
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
}

// LoadMetadata - populates and returns a Metadata structure containing state of the
// system.
func LoadMetadata(myTarget target.Target, noRoot bool, perfPath string, localTempDir string) (metadata Metadata, err error) {
	// CPU Info
	var cpuInfo []map[string]string
	cpuInfo, err = getCPUInfo(myTarget)
	if err != nil || len(cpuInfo) < 1 {
		err = fmt.Errorf("failed to read cpu info: %v", err)
		return
	}
	// Core Count (per socket)
	metadata.CoresPerSocket, err = strconv.Atoi(cpuInfo[0]["cpu cores"])
	if err != nil || metadata.CoresPerSocket == 0 {
		err = fmt.Errorf("failed to retrieve cores per socket: %v", err)
		return
	}
	// Socket Count
	var maxPhysicalID int
	if maxPhysicalID, err = strconv.Atoi(cpuInfo[len(cpuInfo)-1]["physical id"]); err != nil {
		err = fmt.Errorf("failed to retrieve max physical id: %v", err)
		return
	}
	metadata.SocketCount = maxPhysicalID + 1
	// Hyperthreading - threads per core
	if cpuInfo[0]["siblings"] != cpuInfo[0]["cpu cores"] {
		metadata.ThreadsPerCore = 2
	} else {
		metadata.ThreadsPerCore = 1
	}
	// CPUSocketMap
	metadata.CPUSocketMap = createCPUSocketMap(metadata.CoresPerSocket, metadata.SocketCount, metadata.ThreadsPerCore == 2)
	// Model Name
	metadata.ModelName = cpuInfo[0]["model name"]
	// Hostname
	metadata.Hostname = myTarget.GetName()
	// Architecture
	metadata.Architecture, err = myTarget.GetArchitecture()
	if err != nil {
		err = fmt.Errorf("failed to retrieve architecture: %v", err)
		return
	}
	// Vendor
	metadata.Vendor = cpuInfo[0]["vendor_id"]
	// CPU microarchitecture
	cpuDb := cpudb.NewCPUDB()
	cpu, err := cpuDb.GetCPU(cpuInfo[0]["cpu family"], cpuInfo[0]["model"], cpuInfo[0]["stepping"])
	if err != nil {
		return
	}
	metadata.Microarchitecture = cpu.MicroArchitecture

	// PMU driver version
	metadata.PMUDriverVersion, err = getPMUDriverVersion(myTarget, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to retrieve PMU driver version: %v", err)
		return
	}
	// reduce startup time by running the perf commands in their own threads
	slowFuncChannel := make(chan error)
	// perf list
	go func() {
		var err error
		if metadata.PerfSupportedEvents, err = getPerfSupportedEvents(myTarget, perfPath); err != nil {
			err = fmt.Errorf("failed to load perf list: %v", err)
		}
		slowFuncChannel <- err
	}()
	// instructions
	go func() {
		var err error
		var output string
		if metadata.SupportsInstructions, output, err = getSupportsEvent(myTarget, "instructions", noRoot, perfPath, localTempDir); err != nil {
			slog.Warn("failed to determine if instructions event is supported, assuming not supported", slog.String("error", err.Error()))
			err = nil
		} else {
			if !metadata.SupportsInstructions {
				slog.Warn("instructions event not supported", slog.String("output", output))
			}
		}
		slowFuncChannel <- err
	}()
	// ref_cycles
	go func() {
		var err error
		var output string
		if metadata.SupportsRefCycles, output, err = getSupportsEvent(myTarget, "ref-cycles", noRoot, perfPath, localTempDir); err != nil {
			slog.Warn("failed to determine if ref_cycles is supported, assuming not supported", slog.String("error", err.Error()))
			err = nil
		} else {
			if !metadata.SupportsRefCycles {
				slog.Warn("ref-cycles not supported", slog.String("output", output))
			}
		}
		slowFuncChannel <- err
	}()
	// Fixed-counter TMA events
	go func() {
		var err error
		var output string
		if metadata.SupportsFixedTMA, output, err = getSupportsFixedTMA(myTarget, noRoot, perfPath, localTempDir); err != nil {
			slog.Warn("failed to determine if fixed-counter TMA is supported, assuming not supported", slog.String("error", err.Error()))
			err = nil
		} else {
			if !metadata.SupportsFixedTMA {
				slog.Warn("Fixed-counter TMA events not supported", slog.String("output", output))
			}
		}
		slowFuncChannel <- err
	}()
	// Fixed-counter cycles events
	go func() {
		var err error
		var output string
		if metadata.SupportsFixedCycles, output, err = getSupportsFixedEvent(myTarget, "cpu-cycles", cpu.MicroArchitecture, noRoot, perfPath, localTempDir); err != nil {
			slog.Warn("failed to determine if fixed-counter 'cpu-cycles' is supported, assuming not supported", slog.String("error", err.Error()))
			err = nil
		} else {
			if !metadata.SupportsFixedCycles {
				slog.Warn("Fixed-counter 'cpu-cycles' events not supported", slog.String("output", output))
			}
		}
		slowFuncChannel <- err
	}()
	// Fixed-counter instructions events
	go func() {
		var err error
		var output string
		if metadata.SupportsFixedInstructions, output, err = getSupportsFixedEvent(myTarget, "instructions", cpu.MicroArchitecture, noRoot, perfPath, localTempDir); err != nil {
			slog.Warn("failed to determine if fixed-counter 'instructions' is supported, assuming not supported", slog.String("error", err.Error()))
			err = nil
		} else {
			if !metadata.SupportsFixedInstructions {
				slog.Warn("Fixed-counter 'instructions' events not supported", slog.String("output", output))
			}
		}
		slowFuncChannel <- err
	}()
	// PEBS
	go func() {
		var err error
		var output string
		if metadata.SupportsPEBS, output, err = getSupportsPEBS(myTarget, noRoot, perfPath, localTempDir); err != nil {
			slog.Warn("failed to determine if 'PEBS' is supported, assuming not supported", slog.String("error", err.Error()))
			err = nil
		} else {
			if !metadata.SupportsPEBS {
				slog.Warn("'PEBS' events not supported", slog.String("output", output))
			}
		}
		slowFuncChannel <- err
	}()
	// Offcore response
	go func() {
		var err error
		var output string
		if metadata.SupportsOCR, output, err = getSupportsOCR(myTarget, noRoot, perfPath, localTempDir); err != nil {
			slog.Warn("failed to determine if 'OCR' is supported, assuming not supported", slog.String("error", err.Error()))
			err = nil
		} else {
			if !metadata.SupportsOCR {
				slog.Warn("'OCR' events not supported", slog.String("output", output))
			}
		}
		slowFuncChannel <- err
	}()
	defer func() {
		var errs []error
		errs = append(errs, <-slowFuncChannel)
		errs = append(errs, <-slowFuncChannel)
		errs = append(errs, <-slowFuncChannel)
		errs = append(errs, <-slowFuncChannel)
		errs = append(errs, <-slowFuncChannel)
		errs = append(errs, <-slowFuncChannel)
		errs = append(errs, <-slowFuncChannel)
		errs = append(errs, <-slowFuncChannel)
		for _, errInside := range errs {
			if errInside != nil {
				slog.Error("error loading metadata", slog.String("error", errInside.Error()), slog.String("target", myTarget.GetName()))
				err = fmt.Errorf("target not supported, see log for details")
			}
		}
	}()
	// System TSC Frequency
	metadata.TSCFrequencyHz, err = getTSCFreqHz(myTarget, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to retrieve TSC frequency: %v", err)
		return
	}
	// calculate TSC
	metadata.TSC = metadata.SocketCount * metadata.CoresPerSocket * metadata.ThreadsPerCore * metadata.TSCFrequencyHz
	// uncore device IDs
	if metadata.UncoreDeviceIDs, err = getUncoreDeviceIDs(myTarget, localTempDir); err != nil {
		return
	}
	for uncoreDeviceName := range metadata.UncoreDeviceIDs {
		if uncoreDeviceName == "cha" || uncoreDeviceName == "l3" || uncoreDeviceName == "df" { // could be any uncore device
			metadata.SupportsUncore = true
			break
		}
	}
	if !metadata.SupportsUncore {
		slog.Warn("Uncore devices not supported")
	}
	// Kernel Version
	metadata.KernelVersion, err = getKernelVersion(myTarget, localTempDir)
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
// It creates a copy of the Metadata, sets the PerfSupportedEvents field to an empty string,
// and then marshals the copy to JSON format.
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

// getUncoreDeviceIDs - returns a map of device type to list of device indices
// e.g., "upi" -> [0,1,2,3],
func getUncoreDeviceIDs(myTarget target.Target, localTempDir string) (IDs map[string][]int, err error) {
	scriptDef := script.ScriptDefinition{
		Name:           "list uncore devices",
		ScriptTemplate: "find /sys/bus/event_source/devices/ \\( -name uncore_* -o -name amd_* \\)",
		Superuser:      false,
	}
	scriptOutput, err := script.RunScript(myTarget, scriptDef, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to list uncore devices: %s, %d, %v", scriptOutput.Stderr, scriptOutput.Exitcode, err)
		return
	}
	fileNames := strings.Split(scriptOutput.Stdout, "\n")
	IDs = make(map[string][]int)
	re := regexp.MustCompile(`(?:uncore_|amd_)(.*?)(?:_(\d+))?$`)
	for _, fileName := range fileNames {
		match := re.FindStringSubmatch(fileName)
		if match == nil {
			continue
		}
		var id int
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
func getPerfSupportedEvents(myTarget target.Target, perfPath string) (supportedEvents string, err error) {
	cmd := exec.Command(perfPath, "list")
	stdout, stderr, exitcode, err := myTarget.RunCommand(cmd, 0, true)
	if err != nil {
		err = fmt.Errorf("failed to get perf list: %s, %d, %v", stderr, exitcode, err)
		return
	}
	supportedEvents = stdout
	return
}

// getSupportsEvent() - checks if the event is supported by perf
func getSupportsEvent(myTarget target.Target, event string, noRoot bool, perfPath string, localTempDir string) (supported bool, output string, err error) {
	scriptDef := script.ScriptDefinition{
		Name:           "perf stat " + event,
		ScriptTemplate: perfPath + " stat -a -e " + event + " sleep 1",
		Superuser:      !noRoot,
	}
	scriptOutput, err := script.RunScript(myTarget, scriptDef, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to determine if %s is supported: %s, %d, %v", event, scriptOutput.Stderr, scriptOutput.Exitcode, err)
		return
	}
	supported = !strings.Contains(scriptOutput.Stderr, "<not supported>")
	return
}

// getSupportsPEBS() - checks if the PEBS events are supported on the target
// On some VMs, e.g. GCP C4, PEBS events are not supported and perf returns '<not supported>'
// Events that use MSR 0x3F7 are PEBS events. We use the INT_MISC.UNKNOWN_BRANCH_CYCLES event since
// it is a PEBS event that we used in EMR metrics.
func getSupportsPEBS(myTarget target.Target, noRoot bool, perfPath string, localTempDir string) (supported bool, output string, err error) {
	scriptDef := script.ScriptDefinition{
		Name:           "perf stat pebs",
		ScriptTemplate: perfPath + " stat -a -e cpu/event=0xad,umask=0x40,period=1000003,name='INT_MISC.UNKNOWN_BRANCH_CYCLES'/ sleep 1",
		Superuser:      !noRoot,
	}
	scriptOutput, err := script.RunScript(myTarget, scriptDef, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to determine if pebs is supported: %s, %d, %v", scriptOutput.Stderr, scriptOutput.Exitcode, err)
		return
	}
	supported = !strings.Contains(scriptOutput.Stderr, "<not supported>")
	return
}

// getSupportsOCR() - checks if the offcore response events are supported on the target
// On some VMs, e.g. GCP C4, offcore response events are not supported and perf returns '<not supported>'
func getSupportsOCR(myTarget target.Target, noRoot bool, perfPath string, localTempDir string) (supported bool, output string, err error) {
	scriptDef := script.ScriptDefinition{
		Name:           "perf stat ocr",
		ScriptTemplate: perfPath + " stat -a -e cpu/event=0x2a,umask=0x01,offcore_rsp=0x104004477,name='OCR.READS_TO_CORE.LOCAL_DRAM'/ sleep 1",
		Superuser:      !noRoot,
	}
	scriptOutput, err := script.RunScript(myTarget, scriptDef, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to determine if ocr is supported: %s, %d, %v", scriptOutput.Stderr, scriptOutput.Exitcode, err)
		return
	}
	supported = !strings.Contains(scriptOutput.Stderr, "<not supported>")
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
func getSupportsFixedTMA(myTarget target.Target, noRoot bool, perfPath string, localTempDir string) (supported bool, output string, err error) {
	scriptDef := script.ScriptDefinition{
		Name:           "perf stat tma",
		ScriptTemplate: perfPath + " stat -a -e '{cpu/event=0x00,umask=0x04,period=10000003,name='TOPDOWN.SLOTS'/,cpu/event=0x00,umask=0x81,period=10000003,name='PERF_METRICS.BAD_SPECULATION'/}' sleep 1",
		Superuser:      !noRoot,
	}
	scriptOutput, err := script.RunScript(myTarget, scriptDef, localTempDir)
	if err != nil {
		// err from perf stat is 1st indication that these events are not supported, so return a nil error
		supported = false
		output = fmt.Sprint(err)
		err = nil
		return
	}
	// event values being zero or equal to each other is 2nd indication that these events are not (properly) supported
	output = scriptOutput.Stderr
	vals := make(map[string]float64)
	lines := strings.Split(scriptOutput.Stderr, "\n")
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

func getSupportsFixedEvent(myTarget target.Target, event string, uarch string, noRoot bool, perfPath string, localTempDir string) (supported bool, output string, err error) {
	shortUarch := uarch[:3]
	var numGPCounters int
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
	var eventList []string
	for range numGPCounters {
		eventList = append(eventList, event)
	}
	scriptDef := script.ScriptDefinition{
		Name:           "perf stat fixed" + event,
		ScriptTemplate: perfPath + " stat -a -e '{" + strings.Join(eventList, ",") + "}' sleep 1",
		Superuser:      !noRoot,
	}
	scriptOutput, err := script.RunScript(myTarget, scriptDef, localTempDir)
	if err != nil {
		supported = false
		return
	}
	output = scriptOutput.Stderr
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
func getPMUDriverVersion(myTarget target.Target, localTempDir string) (version string, err error) {
	scriptDef := script.ScriptDefinition{
		Name:           "pmu driver version",
		ScriptTemplate: "dmesg | grep -A 1 \"Intel PMU driver\" | tail -1 | awk '{print $NF}'",
		Superuser:      true,
	}
	output, err := script.RunScript(myTarget, scriptDef, localTempDir)
	if err != nil {
		return
	}
	version = strings.TrimSpace(output.Stdout)
	return
}

// getTSCFreqHz returns the frequency of the Time Stamp Counter (TSC) in hertz.
// It takes a myTarget parameter of type target.Target and returns the frequency
// in hertz and an error if any.
func getTSCFreqHz(myTarget target.Target, localTempDir string) (freqHz int, err error) {
	// run tsc app on target to get TSC Frequency in MHz
	scriptDef := script.ScriptDefinition{
		Name:           "tsc",
		ScriptTemplate: "tsc",
		Depends:        []string{"tsc"},
	}
	output, err := script.RunScript(myTarget, scriptDef, localTempDir)
	if err != nil {
		return
	}
	freqMhz, err := strconv.Atoi(output.Stdout)
	if err != nil {
		return
	}
	// convert MHz to Hz
	freqHz = freqMhz * 1000000
	return
}

func getKernelVersion(myTarget target.Target, localTempDir string) (version string, err error) {
	scriptDef := script.ScriptDefinition{
		Name:           "kernel version",
		ScriptTemplate: "uname -r",
	}
	output, err := script.RunScript(myTarget, scriptDef, localTempDir)
	if err != nil {
		return
	}
	version = strings.TrimSpace(output.Stdout)
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
