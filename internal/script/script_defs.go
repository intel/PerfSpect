package script

import (
	"bytes"
	"fmt"
	"strings"
	texttemplate "text/template"
)

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// script_defs.go defines the bash scripts that are used to collect information from target systems

type ScriptDefinition struct {
	Name           string   // just a name
	ScriptTemplate string   // the bash script that will be run
	Architectures  []string // architectures, i.e., x86_64, arm64. If empty, it will run on all architectures.
	Families       []string // families, e.g., 6, 7. If empty, it will run on all families.
	Models         []string // models, e.g., 62, 63. If empty, it will run on all models.
	Lkms           []string // loadable kernel modules
	Depends        []string // binary dependencies that must be available for the script to run
	Superuser      bool     // requires sudo or root
	Sequential     bool     // run script sequentially (not at the same time as others)
	NeedsKill      bool     // process/script needs to be killed after run without a duration specified, i.e., it doesn't stop through SIGINT
}

const (
	HostnameScriptName                          = "hostname"
	DateScriptName                              = "date"
	DmidecodeScriptName                         = "dmidecode"
	LscpuScriptName                             = "lscpu"
	LspciBitsScriptName                         = "lspci bits"
	LspciDevicesScriptName                      = "lspci devices"
	LspciVmmScriptName                          = "lspci vmm"
	UnameScriptName                             = "uname"
	ProcCmdlineScriptName                       = "proc cmdline"
	ProcCpuinfoScriptName                       = "proc cpuinfo"
	EtcReleaseScriptName                        = "etc release"
	GccVersionScriptName                        = "gcc version"
	BinutilsVersionScriptName                   = "binutils version"
	GlibcVersionScriptName                      = "glibc version"
	PythonVersionScriptName                     = "python version"
	Python3VersionScriptName                    = "python3 version"
	JavaVersionScriptName                       = "java version"
	OpensslVersionScriptName                    = "openssl version"
	CpuidScriptName                             = "cpuid"
	BaseFrequencyScriptName                     = "base frequency"
	MaximumFrequencyScriptName                  = "maximum frequency"
	ScalingDriverScriptName                     = "scaling driver"
	ScalingGovernorScriptName                   = "scaling governor"
	MaxCStateScriptName                         = "max c-state"
	CstatesScriptName                           = "c-states"
	SpecTurboFrequenciesScriptName              = "spec turbo frequencies"
	SpecTurboCoresScriptName                    = "spec turbo cores"
	PPINName                                    = "ppin"
	PrefetchControlName                         = "prefetch control"
	PrefetchersName                             = "prefetchers"
	L3WaySizeName                               = "l3 way size"
	PackagePowerLimitName                       = "package power limit"
	EpbOSScriptName                             = "energy performance bias OS"
	EpbBIOSScriptName                           = "energy performance bias BIOS"
	EpbSourceScriptName                         = "energy performance bias source"
	EppScriptName                               = "energy performance preference"
	EppValidScriptName                          = "epp valid"
	EppPackageControlScriptName                 = "epp package control"
	EppPackageScriptName                        = "energy performance preference package"
	IaaDevicesScriptName                        = "iaa devices"
	DsaDevicesScriptName                        = "dsa devices"
	LshwScriptName                              = "lshw"
	MemoryBandwidthAndLatencyScriptName         = "memory bandwidth and latency"
	NumaBandwidthScriptName                     = "numa bandwidth"
	CpuSpeedScriptName                          = "cpu speed"
	TurboFrequenciesScriptName                  = "turbo frequencies"
	TurboFrequencyPowerAndTemperatureScriptName = "turbo frequency power and temperature"
	IdlePowerScriptName                         = "idle power"
	StoragePerfScriptName                       = "storage perf"
	MpstatScriptName                            = "mpstat"
	IostatScriptName                            = "iostat"
	SarMemoryScriptName                         = "sar-memory"
	SarNetworkScriptName                        = "sar-network"
	TurbostatScriptName                         = "turbostat"
	UncoreMaxFromMSRScriptName                  = "uncore max from msr"
	UncoreMinFromMSRScriptName                  = "uncore min from msr"
	UncoreMaxFromTPMIScriptName                 = "uncore max from tpmi"
	UncoreMinFromTPMIScriptName                 = "uncore min from tpmi"
	UncoreDieTypesFromTPMIScriptName            = "uncore die types from tpmi"
	ElcScriptName                               = "efficiency latency control"
	SstTfHighPriorityCoreFrequenciesScriptName  = "sst tf high priority core frequencies"
	SstTfLowPriorityCoreFrequenciesScriptName   = "sst tf low priority core frequencies"
	ChaCountScriptName                          = "cha count"
	MeminfoScriptName                           = "meminfo"
	TransparentHugePagesScriptName              = "transparent huge pages"
	NumaBalancingScriptName                     = "numa balancing"
	NicInfoScriptName                           = "nic info"
	DiskInfoScriptName                          = "disk info"
	HdparmScriptName                            = "hdparm"
	DfScriptName                                = "df"
	FindMntScriptName                           = "findmnt"
	CveScriptName                               = "cve"
	ProcessListScriptName                       = "process list"
	IpmitoolSensorsScriptName                   = "ipmitool sensors"
	IpmitoolChassisScriptName                   = "ipmitool chassis"
	IpmitoolEventsScriptName                    = "ipmitool events"
	IpmitoolEventTimeScriptName                 = "ipmitool event time"
	KernelLogScriptName                         = "kernel log"
	PMUDriverVersionScriptName                  = "pmu driver version"
	PMUBusyScriptName                           = "pmu busy"
	ProfileJavaScriptName                       = "profile java"
	ProfileSystemScriptName                     = "profile system"
	ProfileKernelLockScriptName                 = "profile kernel lock"
	GaudiInfoScriptName                         = "gaudi info"
	GaudiFirmwareScriptName                     = "gaudi firmware"
	GaudiNumaScriptName                         = "gaudi numa"
	InstructionMixScriptName                    = "instruction mix"
)

const (
	x86_64 = "x86_64"
)

// GetScriptByName returns the script definition with the given name. It will panic if the script is not found.
func GetScriptByName(name string) ScriptDefinition {
	return GetParameterizedScriptByName(name, ScriptParams{})
}

type ScriptParams struct {
	Duration   int
	Interval   int
	Frequency  int
	StorageDir string
	PID        int
	Filter     []string
}

type ScriptTemplateParams struct {
	Duration   string
	Interval   string
	Frequency  string
	StorageDir string
	PID        string
	Filter     string
	ScriptName string
}

// GetParameterizedScriptByName returns the script definition with the given name. It will panic if the script is not found.
func GetParameterizedScriptByName(name string, params ScriptParams) ScriptDefinition {
	// if the script doesn't exist, panic
	if _, ok := scripts[name]; !ok {
		panic("script not found: " + name)
	}
	// convert ScriptParams to ScriptTemplateParams
	templateParams := ScriptTemplateParams{
		Duration:   fmt.Sprintf("%d", params.Duration),
		Interval:   fmt.Sprintf("%d", params.Interval),
		Frequency:  fmt.Sprintf("%d", params.Frequency),
		StorageDir: params.StorageDir,
		PID:        fmt.Sprintf("%d", params.PID),
		Filter:     strings.Join(params.Filter, " "),
		ScriptName: sanitizeScriptName(name),
	}
	scriptTemplate := texttemplate.Must(texttemplate.New("scriptTemplate").Parse(scripts[name].ScriptTemplate))
	buf := new(bytes.Buffer)
	err := scriptTemplate.Execute(buf, templateParams)
	if err != nil {
		panic(err)
	}
	ScriptDefinition := scripts[name]
	ScriptDefinition.ScriptTemplate = buf.String()
	return ScriptDefinition
}

// script definitions
var scripts = map[string]ScriptDefinition{
	// configuration scripts
	HostnameScriptName: {
		Name:           HostnameScriptName,
		ScriptTemplate: "hostname",
	},
	DateScriptName: {
		Name:           DateScriptName,
		ScriptTemplate: "date",
	},
	DmidecodeScriptName: {
		Name:           DmidecodeScriptName,
		ScriptTemplate: "dmidecode",
		Superuser:      true,
		Depends:        []string{"dmidecode"},
	},
	LscpuScriptName: {
		Name:           LscpuScriptName,
		ScriptTemplate: "lscpu",
	},
	LspciBitsScriptName: {
		Name:           LspciBitsScriptName,
		ScriptTemplate: `lspci -s $(lspci | grep 325b | awk 'NR==1{{"{"}}print $1{{"}"}}') -xxx |  awk '$1 ~ /^90/{{"{"}}print $9 $8 $7 $6; exit{{"}"}}'`,
		Families:       []string{"6"},                 // Intel
		Models:         []string{"143", "207", "173"}, // SPR, EMR, GNR
		Superuser:      true,
		Depends:        []string{"lspci"},
	},
	LspciDevicesScriptName: {
		Name:           LspciDevicesScriptName,
		ScriptTemplate: "lspci -d 8086:3258 | wc -l",
		Families:       []string{"6"},                 // Intel
		Models:         []string{"143", "207", "173"}, // SPR, EMR, GNR
		Depends:        []string{"lspci"},
	},
	LspciVmmScriptName: {
		Name:           LspciVmmScriptName,
		ScriptTemplate: "lspci -vmm",
		Depends:        []string{"lspci"},
	},
	UnameScriptName: {
		Name:           UnameScriptName,
		ScriptTemplate: "uname -a",
	},
	ProcCmdlineScriptName: {
		Name:           ProcCmdlineScriptName,
		ScriptTemplate: "cat /proc/cmdline",
	},
	ProcCpuinfoScriptName: {
		Name:           ProcCpuinfoScriptName,
		ScriptTemplate: "cat /proc/cpuinfo",
	},
	EtcReleaseScriptName: {
		Name:           EtcReleaseScriptName,
		ScriptTemplate: "cat /etc/*-release",
	},
	GccVersionScriptName: {
		Name:           GccVersionScriptName,
		ScriptTemplate: "gcc --version",
	},
	BinutilsVersionScriptName: {
		Name:           BinutilsVersionScriptName,
		ScriptTemplate: "ld -v",
	},
	GlibcVersionScriptName: {
		Name:           GlibcVersionScriptName,
		ScriptTemplate: "ldd --version",
	},
	PythonVersionScriptName: {
		Name:           PythonVersionScriptName,
		ScriptTemplate: "python --version 2>&1",
	},
	Python3VersionScriptName: {
		Name:           Python3VersionScriptName,
		ScriptTemplate: "python3 --version",
	},
	JavaVersionScriptName: {
		Name:           JavaVersionScriptName,
		ScriptTemplate: "java -version 2>&1",
	},
	OpensslVersionScriptName: {
		Name:           OpensslVersionScriptName,
		ScriptTemplate: "openssl version",
	},
	CpuidScriptName: {
		Name:           CpuidScriptName,
		ScriptTemplate: "cpuid -1",
		Lkms:           []string{"cpuid"},
		Depends:        []string{"cpuid"},
		Superuser:      true,
	},
	BaseFrequencyScriptName: {
		Name:           BaseFrequencyScriptName,
		ScriptTemplate: "cat /sys/devices/system/cpu/cpu0/cpufreq/base_frequency",
	},
	MaximumFrequencyScriptName: {
		Name:           MaximumFrequencyScriptName,
		ScriptTemplate: "cat /sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq",
	},
	ScalingDriverScriptName: {
		Name:           ScalingDriverScriptName,
		ScriptTemplate: "cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_driver",
	},
	ScalingGovernorScriptName: {
		Name:           ScalingGovernorScriptName,
		ScriptTemplate: "cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor",
	},
	MaxCStateScriptName: {
		Name:           MaxCStateScriptName,
		ScriptTemplate: "cat /sys/module/intel_idle/parameters/max_cstate",
	},
	CstatesScriptName: {
		Name: CstatesScriptName,
		ScriptTemplate: `# Directory where C-state information is stored
cstate_dir="/sys/devices/system/cpu/cpu0/cpuidle"

# Check if the directory exists
if [ -d "$cstate_dir" ]; then
	for state in "$cstate_dir"/state*; do
		name=$(cat "$state/name")
		disable=$(cat "$state/disable")
		if [ "$disable" -eq 0 ]; then
			status="Enabled"
		else
			status="Disabled"
		fi
		echo "$name,$status"
	done
else
	echo "C-state directory not found."
fi`,
	},
	SpecTurboCoresScriptName: {
		Name: SpecTurboCoresScriptName,
		ScriptTemplate: `# if SST-TF is supported and enabled, then the MSR values are not valid
supported=$(pcm-tpmi 5 0xF8 -d -b 12:12 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}')
if [ "$supported" -eq 1 ]; then
	enabled=$(pcm-tpmi 5 0x78 -d -b 9:9 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}')
	if [ "$enabled" -eq 1 ]; then
		exit 0
	fi
fi
rdmsr 0x1ae # MSR_TURBO_GROUP_CORE_CNT: Group Size of Active Cores for Turbo Mode Operation
`,
		Architectures: []string{x86_64},
		Families:      []string{"6"}, // Intel
		Lkms:          []string{"msr"},
		Depends:       []string{"rdmsr", "pcm-tpmi"},
		Superuser:     true,
	},
	SpecTurboFrequenciesScriptName: {
		Name: SpecTurboFrequenciesScriptName,
		ScriptTemplate: `# if SST-TF is supported and enabled, then the MSR values are not valid
supported=$(pcm-tpmi 5 0xF8 -d -b 12:12 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}')
if [ "$supported" -eq 1 ]; then
	enabled=$(pcm-tpmi 5 0x78 -d -b 9:9 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}')
	if [ "$enabled" -eq 1 ]; then
		exit 0
	fi
fi
rdmsr 0x1ad # MSR_TURBO_RATIO_LIMIT: Maximum Ratio Limit of Turbo Mode
`,
		Architectures: []string{x86_64},
		Families:      []string{"6"}, // Intel
		Lkms:          []string{"msr"},
		Depends:       []string{"rdmsr", "pcm-tpmi"},
		Superuser:     true,
	},
	PPINName: {
		Name:           PPINName,
		ScriptTemplate: "rdmsr -a 0x4f", // MSR_PPIN: Protected Processor Inventory Number
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	PrefetchControlName: {
		Name:           PrefetchControlName,
		ScriptTemplate: "rdmsr -f 7:0 0x1a4", // MSR_PREFETCH_CONTROL: L2, DCU, and AMP Prefetchers enabled/disabled
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	PrefetchersName: {
		Name:           PrefetchersName,
		ScriptTemplate: "rdmsr 0x6d", // TODO: get name, used to read prefetchers
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	L3WaySizeName: {
		Name:           L3WaySizeName,
		ScriptTemplate: "rdmsr 0xc90", // TODO: get name, used to read l3 size
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	PackagePowerLimitName: {
		Name:           PackagePowerLimitName,
		ScriptTemplate: "rdmsr -f 14:0 0x610", // MSR_PKG_POWER_LIMIT: Package limit in bits 14:0
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EpbSourceScriptName: {
		Name:           EpbSourceScriptName,
		ScriptTemplate: "rdmsr -f 34:34 0x1FC", // MSR_POWER_CTL, PWR_PERF_TUNING_ALT_EPB: Energy Performance Bias Hint Source (1 is from BIOS, 0 is from OS)
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EpbOSScriptName: {
		Name:           EpbOSScriptName,
		ScriptTemplate: "rdmsr -f 3:0 0x1B0", // IA32_ENERGY_PERF_BIAS: Energy Performance Bias Hint (0 is highest perf, 15 is highest energy saving)
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EpbBIOSScriptName: {
		Name:           EpbBIOSScriptName,
		ScriptTemplate: "rdmsr -f 6:3 0xA01", // ENERGY_PERF_BIAS_CONFIG, ALT_ENERGY_PERF_BIAS: Energy Performance Bias Hint from BIOS (0 is highest perf, 15 is highest energy saving)
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EppValidScriptName: {
		Name:           EppValidScriptName,
		ScriptTemplate: "rdmsr -a -f 60:60 0x774", // IA32_HWP_REQUEST: Energy Performance Preference, bit 60 indicates if per-cpu EPP is valid
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EppPackageControlScriptName: {
		Name:           EppPackageControlScriptName,
		ScriptTemplate: "rdmsr -a -f 42:42 0x774", // IA32_HWP_REQUEST: Energy Performance Preference, bit 42 indicates if package control is enabled
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EppScriptName: {
		Name:           EppScriptName,
		ScriptTemplate: "rdmsr -a -f 31:24 0x774", // IA32_HWP_REQUEST: Energy Performance Preference, bits 24-31 (0 is highest perf, 255 is highest energy saving)
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EppPackageScriptName: {
		Name:           EppPackageScriptName,
		ScriptTemplate: "rdmsr -f 31:24 0x772", // IA32_HWP_REQUEST_PKG: Energy Performance Preference, bits 24-31 (0 is highest perf, 255 is highest energy saving)
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	UncoreMaxFromMSRScriptName: {
		Name:           UncoreMaxFromMSRScriptName,
		ScriptTemplate: "rdmsr -f 6:0 0x620", // MSR_UNCORE_RATIO_LIMIT: MAX_RATIO in bits 6:0
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	UncoreMinFromMSRScriptName: {
		Name:           UncoreMinFromMSRScriptName,
		ScriptTemplate: "rdmsr -f 14:8 0x620", // MSR_UNCORE_RATIO_LIMIT: MAX_RATIO in bits 14:8
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	UncoreMaxFromTPMIScriptName: {
		Name:           UncoreMaxFromTPMIScriptName,
		ScriptTemplate: "pcm-tpmi 2 0x18 -d -b 8:14",
		Architectures:  []string{x86_64},
		Families:       []string{"6"},          // Intel
		Models:         []string{"173", "175"}, // GNR, SRF
		Depends:        []string{"pcm-tpmi"},
		Superuser:      true,
	},
	UncoreMinFromTPMIScriptName: {
		Name:           UncoreMinFromTPMIScriptName,
		ScriptTemplate: "pcm-tpmi 2 0x18 -d -b 15:21",
		Architectures:  []string{x86_64},
		Families:       []string{"6"},          // Intel
		Models:         []string{"173", "175"}, // GNR, SRF
		Depends:        []string{"pcm-tpmi"},
		Superuser:      true,
	},
	UncoreDieTypesFromTPMIScriptName: {
		Name:           UncoreDieTypesFromTPMIScriptName,
		ScriptTemplate: "pcm-tpmi 2 0x10 -d -b 26:26",
		Architectures:  []string{x86_64},
		Families:       []string{"6"},          // Intel
		Models:         []string{"173", "175"}, // GNR, SRF
		Depends:        []string{"pcm-tpmi"},
		Superuser:      true,
	},
	ElcScriptName: {
		Name: ElcScriptName,
		ScriptTemplate: `
# Script derived from bhs-power-mode script in Intel PCM repository
# Run the pcm-tpmi command to determine I/O and compute dies
output=$(pcm-tpmi 2 0x10 -d -b 26:26)

# Parse the output to build lists of I/O and compute dies
io_dies=()
compute_dies=()
declare -A die_types
while read -r line; do
	if [[ $line == *"instance 0"* ]]; then
		die=$(echo "$line" | grep -oP 'entry \K[0-9]+')
		if [[ $line == *"value 1"* ]]; then
			die_types[$die]="IO"
	io_dies+=("$die")
		elif [[ $line == *"value 0"* ]]; then
			die_types[$die]="Compute"
	compute_dies+=("$die")
		fi
	fi
done <<< "$output"

# Function to extract and calculate metrics from the value
extract_and_print_metrics() {
	local value=$1
	local socket_id=$2
	local die=$3
	local die_type=${die_types[$die]}

	# Extract bits and calculate metrics
	local min_ratio=$(( (value >> 15) & 0x7F ))
	local max_ratio=$(( (value >> 8) & 0x7F ))
	local eff_latency_ctrl_ratio=$(( (value >> 22) & 0x7F ))
	local eff_latency_ctrl_low_threshold=$(( (value >> 32) & 0x7F ))
	local eff_latency_ctrl_high_threshold=$(( (value >> 40) & 0x7F ))
	local eff_latency_ctrl_high_threshold_enable=$(( (value >> 39) & 0x1 ))

	# Convert to MHz or percentage
	min_ratio=$(( min_ratio * 100 ))
	max_ratio=$(( max_ratio * 100 ))
	eff_latency_ctrl_ratio=$(( eff_latency_ctrl_ratio * 100 ))
	eff_latency_ctrl_low_threshold=$(( (eff_latency_ctrl_low_threshold * 100) / 127 ))
	eff_latency_ctrl_high_threshold=$(( (eff_latency_ctrl_high_threshold * 100) / 127 ))

	# Print metrics
	echo -n "$socket_id,$die,$die_type,$min_ratio,$max_ratio,$eff_latency_ctrl_ratio,"
	echo "$eff_latency_ctrl_low_threshold,$eff_latency_ctrl_high_threshold,$eff_latency_ctrl_high_threshold_enable"
}

# Print CSV header
echo "Socket,Die,Type,Min Ratio (MHz),Max Ratio (MHz),ELC Ratio (MHz),ELC Low Threshold (%),ELC High Threshold (%),ELC High Threshold Enable"

# Iterate over all dies and run pcm-tpmi for each to get the metrics
for die in "${!die_types[@]}"; do
	output=$(pcm-tpmi 2 0x18 -d -e "$die")

	# Parse the output and extract metrics for each socket
	while read -r line; do
		if [[ $line == *"Read value"* ]]; then
			value=$(echo "$line" | grep -oP 'value \K[0-9]+')
			socket_id=$(echo "$line" | grep -oP 'instance \K[0-9]+')
			extract_and_print_metrics "$value" "$socket_id" "$die"
		fi
	done <<< "$output"
done
			`,
		Architectures: []string{x86_64},
		Families:      []string{"6"},          // Intel
		Models:        []string{"173", "175"}, // GNR, SRF
		Depends:       []string{"pcm-tpmi"},
		Superuser:     true,
	},
	SstTfHighPriorityCoreFrequenciesScriptName: {
		Name: SstTfHighPriorityCoreFrequenciesScriptName,
		ScriptTemplate: `# Is SST-TF supported?
supported=$(pcm-tpmi 5 0xF8 -d -b 12:12 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}')
if [ "$supported" -eq 0 ]; then
	echo "SST-TF is not supported"
	exit 0
fi
# Is SST-TF enabled?
enabled=$(pcm-tpmi 5 0x78 -d -b 9:9 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}')
if [ "$enabled" -eq 0 ]; then
	echo "SST-TF is not enabled"
	exit 0
fi
echo "bucket,cores,AVX,AVX2,AVX-512,AVX-512 heavy,AMX"
# up to 5 buckets
for ((i=0; i<5; i++))
do
	# Get the # of cores in this bucket
	bithigh=$((i*8+7))
	bitlow=$((i*8))
	numcores=$(pcm-tpmi 5 0x100 -d -b $bithigh:$bitlow -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}')
	# if the number of cores is 0, skip this bucket
	if [ "$numcores" -eq 0 ]; then
		continue
	fi
	echo -n "$i,$numcores,"
	# Get the frequencies for this bucket
	bithigh=$((i*8+7)) # 8 bits per frequency
	bitlow=$((i*8))
	# 5 isa frequencies per bucket (AVX, AVX2, AVX-512, AVX-512 heavy, AMX)
	for((j=0; j<5; j++))
	do
		offset=$((j*8 + 264)) // 264 is 0x108 (SST_TF_INFO_2) AVX
		freq=$(pcm-tpmi 5 $offset -d -b $bithigh:$bitlow -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}')
		echo -n "$freq"
		if [ $j -lt 4 ]; then
			echo -n ","
		fi
	done
	echo "" # finish the line
done
`,
		Architectures: []string{x86_64},
		Families:      []string{"6"},   // Intel
		Models:        []string{"173"}, // GNR
		Depends:       []string{"pcm-tpmi"},
		Superuser:     true,
	},
	SstTfLowPriorityCoreFrequenciesScriptName: {
		Name: SstTfLowPriorityCoreFrequenciesScriptName,
		ScriptTemplate: `# Is SST-TF supported?
supported=$(pcm-tpmi 5 0xF8 -d -b 12:12 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}')
if [ "$supported" -eq 0 ]; then
	echo "SST-TF is not supported"
	exit 0
fi
# Is SST-TF enabled?
enabled=$(pcm-tpmi 5 0x78 -d -b 9:9 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}')
if [ "$enabled" -eq 0 ]; then
	echo "SST-TF is not enabled"
	exit 0
fi
echo "AVX,AVX2,AVX-512,AVX-512 heavy,AMX"
# Get the low priority core clip ratios (frequencies)
for((j=0; j<5; j++))
do
	bithigh=$((j*8+23))
	bitlow=$((j*8+16))
	freq=$(pcm-tpmi 5 0xF8 -d -b $bithigh:$bitlow -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}')
	echo -n "$freq"
	if [ $j -ne 4 ]; then
		echo -n ","
	fi
done
echo "" # finish the line
`,
		Architectures: []string{x86_64},
		Families:      []string{"6"},   // Intel
		Models:        []string{"173"}, // GNR
		Depends:       []string{"pcm-tpmi"},
		Superuser:     true,
	},
	ChaCountScriptName: {
		Name: ChaCountScriptName,
		ScriptTemplate: `rdmsr 0x396
rdmsr 0x702
rdmsr 0x2FFE`, // uncore client cha count, uncore cha count, uncore cha count spr
		Architectures: []string{x86_64},
		Families:      []string{"6"}, // Intel
		Lkms:          []string{"msr"},
		Depends:       []string{"rdmsr"},
		Superuser:     true,
	},
	IaaDevicesScriptName: {
		Name:           IaaDevicesScriptName,
		ScriptTemplate: "ls -1 /dev/iax",
	},
	DsaDevicesScriptName: {
		Name:           DsaDevicesScriptName,
		ScriptTemplate: "ls -1 /dev/dsa",
	},
	LshwScriptName: {
		Name:           LshwScriptName,
		ScriptTemplate: "timeout 30 lshw -businfo -numeric",
		Depends:        []string{"lshw"},
		Superuser:      true,
	},
	MeminfoScriptName: {
		Name:           MeminfoScriptName,
		ScriptTemplate: "cat /proc/meminfo",
	},
	TransparentHugePagesScriptName: {
		Name:           TransparentHugePagesScriptName,
		ScriptTemplate: "cat /sys/kernel/mm/transparent_hugepage/enabled",
	},
	NumaBalancingScriptName: {
		Name:           NumaBalancingScriptName,
		ScriptTemplate: "cat /proc/sys/kernel/numa_balancing",
	},
	NicInfoScriptName: {
		Name: NicInfoScriptName,
		ScriptTemplate: `timeout 30 lshw -businfo -numeric | grep -E "^(pci|usb).*? \S+\s+network\s+\S.*?" \
| while read -r a ifc c ; do
	ethtool "$ifc"
	ethtool -i "$ifc"
	echo -n "MAC Address: "
	cat /sys/class/net/"$ifc"/address
	echo -n "NUMA Node: "
	cat /sys/class/net/"$ifc"/device/numa_node
	echo -n "CPU Affinity: "
	intlist=$( grep -e "$ifc" /proc/interrupts | cut -d':' -f1 | sed -e 's/^[[:space:]]*//' )
	for int in $intlist; do
		cpu=$( cat /proc/irq/"$int"/smp_affinity_list )
		printf "%s:%s;" "$int" "$cpu"
	done
	printf "\n"
	echo -n "IRQ Balance: "
	pgrep irqbalance >/dev/null && echo "Enabled" || echo "Disabled"
done
	`,
		Depends:   []string{"lshw"},
		Superuser: true,
	},
	DiskInfoScriptName: {
		Name: DiskInfoScriptName,
		ScriptTemplate: `echo "NAME|MODEL|SIZE|MOUNTPOINT|FSTYPE|RQ-SIZE|MIN-IO|FIRMWARE|ADDR|NUMA|LINKSPEED|LINKWIDTH|MAXLINKSPEED|MAXLINKWIDTH"
lsblk -r -o NAME,MODEL,SIZE,MOUNTPOINT,FSTYPE,RQ-SIZE,MIN-IO -e7 -e1 \
| cut -d' ' -f1,2,3,4,5,6,7 --output-delimiter='|' \
| while IFS='|' read -r name model size mountpoint fstype rqsize minio ;
do
	# skip the lsblk output header
	if [ "$name" = "NAME" ] ; then
		continue
	fi
	fw=""
	addr=""
	numa=""
	curlinkspeed=""
	curlinkwidth=""
	maxlinkspeed=""
	maxlinkwidth=""
	# replace \x20 with space in model
	model=${model//\\x20/ }
	# if name refers to an NVMe device e.g, nvme0n1 - nvme99n99
	if [[ $name =~ ^(nvme[0-9]+)n[0-9]+$ ]]; then
		# get the name without the namespace
		nvme=${BASH_REMATCH[1]}
		if [ -f /sys/block/"$name"/device/firmware_rev ] ; then
			fw=$( cat /sys/block/"$name"/device/firmware_rev )
		fi
		if [ -f /sys/block/"$name"/device/address ] ; then
			addr=$( cat /sys/block/"$name"/device/address )
		fi
		if [ -d "/sys/block/$name/device/${nvme}" ]; then
			numa=$( cat /sys/block/"$name"/device/"${nvme}"/numa_node )
			curlinkspeed=$( cat /sys/block/"$name"/device/"${nvme}"/device/current_link_speed )
			curlinkwidth=$( cat /sys/block/"$name"/device/"${nvme}"/device/current_link_width )
			maxlinkspeed=$( cat /sys/block/"$name"/device/"${nvme}"/device/max_link_speed )
			maxlinkwidth=$( cat /sys/block/"$name"/device/"${nvme}"/device/max_link_width )
		elif [ -d "/sys/block/$name/device/device" ]; then
			numa=$( cat /sys/block/"$name"/device/device/numa_node )
			curlinkspeed=$( cat /sys/block/"$name"/device/device/current_link_speed )
			curlinkwidth=$( cat /sys/block/"$name"/device/device/current_link_width )
			maxlinkspeed=$( cat /sys/block/"$name"/device/device/max_link_speed )
			maxlinkwidth=$( cat /sys/block/"$name"/device/device/max_link_width )
		fi
	fi
	echo "$name|$model|$size|$mountpoint|$fstype|$rqsize|$minio|$fw|$addr|$numa|$curlinkspeed|$curlinkwidth|$maxlinkspeed|$maxlinkwidth"
done`,
	},
	HdparmScriptName: {
		Name: HdparmScriptName,
		ScriptTemplate: `lsblk -d -r -o NAME -e7 -e1 -n | while read -r device ; do
	hdparm -i /dev/"$device"
done`,
		Superuser: true,
	},
	DfScriptName: {
		Name:           DfScriptName,
		ScriptTemplate: `df -h`,
	},
	FindMntScriptName: {
		Name:           FindMntScriptName,
		ScriptTemplate: `findmnt -r`,
		Superuser:      true,
	},
	CveScriptName: {
		Name:           CveScriptName,
		ScriptTemplate: "spectre-meltdown-checker.sh --batch text",
		Superuser:      true,
		Lkms:           []string{"msr"},
		Depends:        []string{"spectre-meltdown-checker.sh", "rdmsr"},
	},
	ProcessListScriptName: {
		Name:           ProcessListScriptName,
		ScriptTemplate: `ps -eo pid,ppid,%cpu,%mem,rss,command --sort=-%cpu,-pid | grep -v "]" | head -n 20`,
		Sequential:     true,
	},
	IpmitoolSensorsScriptName: {
		Name:           IpmitoolSensorsScriptName,
		ScriptTemplate: "LC_ALL=C timeout 30 ipmitool sdr list full",
		Superuser:      true,
		Depends:        []string{"ipmitool"},
	},
	IpmitoolChassisScriptName: {
		Name:           IpmitoolChassisScriptName,
		ScriptTemplate: "LC_ALL=C timeout 30 ipmitool chassis status",
		Superuser:      true,
		Depends:        []string{"ipmitool"},
	},
	IpmitoolEventsScriptName: {
		Name:           IpmitoolEventsScriptName,
		ScriptTemplate: `LC_ALL=C timeout 30 ipmitool sel elist | tail -n20 | cut -d'|' -f2-`,
		Superuser:      true,
		Lkms:           []string{"ipmi_devintf", "ipmi_si"},
		Depends:        []string{"ipmitool"},
	},
	IpmitoolEventTimeScriptName: {
		Name:           IpmitoolEventTimeScriptName,
		ScriptTemplate: "LC_ALL=C timeout 30 ipmitool sel time get",
		Superuser:      true,
		Depends:        []string{"ipmitool"},
	},
	KernelLogScriptName: {
		Name:           KernelLogScriptName,
		ScriptTemplate: "dmesg --kernel --human --nopager | tail -n20",
		Superuser:      true,
	},
	PMUDriverVersionScriptName: {
		Name:           PMUDriverVersionScriptName,
		ScriptTemplate: `dmesg | grep -A 1 "Intel PMU driver" | tail -1 | awk '{print $NF}'`,
		Superuser:      true,
		Architectures:  []string{x86_64},
		Families:       []string{"6"}, // Intel
	},
	PMUBusyScriptName: {
		Name: PMUBusyScriptName,
		ScriptTemplate: `# loop through the PMU counters and check if they are active or inactive
for i in 0x30a 0x309 0x30b 0x30c 0xc1 0xc2 0xc3 0xc4 0xc5 0xc6 0xc7 0xc8; do
	arr=()
	# read the value of the msr represented by the hex value 6 times, save results in an array
	for j in {1..6}; do
		val=$(rdmsr $i | tr -d '\n')
		# if the value isn't a hex value, go on to next hex value
		if [[ ! $val =~ ^[0-9a-fA-F]+$ ]]; then
			echo "$i Unknown"
			continue 2
		fi
		arr+=($val)
	done
	# if the first and last value in the array are the same, the counter is inactive
	if [ ${arr[0]} == ${arr[5]} ]; then
		echo "$i Inactive"
	else
		echo "$i Active"
	fi
done`,
		Superuser:     true,
		Architectures: []string{x86_64},
		Families:      []string{"6"}, // Intel
		Lkms:          []string{"msr"},
		Depends:       []string{"rdmsr"},
	},
	GaudiInfoScriptName: {
		Name:           GaudiInfoScriptName,
		ScriptTemplate: `hl-smi -Q module_id,serial,bus_id,driver_version -f csv`,
		Architectures:  []string{"x86_64"},
		Families:       []string{"6"}, // Intel
	},
	GaudiFirmwareScriptName: {
		Name:           GaudiFirmwareScriptName,
		ScriptTemplate: `hl-smi --fw-version`,
		Architectures:  []string{"x86_64"},
		Families:       []string{"6"}, // Intel
	},
	GaudiNumaScriptName: {
		Name:           GaudiNumaScriptName,
		ScriptTemplate: `hl-smi topo -N`,
		Architectures:  []string{"x86_64"},
		Families:       []string{"6"}, // Intel
	},
	MemoryBandwidthAndLatencyScriptName: {
		Name: MemoryBandwidthAndLatencyScriptName,
		ScriptTemplate: `# measure memory loaded latency
#  need at least 2 GB (2,097,152 KB) of huge pages per NUMA node
min_kb=2097152
numa_nodes=$( lscpu | grep "NUMA node(s):" | awk '{print $3}' )
size_huge_pages_kb=$( grep Hugepagesize /proc/meminfo | awk '{print $2}' )
orig_num_huge_pages=$( cat /proc/sys/vm/nr_hugepages )
needed_num_huge_pages=$((numa_nodes * min_kb / size_huge_pages_kb))
if [ $needed_num_huge_pages -gt $orig_num_huge_pages ]; then
  echo $needed_num_huge_pages > /proc/sys/vm/nr_hugepages
fi
mlc --loaded_latency
echo $orig_num_huge_pages > /proc/sys/vm/nr_hugepages`,
		Architectures: []string{x86_64},
		Superuser:     true,
		Lkms:          []string{"msr"},
		Depends:       []string{"mlc"},
		Sequential:    true,
	},
	NumaBandwidthScriptName: {
		Name: NumaBandwidthScriptName,
		ScriptTemplate: `# measure memory bandwidth matrix
#  need at least 2 GB (2,097,152 KB) of huge pages per NUMA node
min_kb=2097152
numa_nodes=$( lscpu | grep "NUMA node(s):" | awk '{print $3}' )
size_huge_pages_kb=$( grep Hugepagesize /proc/meminfo | awk '{print $2}' )
orig_num_huge_pages=$( cat /proc/sys/vm/nr_hugepages )
needed_num_huge_pages=$((numa_nodes * min_kb / size_huge_pages_kb))
if [ $needed_num_huge_pages -gt $orig_num_huge_pages ]; then
  echo $needed_num_huge_pages > /proc/sys/vm/nr_hugepages
fi
mlc --bandwidth_matrix
echo $orig_num_huge_pages > /proc/sys/vm/nr_hugepages`,
		Architectures: []string{x86_64},
		Superuser:     true,
		Lkms:          []string{"msr"},
		Depends:       []string{"mlc"},
		Sequential:    true,
	},
	CpuSpeedScriptName: {
		Name: CpuSpeedScriptName,
		ScriptTemplate: `methods=$( stress-ng --cpu 1 --cpu-method x 2>&1 | cut -d":" -f2 | cut -c 6- )
for method in $methods; do
	printf "%s " "$method"
	stress-ng --cpu 0 -t 1 --cpu-method "$method" --metrics-brief 2>&1 | tail -1 | awk '{print $9}'
done`,
		Superuser:  false,
		Depends:    []string{"stress-ng"},
		Sequential: true,
	},
	TurboFrequenciesScriptName: {
		Name: TurboFrequenciesScriptName,
		ScriptTemplate: `# Function to expand a range of numbers, e.g. "0-24", into an array of numbers
expand_range() {
	local range=$1
	local expanded=()
	IFS=',' read -ra parts <<< "$range"
	for part in "${parts[@]}"; do
		if [[ $part == *-* ]]; then
			IFS='-' read -ra limits <<< "$part"
			for ((i=${limits[0]}; i<=${limits[1]}; i++)); do
				expanded+=("$i")
			done
		else
			expanded+=("$part")
		fi
	done
	echo "${expanded[@]}"
}

# Get the number of NUMA nodes and sockets
num_nodes=$(lscpu | grep 'NUMA node(s):' | awk '{print $3}')
num_sockets=$(lscpu | grep 'Socket(s):' | awk '{print $2}')

# echo "Number of NUMA nodes: $num_nodes"
# echo "Number of sockets: $num_sockets"

# Calculate the number of NUMA nodes per socket
nodes_per_socket=$((num_nodes / num_sockets))

# Array to hold the expanded core lists for each NUMA node
declare -a core_lists

# Loop through each NUMA node in the first socket and expand the core IDs
for ((i=0; i<nodes_per_socket; i++)); do
	core_range=$(lscpu | grep "NUMA node$i CPU(s):" | awk -F: '{print $2}' | tr -d ' ' | cut -d',' -f1)
	core_list=$(expand_range "$core_range")
	core_lists+=("$core_list")
done

# Interleave the core IDs from each NUMA node
interleaved_cores=()
max_length=0

# Find the maximum length of the core lists
for core_list in "${core_lists[@]}"; do
	core_array=($core_list)
	if (( ${#core_array[@]} > max_length )); then
		max_length=${#core_array[@]}
	fi
done

# Interleave the core IDs
for ((i=0; i<max_length; i++)); do
	for core_list in "${core_lists[@]}"; do
		core_array=($core_list)
		if (( i < ${#core_array[@]} )); then
			interleaved_cores+=("${core_array[i]}")
		fi
	done
done

# Form the interleaved core IDs into a comma-separated list
interleaved_core_list=$(IFS=,; echo "${interleaved_cores[*]}")
# echo "Interleaved core IDs: $interleaved_core_list"

# Get the number of cores per socket
num_cores_per_socket=$( lscpu | grep 'Core(s) per socket:' | head -1 | awk '{print $4}' )

# Run the avx-turbo benchmark
avx-turbo --min-threads=1 --max-threads=$num_cores_per_socket --test scalar_iadd,avx128_fma,avx256_fma,avx512_fma --iters=100000 --cpuids=$interleaved_core_list`,
		Superuser:  true,
		Lkms:       []string{"msr"},
		Depends:    []string{"avx-turbo"},
		Sequential: true,
	},
	TurboFrequencyPowerAndTemperatureScriptName: {
		Name:           TurboFrequencyPowerAndTemperatureScriptName,
		ScriptTemplate: `((turbostat -i 2 2>/dev/null &) ; stress-ng --cpu 1 -t 20s 2>&1 ; stress-ng --cpu 0 -t 60s 2>&1 ; pkill -9 -f turbostat) | awk '$0~"stress" {print $0} $1=="Package" || $1=="CPU" || $1=="Core" || $1=="Node" {if(f!=1) print $0;f=1} $1=="-" {print $0}'		`,
		Superuser:      true,
		Lkms:           []string{"msr"},
		Depends:        []string{"turbostat", "stress-ng"},
		Sequential:     true,
	},
	IdlePowerScriptName: {
		Name:           IdlePowerScriptName,
		ScriptTemplate: `turbostat --show PkgWatt -n 1 | sed -n 2p`,
		Superuser:      true,
		Lkms:           []string{"msr"},
		Depends:        []string{"turbostat"},
		Sequential:     true,
	},
	StoragePerfScriptName: {
		Name: StoragePerfScriptName,
		ScriptTemplate: `
file_dir={{.StorageDir}}
test_dir="$file_dir"/fio_test
file_size_g=5
numjobs=1
total_file_size_g=$(($file_size_g * $numjobs))
ramp_time=5s
runtime=120s
ioengine=sync
# confirm the file_dir is a directory, is writeable, and has enough space
if [[ -d "$file_dir" && -w "$file_dir" ]]; then
	available_space=$(df -hP "$file_dir")
	count=$( echo "$available_space" | awk '/[0-9]%%/{print substr($4,1,length($4)-1)}' )
	unit=$( echo "$available_space" | awk '/[0-9]%%/{print substr($4,length($4),1)}' )
	is_enough_gigabytes=$(awk -v c="$count" -v f=$total_file_size_g 'BEGIN{print (c>f)?1:0}')
	is_terabyte_or_more=$(echo "TPEZY" | grep -F -q "$unit" && echo 1 || echo 0)
	if [[ ("$unit" == "G" && "$is_enough_gigabytes" == 0) && "$is_terabyte_or_more" == 1 ]]; then
		echo "ERROR: $file_dir does not have enough available space - $total_file_size_g GB required"
		exit 1
	fi
else
	echo "ERROR: $file_dir does not exist or is not writeable"
	exit 1
fi
# single-threaded read & write bandwidth test
rm -rf $test_dir
mkdir -p $test_dir
sync
/sbin/sysctl -w vm.drop_caches=3 || true
fio --name=bandwidth --directory=$test_dir --numjobs=$numjobs \
--size="$file_size_g"G --time_based --runtime=$runtime --ramp_time=$ramp_time --ioengine=$ioengine \
--direct=1 --verify=0 --bs=1M --iodepth=64 --rw=rw \
--group_reporting=1 --iodepth_batch_submit=64 \
--iodepth_batch_complete_max=64
rm -rf $test_dir
`,
		Superuser:  true,
		Sequential: true,
		Depends:    []string{"fio"},
	},
	// telemetry scripts
	MpstatScriptName: {
		Name: MpstatScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
fi
mpstat -u -T -I SCPU -P ALL $interval $count &
echo $! > {{.ScriptName}}_cmd.pid
wait`,
		Superuser: true,
		Lkms:      []string{},
		Depends:   []string{"mpstat"},
		NeedsKill: true,
	},
	IostatScriptName: {
		Name: IostatScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
fi
S_TIME_FORMAT=ISO iostat -d -t $interval $count | sed '/^loop/d' &
echo $! > {{.ScriptName}}_cmd.pid
wait`,
		Superuser: true,
		Lkms:      []string{},
		Depends:   []string{"iostat"},
		NeedsKill: true,
	},
	SarMemoryScriptName: {
		Name: SarMemoryScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
fi
sar -r $interval $count &
echo $! > {{.ScriptName}}_cmd.pid
wait`,
		Superuser: true,
		Lkms:      []string{},
		Depends:   []string{"sar", "sadc"},
		NeedsKill: true,
	},
	SarNetworkScriptName: {
		Name: SarNetworkScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
fi
sar -n DEV $interval $count &
echo $! > {{.ScriptName}}_cmd.pid
wait`,
		Superuser: true,
		Lkms:      []string{},
		Depends:   []string{"sar", "sadc"},
		NeedsKill: true,
	},
	TurbostatScriptName: {
		Name: TurbostatScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
	count="-n $count"
fi
turbostat -S -s PkgWatt,RAMWatt -q -i $interval $count | awk '{ print strftime("%H:%M:%S"), $0 }' &
echo $! > {{.ScriptName}}_cmd.pid
wait`,
		Superuser: true,
		Lkms:      []string{"msr"},
		Depends:   []string{"turbostat"},
		NeedsKill: true,
	},
	InstructionMixScriptName: {
		Name: InstructionMixScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
	arg_count="-n $count"
fi
if [ $interval -ne 0 ]; then
	arg_interval="-i $interval"
fi
echo TIME: $(date +"%H:%M:%S")
echo INTERVAL: $interval
# if no PID specified, increase the sampling interval (defaults to 100,000) to reduce overhead
if [ {{.PID}} -eq 0 ]; then
	arg_sampling_rate="-s 1000000"
else
	arg_pid="-p {{.PID}}"
fi
# .Filter is a space separated list of ISA categories
# for each category in the list, add -f <category> to the command line
for category in {{.Filter}}; do
    arg_filter="$arg_filter -f $category"
done

processwatch -c $arg_sampling_rate $arg_pid $arg_interval $arg_count $arg_filter &
echo $! > {{.ScriptName}}_cmd.pid
wait`,
		Superuser: true,
		Lkms:      []string{"msr"},
		Depends:   []string{"processwatch"},
		NeedsKill: true,
	},
	ProfileJavaScriptName: {
		Name: ProfileJavaScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
frequency={{.Frequency}}
ap_interval=0
if [ $frequency -ne 0 ]; then
	ap_interval=$((1000000000 / frequency))
fi
# JAVA app call stack collection (run in background)
declare -a java_pids=()
declare -a java_cmds=()
for pid in $( pgrep java ) ; do
	# verify pid is still running
	if [ -d "/proc/$pid" ]; then
		java_pids+=($pid)
		java_cmds+=("$( tr '\000' ' ' <  /proc/$pid/cmdline )")
		# profile pid in background
		async-profiler/profiler.sh start -i "$ap_interval" -o collapsed "$pid"
	fi
done
sleep $duration
# stop java profiling for each java pid
for idx in "${!java_pids[@]}"; do
	pid="${java_pids[$idx]}"
	cmd="${java_cmds[$idx]}"
	echo "########## async-profiler $pid $cmd ##########"
	async-profiler/profiler.sh stop -o collapsed "$pid"
done
`,
		Superuser: true,
		Depends:   []string{"async-profiler"},
	},
	ProfileSystemScriptName: {
		Name: ProfileSystemScriptName,
		ScriptTemplate: `frequency={{.Frequency}}
duration={{.Duration}}

# system-wide call stack collection

# Function to restore original settings
restore_settings() {
	echo "$PERF_EVENT_PARANOID" > /proc/sys/kernel/perf_event_paranoid
	echo "$KPTR_RESTRICT" > /proc/sys/kernel/kptr_restrict
}

# adjust perf_event_paranoid and kptr_restrict
PERF_EVENT_PARANOID=$( cat /proc/sys/kernel/perf_event_paranoid )
echo -1 >/proc/sys/kernel/perf_event_paranoid
KPTR_RESTRICT=$( cat /proc/sys/kernel/kptr_restrict )
echo 0 >/proc/sys/kernel/kptr_restrict

# Ensure settings are restored on exit
trap restore_settings EXIT

# system-wide call stack collection - frame pointer mode
perf_fp_data=$(mktemp /tmp/perf_fp.XXXXXX)
perf record -F $frequency -a -g -o "$perf_fp_data" -m 129 -- sleep $duration &
PERF_FP_PID=$!
if ! kill -0 $PERF_FP_PID 2>/dev/null; then
	echo "Failed to start perf record for frame pointer mode"
	exit 1
fi

# system-wide call stack collection - dwarf mode
perf_dwarf_data=$(mktemp /tmp/perf_dwarf.XXXXXX)
perf record -F $frequency -a -g -o "$perf_dwarf_data" -m 257 --call-graph dwarf,8192 -- sleep $duration &
PERF_DWARF_PID=$!
if ! kill -0 $PERF_DWARF_PID 2>/dev/null; then
	echo "Failed to start perf record for dwarf mode"
	kill $PERF_FP_PID
	exit 1
fi

# wait for perf to finish
wait ${PERF_FP_PID} ${PERF_DWARF_PID}

# collapse perf data
perf_dwarf_folded=$(mktemp /tmp/perf_dwarf_folded.XXXXXX)
perf_fp_folded=$(mktemp /tmp/perf_fp_folded.XXXXXX)
perf script -i "$perf_dwarf_data" | stackcollapse-perf.pl > "$perf_dwarf_folded"
perf script -i "$perf_fp_data" | stackcollapse-perf.pl > "$perf_fp_folded"

# Display results
if [ -f "$perf_dwarf_folded" ]; then
	echo "########## perf_dwarf ##########"
	cat "$perf_dwarf_folded"
fi
if [ -f "$perf_fp_folded" ]; then
	echo "########## perf_fp ##########"
	cat "$perf_fp_folded"
fi

# Clean up temporary files
rm -f "$perf_fp_data" "$perf_dwarf_data" "$perf_dwarf_folded" "$perf_fp_folded"
`,
		Superuser: true,
		Depends:   []string{"perf", "stackcollapse-perf.pl"},
	},
	ProfileKernelLockScriptName: {
		Name: ProfileKernelLockScriptName,
		ScriptTemplate: `frequency={{.Frequency}}
duration={{.Duration}}
# system-wide lock profile collection
# adjust perf_event_paranoid and kptr_restrict
PERF_EVENT_PARANOID=$( cat /proc/sys/kernel/perf_event_paranoid )
echo -1 >/proc/sys/kernel/perf_event_paranoid
KPTR_RESTRICT=$( cat /proc/sys/kernel/kptr_restrict )
echo 0 >/proc/sys/kernel/kptr_restrict

# collect hotspot
perf record -F $frequency -a -g --call-graph dwarf -W -d --phys-data --sample-cpu -e cycles:pp,instructions:pp,cpu/mem-loads,ldlat=30/P,cpu/mem-stores/P -o perf_hotspot.data -- sleep $duration &
PERF_HOTSPOT_PID=$!

# check the availability perf lock -b option 
perf lock contention -a -bv --max-stack 20 2>/dev/null -- sleep 0
PERF_LOCK_CONTENTION_BPF=$?

# collect lock
if [ ${PERF_LOCK_CONTENTION_BPF} -eq 0 ]; then
	perf lock contention -a -bv --max-stack 20 2>perf_lock_contention.txt -- sleep $duration &
	PERF_LOCK_PID=$!
fi

wait ${PERF_HOTSPOT_PID}

if [ ${PERF_LOCK_CONTENTION_BPF} -eq 0 ]; then
	wait ${PERF_LOCK_PID}
fi

# restore perf_event_paranoid and kptr_restrict
echo "$PERF_EVENT_PARANOID" > /proc/sys/kernel/perf_event_paranoid
echo "$KPTR_RESTRICT" > /proc/sys/kernel/kptr_restrict

# collapse perf data
if [ -f "perf_hotspot.data" ]; then
	echo "########## perf_hotspot_no_children ##########"
	perf report -i perf_hotspot.data --no-children --call-graph none --stdio
	echo "########## perf_hotspot_callgraph ##########"
	perf report -i perf_hotspot.data --stdio
fi
if [ -f "perf_hotspot.data" ]; then
	echo "########## perf_c2c_no_children ##########"
	perf c2c report  -i perf_hotspot.data --call-graph none --stdio
	echo "########## perf_c2c_callgraph ##########"
	perf c2c report  -i perf_hotspot.data --stdio
fi
if [ -f "perf_lock_contention.txt" ]; then
	echo "########## perf_lock_contention ##########"
	cat perf_lock_contention.txt
fi
`,
		Superuser: true,
		Depends:   []string{"perf"},
	},
}
