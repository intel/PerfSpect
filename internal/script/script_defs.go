package script

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// script_defs.go defines the bash scripts that are used to collect information from target systems

import (
	"fmt"
	"strconv"
	"strings"
)

type ScriptDefinition struct {
	Name          string   // just a name
	Script        string   // the bash script that will be run
	Architectures []string // architectures, i.e., x86_64, arm64. If empty, it will run on all architectures.
	Families      []string // families, e.g., 6, 7. If empty, it will run on all families.
	Models        []string // models, e.g., 62, 63. If empty, it will run on all models.
	Lkms          []string // loadable kernel modules
	Depends       []string // binary dependencies that must be available for the script to run
	Superuser     bool     // requires sudo or root
	Sequential    bool     // run script sequentially (not at the same time as others)
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
	EpbScriptName                               = "energy performance bias"
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
	MpstatScriptName                            = "mpstat"
	IostatScriptName                            = "iostat"
	SarMemoryScriptName                         = "sar-memory"
	SarNetworkScriptName                        = "sar-network"
	TurbostatScriptName                         = "turbostat"
	UncoreMaxFromMSRScriptName                  = "uncore max from msr"
	UncoreMinFromMSRScriptName                  = "uncore min from msr"
	UncoreMaxFromTPMIScriptName                 = "uncore max from tpmi"
	UncoreMinFromTPMIScriptName                 = "uncore min from tpmi"
	ElcScriptName                               = "efficiency latency control"
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
)

const (
	x86_64 = "x86_64"
)

// GetScriptByName returns the script definition with the given name. It will panic if the script is not found.
func GetScriptByName(name string) ScriptDefinition {
	return GetTimedScriptByName(name, 0, 0, 0)
}

// GetTimedScriptByName returns the script definition with the given name. It will panic if the script is not found.
func GetTimedScriptByName(name string, duration int, interval int, frequency int) ScriptDefinition {
	for _, script := range getCollectionScripts(duration, interval, frequency) {
		if script.Name == name {
			return script
		}
	}
	panic(fmt.Sprintf("script not found: %s", name))
}

// getCollectionScripts returns the script definitions that are used to collect information from the target system.
func getCollectionScripts(duration, interval int, frequency int) (scripts []ScriptDefinition) {

	// script definitions
	scripts = []ScriptDefinition{
		// configuration scripts
		{
			Name:   HostnameScriptName,
			Script: "hostname",
		},
		{
			Name:   DateScriptName,
			Script: "date",
		},
		{
			Name:      DmidecodeScriptName,
			Script:    "dmidecode",
			Superuser: true,
			Depends:   []string{"dmidecode"},
		},
		{
			Name:   LscpuScriptName,
			Script: "lscpu",
		},
		{
			Name:      LspciBitsScriptName,
			Script:    "lspci -s $(lspci | grep 325b | awk 'NR==1{{print $1}}') -xxx |  awk '$1 ~ /^90/{{print $9 $8 $7 $6; exit}}'",
			Families:  []string{"6"},          // Intel
			Models:    []string{"143", "207"}, // SPR, EMR
			Superuser: true,
			Depends:   []string{"lspci"},
		},
		{
			Name:     LspciDevicesScriptName,
			Script:   "lspci -d 8086:3258 | wc -l",
			Families: []string{"6"},          // Intel
			Models:   []string{"143", "207"}, // SPR, EMR
			Depends:  []string{"lspci"},
		},
		{
			Name:    LspciVmmScriptName,
			Script:  "lspci -vmm",
			Depends: []string{"lspci"},
		},
		{
			Name:   UnameScriptName,
			Script: "uname -a",
		},
		{
			Name:   ProcCmdlineScriptName,
			Script: "cat /proc/cmdline",
		},
		{
			Name:   ProcCpuinfoScriptName,
			Script: "cat /proc/cpuinfo",
		},
		{
			Name:   EtcReleaseScriptName,
			Script: "cat /etc/*-release",
		},
		{
			Name:   GccVersionScriptName,
			Script: "gcc --version",
		},
		{
			Name:   BinutilsVersionScriptName,
			Script: "ld -v",
		},
		{
			Name:   GlibcVersionScriptName,
			Script: "ldd --version",
		},
		{
			Name:   PythonVersionScriptName,
			Script: "python --version 2>&1",
		},
		{
			Name:   Python3VersionScriptName,
			Script: "python3 --version",
		},
		{
			Name:   JavaVersionScriptName,
			Script: "java -version 2>&1",
		},
		{
			Name:   OpensslVersionScriptName,
			Script: "openssl version",
		},
		{
			Name:      CpuidScriptName,
			Script:    "cpuid -1",
			Lkms:      []string{"cpuid"},
			Depends:   []string{"cpuid"},
			Superuser: true,
		},
		{
			Name:   BaseFrequencyScriptName,
			Script: "cat /sys/devices/system/cpu/cpu0/cpufreq/base_frequency",
		},
		{
			Name:   MaximumFrequencyScriptName,
			Script: "cat /sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq",
		},
		{
			Name:   ScalingDriverScriptName,
			Script: "cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_driver",
		},
		{
			Name:   ScalingGovernorScriptName,
			Script: "cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor",
		},
		{
			Name:   MaxCStateScriptName,
			Script: "cat /sys/module/intel_idle/parameters/max_cstate",
		},
		{
			Name: CstatesScriptName,
			Script: `# Directory where C-state information is stored
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
		{
			Name:          SpecTurboCoresScriptName,
			Script:        "rdmsr 0x1ae", // MSR_TURBO_GROUP_CORE_CNT: Group Size of Active Cores for Turbo Mode Operation
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          SpecTurboFrequenciesScriptName,
			Script:        "rdmsr 0x1ad", // MSR_TURBO_RATIO_LIMIT: Maximum Ratio Limit of Turbo Mode
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          PPINName,
			Script:        "rdmsr -a 0x4f", // MSR_PPIN: Protected Processor Inventory Number
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          PrefetchControlName,
			Script:        "rdmsr -f 7:0 0x1a4", // MSR_PREFETCH_CONTROL: L2, DCU, and AMP Prefetchers enabled/disabled
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          PrefetchersName,
			Script:        "rdmsr 0x6d", // TODO: get name, used to read prefetchers
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          L3WaySizeName,
			Script:        "rdmsr 0xc90", // TODO: get name, used to read l3 size
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          PackagePowerLimitName,
			Script:        "rdmsr -f 14:0 0x610", // MSR_PKG_POWER_LIMIT: Package limit in bits 14:0
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          EpbScriptName,
			Script:        "rdmsr -a -f 3:0 0x1B0", // IA32_ENERGY_PERF_BIAS: Energy Performance Bias Hint (0 is highest perf, 15 is highest energy saving)
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          EppValidScriptName,
			Script:        "rdmsr -a -f 60:60 0x774", // IA32_HWP_REQUEST: Energy Performance Preference, bit 60 indicates if per-cpu EPP is valid
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          EppPackageControlScriptName,
			Script:        "rdmsr -a -f 42:42 0x774", // IA32_HWP_REQUEST: Energy Performance Preference, bit 42 indicates if package control is enabled
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          EppScriptName,
			Script:        "rdmsr -a -f 31:24 0x774", // IA32_HWP_REQUEST: Energy Performance Preference, bits 24-31 (0 is highest perf, 255 is highest energy saving)
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          EppPackageScriptName,
			Script:        "rdmsr -f 31:24 0x772", // IA32_HWP_REQUEST_PKG: Energy Performance Preference, bits 24-31 (0 is highest perf, 255 is highest energy saving)
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          UncoreMaxFromMSRScriptName,
			Script:        "rdmsr -f 6:0 0x620", // MSR_UNCORE_RATIO_LIMIT: MAX_RATIO in bits 6:0
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          UncoreMinFromMSRScriptName,
			Script:        "rdmsr -f 14:8 0x620", // MSR_UNCORE_RATIO_LIMIT: MAX_RATIO in bits 14:8
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:          UncoreMaxFromTPMIScriptName,
			Script:        "pcm-tpmi 2 0x18 -d -b 8:14",
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Depends:       []string{"pcm-tpmi"},
			Superuser:     true,
		},
		{
			Name:          UncoreMinFromTPMIScriptName,
			Script:        "pcm-tpmi 2 0x18 -d -b 15:21",
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Depends:       []string{"pcm-tpmi"},
			Superuser:     true,
		},
		{
			Name: ElcScriptName,
			Script: `
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
			Families:      []string{"6"}, // Intel
			Depends:       []string{"pcm-tpmi"},
			Superuser:     true,
		},
		{
			Name: ChaCountScriptName,
			Script: `rdmsr 0x396
rdmsr 0x702
rdmsr 0x2FFE`, // uncore client cha count, uncore cha count, uncore cha count spr
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
			Lkms:          []string{"msr"},
			Depends:       []string{"rdmsr"},
			Superuser:     true,
		},
		{
			Name:   IaaDevicesScriptName,
			Script: "ls -1 /dev/iax",
		},
		{
			Name:   DsaDevicesScriptName,
			Script: "ls -1 /dev/dsa",
		},
		{
			Name:      LshwScriptName,
			Script:    "timeout 30 lshw -businfo -numeric",
			Depends:   []string{"lshw"},
			Superuser: true,
		},
		{
			Name:   MeminfoScriptName,
			Script: "cat /proc/meminfo",
		},
		{
			Name:   TransparentHugePagesScriptName,
			Script: "cat /sys/kernel/mm/transparent_hugepage/enabled",
		},
		{
			Name:   NumaBalancingScriptName,
			Script: "cat /proc/sys/kernel/numa_balancing",
		},
		{
			Name: NicInfoScriptName,
			Script: `timeout 30 lshw -businfo -numeric | grep -E "^(pci|usb).*? \S+\s+network\s+\S.*?" \
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
		{
			Name: DiskInfoScriptName,
			Script: `echo "NAME|MODEL|SIZE|MOUNTPOINT|FSTYPE|RQ-SIZE|MIN-IO|FIRMWARE|ADDR|NUMA|LINKSPEED|LINKWIDTH|MAXLINKSPEED|MAXLINKWIDTH"
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
		{
			Name: HdparmScriptName,
			Script: `lsblk -d -r -o NAME -e7 -e1 -n | while read -r device ; do
	hdparm -i /dev/"$device"
done`,
			Superuser: true,
		},
		{
			Name:   DfScriptName,
			Script: `df -h`,
		},
		{
			Name:      FindMntScriptName,
			Script:    `findmnt -r`,
			Superuser: true,
		},
		{
			Name:      CveScriptName,
			Script:    "spectre-meltdown-checker.sh --batch text",
			Superuser: true,
			Lkms:      []string{"msr"},
			Depends:   []string{"spectre-meltdown-checker.sh", "rdmsr"},
		},
		{
			Name:       ProcessListScriptName,
			Script:     `ps -eo pid,ppid,%cpu,%mem,rss,command --sort=-%cpu,-pid | grep -v "]" | head -n 20`,
			Sequential: true,
		},
		{
			Name:      IpmitoolSensorsScriptName,
			Script:    "LC_ALL=C timeout 30 ipmitool sdr list full",
			Superuser: true,
			Depends:   []string{"ipmitool"},
		},
		{
			Name:      IpmitoolChassisScriptName,
			Script:    "LC_ALL=C timeout 30 ipmitool chassis status",
			Superuser: true,
			Depends:   []string{"ipmitool"},
		},
		{
			Name:      IpmitoolEventsScriptName,
			Script:    `LC_ALL=C timeout 30 ipmitool sel elist | tail -n20 | cut -d'|' -f2-`,
			Superuser: true,
			Lkms:      []string{"ipmi_devintf", "ipmi_si"},
			Depends:   []string{"ipmitool"},
		},
		{
			Name:      IpmitoolEventTimeScriptName,
			Script:    "LC_ALL=C timeout 30 ipmitool sel time get",
			Superuser: true,
			Depends:   []string{"ipmitool"},
		},
		{
			Name:      KernelLogScriptName,
			Script:    "dmesg --kernel --human --nopager | tail -n20",
			Superuser: true,
		},
		{
			Name:          PMUDriverVersionScriptName,
			Script:        `dmesg | grep -A 1 "Intel PMU driver" | tail -1 | awk '{print $NF}'`,
			Superuser:     true,
			Architectures: []string{x86_64},
			Families:      []string{"6"}, // Intel
		},
		{
			Name: PMUBusyScriptName,
			Script: `# loop through the PMU counters and check if they are active or inactive
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
		{
			Name:          GaudiInfoScriptName,
			Script:        `hl-smi -Q module_id,serial,bus_id,driver_version -f csv`,
			Architectures: []string{"x86_64"},
			Families:      []string{"6"}, // Intel
		},
		{
			Name:          GaudiFirmwareScriptName,
			Script:        `hl-smi --fw-version`,
			Architectures: []string{"x86_64"},
			Families:      []string{"6"}, // Intel
		},
		{
			Name:          GaudiNumaScriptName,
			Script:        `hl-smi topo -N`,
			Architectures: []string{"x86_64"},
			Families:      []string{"6"}, // Intel
		},
		// benchmarking scripts
		{
			Name: MemoryBandwidthAndLatencyScriptName,
			Script: `# measure memory loaded latency
#  need at least 2 GB (2,097,152 KB) of huge pages per NUMA node
min_kb=2097152
numa_nodes=$( lscpu | grep "NUMA node(s):" | awk '{print $3}' )
size_huge_pages_kb=$( cat /proc/meminfo | grep Hugepagesize | awk '{print $2}' )
orig_num_huge_pages=$( cat /proc/sys/vm/nr_hugepages )
needed_num_huge_pages=$( echo "$numa_nodes * $min_kb / $size_huge_pages_kb" | bc )
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
		{
			Name: NumaBandwidthScriptName,
			Script: `# measure memory bandwidth matrix
#  need at least 2 GB (2,097,152 KB) of huge pages per NUMA node
min_kb=2097152
numa_nodes=$( lscpu | grep "NUMA node(s):" | awk '{print $3}' )
size_huge_pages_kb=$( cat /proc/meminfo | grep Hugepagesize | awk '{print $2}' )
orig_num_huge_pages=$( cat /proc/sys/vm/nr_hugepages )
needed_num_huge_pages=$( echo "$numa_nodes * $min_kb / $size_huge_pages_kb" | bc )
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
		{
			Name: CpuSpeedScriptName,
			Script: `methods=$( stress-ng --cpu 1 --cpu-method x 2>&1 | cut -d":" -f2 | cut -c 6- )
for method in $methods; do
	printf "%s " "$method"
	stress-ng --cpu 0 -t 1 --cpu-method "$method" --metrics-brief 2>&1 | tail -1 | awk '{print $9}'
done`,
			Superuser:  false,
			Depends:    []string{"stress-ng"},
			Sequential: true,
		},
		{
			Name: TurboFrequenciesScriptName,
			Script: `# Function to expand a range of numbers, e.g. "0-24", into an array of numbers
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
		{
			Name:       TurboFrequencyPowerAndTemperatureScriptName,
			Script:     `((turbostat -i 2 2>/dev/null &) ; stress-ng --cpu 1 -t 20s 2>&1 ; stress-ng --cpu 0 -t 60s 2>&1 ; pkill -9 -f turbostat) | awk '$0~"stress" {print $0} $1=="Package" || $1=="CPU" || $1=="Core" || $1=="Node" {if(f!=1) print $0;f=1} $1=="-" {print $0}'		`,
			Superuser:  true,
			Lkms:       []string{"msr"},
			Depends:    []string{"turbostat", "stress-ng"},
			Sequential: true,
		},
		{
			Name:       IdlePowerScriptName,
			Script:     `turbostat --show PkgWatt -n 1 | sed -n 2p`,
			Superuser:  true,
			Lkms:       []string{"msr"},
			Depends:    []string{"turbostat"},
			Sequential: true,
		},
		// telemetry scripts
		{
			Name: MpstatScriptName,
			Script: func() string {
				var count string
				if duration != 0 && interval != 0 {
					countInt := duration / interval
					count = strconv.Itoa(countInt)
				}
				return fmt.Sprintf(`mpstat -u -T -I SCPU -P ALL %d %s`, interval, count)
			}(),
			Superuser: true,
			Lkms:      []string{},
			Depends:   []string{"mpstat"},
		},
		{
			Name: IostatScriptName,
			Script: func() string {
				var count string
				if duration != 0 && interval != 0 {
					countInt := duration / interval
					count = strconv.Itoa(countInt)
				}
				return fmt.Sprintf(`S_TIME_FORMAT=ISO iostat -d -t %d %s | sed '/^loop/d'`, interval, count)
			}(),
			Superuser: true,
			Lkms:      []string{},
			Depends:   []string{"iostat"},
		},
		{
			Name: SarMemoryScriptName,
			Script: func() string {
				var count string
				if duration != 0 && interval != 0 {
					countInt := duration / interval
					count = strconv.Itoa(countInt)
				}
				return fmt.Sprintf(`sar -r %d %s`, interval, count)
			}(),
			Superuser: true,
			Lkms:      []string{},
			Depends:   []string{"sar", "sadc"},
		},
		{
			Name: SarNetworkScriptName,
			Script: func() string {
				var count string
				if duration != 0 && interval != 0 {
					countInt := duration / interval
					count = strconv.Itoa(countInt)
				}
				return fmt.Sprintf(`sar -n DEV %d %s`, interval, count)
			}(),
			Superuser: true,
			Lkms:      []string{},
			Depends:   []string{"sar", "sadc"},
		},
		{
			Name: TurbostatScriptName,
			Script: func() string {
				var count string
				if duration != 0 && interval != 0 {
					countInt := duration / interval
					count = "-n " + strconv.Itoa(countInt)
				}
				return fmt.Sprintf(`turbostat -S -s PkgWatt,RAMWatt -q -i %d %s`, interval, count) + ` | awk '{ print strftime("%H:%M:%S"), $0 }'`
			}(),
			Superuser: true,
			Lkms:      []string{"msr"},
			Depends:   []string{"turbostat"},
		},

		// flamegraph scripts
		{
			Name: ProfileJavaScriptName,
			Script: func() string {
				apInterval := 0
				if frequency > 0 {
					apInterval = int(1 / float64(frequency) * 1000000000)
				}
				return fmt.Sprintf(`# JAVA app call stack collection (run in background)
ap_interval=%d
duration=%d
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
`, apInterval, duration)
			}(),
			Superuser: true,
			Depends:   []string{"async-profiler"},
		},
		{
			Name: ProfileSystemScriptName,
			Script: func() string {
				return fmt.Sprintf(`# system-wide call stack collection
# adjust perf_event_paranoid and kptr_restrict
PERF_EVENT_PARANOID=$( cat /proc/sys/kernel/perf_event_paranoid )
echo -1 >/proc/sys/kernel/perf_event_paranoid
KPTR_RESTRICT=$( cat /proc/sys/kernel/kptr_restrict )
echo 0 >/proc/sys/kernel/kptr_restrict
# system-wide call stack collection - frame pointer mode
frequency=%d
duration=%d
perf record -F $frequency -a -g -o perf_fp.data -m 129 -- sleep $duration &
PERF_FP_PID=$!
# system-wide call stack collection - dwarf mode
perf record -F $frequency -a -g -o perf_dwarf.data -m 257 --call-graph dwarf,8192 -- sleep $duration &
PERF_SYS_PID=$!
# wait for perf to finish
wait ${PERF_FP_PID}
wait ${PERF_SYS_PID}
# restore perf_event_paranoid and kptr_restrict
echo "$PERF_EVENT_PARANOID" > /proc/sys/kernel/perf_event_paranoid
echo "$KPTR_RESTRICT" > /proc/sys/kernel/kptr_restrict
# collapse perf data
perf script -i perf_dwarf.data | stackcollapse-perf.pl > perf_dwarf.folded
perf script -i perf_fp.data | stackcollapse-perf.pl > perf_fp.folded
if [ -f "perf_dwarf.folded" ]; then
    echo "########## perf_dwarf ##########"
    cat perf_dwarf.folded
fi
if [ -f "perf_fp.folded" ]; then
    echo "########## perf_fp ##########"
    cat perf_fp.folded
fi
`, frequency, duration)
			}(),
			Superuser: true,
			Depends:   []string{"perf", "stackcollapse-perf.pl"},
		},
		{
			Name: ProfileKernelLockScriptName,
			Script: func() string {
				return fmt.Sprintf(`# system-wide lock profile collection
# adjust perf_event_paranoid and kptr_restrict
PERF_EVENT_PARANOID=$( cat /proc/sys/kernel/perf_event_paranoid )
echo -1 >/proc/sys/kernel/perf_event_paranoid
KPTR_RESTRICT=$( cat /proc/sys/kernel/kptr_restrict )
echo 0 >/proc/sys/kernel/kptr_restrict

frequency=%d
duration=%d

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
`, frequency, duration)
			}(),
			Superuser: true,
			Depends:   []string{"perf"},
		},
	}

	// validate script definitions
	var scriptNames = make(map[string]bool)
	for i, s := range scripts {
		if _, ok := scriptNames[s.Name]; ok {
			panic(fmt.Sprintf("script %d, duplicate script name: %s", i, s.Name))
		}
		if s.Name == "" {
			panic(fmt.Sprintf("script %d, name cannot be empty", i))
		}
		if s.Script == "" {
			panic(fmt.Sprintf("script %d, script cannot be empty: %s", i, s.Name))
		}
		if strings.ContainsAny(s.Name, "/") {
			panic(fmt.Sprintf("script %d, name cannot contain /: %s", i, s.Name))
		}
	}
	return
}
