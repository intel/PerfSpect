// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package benchmark

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"perfspect/internal/extract"

	"perfspect/internal/cpus"
	"perfspect/internal/script"
	"perfspect/internal/table"
)

// table names
const (
	// benchmark table names
	SpeedBenchmarkTableName               = "Speed"
	PowerBenchmarkTableName               = "Power"
	TemperatureBenchmarkTableName         = "Temperature"
	FrequencyBenchmarkTableName           = "Frequency"
	MemoryLoadedLatencyBenchmarkTableName = "Memory Loaded Latency"
	MemoryBandwidthMatrixBenchmarkName    = "Memory NUMA Bandwidth Matrix (GB/s)"
	MemoryLatencyMatrixBenchmarkName      = "Memory NUMA Latency Matrix (ns)"
	CacheIdleLatencyBenchmarkTableName    = "Cache Idle Latency (ns)"
	CacheMaxBandwidthBenchmarkTableName   = "Cache Maximum Bandwidth (GB/s)"
	StorageBenchmarkTableName             = "Storage"
)

const (
	// menu labels
	SpeedBenchmarksMenuLabel       = "Speed"
	PowerBenchmarksMenuLabel       = "Power"
	TemperatureBenchmarksMenuLabel = "Temperature"
	FrequencyBenchmarksMenuLabel   = "Frequency"
	MemoryBenchmarksMenuLabel      = "Memory"
	CacheBenchmarksMenuLabel       = "Cache"
	StorageBenchmarksMenuLabel     = "Storage"
)

var tableDefinitions = map[string]table.TableDefinition{
	SpeedBenchmarkTableName: {
		Name:      SpeedBenchmarkTableName,
		MenuLabel: SpeedBenchmarksMenuLabel,
		HasRows:   false,
		ScriptNames: []string{
			script.SpeedBenchmarkScriptName,
		},
		FieldsFunc: speedBenchmarkTableValues},
	PowerBenchmarkTableName: {
		Name:          PowerBenchmarkTableName,
		MenuLabel:     PowerBenchmarksMenuLabel,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       false,
		ScriptNames: []string{
			script.IdlePowerBenchmarkScriptName,
			script.PowerBenchmarkScriptName,
		},
		FieldsFunc: powerBenchmarkTableValues},
	TemperatureBenchmarkTableName: {
		Name:          TemperatureBenchmarkTableName,
		MenuLabel:     TemperatureBenchmarksMenuLabel,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       false,
		ScriptNames: []string{
			script.PowerBenchmarkScriptName,
		},
		FieldsFunc: temperatureBenchmarkTableValues},
	FrequencyBenchmarkTableName: {
		Name:          FrequencyBenchmarkTableName,
		MenuLabel:     FrequencyBenchmarksMenuLabel,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.SpecCoreFrequenciesScriptName,
			script.LscpuScriptName,
			script.LspciBitsScriptName,
			script.LspciDevicesScriptName,
			script.FrequencyBenchmarkScriptName,
		},
		FieldsFunc: frequencyBenchmarkTableValues},
	MemoryLoadedLatencyBenchmarkTableName: {
		Name:          MemoryLoadedLatencyBenchmarkTableName,
		MenuLabel:     MemoryBenchmarksMenuLabel,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.MemoryLoadedLatencyBenchmarkScriptName,
		},
		NoDataFound: "No memory loaded latency benchmark data found. Please see the GitHub repository README for instructions on how to install Intel Memory Latency Checker (mlc).",
		FieldsFunc:  memoryBenchmarkTableValues},
	MemoryBandwidthMatrixBenchmarkName: {
		Name:          MemoryBandwidthMatrixBenchmarkName,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.MemoryNUMABandwidthMatrixBenchmarkScriptName,
		},
		FieldsFunc: memoryNUMABandwidthMatrixTableValues},
	MemoryLatencyMatrixBenchmarkName: {
		Name:          MemoryLatencyMatrixBenchmarkName,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.MemoryNUMALatencyMatrixBenchmarkScriptName,
		},
		FieldsFunc: memoryNUMALatencyMatrixTableValues},
	CacheIdleLatencyBenchmarkTableName: {
		Name:          CacheIdleLatencyBenchmarkTableName,
		MenuLabel:     CacheBenchmarksMenuLabel,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       false,
		ScriptNames: []string{
			script.L1IdleLatencyBenchmarkScriptName,
			script.L2IdleLatencyBenchmarkScriptName,
			script.L3IdleLatencyBenchmarkScriptName,
		},
		NoDataFound: "No cache idle latency benchmark data found. Please see the GitHub repository README for instructions on how to install Intel Memory Latency Checker (mlc).",
		FieldsFunc:  cacheIdleLatencyTableValues},
	CacheMaxBandwidthBenchmarkTableName: {
		Name:          CacheMaxBandwidthBenchmarkTableName,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       false,
		ScriptNames: []string{
			script.L1MaxBandwidthBenchmarkScriptName,
			script.L2MaxBandwidthBenchmarkScriptName,
			script.L3MaxBandwidthBenchmarkScriptName,
		},
		NoDataFound: "No cache maximum bandwidth benchmark data found. Please see the GitHub repository README for instructions on how to install Intel Memory Latency Checker (mlc).",
		FieldsFunc:  cacheMaxBandwidthTableValues},
	StorageBenchmarkTableName: {
		Name:      StorageBenchmarkTableName,
		MenuLabel: StorageBenchmarksMenuLabel,
		HasRows:   true,
		ScriptNames: []string{
			script.StorageBenchmarkScriptName,
		},
		FieldsFunc: storageBenchmarkTableValues},
}

func speedBenchmarkTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return []table.Field{
		{Name: "Ops/s", Values: []string{cpuSpeedFromOutput(outputs)}},
	}
}

func powerBenchmarkTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return []table.Field{
		{Name: "Maximum Power", Values: []string{extract.MaxTotalPackagePowerFromOutput(outputs[script.PowerBenchmarkScriptName].Stdout)}},
		{Name: "Minimum Power", Values: []string{extract.MinTotalPackagePowerFromOutput(outputs[script.IdlePowerBenchmarkScriptName].Stdout)}},
	}
}

func temperatureBenchmarkTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return []table.Field{
		{Name: "Maximum Temperature", Values: []string{extract.MaxPackageTemperatureFromOutput(outputs[script.PowerBenchmarkScriptName].Stdout)}},
	}
}

func frequencyBenchmarkTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	// get the sse, avx256, and avx512 frequencies from the avx-turbo output
	instructionFreqs, err := avxTurboFrequenciesFromOutput(outputs[script.FrequencyBenchmarkScriptName].Stdout)
	if err != nil {
		slog.Warn("unable to get avx turbo frequencies", slog.String("error", err.Error()))
		return []table.Field{}
	}
	// we're expecting scalar_iadd, avx256_fma, avx512_fma
	scalarIaddFreqs := instructionFreqs["scalar_iadd"]
	avx256FmaFreqs := instructionFreqs["avx256_fma"]
	avx512FmaFreqs := instructionFreqs["avx512_fma"]
	// stop if we don't have any scalar_iadd frequencies
	if len(scalarIaddFreqs) == 0 {
		slog.Warn("no scalar_iadd frequencies found")
		return []table.Field{}
	}
	// get the spec core frequencies from the spec output
	var specSSEFreqs []string
	frequencyBuckets, err := extract.GetSpecFrequencyBuckets(outputs)
	if err == nil && len(frequencyBuckets) >= 2 {
		// get the frequencies from the buckets
		specSSEFreqs, err = extract.ExpandTurboFrequencies(frequencyBuckets, "sse")
		if err != nil {
			slog.Error("unable to convert buckets to counts", slog.String("error", err.Error()))
			return []table.Field{}
		}
		// trim the spec frequencies to the length of the scalar_iadd frequencies
		// this can happen when the actual core count is less than the number of cores in the spec
		if len(scalarIaddFreqs) < len(specSSEFreqs) {
			specSSEFreqs = specSSEFreqs[:len(scalarIaddFreqs)]
		}
		// pad the spec frequencies with the last value if they are shorter than the scalar_iadd frequencies
		// this can happen when the first die has fewer cores than other dies
		if len(specSSEFreqs) < len(scalarIaddFreqs) {
			diff := len(scalarIaddFreqs) - len(specSSEFreqs)
			for range diff {
				specSSEFreqs = append(specSSEFreqs, specSSEFreqs[len(specSSEFreqs)-1])
			}
		}
	}
	// create the fields
	fields := []table.Field{
		{Name: "cores"},
	}
	coresIdx := 0 // always the first field
	var specSSEFieldIdx int
	var scalarIaddFieldIdx int
	var avx2FieldIdx int
	var avx512FieldIdx int
	if len(specSSEFreqs) > 0 {
		fields = append(fields, table.Field{Name: "SSE (expected)", Description: "The expected frequency, when running SSE instructions, for the given number of active cores."})
		specSSEFieldIdx = len(fields) - 1
	}
	if len(scalarIaddFreqs) > 0 {
		fields = append(fields, table.Field{Name: "SSE", Description: "The measured frequency, when running SSE instructions, for the given number of active cores."})
		scalarIaddFieldIdx = len(fields) - 1
	}
	if len(avx256FmaFreqs) > 0 {
		fields = append(fields, table.Field{Name: "AVX2", Description: "The measured frequency, when running AVX2 instructions, for the given number of active cores."})
		avx2FieldIdx = len(fields) - 1
	}
	if len(avx512FmaFreqs) > 0 {
		fields = append(fields, table.Field{Name: "AVX512", Description: "The measured frequency, when running AVX512 instructions, for the given number of active cores."})
		avx512FieldIdx = len(fields) - 1
	}
	// add the data to the fields
	for i := range scalarIaddFreqs { // scalarIaddFreqs is required
		fields[coresIdx].Values = append(fields[coresIdx].Values, fmt.Sprintf("%d", i+1))
		if specSSEFieldIdx > 0 {
			if len(specSSEFreqs) > i {
				fields[specSSEFieldIdx].Values = append(fields[specSSEFieldIdx].Values, specSSEFreqs[i])
			} else {
				fields[specSSEFieldIdx].Values = append(fields[specSSEFieldIdx].Values, "")
			}
		}
		if scalarIaddFieldIdx > 0 {
			if len(scalarIaddFreqs) > i {
				fields[scalarIaddFieldIdx].Values = append(fields[scalarIaddFieldIdx].Values, fmt.Sprintf("%.1f", scalarIaddFreqs[i]))
			} else {
				fields[scalarIaddFieldIdx].Values = append(fields[scalarIaddFieldIdx].Values, "")
			}
		}
		if avx2FieldIdx > 0 {
			if len(avx256FmaFreqs) > i {
				fields[avx2FieldIdx].Values = append(fields[avx2FieldIdx].Values, fmt.Sprintf("%.1f", avx256FmaFreqs[i]))
			} else {
				fields[avx2FieldIdx].Values = append(fields[avx2FieldIdx].Values, "")
			}
		}
		if avx512FieldIdx > 0 {
			if len(avx512FmaFreqs) > i {
				fields[avx512FieldIdx].Values = append(fields[avx512FieldIdx].Values, fmt.Sprintf("%.1f", avx512FmaFreqs[i]))
			} else {
				fields[avx512FieldIdx].Values = append(fields[avx512FieldIdx].Values, "")
			}
		}
	}
	return fields
}

// loadedLatencyTableValuesFromOutput parses MLC loaded-latency output (latency ns, bandwidth MB/s) into table fields.
func loadedLatencyTableValuesFromOutput(stdout string) []table.Field {
	fields := []table.Field{
		{Name: "Latency (ns)"},
		{Name: "Bandwidth (GB/s)"},
	}
	/* MLC Output:
	Inject	Latency	Bandwidth
	Delay	(ns)	MB/sec
	 00000	261.65	 225060.9
	 ...
	*/
	latencyBandwidthPairs := extract.ValsArrayFromRegexSubmatch(stdout, `\s*[0-9]*\s*([0-9]*\.[0-9]+)\s*([0-9]*\.[0-9]+)`)
	for _, latencyBandwidth := range latencyBandwidthPairs {
		latency := latencyBandwidth[0]
		bandwidth, err := strconv.ParseFloat(latencyBandwidth[1], 32)
		if err != nil {
			slog.Error("Unable to convert bandwidth to float", slog.String("value", latencyBandwidth[1]))
			continue
		}
		fields[0].Values = append([]string{latency}, fields[0].Values...)
		fields[1].Values = append([]string{fmt.Sprintf("%.1f", bandwidth/1000)}, fields[1].Values...)
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func memoryBenchmarkTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return loadedLatencyTableValuesFromOutput(outputs[script.MemoryLoadedLatencyBenchmarkScriptName].Stdout)
}

// numaBandwidthMatrixTableValuesFromOutput parses MLC bandwidth matrix output (node rows, MB/s values) into table fields.
// Handles missing nodes (e.g. disabled sockets): column count is the max across rows; short rows are padded with empty cells.
func numaBandwidthMatrixTableValuesFromOutput(stdout string) []table.Field {
	nodeBandwidthsPairs := extract.ValsArrayFromRegexSubmatch(stdout, `^\s+(\d+)\s+(\d.*)$`)
	if len(nodeBandwidthsPairs) == 0 {
		return []table.Field{}
	}
	numCols := 0
	for _, pair := range nodeBandwidthsPairs {
		bandwidths := strings.Split(strings.TrimSpace(pair[1]), "\t")
		if n := len(bandwidths); n > numCols {
			numCols = n
		}
	}
	if numCols == 0 {
		return []table.Field{}
	}
	fields := make([]table.Field, 1+numCols)
	fields[0] = table.Field{Name: "Node"}
	for c := 0; c < numCols; c++ {
		fields[1+c] = table.Field{Name: strconv.Itoa(c)}
	}
	for _, nodeBandwidthsPair := range nodeBandwidthsPairs {
		fields[0].Values = append(fields[0].Values, nodeBandwidthsPair[0])
		bandwidths := strings.Split(strings.TrimSpace(nodeBandwidthsPair[1]), "\t")
		for c := 0; c < numCols; c++ {
			var cell string
			if c < len(bandwidths) {
				bw := strings.TrimSpace(bandwidths[c])
				val, err := strconv.ParseFloat(bw, 64)
				if err != nil {
					cell = ""
				} else {
					cell = fmt.Sprintf("%.1f", val/1000)
				}
			}
			fields[1+c].Values = append(fields[1+c].Values, cell)
		}
	}
	return fields
}

func memoryNUMABandwidthMatrixTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return numaBandwidthMatrixTableValuesFromOutput(outputs[script.MemoryNUMABandwidthMatrixBenchmarkScriptName].Stdout)
}

// numaLatencyMatrixTableValuesFromOutput parses MLC latency matrix output (node rows, ns values) into table fields.
// Handles missing nodes (e.g. disabled sockets): column count is the max across rows; short rows are padded with empty cells.
func numaLatencyMatrixTableValuesFromOutput(stdout string) []table.Field {
	nodeLatenciesPairs := extract.ValsArrayFromRegexSubmatch(stdout, `^\s+(\d+)\s+(\d.*)$`)
	if len(nodeLatenciesPairs) == 0 {
		return []table.Field{}
	}
	numCols := 0
	for _, pair := range nodeLatenciesPairs {
		latencies := strings.Split(strings.TrimSpace(pair[1]), "\t")
		if n := len(latencies); n > numCols {
			numCols = n
		}
	}
	if numCols == 0 {
		return []table.Field{}
	}
	fields := make([]table.Field, 1+numCols)
	fields[0] = table.Field{Name: "Node"}
	for c := 0; c < numCols; c++ {
		fields[1+c] = table.Field{Name: strconv.Itoa(c)}
	}
	for _, nodeLatencyPairs := range nodeLatenciesPairs {
		fields[0].Values = append(fields[0].Values, nodeLatencyPairs[0])
		latencies := strings.Split(strings.TrimSpace(nodeLatencyPairs[1]), "\t")
		for c := 0; c < numCols; c++ {
			var cell string
			if c < len(latencies) {
				latency := strings.TrimSpace(latencies[c])
				val, err := strconv.ParseFloat(latency, 64)
				if err != nil {
					cell = ""
				} else {
					cell = fmt.Sprintf("%.1f", val)
				}
			}
			fields[1+c].Values = append(fields[1+c].Values, cell)
		}
	}
	return fields
}

func memoryNUMALatencyMatrixTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	return numaLatencyMatrixTableValuesFromOutput(outputs[script.MemoryNUMALatencyMatrixBenchmarkScriptName].Stdout)
}

func idleLatencyNsFromOutput(stdout string) string {
	s := extract.ValFromRegexSubmatch(stdout, `\(\s*([\d.]+)\s*ns\)`)
	if s == "" {
		s = extract.ValFromRegexSubmatch(stdout, `([\d.]+)\s*ns`)
	}
	return strings.TrimSpace(s)
}

// maxBandwidthMBFromOutput parses MLC loaded_latency output into a string.
// The last few lines of output look like this:

// Inject  Latency Bandwidth
// Delay   (ns)    MB/sec
// ==========================
//	00000    0.00  11721640.9

// There will be one row of data so we parse only the last line of output.
func maxBandwidthMBFromOutput(stdout string) string {
	latencyBandwidthPairs := extract.ValsArrayFromRegexSubmatch(stdout, `\s*[0-9]*\s*([0-9]*\.[0-9]+)\s*([0-9]*\.[0-9]+)`)
	if len(latencyBandwidthPairs) == 0 {
		return ""
	}
	latencyBandwidth := latencyBandwidthPairs[len(latencyBandwidthPairs)-1]
	bandwidth, err := strconv.ParseFloat(latencyBandwidth[1], 64)
	if err != nil {
		slog.Error("Unable to convert bandwidth to float", slog.String("value", latencyBandwidth[1]))
		return ""
	}
	return fmt.Sprintf("%.1f", bandwidth)
}

func cacheIdleLatencyTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	l1 := idleLatencyNsFromOutput(outputs[script.L1IdleLatencyBenchmarkScriptName].Stdout)
	l2 := idleLatencyNsFromOutput(outputs[script.L2IdleLatencyBenchmarkScriptName].Stdout)
	l3 := idleLatencyNsFromOutput(outputs[script.L3IdleLatencyBenchmarkScriptName].Stdout)
	if l1 == "" && l2 == "" && l3 == "" {
		return []table.Field{}
	}
	return []table.Field{
		{Name: "L1", Values: []string{l1}},
		{Name: "L2", Values: []string{l2}},
		{Name: "L3", Values: []string{l3}},
	}
}

func cacheMaxBandwidthTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	l1MB := maxBandwidthMBFromOutput(outputs[script.L1MaxBandwidthBenchmarkScriptName].Stdout)
	l2MB := maxBandwidthMBFromOutput(outputs[script.L2MaxBandwidthBenchmarkScriptName].Stdout)
	l3MB := maxBandwidthMBFromOutput(outputs[script.L3MaxBandwidthBenchmarkScriptName].Stdout)
	if l1MB == "" && l2MB == "" && l3MB == "" {
		return []table.Field{}
	}
	var l1, l2, l3 string
	if v, err := strconv.ParseFloat(l1MB, 64); err == nil {
		l1 = fmt.Sprintf("%.1f", v/1000)
	}
	if v, err := strconv.ParseFloat(l2MB, 64); err == nil {
		l2 = fmt.Sprintf("%.1f", v/1000)
	}
	if v, err := strconv.ParseFloat(l3MB, 64); err == nil {
		l3 = fmt.Sprintf("%.1f", v/1000)
	}
	return []table.Field{
		{Name: "L1", Values: []string{l1}},
		{Name: "L2", Values: []string{l2}},
		{Name: "L3", Values: []string{l3}},
	}
}

// formatOrEmpty formats a value and returns an empty string if the formatted value is "0".
func formatOrEmpty(format string, value any) string {
	s := fmt.Sprintf(format, value)
	if s == "0" {
		return ""
	}
	return s
}

func storageBenchmarkTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fioData, err := storagePerfFromOutput(outputs)
	if err != nil {
		slog.Warn("failed to get storage benchmark data", slog.String("error", err.Error()))
		return []table.Field{}
	}
	// Initialize the fields for metrics (column headers)
	fields := []table.Field{
		{Name: "Job"},
		{Name: "Read Latency (us)"},
		{Name: "Read IOPs"},
		{Name: "Read Bandwidth (MiB/s)"},
		{Name: "Write Latency (us)"},
		{Name: "Write IOPs"},
		{Name: "Write Bandwidth (MiB/s)"},
	}
	// For each FIO job, create a new row and populate its values
	for _, job := range fioData.Jobs {
		fields[0].Values = append(fields[0].Values, job.Jobname)
		fields[1].Values = append(fields[1].Values, formatOrEmpty("%.0f", job.Read.LatNs.Mean/1000))
		fields[2].Values = append(fields[2].Values, formatOrEmpty("%.0f", job.Read.IopsMean))
		fields[3].Values = append(fields[3].Values, formatOrEmpty("%d", job.Read.Bw/1024))
		fields[4].Values = append(fields[4].Values, formatOrEmpty("%.0f", job.Write.LatNs.Mean/1000))
		fields[5].Values = append(fields[5].Values, formatOrEmpty("%.0f", job.Write.IopsMean))
		fields[6].Values = append(fields[6].Values, formatOrEmpty("%d", job.Write.Bw/1024))
	}
	return fields
}
