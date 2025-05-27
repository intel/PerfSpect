package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"perfspect/internal/script"
	"perfspect/internal/util"
	"regexp"
	"strconv"
	"strings"
)

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

func storagePerfFromOutput(outputs map[string]script.ScriptOutput) (readBW, writeBW string) {
	// fio output format:
	// READ: bw=140MiB/s (146MB/s), 140MiB/s-140MiB/s (146MB/s-146MB/s), io=16.4GiB (17.6GB), run=120004-120004msec
	// WRITE: bw=139MiB/s (146MB/s), 139MiB/s-139MiB/s (146MB/s-146MB/s), io=16.3GiB (17.5GB), run=120004-120004msec
	re := regexp.MustCompile(` bw=(\d+[.]?[\d]*\w+\/s)`)
	for line := range strings.SplitSeq(strings.TrimSpace(outputs[script.StorageBenchmarkScriptName].Stdout), "\n") {
		if strings.Contains(line, "READ: bw=") {
			matches := re.FindStringSubmatch(line)
			if len(matches) != 0 {
				readBW = matches[1]
			}
		} else if strings.Contains(line, "WRITE: bw=") {
			matches := re.FindStringSubmatch(line)
			if len(matches) != 0 {
				writeBW = matches[1]
			}
		} else if strings.Contains(line, "ERROR: ") {
			slog.Error("failed to run storage benchmark", slog.String("line", line))
		}
	}
	return
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
