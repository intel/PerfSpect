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
// The string in quotes results determines how the script is named in /tmp/perfspect.tmp.*/
// e.g. get architecture becomes /tmp/perfspect.tmp.*/get_architecture.sh
const (
	scriptGetArchitecture        = "get architecture"
	scriptPerfSupportedEvents    = "perf supported events"
	scriptPerfAllSupportedEvents = "perf all supported events"
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
	scriptPerfStatAMDUncoreProbe = "perf stat amd uncore probe"
	scriptPMUEnumerationSafety   = "pmu enumeration safety"
)

// Some virtualized guests are given a PMU that advertises a fixed counter the kernel
// cannot use, and enumerating events on one faults the kernel outright. Observed on AWS
// m6i.16xlarge (Ice Lake guest, kernel 6.14.0-1015-aws): the guest reports 4
// fixed-purpose counters, which includes counter 3, TOPDOWN.SLOTS, while lacking
// GLOBAL_CTRL_EN_PERF_METRICS -- so opening an event that lands on that counter gives
//
//	Oops: general protection fault, maybe for address 0x1
//	x86_perf_event_update+0x48 <- intel_pmu_set_period <- x86_pmu_start
//	  <- x86_pmu_enable <- __perf_event_enable <- _perf_ioctl
//
// The faulting task then dies inside x86_pmu_enable still holding perf's context lock
// and parks in D state in perf_event_release_kernel, after which every perf_event_open
// on the machine blocks in account_event/perf_event_alloc, in D state, ignoring SIGKILL.
// RCU stalls and soft lockups follow and the instance leaves the network. No shell-level
// timeout can contain that: a D-state task does not take signals.
//
// 'perf list' reaches the fault because it calls perf_event_open per candidate event to
// test support, so it is metadata collection -- not metric collection -- that takes the
// machine down. This probe decides whether that is a risk here, and it reads only dmesg
// and sysfs: it opens no perf events, so it is safe on every target.
//
// The predicate is the incoherence itself, not a model or instance-type list: 4 or more
// fixed-purpose counters advertised while the kernel exposes no slots/topdown event.
// Every comparable cell is coherent one way or the other and is left alone -- bare-metal
// Ice Lake exposes slots and passes, m7i.24xlarge advertises 2 fixed counters, and
// m5.24xlarge is Skylake with no slots counter to advertise.
var pmuEnumerationSafetyScript = script.ScriptDefinition{
	Name: scriptPMUEnumerationSafety,
	ScriptTemplate: `fixed=$(dmesg 2>/dev/null | grep -oE 'fixed-purpose events:[[:space:]]*[0-9]+' | grep -oE '[0-9]+$' | tail -1)
slots=no
# Unmatched globs stay literal here, and [[ -e ]] on a literal is false, so a machine
# with no such event correctly reports "no" rather than matching the pattern itself.
for event in /sys/bus/event_source/devices/cpu*/events/slots \
	/sys/bus/event_source/devices/cpu*/events/topdown-*; do
	[[ -e "$event" ]] && slots=yes
done
echo "fixed_purpose_counters=${fixed:-unknown}"
echo "slots_event_exposed=$slots"
`,
	Architectures: []string{cpus.X86Architecture},
}

// Event enumeration from sysfs, for targets where asking perf to do it would fault the
// kernel (see pmuEnumerationSafetyScript). Reading sysfs opens no events, so it cannot
// fault, and it has a second property that matters more than being safe: sysfs is the
// kernel's own list of events it will accept. Anything absent from it -- including the
// topdown events at the root of the fault -- is then dropped from the metric definitions
// by the existing IsCollectable checks, so metric collection cannot walk into the fault
// later either. The event set is narrower than 'perf list' would report, which is the
// price of collecting anything at all on such a target.
//
// Output must match the shape 'perf list --json' is parsed into: one event name per
// line, core events bare and the rest as perf spells them, "pmu/event/".
const sysfsEventEnumerationPreamble = `# See pmuEnumerationSafetyScript in cmd/metrics/metadata.go for why this reads sysfs
# rather than asking perf to enumerate events.
for events_dir in /sys/bus/event_source/devices/*/events; do
	[[ -d "$events_dir" ]] || continue
	pmu=${events_dir%/events}
	pmu=${pmu##*/}
	for event_path in "$events_dir"/*; do
		[[ -f "$event_path" ]] || continue
		event=${event_path##*/}
		# Sibling metadata files, not events in their own right.
		case "$event" in
		*.scale | *.unit | *.snapshot | *.per-pkg) continue ;;
		esac
`

// Hardware events plus cstate and power events: the same selection the awk filter makes
// from 'perf list --json' output.
const sysfsSupportedEventsScript = sysfsEventEnumerationPreamble + `		case "$pmu" in
		cpu | cpu_core | cpu_atom) echo "$event" ;;
		cstate_core | cstate_pkg | power) echo "$pmu/$event/" ;;
		esac
	done
done
`

// Every event the kernel publishes, from every PMU, unfiltered.
const sysfsAllSupportedEventsScript = sysfsEventEnumerationPreamble + `		case "$pmu" in
		cpu | cpu_core | cpu_atom) echo "$event" ;;
		*) echo "$pmu/$event/" ;;
		esac
	done
done
`

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
	PerfAllSupportedEvents    string
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
	CollectionInterval  time.Duration
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
		Name:           scriptGetArchitecture,
		ScriptTemplate: "uname -m",
	},
	{
		Name: scriptPerfAllSupportedEvents,
		ScriptTemplate: `# Parse perf list JSON output to extract Hardware events and cstate/power events
perf list --json 2>/dev/null | awk '
BEGIN {
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

# At end of object (closing brace), check if we should print
/^}/ {

	if (event_name != "") {
		print event_name
	}
    # Reset for next object
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
		Name:           scriptPerfStatAMDUncoreProbe,
		ScriptTemplate: `perf stat -a -e "l3/event=0x4,umask=0xff,enallcores=0x1,enallslices=0x1,threadmask=0x3,name='l3_lookup_state.all_coherent_accesses_to_l3'/" sleep 1`,
		Architectures:  []string{cpus.X86Architecture},
		Vendors:        []string{cpus.AMDVendor},
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
//
// enumerateEventsFromSysfs replaces the two 'perf list --json' scripts with sysfs reads,
// for targets where letting perf enumerate events would fault the kernel. Both are
// replaced: they run in the same concurrent batch, so leaving either one is enough to
// take the machine down. See pmuEnumerationSafetyScript.
func getMetadataScripts(noRoot bool, noSystemSummary bool, numGPCounters int, enumerateEventsFromSysfs bool) ([]script.ScriptDefinition, error) {
	metadataScripts := make([]script.ScriptDefinition, 0, len(baseMetadataScripts))

	// Copy base scripts and apply settings
	for _, baseDef := range baseMetadataScripts {
		scriptDef := baseDef
		scriptDef.Superuser = !noRoot

		if enumerateEventsFromSysfs {
			switch scriptDef.Name {
			case scriptPerfSupportedEvents:
				scriptDef.ScriptTemplate = sysfsSupportedEventsScript
				scriptDef.Depends = nil // sysfs enumeration does not need perf
			case scriptPerfAllSupportedEvents:
				scriptDef.ScriptTemplate = sysfsAllSupportedEventsScript
				scriptDef.Depends = nil
			}
		}

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

// perfEnumerationFaultsKernel reports whether asking perf to enumerate events on this
// target can be expected to fault the kernel, and if so a reason worth logging. It must
// be called before the metadata scripts run, because the command that faults is one of
// them. See pmuEnumerationSafetyScript for the fault itself.
//
// Fails open. When the answer cannot be established -- the probe did not run, dmesg is
// restricted, the fields are missing -- this reports false and the caller keeps perf
// enumeration, because wrongly reporting true would narrow a healthy target's event set
// for no reason. The cost of failing open is that a target with restricted dmesg and this
// specific broken PMU is still exposed, which is the narrower of the two risks.
func perfEnumerationFaultsKernel(t target.Target, localTempDir string, noRoot bool) (bool, string) {
	scriptDef := pmuEnumerationSafetyScript
	scriptDef.Superuser = !noRoot
	scriptOutput, err := script.RunScript(t, scriptDef, localTempDir)
	if err != nil {
		slog.Debug("could not check whether perf event enumeration is safe; assuming it is",
			slog.String("error", err.Error()))
		return false, ""
	}
	return perfEnumerationFaultsKernelFromOutput(scriptOutput.Stdout)
}

// perfEnumerationFaultsKernelFromOutput holds the decision itself, separately from
// running the probe, so it can be tested against the capability data from real targets.
func perfEnumerationFaultsKernelFromOutput(stdout string) (bool, string) {
	var fixedCounters, slotsExposed string
	for _, line := range strings.Split(stdout, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found {
			continue
		}
		switch key {
		case "fixed_purpose_counters":
			fixedCounters = value
		case "slots_event_exposed":
			slotsExposed = value
		}
	}
	numFixedCounters, err := strconv.Atoi(fixedCounters)
	if err != nil {
		slog.Debug("PMU fixed counter count unavailable; assuming perf event enumeration is safe",
			slog.String("fixed_purpose_counters", fixedCounters))
		return false, ""
	}
	// Counter 3 is TOPDOWN.SLOTS, so a count of 4 or more advertises it. The kernel
	// exposing no slots or topdown event means it knows the counter is unusable here --
	// while perf will still happily program it from its own event tables.
	if numFixedCounters >= 4 && slotsExposed == "no" {
		return true, fmt.Sprintf("PMU advertises %d fixed-purpose counters, including TOPDOWN.SLOTS, "+
			"but the kernel exposes no slots or topdown event; enumerating events with perf would fault the kernel",
			numFixedCounters)
	}
	return false, ""
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

// getPerfAllSupportedEvents returns the output from 'perf list'.
func getPerfAllSupportedEvents(scriptOutputs map[string]script.ScriptOutput) (supportedEvents string, err error) {
	supportedEvents = scriptOutputs[scriptPerfAllSupportedEvents].Stdout
	if scriptOutputs[scriptPerfAllSupportedEvents].Exitcode != 0 {
		err = fmt.Errorf("failed to get all perf supported events: %s", scriptOutputs[scriptPerfAllSupportedEvents].Stderr)
		return
	}
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
