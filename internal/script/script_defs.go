package script

import (
	"bytes"
	texttemplate "text/template" // nosemgrep
)

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// script_defs.go defines the bash scripts that are used to collect information from target systems

type ScriptDefinition struct {
	Name           string   // just a name
	ScriptTemplate string   // the bash script that will be run
	Architectures  []string // architectures, i.e., x86_64, aarch64. If empty, it will run on all architectures.
	Vendors        []string // vendors, i.e., GenuineIntel, AuthenticAMD. If empty, it will run on all vendors.
	Families       []string // families, e.g., 6, 7. If empty, it will run on all families.
	Models         []string // models, e.g., 62, 63. If empty, it will run on all models.
	Lkms           []string // loadable kernel modules
	Depends        []string // binary dependencies that must be available for the script to run
	Superuser      bool     // requires sudo or root
	Sequential     bool     // run script sequentially (not at the same time as others)
	NeedsKill      bool     // process/script needs to be killed after run without a duration specified, i.e., it doesn't stop through SIGINT
}

// script names, these must be unique
const (
	// report and configuration (reading) scripts
	HostnameScriptName               = "hostname"
	DateScriptName                   = "date"
	DmidecodeScriptName              = "dmidecode"
	LscpuScriptName                  = "lscpu"
	LspciBitsScriptName              = "lspci bits"
	LspciDevicesScriptName           = "lspci devices"
	LspciVmmScriptName               = "lspci vmm"
	UnameScriptName                  = "uname"
	ProcCmdlineScriptName            = "proc cmdline"
	ProcCpuinfoScriptName            = "proc cpuinfo"
	SysctlScriptName                 = "sysctl"
	EtcReleaseScriptName             = "etc release"
	GccVersionScriptName             = "gcc version"
	BinutilsVersionScriptName        = "binutils version"
	GlibcVersionScriptName           = "glibc version"
	PythonVersionScriptName          = "python version"
	Python3VersionScriptName         = "python3 version"
	JavaVersionScriptName            = "java version"
	OpensslVersionScriptName         = "openssl version"
	CpuidScriptName                  = "cpuid"
	BaseFrequencyScriptName          = "base frequency"
	MaximumFrequencyScriptName       = "maximum frequency"
	ScalingDriverScriptName          = "scaling driver"
	ScalingGovernorScriptName        = "scaling governor"
	CstatesScriptName                = "c-states"
	C1DemotionScriptName             = "c1 demotion"
	SpecCoreFrequenciesScriptName    = "spec core frequencies"
	PPINName                         = "ppin"
	PrefetchControlName              = "prefetch control"
	PrefetchersName                  = "prefetchers"
	PrefetchersAtomName              = "prefetchers atom"
	L3CacheWayEnabledName            = "l3 way enabled"
	PackagePowerLimitName            = "package power limit"
	EpbScriptName                    = "energy performance bias"
	EpbSourceScriptName              = "energy performance bias source"
	EppScriptName                    = "energy performance preference"
	EppValidScriptName               = "epp valid"
	EppPackageControlScriptName      = "epp package control"
	EppPackageScriptName             = "energy performance preference package"
	IaaDevicesScriptName             = "iaa devices"
	DsaDevicesScriptName             = "dsa devices"
	LshwScriptName                   = "lshw"
	UncoreMaxFromMSRScriptName       = "uncore max from msr"
	UncoreMinFromMSRScriptName       = "uncore min from msr"
	UncoreMaxFromTPMIScriptName      = "uncore max from tpmi"
	UncoreMinFromTPMIScriptName      = "uncore min from tpmi"
	UncoreDieTypesFromTPMIScriptName = "uncore die types from tpmi"
	ElcScriptName                    = "efficiency latency control"
	SSTTFHPScriptName                = "ssttf hp frequencies"
	SSTTFLPScriptName                = "ssttf lp frequencies"
	ChaCountScriptName               = "cha count"
	MeminfoScriptName                = "meminfo"
	TransparentHugePagesScriptName   = "transparent huge pages"
	NumaBalancingScriptName          = "numa balancing"
	NicInfoScriptName                = "nic info"
	DiskInfoScriptName               = "disk info"
	HdparmScriptName                 = "hdparm"
	DfScriptName                     = "df"
	FindMntScriptName                = "findmnt"
	CveScriptName                    = "cve"
	ProcessListScriptName            = "process list"
	IpmitoolSensorsScriptName        = "ipmitool sensors"
	IpmitoolChassisScriptName        = "ipmitool chassis"
	IpmitoolEventsScriptName         = "ipmitool events"
	TmeScriptName                    = "tme"
	KernelLogScriptName              = "kernel log"
	PMUDriverVersionScriptName       = "pmu driver version"
	PMUBusyScriptName                = "pmu busy"
	GaudiInfoScriptName              = "gaudi info"
	GaudiFirmwareScriptName          = "gaudi firmware"
	GaudiNumaScriptName              = "gaudi numa"
	// benchmark scripts
	MemoryBenchmarkScriptName    = "memory benchmark"
	NumaBenchmarkScriptName      = "numa benchmark"
	SpeedBenchmarkScriptName     = "speed benchmark"
	FrequencyBenchmarkScriptName = "frequency benchmark"
	PowerBenchmarkScriptName     = "power benchmark"
	IdlePowerBenchmarkScriptName = "idle power benchmark"
	StorageBenchmarkScriptName   = "storage benchmark"
	// telemetry scripts
	MpstatTelemetryScriptName      = "mpstat telemetry"
	IostatTelemetryScriptName      = "iostat telemetry"
	MemoryTelemetryScriptName      = "memory telemetry"
	NetworkTelemetryScriptName     = "network telemetry"
	TurbostatTelemetryScriptName   = "turbostat telemetry"
	InstructionTelemetryScriptName = "instruction telemetry"
	GaudiTelemetryScriptName       = "gaudi telemetry"
	// flamegraph scripts
	CollapsedCallStacksScriptName = "collapsed call stacks"
	// lock scripts
	ProfileKernelLockScriptName = "profile kernel lock"
)

const (
	x86_64 = "x86_64"
)

// GetScriptByName returns the script definition with the given name. It will panic if the script is not found.
func GetScriptByName(name string) ScriptDefinition {
	return GetParameterizedScriptByName(name, nil)
}

// GetParameterizedScriptByName returns the script definition with the given name. It will panic if the script is not found.
func GetParameterizedScriptByName(name string, params map[string]string) ScriptDefinition {
	// if the script doesn't exist, panic
	if _, ok := scriptDefinitions[name]; !ok {
		panic("script not found: " + name)
	}
	if params == nil {
		params = make(map[string]string)
	}
	// augment params with script name
	params["ScriptName"] = sanitizeScriptName(name)
	// replace the script template with the parameterized version
	scriptTemplate := texttemplate.Must(texttemplate.New("scriptTemplate").Parse(scriptDefinitions[name].ScriptTemplate))
	buf := new(bytes.Buffer)
	err := scriptTemplate.Execute(buf, params)
	if err != nil {
		panic(err)
	}
	scriptDefinition := scriptDefinitions[name]
	scriptDefinition.ScriptTemplate = buf.String()
	return scriptDefinition
}

// script definitions
var scriptDefinitions = map[string]ScriptDefinition{
	// report and configuration (read) scripts
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
		Families:       []string{"6"},          // Intel
		Models:         []string{"143", "207"}, // SPR, EMR
		Superuser:      true,
		Depends:        []string{"lspci"},
	},
	LspciDevicesScriptName: {
		Name:           LspciDevicesScriptName,
		ScriptTemplate: "lspci -d 8086:3258 | wc -l",
		Families:       []string{"6"},                        // Intel
		Models:         []string{"173", "174", "175", "221"}, // GNR, GNR-D, SRF, CWF
		Depends:        []string{"lspci"},
	},
	LspciVmmScriptName: {
		Name:           LspciVmmScriptName,
		ScriptTemplate: "lspci -i pci.ids.gz -vmm",
		Depends:        []string{"lspci", "pci.ids.gz"},
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
	SysctlScriptName: {
		Name:           SysctlScriptName,
		ScriptTemplate: "sysctl -a",
		Superuser:      true,
	}, EtcReleaseScriptName: {
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
fi
`,
	},
	C1DemotionScriptName: {
		Name: C1DemotionScriptName,
		ScriptTemplate: `# if both bit 26 and bit 28 are set then C1 demotion is enabled
bit26=$(rdmsr -f 26:26 0xe2 2>/dev/null)
bit28=$(rdmsr -f 28:28 0xe2 2>/dev/null)
if [[ "$bit26" == "1" && "$bit28" == "1" ]]; then
    echo "Enabled"
elif [[ "$bit26" == "0" && "$bit28" == "0" ]]; then
    echo "Disabled"
else
    exit 1
fi
`,
		Architectures: []string{x86_64},
		Vendors:       []string{"GenuineIntel"},
		Lkms:          []string{"msr"},
		Depends:       []string{"rdmsr"},
		Superuser:     true,
	},
	SpecCoreFrequenciesScriptName: {
		Name: SpecCoreFrequenciesScriptName,
		ScriptTemplate: `lscpu=$(lscpu)
family=$(echo "$lscpu" | grep -E "^CPU family:" | awk '{print $3}')
model=$(echo "$lscpu" | grep -E "^Model:" | awk '{print $2}')
vendor=$(echo "$lscpu" | grep -E "^Vendor ID:" | awk '{print $3}')
# if vendor isn't Intel
if [ "$vendor" != "GenuineIntel" ]; then
	exit 1
fi
# if cpu is GNR get the frequencies from tpmi
if [ "$family" -eq 6 ] && [ "$model" -eq 173 ]; then  # GNR
	cores=$(pcm-tpmi 0x5 0xD8 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $3}') # SST_PP_INFO_10
	# this works unless the TRL is overridden on MSR 0x1AD --> sse=$(pcm-tpmi 0x5 0xA8 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $3}') # SST_PP_INFO_4
	sse=$(rdmsr 0x1ad) # MSR_TURBO_RATIO_LIMIT: Maximum Ratio Limit of Turbo Mode
	avx2=$(pcm-tpmi 0x5 0xB0 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $3}') # SST_PPINFO_5
	avx512=$(pcm-tpmi 0x5 0xB8 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $3}') # SST_PPINFO_6
	avx512h=$(pcm-tpmi 0x5 0xC0 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $3}') # SST_PPINFO_7
	amx=$(pcm-tpmi 0x5 0xC8 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $3}') # SST_PPINFO_8
elif [ "$family" -eq 6 ] && ( [ "$model" -eq 175 ] || [ "$model" -eq 221 ] ); then  # SRF, CWF
	cores=$(rdmsr 0x1ae) # MSR_TURBO_GROUP_CORE_CNT: Group Size of Active Cores for Turbo Mode Operation
	# if pstate driver is intel_pstate use 0x774 else use 0x199
	driver=$(cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_driver)
	if [ "$driver" = "intel_pstate" ]; then
		sse=$(rdmsr 0x774 -f 15:8) # IA32_HWP_REQUEST
	else
		sse=$(rdmsr 0x199 -f 15:8) # IA32_PERF_CTL
	fi
	avx2=0
	avx512=0
	avx512h=0
	amx=0
else # not SRF, CWF or GNR
	cores=$(rdmsr 0x1ae) # MSR_TURBO_GROUP_CORE_CNT: Group Size of Active Cores for Turbo Mode Operation
	sse=$(rdmsr 0x1ad) # MSR_TURBO_RATIO_LIMIT: Maximum Ratio Limit of Turbo Mode
	avx2=0
	avx512=0
	avx512h=0
	amx=0
fi
echo "cores sse avx2 avx512 avx512h amx"
echo "$cores" "$sse" "$avx2" "$avx512" "$avx512h" "$amx"`,
		Architectures: []string{x86_64},
		Vendors:       []string{"GenuineIntel"},
		Lkms:          []string{"msr"},
		Depends:       []string{"rdmsr", "pcm-tpmi"},
		Superuser:     true,
	},
	PPINName: {
		Name:           PPINName,
		ScriptTemplate: "rdmsr -a 0x4f", // MSR_PPIN: Protected Processor Inventory Number
		Architectures:  []string{x86_64},
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	PrefetchControlName: {
		Name:           PrefetchControlName,
		ScriptTemplate: "rdmsr -f 7:0 0x1a4", // MSR_PREFETCH_CONTROL: L2, DCU, and AMP Prefetchers enabled/disabled
		Architectures:  []string{x86_64},
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	PrefetchersName: {
		Name:           PrefetchersName,
		ScriptTemplate: "rdmsr 0x6d", // TODO: get name, used to read prefetchers
		Architectures:  []string{x86_64},
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	PrefetchersAtomName: {
		Name:           PrefetchersAtomName,
		ScriptTemplate: "rdmsr 0x1320", // Atom Pref_tuning1
		Architectures:  []string{x86_64},
		Vendors:        []string{"GenuineIntel"},
		Models:         []string{"175", "221"}, // SRF, CWF
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	L3CacheWayEnabledName: {
		Name:           L3CacheWayEnabledName,
		ScriptTemplate: "rdmsr 0xc90", // TODO: get name, used to read l3 size
		Architectures:  []string{x86_64},
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	PackagePowerLimitName: {
		Name:           PackagePowerLimitName,
		ScriptTemplate: "rdmsr -f 14:0 0x610", // MSR_PKG_POWER_LIMIT: Package limit in bits 14:0
		Architectures:  []string{x86_64},
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EpbSourceScriptName: {
		Name:           EpbSourceScriptName,
		ScriptTemplate: "rdmsr -f 34:34 0x1FC", // MSR_POWER_CTL, PWR_PERF_TUNING_ALT_EPB: Energy Performance Bias Hint Source (1 is from BIOS, 0 is from OS)
		Architectures:  []string{x86_64},
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	}, EpbScriptName: {
		Name: EpbScriptName,
		ScriptTemplate: `# get EPB source
# MSR_POWER_CTL, PWR_PERF_TUNING_ALT_EPB: Energy Performance Bias Hint Source (1 is from BIOS, 0 is from OS)
if ! source=$(rdmsr -f 34:34 0x1FC); then
    echo "Error: Failed to read MSR 0x1FC" >&2
    exit 1
fi
if [ "$source" -eq 1 ]; then
    # get EPB from BIOS
    # ENERGY_PERF_BIAS_CONFIG, ALT_ENERGY_PERF_BIAS: Energy Performance Bias Hint from BIOS (0 is highest perf, 15 is highest energy saving)
    if ! epb=$(rdmsr -f 6:3 0xA01); then
        echo "Error: Failed to read MSR 0xA01" >&2
        exit 1
    fi
else
    # get EPB from OS
    # IA32_ENERGY_PERF_BIAS: Energy Performance Bias Hint (0 is highest perf, 15 is highest energy saving))
    if ! epb=$(rdmsr -f 3:0 0x1B0); then
        echo "Error: Failed to read MSR 0x1B0" >&2
        exit 1
    fi
fi
echo "$epb"`,
		Architectures: []string{x86_64},
		Vendors:       []string{"GenuineIntel"},
		Lkms:          []string{"msr"},
		Depends:       []string{"rdmsr"},
		Superuser:     true,
	},
	EppValidScriptName: {
		Name:           EppValidScriptName,
		ScriptTemplate: "rdmsr -a -f 60:60 0x774", // IA32_HWP_REQUEST: Energy Performance Preference, bit 60 indicates if per-cpu EPP is valid
		Architectures:  []string{x86_64},
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EppPackageControlScriptName: {
		Name:           EppPackageControlScriptName,
		ScriptTemplate: "rdmsr -a -f 42:42 0x774", // IA32_HWP_REQUEST: Energy Performance Preference, bit 42 indicates if package control is enabled
		Architectures:  []string{x86_64},
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EppScriptName: {
		Name:           EppScriptName,
		ScriptTemplate: "rdmsr -a -f 31:24 0x774", // IA32_HWP_REQUEST: Energy Performance Preference, bits 24-31 (0 is highest perf, 255 is highest energy saving)
		Architectures:  []string{x86_64},
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EppPackageScriptName: {
		Name:           EppPackageScriptName,
		ScriptTemplate: "rdmsr -f 31:24 0x772", // IA32_HWP_REQUEST_PKG: Energy Performance Preference, bits 24-31 (0 is highest perf, 255 is highest energy saving)
		Architectures:  []string{x86_64},
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	UncoreMaxFromMSRScriptName: {
		Name:           UncoreMaxFromMSRScriptName,
		ScriptTemplate: "rdmsr -f 6:0 0x620", // MSR_UNCORE_RATIO_LIMIT: MAX_RATIO in bits 6:0
		Architectures:  []string{x86_64},
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	UncoreMinFromMSRScriptName: {
		Name:           UncoreMinFromMSRScriptName,
		ScriptTemplate: "rdmsr -f 14:8 0x620", // MSR_UNCORE_RATIO_LIMIT: MAX_RATIO in bits 14:8
		Architectures:  []string{x86_64},
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	UncoreMaxFromTPMIScriptName: {
		Name:           UncoreMaxFromTPMIScriptName,
		ScriptTemplate: "pcm-tpmi 2 0x18 -d -b 8:14",
		Architectures:  []string{x86_64},
		Families:       []string{"6"},                        // Intel
		Models:         []string{"173", "174", "175", "221"}, // GNR, GNR-D, SRF, CWF
		Depends:        []string{"pcm-tpmi"},
		Superuser:      true,
	},
	UncoreMinFromTPMIScriptName: {
		Name:           UncoreMinFromTPMIScriptName,
		ScriptTemplate: "pcm-tpmi 2 0x18 -d -b 15:21",
		Architectures:  []string{x86_64},
		Families:       []string{"6"},                        // Intel
		Models:         []string{"173", "174", "175", "221"}, // GNR, GNR-D, SRF, CWF
		Depends:        []string{"pcm-tpmi"},
		Superuser:      true,
	},
	UncoreDieTypesFromTPMIScriptName: {
		Name:           UncoreDieTypesFromTPMIScriptName,
		ScriptTemplate: "pcm-tpmi 2 0x10 -d -b 26:26",
		Architectures:  []string{x86_64},
		Families:       []string{"6"},                        // Intel
		Models:         []string{"173", "174", "175", "221"}, // GNR, GNR-D, SRF, CWF
		Depends:        []string{"pcm-tpmi"},
		Superuser:      true,
	},
	ElcScriptName: {
		Name: ElcScriptName,
		ScriptTemplate: `# Script derived from bhs-power-mode script in Intel PCM repository
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
		Families:      []string{"6"},                        // Intel
		Models:        []string{"173", "174", "175", "221"}, // GNR, GNR-D, SRF, CWF
		Depends:       []string{"pcm-tpmi"},
		Superuser:     true,
	},
	SSTTFHPScriptName: {
		Name: SSTTFHPScriptName,
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
		Families:      []string{"6"},          // Intel
		Models:        []string{"173", "174"}, // GNR, GNR-D
		Depends:       []string{"pcm-tpmi"},
		Superuser:     true,
	},
	SSTTFLPScriptName: {
		Name: SSTTFLPScriptName,
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
		Families:      []string{"6"},          // Intel
		Models:        []string{"173", "174"}, // GNR, GNR-D
		Depends:       []string{"pcm-tpmi"},
		Superuser:     true,
	},
	ChaCountScriptName: {
		Name: ChaCountScriptName,
		ScriptTemplate: `rdmsr 0x396
rdmsr 0x702
rdmsr 0x2FFE
`, // uncore client cha count, uncore cha count, uncore cha count spr
		Architectures: []string{x86_64},
		Vendors:       []string{"GenuineIntel"},
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
		ScriptTemplate: `for ifc_path in /sys/class/net/*; do
	ifc=$(basename "$ifc_path")
	if [ "$ifc" = "lo" ]; then
		continue
	fi
	if ! ethtool_out=$(ethtool "$ifc" 2>/dev/null); then
		continue
	fi
	if ! ethtool_i_out=$(ethtool -i "$ifc" 2>/dev/null); then
		continue
	fi
	echo "Interface: $ifc"
	udevadm_out=$(udevadm info --query=all --path=/sys/class/net/"$ifc")
	echo "Vendor ID: $(echo "$udevadm_out" | grep ID_VENDOR_ID= | cut -d'=' -f2)"
	echo "Model ID: $(echo "$udevadm_out" | grep ID_MODEL_ID= | cut -d'=' -f2)"
	echo "Vendor: $(echo "$udevadm_out" | grep ID_VENDOR_FROM_DATABASE= | cut -d'=' -f2)"
	echo "Model: $(echo "$udevadm_out" | grep ID_MODEL_FROM_DATABASE= | cut -d'=' -f2)"
	echo "$ethtool_out"
	echo "$ethtool_i_out"
	echo "MAC Address: $(cat /sys/class/net/"$ifc"/address 2>/dev/null)"
	echo "NUMA Node: $(cat /sys/class/net/"$ifc"/device/numa_node 2>/dev/null)"
	echo -n "CPU Affinity: "
	intlist=$( grep -e "$ifc" /proc/interrupts | cut -d':' -f1 | sed -e 's/^[[:space:]]*//' )
	for int in $intlist; do
		cpu=$( cat /proc/irq/"$int"/smp_affinity_list 2>/dev/null)
		printf "%s:%s;" "$int" "$cpu"
	done
	printf "\n"
	echo "IRQ Balance: $(pgrep irqbalance >/dev/null 2>&1 && echo "Enabled" || echo "Disabled")"
	echo "----------------------------------------"
done
`,
		Depends:   []string{"ethtool"},
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
done
`,
	},
	HdparmScriptName: {
		Name: HdparmScriptName,
		ScriptTemplate: `lsblk -d -r -o NAME -e7 -e1 -n | while read -r device ; do
	hdparm -i /dev/"$device"
done
`,
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
	TmeScriptName: {
		Name: TmeScriptName,
		ScriptTemplate: `output=$(dmesg | grep -i "x86/tme")
if [[ $output == *"not enabled by BIOS"* ]]; then
    echo "Disabled"
elif [[ $output == *"enabled"* ]]; then
    echo "Enabled"
else
    echo "Unknown"
fi`,
		Superuser: true,
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
		Vendors:        []string{"GenuineIntel"},
	},
	PMUBusyScriptName: {
		Name: PMUBusyScriptName,
		ScriptTemplate: `# define the list of PMU counters
pmu_counters=(0x30a 0x309 0x30b 0x30c 0xc1 0xc2 0xc3 0xc4 0xc5 0xc6 0xc7 0xc8)

# define the number of times to loop, i.e., read the MSR value
num_loops=6

# initialize an associative array to store the values for each PMU counter
declare -A pmu_values

# read the value of the msr represented by the hex value num_loops times for each PMU counter
for ((j=1; j<=num_loops; j++)); do
    for i in "${pmu_counters[@]}"; do
        val=$(rdmsr $i | tr -d '\n')
        # if the value isn't a hex value, go on to next hex value
        if [[ ! $val =~ ^[0-9a-fA-F]+$ ]]; then
            echo "$i Unknown"
            continue 2
        fi
        # append the value to the array for the current PMU counter
        pmu_values[$i]+="$val "
    done
done

# check if the first and last value in the array are the same for each PMU counter
for i in "${pmu_counters[@]}"; do
    # convert the space-separated string to an array
    arr=(${pmu_values[$i]})
    if [ ${arr[0]} == ${arr[5]} ]; then
        echo "$i Inactive"
    else
        echo "$i Active"
    fi
    # print the full list of PMU values
    echo "Values: ${pmu_values[$i]}"
done
`,
		Superuser:     true,
		Architectures: []string{x86_64},
		Vendors:       []string{"GenuineIntel"},
		Lkms:          []string{"msr"},
		Depends:       []string{"rdmsr"},
	},
	GaudiInfoScriptName: {
		Name:           GaudiInfoScriptName,
		ScriptTemplate: `hl-smi -Q module_id,serial,bus_id,driver_version -f csv`,
		Vendors:        []string{"GenuineIntel"},
	},
	GaudiFirmwareScriptName: {
		Name:           GaudiFirmwareScriptName,
		ScriptTemplate: `hl-smi --fw-version`,
		Vendors:        []string{"GenuineIntel"},
	},
	GaudiNumaScriptName: {
		Name:           GaudiNumaScriptName,
		ScriptTemplate: `hl-smi topo -N`,
		Vendors:        []string{"GenuineIntel"},
	},
	MemoryBenchmarkScriptName: {
		Name: MemoryBenchmarkScriptName,
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
echo $orig_num_huge_pages > /proc/sys/vm/nr_hugepages
`,
		Architectures: []string{x86_64},
		Superuser:     true,
		Lkms:          []string{"msr"},
		Depends:       []string{"mlc"},
		Sequential:    true,
	},
	NumaBenchmarkScriptName: {
		Name: NumaBenchmarkScriptName,
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
echo $orig_num_huge_pages > /proc/sys/vm/nr_hugepages
`,
		Architectures: []string{x86_64},
		Superuser:     true,
		Lkms:          []string{"msr"},
		Depends:       []string{"mlc"},
		Sequential:    true,
	},
	SpeedBenchmarkScriptName: {
		Name: SpeedBenchmarkScriptName,
		ScriptTemplate: `methods=$( stress-ng --cpu 1 --cpu-method x 2>&1 | cut -d":" -f2 | cut -c 6- )
for method in $methods; do
	printf "%s " "$method"
	stress-ng --cpu 0 -t 1 --cpu-method "$method" --metrics-brief 2>&1 | tail -1 | awk '{print $9}'
done
`,
		Superuser:  false,
		Depends:    []string{"stress-ng"},
		Sequential: true,
	},
	FrequencyBenchmarkScriptName: {
		Name: FrequencyBenchmarkScriptName,
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

num_cores_per_socket=$( lscpu | grep -E 'Core\(s\) per socket:' | head -1 | awk '{print $4}' )
# echo "Number of cores per socket: $num_cores_per_socket"
family=$(lscpu | grep -E '^CPU family:' | awk '{print $3}')
model=$(lscpu | grep -E '^Model:' | awk '{print $2}')

# if GNR (family 6, model 173), we need to interleave the core-ids across dies
if [ $family -eq 6 ] && [ $model -eq 173 ]; then
    # Get the number of dies and sockets
    num_devices=$(lspci -d 8086:3258 | wc -l)
    num_sockets=$(lscpu | grep -E '^Socket\(s\):' | awk '{print $2}')
    # echo "Number of devices: $num_devices"
    # echo "Number of sockets: $num_sockets"
    num_devices_per_die=2
    # Calculate the number of dies per socket
    dies_per_socket=$((num_devices / num_sockets / num_devices_per_die))
    # echo "Number of dies per socket: $dies_per_socket"
    # Calculate the number of cores per die
    cores_per_die=$((num_cores_per_socket / dies_per_socket))
    # echo "Number of cores per die: $cores_per_die"

    # Array to hold the expanded core lists for each die
    declare -a core_lists

    # Loop through each die in the first socket and expand the core IDs
    for ((i=0; i<dies_per_socket; i++)); do
        core_range_start=$((i * cores_per_die))
        core_range_end=$((core_range_start + cores_per_die - 1))
        core_range="$core_range_start-$core_range_end"
        # echo "Core range for die $i: $core_range"
        core_list=$(expand_range "$core_range")
        core_lists+=("$core_list")
    done

    # Interleave the core IDs from each die
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
    cpu_ids="--cpuids=$interleaved_core_list"
else
    cpu_ids=""
fi

avx-turbo --min-threads=1 --max-threads=$num_cores_per_socket --test scalar_iadd,avx256_fma,avx512_fma --iters=100000 $cpu_ids
`,
		Superuser:  true,
		Lkms:       []string{"msr"},
		Depends:    []string{"avx-turbo", "lspci"},
		Sequential: true,
	},
	PowerBenchmarkScriptName: {
		Name:           PowerBenchmarkScriptName,
		ScriptTemplate: `((turbostat -i 2 2>/dev/null &) ; stress-ng --cpu 0 --bsearch 0 -t 60s >/dev/null 2>&1 ; pkill -9 -f turbostat)`,
		Superuser:      true,
		Lkms:           []string{"msr"},
		Depends:        []string{"turbostat", "stress-ng"},
		Sequential:     true,
	},
	IdlePowerBenchmarkScriptName: {
		Name:           IdlePowerBenchmarkScriptName,
		ScriptTemplate: `turbostat -i 2 -n 2 2>/dev/null`,
		Superuser:      true,
		Lkms:           []string{"msr"},
		Depends:        []string{"turbostat"},
		Sequential:     true,
	},
	StorageBenchmarkScriptName: {
		Name: StorageBenchmarkScriptName,
		ScriptTemplate: `
file_size_g=5
numjobs=1
total_file_size_g=$(($file_size_g * $numjobs))
ramp_time=5s
runtime=120s
ioengine=sync
# confirm that .StorageDir is a directory, is writeable, and has enough space
if [[ -d "{{.StorageDir}}" && -w "{{.StorageDir}}" ]]; then
	available_space=$(df -hP "{{.StorageDir}}")
	count=$( echo "$available_space" | awk '/[0-9]%%/{print substr($4,1,length($4)-1)}' )
	unit=$( echo "$available_space" | awk '/[0-9]%%/{print substr($4,length($4),1)}' )
	is_enough_gigabytes=$(awk -v c="$count" -v f=$total_file_size_g 'BEGIN{print (c>f)?1:0}')
	is_terabyte_or_more=$(echo "TPEZY" | grep -F -q "$unit" && echo 1 || echo 0)
	if [[ ("$unit" == "G" && "$is_enough_gigabytes" == 0) && "$is_terabyte_or_more" == 1 ]]; then
		echo "ERROR: {{.StorageDir}} does not have enough available space - $total_file_size_g GB required"
		exit 1
	fi
else
	echo "ERROR: {{.StorageDir}} does not exist or is not writeable"
	exit 1
fi
# single-threaded read & write bandwidth test
test_dir="{{.StorageDir}}"/fio_test
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
	MpstatTelemetryScriptName: {
		Name: MpstatTelemetryScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
fi
mpstat -u -T -I SCPU -P ALL $interval $count &
echo $! > {{.ScriptName}}_cmd.pid
wait
`,
		Superuser: true,
		Lkms:      []string{},
		Depends:   []string{"mpstat"},
		NeedsKill: true,
	},
	IostatTelemetryScriptName: {
		Name: IostatTelemetryScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
fi
S_TIME_FORMAT=ISO iostat -d -t $interval $count | sed '/^loop/d' &
echo $! > {{.ScriptName}}_cmd.pid
wait
`,
		Superuser: true,
		Lkms:      []string{},
		Depends:   []string{"iostat"},
		NeedsKill: true,
	},
	MemoryTelemetryScriptName: {
		Name: MemoryTelemetryScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
fi
sar -r $interval $count &
echo $! > {{.ScriptName}}_cmd.pid
wait
`,
		Superuser: true,
		Lkms:      []string{},
		Depends:   []string{"sar", "sadc"},
		NeedsKill: true,
	},
	NetworkTelemetryScriptName: {
		Name: NetworkTelemetryScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
fi
sar -n DEV $interval $count &
echo $! > {{.ScriptName}}_cmd.pid
wait
`,
		Superuser: true,
		Lkms:      []string{},
		Depends:   []string{"sar", "sadc"},
		NeedsKill: true,
	},
	TurbostatTelemetryScriptName: {
		Name: TurbostatTelemetryScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
	count="-n $count"
else
	count=""
fi
echo TIME: $(date +"%H:%M:%S")
echo INTERVAL: $interval
turbostat -i $interval $count &
echo $! > {{.ScriptName}}_cmd.pid
wait
`,
		Superuser: true,
		Lkms:      []string{"msr"},
		Depends:   []string{"turbostat"},
		NeedsKill: true,
	},
	InstructionTelemetryScriptName: {
		Name: InstructionTelemetryScriptName,
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
if [ {{.InstrMixPID}} -eq 0 ]; then
    arg_sampling_rate="-s {{.InstrMixFrequency}}"
else
    arg_pid="-p {{.InstrMixPID}}"
fi
# .InstrMixFilter is a space separated list of ISA categories
# for each category in the list, add -f <category> to the command line
for category in {{.InstrMixFilter}}; do
    arg_filter="$arg_filter -f $category"
done

processwatch -c $arg_sampling_rate $arg_pid $arg_interval $arg_count $arg_filter &
echo $! > {{.ScriptName}}_cmd.pid
wait
`,
		Superuser: true,
		Lkms:      []string{"msr"},
		Depends:   []string{"processwatch"},
		NeedsKill: true,
	},
	GaudiTelemetryScriptName: {
		Name: GaudiTelemetryScriptName,
		ScriptTemplate: `
# if the hl-smi program is in the path
if command -v hl-smi &> /dev/null; then
	hl-smi --query-aip=timestamp,name,temperature.aip,module_id,utilization.aip,memory.total,memory.free,memory.used,power.draw --format=csv,nounits -l {{.Interval}} &
	echo $! > {{.ScriptName}}_cmd.pid
	# if duration is set, sleep for the duration then kill the process
	if [ {{.Duration}} -ne 0 ]; then
		sleep {{.Duration}}
		kill -SIGINT $(cat {{.ScriptName}}_cmd.pid)
	fi
	wait
else
	echo "hl-smi not found in the path" >&2
	exit 1
fi
`,
		Superuser: true,
		NeedsKill: true,
	},
	// flamegraph scripts
	CollapsedCallStacksScriptName: {
		Name: CollapsedCallStacksScriptName,
		ScriptTemplate: `# Combined (perf record and async profiler) call stack collection
pids={{.PIDs}}
duration={{.Duration}}
frequency={{.Frequency}}
maxdepth={{.MaxDepth}}

ap_interval=0
if [ "$frequency" -ne 0 ]; then
    ap_interval=$((1000000000 / frequency))
fi

# Function to restore original settings and clean up
restore_settings() {
    echo "$PERF_EVENT_PARANOID" > /proc/sys/kernel/perf_event_paranoid
    echo "$KPTR_RESTRICT" > /proc/sys/kernel/kptr_restrict
    if [ -n "$perf_fp_pid" ]; then
        kill -0 $perf_fp_pid 2>/dev/null && kill -INT $perf_fp_pid
    fi
    if [ -n "$perf_dwarf_pid" ]; then
        kill -0 $perf_dwarf_pid 2>/dev/null && kill -INT $perf_dwarf_pid
    fi
    for pid in "${java_pids[@]}"; do
        async-profiler/bin/asprof stop -o collapsed "$pid"
    done
}

# Adjust perf_event_paranoid and kptr_restrict
PERF_EVENT_PARANOID=$(cat /proc/sys/kernel/perf_event_paranoid)
echo -1 >/proc/sys/kernel/perf_event_paranoid
KPTR_RESTRICT=$(cat /proc/sys/kernel/kptr_restrict)
echo 0 >/proc/sys/kernel/kptr_restrict

# Ensure settings are restored on exit
trap restore_settings EXIT

# Check if at least one process is running
if [ -n "$pids" ]; then
    IFS=',' read -r -a pid_array <<< "$pids"
    for p in "${pid_array[@]}"; do
        if ps -p "$p" > /dev/null; then
            if tr '\000' ' ' < /proc/"$p"/cmdline | grep -q java; then
                java_pids+=("$p")
            fi
        else
            echo "Error: Process $p is not running." >&2
            exit 1
        fi
    done
else
    mapfile -t java_pids < <(pgrep java)
fi

# Frame pointer mode
if [ -n "$pids" ]; then
    perf record -F "$frequency" -p "$pids" -g -o perf_fp_data -m 129 &
else
    perf record -F "$frequency" -a -g -o perf_fp_data -m 129 &
fi
perf_fp_pid=$!
if ! kill -0 $perf_fp_pid 2>/dev/null; then
    echo "Failed to start perf record in frame pointer mode" >&2
    exit 1
fi

# Dwarf mode
if [ -n "$pids" ]; then
    perf record -F "$frequency" -p "$pids" -g -o perf_dwarf_data -m 257 --call-graph dwarf,8192 &
else
    perf record -F "$frequency" -a -g -o perf_dwarf_data -m 257 --call-graph dwarf,8192 &
fi
perf_dwarf_pid=$!
if ! kill -0 $perf_dwarf_pid 2>/dev/null; then
    echo "Failed to start perf record in dwarf mode" >&2
    exit 1
fi

# Start Java profiling for each Java PID
for pid in "${java_pids[@]}"; do
    java_cmds+=("$(tr '\000' ' ' < /proc/"$pid"/cmdline)")
    async-profiler/bin/asprof start -i "$ap_interval" -F probesp+vtable "$pid"
done

# Wait for the specified duration
sleep "$duration"

# Stop perf recording
if ! kill -0 $perf_fp_pid 2>/dev/null; then
    echo "Frame pointer mode already stopped" >&2
else
    kill -INT $perf_fp_pid
fi
if ! kill -0 $perf_dwarf_pid 2>/dev/null; then
    echo "Dwarf mode already stopped" >&2
else
    kill -INT $perf_dwarf_pid
fi

# Stop Java profiling, write output to ap_folded_<pid> files
for pid in "${java_pids[@]}"; do
    async-profiler/bin/asprof stop -o collapsed -f ap_folded_"$pid" "$pid"
done

# Wait for perf to finish
wait ${perf_fp_pid} ${perf_dwarf_pid}

# Collapse perf data
if [ -f perf_dwarf_data ]; then
    perf script -i perf_dwarf_data > perf_dwarf_stacks
    stackcollapse-perf perf_dwarf_stacks > perf_dwarf_folded
else
    echo "Error: perf_dwarf_data file not found" >&2
fi
if [ -f perf_fp_data ]; then
    perf script -i perf_fp_data > perf_fp_stacks
    stackcollapse-perf perf_fp_stacks > perf_fp_folded
else
    echo "Error: perf_fp_data file not found" >&2
fi

# Dump results to stdout
echo "########## maximum depth ##########"
echo "$maxdepth"

if [ -f perf_dwarf_folded ]; then
    echo "########## perf_dwarf ##########"
    cat perf_dwarf_folded
fi
if [ -f perf_fp_folded ]; then
    echo "########## perf_fp ##########"
    cat perf_fp_folded
fi

for idx in "${!java_pids[@]}"; do
    pid="${java_pids[$idx]}"
    cmd="${java_cmds[$idx]}"
    echo "########## async-profiler $pid $cmd ##########"
    if [ -f ap_folded_"$pid" ]; then
        cat ap_folded_"$pid"
    else
        echo "Error: async-profiler output file not found for PID $pid" >&2
    fi
done
`,
		Superuser:  true,
		Sequential: true,
		Depends:    []string{"async-profiler", "perf", "stackcollapse-perf"},
	},
	// lock analysis scripts
	ProfileKernelLockScriptName: {
		Name: ProfileKernelLockScriptName,
		ScriptTemplate: `frequency={{.Frequency}}
duration={{.Duration}}
package={{.Package}}
# system-wide lock profile collection
# adjust perf_event_paranoid and kptr_restrict
PERF_EVENT_PARANOID=$( cat /proc/sys/kernel/perf_event_paranoid )
echo -1 >/proc/sys/kernel/perf_event_paranoid
KPTR_RESTRICT=$( cat /proc/sys/kernel/kptr_restrict )
echo 0 >/proc/sys/kernel/kptr_restrict

PERF_HOTSPOT_DATA=$(mktemp -d)/perf_hotspot.data
PERF_CONTENTION_DATA=$(mktemp -d)/perf_lock_contention.txt

# collect hotspot
perf record -m 256M --kcore -F $frequency -a -g --call-graph dwarf,512 -W -d --phys-data --sample-cpu -e cycles:pp,instructions:pp,cpu/mem-loads,ldlat=30/P,cpu/mem-stores/P -o ${PERF_HOTSPOT_DATA} -- sleep $duration &
PERF_HOTSPOT_PID=$!

# check the availability perf lock -b option 
perf lock contention -a -bv --max-stack 20 2>/dev/null -- sleep 0
PERF_LOCK_CONTENTION_BPF=$?

# collect lock
if [ ${PERF_LOCK_CONTENTION_BPF} -eq 0 ]; then
	perf lock contention -a -bv --max-stack 20 2>${PERF_CONTENTION_DATA} -- sleep $duration &
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
if [ -d "${PERF_HOTSPOT_DATA}" ]; then
	echo "########## perf_hotspot_no_children ##########"
	perf report -i ${PERF_HOTSPOT_DATA} --no-children --call-graph none --stdio
	echo "########## perf_hotspot_callgraph ##########"
	perf report -i ${PERF_HOTSPOT_DATA} --stdio
	echo "########## perf_c2c_no_children ##########"
	perf c2c report  -i ${PERF_HOTSPOT_DATA} --call-graph none --stdio
	echo "########## perf_c2c_callgraph ##########"
	perf c2c report  -i ${PERF_HOTSPOT_DATA} --stdio

	if [ "${package,,}" = "true" ]; then
		echo "########## perf_package_path ##########"
		PERF_HOTSPOT_DATA_DIR=$(dirname "${PERF_HOTSPOT_DATA}")
		( cd ${PERF_HOTSPOT_DATA_DIR}; perf archive --all ${PERF_HOTSPOT_DATA} > /dev/null 2>&1; chown ${SUDO_UID}.${SUDO_UID} -R ${PERF_HOTSPOT_DATA_DIR} )
		ls ${PERF_HOTSPOT_DATA_DIR}/perf.all*.tar.bz2
	fi
fi
if [ -f "${PERF_CONTENTION_DATA}" ]; then
	echo "########## perf_lock_contention ##########"
	cat ${PERF_CONTENTION_DATA}
fi
`,
		Superuser: true,
		Depends:   []string{"perf", "perf-archive"},
	},
}
