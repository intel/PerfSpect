package config

import (
	"fmt"
	"log/slog"
	"math"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/target"
	"perfspect/internal/util"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

func setCoreCount(cores int, myTarget target.Target, localTempDir string, completeChannel chan setOutput, goRoutineId int) {
	setScript := script.ScriptDefinition{
		Name: "set core count",
		ScriptTemplate: fmt.Sprintf(`
desired_core_count_per_socket=%d
num_cpus=$(ls /sys/devices/system/cpu/ | grep -E "^cpu[0-9]+$" | wc -l)
num_threads=$(lscpu | grep 'Thread(s) per core' | awk '{print $NF}')
num_sockets=$(lscpu | grep 'Socket(s)' | awk '{print $NF}')
num_cores_per_socket=$((num_cpus / num_sockets / num_threads))

# if desired core count is greater than current core count, exit
if [[ $desired_core_count_per_socket -gt $num_cores_per_socket ]]; then
	echo "requested core count ($desired_core_count_per_socket) is greater than physical cores ($num_cores_per_socket)"
	exit 1
fi

# enable all logical CPUs
echo 1 | tee /sys/devices/system/cpu/cpu*/online > /dev/null

# if no cores to disable, exit
num_cores_to_disable_per_socket=$((num_cores_per_socket - desired_core_count_per_socket))
if [[ $num_cores_to_disable_per_socket -eq 0 ]]; then
    echo "no cpus to off-line"
    exit 0
fi

# get lines from cpuinfo that match the fields we need
proc_cpuinfo_filtered=$(grep -E '(processor|core id|physical id)' /proc/cpuinfo)

# loop through each line of text in proc_cpuinfo_filtered, creating a new record for each logical CPU
while IFS= read -r line; do
    # if line contains 'processor', start a new record
    if [[ $line =~ "processor" ]]; then
        # if record isn't empty (is empty first time through loop), put the record in the list of cpuinfo records
        if [[ -n "$record" ]]; then
            cpuinfo+=("$record")
        fi
        record="$line"$'\n'
    else
        record+="$line"$'\n'
    fi
done <<< "$proc_cpuinfo_filtered"
# add the last record
if [[ -n "$record" ]]; then
    cpuinfo+=("$record")
fi

# build a unique list of core ids from the records
core_ids=()
for record in "${cpuinfo[@]}"; do
    core_id=$(echo "$record" | grep 'core id' | awk '{print $NF}')
    found=0
    for id in "${core_ids[@]}"; do
        if [[ "$id" == "$core_id" ]]; then
            found=1
            break
        fi
    done
    if [[ $found -eq 0 ]]; then
        core_ids+=("$core_id")
    fi
done

# disable logical CPUs to reach the desired core count per socket
for ((socket=0; socket<num_sockets; socket++)); do
    offlined_cores=0
    # loop through core_ids in reverse order to off-line the highest numbered cores first
    for ((i=${#core_ids[@]}-1; i>=0; i--)); do
        core=${core_ids[i]}
        if [[ $offlined_cores -eq $num_cores_to_disable_per_socket ]]; then
            break
        fi
        offlined_cores=$((offlined_cores+1))
        # find record that matches socket and core and off-line the logical CPU
        for record in "${cpuinfo[@]}"; do
            processor=$(echo "$record" | grep 'processor' | awk '{print $NF}')
            core_id=$(echo "$record" | grep 'core id' | awk '{print $NF}')
            physical_id=$(echo "$record" | grep 'physical id' | awk '{print $NF}')
            if [[ $physical_id -eq $socket && $core_id -eq $core ]]; then
                echo "Off-lining processor $processor (socket $physical_id, core $core_id)"
                echo 0 | tee /sys/devices/system/cpu/cpu"$processor"/online > /dev/null
                num_disabled_cores=$((num_disabled_cores+1))
            fi
        done
    done
done
`, cores),
		Superuser: true,
	}
	_, err := runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set core count: %w", err)
	}
	completeChannel <- setOutput{goRoutineID: goRoutineId, err: err}
}

func setLlcSize(desiredLlcSize float64, myTarget target.Target, localTempDir string, completeChannel chan setOutput, goRoutineId int) {
	// get the data we need to set the LLC size
	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.GetScriptByName(script.LscpuScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciBitsScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciDevicesScriptName))
	scripts = append(scripts, script.GetScriptByName(script.L3CacheWayEnabledName))
	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to run scripts on target: %w", err)}
		return
	}

	uarch := report.UarchFromOutput(outputs)
	cpu, err := report.GetCPUByMicroArchitecture(uarch)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get CPU by microarchitecture: %w", err)}
		return
	}
	if cpu.CacheWayCount == 0 {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("cache way count is zero")}
		return
	}
	maximumLlcSize, err := report.GetL3LscpuMB(outputs)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get maximum LLC size: %w", err)}
		return
	}
	currentLlcSize, err := report.GetL3MSRMB(outputs)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get current LLC size: %w", err)}
		return
	}
	if currentLlcSize == desiredLlcSize {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("LLC size is already set to %.2f MB", desiredLlcSize)}
		return
	}
	if desiredLlcSize > maximumLlcSize {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("LLC size is too large, maximum is %.2f MB", maximumLlcSize)}
		return
	}
	// calculate the number of ways to set
	cachePerWay := maximumLlcSize / float64(cpu.CacheWayCount)
	waysToSet := int(math.Ceil(desiredLlcSize / cachePerWay))
	if waysToSet > cpu.CacheWayCount {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("LLC size is too large, maximum is %.2f MB", maximumLlcSize)}
		return
	}
	// set the LLC size
	msrVal, err := util.Uint64FromNumLowerBits(waysToSet)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to convert waysToSet to uint64: %w", err)}
		return
	}
	setScript := script.ScriptDefinition{
		Name:           "set LLC size",
		ScriptTemplate: fmt.Sprintf("wrmsr -a 0xC90 %d", msrVal),
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set LLC size: %w", err)
	}
	completeChannel <- setOutput{goRoutineID: goRoutineId, err: err}
}

func setCoreFrequency(coreFrequency float64, myTarget target.Target, localTempDir string, completeChannel chan setOutput, goRoutineId int) {
	targetFamily, err := myTarget.GetFamily()
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get target family: %w", err)}
		return
	}
	targetModel, err := myTarget.GetModel()
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get target model: %w", err)}
		return
	}
	targetVendor, err := myTarget.GetVendor()
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get target vendor: %w", err)}
		return
	}
	if targetVendor != "GenuineIntel" {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("core frequency setting not supported on %s due to vendor mismatch", myTarget.GetName())}
		return
	}
	var setScript script.ScriptDefinition
	freqInt := uint64(coreFrequency * 10)
	if targetFamily == "6" && (targetModel == "175" || targetModel == "221") { // SRF, CWF
		// get the pstate driver
		getScript := script.ScriptDefinition{
			Name:           "get pstate driver",
			ScriptTemplate: "cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_driver",
			Vendors:        []string{"GenuineIntel"},
		}
		output, err := runScript(myTarget, getScript, localTempDir)
		if err != nil {
			completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get pstate driver: %w", err)}
			return
		}
		if strings.Contains(output, "intel_pstate") {
			var value uint64
			var i uint
			for i = range 2 {
				value = value | freqInt<<i*8
			}
			setScript = script.ScriptDefinition{
				Name:           "set frequency bins",
				ScriptTemplate: fmt.Sprintf("wrmsr 0x774 %d", value),
				Superuser:      true,
				Vendors:        []string{"GenuineIntel"},
				// Depends:        []string{"wrmsr"},
				// Lkms:           []string{"msr"},
			}
		} else {
			value := freqInt << uint(2*8)
			setScript = script.ScriptDefinition{
				Name:           "set frequency bins",
				ScriptTemplate: fmt.Sprintf("wrmsr 0x199 %d", value),
				Superuser:      true,
				Vendors:        []string{"GenuineIntel"},
				// Depends:        []string{"wrmsr"},
				// Lkms:           []string{"msr"},
			}
		}
	} else {
		var value uint64
		var i uint
		for i = range 8 {
			value = value | freqInt<<i*8
		}
		setScript = script.ScriptDefinition{
			Name:           "set frequency bins",
			ScriptTemplate: fmt.Sprintf("wrmsr -a 0x1AD %d", value),
			Superuser:      true,
			Vendors:        []string{"GenuineIntel"},
			// Depends:        []string{"wrmsr"},
			// Lkms:           []string{"msr"},
		}
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set core frequency: %w", err)
	}
	completeChannel <- setOutput{goRoutineID: goRoutineId, err: err}
}

func setUncoreDieFrequency(maxFreq bool, computeDie bool, uncoreFrequency float64, myTarget target.Target, localTempDir string, completeChannel chan setOutput, goRoutineId int) {
	targetFamily, err := myTarget.GetFamily()
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get target family: %w", err)}
		return
	}
	targetModel, err := myTarget.GetModel()
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get target model: %w", err)}
		return
	}
	if targetFamily != "6" || (targetFamily == "6" && targetModel != "173" && targetModel != "174" && targetModel != "175" && targetModel != "221") { // not Intel || not GNR, GNR-D, SRF, CWF
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("uncore frequency setting not supported on %s due to family/model mismatch", myTarget.GetName())}
		return
	}
	type dieId struct {
		instance string
		entry    string
	}
	var dies []dieId
	// build list of compute or IO dies
	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.GetScriptByName(script.UncoreDieTypesFromTPMIScriptName))
	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to run scripts on target: %w", err)}
		return
	}
	re := regexp.MustCompile(`Read bits \d+:\d+ value (\d+) from TPMI ID .* for entry (\d+) in instance (\d+)`)
	for line := range strings.SplitSeq(outputs[script.UncoreDieTypesFromTPMIScriptName].Stdout, "\n") {
		match := re.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if computeDie && match[1] == "0" {
			dies = append(dies, dieId{instance: match[3], entry: match[2]})
		}
		if !computeDie && match[1] == "1" {
			dies = append(dies, dieId{instance: match[3], entry: match[2]})
		}
	}

	value := uint64(uncoreFrequency * 10)
	var bits string
	if maxFreq {
		bits = "8:14" // bits 8:14 are the max frequency
	} else {
		bits = "15:21" // bits 15:21 are the min frequency
	}
	// run script for each die of specified type
	for _, die := range dies {
		setScript := script.ScriptDefinition{
			Name:           "write max and min uncore frequency TPMI",
			ScriptTemplate: fmt.Sprintf("pcm-tpmi 2 0x18 -d -b %s -w %d -i %s -e %s", bits, value, die.instance, die.entry),
			Vendors:        []string{"GenuineIntel"},
			Depends:        []string{"pcm-tpmi"},
			Superuser:      true,
		}
		_, err = runScript(myTarget, setScript, localTempDir)
		if err != nil {
			err = fmt.Errorf("failed to set uncore die frequency: %w", err)
			break
		}
	}
	completeChannel <- setOutput{goRoutineID: goRoutineId, err: err}
}

func setUncoreFrequency(maxFreq bool, uncoreFrequency float64, myTarget target.Target, localTempDir string, completeChannel chan setOutput, goRoutineId int) {
	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.ScriptDefinition{
		Name:           "get uncore frequency MSR",
		ScriptTemplate: "rdmsr 0x620",
		Vendors:        []string{"GenuineIntel"},
		Superuser:      true,
		// Depends:        []string{"rdmsr"},
		// Lkms:           []string{"msr"},
	})
	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to run scripts on target: %w", err)}
		return
	}
	targetFamily, err := myTarget.GetFamily()
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get target family: %w", err)}
		return
	}
	targetModel, err := myTarget.GetModel()
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get target model: %w", err)}
		return
	}
	if targetFamily != "6" || (targetFamily == "6" && (targetModel == "173" || targetModel == "174" || targetModel == "175" || targetModel == "221")) { // not Intel || not GNR, GNR-D, SRF, CWF
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("uncore frequency setting not supported on %s due to family/model mismatch", myTarget.GetName())}
		return
	}
	msrUint, err := strconv.ParseUint(strings.TrimSpace(outputs["get uncore frequency MSR"].Stdout), 16, 0)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to parse uncore frequency MSR: %w", err)}
		return
	}
	newFreq := uint64((uncoreFrequency * 1000) / 100)
	var newVal uint64
	if maxFreq {
		// mask out lower 6 bits to write the max frequency
		newVal = msrUint & 0xFFFFFFFFFFFFFFC0
		// add in the new frequency value
		newVal = newVal | newFreq
	} else {
		// mask bits 8:14 to write the min frequency
		newVal = msrUint & 0xFFFFFFFFFFFF80FF
		// add in the new frequency value
		newVal = newVal | newFreq<<8
	}
	setScript := script.ScriptDefinition{
		Name:           "set uncore frequency MSR",
		ScriptTemplate: fmt.Sprintf("wrmsr -a 0x620 %d", newVal),
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set uncore frequency: %w", err)
	}
	completeChannel <- setOutput{goRoutineID: goRoutineId, err: err}
}

func setTDP(power int, myTarget target.Target, localTempDir string, completeChannel chan setOutput, goRoutineId int) {
	readScript := script.ScriptDefinition{
		Name:           "get power MSR",
		ScriptTemplate: "rdmsr 0x610",
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		// Lkms:           []string{"msr"},
		// Depends:        []string{"rdmsr"},
	}
	readOutput, err := script.RunScript(myTarget, readScript, localTempDir)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to read power MSR: %w", err)}
		return
	} else {
		msrHex := strings.TrimSpace(readOutput.Stdout)
		msrUint, err := strconv.ParseUint(msrHex, 16, 0)
		if err != nil {
			completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to parse power MSR: %w", err)}
			return
		} else {
			// mask out lower 14 bits
			newVal := uint64(msrUint) & 0xFFFFFFFFFFFFC000
			// add in the new power value
			newVal = newVal | uint64(power*8) // #nosec G115
			setScript := script.ScriptDefinition{
				Name:           "set tdp",
				ScriptTemplate: fmt.Sprintf("wrmsr -a 0x610 %d", newVal),
				Superuser:      true,
				Vendors:        []string{"GenuineIntel"},
				// Depends:        []string{"wrmsr"},
				// Lkms:           []string{"msr"},
			}
			_, err := runScript(myTarget, setScript, localTempDir)
			if err != nil {
				completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to set power: %w", err)}
				return
			}
		}
	}
	completeChannel <- setOutput{goRoutineID: goRoutineId, err: nil}
}

func setEPB(epb int, myTarget target.Target, localTempDir string, completeChannel chan setOutput, goRoutineId int) {
	epbSourceScript := script.GetScriptByName(script.EpbSourceScriptName)
	epbSourceOutput, err := runScript(myTarget, epbSourceScript, localTempDir)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get EPB source: %w", err)}
		return
	}
	epbSource := strings.TrimSpace(epbSourceOutput)
	source, err := strconv.ParseInt(epbSource, 16, 0)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to parse EPB source: %w", err)}
		return
	}
	var msr string
	var bitOffset uint
	if source == 0 { // 0 means the EPB is controlled by the OS
		msr = "0x1B0"
		bitOffset = 0
	} else { // 1 means the EPB is controlled by the BIOS
		msr = "0xA01"
		bitOffset = 3
	}
	readScript := script.ScriptDefinition{
		Name:           "read " + msr,
		ScriptTemplate: "rdmsr " + msr,
		Vendors:        []string{"GenuineIntel"},
		Superuser:      true,
		// Lkms:           []string{"msr"},
		// Depends:        []string{"rdmsr"},
	}
	readOutput, err := runScript(myTarget, readScript, localTempDir)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to read EPB MSR %s: %w", msr, err)}
		return
	}
	msrValue, err := strconv.ParseUint(strings.TrimSpace(readOutput), 16, 64)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to parse EPB MSR %s: %w", msr, err)}
		return
	}
	// mask out 4 bits starting at bitOffset
	maskedValue := msrValue &^ (0xF << bitOffset)
	// put the EPB value in the masked bits
	msrValue = maskedValue | uint64(epb)<<bitOffset // #nosec G115
	// write the new value to the MSR
	setScript := script.ScriptDefinition{
		Name:           "set epb",
		ScriptTemplate: fmt.Sprintf("wrmsr -a %s %d", msr, msrValue),
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set EPB: %w", err)
	}
	completeChannel <- setOutput{goRoutineID: goRoutineId, err: err}
}

func setEPP(epp int, myTarget target.Target, localTempDir string, completeChannel chan setOutput, goRoutineId int) {
	// Set both the per-core EPP value and the package EPP value
	// Reference: 15.4.4 Managing HWP in the Intel SDM

	// get the current value of the IAEW_HWP_REQUEST MSR that includes the current EPP valid value in bit 60
	getScript := script.ScriptDefinition{
		Name:           "get epp msr",
		ScriptTemplate: "rdmsr 0x774", // IA32_HWP_REQUEST
		Vendors:        []string{"GenuineIntel"},
		Superuser:      true,
		// Lkms:           []string{"msr"},
		// Depends:        []string{"rdmsr"},
	}
	stdout, err := runScript(myTarget, getScript, localTempDir)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to read EPP MSR %s: %w", "0x774", err)}
		return
	}
	msrValue, err := strconv.ParseUint(strings.TrimSpace(stdout), 16, 64)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to parse EPP MSR %s: %w", "0x774", err)}
		return
	}
	// mask out bits 24-31 IA32_HWP_REQUEST MSR value
	maskedValue := msrValue & 0xFFFFFFFF00FFFFFF
	// put the EPP value in bits 24-31
	eppValue := maskedValue | uint64(epp)<<24 // #nosec G115
	// write it back to the MSR
	setScript := script.ScriptDefinition{
		Name:           "set epp",
		ScriptTemplate: fmt.Sprintf("wrmsr -a 0x774 %d", eppValue),
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to set EPP: %w", err)}
		return
	}
	// get the current value of the IA32_HWP_REQUEST_PKG MSR that includes the current package EPP value
	getScript = script.ScriptDefinition{
		Name:           "get epp pkg msr",
		ScriptTemplate: "rdmsr 0x772", // IA32_HWP_REQUEST_PKG
		Vendors:        []string{"GenuineIntel"},
		Superuser:      true,
		// Lkms:           []string{"msr"},
		// Depends:        []string{"rdmsr"},
	}
	stdout, err = runScript(myTarget, getScript, localTempDir)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to read EPP pkg MSR %s: %w", "0x772", err)}
		return
	}
	msrValue, err = strconv.ParseUint(strings.TrimSpace(stdout), 16, 64)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to parse EPP pkg MSR %s: %w", "0x772", err)}
		return
	}
	// mask out bits 24-31 IA32_HWP_REQUEST_PKG MSR value
	maskedValue = msrValue & 0xFFFFFFFF00FFFFFF
	// put the EPP value in bits 24-31
	eppValue = maskedValue | uint64(epp)<<24 // #nosec G115
	// write it back to the MSR
	setScript = script.ScriptDefinition{
		Name:           "set epp",
		ScriptTemplate: fmt.Sprintf("wrmsr -a 0x772 %d", eppValue),
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set EPP pkg: %w", err)
	}
	completeChannel <- setOutput{goRoutineID: goRoutineId, err: err}
}

func setGovernor(governor string, myTarget target.Target, localTempDir string, completeChannel chan setOutput, goRoutineId int) {
	setScript := script.ScriptDefinition{
		Name:           "set governor",
		ScriptTemplate: fmt.Sprintf("echo %s | tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor", governor),
		Superuser:      true,
	}
	_, err := runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set governor: %w", err)
	}
	completeChannel <- setOutput{goRoutineID: goRoutineId, err: err}
}

func setELC(elc string, myTarget target.Target, localTempDir string, completeChannel chan setOutput, goRoutineId int) {
	var mode string
	switch elc {
	case elcOptions[0]:
		mode = "latency-optimized-mode"
	case elcOptions[1]:
		mode = "default"
	default:
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("invalid ELC mode: %s", elc)}
		return
	}
	setScript := script.ScriptDefinition{
		Name:           "set elc",
		ScriptTemplate: fmt.Sprintf("bhs-power-mode.sh --%s", mode),
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		Models:         []string{"173", "174", "175", "221"}, // GNR, GNR-D, SRF, CWF
		Depends:        []string{"bhs-power-mode.sh", "pcm-tpmi"},
	}
	_, err := runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set ELC mode: %w", err)
	}
	completeChannel <- setOutput{goRoutineID: goRoutineId, err: err}
}

func setPrefetcher(enableDisable string, myTarget target.Target, localTempDir string, prefetcherType string, completeChannel chan setOutput, goRoutineId int) {
	pf, err := report.GetPrefetcherDefByName(prefetcherType)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get prefetcher definition: %w", err)}
		return
	}
	// check if the prefetcher is supported on this target's architecture
	// get the uarch
	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.GetScriptByName(script.LscpuScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciBitsScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciDevicesScriptName))
	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to run scripts on target: %w", err)}
		return
	}
	uarch := report.UarchFromOutput(outputs)
	if uarch == "" {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get microarchitecture")}
		return
	}
	// is the prefetcher supported on this uarch?
	if !slices.Contains(pf.Uarchs, "all") && !slices.Contains(pf.Uarchs, uarch[:3]) {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("prefetcher %s is not supported on %s", prefetcherType, uarch)}
		return
	}
	// get the current value of the prefetcher MSR
	getScript := script.ScriptDefinition{
		Name:           "get prefetcher msr",
		ScriptTemplate: fmt.Sprintf("rdmsr %d", pf.Msr),
		Vendors:        []string{"GenuineIntel"},
		Superuser:      true,
		// Lkms:           []string{"msr"},
		// Depends:        []string{"rdmsr"},
	}
	stdout, err := runScript(myTarget, getScript, localTempDir)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to read prefetcher MSR: %w", err)}
		return
	}
	msrValue, err := strconv.ParseUint(strings.TrimSpace(stdout), 16, 64)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to parse prefetcher MSR: %w", err)}
		return
	}
	// set the prefetcher bit to bitValue determined by the onOff value, note: 0 is enable, 1 is disable
	var bitVal uint64
	switch enableDisable {
	case prefetcherOptions[0]:
		bitVal = 0
	case prefetcherOptions[1]:
		bitVal = 1
	default:
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("invalid prefetcher setting: %s", enableDisable)}
		return
	}
	// mask out the prefetcher bit
	maskedValue := msrValue &^ (1 << pf.Bit)
	// set the prefetcher bit
	newVal := maskedValue | uint64(bitVal<<pf.Bit)
	// write the new value to the MSR
	setScript := script.ScriptDefinition{
		Name:           "set prefetcher" + prefetcherType,
		ScriptTemplate: fmt.Sprintf("wrmsr -a %d %d", pf.Msr, newVal),
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set %s prefetcher: %w", prefetcherType, err)
	}
	completeChannel <- setOutput{goRoutineID: goRoutineId, err: err}
}

// setC6 enables or disables C6 C-States
func setC6(enableDisable string, myTarget target.Target, localTempDir string, completeChannel chan setOutput, goRoutineId int) {
	getScript := script.ScriptDefinition{
		Name: "get C6 state folder names",
		ScriptTemplate: `# This script finds the states of the CPU that include "C6" in their name
cstate_dir="/sys/devices/system/cpu/cpu0/cpuidle"
if [ -d "$cstate_dir" ]; then
	# if the state name includes "C6", print it
	for state in "$cstate_dir"/state*; do
		name=$(cat "$state/name")
		if [[ $name == *"C6"* ]]; then
			basename "$state"
		fi	
	done
fi
`,
		Superuser: true,
	}
	stdout, err := runScript(myTarget, getScript, localTempDir)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get C6 state folders: %w", err)}
		return
	}
	c6StateFolders := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(c6StateFolders) == 0 {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("no C6 state folders found")}
		return
	}
	var enableDisableValue int
	switch enableDisable {
	case c6Options[0]: // enable
		enableDisableValue = 0
	case c6Options[1]: // disable
		enableDisableValue = 1
	default:
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("invalid C6 setting: %s", enableDisable)}
		return
	}
	bash := "for cpu in /sys/devices/system/cpu/cpu[0-9]*; do\n"
	for _, folder := range c6StateFolders {
		bash += fmt.Sprintf("  echo %d > $cpu/cpuidle/%s/disable\n", enableDisableValue, folder)
	}
	bash += "done\n"
	setScript := script.ScriptDefinition{
		Name:           "configure c6",
		ScriptTemplate: bash,
		Superuser:      true,
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set C6: %w", err)
	}
	completeChannel <- setOutput{goRoutineID: goRoutineId, err: err}
}

func setC1Demotion(enableDisable string, myTarget target.Target, localTempDir string, completeChannel chan setOutput, goRoutineId int) {
	getScript := script.ScriptDefinition{
		Name:           "get C1 demotion",
		ScriptTemplate: "rdmsr 0xe2",
		Vendors:        []string{"GenuineIntel"},
		Superuser:      true,
		// Lkms:           []string{"msr"},
		// Depends:        []string{"rdmsr"},
	}
	stdout, err := runScript(myTarget, getScript, localTempDir)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to get C1 demotion: %w", err)}
		return
	}
	msrValue, err := strconv.ParseUint(strings.TrimSpace(stdout), 16, 64)
	if err != nil {
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("failed to parse C1 demotion MSR: %w", err)}
		return
	}
	// set the c1 demotion bits to bitValue, note: 1 is enable, 0 is disable
	var bitVal uint64
	switch enableDisable {
	case c1DemotionOptions[0]: // enable
		bitVal = 1
	case c1DemotionOptions[1]: // disable
		bitVal = 0
	default:
		completeChannel <- setOutput{goRoutineID: goRoutineId, err: fmt.Errorf("invalid C1 demotion setting: %s", enableDisable)}
		return
	}
	// mask out the C1 demotion bits (26 and 28)
	maskedValue := msrValue &^ (1 << 26)
	maskedValue = maskedValue &^ (1 << 28)
	// set the C1 demotion bits
	newVal := maskedValue | uint64(bitVal<<26) | uint64(bitVal<<28)
	// write the new value to the MSR
	setScript := script.ScriptDefinition{
		Name:           "set C1 demotion",
		ScriptTemplate: fmt.Sprintf("wrmsr -a %d %d", 0xe2, newVal),
		Vendors:        []string{"GenuineIntel"},
		Superuser:      true,
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set C1 demotion: %w", err)
	}
	completeChannel <- setOutput{goRoutineID: goRoutineId, err: err}
}

// runScript runs a script on the target and returns the output
func runScript(myTarget target.Target, myScript script.ScriptDefinition, localTempDir string) (string, error) {
	output, err := script.RunScript(myTarget, myScript, localTempDir) // nosemgrep
	if err != nil {
		slog.Error("failed to run script on target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()), slog.String("stdout", output.Stdout), slog.String("stderr", output.Stderr))
	} else {
		slog.Debug("ran script on target", slog.String("target", myTarget.GetName()), slog.String("script", myScript.Name), slog.String("stdout", output.Stdout), slog.String("stderr", output.Stderr))
	}
	return output.Stdout, err
}
