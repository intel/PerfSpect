package common

import (
	"fmt"
	"log/slog"
	"perfspect/internal/cpus"
	"perfspect/internal/script"
	"perfspect/internal/util"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// BaseFrequencyFromOutput gets base core frequency
//
//	1st option) /sys/devices/system/cpu/cpu0/cpufreq/base_frequency
//	2nd option) from dmidecode "Current Speed"
//	3nd option) parse it from the model name
func BaseFrequencyFromOutput(outputs map[string]script.ScriptOutput) string {
	cmdout := strings.TrimSpace(outputs[script.BaseFrequencyScriptName].Stdout)
	if cmdout != "" {
		freqf, err := strconv.ParseFloat(cmdout, 64)
		if err == nil {
			freqf = freqf / 1000000
			return fmt.Sprintf("%.1fGHz", freqf)
		}
	}
	currentSpeedVal := ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "4", `Current Speed:\s(.*)$`)
	tokens := strings.Split(currentSpeedVal, " ")
	if len(tokens) == 2 {
		num, err := strconv.ParseFloat(tokens[0], 64)
		if err == nil {
			unit := tokens[1]
			if unit == "MHz" {
				num = num / 1000
				unit = "GHz"
			}
			return fmt.Sprintf("%.1f%s", num, unit)
		}
	}
	// the frequency (if included) is at the end of the model name in lscpu's output
	modelName := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^[Mm]odel name.*:\s*(.+?)$`)
	tokens = strings.Split(modelName, " ")
	if len(tokens) > 0 {
		lastToken := tokens[len(tokens)-1]
		if len(lastToken) > 0 && lastToken[len(lastToken)-1] == 'z' {
			return lastToken
		}
	}
	return ""
}

// getFrequenciesFromHex
func getFrequenciesFromHex(hex string) ([]int, error) {
	freqs, err := util.HexToIntList(hex)
	if err != nil {
		return nil, err
	}
	// reverse the order of the frequencies
	slices.Reverse(freqs)
	return freqs, nil
}

// getBucketSizesFromHex
func getBucketSizesFromHex(hex string) ([]int, error) {
	bucketSizes, err := util.HexToIntList(hex)
	if err != nil {
		return nil, err
	}
	if len(bucketSizes) != 8 {
		err = fmt.Errorf("expected 8 bucket sizes, got %d", len(bucketSizes))
		return nil, err
	}
	// reverse the order of the core counts
	slices.Reverse(bucketSizes)
	return bucketSizes, nil
}

// padFrequencies adds items to the frequencies slice until it reaches the desired length.
// The value of the added items is the same as the last item in the original slice.
func padFrequencies(freqs []int, desiredLength int) ([]int, error) {
	if len(freqs) == 0 {
		return nil, fmt.Errorf("cannot pad empty frequencies slice")
	}
	for len(freqs) < desiredLength {
		freqs = append(freqs, freqs[len(freqs)-1])
	}
	return freqs, nil
}

// GetSpecFrequencyBuckets gets the core frequency buckets from the script output
// returns slice of rows
// first row is header
// each row is a slice of strings
// "cores", "cores per die", "sse", "avx2", "avx512", "avx512h", "amx"
// "0-41", "0-20", "3.5", "3.5", "3.3", "3.2", "3.1"
// "42-63", "21-31", "3.5", "3.5", "3.3", "3.2", "3.1"
// "64-85", "32-43", "3.5", "3.5", "3.3", "3.2", "3.1"
// ...
// the "cores per die" column is only present for some architectures
func GetSpecFrequencyBuckets(outputs map[string]script.ScriptOutput) ([][]string, error) {
	arch := UarchFromOutput(outputs)
	if arch == "" {
		return nil, fmt.Errorf("uarch is required")
	}
	out := outputs[script.SpecCoreFrequenciesScriptName].Stdout
	// expected script output format, the number of fields may vary:
	// "cores sse avx2 avx512 avx512h amx"
	// "hex hex hex hex hex hex"
	if out == "" {
		return nil, fmt.Errorf("no core frequencies found")
	}
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected output format")
	}
	fieldNames := strings.Fields(lines[0])
	if len(fieldNames) < 2 {
		return nil, fmt.Errorf("unexpected output format")
	}
	values := strings.Fields(lines[1])
	if len(values) != len(fieldNames) {
		return nil, fmt.Errorf("unexpected output format")
	}
	// get list of buckets sizes
	bucketCoreCounts, err := getBucketSizesFromHex(values[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket sizes from Hex string: %w", err)
	}
	// create buckets
	var totalCoreBuckets []string // only for multi-die architectures
	var dieCoreBuckets []string
	totalCoreStartRange := 1
	startRange := 1
	var archMultiplier int
	if strings.Contains(arch, cpus.UarchSRF) || strings.Contains(arch, cpus.UarchCWF) {
		archMultiplier = 4
	} else if strings.Contains(arch, cpus.UarchGNR_X3) {
		archMultiplier = 3
	} else if strings.Contains(arch, cpus.UarchGNR_X2) {
		archMultiplier = 2
	} else {
		archMultiplier = 1
	}
	for _, count := range bucketCoreCounts {
		if startRange > count {
			break
		}
		if archMultiplier > 1 {
			totalCoreCount := count * archMultiplier
			if totalCoreStartRange > int(totalCoreCount) {
				break
			}
			totalCoreBuckets = append(totalCoreBuckets, fmt.Sprintf("%d-%d", totalCoreStartRange, totalCoreCount))
			totalCoreStartRange = int(totalCoreCount) + 1
		}
		dieCoreBuckets = append(dieCoreBuckets, fmt.Sprintf("%d-%d", startRange, count))
		startRange = int(count) + 1
	}
	// get the frequencies for each isa
	var allIsaFreqs [][]string
	for _, isaHex := range values[1:] {
		var isaFreqs []string
		var freqs []int
		if isaHex != "0" {
			var err error
			freqs, err = getFrequenciesFromHex(isaHex)
			if err != nil {
				return nil, fmt.Errorf("failed to get frequencies from Hex string: %w", err)
			}
		} else {
			// if the ISA is not supported, set the frequency to zero for all buckets
			freqs = make([]int, len(bucketCoreCounts))
			for i := range freqs {
				freqs[i] = 0
			}
		}
		if len(freqs) != len(bucketCoreCounts) {
			freqs, err = padFrequencies(freqs, len(bucketCoreCounts))
			if err != nil {
				return nil, fmt.Errorf("failed to pad frequencies: %w", err)
			}
		}
		for _, freq := range freqs {
			// convert freq to GHz
			freqf := float64(freq) / 10.0
			isaFreqs = append(isaFreqs, fmt.Sprintf("%.1f", freqf))
		}
		allIsaFreqs = append(allIsaFreqs, isaFreqs)
	}
	// format the output
	var specCoreFreqs [][]string
	specCoreFreqs = make([][]string, 1, len(dieCoreBuckets)+1)
	// add bucket field name(s)
	specCoreFreqs[0] = append(specCoreFreqs[0], "Cores")
	if archMultiplier > 1 {
		specCoreFreqs[0] = append(specCoreFreqs[0], "Cores per Die")
	}
	// add fieldNames for ISAs that have frequencies
	for i := range allIsaFreqs {
		if allIsaFreqs[i][0] == "0.0" {
			continue
		}
		specCoreFreqs[0] = append(specCoreFreqs[0], strings.ToUpper(fieldNames[i+1]))
	}
	for i, bucket := range dieCoreBuckets {
		row := make([]string, 0, len(allIsaFreqs)+2)
		// add the total core buckets for multi-die architectures
		if archMultiplier > 1 {
			row = append(row, totalCoreBuckets[i])
		}
		// add the die core buckets
		row = append(row, bucket)
		// add the frequencies for each ISA
		for _, isaFreqs := range allIsaFreqs {
			if isaFreqs[0] == "0.0" {
				continue
			} else {
				if i >= len(isaFreqs) {
					return nil, fmt.Errorf("index out of range for isa frequencies")
				}
				row = append(row, isaFreqs[i])
			}
		}
		specCoreFreqs = append(specCoreFreqs, row)
	}
	return specCoreFreqs, nil
}

// ExpandTurboFrequencies expands the turbo frequencies to a list of frequencies
// input is the output of getSpecFrequencyBuckets, e.g.:
// "cores", "cores per die", "sse", "avx2", "avx512", "avx512h", "amx"
// "0-41", "0-20", "3.5", "3.5", "3.3", "3.2", "3.1"
// "42-63", "21-31", "3.5", "3.5", "3.3", "3.2", "3.1"
// ...
// output is the expanded list of the frequencies for the requested ISA
func ExpandTurboFrequencies(specFrequencyBuckets [][]string, isa string) ([]string, error) {
	if len(specFrequencyBuckets) < 2 || len(specFrequencyBuckets[0]) < 2 {
		return nil, fmt.Errorf("unable to parse core frequency buckets")
	}
	rangeIdx := 0 // the first column is the bucket, e.g., 1-44
	// find the index of the ISA column
	var isaIdx int
	for i := 1; i < len(specFrequencyBuckets[0]); i++ {
		if strings.EqualFold(specFrequencyBuckets[0][i], isa) {
			isaIdx = i
			break
		}
	}
	if isaIdx == 0 {
		return nil, fmt.Errorf("unable to find %s frequency column", isa)
	}
	var freqs []string
	for i := 1; i < len(specFrequencyBuckets); i++ {
		bucketCores, err := util.IntRangeToIntList(strings.TrimSpace(specFrequencyBuckets[i][rangeIdx]))
		if err != nil {
			return nil, fmt.Errorf("unable to parse bucket range %s", specFrequencyBuckets[i][rangeIdx])
		}
		bucketFreq := strings.TrimSpace(specFrequencyBuckets[i][isaIdx])
		if bucketFreq == "" {
			return nil, fmt.Errorf("unable to parse bucket frequency %s", specFrequencyBuckets[i][isaIdx])
		}
		for range bucketCores {
			freqs = append(freqs, bucketFreq)
		}
	}
	return freqs, nil
}

// MaxFrequencyFromOutput gets max core frequency from MSR/TPMI
func MaxFrequencyFromOutput(outputs map[string]script.ScriptOutput) string {
	specCoreFrequencies, err := GetSpecFrequencyBuckets(outputs)
	if err == nil {
		sseFreqs := GetSSEFreqsFromBuckets(specCoreFrequencies)
		if len(sseFreqs) > 0 {
			// max (single-core) frequency is the first SSE frequency
			return sseFreqs[0] + "GHz"
		}
	}
	return ""
}

func GetSSEFreqsFromBuckets(buckets [][]string) []string {
	if len(buckets) < 2 {
		return nil
	}
	// find the SSE column
	sseColumn := -1
	for i, col := range buckets[0] {
		if strings.ToUpper(col) == "SSE" {
			sseColumn = i
			break
		}
	}
	if sseColumn == -1 {
		return nil
	}
	// get the SSE values from the buckets
	sse := make([]string, 0, len(buckets)-1)
	for i := 1; i < len(buckets); i++ {
		if len(buckets[i]) > sseColumn {
			sse = append(sse, buckets[i][sseColumn])
		}
	}
	return sse
}

func AllCoreMaxFrequencyFromOutput(outputs map[string]script.ScriptOutput) string {
	specCoreFrequencies, err := GetSpecFrequencyBuckets(outputs)
	if err != nil {
		return ""
	}
	sseFreqs := GetSSEFreqsFromBuckets(specCoreFrequencies)
	if len(sseFreqs) < 1 {
		return ""
	}
	// all core max frequency is the last SSE frequency
	return sseFreqs[len(sseFreqs)-1] + "GHz"
}

func UncoreMinMaxDieFrequencyFromOutput(maxFreq bool, computeDie bool, outputs map[string]script.ScriptOutput) string {
	// find the first die that matches requrested die type (compute or I/O)
	re := regexp.MustCompile(`Read bits \d+:\d+ value (\d+) from TPMI ID .* for entry (\d+) in instance (\d+)`)
	var instance, entry string
	found := false
	for line := range strings.SplitSeq(outputs[script.UncoreDieTypesFromTPMIScriptName].Stdout, "\n") {
		match := re.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if computeDie && match[1] == "0" {
			found = true
			entry = match[2]
			instance = match[3]
			break
		}
		if !computeDie && match[1] == "1" {
			found = true
			entry = match[2]
			instance = match[3]
			break
		}
	}
	if !found {
		slog.Warn("failed to find uncore die type in TPMI output", slog.String("output", outputs[script.UncoreDieTypesFromTPMIScriptName].Stdout))
		return ""
	}
	// get the frequency for the found die
	re = regexp.MustCompile(fmt.Sprintf(`Read bits \d+:\d+ value (\d+) from TPMI ID .* for entry %s in instance %s`, entry, instance))
	found = false
	var parsed int64
	var err error
	var scriptName string
	if maxFreq {
		scriptName = script.UncoreMaxFromTPMIScriptName
	} else {
		scriptName = script.UncoreMinFromTPMIScriptName
	}
	for line := range strings.SplitSeq(outputs[scriptName].Stdout, "\n") {
		match := re.FindStringSubmatch(line)
		if len(match) > 0 {
			found = true
			parsed, err = strconv.ParseInt(match[1], 10, 64)
			if err != nil {
				slog.Error("failed to parse uncore frequency", slog.String("error", err.Error()), slog.String("line", line))
				return ""
			}
			break
		}
	}
	if !found {
		slog.Error("failed to find uncore frequency in TPMI output", slog.String("output", outputs[scriptName].Stdout))
		return ""
	}
	return fmt.Sprintf("%.1fGHz", float64(parsed)/10)
}

func UncoreMinMaxFrequencyFromOutput(maxFreq bool, outputs map[string]script.ScriptOutput) string {
	var parsed int64
	var err error
	var scriptName string
	if maxFreq {
		scriptName = script.UncoreMaxFromMSRScriptName
	} else {
		scriptName = script.UncoreMinFromMSRScriptName
	}
	hex := strings.TrimSpace(outputs[scriptName].Stdout)
	if hex != "" && hex != "0" {
		parsed, err = strconv.ParseInt(hex, 16, 64)
		if err != nil {
			slog.Error("failed to parse uncore frequency", slog.String("error", err.Error()), slog.String("hex", hex))
			return ""
		}
	} else {
		slog.Warn("failed to get uncore frequency from MSR", slog.String("hex", hex))
		return ""
	}
	return fmt.Sprintf("%.1fGHz", float64(parsed)/10)
}

func UncoreMinFrequencyFromOutput(outputs map[string]script.ScriptOutput) string {
	return UncoreMinMaxFrequencyFromOutput(false, outputs)
}

func UncoreMaxFrequencyFromOutput(outputs map[string]script.ScriptOutput) string {
	return UncoreMinMaxFrequencyFromOutput(true, outputs)
}
