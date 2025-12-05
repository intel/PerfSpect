package common

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"perfspect/internal/cpus"
	"perfspect/internal/script"
	"perfspect/internal/util"
	"strconv"
	"strings"
)

// GetL3MSRMB returns the L3 cache size per cache instance (per socket on Intel) and total in MB from MSR.
// We read from the MSR to handle the case where some cache ways are disabled, i.e.,
// when testing different cache sizes. The lscpu output always shows the maximum possible
// cache size, even if some ways are disabled.
func GetL3MSRMB(outputs map[string]script.ScriptOutput) (instance float64, total float64, err error) {
	uarch := UarchFromOutput(outputs)
	cpu, err := cpus.GetCPUByMicroArchitecture(uarch)
	if err != nil {
		return 0, 0, err
	}
	if cpu.CacheWayCount == 0 {
		err = fmt.Errorf("L3 cache way count is zero")
		return 0, 0, err
	}
	sockets := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)
	if sockets == "" {
		return 0, 0, fmt.Errorf("failed to parse sockets from lscpu output")
	}
	numSockets, err := strconv.Atoi(sockets)
	if err != nil || numSockets == 0 {
		return 0, 0, fmt.Errorf("failed to parse sockets from lscpu output: %s, %v", sockets, err)
	}
	// we get the unmodified/maximum possible L3 size from lscpu
	l3MaximumMB, _, err := GetL3LscpuMB(outputs)
	if err != nil {
		return 0, 0, err
	}
	// for every bit set in l3WayEnabled, a way is enabled
	l3WayEnabledMSRVal := strings.TrimSpace(outputs[script.L3CacheWayEnabledName].Stdout)
	if l3WayEnabledMSRVal == "" {
		err = fmt.Errorf("L3 cache way enabled MSR value is empty")
		return 0, 0, err
	}
	l3WayEnabled, err := strconv.ParseUint(l3WayEnabledMSRVal, 16, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse L3 way enabled MSR value: %s, %v", l3WayEnabledMSRVal, err)
	}
	if l3WayEnabled == 0 {
		err = fmt.Errorf("zero cache ways enabled: %s", l3WayEnabledMSRVal)
		return 0, 0, err
	}
	numCacheWaysEnabled := util.NumUint64Bits(l3WayEnabled)
	if numCacheWaysEnabled == 0 {
		err = fmt.Errorf("zero cache way bits set: %s", l3WayEnabledMSRVal)
		return 0, 0, err
	}

	cpul3SizeGB := l3MaximumMB / 1024
	GBperWay := cpul3SizeGB / float64(cpu.CacheWayCount)

	currentL3SizeGB := float64(numCacheWaysEnabled) * GBperWay
	return currentL3SizeGB * 1024, currentL3SizeGB * 1024 * float64(numSockets), nil
}

// GetL3LscpuMB returns the L3 cache size in MB as reported by lscpu.
func GetL3LscpuMB(outputs map[string]script.ScriptOutput) (instance float64, total float64, err error) {
	lscpuCache, err := ParseLscpuCacheOutput(outputs[script.LscpuCacheScriptName].Stdout)
	if err != nil {
		return 0, 0, err
	}
	l3CacheEntry, ok := lscpuCache["L3"]
	if !ok {
		return 0, 0, fmt.Errorf("L3 cache entry not found in lscpu cache output")
	}
	instance, err = l3CacheInstanceSizeFromLscpuCacheMB(l3CacheEntry)
	if err != nil {
		return 0, 0, err
	}
	total, err = l3CacheTotalSizeFromLscpuCacheMB(l3CacheEntry)
	if err != nil {
		return 0, 0, err
	}
	return instance, total, nil
}

// L3FromOutput attempts to retrieve the L3 cache size in megabytes from the provided
// script outputs. It first tries to obtain the value using GetL3MSRMB. If that fails,
// it falls back to using lscpu cache output. If both methods fail, it logs the errors and
// returns an empty string. On success, it returns the formatted cache size as a string.
func L3FromOutput(outputs map[string]script.ScriptOutput) string {
	l3InstanceMB, l3TotalMB, err := GetL3MSRMB(outputs)
	if err != nil {
		slog.Info("Could not get L3 size from MSR, falling back to lscpu", slog.String("error", err.Error()))
		l3InstanceMB, l3TotalMB, err = GetL3LscpuMB(outputs)
		if err != nil {
			slog.Error("Could not get L3 size from lscpu", slog.String("error", err.Error()))
			return ""
		}
	}
	return fmt.Sprintf("%s/%s", FormatCacheSizeMB(l3InstanceMB), FormatCacheSizeMB(l3TotalMB))
}

// L3PerCoreFromOutput calculates the amount of L3 cache (in MiB) available per core
// based on the provided script outputs. It first checks if the host is virtualized,
// in which case it returns an empty string since the calculation is not applicable.
// It parses the number of cores per socket and the number of sockets from the lscpu
// output. It attempts to retrieve the total L3 cache size using MSR data, falling
// back to parsing lscpu output if necessary. The result is formatted as a string
// with up to three decimal places, followed by " MiB". If any required data cannot
// be parsed, it logs an error and returns an empty string.
func L3PerCoreFromOutput(outputs map[string]script.ScriptOutput) string {
	virtualization := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Virtualization.*:\s*(.+?)$`)
	if virtualization == "full" {
		slog.Info("Can't calculate L3 per Core on virtualized host.")
		return ""
	}
	coresPerSocket, err := strconv.Atoi(ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket.*:\s*(.+?)$`))
	if err != nil {
		slog.Error("failed to parse cores per socket", slog.String("error", err.Error()))
		return ""
	}
	if coresPerSocket == 0 {
		slog.Error("cores per socket is zero")
		return ""
	}
	sockets, err := strconv.Atoi(ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+?)$`))
	if err != nil {
		slog.Error("failed to parse sockets from lscpu output", slog.String("error", err.Error()))
		return ""
	}
	if sockets == 0 {
		slog.Error("sockets is zero")
		return ""
	}
	var l3TotalMB float64
	_, l3TotalMB, err = GetL3MSRMB(outputs)
	if err != nil {
		slog.Debug("Could not get L3 size from MSR, falling back to lscpu", slog.String("error", err.Error()))
		_, l3TotalMB, err = GetL3LscpuMB(outputs)
		if err != nil {
			slog.Error("Could not get L3 size from lscpu", slog.String("error", err.Error()))
			return ""
		}
	}
	return FormatCacheSizeMB(l3TotalMB / (float64(coresPerSocket) * float64(sockets)))
}

// FormatCacheSizeMB formats a floating-point cache size value (in MB) as a string
// with the "M" unit suffix.
func FormatCacheSizeMB(size float64) string {
	val := strconv.FormatFloat(size, 'f', 3, 64)
	val = strings.TrimRight(val, "0") // trim trailing zeros
	val = strings.TrimRight(val, ".") // trim decimal point if trailing
	return fmt.Sprintf("%sM", val)
}

type lscpuCacheEntry struct {
	Name          string
	OneSize       string
	AllSize       string
	Ways          string
	Type          string
	Level         string
	Sets          string
	PhyLine       string
	CoherencySize string
}

// ParseLscpuCacheOutput parses the output of `lscpu -C` (text/tabular)
// Example output:
// NAME ONE-SIZE ALL-SIZE WAYS TYPE        LEVEL   SETS PHY-LINE COHERENCY-SIZE
// L1d       48K     8.1M   12 Data            1     64        1             64
// L1i       64K    10.8M   16 Instruction     1     64        1             64
// L2         2M     344M   16 Unified         2   2048        1             64
// L3       336M     672M   16 Unified         3 344064        1             64
func ParseLscpuCacheOutput(LscpuCacheOutput string) (map[string]lscpuCacheEntry, error) {
	trimmed := strings.TrimSpace(LscpuCacheOutput)
	if trimmed == "" {
		slog.Warn("lscpu cache output is empty")
		return nil, fmt.Errorf("lscpu cache output is empty")
	}
	lines := strings.Split(trimmed, "\n")
	// header-only is not valid; require at least one data line
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected lscpu cache output format: header only")
	}
	headerCols := strings.Fields(strings.TrimSpace(lines[0]))
	if len(headerCols) == 0 || strings.ToLower(headerCols[0]) != "name" {
		return nil, fmt.Errorf("invalid lscpu cache header")
	}
	// map header name (normalized) -> index
	idx := map[string]int{}
	for i, h := range headerCols {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	out := map[string]lscpuCacheEntry{}
	for _, line := range lines[1:] {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		cols := strings.Fields(l)
		if len(cols) < 4 {
			continue
		}
		entry := lscpuCacheEntry{}
		if i, ok := idx["name"]; ok && i < len(cols) {
			entry.Name = cols[i]
		}
		if i, ok := idx["one-size"]; ok && i < len(cols) {
			entry.OneSize = cols[i]
		}
		if i, ok := idx["all-size"]; ok && i < len(cols) {
			entry.AllSize = cols[i]
		}
		if i, ok := idx["ways"]; ok && i < len(cols) {
			entry.Ways = cols[i]
		}
		if i, ok := idx["type"]; ok && i < len(cols) {
			entry.Type = cols[i]
		}
		if i, ok := idx["level"]; ok && i < len(cols) {
			entry.Level = cols[i]
		}
		if i, ok := idx["sets"]; ok && i < len(cols) {
			entry.Sets = cols[i]
		}
		if i, ok := idx["phy-line"]; ok && i < len(cols) {
			entry.PhyLine = cols[i]
		}
		if i, ok := idx["coherency-size"]; ok && i < len(cols) {
			entry.CoherencySize = cols[i]
		}
		if entry.Name == "" {
			continue
		}
		out[entry.Name] = entry
	}
	return out, nil
}

// L1l2CacheSizeFromLscpuCache extracts the data cache size from the provided lscpuCacheEntry.
func L1l2CacheSizeFromLscpuCache(entry lscpuCacheEntry) string {
	return entry.OneSize
}

// parseCacheSizeToMB parses a cache size string (e.g., "32K", "2M", "1G") and converts it to megabytes.
// The input string can have optional "B" suffix and supports K, M, G units.
func parseCacheSizeToMB(sizeString, fieldName string) (float64, error) {
	if sizeString == "" {
		return 0, fmt.Errorf("%s is empty", fieldName)
	}
	sizeStr := strings.ToUpper(strings.TrimSpace(sizeString))
	sizeStr = strings.TrimRight(sizeStr, "B") // remove trailing B if present

	var multiplier float64
	if strings.HasSuffix(sizeStr, "K") {
		multiplier = 1.0 / 1024.0
		sizeStr = strings.TrimRight(sizeStr, "K")
	} else if strings.HasSuffix(sizeStr, "M") {
		multiplier = 1.0
		sizeStr = strings.TrimRight(sizeStr, "M")
	} else if strings.HasSuffix(sizeStr, "G") {
		multiplier = 1024.0
		sizeStr = strings.TrimRight(sizeStr, "G")
	} else {
		return 0, fmt.Errorf("unknown size suffix in %s: %s", fieldName, sizeString)
	}

	sizeVal, err := strconv.ParseFloat(sizeStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s value: %s, %v", fieldName, sizeString, err)
	}
	return sizeVal * multiplier, nil
}

// l3CacheTotalSizeFromLscpuCacheMB extracts the total L3 cache size in megabytes from the provided lscpuCacheEntry.
func l3CacheTotalSizeFromLscpuCacheMB(entry lscpuCacheEntry) (float64, error) {
	return parseCacheSizeToMB(entry.AllSize, "L3 cache all-size")
}

// l3CacheInstanceSizeFromLscpuCacheMB extracts the L3 cache instance size in megabytes from the provided lscpuCacheEntry.
func l3CacheInstanceSizeFromLscpuCacheMB(entry lscpuCacheEntry) (float64, error) {
	return parseCacheSizeToMB(entry.OneSize, "L3 cache one-size")
}
