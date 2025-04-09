package report

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"perfspect/internal/script"
	"perfspect/internal/util"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

func cpuSpeedFromOutput(outputs map[string]script.ScriptOutput) string {
	var vals []float64
	for line := range strings.SplitSeq(strings.TrimSpace(outputs[script.CpuSpeedScriptName].Stdout), "\n") {
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
	for line := range strings.SplitSeq(strings.TrimSpace(outputs[script.StoragePerfScriptName].Stdout), "\n") {
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

// ParseTurbostatOutput parses the output of turbostat and returns the turbo frequencies, power and temperature
// turbostat output format:
// PkgTmp  PkgWatt
// 55      537.14
// 51      266.41
// 55      267.45
// 54      445.73
// 51      252.80
// 54      252.17
// 55      569.81
// 52      248.99
// 55      249.12
// 57      498.30
// 53      249.78
// 57      249.63
// It is possible that
// -- the output is empty
// -- the output includes only one column (PkgTmp or PkgWatt)
// -- the output includes both columns
// We capture the max of the power and temperature values
func ParseTurbostatOutput(output string) (turboPower, turboTemperature string) {
	// confirm output is in expected format
	var fieldNames []string
	var temps []float64
	var watts []float64
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if i == 0 {
			fieldNames = strings.Fields(line)
			if len(fieldNames) < 1 {
				slog.Warn("unexpected turbostat output format", slog.String("line", line))
				return
			}
			continue
		}
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < len(fieldNames) {
			slog.Warn("unexpected turbostat output format", slog.String("line", line))
			return
		}
		if len(fields) == 2 {
			tmp, err := strconv.ParseFloat(fields[0], 64)
			if err != nil {
				slog.Warn("unexpected turbostat output format", slog.String("line", line))
				return
			}
			temps = append(temps, tmp)
			watt, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				slog.Warn("unexpected turbostat output format", slog.String("line", line))
				return
			}
			watts = append(watts, watt)
		}
		if len(fields) == 1 {
			if strings.Contains(fieldNames[0], "PkgTmp") {
				tmp, err := strconv.ParseFloat(fields[0], 64)
				if err != nil {
					slog.Warn("unexpected turbostat output format", slog.String("line", line))
					return
				}
				temps = append(temps, tmp)
			} else {
				watt, err := strconv.ParseFloat(fields[0], 64)
				if err != nil {
					slog.Warn("unexpected turbostat output format", slog.String("line", line))
					return
				}
				watts = append(watts, watt)
			}
		}
	}
	if len(temps) > 1 {
		maxTemp := slices.Max(temps[1:]) // max temperature, skip first entry as it can be misleading
		if maxTemp > 0 {
			turboTemperature = fmt.Sprintf("%.0f C", maxTemp)
		}
	}
	if len(watts) > 1 {
		maxWatt := slices.Max(watts[1:]) // max power, skip first entry as it can be misleading
		if maxWatt > 0 {
			turboPower = fmt.Sprintf("%.2f Watts", maxWatt)
		}
	}
	return
}

func maxPowerFromOutput(outputs map[string]script.ScriptOutput) string {
	power, _ := ParseTurbostatOutput(outputs[script.MaxPowerAndTemperatureScriptName].Stdout)
	return power
}

func minPowerFromOutput(outputs map[string]script.ScriptOutput) string {
	watts := strings.TrimSpace(outputs[script.IdlePowerScriptName].Stdout)
	if watts == "" || watts == "0.00" {
		return ""
	}
	return watts + " Watts"
}

func maxTemperatureFromOutput(outputs map[string]script.ScriptOutput) string {
	_, temperature := ParseTurbostatOutput(outputs[script.MaxPowerAndTemperatureScriptName].Stdout)
	return temperature
}

// Sample avx-turbo output
// ...
// Will test up to 64 CPUs
// Cores | ID          | Description            | OVRLP3 | Mops | A/M-ratio | A/M-MHz | M/tsc-ratio
// 1     | scalar_iadd | Scalar integer adds    |  1.000 | 3901 |      1.95 |    3900 |        1.00
// 1     | avx128_fma  | 128-bit serial DP FMAs |  1.000 |  974 |      1.95 |    3900 |        1.00
// 1     | avx256_fma  | 256-bit serial DP FMAs |  1.000 |  974 |      1.95 |    3900 |        1.00
// 1     | avx512_fma  | 512-bit serial DP FMAs |  1.000 |  974 |      1.95 |    3900 |        1.00

// Cores | ID          | Description            | OVRLP3 |       Mops |    A/M-ratio |    A/M-MHz | M/tsc-ratio
// 2     | scalar_iadd | Scalar integer adds    |  1.000 | 3901, 3901 |  1.95,  1.95 | 3900, 3900 |  1.00, 1.00
// 2     | avx128_fma  | 128-bit serial DP FMAs |  1.000 |  974,  974 |  1.95,  1.95 | 3900, 3900 |  1.00, 1.00
// 2     | avx256_fma  | 256-bit serial DP FMAs |  1.000 |  974,  974 |  1.95,  1.95 | 3900, 3900 |  1.00, 1.00
// 2     | avx512_fma  | 512-bit serial DP FMAs |  1.000 |  974,  974 |  1.95,  1.95 | 3900, 3900 |  1.00, 1.00

// Cores | ID          | Description            | OVRLP3 |             Mops |           A/M-ratio |          A/M-MHz |      M/tsc-ratio
// 3     | scalar_iadd | Scalar integer adds    |  1.000 | 3900, 3901, 3901 |  1.95,  1.95,  1.95 | 3900, 3900, 3900 | 1.00, 1.00, 1.00
// 3     | avx128_fma  | 128-bit serial DP FMAs |  1.000 |  974,  975,  975 |  1.95,  1.95,  1.95 | 3900, 3900, 3900 | 1.00, 1.00, 1.00
// 3     | avx256_fma  | 256-bit serial DP FMAs |  1.000 |  974,  975,  975 |  1.95,  1.95,  1.95 | 3900, 3900, 3900 | 1.00, 1.00, 1.00
// 3     | avx512_fma  | 512-bit serial DP FMAs |  1.000 |  974,  975,  974 |  1.95,  1.95,  1.95 | 3900, 3900, 3900 | 1.00, 1.00, 1.00
// ...
func avxTurboFrequenciesFromOutput(output string) (nonavxFreqs, avx128Freqs, avx256Freqs, avx512Freqs []float64, err error) {
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
		if strings.Contains(fields[1], "scalar_iadd") {
			nonavxFreqs = append(nonavxFreqs, avgFreq/1000.0)
		} else if strings.Contains(fields[1], "avx128_fma") {
			avx128Freqs = append(avx128Freqs, avgFreq/1000.0)
		} else if strings.Contains(fields[1], "avx256_fma") {
			avx256Freqs = append(avx256Freqs, avgFreq/1000.0)
		} else if strings.Contains(fields[1], "avx512_fma") {
			avx512Freqs = append(avx512Freqs, avgFreq/1000.0)
		} else {
			err = fmt.Errorf("unexpected data from avx-turbo, unknown instruction type")
			return
		}
	}
	return
}
