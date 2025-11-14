package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// table_helpers_frequency.go contains helper functions for parsing and processing CPU frequency data.

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"perfspect/internal/script"
	"perfspect/internal/util"

	"slices"
)

// getFrequenciesFromHex converts a hex string to a list of frequency integers.
// The frequencies are reversed to match the expected order.
func getFrequenciesFromHex(hex string) ([]int, error) {
	freqs, err := util.HexToIntList(hex)
	if err != nil {
		return nil, err
	}
	// reverse the order of the frequencies
	slices.Reverse(freqs)
	return freqs, nil
}

// getBucketSizesFromHex extracts bucket sizes from a hex string.
// Expects exactly 8 bucket sizes and reverses their order.
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

// getArchMultiplier returns the die multiplier for multi-die architectures.
// Returns 1 for single-die architectures.
func getArchMultiplier(arch string) int {
	if strings.Contains(arch, "SRF") || strings.Contains(arch, "CWF") {
		return 4
	} else if strings.Contains(arch, "GNR_X3") {
		return 3
	} else if strings.Contains(arch, "GNR_X2") {
		return 2
	}
	return 1
}

// parseFrequencyScriptOutput validates and parses the raw script output.
// Returns field names and hex values.
func parseFrequencyScriptOutput(output string) (fieldNames []string, hexValues []string, err error) {
	if output == "" {
		return nil, nil, fmt.Errorf("no core frequencies found")
	}

	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return nil, nil, fmt.Errorf("unexpected output format: need at least 2 lines")
	}

	fieldNames = strings.Fields(lines[0])
	if len(fieldNames) < 2 {
		return nil, nil, fmt.Errorf("unexpected output format: need at least 2 fields")
	}

	hexValues = strings.Fields(lines[1])
	if len(hexValues) != len(fieldNames) {
		return nil, nil, fmt.Errorf("unexpected output format: field count mismatch")
	}

	return fieldNames, hexValues, nil
}

// buildCoreBuckets creates core range strings for both total and per-die cores.
// For single-die architectures, totalCoreBuckets will be empty.
func buildCoreBuckets(bucketCoreCounts []int, archMultiplier int) (totalCoreBuckets, dieCoreBuckets []string) {
	totalCoreStart := 1
	dieStart := 1

	for _, count := range bucketCoreCounts {
		if dieStart > count {
			break
		}

		// Build per-die bucket
		dieCoreBuckets = append(dieCoreBuckets, fmt.Sprintf("%d-%d", dieStart, count))
		dieStart = count + 1

		// Build total bucket for multi-die architectures
		if archMultiplier > 1 {
			totalCoreCount := count * archMultiplier
			if totalCoreStart > totalCoreCount {
				break
			}
			totalCoreBuckets = append(totalCoreBuckets, fmt.Sprintf("%d-%d", totalCoreStart, totalCoreCount))
			totalCoreStart = totalCoreCount + 1
		}
	}

	return totalCoreBuckets, dieCoreBuckets
}

// parseISAFrequencies converts hex frequency values to GHz strings.
func parseISAFrequencies(isaHex string, bucketCount int) ([]string, error) {
	freqs, err := getFrequenciesFromHex(isaHex)
	if err != nil {
		return nil, fmt.Errorf("failed to get frequencies from hex: %w", err)
	}

	// Pad if necessary to match bucket count
	if len(freqs) != bucketCount {
		freqs, err = padFrequencies(freqs, bucketCount)
		if err != nil {
			return nil, fmt.Errorf("failed to pad frequencies: %w", err)
		}
	}

	// Convert to GHz strings
	isaFreqs := make([]string, len(freqs))
	for i, freq := range freqs {
		freqGHz := float64(freq) / 10.0
		isaFreqs[i] = fmt.Sprintf("%.1f", freqGHz)
	}

	return isaFreqs, nil
}

// isISASupported checks if an ISA has non-zero frequencies.
func isISASupported(isaFreqs []string) bool {
	return len(isaFreqs) > 0 && isaFreqs[0] != "0.0"
}

// buildFrequencyTableHeader creates the header row for the frequency table.
func buildFrequencyTableHeader(fieldNames []string, allIsaFreqs [][]string, archMultiplier int) []string {
	header := []string{"Cores"}

	if archMultiplier > 1 {
		header = append(header, "Cores per Die")
	}

	// Add ISA names for supported ISAs only
	for i, isaFreqs := range allIsaFreqs {
		if isISASupported(isaFreqs) {
			header = append(header, strings.ToUpper(fieldNames[i+1]))
		}
	}

	return header
}

// buildFrequencyTableRow creates a single data row for the frequency table.
func buildFrequencyTableRow(bucketIdx int, totalCoreBuckets, dieCoreBuckets []string,
	allIsaFreqs [][]string, archMultiplier int) ([]string, error) {

	row := make([]string, 0, len(allIsaFreqs)+2)

	// Add total core bucket for multi-die architectures
	if archMultiplier > 1 {
		if bucketIdx >= len(totalCoreBuckets) {
			return nil, fmt.Errorf("bucket index %d out of range for total core buckets", bucketIdx)
		}
		row = append(row, totalCoreBuckets[bucketIdx])
	}

	// Add per-die core bucket
	row = append(row, dieCoreBuckets[bucketIdx])

	// Add frequency values for supported ISAs
	for _, isaFreqs := range allIsaFreqs {
		if !isISASupported(isaFreqs) {
			continue
		}
		if bucketIdx >= len(isaFreqs) {
			return nil, fmt.Errorf("bucket index %d out of range for ISA frequencies", bucketIdx)
		}
		row = append(row, isaFreqs[bucketIdx])
	}

	return row, nil
}

// getSpecFrequencyBuckets parses turbo frequency data and returns a formatted table.
// The table structure is:
//   - First row: header with column names (Cores, [Cores per Die], ISA1, ISA2, ...)
//   - Subsequent rows: frequency data for each core count bucket
//
// Example output for multi-die architecture:
//
//	["Cores", "Cores per Die", "SSE", "AVX2", "AVX512"]
//	["0-41", "0-20", "3.5", "3.5", "3.3"]
//	["42-63", "21-31", "3.5", "3.5", "3.3"]
//
// The "Cores per Die" column is only present for multi-die architectures (GNR_X2, GNR_X3, SRF, CWF).
func getSpecFrequencyBuckets(outputs map[string]script.ScriptOutput) ([][]string, error) {
	// Get architecture to determine die multiplier
	arch := UarchFromOutput(outputs)
	if arch == "" {
		return nil, fmt.Errorf("uarch is required")
	}
	archMultiplier := getArchMultiplier(arch)

	// Parse script output
	fieldNames, hexValues, err := parseFrequencyScriptOutput(outputs[script.SpecCoreFrequenciesScriptName].Stdout)
	if err != nil {
		return nil, err
	}

	// Extract bucket sizes from first hex value
	bucketCoreCounts, err := getBucketSizesFromHex(hexValues[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket sizes: %w", err)
	}

	// Build core range strings
	totalCoreBuckets, dieCoreBuckets := buildCoreBuckets(bucketCoreCounts, archMultiplier)

	// Parse ISA frequencies from remaining hex values
	allIsaFreqs := make([][]string, 0, len(hexValues)-1)
	for _, isaHex := range hexValues[1:] {
		isaFreqs, err := parseISAFrequencies(isaHex, len(bucketCoreCounts))
		if err != nil {
			return nil, err
		}
		allIsaFreqs = append(allIsaFreqs, isaFreqs)
	}

	// Build output table
	table := make([][]string, 0, len(dieCoreBuckets)+1)

	// Add header row
	header := buildFrequencyTableHeader(fieldNames, allIsaFreqs, archMultiplier)
	table = append(table, header)

	// Add data rows
	for i := range dieCoreBuckets {
		row, err := buildFrequencyTableRow(i, totalCoreBuckets, dieCoreBuckets, allIsaFreqs, archMultiplier)
		if err != nil {
			return nil, err
		}
		table = append(table, row)
	}

	return table, nil
}

// expandTurboFrequencies expands the turbo frequencies to a list of frequencies
// input is the output of getSpecFrequencyBuckets, e.g.:
// "cores", "cores per die", "sse", "avx2", "avx512", "avx512h", "amx"
// "0-41", "0-20", "3.5", "3.5", "3.3", "3.2", "3.1"
// "42-63", "21-31", "3.5", "3.5", "3.3", "3.2", "3.1"
// ...
// output is the expanded list of the frequencies for the requested ISA
func expandTurboFrequencies(specFrequencyBuckets [][]string, isa string) ([]string, error) {
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

// maxFrequencyFromOutput gets max core frequency
//
//	1st option) /sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq
//	2nd option) from MSR/tpmi
//	3rd option) from dmidecode "Max Speed"
func maxFrequencyFromOutput(outputs map[string]script.ScriptOutput) string {
	cmdout := strings.TrimSpace(outputs[script.MaximumFrequencyScriptName].Stdout)
	if cmdout != "" {
		freqf, err := strconv.ParseFloat(cmdout, 64)
		if err == nil {
			freqf = freqf / 1000000
			return fmt.Sprintf("%.1fGHz", freqf)
		}
	}
	// get the max frequency from the MSR/tpmi
	specCoreFrequencies, err := getSpecFrequencyBuckets(outputs)
	if err == nil {
		sseFreqs := getSSEFreqsFromBuckets(specCoreFrequencies)
		if len(sseFreqs) > 0 {
			// max (single-core) frequency is the first SSE frequency
			return sseFreqs[0] + "GHz"
		}
	}
	return valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "4", `Max Speed:\s(.*)`)
}

// getSSEFreqsFromBuckets extracts SSE frequency values from frequency buckets.
func getSSEFreqsFromBuckets(buckets [][]string) []string {
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

// allCoreMaxFrequencyFromOutput gets the all-core max frequency.
func allCoreMaxFrequencyFromOutput(outputs map[string]script.ScriptOutput) string {
	specCoreFrequencies, err := getSpecFrequencyBuckets(outputs)
	if err != nil {
		return ""
	}
	sseFreqs := getSSEFreqsFromBuckets(specCoreFrequencies)
	if len(sseFreqs) < 1 {
		return ""
	}
	// all core max frequency is the last SSE frequency
	return sseFreqs[len(sseFreqs)-1] + "GHz"
}

// baseFrequencyFromOutput gets base core frequency
//
//	1st option) /sys/devices/system/cpu/cpu0/cpufreq/base_frequency
//	2nd option) from dmidecode "Current Speed"
//	3nd option) parse it from the model name
func baseFrequencyFromOutput(outputs map[string]script.ScriptOutput) string {
	cmdout := strings.TrimSpace(outputs[script.BaseFrequencyScriptName].Stdout)
	if cmdout != "" {
		freqf, err := strconv.ParseFloat(cmdout, 64)
		if err == nil {
			freqf = freqf / 1000000
			return fmt.Sprintf("%.1fGHz", freqf)
		}
	}
	currentSpeedVal := valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "4", `Current Speed:\s(.*)$`)
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
	modelName := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^[Mm]odel name.*:\s*(.+?)$`)
	tokens = strings.Split(modelName, " ")
	if len(tokens) > 0 {
		lastToken := tokens[len(tokens)-1]
		if len(lastToken) > 0 && lastToken[len(lastToken)-1] == 'z' {
			return lastToken
		}
	}
	return ""
}

func uncoreMinMaxDieFrequencyFromOutput(maxFreq bool, computeDie bool, outputs map[string]script.ScriptOutput) string {
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
		slog.Error("failed to find uncore die type in TPMI output", slog.String("output", outputs[script.UncoreDieTypesFromTPMIScriptName].Stdout))
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

func uncoreMinMaxFrequencyFromOutput(maxFreq bool, outputs map[string]script.ScriptOutput) string {
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

func uncoreMinFrequencyFromOutput(outputs map[string]script.ScriptOutput) string {
	return uncoreMinMaxFrequencyFromOutput(false, outputs)
}

func uncoreMaxFrequencyFromOutput(outputs map[string]script.ScriptOutput) string {
	return uncoreMinMaxFrequencyFromOutput(true, outputs)
}
