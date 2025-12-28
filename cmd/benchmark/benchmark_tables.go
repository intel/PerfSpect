package benchmark

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

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
	SpeedBenchmarkTableName       = "Speed Benchmark"
	PowerBenchmarkTableName       = "Power Benchmark"
	TemperatureBenchmarkTableName = "Temperature Benchmark"
	FrequencyBenchmarkTableName   = "Frequency Benchmark"
	MemoryBenchmarkTableName      = "Memory Benchmark"
	NUMABenchmarkTableName        = "NUMA Benchmark"
	StorageBenchmarkTableName     = "Storage Benchmark"
)

var tableDefinitions = map[string]table.TableDefinition{
	SpeedBenchmarkTableName: {
		Name:      SpeedBenchmarkTableName,
		MenuLabel: SpeedBenchmarkTableName,
		HasRows:   false,
		ScriptNames: []string{
			script.SpeedBenchmarkScriptName,
		},
		FieldsFunc: speedBenchmarkTableValues},
	PowerBenchmarkTableName: {
		Name:          PowerBenchmarkTableName,
		MenuLabel:     PowerBenchmarkTableName,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       false,
		ScriptNames: []string{
			script.IdlePowerBenchmarkScriptName,
			script.PowerBenchmarkScriptName,
		},
		FieldsFunc: powerBenchmarkTableValues},
	TemperatureBenchmarkTableName: {
		Name:          TemperatureBenchmarkTableName,
		MenuLabel:     TemperatureBenchmarkTableName,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       false,
		ScriptNames: []string{
			script.PowerBenchmarkScriptName,
		},
		FieldsFunc: temperatureBenchmarkTableValues},
	FrequencyBenchmarkTableName: {
		Name:          FrequencyBenchmarkTableName,
		MenuLabel:     FrequencyBenchmarkTableName,
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
	MemoryBenchmarkTableName: {
		Name:          MemoryBenchmarkTableName,
		MenuLabel:     MemoryBenchmarkTableName,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.MemoryBenchmarkScriptName,
		},
		NoDataFound: "No memory benchmark data found. Please see the GitHub repository README for instructions on how to install Intel Memory Latency Checker (mlc).",
		FieldsFunc:  memoryBenchmarkTableValues},
	NUMABenchmarkTableName: {
		Name:          NUMABenchmarkTableName,
		MenuLabel:     NUMABenchmarkTableName,
		Architectures: []string{cpus.X86Architecture},
		HasRows:       true,
		ScriptNames: []string{
			script.NumaBenchmarkScriptName,
		},
		NoDataFound: "No NUMA benchmark data found. Please see the GitHub repository README for instructions on how to install Intel Memory Latency Checker (mlc).",
		FieldsFunc:  numaBenchmarkTableValues},
	StorageBenchmarkTableName: {
		Name:      StorageBenchmarkTableName,
		MenuLabel: StorageBenchmarkTableName,
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

func memoryBenchmarkTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Latency (ns)"},
		{Name: "Bandwidth (GB/s)"},
	}
	/* MLC Output:
	Inject	Latency	Bandwidth
	Delay	(ns)	MB/sec
	==========================
	 00000	261.65	 225060.9
	 00002	261.63	 225040.5
	 00008	261.54	 225073.3
	 ...
	*/
	latencyBandwidthPairs := extract.ValsArrayFromRegexSubmatch(outputs[script.MemoryBenchmarkScriptName].Stdout, `\s*[0-9]*\s*([0-9]*\.[0-9]+)\s*([0-9]*\.[0-9]+)`)
	for _, latencyBandwidth := range latencyBandwidthPairs {
		latency := latencyBandwidth[0]
		bandwidth, err := strconv.ParseFloat(latencyBandwidth[1], 32)
		if err != nil {
			slog.Error(fmt.Sprintf("Unable to convert bandwidth to float: %s", latencyBandwidth[1]))
			continue
		}
		// insert into beginning of list
		fields[0].Values = append([]string{latency}, fields[0].Values...)
		fields[1].Values = append([]string{fmt.Sprintf("%.1f", bandwidth/1000)}, fields[1].Values...)
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
}

func numaBenchmarkTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Node"},
	}
	/* MLC Output:
		Numa node
	Numa node	     0	     1
	       0	175610.3	 55579.7
	       1	 55575.2	175656.7
	*/
	nodeBandwidthsPairs := extract.ValsArrayFromRegexSubmatch(outputs[script.NumaBenchmarkScriptName].Stdout, `^\s+(\d)\s+(\d.*)$`)
	// add 1 field per numa node
	for _, nodeBandwidthsPair := range nodeBandwidthsPairs {
		fields = append(fields, table.Field{Name: nodeBandwidthsPair[0]})
	}
	// add rows
	for _, nodeBandwidthsPair := range nodeBandwidthsPairs {
		fields[0].Values = append(fields[0].Values, nodeBandwidthsPair[0])
		bandwidths := strings.Split(strings.TrimSpace(nodeBandwidthsPair[1]), "\t")
		if len(bandwidths) != len(nodeBandwidthsPairs) {
			slog.Warn(fmt.Sprintf("Mismatched number of bandwidths for numa node %s, %s", nodeBandwidthsPair[0], nodeBandwidthsPair[1]))
			return []table.Field{}
		}
		for i, bw := range bandwidths {
			bw = strings.TrimSpace(bw)
			val, err := strconv.ParseFloat(bw, 64)
			if err != nil {
				slog.Error(fmt.Sprintf("Unable to convert bandwidth to float: %s", bw))
				continue
			}
			fields[i+1].Values = append(fields[i+1].Values, fmt.Sprintf("%.1f", val/1000))
		}
	}
	if len(fields[0].Values) == 0 {
		return []table.Field{}
	}
	return fields
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
