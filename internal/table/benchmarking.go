package table

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"perfspect/internal/script"
	"perfspect/internal/util"
	"strconv"
	"strings"
)

// fioOutput is the top-level struct for the FIO JSON report.
// ref: https://fio.readthedocs.io/en/latest/fio_doc.html#json-output
type fioOutput struct {
	FioVersion  string   `json:"fio version"`
	Timestamp   int64    `json:"timestamp"`
	TimestampMs int64    `json:"timestamp_ms"`
	Time        string   `json:"time"`
	Jobs        []fioJob `json:"jobs"`
}

// Job represents a single job's results within the FIO report.
type fioJob struct {
	Jobname           string             `json:"jobname"`
	Groupid           int                `json:"groupid"`
	JobStart          int64              `json:"job_start"`
	Error             int                `json:"error"`
	Eta               int                `json:"eta"`
	Elapsed           int                `json:"elapsed"`
	Read              fioIOStats         `json:"read"`
	Write             fioIOStats         `json:"write"`
	Trim              fioIOStats         `json:"trim"`
	JobRuntime        int                `json:"job_runtime"`
	UsrCPU            float64            `json:"usr_cpu"`
	SysCPU            float64            `json:"sys_cpu"`
	Ctx               int                `json:"ctx"`
	Majf              int                `json:"majf"`
	Minf              int                `json:"minf"`
	IodepthLevel      map[string]float64 `json:"iodepth_level"`
	IodepthSubmit     map[string]float64 `json:"iodepth_submit"`
	IodepthComplete   map[string]float64 `json:"iodepth_complete"`
	LatencyNs         map[string]float64 `json:"latency_ns"`
	LatencyUs         map[string]float64 `json:"latency_us"`
	LatencyMs         map[string]float64 `json:"latency_ms"`
	LatencyDepth      int                `json:"latency_depth"`
	LatencyTarget     int                `json:"latency_target"`
	LatencyPercentile float64            `json:"latency_percentile"`
	LatencyWindow     int                `json:"latency_window"`
}

// IOStats holds the detailed I/O statistics for read, write, or trim operations.
type fioIOStats struct {
	IoBytes     int64                      `json:"io_bytes"`
	IoKbytes    int64                      `json:"io_kbytes"`
	BwBytes     int64                      `json:"bw_bytes"`
	Bw          int64                      `json:"bw"`
	Iops        float64                    `json:"iops"`
	Runtime     int                        `json:"runtime"`
	TotalIos    int                        `json:"total_ios"`
	ShortIos    int                        `json:"short_ios"`
	DropIos     int                        `json:"drop_ios"`
	SlatNs      fioLatencyStats            `json:"slat_ns"`
	ClatNs      fioLatencyStatsPercentiles `json:"clat_ns"`
	LatNs       fioLatencyStats            `json:"lat_ns"`
	BwMin       int                        `json:"bw_min"`
	BwMax       int                        `json:"bw_max"`
	BwAgg       float64                    `json:"bw_agg"`
	BwMean      float64                    `json:"bw_mean"`
	BwDev       float64                    `json:"bw_dev"`
	BwSamples   int                        `json:"bw_samples"`
	IopsMin     int                        `json:"iops_min"`
	IopsMax     int                        `json:"iops_max"`
	IopsMean    float64                    `json:"iops_mean"`
	IopsStddev  float64                    `json:"iops_stddev"`
	IopsSamples int                        `json:"iops_samples"`
}

// fioLatencyStats holds basic latency metrics.
type fioLatencyStats struct {
	Min    int64   `json:"min"`
	Max    int64   `json:"max"`
	Mean   float64 `json:"mean"`
	Stddev float64 `json:"stddev"`
	N      int     `json:"N"`
}

// LatencyStatsPercentiles holds latency metrics including percentiles.
type fioLatencyStatsPercentiles struct {
	Min        int64            `json:"min"`
	Max        int64            `json:"max"`
	Mean       float64          `json:"mean"`
	Stddev     float64          `json:"stddev"`
	N          int              `json:"N"`
	Percentile map[string]int64 `json:"percentile"`
}

func cpuSpeedFromOutput(outputs map[string]script.ScriptOutput) string {
	var vals []float64
	for line := range strings.SplitSeq(strings.TrimSpace(outputs[script.SpeedBenchmarkScriptName].Stdout), "\n") {
		tokens := strings.Split(line, " ")
		if len(tokens) != 2 {
			slog.Error("unexpected stress-ng output format", slog.String("line", line))
			return ""
		}
		fv, err := strconv.ParseFloat(tokens[1], 64)
		if err != nil {
			slog.Error("unexpected value in 2nd token (%s), expected float in line: %s", slog.String("token", tokens[1]), slog.String("line", line))
			return ""
		}
		vals = append(vals, fv)
	}
	if len(vals) == 0 {
		slog.Warn("no values detected in stress-ng output")
		return ""
	}
	return fmt.Sprintf("%.0f", util.GeoMean(vals))
}

func storagePerfFromOutput(outputs map[string]script.ScriptOutput) (fioOutput, error) {
	output := outputs[script.StorageBenchmarkScriptName].Stdout

	if strings.Contains(output, "ERROR:") {
		return fioOutput{}, fmt.Errorf("failed to run storage benchmark: %s", output)
	}
	i := strings.Index(output, "{\n  \"fio version\"")
	if i >= 0 {
		output = output[i:]
	} else {
		outputLen := min(len(output), 100)
		slog.Info("fio output snip", "output", output[:outputLen], "stderr", outputs[script.StorageBenchmarkScriptName].Stderr)
		return fioOutput{}, fmt.Errorf("unable to find fio output")
	}

	slog.Debug("parsing storage benchmark output")
	var fioData fioOutput
	if err := json.Unmarshal([]byte(output), &fioData); err != nil {
		return fioOutput{}, fmt.Errorf("error unmarshalling JSON: %w", err)
	}
	if len(fioData.Jobs) == 0 {
		return fioOutput{}, fmt.Errorf("no jobs found in storage benchmark output")
	}

	return fioData, nil
}

// avxTurboFrequenciesFromOutput parses the output of avx-turbo and returns the turbo frequencies as a map of instruction type to frequencies
// Sample avx-turbo output
// ...
// Cores | ID          | Description            | OVRLP3 | Mops | A/M-ratio | A/M-MHz | M/tsc-ratio
// 1     | scalar_iadd | Scalar integer adds    |  1.000 | 3901 |      1.95 |    3900 |        1.00
// 1     | avx256_fma  | 256-bit serial DP FMAs |  1.000 |  974 |      1.95 |    3900 |        1.00
// 1     | avx512_fma  | 512-bit serial DP FMAs |  1.000 |  974 |      1.95 |    3900 |        1.00
// Cores | ID          | Description            | OVRLP3 |       Mops |    A/M-ratio |    A/M-MHz | M/tsc-ratio
// 2     | scalar_iadd | Scalar integer adds    |  1.000 | 3901, 3901 |  1.95,  1.95 | 3900, 3900 |  1.00, 1.00
// 2     | avx256_fma  | 256-bit serial DP FMAs |  1.000 |  974,  974 |  1.95,  1.95 | 3900, 3900 |  1.00, 1.00
// 2     | avx512_fma  | 512-bit serial DP FMAs |  1.000 |  974,  974 |  1.95,  1.95 | 3900, 3900 |  1.00, 1.00
// Cores | ID          | Description            | OVRLP3 |             Mops |           A/M-ratio |          A/M-MHz |      M/tsc-ratio
// 3     | scalar_iadd | Scalar integer adds    |  1.000 | 3900, 3901, 3901 |  1.95,  1.95,  1.95 | 3900, 3900, 3900 | 1.00, 1.00, 1.00
// 3     | avx256_fma  | 256-bit serial DP FMAs |  1.000 |  974,  975,  975 |  1.95,  1.95,  1.95 | 3900, 3900, 3900 | 1.00, 1.00, 1.00
// 3     | avx512_fma  | 512-bit serial DP FMAs |  1.000 |  974,  975,  974 |  1.95,  1.95,  1.95 | 3900, 3900, 3900 | 1.00, 1.00, 1.00
// ...
func avxTurboFrequenciesFromOutput(output string) (instructionFreqs map[string][]float64, err error) {
	instructionFreqs = make(map[string][]float64)
	started := false
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "Cores | ID") {
			started = true
			continue
		}
		if !started {
			continue
		}
		if line == "" {
			started = false
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) < 7 {
			err = fmt.Errorf("avx-turbo unable to measure frequencies")
			return
		}
		freqs := strings.Split(fields[6], ",")
		var sumFreqs float64
		for _, freq := range freqs {
			var f float64
			f, err = strconv.ParseFloat(strings.TrimSpace(freq), 64)
			if err != nil {
				return
			}
			sumFreqs += f
		}
		avgFreq := sumFreqs / float64(len(freqs))
		instructionType := strings.TrimSpace(fields[1])
		if _, ok := instructionFreqs[instructionType]; !ok {
			instructionFreqs[instructionType] = []float64{}
		}
		instructionFreqs[instructionType] = append(instructionFreqs[instructionType], avgFreq/1000.0)
	}
	return
}
