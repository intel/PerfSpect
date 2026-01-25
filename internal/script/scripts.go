// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package script

import (
	"bytes"
	"perfspect/internal/cpus"
	texttemplate "text/template" // nosemgrep
)

// scripts.go defines the bash scripts that are used to collect information from target systems

type ScriptDefinition struct {
	Name               string   // just a name
	ScriptTemplate     string   // the bash script that will be run
	Architectures      []string // architectures, i.e., x86_64, aarch64. If empty, it will run on all architectures.
	Vendors            []string // vendors, i.e., GenuineIntel, AuthenticAMD. If empty, it will run on all vendors.
	MicroArchitectures []string // microarchitectures, e.g., SPR, EMR. If empty, it will run on all microarchitectures.
	Lkms               []string // loadable kernel modules
	Depends            []string // binary dependencies that must be available for the script to run
	Superuser          bool     // requires sudo or root
	Sequential         bool     // run script sequentially (not at the same time as others)
}

// script names, these must be unique
const (
	// report and configuration (reading) scripts
	HostnameScriptName               = "hostname"
	DateScriptName                   = "date"
	DmidecodeScriptName              = "dmidecode"
	LscpuScriptName                  = "lscpu"
	LscpuCacheScriptName             = "lscpu cache"
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
	IRQBalanceScriptName             = "irq balance"
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
	GaudiArchitectureScriptName      = "gaudi architecture"
	ArmImplementerScriptName         = "arm implementer"
	ArmPartScriptName                = "arm part"
	ArmDmidecodePartScriptName       = "arm dmidecode part"
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
	PDUTelemetryScriptName         = "pdu telemetry"
	// flamegraph scripts
	FlameGraphScriptName = "flamegraph"
	// lock scripts
	ProfileKernelLockScriptName = "profile kernel lock"
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
	LscpuCacheScriptName: {
		Name:           LscpuCacheScriptName,
		ScriptTemplate: `lscpu -C`,
	},
	LspciBitsScriptName: {
		Name:               LspciBitsScriptName,
		ScriptTemplate:     `lspci -s $(lspci | grep 325b | awk 'NR==1{{"{"}}print $1{{"}"}}') -xxx |  awk '$1 ~ /^90/{{"{"}}print $9 $8 $7 $6; exit{{"}"}}'`,
		MicroArchitectures: []string{cpus.UarchSPR, cpus.UarchEMR},
		Superuser:          true,
		Depends:            []string{"lspci"},
	},
	LspciDevicesScriptName: {
		Name:               LspciDevicesScriptName,
		ScriptTemplate:     "lspci -d 8086:3258 | wc -l",
		MicroArchitectures: []string{cpus.UarchGNR, cpus.UarchGNR_D, cpus.UarchSRF, cpus.UarchCWF, cpus.UarchDMR},
		Depends:            []string{"lspci"},
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
		Vendors:   []string{cpus.IntelVendor},
		Lkms:      []string{"msr"},
		Depends:   []string{"rdmsr"},
		Superuser: true,
	},
	SpecCoreFrequenciesScriptName: {
		Name: SpecCoreFrequenciesScriptName,
		ScriptTemplate: `lscpu=$(lscpu)
family=$(echo "$lscpu" | grep -E "^CPU family:" | awk '{print $3}')
model=$(echo "$lscpu" | grep -E "^Model:" | awk '{print $2}')
# if cpu is GNR, GNR-D, or DMR get the frequencies from tpmi
if ( [ "$family" -eq 6 ] && [ "$model" -eq 173 ] ) || ( [ "$family" -eq 6 ] && [ "$model" -eq 174 ] ) || ( [ "$family" -eq 19 ] && [ "$model" -eq 1 ] ); then  # GNR, GNR-D, DMR
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
		Vendors:   []string{cpus.IntelVendor},
		Lkms:      []string{"msr"},
		Depends:   []string{"rdmsr", "pcm-tpmi"},
		Superuser: true,
	},
	PPINName: {
		Name:           PPINName,
		ScriptTemplate: "rdmsr -a 0x4f", // MSR_PPIN: Protected Processor Inventory Number
		Vendors:        []string{cpus.IntelVendor},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	PrefetchControlName: {
		Name:           PrefetchControlName,
		ScriptTemplate: "rdmsr -f 7:0 0x1a4", // MSR_PREFETCH_CONTROL: L2, DCU, and AMP Prefetchers enabled/disabled
		Vendors:        []string{cpus.IntelVendor},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	PrefetchersName: {
		Name:           PrefetchersName,
		ScriptTemplate: "rdmsr 0x6d", // TODO: get name, used to read prefetchers
		Vendors:        []string{cpus.IntelVendor},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	PrefetchersAtomName: {
		Name:               PrefetchersAtomName,
		ScriptTemplate:     "rdmsr 0x1320", // Atom Pref_tuning1
		Vendors:            []string{cpus.IntelVendor},
		MicroArchitectures: []string{cpus.UarchSRF, cpus.UarchCWF}, // SRF, CWF
		Lkms:               []string{"msr"},
		Depends:            []string{"rdmsr"},
		Superuser:          true,
	},
	L3CacheWayEnabledName: {
		Name:           L3CacheWayEnabledName,
		ScriptTemplate: "rdmsr 0xc90", // TODO: get name, used to read l3 size
		Vendors:        []string{cpus.IntelVendor},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	PackagePowerLimitName: {
		Name:           PackagePowerLimitName,
		ScriptTemplate: "rdmsr -f 14:0 0x610", // MSR_PKG_POWER_LIMIT: Package limit in bits 14:0
		Vendors:        []string{cpus.IntelVendor},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EpbSourceScriptName: {
		Name:           EpbSourceScriptName,
		ScriptTemplate: "rdmsr -f 34:34 0x1FC", // MSR_POWER_CTL, PWR_PERF_TUNING_ALT_EPB: Energy Performance Bias Hint Source (1 is from BIOS, 0 is from OS)
		Vendors:        []string{cpus.IntelVendor},
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
		Vendors:   []string{cpus.IntelVendor},
		Lkms:      []string{"msr"},
		Depends:   []string{"rdmsr"},
		Superuser: true,
	},
	EppValidScriptName: {
		Name:           EppValidScriptName,
		ScriptTemplate: "rdmsr -a -f 60:60 0x774", // IA32_HWP_REQUEST: Energy Performance Preference, bit 60 indicates if per-cpu EPP is valid
		Vendors:        []string{cpus.IntelVendor},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EppPackageControlScriptName: {
		Name:           EppPackageControlScriptName,
		ScriptTemplate: "rdmsr -a -f 42:42 0x774", // IA32_HWP_REQUEST: Energy Performance Preference, bit 42 indicates if package control is enabled
		Vendors:        []string{cpus.IntelVendor},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EppScriptName: {
		Name:           EppScriptName,
		ScriptTemplate: "rdmsr -a -f 31:24 0x774", // IA32_HWP_REQUEST: Energy Performance Preference, bits 24-31 (0 is highest perf, 255 is highest energy saving)
		Vendors:        []string{cpus.IntelVendor},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	EppPackageScriptName: {
		Name:           EppPackageScriptName,
		ScriptTemplate: "rdmsr -f 31:24 0x772", // IA32_HWP_REQUEST_PKG: Energy Performance Preference, bits 24-31 (0 is highest perf, 255 is highest energy saving)
		Vendors:        []string{cpus.IntelVendor},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	UncoreMaxFromMSRScriptName: {
		Name:           UncoreMaxFromMSRScriptName,
		ScriptTemplate: "rdmsr -f 6:0 0x620", // MSR_UNCORE_RATIO_LIMIT: MAX_RATIO in bits 6:0
		Vendors:        []string{cpus.IntelVendor},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	UncoreMinFromMSRScriptName: {
		Name:           UncoreMinFromMSRScriptName,
		ScriptTemplate: "rdmsr -f 14:8 0x620", // MSR_UNCORE_RATIO_LIMIT: MAX_RATIO in bits 14:8
		Vendors:        []string{cpus.IntelVendor},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	},
	UncoreMaxFromTPMIScriptName: {
		Name:               UncoreMaxFromTPMIScriptName,
		ScriptTemplate:     "pcm-tpmi 2 0x18 -d -b 8:14",
		MicroArchitectures: []string{cpus.UarchGNR, cpus.UarchGNR_D, cpus.UarchSRF, cpus.UarchCWF, cpus.UarchDMR},
		Depends:            []string{"pcm-tpmi"},
		Superuser:          true,
	},
	UncoreMinFromTPMIScriptName: {
		Name:               UncoreMinFromTPMIScriptName,
		ScriptTemplate:     "pcm-tpmi 2 0x18 -d -b 15:21",
		MicroArchitectures: []string{cpus.UarchGNR, cpus.UarchGNR_D, cpus.UarchSRF, cpus.UarchCWF, cpus.UarchDMR},
		Depends:            []string{"pcm-tpmi"},
		Superuser:          true,
	},
	UncoreDieTypesFromTPMIScriptName: {
		Name:               UncoreDieTypesFromTPMIScriptName,
		ScriptTemplate:     "pcm-tpmi 2 0x10 -d -b 26:26",
		MicroArchitectures: []string{cpus.UarchGNR, cpus.UarchGNR_D, cpus.UarchSRF, cpus.UarchCWF, cpus.UarchDMR},
		Depends:            []string{"pcm-tpmi"},
		Superuser:          true,
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
	eff_latency_ctrl_low_threshold=$(( (eff_latency_ctrl_low_threshold * 100) / 100 ))
	eff_latency_ctrl_high_threshold=$(( (eff_latency_ctrl_high_threshold * 100) / 100 ))

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
		MicroArchitectures: []string{cpus.UarchGNR, cpus.UarchGNR_D, cpus.UarchSRF, cpus.UarchCWF, cpus.UarchDMR},
		Depends:            []string{"pcm-tpmi"},
		Superuser:          true,
	},
	SSTTFHPScriptName: {
		Name: SSTTFHPScriptName,
		ScriptTemplate: `# Is SST-TF supported?
if ! supported=$(pcm-tpmi 5 0xF8 -d -b 12:12 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}'); then
    echo "Error: Failed to check if SST-TF is supported" >&2
    exit 1
fi
if [[ ! "$supported" =~ ^[0-9]+$ ]]; then
	echo "Error: Invalid output from pcm-tpmi when checking support" >&2
	exit 1
fi
if [ "$supported" -eq 0 ]; then
	echo "SST-TF is not supported"
	exit 0
fi
# Is SST-TF enabled?
if ! enabled=$(pcm-tpmi 5 0x78 -d -b 9:9 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}'); then
    echo "Error: Failed to check if SST-TF is enabled" >&2
    exit 1
fi
if [[ ! "$enabled" =~ ^[0-9]+$ ]]; then
	echo "Error: Invalid output from pcm-tpmi when checking enabled status" >&2
	exit 1
fi
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
	if ! numcores=$(pcm-tpmi 5 0x100 -d -b $bithigh:$bitlow -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}'); then
		echo "Error: Failed to get number of cores for bucket $i" >&2
		exit 1
	fi
	if [[ ! "$numcores" =~ ^[0-9]+$ ]]; then
		echo "Error: Invalid output from pcm-tpmi when getting number of cores for bucket $i" >&2
		exit 1
	fi
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
		offset=$((j*8 + 264)) # 264 is 0x108 (SST_TF_INFO_2) AVX
		if ! freq=$(pcm-tpmi 5 $offset -d -b $bithigh:$bitlow -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}'); then
			echo "Error: Failed to get frequency for instruction set $j in bucket $i" >&2
			exit 1
		fi
		if [[ ! "$freq" =~ ^[0-9]+$ ]]; then
			echo "Error: Invalid frequency value for instruction set $j in bucket $i" >&2
			exit 1
		fi
		echo -n "$freq"
		if [ $j -lt 4 ]; then
			echo -n ","
		fi
	done
	echo "" # finish the line
done
`,
		MicroArchitectures: []string{cpus.UarchGNR, cpus.UarchGNR_D, cpus.UarchDMR},
		Depends:            []string{"pcm-tpmi"},
		Superuser:          true,
	},
	SSTTFLPScriptName: {
		Name: SSTTFLPScriptName,
		ScriptTemplate: `# Is SST-TF supported?
if ! supported=$(pcm-tpmi 5 0xF8 -d -b 12:12 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}'); then
    echo "Error: Failed to check if SST-TF is supported" >&2
    exit 1
fi
if [[ ! "$supported" =~ ^[0-9]+$ ]]; then
	echo "Error: Invalid output from pcm-tpmi when checking support" >&2
	exit 1
fi

if [ "$supported" -eq 0 ]; then
	echo "SST-TF is not supported"
	exit 0
fi
# Is SST-TF enabled?
if ! enabled=$(pcm-tpmi 5 0x78 -d -b 9:9 -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}'); then
    echo "Error: Failed to check if SST-TF is enabled" >&2
    exit 1
fi
if [[ ! "$enabled" =~ ^[0-9]+$ ]]; then
	echo "Error: Invalid output from pcm-tpmi when checking enabled status" >&2
	exit 1
fi

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
	if ! freq=$(pcm-tpmi 5 0xF8 -d -b $bithigh:$bitlow -i 0 -e 0 | tail -n 2 | head -n 1 | awk '{print $5}'); then
		echo "Error: Failed to get frequency for instruction set $j" >&2
		exit 1
	fi
	if [[ ! "$freq" =~ ^[0-9]+$ ]]; then
		echo "Error: Invalid frequency value for instruction set $j" >&2
		exit 1
	fi
	echo -n "$freq"
	if [ $j -ne 4 ]; then
		echo -n ","
	fi
done
echo "" # finish the line
`,
		MicroArchitectures: []string{cpus.UarchGNR, cpus.UarchGNR_D, cpus.UarchDMR},
		Depends:            []string{"pcm-tpmi"},
		Superuser:          true,
	},
	ChaCountScriptName: {
		Name: ChaCountScriptName,
		ScriptTemplate: `rdmsr 0x396
rdmsr 0x702
rdmsr 0x2FFE
`, // uncore client cha count, uncore cha count, uncore cha count spr
		Vendors:   []string{cpus.IntelVendor},
		Lkms:      []string{"msr"},
		Depends:   []string{"rdmsr"},
		Superuser: true,
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
	echo "MTU: $(cat /sys/class/net/"$ifc"/mtu 2>/dev/null)"
	echo "$ethtool_out"
	echo "$ethtool_i_out"
	if ethtool_c_out=$(ethtool -c "$ifc" 2>/dev/null); then
		echo "$ethtool_c_out"
	fi
	echo "MAC Address: $(cat /sys/class/net/"$ifc"/address 2>/dev/null)"
	echo "NUMA Node: $(cat /sys/class/net/"$ifc"/device/numa_node 2>/dev/null)"
	# Check if this is a virtual function
	if [ -L /sys/class/net/"$ifc"/device/physfn ]; then
		echo "Virtual Function: yes"
	else
		echo "Virtual Function: no"
	fi
	echo -n "CPU Affinity: "
	intlist=$( grep -e "$ifc" /proc/interrupts | cut -d':' -f1 | sed -e 's/^[[:space:]]*//' )
	for int in $intlist; do
		cpu=$( cat /proc/irq/"$int"/smp_affinity_list 2>/dev/null)
		printf "%s:%s;" "$int" "$cpu"
	done
	printf "\n"
	echo "TX Queues: $(ls -d /sys/class/net/"$ifc"/queues/tx-* | wc -l)"
	echo "RX Queues: $(ls -d /sys/class/net/"$ifc"/queues/rx-* | wc -l)"
	for q in /sys/class/net/"$ifc"/queues/tx-*; do
		if [ -f "$q/xps_cpus" ]; then
			echo "xps_cpus $(basename "$q"): $(cat "$q/xps_cpus")"
		fi
	done
	for q in /sys/class/net/"$ifc"/queues/rx-*; do
		if [ -f "$q/rps_cpus" ]; then
			echo "rps_cpus $(basename "$q"): $(cat "$q/rps_cpus")"
		fi
	done
	echo "----------------------------------------"
done
`,
		Depends:   []string{"ethtool"},
		Superuser: true,
	},
	IRQBalanceScriptName: {
		Name:           IRQBalanceScriptName,
		ScriptTemplate: "pgrep irqbalance >/dev/null 2>&1 && echo 'Enabled' || echo 'Disabled'",
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
		Vendors:        []string{cpus.IntelVendor},
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
		Superuser: true,
		Vendors:   []string{cpus.IntelVendor},
		Lkms:      []string{"msr"},
		Depends:   []string{"rdmsr"},
	},
	GaudiInfoScriptName: {
		Name:           GaudiInfoScriptName,
		ScriptTemplate: `hl-smi -Q module_id,serial,bus_id,driver_version -f csv`,
		Vendors:        []string{cpus.IntelVendor},
	},
	GaudiFirmwareScriptName: {
		Name:           GaudiFirmwareScriptName,
		ScriptTemplate: `hl-smi --fw-version`,
		Vendors:        []string{cpus.IntelVendor},
	},
	GaudiNumaScriptName: {
		Name:           GaudiNumaScriptName,
		ScriptTemplate: `hl-smi topo -N`,
		Vendors:        []string{cpus.IntelVendor},
	},
	GaudiArchitectureScriptName: {
		Name: GaudiArchitectureScriptName,
		ScriptTemplate: `# Determine the default HL_DEVICE based on PCI ID
__DEFAULT_HL_DEVICE=
__pcidev=$(grep PCI_ID /sys/bus/pci/devices/*/uevent | grep -i 1da3: || echo "")
if echo $__pcidev | grep -qE '1000|1001|1010|1011'; then
	__DEFAULT_HL_DEVICE="gaudi"
elif echo $__pcidev | grep -qE '1020|1030'; then
	__DEFAULT_HL_DEVICE="gaudi2"
elif echo $__pcidev | grep -qE '106[0-9]'; then
	__DEFAULT_HL_DEVICE="gaudi3"
fi
echo $__DEFAULT_HL_DEVICE
`,
		Vendors: []string{cpus.IntelVendor},
	},
	ArmImplementerScriptName: {
		Name:           ArmImplementerScriptName,
		ScriptTemplate: "grep -i \"^CPU implementer\" /proc/cpuinfo | head -1 | awk '{print $NF}'",
		Architectures:  []string{cpus.ARMArchitecture},
	},
	ArmPartScriptName: {
		Name:           ArmPartScriptName,
		ScriptTemplate: "grep -i \"^CPU part\" /proc/cpuinfo | head -1 | awk '{print $NF}'",
		Architectures:  []string{cpus.ARMArchitecture},
	},
	ArmDmidecodePartScriptName: {
		Name:           ArmDmidecodePartScriptName,
		ScriptTemplate: "dmidecode -t processor | grep -m 1 \"Part Number\" | awk -F': ' '{print $2}'",
		Architectures:  []string{cpus.ARMArchitecture},
		Superuser:      true,
		Depends:        []string{"dmidecode"},
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
mlc --loaded_latency -b500m -X
echo $orig_num_huge_pages > /proc/sys/vm/nr_hugepages
`,
		Superuser:  true,
		Lkms:       []string{"msr"},
		Depends:    []string{"mlc"},
		Sequential: true,
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
mlc --bandwidth_matrix -b500m -X
echo $orig_num_huge_pages > /proc/sys/vm/nr_hugepages
`,
		Superuser:  true,
		Lkms:       []string{"msr"},
		Depends:    []string{"mlc"},
		Sequential: true,
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
		Name:          FrequencyBenchmarkScriptName,
		Architectures: []string{cpus.X86Architecture},
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
		Architectures:  []string{cpus.X86Architecture},
		ScriptTemplate: `((turbostat -i 2 2>/dev/null &) ; stress-ng --cpu 0 --bsearch 0 -t 60s >/dev/null 2>&1 ; pkill -9 -f turbostat)`,
		Superuser:      true,
		Lkms:           []string{"msr"},
		Depends:        []string{"turbostat", "stress-ng"},
		Sequential:     true,
	},
	IdlePowerBenchmarkScriptName: {
		Name:           IdlePowerBenchmarkScriptName,
		Architectures:  []string{cpus.X86Architecture},
		ScriptTemplate: `turbostat -i 2 -n 2 2>/dev/null`,
		Superuser:      true,
		Lkms:           []string{"msr"},
		Depends:        []string{"turbostat"},
		Sequential:     true,
	},
	StorageBenchmarkScriptName: {
		Name: StorageBenchmarkScriptName,
		ScriptTemplate: `
test_dir=$(mktemp -d --tmpdir="{{.StorageDir}}")
numjobs_bw=16
file_size_bw_g=1
space_needed_bw_k=$(( (file_size_bw_g + 1) * 1024 * 1024 * numjobs_bw )) # space needed in kilobytes: (file_size_bw_g + 1) GB per job
runtime=30s

# check if .StorageDir is a directory
if [[ ! -d "{{.StorageDir}}" ]]; then
        echo "ERROR: {{.StorageDir}} does not exist"
        exit 1
fi
# check if .StorageDir is writeable
if [[ ! -w "{{.StorageDir}}" ]]; then
        echo "ERROR: {{.StorageDir}} is not writeable"
        exit 1
fi
# check if .StorageDir has enough space
# example output for df -P /tmp:
# Filesystem     1024-blocks      Used Available Capacity Mounted on
# /dev/sdd        1055762868 196668944 805390452      20% /
available_space=$(df -P "{{.StorageDir}}" | awk 'NR==2 {print $4}')
if [[ $available_space -lt $space_needed_bw_k ]]; then
        echo "ERROR: {{.StorageDir}} has ${available_space}K available space. A minimum of ${space_needed_bw_k}K is required to run the IO bandwidth benchmark job."
        exit 1
fi

sync
/sbin/sysctl -w vm.drop_caches=3 || true

FIO_JOBFILE=$(mktemp $test_dir/fio-job-XXXXXX.fio)
cat > $FIO_JOBFILE <<EOF
[global]
ioengine=libaio
direct=1
size=5G
ramp_time=5s
time_based
create_on_open=1
unlink=1
directory=$test_dir

[iodepth_1_bs_4k_rand]
wait_for_previous
runtime=${runtime}
rw=randrw
iodepth=1
blocksize=4k
iodepth_batch_submit=1
iodepth_batch_complete_max=1

[iodepth_256_bs_4k_rand]
wait_for_previous
runtime=${runtime}
rw=randrw
iodepth=256
blocksize=4k
iodepth_batch_submit=256
iodepth_batch_complete_max=256

[iodepth_1_bs_1M_numjobs_${numjobs_bw}]
wait_for_previous
size=${file_size_bw_g}G
runtime=${runtime}
rw=readwrite
iodepth=1
iodepth_batch_submit=1
iodepth_batch_complete_max=1
blocksize=1M
numjobs=$numjobs_bw
group_reporting=1

[iodepth_64_bs_1M_numjobs_${numjobs_bw}]
wait_for_previous
size=${file_size_bw_g}G
runtime=${runtime}
rw=readwrite
iodepth=64
iodepth_batch_submit=64
iodepth_batch_complete_max=64
blocksize=1M
numjobs=$numjobs_bw
group_reporting=1
EOF

fio --output-format=json $FIO_JOBFILE

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
LC_TIME=C mpstat -u -T -I SCPU -P ALL $interval $count
`,
		Superuser: false,
		Lkms:      []string{},
		Depends:   []string{"mpstat"},
	},
	IostatTelemetryScriptName: {
		Name: IostatTelemetryScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
fi
S_TIME_FORMAT=ISO iostat -d -t $interval $count | sed '/^loop/d'
`,
		Superuser: false,
		Lkms:      []string{},
		Depends:   []string{"iostat"},
	},
	MemoryTelemetryScriptName: {
		Name: MemoryTelemetryScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
fi
LC_TIME=C sar -r $interval $count
`,
		Superuser: false,
		Lkms:      []string{},
		Depends:   []string{"sar", "sadc"},
	},
	NetworkTelemetryScriptName: {
		Name: NetworkTelemetryScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
	count=$((duration / interval))
fi
LC_TIME=C sar -n DEV $interval $count
`,
		Superuser: false,
		Lkms:      []string{},
		Depends:   []string{"sar", "sadc"},
	},
	TurbostatTelemetryScriptName: {
		Name:          TurbostatTelemetryScriptName,
		Architectures: []string{cpus.X86Architecture},
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
turbostat -i $interval $count
`,
		Superuser: true,
		Lkms:      []string{"msr"},
		Depends:   []string{"turbostat"},
	},
	InstructionTelemetryScriptName: {
		Name: InstructionTelemetryScriptName,
		ScriptTemplate: `interval={{.Interval}}
duration={{.Duration}}
pid={{.InstrMixPID}}

cleanup_done=0

finalize() {
    if [ $cleanup_done -eq 1 ]; then
        return
    fi
    cleanup_done=1
    
    # Explicitly write to fd 2 to ensure it reaches stderr
    echo "Finalizing instruction mix telemetry (pw_pid=$pw_pid)..." 1>&2
    
    # kill the processwatch pipeline process group if it is still running
    if [ -n "${pw_pid:-}" ] && [ "$pw_pid" -gt 0 ]; then
        # Try SIGTERM first
        kill -TERM -"$pw_pid" 2>/dev/null || true
        sleep 0.1
        # Then SIGKILL if needed
        kill -KILL -"$pw_pid" 2>/dev/null || true
    fi
}
trap finalize INT TERM EXIT

if [ $duration -ne 0 ] && [ $interval -ne 0 ]; then
    count=$((duration / interval))
    arg_count="-n $count"
fi
if [ $interval -ne 0 ]; then
    arg_interval="-i $interval"
fi
echo TIME: "$(date +"%H:%M:%S")"
echo INTERVAL: $interval
# if no PID specified, increase the sampling interval (defaults to 100,000) to reduce overhead
if [ $pid -eq 0 ]; then
    arg_sampling_rate="-s {{.InstrMixFrequency}}"
else
    arg_pid="-p $pid"
fi
# -c: CSV output, -a: all categories, -p: PID, -s: sampling rate, -i: interval, -n: count
# example output:
# interval,pid,name,INVALID,ADOX_ADCX,AES
# 0,ALL,ALL,0.000000,0.000000,0.000000,0.000000
# 0,2038501,stress-ng-cpu,0.000000,0.000000,0.000000,0.000000
# We only need the header line and the subsequent "ALL" lines (for each interval)
# Filter output: keep header (NR==1) and lines where 2nd and 3rd columns are ALL
( processwatch -c -a "$arg_pid" "$arg_sampling_rate" "$arg_interval" "$arg_count" | awk -F',' 'NR==1 || ($2=="ALL" && $3=="ALL")' ) &
pw_pid=$!
# wait for processwatch subshell to finish. It will finish naturally if a duration is provided,
# otherwise it will be killed in the finalize() function upon receiving a signal.
wait $pw_pid 2>/dev/null || true
finalize
`,
		Superuser: true,
		Lkms:      []string{"msr"},
		Depends:   []string{"processwatch"},
	},
	GaudiTelemetryScriptName: {
		Name: GaudiTelemetryScriptName,
		ScriptTemplate: `
if command -v {{.GaudiHlsmiPath}} &> /dev/null; then
    {{.GaudiHlsmiPath}} --query-aip=timestamp,name,temperature.aip,module_id,utilization.aip,memory.total,memory.free,memory.used,power.draw --format=csv,nounits -l {{.Interval}} -d {{.Duration}}
else
    echo "hl-smi not found at {{.GaudiHlsmiPath}}" >&2
    exit 1
fi
`,
		Superuser: true,
	},
	PDUTelemetryScriptName: {
		Name: PDUTelemetryScriptName,
		ScriptTemplate: `
duration={{.Duration}}       # total duration in seconds
interval={{.Interval}}       # time between readings in seconds
pdu="{{.PDUHost}}"           # PDU hostname or IP address (must not start with protocol like http://, may include port)
pdu_ip=$(echo "$pdu" | awk -F/ '{print $NF}' | awk -F: '{print $1}') # remove http:// or https:// and port if present
pdu_username="{{.PDUUser}}"
pdu_password="{{.PDUPassword}}"
outletgroup="{{.PDUOutlet}}"
count=$((duration / interval))
echo "Timestamp,ActivePower(W)"
for ((i=0; i<count; i++)); do
    timestamp=$(date +%H:%M:%S)
    response=$(no_proxy=$pdu_ip curl --max-time $interval -sS -k -u "$pdu_username:$pdu_password" \
        -d "{'jsonrpc':'2.0','method':'getReading'}" \
        "https://${pdu}/model/outletgroup/${outletgroup}/activePower")
    # Check if curl succeeded
    if [ $? -ne 0 ] || [ -z "$response" ]; then
        echo "ERROR: Failed to retrieve data from PDU" >&2
        exit 1
    fi
    # Try to parse the value using jq
    w=$(echo "$response" | jq -r '.result._ret_.value // empty')
    if [ -z "$w" ]; then
        echo "ERROR: Invalid response format or missing value" >&2
        exit 1
    fi
    echo "$timestamp,$w"
    sleep $interval
done
`,
		Superuser: false,
	},
	// flamegraph scripts
	FlameGraphScriptName: {
		Name: FlameGraphScriptName,
		ScriptTemplate: `# Combined (perf record and async profiler) call stack collection
pids={{.PIDs}}
duration={{.Duration}}
frequency={{.Frequency}}
maxdepth={{.MaxDepth}}
perf_event={{.PerfEvent}}

ap_interval=0
if [ "$frequency" -ne 0 ]; then
    ap_interval=$((1000000000 / frequency))
fi

# Function to stop profiling
stop_profiling() {
    if [ -n "$perf_fp_pid" ]; then
        kill -0 "$perf_fp_pid" 2>/dev/null && kill -INT "$perf_fp_pid"
        wait "$perf_fp_pid" || true
    fi
    if [ -n "$perf_dwarf_pid" ]; then
        kill -0 "$perf_dwarf_pid" 2>/dev/null && kill -INT "$perf_dwarf_pid"
        wait "$perf_dwarf_pid" || true
    fi
    for pid in "${java_pids[@]}"; do
        async-profiler/bin/asprof stop -o collapsed -f ap_folded_"$pid" "$pid"
    done
    # Restore original settings
    echo "$perf_event_paranoid" > /proc/sys/kernel/perf_event_paranoid
    echo "$kptr_restrict" > /proc/sys/kernel/kptr_restrict
}

# Function to collapse perf data
collapse_perf_data() {
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
}

# Function to print results to stdout
print_results() {
    echo "########## maximum depth ##########"
    echo "$maxdepth"

	echo "########## perf_event ##########"
	echo "$perf_event"

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
}

_finalize=0 # flag to indicate if finalize has been called

# Function to finalize profiling and output
finalize() {
    if [ $_finalize -eq 1 ]; then
        return
    fi
    _finalize=1
    stop_profiling
    collapse_perf_data
    print_results
    rm -f controller.pid
    exit 0
}

# Adjust perf_event_paranoid and kptr_restrict
perf_event_paranoid=$(cat /proc/sys/kernel/perf_event_paranoid)
echo -1 >/proc/sys/kernel/perf_event_paranoid
kptr_restrict=$(cat /proc/sys/kernel/kptr_restrict)
echo 0 >/proc/sys/kernel/kptr_restrict

# If pids specified, check if at least one of them is running
if [ -n "$pids" ]; then
    IFS=',' read -r -a pid_array <<< "$pids"
    for p in "${pid_array[@]}"; do
        if ps -p "$p" > /dev/null; then
            if tr '\000' ' ' < /proc/"$p"/cmdline | grep -q java; then
                java_pids+=("$p")
            fi
        else
            echo "Error: Process $p is not running." >&2
            stop_profiling
            exit 1
        fi
    done
else
    mapfile -t java_pids < <(pgrep java)
fi

# Start profiling with perf in frame pointer mode
if [ -n "$pids" ]; then
    perf record -e "$perf_event" -F "$frequency" -p "$pids" -g -o perf_fp_data -m 129 &
else
    perf record -e "$perf_event" -F "$frequency" -a -g -o perf_fp_data -m 129 &
fi
perf_fp_pid=$!
if ! kill -0 $perf_fp_pid 2>/dev/null; then
    echo "Failed to start perf record in frame pointer mode" >&2
    stop_profiling
    exit 1
fi

# Start profiling with perf in dwarf mode
if [ -n "$pids" ]; then
    perf record -e "$perf_event" -F "$frequency" -p "$pids" -g -o perf_dwarf_data -m 257 --call-graph dwarf,8192 &
else
    perf record -e "$perf_event" -F "$frequency" -a -g -o perf_dwarf_data -m 257 --call-graph dwarf,8192 &
fi
perf_dwarf_pid=$!
if ! kill -0 $perf_dwarf_pid 2>/dev/null; then
    echo "Failed to start perf record in dwarf mode" >&2
    stop_profiling
    exit 1
fi

# Start profiling Java with async-profiler for each Java PID
for pid in "${java_pids[@]}"; do
    java_cmds+=("$(tr '\000' ' ' < /proc/"$pid"/cmdline)")
    async-profiler/bin/asprof start -i "$ap_interval" -F probesp+vtable "$pid"
done

# profiling has been started, set up trap to finalize on interrupt
trap finalize INT TERM EXIT

# wait
if [ "$duration" -gt 0 ]; then
    # wait for the specified duration (seconds), then wrap it up by calling finalize
    sleep "$duration"
else
    # wait indefinitely until child processes are killed or interrupted
    wait
fi

finalize
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
