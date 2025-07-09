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
	l3MB, err := getCacheMBLscpu(outputs, `^L3 cache.*:\s*(.+?)$`)
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

func l2FromOutput(outputs map[string]script.ScriptOutput) string {
	l2MB, err := getCacheMBLscpu(outputs, `^L2 cache:\s*(.+)$`)
	if err != nil {
		slog.Error("Failed to get L2 cache size from lscpu", slog.String("error", err.Error()))
		return ""
	}
	return formatCacheSizeMB(l2MB)
}

func l1dFromOutput(outputs map[string]script.ScriptOutput) string {
	l1dMB, err := getCacheMBLscpu(outputs, `^L1d cache:\s*(.+)$`)
	if err != nil {
		slog.Error("Failed to get L1d cache size from lscpu", slog.String("error", err.Error()))
		return ""
	}
	return formatCacheSizeMB(l1dMB)
}

func l1iFromOutput(outputs map[string]script.ScriptOutput) string {
	l1iMB, err := getCacheMBLscpu(outputs, `^L1i cache:\s*(.+)$`)
	if err != nil {
		slog.Error("Failed to get L1i cache size from lscpu", slog.String("error", err.Error()))
		return ""
	}
	return formatCacheSizeMB(l1iMB)
}

func getCacheLscpuParts(lscpuCache string) (size float64, units string, instances int, err error) {
	re := regexp.MustCompile(`(\d+\.?\d*)\s*(\w+)\s+\((\d+) instance[s]*\)`) // match known formats
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

func formatCacheSizeMB(size float64) string {
	return fmt.Sprintf("%s MiB", strconv.FormatFloat(size, 'f', -1, 64))
}

func getCacheMBLscpu(outputs map[string]script.ScriptOutput, cacheRegex string) (float64, error) {
	sockets := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)
	if sockets == "" {
		return 0, fmt.Errorf("failed to parse sockets from lscpu output")
	}
	numSockets, err := strconv.Atoi(sockets)
	if err != nil || numSockets == 0 {
		return 0, fmt.Errorf("failed to parse sockets from lscpu output: %s, %v", sockets, err)
	}
	cacheSize := valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, cacheRegex)
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
