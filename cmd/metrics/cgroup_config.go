// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

// Static configuration of the cgroups monitored at cgroup scope. These are the limits and
// affinities set on the cgroup, not measurements of its behavior, and they give the report's
// per-cgroup metrics the context needed to interpret them, e.g., how many CPUs a metric's
// value is spread across.

import (
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"perfspect/internal/script"
	"perfspect/internal/target"
	"perfspect/internal/util"
	"perfspect/internal/workflow"
)

// unlimited is the value used for a limit that is not set, i.e., the cgroup file holds "max"
const unlimited = int64(-1)

// CgroupConfig holds the static configuration of one cgroup. Limit fields hold 'unlimited'
// when the corresponding cgroup file holds "max", and 0 is a real limit of zero.
type CgroupConfig struct {
	Name string `json:"name"` // cgroup path as it appears in the report's CID field, i.e., no leading slash
	// CPU
	CPUs     string  `json:"cpus"`     // cpuset.cpus.effective, e.g., "0-3,96-99"
	CPUCount int     `json:"cpuCount"` // number of CPUs in CPUs
	CPUQuota float64 `json:"cpuQuota"` // cpu.max expressed as a count of CPUs, or 'unlimited'
	// memory
	NUMANodes       string `json:"numaNodes"`       // cpuset.mems.effective, e.g., "0"
	MemoryLimit     int64  `json:"memoryLimit"`     // lowest memory.max in this cgroup's ancestry
	MemoryLimitFrom string `json:"memoryLimitFrom"` // the cgroup that imposes MemoryLimit, when not this one
	MemoryHigh      int64  `json:"memoryHigh"`      // this cgroup's memory.high
	SwapLimit       int64  `json:"swapLimit"`       // this cgroup's memory.swap.max
	// block IO
	IOLimits        []IODeviceLimit  `json:"ioLimits"`        // io.max, per device
	IOWeight        int              `json:"ioWeight"`        // io.weight's default, 0 when unavailable
	IOWeightDevices []IODeviceWeight `json:"ioWeightDevices"` // io.weight's per-device overrides
}

// IODeviceLimit holds one device's io.max throttling limits. Each field holds 'unlimited'
// when it is not set, which is the case for every field of a device absent from io.max.
type IODeviceLimit struct {
	Device string `json:"device"` // block device as the kernel reports it, i.e., "major:minor"
	RBPS   int64  `json:"rbps"`   // read bytes per second
	WBPS   int64  `json:"wbps"`   // write bytes per second
	RIOPS  int64  `json:"riops"`  // read IOs per second
	WIOPS  int64  `json:"wiops"`  // write IOs per second
}

// IODeviceWeight holds one device's io.weight override, i.e., a line of io.weight other
// than its "default" line
type IODeviceWeight struct {
	Device string `json:"device"` // block device as the kernel reports it, i.e., "major:minor"
	Weight int    `json:"weight"`
}

// GetCgroupConfigs reads the static configuration of each of the given cgroups from the
// target's cgroup v2 filesystem. The cgroups are named as they are passed to perf, i.e.,
// relative to the cgroup mount point.
//
// A cgroup whose configuration cannot be read is omitted rather than failing the
// collection; this information is supplementary to the metrics.
func GetCgroupConfigs(myTarget target.Target, cgroups []string, localTempDir string) ([]CgroupConfig, error) {
	if len(cgroups) == 0 {
		return nil, nil
	}
	quoted := make([]string, 0, len(cgroups))
	for _, cgroup := range cgroups {
		// single-quote for the shell, escaping any embedded single quotes
		quoted = append(quoted, "'"+strings.ReplaceAll(cgroup, "'", `'\''`)+"'")
	}
	// memory.max is not resolved across the hierarchy by the kernel the way
	// cpuset.cpus.effective is, so the effective limit is the lowest memory.max from the
	// cgroup up to the root.
	cgroupConfigScript := script.ScriptDefinition{
		Name: "cgroup_config",
		ScriptTemplate: fmt.Sprintf(`
root="/sys/fs/cgroup"
for rel in %s; do
    rel=${rel#/}
    d="$root/$rel"
    if [ ! -d "$d" ]; then
        continue
    fi
    cpus=$(cat "$d/cpuset.cpus.effective" 2>/dev/null)
    mems=$(cat "$d/cpuset.mems.effective" 2>/dev/null)
    cpumax=$(cat "$d/cpu.max" 2>/dev/null)
    memhigh=$(cat "$d/memory.high" 2>/dev/null)
    swapmax=$(cat "$d/memory.swap.max" 2>/dev/null)
    lim=""
    src=""
    dd="$d"
    while :; do
        v=$(cat "$dd/memory.max" 2>/dev/null)
        if [ -n "$v" ] && [ "$v" != "max" ]; then
            if [ -z "$lim" ] || [ "$v" -lt "$lim" ]; then
                lim="$v"
                src=${dd#"$root"}
            fi
        fi
        if [ "$dd" = "$root" ]; then
            break
        fi
        dd=$(dirname "$dd")
    done
    printf 'CG %%s|%%s|%%s|%%s|%%s|%%s|%%s|%%s\n' "$rel" "$cpus" "$mems" "$cpumax" "$lim" "$src" "$memhigh" "$swapmax"
    # io.max and io.weight are keyed by "major:minor", except for io.weight's "default"
    # line. The device is reported as the kernel spells it: resolving it to a name like
    # "sda" would need readlink, which is not part of every Linux userland. io.max is empty
    # when no throttling is configured, and io.weight is absent unless the IO controller
    # supports weights.
    for which in max weight; do
        if [ ! -f "$d/io.$which" ]; then
            continue
        fi
        while read -r dev rest; do
            if [ -z "$dev" ]; then
                continue
            fi
            printf 'IO%%s %%s|%%s|%%s\n' "$which" "$rel" "$dev" "$rest"
        done < "$d/io.$which"
    done
done
`, strings.Join(quoted, " ")),
		Superuser: true,
	}
	output, err := workflow.RunScript(myTarget, cgroupConfigScript, localTempDir, flagNoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get cgroup configuration: %v", err)
	}
	return parseCgroupConfigs(output.Stdout), nil
}

// mergeCgroupConfigs adds the newly collected cgroup configurations to those already
// collected, replacing the configuration of a cgroup that is already present. The
// monitored cgroups can change when the "hot" cgroup list is refreshed, and the report
// presents every cgroup that was monitored, so the configurations accumulate.
func mergeCgroupConfigs(existing []CgroupConfig, collected []CgroupConfig) []CgroupConfig {
	for _, config := range collected {
		idx := slices.IndexFunc(existing, func(e CgroupConfig) bool { return e.Name == config.Name })
		if idx == -1 {
			existing = append(existing, config)
			continue
		}
		existing[idx] = config
	}
	return existing
}

// parseCgroupConfigs parses the output of the cgroup configuration script
func parseCgroupConfigs(out string) []CgroupConfig {
	var configs []CgroupConfig
	// the IO records follow their cgroup's CG record, so index by name to attach them
	byName := make(map[string]int)
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "IOmax "), strings.HasPrefix(line, "IOweight "):
			prefix, rest, _ := strings.Cut(line, " ")
			fields := strings.SplitN(rest, "|", 3)
			if len(fields) != 3 {
				slog.Debug("unrecognized cgroup io line", slog.String("line", line))
				continue
			}
			idx, ok := byName[strings.TrimPrefix(fields[0], "/")]
			if !ok {
				slog.Debug("cgroup io line for unknown cgroup", slog.String("line", line))
				continue
			}
			if prefix == "IOmax" {
				values := parseIOFields(fields[2])
				configs[idx].IOLimits = append(configs[idx].IOLimits, IODeviceLimit{
					Device: fields[1],
					RBPS:   ioLimit(values, "rbps"),
					WBPS:   ioLimit(values, "wbps"),
					RIOPS:  ioLimit(values, "riops"),
					WIOPS:  ioLimit(values, "wiops"),
				})
				continue
			}
			// io.weight is "default <weight>" plus a line per device that overrides it
			weight, err := strconv.Atoi(strings.TrimSpace(fields[2]))
			if err != nil {
				slog.Debug("unrecognized cgroup io weight", slog.String("line", line))
				continue
			}
			if fields[1] == "default" {
				configs[idx].IOWeight = weight
				continue
			}
			configs[idx].IOWeightDevices = append(configs[idx].IOWeightDevices, IODeviceWeight{
				Device: fields[1],
				Weight: weight,
			})
		case strings.HasPrefix(line, "CG "):
			fields := strings.Split(strings.TrimPrefix(line, "CG "), "|")
			if len(fields) != 8 {
				slog.Debug("unrecognized cgroup config line", slog.String("line", line))
				continue
			}
			config := CgroupConfig{
				Name:            strings.TrimPrefix(fields[0], "/"),
				CPUs:            fields[1],
				CPUCount:        countCPUList(fields[1]),
				CPUQuota:        parseCPUMax(fields[3]),
				NUMANodes:       fields[2],
				MemoryLimit:     parseCgroupLimit(fields[4]),
				MemoryLimitFrom: strings.TrimPrefix(fields[5], "/"),
				MemoryHigh:      parseCgroupLimit(fields[6]),
				SwapLimit:       parseCgroupLimit(fields[7]),
			}
			// the memory limit source is only interesting when an ancestor imposes it
			if config.MemoryLimitFrom == config.Name {
				config.MemoryLimitFrom = ""
			}
			byName[config.Name] = len(configs)
			configs = append(configs, config)
		}
	}
	return configs
}

// parseIOFields parses the "key=value key=value ..." portion of an io.max or io.stat line
func parseIOFields(fields string) map[string]string {
	values := make(map[string]string)
	for field := range strings.FieldsSeq(fields) {
		key, value, found := strings.Cut(field, "=")
		if !found {
			continue
		}
		values[key] = value
	}
	return values
}

// ioLimit returns an io.max limit, which is unlimited when absent or "max"
func ioLimit(values map[string]string, key string) int64 {
	value, ok := values[key]
	if !ok {
		return unlimited
	}
	return parseCgroupLimit(value)
}

// parseCgroupLimit parses a cgroup limit file's value, which is either a number of bytes
// or "max". An empty or unparsable value is reported as unlimited.
func parseCgroupLimit(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" || value == "max" {
		return unlimited
	}
	bytes, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return unlimited
	}
	return bytes
}

// parseCPUMax converts a cpu.max value, "$quota $period" in microseconds, into a count of
// CPUs. A quota of "max" is reported as unlimited.
func parseCPUMax(value string) float64 {
	fields := strings.Fields(value)
	if len(fields) != 2 || fields[0] == "max" {
		return float64(unlimited)
	}
	quota, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return float64(unlimited)
	}
	period, err := strconv.ParseFloat(fields[1], 64)
	if err != nil || period == 0 {
		return float64(unlimited)
	}
	return quota / period
}

// countCPUList counts the CPUs in a cpuset list, e.g., "0-3,96-99" is 8 CPUs. An empty
// list, which is how a cgroup with no cpuset of its own reads, is no CPUs.
func countCPUList(list string) int {
	list = strings.TrimSpace(list)
	if list == "" {
		return 0
	}
	cpus, err := util.SelectiveIntRangeToIntList(list)
	if err != nil {
		slog.Debug("unrecognized cpuset list", slog.String("list", list), slog.String("error", err.Error()))
		return 0
	}
	return len(cpus)
}

// formatBytesIEC formats a byte count using IEC units, e.g., 68719476736 is "64.0 GiB"
func formatBytesIEC(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	value := float64(bytes)
	for _, unit := range []string{"KiB", "MiB", "GiB", "TiB", "PiB"} {
		value /= 1024
		if value < 1024 {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%.1f EiB", value/1024)
}

// summarizeIOLimits renders the configured io.max limits, one entry per device named in
// io.max, e.g., "sda rbps=1.0 MiB/s wbps=max riops=max wiops=500". Every field is shown
// whether or not it is set, using io.max's own spelling of "max" for a field with no
// limit, and no device is omitted. The result is "unlimited" only when io.max names no
// device at all, which is the common case.
func summarizeIOLimits(limits []IODeviceLimit) string {
	if len(limits) == 0 {
		return "unlimited"
	}
	entries := make([]string, 0, len(limits))
	for _, limit := range limits {
		entries = append(entries, fmt.Sprintf("%s rbps=%s wbps=%s riops=%s wiops=%s",
			limit.Device,
			ioRateLimit(limit.RBPS), ioRateLimit(limit.WBPS),
			ioCountLimit(limit.RIOPS), ioCountLimit(limit.WIOPS)))
	}
	return strings.Join(entries, "; ")
}

// ioRateLimit renders an io.max bytes-per-second field
func ioRateLimit(value int64) string {
	if value == unlimited {
		return "max"
	}
	return formatBytesIEC(value) + "/s"
}

// ioCountLimit renders an io.max IOs-per-second field
func ioCountLimit(value int64) string {
	if value == unlimited {
		return "max"
	}
	return strconv.FormatInt(value, 10)
}

// summarizeIOWeight renders the io.weight configuration: the default weight, followed by
// any per-device overrides. It is empty when io.weight is absent, which is the case unless
// the IO controller supports weights.
func summarizeIOWeight(weight int, overrides []IODeviceWeight) string {
	if weight == 0 && len(overrides) == 0 {
		return ""
	}
	out := strconv.Itoa(weight)
	if len(overrides) == 0 {
		return out
	}
	entries := make([]string, 0, len(overrides))
	for _, override := range overrides {
		entries = append(entries, fmt.Sprintf("%s=%d", override.Device, override.Weight))
	}
	return fmt.Sprintf("%s (%s)", out, strings.Join(entries, ", "))
}

// cgroupConfigRows arranges the cgroup configurations into table rows for the report: a
// label followed by one value per cgroup, in the order the cgroups appear in the report.
//
// Each row is a label, the cgroup v2 file the value is read from, then one value per
// cgroup. Naming the file makes the value traceable and resolves ambiguity the label
// cannot, e.g., that the CPU list comes from cpuset.cpus.effective rather than
// cpuset.cpus, which usually reads empty.
//
// Every row is always returned, including rows that are blank or "unlimited" for every
// cgroup, so that the report presents the same fields in the same order regardless of how
// the cgroups happen to be configured. Omitting the configuration altogether, e.g., on a
// cgroup v1 host where none of it can be read, is the caller's decision.
func cgroupConfigRows(configs []CgroupConfig, names []string) [][]string {
	byName := make(map[string]CgroupConfig, len(configs))
	for _, config := range configs {
		byName[config.Name] = config
	}
	limit := func(value int64) string {
		switch value {
		case unlimited:
			return "unlimited"
		case 0:
			return "0"
		default:
			return formatBytesIEC(value)
		}
	}
	// label, then the cgroup v2 file the value comes from. memory.max is marked as walked
	// because the value is the lowest in the cgroup's ancestry, not just its own file.
	rows := [][]string{
		{"CPUs", "cpuset.cpus.effective"},
		{"CPU quota", "cpu.max"},
		{"NUMA nodes", "cpuset.mems.effective"},
		{"Memory limit", "memory.max (incl. ancestors)"},
		{"Memory high", "memory.high"},
		{"Swap limit", "memory.swap.max"},
		{"IO limits", "io.max"},
		{"IO weight", "io.weight"},
	}
	for _, name := range names {
		config, ok := byName[name]
		if !ok {
			for i := range rows {
				rows[i] = append(rows[i], "")
			}
			continue
		}
		cpus := config.CPUs
		if cpus != "" && config.CPUCount > 0 {
			cpus = fmt.Sprintf("%s (%d)", cpus, config.CPUCount)
		}
		quota := "unlimited"
		if config.CPUQuota != float64(unlimited) {
			// %g rather than a fixed precision: a quota is an exact quota/period ratio, and
			// rounding it would misstate the limit, e.g., 12.5 CPUs as 12
			quota = fmt.Sprintf("%g CPUs", config.CPUQuota)
		}
		memLimit := limit(config.MemoryLimit)
		if config.MemoryLimit != unlimited && config.MemoryLimitFrom != "" {
			memLimit = fmt.Sprintf("%s (from %s)", memLimit, config.MemoryLimitFrom)
		}
		values := []string{
			cpus, quota, config.NUMANodes, memLimit, limit(config.MemoryHigh), limit(config.SwapLimit),
			summarizeIOLimits(config.IOLimits), summarizeIOWeight(config.IOWeight, config.IOWeightDevices),
		}
		for i, value := range values {
			rows[i] = append(rows[i], value)
		}
	}
	return rows
}
