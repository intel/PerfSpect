package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// table_helpers.go contains helper functions that are used to extract values from the output of the scripts.

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"perfspect/internal/script"
	"perfspect/internal/util"
	"slices"
)

// valFromRegexSubmatch searches for a regex pattern in the given output string and returns the first captured group.
// If no match is found, an empty string is returned.
func valFromRegexSubmatch(output string, regex string) string {
	re := regexp.MustCompile(regex)
	for line := range strings.SplitSeq(output, "\n") {
		match := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

// valsFromRegexSubmatch extracts the captured groups from each line in the output
// that matches the given regular expression.
// It returns a slice of strings containing the captured values.
func valsFromRegexSubmatch(output string, regex string) []string {
	var vals []string
	re := regexp.MustCompile(regex)
	for line := range strings.SplitSeq(output, "\n") {
		match := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) > 1 {
			vals = append(vals, match[1])
		}
	}
	return vals
}

// return all matches for all capture groups in regex
func valsArrayFromRegexSubmatch(output string, regex string) (vals [][]string) {
	re := regexp.MustCompile(regex)
	for line := range strings.SplitSeq(output, "\n") {
		match := re.FindStringSubmatch(line)
		if len(match) > 1 {
			vals = append(vals, match[1:])
		}
	}
	return
}

// valFromDmiDecodeRegexSubmatch extracts a value from the DMI decode output using a regular expression.
// It takes the DMI decode output, the DMI type, and the regular expression as input parameters.
// It returns the extracted value as a string.
func valFromDmiDecodeRegexSubmatch(dmiDecodeOutput string, dmiType string, regex string) string {
	return valFromRegexSubmatch(getDmiDecodeType(dmiDecodeOutput, dmiType), regex)
}

func valsArrayFromDmiDecodeRegexSubmatch(dmiDecodeOutput string, dmiType string, regexes ...string) (vals [][]string) {
	var res []*regexp.Regexp
	for _, r := range regexes {
		re := regexp.MustCompile(r)
		res = append(res, re)
	}
	for _, entry := range getDmiDecodeEntries(dmiDecodeOutput, dmiType) {
		row := make([]string, len(res))
		for _, line := range entry {
			for i, re := range res {
				match := re.FindStringSubmatch(strings.TrimSpace(line))
				if len(match) > 1 {
					row[i] = match[1]
				}
			}
		}
		vals = append(vals, row)
	}
	return
}

// getDmiDecodeType extracts the lines from the given `dmiDecodeOutput` that belong to the specified `dmiType`.
func getDmiDecodeType(dmiDecodeOutput string, dmiType string) string {
	var lines []string
	start := false
	for line := range strings.SplitSeq(dmiDecodeOutput, "\n") {
		if start && strings.HasPrefix(line, "Handle ") {
			start = false
		}
		if strings.Contains(line, "DMI type "+dmiType+",") {
			start = true
		}
		if start {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// getDmiDecodeEntries extracts the entries from the given `dmiDecodeOutput` that belong to the specified `dmiType`.
func getDmiDecodeEntries(dmiDecodeOutput string, dmiType string) (entries [][]string) {
	lines := strings.Split(dmiDecodeOutput, "\n")
	var entry []string
	typeMatch := false
	for _, line := range lines {
		if strings.HasPrefix(line, "Handle ") {
			if strings.Contains(line, "DMI type "+dmiType+",") {
				// type match
				typeMatch = true
				entry = []string{}
			} else {
				// not a type match
				typeMatch = false
			}
		}
		if !typeMatch {
			continue
		}
		if line == "" {
			// end of type match entry
			entries = append(entries, entry)
		} else {
			// a line in the entry
			entry = append(entry, line)
		}
	}
	return
}

// uarchFromOutput returns the architecture of the CPU that matches family, model, stepping,
// capid4, and devices information from the output or an empty string, if no match is found.
func uarchFromOutput(outputs map[string]script.ScriptOutput) string {
	family := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU family:\s*(.+)$`)
	model := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model:\s*(.+)$`)
	stepping := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Stepping:\s*(.+)$`)
	capid4 := valFromRegexSubmatch(outputs[script.LspciBitsScriptName].Stdout, `^([0-9a-fA-F]+)`)
	devices := valFromRegexSubmatch(outputs[script.LspciDevicesScriptName].Stdout, `^([0-9]+)`)
	cpu, err := getCPUExtended(family, model, stepping, capid4, devices)
	if err == nil {
		return cpu.MicroArchitecture
	}
	return ""
}

// UarchFromOutput exports the uarchFromOutput function
func UarchFromOutput(outputs map[string]script.ScriptOutput) string {
	return uarchFromOutput(outputs)
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

// getSpecFrequencyBuckets
// returns slice of rows
// first row is header
// each row is a slice of strings
// "cores", "cores per die", "sse", "avx2", "avx512", "avx512h", "amx"
// "0-41", "0-20", "3.5", "3.5", "3.3", "3.2", "3.1"
// "42-63", "21-31", "3.5", "3.5", "3.3", "3.2", "3.1"
// "64-85", "32-43", "3.5", "3.5", "3.3", "3.2", "3.1"
// ...
// the "cores per die" column is only present for some architectures
func getSpecFrequencyBuckets(outputs map[string]script.ScriptOutput) ([][]string, error) {
	arch := uarchFromOutput(outputs)
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
	if strings.Contains(arch, "SRF") || strings.Contains(arch, "CWF") {
		archMultiplier = 4
	} else if strings.Contains(arch, "GNR_X3") {
		archMultiplier = 3
	} else if strings.Contains(arch, "GNR_X2") {
		archMultiplier = 2
	} else {
		archMultiplier = 1
	}
	for _, count := range bucketCoreCounts {
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
				row = append(row, isaFreqs[i])
			}
		}
		specCoreFreqs = append(specCoreFreqs, row)
	}
	return specCoreFreqs, nil
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

// maxFrequencyFromOutputs gets max core frequency
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

func hyperthreadingFromOutput(outputs map[string]script.ScriptOutput) string {
	family := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU family:\s*(.+)$`)
	model := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model:\s*(.+)$`)
	stepping := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Stepping:\s*(.+)$`)
	sockets := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)
	coresPerSocket := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(.+)$`)
	cpus := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU\(.*:\s*(.+?)$`)
	numCPUs, err := strconv.Atoi(cpus) // logical CPUs
	if err != nil {
		slog.Error("error parsing cpus from lscpu")
		return ""
	}
	numSockets, err := strconv.Atoi(sockets)
	if err != nil {
		slog.Error("error parsing sockets from lscpu")
		return ""
	}
	numCoresPerSocket, err := strconv.Atoi(coresPerSocket) // physical cores
	if err != nil {
		slog.Error("error parsing cores per sockets from lscpu")
		return ""
	}
	cpu, err := getCPUExtended(family, model, stepping, "", "")
	if err != nil {
		return ""
	}
	if cpu.LogicalThreadCount < 2 {
		return "N/A"
	} else if numCPUs > numCoresPerSocket*numSockets {
		return "Enabled"
	} else {
		return "Disabled"
	}
}

func numaCPUListFromOutput(outputs map[string]script.ScriptOutput) string {
	nodeCPUs := valsFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node[0-9] CPU\(.*:\s*(.+?)$`)
	return strings.Join(nodeCPUs, " :: ")
}

func ppinsFromOutput(outputs map[string]script.ScriptOutput) string {
	uniquePpins := []string{}
	for line := range strings.SplitSeq(outputs[script.PPINName].Stdout, "\n") {
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		ppin := strings.TrimSpace(parts[1])
		found := false
		for _, p := range uniquePpins {
			if string(p) == ppin {
				found = true
				break
			}
		}
		if !found && ppin != "" {
			uniquePpins = append(uniquePpins, ppin)
		}
	}
	return strings.Join(uniquePpins, ", ")
}

func channelsFromOutput(outputs map[string]script.ScriptOutput) string {
	family := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU family:\s*(.+)$`)
	model := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model:\s*(.+)$`)
	stepping := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Stepping:\s*(.+)$`)
	capid4 := valFromRegexSubmatch(outputs[script.LspciBitsScriptName].Stdout, `^([0-9a-fA-F]+)`)
	devices := valFromRegexSubmatch(outputs[script.LspciDevicesScriptName].Stdout, `^([0-9]+)`)
	cpu, err := getCPUExtended(family, model, stepping, capid4, devices)
	if err != nil {
		slog.Error("error getting CPU from CPUdb", slog.String("error", err.Error()))
		return ""
	}
	return fmt.Sprintf("%d", cpu.MemoryChannelCount)
}

func turboEnabledFromOutput(outputs map[string]script.ScriptOutput) string {
	vendor := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Vendor ID:\s*(.+)$`)
	if vendor == "GenuineIntel" {
		val := valFromRegexSubmatch(outputs[script.CpuidScriptName].Stdout, `^Intel Turbo Boost Technology\s*= (.+?)$`)
		if val == "true" {
			return "Enabled"
		}
		if val == "false" {
			return "Disabled"
		}
		return "" // unknown value
	} else if vendor == "AuthenticAMD" {
		val := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Frequency boost.*:\s*(.+?)$`)
		if val != "" {
			return val + " (AMD Frequency Boost)"
		}
	}
	return ""
}

func isPrefetcherEnabled(msrValue string, bit int) (bool, error) {
	if msrValue == "" {
		return false, fmt.Errorf("msrValue is empty")
	}
	msrInt, err := strconv.ParseInt(msrValue, 16, 64)
	if err != nil {
		return false, fmt.Errorf("failed to parse msrValue: %s, %v", msrValue, err)
	}
	bitMask := int64(1) << bit
	// enabled if bit is zero
	return bitMask&msrInt == 0, nil
}

func prefetchersFromOutput(outputs map[string]script.ScriptOutput) [][]string {
	out := make([][]string, 0)
	uarch := uarchFromOutput(outputs)
	if uarch == "" {
		// uarch is required
		return [][]string{}
	}
	for _, pf := range prefetcherDefinitions {
		if slices.Contains(pf.Uarchs, "all") || slices.Contains(pf.Uarchs, uarch[:3]) {
			var scriptName string
			if pf.Msr == MsrPrefetchControl {
				scriptName = script.PrefetchControlName
			} else if pf.Msr == MsrPrefetchers {
				scriptName = script.PrefetchersName
			} else if pf.Msr == MsrAtomPrefTuning1 {
				scriptName = script.PrefetchersAtomName
			} else {
				slog.Error("unknown msr for prefetcher", slog.String("msr", fmt.Sprintf("0x%x", pf.Msr)))
				continue
			}
			msrVal := valFromRegexSubmatch(outputs[scriptName].Stdout, `^([0-9a-fA-F]+)`)
			if msrVal == "" {
				continue
			}
			var enabledDisabled string
			enabled, err := isPrefetcherEnabled(msrVal, pf.Bit)
			if err != nil {
				slog.Warn("error checking prefetcher enabled status", slog.String("error", err.Error()))
				continue
			}
			if enabled {
				enabledDisabled = "Enabled"
			} else {
				enabledDisabled = "Disabled"
			}
			out = append(out, []string{pf.ShortName, pf.Description, fmt.Sprintf("0x%04X", pf.Msr), strconv.Itoa(pf.Bit), enabledDisabled})
		}
	}
	return out
}

func prefetchersSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	uarch := uarchFromOutput(outputs)
	if uarch == "" {
		// uarch is required
		return ""
	}
	var prefList []string
	for _, pf := range prefetcherDefinitions {
		if slices.Contains(pf.Uarchs, "all") || slices.Contains(pf.Uarchs, uarch[:3]) {
			var scriptName string
			if pf.Msr == MsrPrefetchControl {
				scriptName = script.PrefetchControlName
			} else if pf.Msr == MsrPrefetchers {
				scriptName = script.PrefetchersName
			} else if pf.Msr == MsrAtomPrefTuning1 {
				scriptName = script.PrefetchersAtomName
			} else {
				slog.Error("unknown msr for prefetcher", slog.String("msr", fmt.Sprintf("0x%x", pf.Msr)))
				continue
			}
			msrVal := valFromRegexSubmatch(outputs[scriptName].Stdout, `^([0-9a-fA-F]+)`)
			if msrVal == "" {
				continue
			}
			var enabledDisabled string
			enabled, err := isPrefetcherEnabled(msrVal, pf.Bit)
			if err != nil {
				slog.Warn("error checking prefetcher enabled status", slog.String("error", err.Error()))
				continue
			}
			if enabled {
				enabledDisabled = "Enabled"
			} else {
				enabledDisabled = "Disabled"
			}
			prefList = append(prefList, fmt.Sprintf("%s: %s", pf.ShortName, enabledDisabled))
		}
	}
	if len(prefList) > 0 {
		return strings.Join(prefList, ", ")
	}
	return "None"
}

// getL3LscpuParts extracts the size, units, and instances of the L3 cache from the lscpu output.
// Examples:
// L3 cache:                   576 MiB (2 instances)
// L3 cache:                   210 MiB
func getL3LscpuParts(outputs map[string]script.ScriptOutput) (size float64, units string, instances int, err error) {
	l3Lscpu := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^L3 cache.*:\s*(.+?)$`)
	re := regexp.MustCompile(`(\d+\.?\d*)\s*(\w+)\s+\((\d+) instance[s]*\)`) // match known formats
	match := re.FindStringSubmatch(l3Lscpu)
	if match != nil {
		instances, err = strconv.Atoi(match[3])
		if err != nil {
			err = fmt.Errorf("failed to parse L3 instances from lscpu: %s, %v", l3Lscpu, err)
			return
		}
	} else {
		// try regex without the instance count
		re = regexp.MustCompile(`(\d+\.?\d*)\s*(\w+)`)
		match = re.FindStringSubmatch(l3Lscpu)
		if match == nil {
			err = fmt.Errorf("unknown L3 format in lscpu: %s", l3Lscpu)
			return
		}
		instances = 1
	}
	size, err = strconv.ParseFloat(match[1], 64)
	if err != nil {
		err = fmt.Errorf("failed to parse L3 size from lscpu: %s, %v", l3Lscpu, err)
		return
	}
	units = match[2]
	return
}

// get L3 per instance in MB from lscpu
func getL3LscpuMB(outputs map[string]script.ScriptOutput) (instanceSizeMB float64, instances int, err error) {
	l3SizeNoUnit, units, instances, err := getL3LscpuParts(outputs)
	if err != nil {
		return
	}
	if strings.ToLower(units[:1]) == "g" {
		instanceSizeMB = l3SizeNoUnit * 1024 / float64(instances)
		return
	}
	if strings.ToLower(units[:1]) == "m" {
		instanceSizeMB = l3SizeNoUnit / float64(instances)
		return
	}
	if strings.ToLower(units[:1]) == "k" {
		instanceSizeMB = l3SizeNoUnit / 1024 / float64(instances)
		return
	}
	err = fmt.Errorf("unknown L3 units in lscpu: %s", units)
	return
}

// GetL3LscpuMB exports getL3LscpuMB
func GetL3LscpuMB(outputs map[string]script.ScriptOutput) (instanceSizeMB float64, instances int, err error) {
	return getL3LscpuMB(outputs)
}

func getCacheWays(uArch string) (cacheWays []uint64) {
	cpu, err := getCPUByMicroArchitecture(uArch)
	if err != nil {
		return
	}
	return getCPUCacheWays(cpu)
}

// GetCacheWays exports the getCacheWays function
func GetCacheWays(uArch string) (cacheWays []uint64) {
	return getCacheWays(uArch)
}

// get L3 in MB from MSR
func getL3MSRMB(outputs map[string]script.ScriptOutput) (val float64, err error) {
	uarch := uarchFromOutput(outputs)
	if uarch == "" {
		err = fmt.Errorf("uarch is required")
		return
	}
	l3LscpuMB, _, err := getL3LscpuMB(outputs)
	if err != nil {
		return
	}
	l3MSRHex := strings.TrimSpace(outputs[script.L3WaySizeName].Stdout)
	l3MSR, err := strconv.ParseUint(l3MSRHex, 16, 64)
	if err != nil {
		err = fmt.Errorf("failed to parse MSR output: %s", l3MSRHex)
		return
	}
	cacheWays := getCacheWays(uarch)
	if len(cacheWays) == 0 {
		err = fmt.Errorf("failed to get cache ways for uArch: %s", uarch)
		return
	}
	cpul3SizeGB := l3LscpuMB / 1024
	GBperWay := cpul3SizeGB / float64(len(cacheWays))
	for i, way := range cacheWays {
		if way == l3MSR {
			val = float64(i+1) * GBperWay * 1024
			return
		}
	}
	err = fmt.Errorf("did not find %d in cache ways", l3MSR)
	return
}

// GetL3MSRMB exports the getL3MSRMB function
func GetL3MSRMB(outputs map[string]script.ScriptOutput) (val float64, err error) {
	return getL3MSRMB(outputs)
}

func l3FromOutput(outputs map[string]script.ScriptOutput) string {
	l3, err := getL3MSRMB(outputs)
	if err != nil {
		slog.Info("Could not get L3 size from MSR, falling back to lscpu", slog.String("error", err.Error()))
		l3, _, err = getL3LscpuMB(outputs)
		if err != nil {
			slog.Error("Could not get L3 size from lscpu", slog.String("error", err.Error()))
			return ""
		}
	}
	return fmt.Sprintf("%s MiB", strconv.FormatFloat(l3, 'f', -1, 64))
}

func l3PerCoreFromOutput(outputs map[string]script.ScriptOutput) string {
	virtualization := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Virtualization.*:\s*(.+?)$`)
	if virtualization == "full" {
		slog.Info("Can't calculate L3 per Core on virtualized host.")
		return ""
	}
	var l3PerCoreMB float64
	if l3, err := getL3MSRMB(outputs); err == nil {
		coresPerSocket, err := strconv.Atoi(valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket.*:\s*(.+?)$`))
		if err != nil || coresPerSocket == 0 {
			slog.Error("failed to parse cores per socket", slog.String("error", err.Error()))
			return ""
		}
		l3PerCoreMB = l3 / float64(coresPerSocket)
	} else {
		slog.Info("Could not get L3 size from MSR, falling back to lscpu", slog.String("error", err.Error()))
		instanceSizeMB, instances, err := getL3LscpuMB(outputs)
		if err != nil {
			slog.Error("Could not get L3 size from lscpu", slog.String("error", err.Error()))
			return ""
		}
		coresPerSocket, err := strconv.Atoi(valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket.*:\s*(.+?)$`))
		if err != nil || coresPerSocket == 0 {
			slog.Error("failed to parse cores per socket", slog.String("error", err.Error()))
			return ""
		}
		numSockets, err := strconv.Atoi(valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`))
		if err != nil || numSockets == 0 {
			slog.Error("failed to parse sockets", slog.String("error", err.Error()))
			return ""
		}
		l3PerCoreMB = (instanceSizeMB * float64(instances)) / (float64(coresPerSocket) * float64(numSockets))
	}
	val := strconv.FormatFloat(l3PerCoreMB, 'f', 3, 64)
	val = strings.TrimRight(val, "0") // trim trailing zeros
	val = strings.TrimRight(val, ".") // trim decimal point if trailing
	val += " MiB"
	return val
}

func acceleratorNames() []string {
	var names []string
	for _, accel := range acceleratorDefinitions {
		names = append(names, accel.Name)
	}
	return names
}

func acceleratorCountsFromOutput(outputs map[string]script.ScriptOutput) []string {
	var counts []string
	lshw := outputs[script.LshwScriptName].Stdout
	for _, accel := range acceleratorDefinitions {
		regex := fmt.Sprintf("%s:%s", accel.MfgID, accel.DevID)
		re := regexp.MustCompile(regex)
		count := len(re.FindAllString(lshw, -1))
		counts = append(counts, fmt.Sprintf("%d", count))
	}
	return counts
}

func acceleratorWorkQueuesFromOutput(outputs map[string]script.ScriptOutput) []string {
	var queues []string
	for _, accel := range acceleratorDefinitions {
		if accel.Name == "IAA" || accel.Name == "DSA" {
			var scriptName string
			if accel.Name == "IAA" {
				scriptName = script.IaaDevicesScriptName
			} else {
				scriptName = script.DsaDevicesScriptName
			}
			devices := outputs[scriptName].Stdout
			lines := strings.Split(devices, "\n")
			// get non-empty lines
			var nonEmptyLines []string
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					nonEmptyLines = append(nonEmptyLines, line)
				}
			}
			if len(nonEmptyLines) == 0 {
				queues = append(queues, "None")
			} else {
				queues = append(queues, strings.Join(nonEmptyLines, ", "))
			}
		} else {
			queues = append(queues, "N/A")
		}
	}
	return queues
}

func acceleratorFullNamesFromYaml() []string {
	var fullNames []string
	for _, accel := range acceleratorDefinitions {
		fullNames = append(fullNames, accel.FullName)
	}
	return fullNames
}

func acceleratorDescriptionsFromYaml() []string {
	var descriptions []string
	for _, accel := range acceleratorDefinitions {
		descriptions = append(descriptions, accel.Description)
	}
	return descriptions
}

func tdpFromOutput(outputs map[string]script.ScriptOutput) string {
	msrHex := strings.TrimSpace(outputs[script.PackagePowerLimitName].Stdout)
	msr, err := strconv.ParseInt(msrHex, 16, 0)
	if err != nil || msr == 0 {
		return ""
	}
	return fmt.Sprint(msr/8) + "W"
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

func chaCountFromOutput(outputs map[string]script.ScriptOutput) string {
	// output is the result of three rdmsr calls
	// - client cha count
	// - cha count
	// - spr cha count
	// stop when we find a non-zero value
	// note: rdmsr writes to stderr on error so we will likely have fewer than 3 lines in stdout
	for hexCount := range strings.SplitSeq(outputs[script.ChaCountScriptName].Stdout, "\n") {
		if hexCount != "" && hexCount != "0" {
			count, err := strconv.ParseInt(hexCount, 16, 64)
			if err == nil {
				return fmt.Sprintf("%d", count)
			}
		}
	}
	return ""
}

func elcFieldValuesFromOutput(outputs map[string]script.ScriptOutput) (fieldValues []Field) {
	if outputs[script.ElcScriptName].Stdout == "" {
		return
	}
	r := csv.NewReader(strings.NewReader(outputs[script.ElcScriptName].Stdout))
	rows, err := r.ReadAll()
	if err != nil {
		return
	}
	if len(rows) < 2 {
		return
	}
	// first row is headers
	for fieldNamesIndex, fieldName := range rows[0] {
		values := []string{}
		// value rows
		for _, row := range rows[1:] {
			values = append(values, row[fieldNamesIndex])
		}
		fieldValues = append(fieldValues, Field{Name: fieldName, Values: values})
	}

	// let's add an interpretation of the values in an additional column
	values := []string{}
	// value rows
	for _, row := range rows[1:] {
		var mode string
		if row[2] == "IO" {
			if row[5] == "0" && row[6] == "0" && row[7] == "0" {
				mode = "Latency Optimized"
			} else if row[5] == "800" && row[6] == "10" && row[7] == "94" {
				mode = "Default"
			} else {
				mode = "Custom"
			}
		} else { // COMPUTE
			if row[5] == "0" {
				mode = "Latency Optimized"
			} else if row[5] == "1200" {
				mode = "Default"
			} else {
				mode = "Custom"
			}
		}
		values = append(values, mode)
	}
	fieldValues = append(fieldValues, Field{Name: "Mode", Values: values})
	return
}

func elcSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	fieldValues := elcFieldValuesFromOutput(outputs)
	if len(fieldValues) == 0 {
		return ""
	}
	if len(fieldValues) < 10 {
		return ""
	}
	if len(fieldValues[9].Values) == 0 {
		return ""
	}
	summary := fieldValues[9].Values[0]
	for _, value := range fieldValues[9].Values[1:] {
		if value != summary {
			return "mixed"
		}
	}
	return summary
}

// epbFromOutput gets EPB value from script outputs
func epbFromOutput(outputs map[string]script.ScriptOutput) string {
	if outputs[script.EpbScriptName].Exitcode != 0 || len(outputs[script.EpbScriptName].Stdout) == 0 {
		slog.Warn("EPB scripts failed or produced no output")
		return ""
	}
	epb := strings.TrimSpace(outputs[script.EpbScriptName].Stdout)
	msr, err := strconv.ParseInt(epb, 16, 0)
	if err != nil {
		slog.Error("failed to parse EPB value", slog.String("error", err.Error()), slog.String("epb", epb))
		return ""
	}
	return epbValToLabel(int(msr))
}

func epbValToLabel(msr int) string {
	var val string
	if msr >= 0 && msr <= 3 {
		val = "Performance"
	} else if msr >= 4 && msr <= 7 {
		val = "Balanced Performance"
	} else if msr >= 8 && msr <= 11 {
		val = "Balanced Energy"
	} else if msr >= 12 {
		val = "Energy Efficient"
	}
	return fmt.Sprintf("%s (%d)", val, msr)
}

func eppValToLabel(msr int) string {
	var val string
	if msr == 128 {
		val = "Normal"
	} else if msr < 128 && msr > 64 {
		val = "Balanced Performance"
	} else if msr <= 64 {
		val = "Performance"
	} else if msr > 128 && msr < 192 {
		val = "Balanced Powersave"
	} else {
		val = "Powersave"
	}
	return fmt.Sprintf("%s (%d)", val, msr)
}

// eppFromOutput gets EPP value from script outputs
// IF 0x774[42] is '1' AND 0x774[60] is '0'
// THEN
//       get EPP from 0x772 (package)
// ELSE
//       get EPP from 0x774 (per core)
func eppFromOutput(outputs map[string]script.ScriptOutput) string {
	// if we couldn't get the EPP values, return empty string
	if outputs[script.EppValidScriptName].Exitcode != 0 || len(outputs[script.EppValidScriptName].Stdout) == 0 ||
		outputs[script.EppPackageControlScriptName].Exitcode != 0 || len(outputs[script.EppPackageControlScriptName].Stdout) == 0 ||
		outputs[script.EppPackageScriptName].Exitcode != 0 || len(outputs[script.EppPackageScriptName].Stdout) == 0 {
		slog.Warn("EPP scripts failed or produced no output")
		return ""
	}
	// check if the epp valid bit is set and consistent across all cores
	var eppValid string
	for i, line := range strings.Split(outputs[script.EppValidScriptName].Stdout, "\n") { // MSR 0x774, bit 60
		if line == "" {
			continue
		}
		currentEpbValid := strings.TrimSpace(strings.Split(line, ":")[1])
		if i == 0 {
			eppValid = currentEpbValid
			continue
		}
		if currentEpbValid != eppValid {
			slog.Warn("EPP valid bit is inconsistent across cores")
			return "inconsistent"
		}
	}
	// check if epp package control bit is set and consistent across all cores
	var eppPkgCtrl string
	for i, line := range strings.Split(outputs[script.EppPackageControlScriptName].Stdout, "\n") { // MSR 0x774, bit 42
		if line == "" {
			continue
		}
		currentEppPkgCtrl := strings.TrimSpace(strings.Split(line, ":")[1])
		if i == 0 {
			eppPkgCtrl = currentEppPkgCtrl
			continue
		}
		if currentEppPkgCtrl != eppPkgCtrl {
			slog.Warn("EPP package control bit is inconsistent across cores")
			return "inconsistent"
		}
	}
	if eppPkgCtrl == "1" && eppValid == "0" {
		eppPackage := strings.TrimSpace(outputs[script.EppPackageScriptName].Stdout) // MSR 0x772, bits 24-31  (package)
		msr, err := strconv.ParseInt(eppPackage, 16, 0)
		if err != nil {
			slog.Error("failed to parse EPP package value", slog.String("error", err.Error()), slog.String("epp", eppPackage))
			return ""
		}
		return eppValToLabel(int(msr))
	} else {
		var epp string
		for i, line := range strings.Split(outputs[script.EppScriptName].Stdout, "\n") { // MSR 0x774, bits 24-31 (per-core)
			if line == "" {
				continue
			}
			currentEpp := strings.TrimSpace(strings.Split(line, ":")[1])
			if i == 0 {
				epp = currentEpp
				continue
			}
			if currentEpp != epp {
				slog.Warn("EPP is inconsistent across cores")
				return "inconsistent"
			}
		}
		msr, err := strconv.ParseInt(epp, 16, 0)
		if err != nil {
			slog.Error("failed to parse EPP value", slog.String("error", err.Error()), slog.String("epp", epp))
			return ""
		}
		return eppValToLabel(int(msr))
	}
}

func operatingSystemFromOutput(outputs map[string]script.ScriptOutput) string {
	os := valFromRegexSubmatch(outputs[script.EtcReleaseScriptName].Stdout, `^PRETTY_NAME=\"(.+?)\"`)
	centos := valFromRegexSubmatch(outputs[script.EtcReleaseScriptName].Stdout, `^(CentOS Linux release .*)`)
	if centos != "" {
		os = centos
	}
	return os
}

type cstateInfo struct {
	Name   string
	Status string
}

func cstatesFromOutput(outputs map[string]script.ScriptOutput) []cstateInfo {
	var cstatesInfo []cstateInfo
	output := outputs[script.CstatesScriptName].Stdout
	for line := range strings.SplitSeq(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			return nil
		}
		cstatesInfo = append(cstatesInfo, cstateInfo{Name: parts[0], Status: parts[1]})
	}
	return cstatesInfo
}

func cstatesSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	cstatesInfo := cstatesFromOutput(outputs)
	if cstatesInfo == nil {
		return ""
	}
	summaryParts := []string{}
	for _, cstateInfo := range cstatesInfo {
		summaryParts = append(summaryParts, fmt.Sprintf("%s: %s", cstateInfo.Name, cstateInfo.Status))
	}
	return strings.Join(summaryParts, ", ")
}

type ISA struct {
	Name     string
	FullName string
	CPUID    string
}

var isas = []ISA{
	{"AES", "Advanced Encryption Standard New Instructions (AES-NI)", "AES instruction"},
	{"AMX", "Advanced Matrix Extensions (AMX)", "AMX-BF16: tile bfloat16 support"},
	{"AMX-COMPLEX", "AMX-COMPLEX Instruction", "AVX-COMPLEX instructions"},
	{"AMX-FP16", "AMX-FP16 Instruction", "AMX-FP16: FP16 tile operations"},
	{"AVX-IFMA", "AVX-IFMA Instruction", "AVX-IFMA: integer fused multiply add"},
	{"AVX-NE-CONVERT", "AVX-NE-CONVERT Instruction", "AVX-NE-CONVERT instructions"},
	{"AVX-VNNI-INT8", "AVX-VNNI-INT8 Instruction", "AVX-VNNI-INT8 instructions"},
	{"AVX512F", "AVX-512 Foundation", "AVX512F: AVX-512 foundation instructions"},
	{"AVX512_BF16", "Vector Neural Network Instructions (AVX512_BF16)", "AVX512_BF16: bfloat16 instructions"},
	{"AVX512_FP16", "Advanced Vector Extensions (AVX512_FP16)", "AVX512_FP16: fp16 support"},
	{"AVX512_VNNI", "Vector Neural Network Instructions (AVX512_VNNI)", "AVX512_VNNI: neural network instructions"},
	{"CLDEMOTE", "Cache Line Demote (CLDEMOTE)", "CLDEMOTE supports cache line demote"},
	{"CMPCCXADD", "Compare and Add if Condition is Met (CMPCCXADD)", "CMPccXADD instructions"},
	{"ENQCMD", "Enqueue Command Instruction (ENQCMD)", "ENQCMD instruction"},
	{"MOVDIRI", "Move Doubleword as Direct Store (MOVDIRI)", "MOVDIRI instruction"},
	{"MOVDIR64B", "Move 64 Bytes as Direct Store (MOVDIR64B)", "MOVDIR64B instruction"},
	{"PREFETCHIT0/1", "PREFETCHIT0/1 Instruction", "PREFETCHIT0, PREFETCHIT1 instructions"},
	{"SERIALIZE", "SERIALIZE Instruction", "SERIALIZE instruction"},
	{"SHA_NI", "SHA1/SHA256 Instruction Extensions (SHA_NI)", "SHA instructions"},
	{"TSXLDTRK", "Transactional Synchronization Extensions (TSXLDTRK)", "TSXLDTRK: TSX suspend load addr tracking"},
	{"VAES", "Vector AES", "VAES instructions"},
	{"WAITPKG", "UMONITOR, UMWAIT, TPAUSE Instructions", "WAITPKG instructions"},
}

func isaFullNames() []string {
	var names []string
	for _, isa := range isas {
		names = append(names, isa.FullName)
	}
	return names
}

func yesIfTrue(val string) string {
	if val == "true" {
		return "Yes"
	}
	return "No"
}

func isaSupportedFromOutput(outputs map[string]script.ScriptOutput) []string {
	var supported []string
	for _, isa := range isas {
		oneSupported := yesIfTrue(valFromRegexSubmatch(outputs[script.CpuidScriptName].Stdout, isa.CPUID+`\s*= (.+?)$`))
		supported = append(supported, oneSupported)
	}
	return supported
}

func numaBalancingFromOutput(outputs map[string]script.ScriptOutput) string {
	if strings.Contains(outputs[script.NumaBalancingScriptName].Stdout, "1") {
		return "Enabled"
	} else if strings.Contains(outputs[script.NumaBalancingScriptName].Stdout, "0") {
		return "Disabled"
	}
	return ""
}

func clusteringModeFromOutput(outputs map[string]script.ScriptOutput) string {
	uarch := uarchFromOutput(outputs)
	sockets := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)
	nodes := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^NUMA node\(s\):\s*(.+)$`)
	if uarch == "" || sockets == "" || nodes == "" {
		return ""
	}
	socketCount, err := strconv.Atoi(sockets)
	if err != nil {
		slog.Error("failed to parse socket count", slog.String("error", err.Error()))
		return ""
	}
	nodeCount, err := strconv.Atoi(nodes)
	if err != nil {
		slog.Error("failed to parse node count", slog.String("error", err.Error()))
		return ""
	}
	if nodeCount == 0 || socketCount == 0 {
		slog.Error("node count or socket count is zero")
		return ""
	}
	nodesPerSocket := nodeCount / socketCount
	if uarch == "GNR_X1" {
		return "All2All"
	} else if uarch == "GNR_X2" {
		if nodesPerSocket == 1 {
			return "UMA 4 (Quad)"
		} else if nodesPerSocket == 2 {
			return "SNC 2"
		}
	} else if uarch == "GNR_X3" {
		if nodesPerSocket == 1 {
			return "UMA 6 (Hex)"
		} else if nodesPerSocket == 3 {
			return "SNC 3"
		}
	} else if uarch == "SRF_SP" {
		return "UMA 2 (Hemi)"
	} else if uarch == "SRF_AP" {
		if nodesPerSocket == 1 {
			return "UMA 4 (Quad)"
		} else if nodesPerSocket == 2 {
			return "SNC 2"
		}
	} else if uarch == "CWF" {
		if nodesPerSocket == 1 {
			return "UMA 6 (Hex)"
		} else if nodesPerSocket == 3 {
			return "SNC 3"
		}
	}
	return ""
}

type nicInfo struct {
	Name            string
	Model           string
	Speed           string
	Link            string
	Bus             string
	Driver          string
	DriverVersion   string
	FirmwareVersion string
	MACAddress      string
	NUMANode        string
	CPUAffinity     string
	IRQBalance      string
}

func nicInfoFromOutput(outputs map[string]script.ScriptOutput) []nicInfo {
	// get nic names and models from lshw
	namesAndModels := valsArrayFromRegexSubmatch(outputs[script.LshwScriptName].Stdout, `^\S+\s+(\S+)\s+network\s+([^\[]+?)(?:\s+\[.*\])?$`)
	usbNICs := valsArrayFromRegexSubmatch(outputs[script.LshwScriptName].Stdout, `^usb.*? (\S+)\s+network\s+(\S.*?)$`)
	// if USB NIC name isn't already in the list, add it
	for _, usbNIC := range usbNICs {
		found := false
		for _, nameAndModel := range namesAndModels {
			if nameAndModel[0] == usbNIC[0] {
				found = true
				break
			}
		}
		if !found {
			namesAndModels = append(namesAndModels, usbNIC)
		}
	}
	if len(namesAndModels) == 0 {
		return nil
	}

	var nicInfos []nicInfo
	for _, nameAndModel := range namesAndModels {
		nicInfos = append(nicInfos, nicInfo{Name: nameAndModel[0], Model: nameAndModel[1]})
	}
	// separate output from the nic info script into per-nic output
	var perNicOutput [][]string
	reBeginNicInfo := regexp.MustCompile(`Settings for (.*):`)
	nicIndex := -1
	for line := range strings.SplitSeq(outputs[script.NicInfoScriptName].Stdout, "\n") {
		match := reBeginNicInfo.FindStringSubmatch(line)
		if len(match) > 0 {
			nicIndex++
			perNicOutput = append(perNicOutput, []string{})
		}
		if nicIndex == -1 { // we shouldn't be able to get here while nicIndex is -1
			slog.Warn("unexpected line in nic info output", slog.String("line", line))
			continue
		}
		perNicOutput[nicIndex] = append(perNicOutput[nicIndex], line)
	}
	if len(perNicOutput) != len(nicInfos) {
		slog.Error("number of nics in lshw and nicinfo output do not match")
		return nil
	}
	for nicIndex := range nicInfos {
		for _, line := range perNicOutput[nicIndex] {
			if strings.Contains(line, "Speed:") {
				nicInfos[nicIndex].Speed = strings.TrimSpace(line[strings.Index(line, ":")+1:])
			} else if strings.Contains(line, "Link detected:") {
				nicInfos[nicIndex].Link = strings.TrimSpace(line[strings.Index(line, ":")+1:])
			} else if strings.HasPrefix(line, "bus-info:") {
				nicInfos[nicIndex].Bus = strings.TrimSpace(line[strings.Index(line, ":")+1:])
			} else if strings.HasPrefix(line, "driver:") {
				nicInfos[nicIndex].Driver = strings.TrimSpace(line[strings.Index(line, ":")+1:])
			} else if strings.HasPrefix(line, "version:") {
				nicInfos[nicIndex].DriverVersion = strings.TrimSpace(line[strings.Index(line, ":")+1:])
			} else if strings.HasPrefix(line, "firmware-version:") {
				nicInfos[nicIndex].FirmwareVersion = strings.TrimSpace(line[strings.Index(line, ":")+1:])
			} else if strings.HasPrefix(line, "MAC Address:") {
				nicInfos[nicIndex].MACAddress = strings.TrimSpace(line[strings.Index(line, ":")+1:])
			} else if strings.HasPrefix(line, "NUMA Node:") {
				nicInfos[nicIndex].NUMANode = strings.TrimSpace(line[strings.Index(line, ":")+1:])
			} else if strings.Contains(line, "CPU Affinity:") {
				nicInfos[nicIndex].CPUAffinity = strings.TrimSpace(line[strings.Index(line, ":")+1:])
			} else if strings.Contains(line, "IRQ Balance:") {
				nicInfos[nicIndex].IRQBalance = strings.TrimSpace(line[strings.Index(line, ":")+1:])
			}
		}
	}
	return nicInfos
}

type diskInfo struct {
	Name             string
	Model            string
	Size             string
	MountPoint       string
	Type             string
	RequestQueueSize string
	MinIOSize        string
	FirmwareVersion  string
	PCIeAddress      string
	NUMANode         string
	LinkSpeed        string
	LinkWidth        string
	MaxLinkSpeed     string
	MaxLinkWidth     string
}

func diskInfoFromOutput(outputs map[string]script.ScriptOutput) []diskInfo {
	diskInfos := []diskInfo{}
	for i, line := range strings.Split(outputs[script.DiskInfoScriptName].Stdout, "\n") {
		// first line is the header
		if i == 0 {
			continue
		}
		if line == "" {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) != 14 {
			slog.Error("unexpected number of fields in disk info output", slog.String("line", line))
			return nil
		}
		// clean up the model name
		fields[1] = strings.TrimSpace(fields[1])
		// if we don't have a firmware version, try to get it from another source
		if fields[7] == "" {
			reFwRev := regexp.MustCompile(`FwRev=(\w+)`)
			reDev := regexp.MustCompile(fmt.Sprintf(`/dev/%s:`, fields[0]))
			devFound := false
			for line := range strings.SplitSeq(outputs[script.HdparmScriptName].Stdout, "\n") {
				if !devFound {
					if reDev.FindString(line) != "" {
						devFound = true
						continue
					}
				} else {
					match := reFwRev.FindStringSubmatch(line)
					if match != nil {
						fields[7] = match[1]
						break
					}
				}
			}
		}
		diskInfos = append(diskInfos, diskInfo{fields[0], fields[1], fields[2], fields[3], fields[4], fields[5], fields[6], fields[7], fields[8], fields[9], fields[10], fields[11], fields[12], fields[13]})
	}
	return diskInfos
}

func filesystemFieldValuesFromOutput(outputs map[string]script.ScriptOutput) []Field {
	fieldValues := []Field{}
	reFindmnt := regexp.MustCompile(`(.*)\s(.*)\s(.*)\s(.*)`)
	for i, line := range strings.Split(outputs[script.DfScriptName].Stdout, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		// "Mounted On" gets split into two fields, rejoin
		if i == 0 && fields[len(fields)-2] == "Mounted" && fields[len(fields)-1] == "on" {
			fields[len(fields)-2] = "Mounted on"
			fields = fields[:len(fields)-1]
			for _, field := range fields {
				fieldValues = append(fieldValues, Field{Name: field, Values: []string{}})
			}
			// add an additional field
			fieldValues = append(fieldValues, Field{Name: "Mount Options", Values: []string{}})
			continue
		}
		if len(fields) != len(fieldValues)-1 {
			slog.Error("unexpected number of fields in df output", slog.String("line", line))
			return nil
		}
		for i, field := range fields {
			fieldValues[i].Values = append(fieldValues[i].Values, field)
		}
		// get mount options for the current file system
		var options string
		for i, line := range strings.Split(outputs[script.FindMntScriptName].Stdout, "\n") {
			if i == 0 {
				continue
			}
			match := reFindmnt.FindStringSubmatch(line)
			if match != nil {
				target := match[1]
				source := match[2]
				if fields[0] == source && fields[5] == target {
					options = match[4]
					break
				}
			}
		}
		fieldValues[len(fieldValues)-1].Values = append(fieldValues[len(fieldValues)-1].Values, options)
	}
	return fieldValues
}

type GPU struct {
	Manufacturer string
	Model        string
	PCIID        string
}

func gpuInfoFromOutput(outputs map[string]script.ScriptOutput) []GPU {
	gpus := []GPU{}
	gpusLshw := valsArrayFromRegexSubmatch(outputs[script.LshwScriptName].Stdout, `^pci.*?\s+display\s+(\w+).*?\s+\[(\w+):(\w+)]$`)
	idxMfgName := 0
	idxMfgID := 1
	idxDevID := 2
	for _, gpu := range gpusLshw {
		// Find GPU in GPU defs, note the model
		var model string
		for _, intelGPU := range gpuDefinitions {
			if gpu[idxMfgID] == intelGPU.MfgID {
				model = intelGPU.Model
				break
			}
			re := regexp.MustCompile(intelGPU.DevID)
			if re.FindString(gpu[idxDevID]) != "" {
				model = intelGPU.Model
				break
			}
		}
		if model == "" {
			if gpu[idxMfgID] == "8086" {
				model = "Unknown Intel"
			} else {
				model = "Unknown"
			}
		}
		gpus = append(gpus, GPU{Manufacturer: gpu[idxMfgName], Model: model, PCIID: gpu[idxMfgID] + ":" + gpu[idxDevID]})
	}
	return gpus
}

type Gaudi struct {
	ModuleID      string
	SerialNumber  string
	BusID         string
	DriverVersion string
	EROM          string
	CPLD          string
	SPI           string
	NUMA          string
}

// output from the GaudiInfo script:
// module_id, serial, bus_id, driver_version
// 2, AM50016189, 0000:19:00.0, 1.17.0-28a11ca
// 6, AM50016165, 0000:b3:00.0, 1.17.0-28a11ca
// 3, AM50016119, 0000:1a:00.0, 1.17.0-28a11ca
// 0, AM50016134, 0000:43:00.0, 1.17.0-28a11ca
// 7, AM50016150, 0000:b4:00.0, 1.17.0-28a11ca
// 1, AM50016130, 0000:44:00.0, 1.17.0-28a11ca
// 4, AM50016127, 0000:cc:00.0, 1.17.0-28a11ca
// 5, AM50016122, 0000:cd:00.0, 1.17.0-28a11ca
//
// output from the GaudiNuma script:
// modID   NUMA Affinity
// -----   -------------
// 0       0
// 1       0
// 2       0
// 3       0
// 4       1
// 5       1
// 6       1
// 7       1
//
// output from the GaudiFirmware script:
// [0] AIP (accel0) 0000:19:00.0
//         erom
//                 component               : hl-gaudi2-1.17.0-fw-51.2.0-sec-9 (Jul 24 2024 - 11:31:46)
//                 fw os                   :
//         fuse
//                 component               : 01P0-HL2080A0-15-TF8A81-03-07-03
//                 fw os                   :
//         cpld
//                 component               : 0x00000010.653FB250
//                 fw os                   :
//         uboot
//                 component               : U-Boot 2021.04-hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//                 fw os                   :
//         arcmp
//                 component               : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//                 fw os                   : Linux gaudi2 5.10.18-hl-gaudi2-1.17.0-fw-51.2.0-sec-9 #1 SMP PREEMPT Wed Jul 24 11:44:52 IDT 2024 aarch64 GNU/Linux
//
//         preboot
//                 component               : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//                 fw os                   : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//                         eeprom          : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//                         boot_info       : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//         mgmt
//                 component               : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//                 fw os                   : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//                         i2c             : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//                         eeprom          : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//                         boot_info       : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//         pid
//                 component               : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//                 fw os                   : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//                         eeprom          : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//                         boot_info       : hl-gaudi2-1.17.0-fw-51.2.0-sec-9
//
// [1] AIP (accel1) 0000:b3:00.0 .......

func gaudiInfoFromOutput(outputs map[string]script.ScriptOutput) []Gaudi {
	gaudis := []Gaudi{}
	for i, line := range strings.Split(outputs[script.GaudiInfoScriptName].Stdout, "\n") {
		if line == "" || i == 0 { // skip blank lines and header
			continue
		}
		fields := strings.Split(line, ", ")
		if len(fields) != 4 {
			slog.Error("unexpected number of fields in gaudi info output", slog.String("line", line))
			continue
		}
		gaudis = append(gaudis, Gaudi{ModuleID: fields[0], SerialNumber: fields[1], BusID: fields[2], DriverVersion: fields[3]})
	}
	// sort the gaudis by module ID
	sort.Slice(gaudis, func(i, j int) bool {
		return gaudis[i].ModuleID < gaudis[j].ModuleID
	})
	// get NUMA affinity
	numaAffinities := valsArrayFromRegexSubmatch(outputs[script.GaudiNumaScriptName].Stdout, `^(\d+)\s+(\d+)\s+$`)
	if len(numaAffinities) != len(gaudis) {
		slog.Error("number of gaudis in gaudi info and numa output do not match", slog.Int("gaudis", len(gaudis)), slog.Int("numaAffinities", len(numaAffinities)))
		return nil
	}
	for i, numaAffinity := range numaAffinities {
		gaudis[i].NUMA = numaAffinity[1]
	}
	// get firmware versions
	reDevice := regexp.MustCompile(`^\[(\d+)] AIP \(accel\d+\) (.*)$`)
	reErom := regexp.MustCompile(`\s+erom$`)
	reCpld := regexp.MustCompile(`\s+cpld$`)
	rePreboot := regexp.MustCompile(`\s+preboot$`)
	reComponent := regexp.MustCompile(`^\s+component\s+:\s+hl-gaudi\d-(.*)-sec-\d+`)
	reCpldComponent := regexp.MustCompile(`^\s+component\s+:\s+(0x[0-9a-fA-F]+\.[0-9a-fA-F]+)$`)
	deviceIdx := -1
	state := -1
	for line := range strings.SplitSeq(outputs[script.GaudiFirmwareScriptName].Stdout, "\n") {
		if line == "" {
			continue
		}
		match := reDevice.FindStringSubmatch(line)
		if match != nil {
			var err error
			deviceIdx, err = strconv.Atoi(match[1])
			if err != nil {
				slog.Error("failed to parse device index", slog.String("deviceIdx", match[1]))
				return nil
			}
			if deviceIdx >= len(gaudis) {
				slog.Error("device index out of range", slog.Int("deviceIdx", deviceIdx), slog.Int("gaudis", len(gaudis)))
				return nil
			}
			continue
		}
		if deviceIdx == -1 {
			continue
		}
		if reErom.FindString(line) != "" {
			state = 0
			continue
		}
		if reCpld.FindString(line) != "" {
			state = 1
			continue
		}
		if rePreboot.FindString(line) != "" {
			state = 2
			continue
		}
		if state != -1 {
			switch state {
			case 0:
				match := reComponent.FindStringSubmatch(line)
				if match != nil {
					gaudis[deviceIdx].EROM = match[1]
				}
			case 1:
				match := reCpldComponent.FindStringSubmatch(line)
				if match != nil {
					gaudis[deviceIdx].CPLD = match[1]
				}
			case 2:
				match := reComponent.FindStringSubmatch(line)
				if match != nil {
					gaudis[deviceIdx].SPI = match[1]
				}
			}
			state = -1
		}
	}
	return gaudis
}

// return all PCI Devices of specified class
func getPCIDevices(class string, outputs map[string]script.ScriptOutput) (devices []map[string]string) {
	device := make(map[string]string)
	re := regexp.MustCompile(`^(\w+):\s+(.*)$`)
	for line := range strings.SplitSeq(outputs[script.LspciVmmScriptName].Stdout, "\n") {
		if line == "" { // end of device
			if devClass, ok := device["Class"]; ok {
				if devClass == class {
					devices = append(devices, device)
				}
			}
			device = make(map[string]string)
			continue
		}
		match := re.FindStringSubmatch(line)
		if len(match) > 0 {
			key := match[1]
			value := match[2]
			device[key] = value
		}
	}
	return
}

func cveInfoFromOutput(outputs map[string]script.ScriptOutput) [][]string {
	vulns := make(map[string]string)
	// from spectre-meltdown-checker
	for _, pair := range valsArrayFromRegexSubmatch(outputs[script.CveScriptName].Stdout, `(CVE-\d+-\d+): (.+)`) {
		vulns[pair[0]] = pair[1]
	}
	// sort the vulnerabilities by CVE ID
	var ids []string
	for id := range vulns {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	cves := make([][]string, 0)
	for _, id := range ids {
		cves = append(cves, []string{id, vulns[id]})
	}
	return cves
}

func turbostatSummaryRows(turboStatScriptOutput script.ScriptOutput, fieldNames []string) ([][]string, error) {
	var fieldValues [][]string
	// initialize indices with -1
	fieldIndices := make([]int, len(fieldNames))
	for i := range fieldIndices {
		fieldIndices[i] = -1
	}
	var startTime time.Time
	var interval int
	var sampleTime time.Time
	sampleCount := 0
	// parse the turbostat output
	for i, line := range strings.Split(turboStatScriptOutput.Stdout, "\n") {
		if i == 0 { // first line is the time stamp, e.g., TIME: 15:04:05
			var err error
			startTime, err = time.Parse("15:04:05", strings.TrimPrefix(line, "TIME: "))
			if err != nil {
				err := fmt.Errorf("unable to parse power stats start time: %s", line)
				return nil, err
			}
			continue
		} else if i == 1 { // second line is the collection interval, e.g., INTERVAL: 2
			var err error
			interval, err = strconv.Atoi(strings.TrimPrefix(line, "INTERVAL: "))
			if err != nil {
				err := fmt.Errorf("unable to parse power stats interval: %s", line)
				return nil, err
			}
			continue
		} else if i == 2 { // third line is the header, e.g., Package	Die	Node	Core	CPU	Avg_MHz	Busy%	Bzy_MHz	TSC_MHz	IPC
			// parse the field names from the header line
			tsFields := strings.Fields(line)
			// find the index of the fields we are interested in
			for j, tsFieldName := range tsFields {
				for k, fieldName := range fieldNames {
					if tsFieldName == fieldName {
						fieldIndices[k] = j
					}
				}
			}
			// check that we found all the fields
			if slices.Contains(fieldIndices, -1) {
				err := fmt.Errorf("turbostat output is missing field: %s", fieldNames)
				return nil, err
			}
			continue
		}
		// alright...we should be at the data now
		tsRowValues := strings.Fields(line)
		if len(tsRowValues) == 0 || tsRowValues[0] != "-" { // summary rows have a "-" in the first colum
			continue
		}
		sampleTime = startTime.Add(time.Duration(sampleCount*interval) * time.Second)
		sampleCount++
		rowValues := []string{sampleTime.Format("15:04:05")}
		for _, fieldIndex := range fieldIndices {
			rowValues = append(rowValues, tsRowValues[fieldIndex])
		}
		fieldValues = append(fieldValues, rowValues)
	}
	return fieldValues, nil
}

func nicIRQMappingsFromOutput(outputs map[string]script.ScriptOutput) [][]string {
	nicInfos := nicInfoFromOutput(outputs)
	if len(nicInfos) == 0 {
		return nil
	}
	nicIRQMappings := [][]string{}
	for _, nic := range nicInfos {
		// command output is formatted like this: 200:1;201:1-17,36-53;202:44
		// which is <irq>:<cpu(s)>
		// we need to reverse it to <cpu>:<irq(s)>
		cpuIRQMappings := make(map[int][]int)
		for pair := range strings.SplitSeq(nic.CPUAffinity, ";") {
			if pair == "" {
				continue
			}
			tokens := strings.Split(pair, ":")
			irq, err := strconv.Atoi(tokens[0])
			if err != nil {
				continue
			}
			cpuList := tokens[1]
			cpus, err := util.SelectiveIntRangeToIntList(cpuList)
			if err != nil {
				slog.Warn("failed to parse CPU list", slog.String("cpuList", cpuList), slog.String("error", err.Error()))
				continue
			}
			for _, cpu := range cpus {
				cpuIRQMappings[cpu] = append(cpuIRQMappings[cpu], irq)
			}
		}
		var cpuIRQs []string
		var cpus []int
		for k := range cpuIRQMappings {
			cpus = append(cpus, k)
		}
		sort.Ints(cpus)
		for _, cpu := range cpus {
			irqs := cpuIRQMappings[cpu]
			cpuIRQ := fmt.Sprintf("%d:", cpu)
			var irqStrings []string
			for _, irq := range irqs {
				irqStrings = append(irqStrings, fmt.Sprintf("%d", irq))
			}
			cpuIRQ += strings.Join(irqStrings, ",")
			cpuIRQs = append(cpuIRQs, cpuIRQ)
		}
		nicIRQMappings = append(nicIRQMappings, []string{nic.Name, strings.Join(cpuIRQs, " ")})
	}
	return nicIRQMappings
}

func nicSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	nics := nicInfoFromOutput(outputs)
	if len(nics) == 0 {
		return "N/A"
	}
	modelCount := make(map[string]int)
	for _, nic := range nics {
		modelCount[nic.Model]++
	}
	var summary []string
	for model, count := range modelCount {
		summary = append(summary, fmt.Sprintf("%dx %s", count, model))
	}
	return strings.Join(summary, ", ")
}

func diskSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	disks := diskInfoFromOutput(outputs)
	if len(disks) == 0 {
		return "N/A"
	}
	type ModelSize struct {
		model string
		size  string
	}
	modelSizeCount := make(map[ModelSize]int)
	for _, disk := range disks {
		if disk.Model == "" {
			continue
		}
		modelSize := ModelSize{model: disk.Model, size: disk.Size}
		modelSizeCount[modelSize]++
	}
	var summary []string
	for modelSize, count := range modelSizeCount {
		summary = append(summary, fmt.Sprintf("%dx %s %s", count, modelSize.size, modelSize.model))
	}
	return strings.Join(summary, ", ")
}

func acceleratorSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	var summary []string
	accelerators := acceleratorNames()
	counts := acceleratorCountsFromOutput(outputs)
	for i, name := range accelerators {
		if strings.Contains(name, "chipset") { // skip "QAT (on chipset)" in this table
			continue
		} else if strings.Contains(name, "CPU") { // rename "QAT (on CPU) to simply "QAT"
			name = "QAT"
		}
		summary = append(summary, fmt.Sprintf("%s %s [0]", name, counts[i]))
	}
	return strings.Join(summary, ", ")
}

func cveSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	cves := cveInfoFromOutput(outputs)
	if len(cves) == 0 {
		return ""
	}
	var numOK int
	var numVuln int
	for _, cve := range cves {
		if strings.HasPrefix(cve[1], "OK") {
			numOK++
		} else {
			numVuln++
		}
	}
	return fmt.Sprintf("%d OK, %d Vulnerable", numOK, numVuln)
}

func systemSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	// BASELINE: 1-node, 2x Intel® Xeon® <SKU, processor>, xx cores, 100W TDP, HT On/Off?, Turbo On/Off?, Total Memory xxx GB (xx slots/ xx GB/ xxxx MHz [run @ xxxx MHz] ), <BIOS version>, <ucode version>, <OS Version>, <kernel version>. Test by Intel as of <mm/dd/yy>.
	template := "1-node, %sx %s, %s cores, %s TDP, HT %s, Turbo %s, Total Memory %s, BIOS %s, microcode %s, %s, %s, %s, %s. Test by Intel as of %s."
	var socketCount, cpuModel, coreCount, tdp, htOnOff, turboOnOff, installedMem, biosVersion, uCodeVersion, nics, disks, operatingSystem, kernelVersion, date string

	// socket count
	socketCount = valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(\d+)$`)
	// CPU model
	cpuModel = valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model name:\s*(.+?)$`)
	// core count
	coreCount = valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(\d+)$`)
	// TDP
	tdp = tdpFromOutput(outputs)
	if tdp == "" {
		tdp = "?"
	}
	// hyperthreading
	htOnOff = hyperthreadingFromOutput(outputs)
	if htOnOff == "Enabled" {
		htOnOff = "On"
	} else if htOnOff == "Disabled" {
		htOnOff = "Off"
	} else if htOnOff == "N/A" {
		htOnOff = "N/A"
	} else {
		htOnOff = "?"
	}
	// turbo
	turboOnOff = turboEnabledFromOutput(outputs)
	if strings.Contains(strings.ToLower(turboOnOff), "enabled") {
		turboOnOff = "On"
	} else if strings.Contains(strings.ToLower(turboOnOff), "disabled") {
		turboOnOff = "Off"
	} else {
		turboOnOff = "?"
	}
	// memory
	installedMem = installedMemoryFromOutput(outputs)
	// BIOS
	biosVersion = valFromRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, `^Version:\s*(.+?)$`)
	// microcode
	uCodeVersion = valFromRegexSubmatch(outputs[script.ProcCpuinfoScriptName].Stdout, `^microcode.*:\s*(.+?)$`)
	// NICs
	nics = nicSummaryFromOutput(outputs)
	// disks
	disks = diskSummaryFromOutput(outputs)
	// OS
	operatingSystem = operatingSystemFromOutput(outputs)
	// kernel
	kernelVersion = valFromRegexSubmatch(outputs[script.UnameScriptName].Stdout, `^Linux \S+ (\S+)`)
	// date
	date = strings.TrimSpace(outputs[script.DateScriptName].Stdout)
	// put it all together
	return fmt.Sprintf(template, socketCount, cpuModel, coreCount, tdp, htOnOff, turboOnOff, installedMem, biosVersion, uCodeVersion, nics, disks, operatingSystem, kernelVersion, date)
}

// getSectionsFromOutput parses output into sections, where the section name
// is the key in a map and the section content is the value
// sections are delimited by lines of the form ########## <section name> ##########
// example:
// ########## <section A name> ##########
// <section content>
// <section content>
// ########## <section B name> ##########
// <section content>
//
// returns a map of section name to section content
// if the output is empty or contains no section headers, returns an empty map
// if a section contains no content, the value for that section is an empty string
func getSectionsFromOutput(output string) map[string]string {
	sections := make(map[string]string)
	re := regexp.MustCompile(`^########## (.+?) ##########$`)
	var sectionName string
	for line := range strings.SplitSeq(output, "\n") {
		// check if the line is a section header
		match := re.FindStringSubmatch(line)
		if match != nil {
			// if the section name isn't in the map yet, add it
			if _, ok := sections[match[1]]; !ok {
				sections[match[1]] = ""
			}
			// save the section name
			sectionName = match[1]
			continue
		}
		if sectionName != "" {
			sections[sectionName] += line + "\n"
		}
	}
	return sections
}

// sectionValueFromOutput returns the content of a section from the output
// if the section doesn't exist, returns an empty string
// if the section exists but has no content, returns an empty string
func sectionValueFromOutput(output string, sectionName string) string {
	sections := getSectionsFromOutput(output)
	if len(sections) == 0 {
		slog.Warn("no sections in output")
		return ""
	}
	if _, ok := sections[sectionName]; !ok {
		slog.Warn("section not found in output", slog.String("section", sectionName))
		return ""
	}
	if sections[sectionName] == "" {
		slog.Warn("No content for section:", slog.String("section", sectionName))
		return ""
	}
	return sections[sectionName]
}

func javaFoldedFromOutput(outputs map[string]script.ScriptOutput) string {
	sections := getSectionsFromOutput(outputs[script.CollapsedCallStacksScriptName].Stdout)
	if len(sections) == 0 {
		slog.Warn("no sections in collapsed call stack output")
		return ""
	}
	javaFolded := make(map[string]string)
	re := regexp.MustCompile(`^async-profiler (\d+) (.*)$`)
	for header, stacks := range sections {
		match := re.FindStringSubmatch(header)
		if match == nil {
			continue
		}
		pid := match[1]
		processName := match[2]
		if stacks == "" {
			slog.Warn("no stacks for java process", slog.String("header", header))
			continue
		}
		if strings.HasPrefix(stacks, "Failed to inject profiler") {
			slog.Error("profiling data error", slog.String("header", header))
			continue
		}
		_, ok := javaFolded[processName]
		if processName == "" {
			processName = "java (" + pid + ")"
		} else if ok {
			processName = processName + " (" + pid + ")"
		}
		javaFolded[processName] = stacks
	}
	folded, err := mergeJavaFolded(javaFolded)
	if err != nil {
		slog.Error("failed to merge java stacks", slog.String("error", err.Error()))
	}
	return folded
}

func nativeFoldedFromOutput(outputs map[string]script.ScriptOutput) string {
	sections := getSectionsFromOutput(outputs[script.CollapsedCallStacksScriptName].Stdout)
	if len(sections) == 0 {
		slog.Warn("no sections in collapsed call stack output")
		return ""
	}
	var dwarfFolded, fpFolded string
	for header, content := range sections {
		if header == "perf_dwarf" {
			dwarfFolded = content
		} else if header == "perf_fp" {
			fpFolded = content
		}
	}
	if dwarfFolded == "" && fpFolded == "" {
		return ""
	}
	folded, err := mergeSystemFolded(fpFolded, dwarfFolded)
	if err != nil {
		slog.Error("failed to merge native stacks", slog.String("error", err.Error()))
	}
	return folded
}

func maxRenderDepthFromOutput(outputs map[string]script.ScriptOutput) string {
	sections := getSectionsFromOutput(outputs[script.CollapsedCallStacksScriptName].Stdout)
	if len(sections) == 0 {
		slog.Warn("no sections in collapsed call stack output")
		return ""
	}
	for header, content := range sections {
		if header == "maximum depth" {
			return content
		}
	}
	return ""
}
