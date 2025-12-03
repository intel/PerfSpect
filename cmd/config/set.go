package config

import (
	"fmt"
	"log/slog"
	"math"
	"perfspect/internal/cpus"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/target"
	"perfspect/internal/util"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

var uncoreDieFrequencyMutex sync.Mutex
var uncoreFrequencyMutex sync.Mutex

func setCoreCount(cores int, myTarget target.Target, localTempDir string) error {
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
	return err
}

func setLlcSize(desiredLlcSize float64, myTarget target.Target, localTempDir string) error {
	// get the data we need to set the LLC size
	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.GetScriptByName(script.LscpuScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LscpuCacheScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciBitsScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciDevicesScriptName))
	scripts = append(scripts, script.GetScriptByName(script.L3CacheWayEnabledName))
	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir, nil, "")
	if err != nil {
		return fmt.Errorf("failed to run scripts on target: %w", err)
	}

	uarch := report.UarchFromOutput(outputs)
	cpu, err := cpus.GetCPUByMicroArchitecture(uarch)
	if err != nil {
		return fmt.Errorf("failed to get CPU by microarchitecture: %w", err)
	}
	if cpu.CacheWayCount == 0 {
		return fmt.Errorf("cache way count is zero")
	}
	maximumLlcSize, _, err := report.GetL3LscpuMB(outputs)
	if err != nil {
		return fmt.Errorf("failed to get maximum LLC size: %w", err)
	}
	currentLlcSize, _, err := report.GetL3MSRMB(outputs)
	if err != nil {
		return fmt.Errorf("failed to get current LLC size: %w", err)
	}
	if currentLlcSize == desiredLlcSize {
		// return success
		return nil
	}
	if desiredLlcSize > maximumLlcSize {
		return fmt.Errorf("LLC size is too large, maximum is %.2f MB", maximumLlcSize)
	}
	// calculate the number of ways to set
	cachePerWay := maximumLlcSize / float64(cpu.CacheWayCount)
	waysToSet := int(math.Ceil(desiredLlcSize / cachePerWay))
	if waysToSet > cpu.CacheWayCount {
		return fmt.Errorf("LLC size is too large, maximum is %.2f MB", maximumLlcSize)
	}
	// set the LLC size
	msrVal, err := util.Uint64FromNumLowerBits(waysToSet)
	if err != nil {
		return fmt.Errorf("failed to convert waysToSet to uint64: %w", err)
	}
	setScript := script.ScriptDefinition{
		Name:           "set LLC size",
		ScriptTemplate: fmt.Sprintf("wrmsr -a 0xC90 %d", msrVal),
		Superuser:      true,
		Vendors:        []string{cpus.IntelVendor},
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set LLC size: %w", err)
	}
	return err
}

func setSSEFrequency(sseFrequency float64, myTarget target.Target, localTempDir string) error {
	targetFamily, err := myTarget.GetFamily()
	if err != nil {
		return fmt.Errorf("failed to get target family: %w", err)
	}
	targetModel, err := myTarget.GetModel()
	if err != nil {
		return fmt.Errorf("failed to get target model: %w", err)
	}
	targetVendor, err := myTarget.GetVendor()
	if err != nil {
		return fmt.Errorf("failed to get target vendor: %w", err)
	}
	if targetVendor != cpus.IntelVendor {
		return fmt.Errorf("core frequency setting not supported on %s due to vendor mismatch", myTarget.GetName())
	}
	var setScript script.ScriptDefinition
	freqInt := uint64(sseFrequency * 10)
	if targetFamily == "6" && (targetModel == "175" || targetModel == "221") { // SRF, CWF
		// get the pstate driver
		getScript := script.ScriptDefinition{
			Name:           "get pstate driver",
			ScriptTemplate: "cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_driver",
			Vendors:        []string{cpus.IntelVendor},
		}
		output, err := runScript(myTarget, getScript, localTempDir)
		if err != nil {
			return fmt.Errorf("failed to get pstate driver: %w", err)
		}
		if strings.Contains(output, "intel_pstate") {
			var value uint64
			var i uint
			for i = range 2 {
				value = value | freqInt<<(i*8)
			}
			setScript = script.ScriptDefinition{
				Name:           "set frequency bins",
				ScriptTemplate: fmt.Sprintf("wrmsr 0x774 %d", value),
				Superuser:      true,
				Vendors:        []string{cpus.IntelVendor},
				// Depends:        []string{"wrmsr"},
				// Lkms:           []string{"msr"},
			}
		} else {
			value := freqInt << uint(2*8)
			setScript = script.ScriptDefinition{
				Name:           "set frequency bins",
				ScriptTemplate: fmt.Sprintf("wrmsr 0x199 %d", value),
				Superuser:      true,
				Vendors:        []string{cpus.IntelVendor},
				// Depends:        []string{"wrmsr"},
				// Lkms:           []string{"msr"},
			}
		}
	} else {
		var value uint64
		var i uint
		for i = range 8 {
			value = value | freqInt<<(i*8)
		}
		setScript = script.ScriptDefinition{
			Name:           "set frequency bins",
			ScriptTemplate: fmt.Sprintf("wrmsr -a 0x1AD %d", value),
			Superuser:      true,
			Vendors:        []string{cpus.IntelVendor},
			// Depends:        []string{"wrmsr"},
			// Lkms:           []string{"msr"},
		}
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set core frequency: %w", err)
	}
	return err
}

// expandConsolidatedFrequencies takes a consolidated frequency string and bucket sizes,
// and returns the 8 individual bucket frequencies.
// Input format: "1-40/3.5, 41-60/3.4, 61-86/3.2"
// bucketSizes: slice of 8 integers representing the end core number of each bucket (e.g., [20, 40, 60, 80, 86, 86, 86, 86]).
// This example corresponds to the following buckets: 0-19, 20-39, 40-59, 60-79, 80-85, 80-85, 80-85, 80-85
// Returns: slice of 8 float64 values, one frequency per bucket
func expandConsolidatedFrequencies(consolidatedStr string, bucketSizes []int) ([]float64, error) {
	if len(bucketSizes) != 8 {
		return nil, fmt.Errorf("expected 8 bucket sizes, got %d", len(bucketSizes))
	}

	bucketFrequencies := make([]float64, 8)
	entries := strings.Split(consolidatedStr, ", ")

	// Parse all consolidated entries
	type consolidatedRange struct {
		startCore int
		endCore   int
		freq      float64
	}
	var ranges []consolidatedRange

	for _, entry := range entries {
		// Parse each entry in format "start-end/freq"
		parts := strings.Split(entry, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format for entry: %s", entry)
		}

		// Parse the frequency
		freq, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid frequency in entry %s: %w", entry, err)
		}

		// Parse the range
		rangeParts := strings.Split(parts[0], "-")
		if len(rangeParts) != 2 {
			return nil, fmt.Errorf("invalid range format in entry: %s", entry)
		}

		startCore, err := strconv.Atoi(rangeParts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid start core in entry %s: %w", entry, err)
		}

		endCore, err := strconv.Atoi(rangeParts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid end core in entry %s: %w", entry, err)
		}

		ranges = append(ranges, consolidatedRange{startCore, endCore, freq})
	}

	// Map each original bucket to its frequency
	for i, bucketSize := range bucketSizes {
		// Calculate the start and end of this original bucket
		var bucketStart, bucketEnd int
		if i == 0 {
			bucketStart = 1
		} else {
			bucketStart = bucketSizes[i-1] + 1
		}
		bucketEnd = bucketSize

		// Find which consolidated range contains the midpoint of this bucket
		bucketMidpoint := (bucketStart + bucketEnd) / 2
		for _, r := range ranges {
			if bucketMidpoint >= r.startCore && bucketMidpoint <= r.endCore {
				bucketFrequencies[i] = r.freq
				break
			}
		}
	}

	return bucketFrequencies, nil
}

// setSSEFrequencies sets the SSE frequencies for all core buckets
// The input string should be in the format "start-end/freq", comma-separated
// e.g., "1-40/3.5, 41-60/3.4, 61-86/3.2"
// Note that the buckets have been consolidated where frequencies are the same, so they
// will need to be expanded back out to individual buckets for setting.
func setSSEFrequencies(sseFrequencies string, myTarget target.Target, localTempDir string) error {
	targetFamily, err := myTarget.GetFamily()
	if err != nil {
		return fmt.Errorf("failed to get target family: %w", err)
	}
	targetModel, err := myTarget.GetModel()
	if err != nil {
		return fmt.Errorf("failed to get target model: %w", err)
	}
	targetVendor, err := myTarget.GetVendor()
	if err != nil {
		return fmt.Errorf("failed to get target vendor: %w", err)
	}
	if targetVendor != cpus.IntelVendor {
		return fmt.Errorf("core frequency setting not supported on %s due to vendor mismatch", myTarget.GetName())
	}

	// retrieve the original frequency bucket sizes so that we can expand the consolidated input
	output, err := runScript(myTarget, script.GetScriptByName(script.SpecCoreFrequenciesScriptName), localTempDir)
	if err != nil {
		return fmt.Errorf("failed to get original frequency buckets: %w", err)
	}
	// expected script output format, the number of fields may vary:
	// "cores sse avx2 avx512 avx512h amx"
	// "hex hex hex hex hex hex"

	// confirm output format
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("unexpected output format from spec-core-frequencies script")
	}
	// extract the bucket sizes from the first field (cores) in the 2nd line
	coreCountsHex := strings.Fields(lines[1])[0]
	bucketSizes, err := util.HexToIntList(coreCountsHex)
	if err != nil {
		return fmt.Errorf("failed to parse core counts from hex: %w", err)
	}
	// there should be 8 buckets
	if len(bucketSizes) != 8 {
		return fmt.Errorf("unexpected number of core buckets: %d", len(bucketSizes))
	}
	// they are in reverse order, so reverse the slice
	slices.Reverse(bucketSizes)

	// expand the consolidated input into the 8 original bucket sizes
	// archMultiplier is used to adjust core numbering for certain architectures, i.e., multiply core numbers by 2, 3, or 4.
	uarch, err := getUarch(myTarget, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to get microarchitecture: %w", err)
	}
	var archMultiplier int
	if strings.Contains(uarch, "SRF") || strings.Contains(uarch, "CWF") {
		archMultiplier = 4
	} else if strings.Contains(uarch, "GNR_X3") {
		archMultiplier = 3
	} else if strings.Contains(uarch, "GNR_X2") {
		archMultiplier = 2
	} else {
		archMultiplier = 1
	}
	if archMultiplier == 0 {
		return fmt.Errorf("unsupported microarchitecture for SSE frequency setting: %s", uarch)
	}
	adjustedBucketSizes := make([]int, len(bucketSizes))
	for i, size := range bucketSizes {
		adjustedBucketSizes[i] = size * archMultiplier
	}

	bucketFrequencies, err := expandConsolidatedFrequencies(sseFrequencies, adjustedBucketSizes)
	if err != nil {
		return fmt.Errorf("failed to expand consolidated frequencies: %w", err)
	}

	// Now set the frequencies using the same approach as setSSEFrequency
	var setScript script.ScriptDefinition

	if targetFamily == "6" && (targetModel == "175" || targetModel == "221") { // SRF, CWF
		// get the pstate driver
		getScript := script.ScriptDefinition{
			Name:           "get pstate driver",
			ScriptTemplate: "cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_driver",
			Vendors:        []string{cpus.IntelVendor},
		}
		output, err := runScript(myTarget, getScript, localTempDir)
		if err != nil {
			return fmt.Errorf("failed to get pstate driver: %w", err)
		}
		if strings.Contains(output, "intel_pstate") {
			// For SRF/CWF with intel_pstate, we only set 2 buckets
			var value uint64
			for i := range uint(2) {
				freqInt := uint64(bucketFrequencies[i] * 10)
				value = value | freqInt<<(i*8)
			}
			setScript = script.ScriptDefinition{
				Name:           "set frequency bins",
				ScriptTemplate: fmt.Sprintf("wrmsr 0x774 %d", value),
				Superuser:      true,
				Vendors:        []string{cpus.IntelVendor},
			}
		} else {
			// For non-intel_pstate driver
			freqInt := uint64(bucketFrequencies[0] * 10)
			value := freqInt << uint(2*8)
			setScript = script.ScriptDefinition{
				Name:           "set frequency bins",
				ScriptTemplate: fmt.Sprintf("wrmsr 0x199 %d", value),
				Superuser:      true,
				Vendors:        []string{cpus.IntelVendor},
			}
		}
	} else {
		// For other platforms, set all 8 buckets
		var value uint64
		for i := range uint(8) {
			freqInt := uint64(bucketFrequencies[i] * 10)
			value = value | freqInt<<(i*8)
		}
		setScript = script.ScriptDefinition{
			Name:           "set frequency bins",
			ScriptTemplate: fmt.Sprintf("wrmsr -a 0x1AD %d", value),
			Superuser:      true,
			Vendors:        []string{cpus.IntelVendor},
		}
	}

	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set core frequencies: %w", err)
	}
	return err
}

func setUncoreDieFrequency(maxFreq bool, computeDie bool, uncoreFrequency float64, myTarget target.Target, localTempDir string) error {
	// Acquire mutex lock to protect concurrent access
	uncoreDieFrequencyMutex.Lock()
	defer uncoreDieFrequencyMutex.Unlock()

	targetFamily, err := myTarget.GetFamily()
	if err != nil {
		return fmt.Errorf("failed to get target family: %w", err)
	}
	targetModel, err := myTarget.GetModel()
	if err != nil {
		return fmt.Errorf("failed to get target model: %w", err)
	}
	if targetFamily != "6" || (targetFamily == "6" && targetModel != "173" && targetModel != "174" && targetModel != "175" && targetModel != "221") { // not Intel || not GNR, GNR-D, SRF, CWF
		return fmt.Errorf("uncore frequency setting not supported on %s due to family/model mismatch", myTarget.GetName())
	}
	type dieId struct {
		instance string
		entry    string
	}
	var dies []dieId
	// build list of compute or IO dies
	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.GetScriptByName(script.UncoreDieTypesFromTPMIScriptName))
	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir, nil, "")
	if err != nil {
		return fmt.Errorf("failed to run scripts on target: %w", err)
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
	var freqType string
	if maxFreq {
		bits = "8:14" // bits 8:14 are the max frequency
		freqType = "max"
	} else {
		bits = "15:21" // bits 15:21 are the min frequency
		freqType = "min"
	}
	// run script for each die of specified type
	scripts = []script.ScriptDefinition{}
	for _, die := range dies {
		setScript := script.ScriptDefinition{
			Name:           fmt.Sprintf("write %s uncore frequency TPMI %s %s", freqType, die.instance, die.entry),
			ScriptTemplate: fmt.Sprintf("pcm-tpmi 2 0x18 -d -b %s -w %d -i %s -e %s", bits, value, die.instance, die.entry),
			Vendors:        []string{cpus.IntelVendor},
			Depends:        []string{"pcm-tpmi"},
			Superuser:      true,
			Sequential:     true,
		}
		scripts = append(scripts, setScript)
	}
	_, err = script.RunScripts(myTarget, scripts, false, localTempDir, nil, "")
	if err != nil {
		err = fmt.Errorf("failed to set uncore die frequency: %w", err)
		slog.Error(err.Error())
	}
	return err
}

func setUncoreFrequency(maxFreq bool, uncoreFrequency float64, myTarget target.Target, localTempDir string) error {
	// Acquire mutex lock to protect concurrent access
	uncoreFrequencyMutex.Lock()
	defer uncoreFrequencyMutex.Unlock()

	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.ScriptDefinition{
		Name:           "get uncore frequency MSR",
		ScriptTemplate: "rdmsr 0x620",
		Vendors:        []string{cpus.IntelVendor},
		Superuser:      true,
		// Depends:        []string{"rdmsr"},
		// Lkms:           []string{"msr"},
	})
	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir, nil, "")
	if err != nil {
		return fmt.Errorf("failed to run scripts on target: %w", err)
	}
	targetFamily, err := myTarget.GetFamily()
	if err != nil {
		return fmt.Errorf("failed to get target family: %w", err)
	}
	targetModel, err := myTarget.GetModel()
	if err != nil {
		return fmt.Errorf("failed to get target model: %w", err)
	}
	if targetFamily != "6" || (targetFamily == "6" && (targetModel == "173" || targetModel == "174" || targetModel == "175" || targetModel == "221")) { // not Intel || not GNR, GNR-D, SRF, CWF
		return fmt.Errorf("uncore frequency setting not supported on %s due to family/model mismatch", myTarget.GetName())
	}
	msrUint, err := strconv.ParseUint(strings.TrimSpace(outputs["get uncore frequency MSR"].Stdout), 16, 0)
	if err != nil {
		return fmt.Errorf("failed to parse uncore frequency MSR: %w", err)
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
		Vendors:        []string{cpus.IntelVendor},
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set uncore frequency: %w", err)
	}
	return err
}

func setTDP(power int, myTarget target.Target, localTempDir string) error {
	readScript := script.ScriptDefinition{
		Name:           "get power MSR",
		ScriptTemplate: "rdmsr 0x610",
		Superuser:      true,
		Vendors:        []string{cpus.IntelVendor},
		// Lkms:           []string{"msr"},
		// Depends:        []string{"rdmsr"},
	}
	readOutput, err := script.RunScript(myTarget, readScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to read power MSR: %w", err)
	} else {
		msrHex := strings.TrimSpace(readOutput.Stdout)
		msrUint, err := strconv.ParseUint(msrHex, 16, 0)
		if err != nil {
			return fmt.Errorf("failed to parse power MSR: %w", err)
		} else {
			// mask out lower 14 bits
			newVal := uint64(msrUint) & 0xFFFFFFFFFFFFC000
			// add in the new power value
			newVal = newVal | uint64(power*8) // #nosec G115
			setScript := script.ScriptDefinition{
				Name:           "set tdp",
				ScriptTemplate: fmt.Sprintf("wrmsr -a 0x610 %d", newVal),
				Superuser:      true,
				Vendors:        []string{cpus.IntelVendor},
				// Depends:        []string{"wrmsr"},
				// Lkms:           []string{"msr"},
			}
			_, err := runScript(myTarget, setScript, localTempDir)
			if err != nil {
				return fmt.Errorf("failed to set power: %w", err)
			}
		}
	}
	return nil
}

func setEPB(epb int, myTarget target.Target, localTempDir string) error {
	epbSourceScript := script.GetScriptByName(script.EpbSourceScriptName)
	epbSourceOutput, err := runScript(myTarget, epbSourceScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to get EPB source: %w", err)
	}
	epbSource := strings.TrimSpace(epbSourceOutput)
	source, err := strconv.ParseInt(epbSource, 16, 0)
	if err != nil {
		return fmt.Errorf("failed to parse EPB source: %w", err)
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
		Vendors:        []string{cpus.IntelVendor},
		Superuser:      true,
		// Lkms:           []string{"msr"},
		// Depends:        []string{"rdmsr"},
	}
	readOutput, err := runScript(myTarget, readScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to read EPB MSR %s: %w", msr, err)
	}
	msrValue, err := strconv.ParseUint(strings.TrimSpace(readOutput), 16, 64)
	if err != nil {
		return fmt.Errorf("failed to parse EPB MSR %s: %w", msr, err)
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
		Vendors:        []string{cpus.IntelVendor},
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set EPB: %w", err)
	}
	return err
}

func setEPP(epp int, myTarget target.Target, localTempDir string) error {
	// Set both the per-core EPP value and the package EPP value
	// Reference: 15.4.4 Managing HWP in the Intel SDM

	// get the current value of the IAEW_HWP_REQUEST MSR that includes the current EPP valid value in bit 60
	getScript := script.ScriptDefinition{
		Name:           "get epp msr",
		ScriptTemplate: "rdmsr 0x774", // IA32_HWP_REQUEST
		Vendors:        []string{cpus.IntelVendor},
		Superuser:      true,
		// Lkms:           []string{"msr"},
		// Depends:        []string{"rdmsr"},
	}
	stdout, err := runScript(myTarget, getScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to read EPP MSR %s: %w", "0x774", err)
	}
	msrValue, err := strconv.ParseUint(strings.TrimSpace(stdout), 16, 64)
	if err != nil {
		return fmt.Errorf("failed to parse EPP MSR %s: %w", "0x774", err)
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
		Vendors:        []string{cpus.IntelVendor},
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to set EPP: %w", err)
	}
	// get the current value of the IA32_HWP_REQUEST_PKG MSR that includes the current package EPP value
	getScript = script.ScriptDefinition{
		Name:           "get epp pkg msr",
		ScriptTemplate: "rdmsr 0x772", // IA32_HWP_REQUEST_PKG
		Vendors:        []string{cpus.IntelVendor},
		Superuser:      true,
		// Lkms:           []string{"msr"},
		// Depends:        []string{"rdmsr"},
	}
	stdout, err = runScript(myTarget, getScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to read EPP pkg MSR %s: %w", "0x772", err)
	}
	msrValue, err = strconv.ParseUint(strings.TrimSpace(stdout), 16, 64)
	if err != nil {
		return fmt.Errorf("failed to parse EPP pkg MSR %s: %w", "0x772", err)
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
		Vendors:        []string{cpus.IntelVendor},
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set EPP pkg: %w", err)
	}
	return err
}

func setGovernor(governor string, myTarget target.Target, localTempDir string) error {
	setScript := script.ScriptDefinition{
		Name:           "set governor",
		ScriptTemplate: fmt.Sprintf("echo %s | tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor", governor),
		Superuser:      true,
	}
	_, err := runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set governor: %w", err)
	}
	return err
}

func setELC(elc string, myTarget target.Target, localTempDir string) error {
	var mode string
	switch elc {
	case elcOptions[0]:
		mode = "latency-optimized-mode"
	case elcOptions[1]:
		mode = "default"
	default:
		return fmt.Errorf("invalid ELC mode: %s", elc)
	}
	setScript := script.ScriptDefinition{
		Name:               "set elc",
		ScriptTemplate:     fmt.Sprintf("bhs-power-mode.sh --%s", mode),
		Superuser:          true,
		Vendors:            []string{cpus.IntelVendor},
		MicroArchitectures: []string{"GNR", "GNR-D", "SRF", "CWF"},
		Depends:            []string{"bhs-power-mode.sh", "pcm-tpmi"},
	}
	_, err := runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set ELC mode: %w", err)
	}
	return err
}

func getUarch(myTarget target.Target, localTempDir string) (string, error) {
	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.GetScriptByName(script.LscpuScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciBitsScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciDevicesScriptName))
	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to run scripts on target: %w", err)
	}
	uarch := report.UarchFromOutput(outputs)
	if uarch == "" {
		return "", fmt.Errorf("failed to get microarchitecture")
	}
	return uarch, nil
}

func setPrefetcher(enableDisable string, myTarget target.Target, localTempDir string, prefetcherType string) error {
	pf, err := report.GetPrefetcherDefByName(prefetcherType)
	if err != nil {
		return fmt.Errorf("failed to get prefetcher definition: %w", err)
	}
	uarch, err := getUarch(myTarget, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to get microarchitecture: %w", err)
	}
	// is the prefetcher supported on this uarch?
	if !slices.Contains(pf.Uarchs, "all") && !slices.Contains(pf.Uarchs, uarch[:3]) {
		return fmt.Errorf("prefetcher %s is not supported on %s", prefetcherType, uarch)
	}
	// get the current value of the prefetcher MSR
	getScript := script.ScriptDefinition{
		Name:           "get prefetcher msr",
		ScriptTemplate: fmt.Sprintf("rdmsr %d", pf.Msr),
		Vendors:        []string{cpus.IntelVendor},
		Superuser:      true,
		// Lkms:           []string{"msr"},
		// Depends:        []string{"rdmsr"},
	}
	stdout, err := runScript(myTarget, getScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to read prefetcher MSR: %w", err)
	}
	msrValue, err := strconv.ParseUint(strings.TrimSpace(stdout), 16, 64)
	if err != nil {
		return fmt.Errorf("failed to parse prefetcher MSR: %w", err)
	}
	// set the prefetcher bit to bitValue determined by the onOff value, note: 0 is enable, 1 is disable
	var bitVal uint64
	switch enableDisable {
	case prefetcherOptions[0]:
		bitVal = 0
	case prefetcherOptions[1]:
		bitVal = 1
	default:
		return fmt.Errorf("invalid prefetcher setting: %s", enableDisable)
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
		Vendors:        []string{cpus.IntelVendor},
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set %s prefetcher: %w", prefetcherType, err)
	}
	return err
}

// setC6 enables or disables C6 C-States
func setC6(enableDisable string, myTarget target.Target, localTempDir string) error {
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
		return fmt.Errorf("failed to get C6 state folders: %w", err)
	}
	c6StateFolders := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(c6StateFolders) == 0 {
		return fmt.Errorf("no C6 state folders found")
	}
	var enableDisableValue int
	switch enableDisable {
	case c6Options[0]: // enable
		enableDisableValue = 0
	case c6Options[1]: // disable
		enableDisableValue = 1
	default:
		return fmt.Errorf("invalid C6 setting: %s", enableDisable)
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
	return err
}

func setC1Demotion(enableDisable string, myTarget target.Target, localTempDir string) error {
	getScript := script.ScriptDefinition{
		Name:           "get C1 demotion",
		ScriptTemplate: "rdmsr 0xe2",
		Vendors:        []string{cpus.IntelVendor},
		Superuser:      true,
		// Lkms:           []string{"msr"},
		// Depends:        []string{"rdmsr"},
	}
	stdout, err := runScript(myTarget, getScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to get C1 demotion: %w", err)
	}
	msrValue, err := strconv.ParseUint(strings.TrimSpace(stdout), 16, 64)
	if err != nil {
		return fmt.Errorf("failed to parse C1 demotion MSR: %w", err)
	}
	// set the c1 demotion bits to bitValue, note: 1 is enable, 0 is disable
	var bitVal uint64
	switch enableDisable {
	case c1DemotionOptions[0]: // enable
		bitVal = 1
	case c1DemotionOptions[1]: // disable
		bitVal = 0
	default:
		return fmt.Errorf("invalid C1 demotion setting: %s", enableDisable)
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
		Vendors:        []string{cpus.IntelVendor},
		Superuser:      true,
		// Depends:        []string{"wrmsr"},
		// Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set C1 demotion: %w", err)
	}
	return err
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
