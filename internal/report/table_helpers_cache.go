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

// GetL3LscpuMB returns the L3 cache size (per socket) in MB from lscpu output.
func GetL3LscpuMB(outputs map[string]script.ScriptOutput) (float64, error) {
	l3MB, err := getCacheMBLscpu(outputs[script.LscpuScriptName].Stdout, `^L3 cache.*:\s*(.+?)$`)
	if err != nil {
		return 0, fmt.Errorf("failed to get L3 cache size from lscpu: %v", err)
	}
	return l3MB, nil
}

// GetL3MSRMB returns the L3 cache size in MB from MSR.
func GetL3MSRMB(outputs map[string]script.ScriptOutput) (float64, error) {
	uarch := UarchFromOutput(outputs)
	cpu, err := GetCPUByMicroArchitecture(uarch)
	if err != nil {
		return 0, err
	}
	if cpu.CacheWayCount == 0 {
		err = fmt.Errorf("L3 cache way count is zero")
		return 0, err
	}
	// we get the unmodified/maximum possible L3 size from lscpu
	l3MaximumMB, err := GetL3LscpuMB(outputs)
	if err != nil {
		return 0, err
	}
	// for every bit set in l3WayEnabled, a way is enabled
	l3WayEnabledMSRVal := strings.TrimSpace(outputs[script.L3CacheWayEnabledName].Stdout)
	if l3WayEnabledMSRVal == "" {
		err = fmt.Errorf("L3 cache way enabled MSR value is empty")
		return 0, err
	}
	l3WayEnabled, err := strconv.ParseUint(l3WayEnabledMSRVal, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse L3 way enabled MSR value: %s, %v", l3WayEnabledMSRVal, err)
	}
	if l3WayEnabled == 0 {
		err = fmt.Errorf("zero cache ways enabled: %s", l3WayEnabledMSRVal)
		return 0, err
	}
	numCacheWaysEnabled := util.NumUint64Bits(l3WayEnabled)
	if numCacheWaysEnabled == 0 {
		err = fmt.Errorf("zero cache way bits set: %s", l3WayEnabledMSRVal)
		return 0, err
	}

	cpul3SizeGB := l3MaximumMB / 1024
	GBperWay := cpul3SizeGB / float64(cpu.CacheWayCount)

	currentL3SizeGB := float64(numCacheWaysEnabled) * GBperWay
	return currentL3SizeGB * 1024, nil
}

// l3FromOutput attempts to retrieve the L3 cache size in megabytes from the provided
// script outputs. It first tries to obtain the value using GetL3MSRMB. If that fails,
// it falls back to using GetL3LscpuMB. If both methods fail, it logs the errors and
// returns an empty string. On success, it returns the formatted cache size as a string.
func l3FromOutput(outputs map[string]script.ScriptOutput) string {
	l3MB, err := GetL3MSRMB(outputs)
	if err != nil {
		slog.Info("Could not get L3 size from MSR, falling back to lscpu", slog.String("error", err.Error()))
		l3MB, err = GetL3LscpuMB(outputs)
		if err != nil {
			slog.Error("Could not get L3 size from lscpu", slog.String("error", err.Error()))
			return ""
		}
	}
	return formatCacheSizeMB(l3MB)
}

// l3PerCoreFromOutput calculates the amount of L3 cache (in MiB) available per core
// based on the provided script outputs. It first checks if the host is virtualized,
// in which case it returns an empty string since the calculation is not applicable.
// It parses the number of cores per socket and the number of sockets from the lscpu
// output. It attempts to retrieve the total L3 cache size using MSR data, falling
// back to parsing lscpu output if necessary. The result is formatted as a string
// with up to three decimal places, followed by " MiB". If any required data cannot
// be parsed, it logs an error and returns an empty string.
func l3PerCoreFromOutput(outputs map[string]script.ScriptOutput) string {
	virtualization := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Virtualization.*:\s*(.+?)$`)
	if virtualization == "full" {
		slog.Info("Can't calculate L3 per Core on virtualized host.")
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
	var l3PerCoreMB float64
	if l3MB, err := GetL3MSRMB(outputs); err == nil {
		l3PerCoreMB = l3MB / float64(coresPerSocket)
	} else {
		slog.Info("Could not get L3 size from MSR, falling back to lscpu", slog.String("error", err.Error()))
		l3MB, err := GetL3LscpuMB(outputs)
		if err != nil {
			slog.Error("Could not get L3 size from lscpu", slog.String("error", err.Error()))
			return ""
		}
		l3PerCoreMB = l3MB / float64(coresPerSocket)
	}
	val := strconv.FormatFloat(l3PerCoreMB, 'f', 3, 64)
	val = strings.TrimRight(val, "0") // trim trailing zeros
	val = strings.TrimRight(val, ".") // trim decimal point if trailing
	val += " MiB"
	return val
}

// l2FromOutput extracts the L2 cache size from the provided script outputs map,
// formats it as a human-readable string, and returns it. If extraction or formatting
// fails, it logs an error and returns an empty string.
func l2FromOutput(outputs map[string]script.ScriptOutput) string {
	l2MB, err := getCacheMBLscpu(outputs[script.LscpuScriptName].Stdout, `^L2 cache:\s*(.+)$`)
	if err != nil {
		slog.Error("Failed to get L2 cache size from lscpu", slog.String("error", err.Error()))
		return ""
	}
	return formatCacheSizeMB(l2MB)
}

// l1dFromOutput extracts the L1 data cache size from the provided script outputs map,
// formats it as a human-readable string, and returns it. If extraction or formatting fails,
// it logs an error and returns an empty string.
func l1dFromOutput(outputs map[string]script.ScriptOutput) string {
	l1dMB, err := getCacheMBLscpu(outputs[script.LscpuScriptName].Stdout, `^L1d cache:\s*(.+)$`)
	if err != nil {
		slog.Error("Failed to get L1d cache size from lscpu", slog.String("error", err.Error()))
		return ""
	}
	return formatCacheSizeMB(l1dMB)
}

// l1iFromOutput extracts the L1 instruction cache size from the provided script outputs map,
// formats it as a human-readable string, and returns it. If extraction or formatting fails,
// it logs an error and returns an empty string.
func l1iFromOutput(outputs map[string]script.ScriptOutput) string {
	l1iMB, err := getCacheMBLscpu(outputs[script.LscpuScriptName].Stdout, `^L1i cache:\s*(.+)$`)
	if err != nil {
		slog.Error("Failed to get L1i cache size from lscpu", slog.String("error", err.Error()))
		return ""
	}
	return formatCacheSizeMB(l1iMB)
}

// getCacheLscpuParts parses an lscpu cache string and extracts the cache size, units, and number of instances.
// The input string is expected to be in the format "<size> <units> (<instances> instance[s])" or "<size> <units>".
// Returns the parsed size as float64, units as string, instances as int, and an error if parsing fails.
func getCacheLscpuParts(lscpuCache string) (size float64, units string, instances int, err error) {
	re := regexp.MustCompile(`(\d+\.?\d*)\s*(\w+)\s+\((.*) instance[s]*\)`) // match known formats
	match := re.FindStringSubmatch(lscpuCache)
	if match != nil {
		instances, err = strconv.Atoi(match[3])
		if err != nil {
			err = fmt.Errorf("failed to parse cache instances from lscpu: %s, %v", lscpuCache, err)
			return
		}
	} else {
		// try regex without the instance count
		re = regexp.MustCompile(`(\d+\.?\d*)\s*(\w+)`)
		match = re.FindStringSubmatch(lscpuCache)
		if match == nil {
			err = fmt.Errorf("unknown cache format in lscpu: %s", lscpuCache)
			return
		}
		instances = 1
	}
	size, err = strconv.ParseFloat(match[1], 64)
	if err != nil {
		err = fmt.Errorf("failed to parse cache size from lscpu: %s, %v", lscpuCache, err)
		return
	}
	units = match[2]
	return
}

// formatCacheSizeMB formats a floating-point cache size value (in MiB) as a string
// with the "MiB" unit suffix. The size is formatted using decimal notation
// with no fixed precision.
func formatCacheSizeMB(size float64) string {
	return fmt.Sprintf("%s MiB", strconv.FormatFloat(size, 'f', -1, 64))
}

// getCacheMBLscpu parses the output of the `lscpu` command to extract the cache size (in MB) per socket.
// It takes the lscpu output as a string and a regular expression to match the desired cache line.
// The function returns the cache size in megabytes per socket, or an error if parsing fails.
func getCacheMBLscpu(lscpuOutput string, cacheRegex string) (float64, error) {
	sockets := valFromRegexSubmatch(lscpuOutput, `^Socket\(s\):\s*(.+)$`)
	if sockets == "" {
		return 0, fmt.Errorf("failed to parse sockets from lscpu output")
	}
	numSockets, err := strconv.Atoi(sockets)
	if err != nil || numSockets == 0 {
		return 0, fmt.Errorf("failed to parse sockets from lscpu output: %s, %v", sockets, err)
	}
	cacheSize := valFromRegexSubmatch(lscpuOutput, cacheRegex)
	if cacheSize == "" {
		return 0, fmt.Errorf("cache size not found in lscpu output")
	}
	size, units, _, err := getCacheLscpuParts(cacheSize)
	if err != nil {
		return 0, fmt.Errorf("failed to parse cache size from lscpu: %s, %v", cacheSize, err)
	}
	switch strings.ToLower(units[:1]) {
	case "g":
		return size * 1024 / float64(numSockets), nil
	case "m":
		return size / float64(numSockets), nil
	case "k":
		return size / 1024 / float64(numSockets), nil
	}
	return 0, fmt.Errorf("unknown cache units in lscpu: %s", units)
}
