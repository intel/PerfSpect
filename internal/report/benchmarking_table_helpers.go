package report

// Copyright (C) 2021-2024 Intel Corporation
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
	for _, line := range strings.Split(strings.TrimSpace(outputs[script.CpuSpeedScriptName].Stdout), "\n") {
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
	for _, line := range strings.Split(strings.TrimSpace(outputs[script.StoragePerfScriptName].Stdout), "\n") {
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

func ParseTurbostatOutput(output string) (singleCoreTurbo, allCoreTurbo, turboPower, turboTemperature string) {
	var allTurbos []string
	var allTDPs []string
	var allTemps []string
	var turbos []string
	var tdps []string
	var temps []string
	var headers []string
	idxTurbo := -1
	idxTdp := -1
	idxTemp := -1
	re := regexp.MustCompile(`\s+`) // whitespace
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, "stress-ng") {
			if strings.Contains(line, "completed") {
				if idxTurbo >= 0 && len(allTurbos) >= 2 {
					turbos = append(turbos, allTurbos[len(allTurbos)-2])
					allTurbos = nil
				}
				if idxTdp >= 0 && len(allTDPs) >= 2 {
					tdps = append(tdps, allTDPs[len(allTDPs)-2])
					allTDPs = nil
				}
				if idxTemp >= 0 && len(allTemps) >= 2 {
					temps = append(temps, allTemps[len(allTemps)-2])
					allTemps = nil
				}
			}
			continue
		}
		if strings.Contains(line, "Package") || strings.Contains(line, "CPU") || strings.Contains(line, "Core") || strings.Contains(line, "Node") {
			headers = re.Split(line, -1) // split by whitespace
			for i, h := range headers {
				if h == "Bzy_MHz" {
					idxTurbo = i
				} else if h == "PkgWatt" {
					idxTdp = i
				} else if h == "PkgTmp" {
					idxTemp = i
				}
			}
			continue
		}
		tokens := re.Split(line, -1)
		if idxTurbo >= 0 {
			allTurbos = append(allTurbos, tokens[idxTurbo])
		}
		if idxTdp >= 0 {
			allTDPs = append(allTDPs, tokens[idxTdp])
		}
		if idxTemp >= 0 {
			allTemps = append(allTemps, tokens[idxTemp])
		}
	}
	if len(turbos) == 2 {
		singleCoreTurbo = turbos[0] + " MHz"
		allCoreTurbo = turbos[1] + " MHz"
	}
	if len(tdps) == 2 {
		turboPower = tdps[1] + " Watts"
	}
	if len(temps) == 2 {
		turboTemperature = temps[1] + " C"
	}
	return
}

func maxPowerFromOutput(outputs map[string]script.ScriptOutput) string {
	_, _, power, _ := ParseTurbostatOutput(outputs[script.TurboFrequencyPowerAndTemperatureScriptName].Stdout)
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
	_, _, _, temperature := ParseTurbostatOutput(outputs[script.TurboFrequencyPowerAndTemperatureScriptName].Stdout)
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
	for _, line := range strings.Split(output, "\n") {
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
