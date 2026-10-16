// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCountCPUList(t *testing.T) {
	tests := []struct {
		name     string
		list     string
		expected int
	}{
		{name: "empty, as a cgroup with no cpuset of its own reads", list: "", expected: 0},
		{name: "single cpu", list: "3", expected: 1},
		{name: "single range", list: "0-7", expected: 8},
		{name: "ranges and singles, as a pinned container reports", list: "0-3,96-99", expected: 8},
		{name: "mixed", list: "0,2-4,8", expected: 5},
		{name: "whole machine", list: "0-191", expected: 192},
		{name: "reversed range", list: "7-0", expected: 0},
		{name: "unparsable", list: "a-b", expected: 0},
		// a malformed entry rejects the whole list rather than reporting a count that is
		// quietly short, since a wrong CPU count would misrepresent every metric beside it
		{name: "partially malformed", list: "0-3,bogus", expected: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, countCPUList(test.list))
		})
	}
}

func TestParseCgroupLimit(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected int64
	}{
		{name: "max is unlimited", value: "max", expected: unlimited},
		{name: "empty is unlimited", value: "", expected: unlimited},
		{name: "unparsable is unlimited", value: "nonsense", expected: unlimited},
		{name: "zero is a real limit", value: "0", expected: 0},
		{name: "byte count", value: "68719476736", expected: 68719476736},
		{name: "trailing whitespace", value: "1024\n", expected: 1024},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, parseCgroupLimit(test.value))
		})
	}
}

func TestParseCPUMax(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected float64
	}{
		{name: "unlimited quota", value: "max 100000", expected: float64(unlimited)},
		{name: "four cpus", value: "400000 100000", expected: 4},
		{name: "half a cpu", value: "50000 100000", expected: 0.5},
		{name: "missing period", value: "400000", expected: float64(unlimited)},
		{name: "zero period", value: "400000 0", expected: float64(unlimited)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, parseCPUMax(test.value))
		})
	}
}

func TestFormatBytesIEC(t *testing.T) {
	assert.Equal(t, "512 B", formatBytesIEC(512))
	assert.Equal(t, "1.0 KiB", formatBytesIEC(1024))
	assert.Equal(t, "64.0 GiB", formatBytesIEC(68719476736))
	assert.Equal(t, "1.0 TiB", formatBytesIEC(1099511627776))
}

// parseCgroupConfigs is given the script's output verbatim, in which each cgroup's IOmax
// and IOweight records follow its CG record.
func TestParseCgroupConfigs(t *testing.T) {
	out := `CG /system.slice/docker-aaa.scope|0-3,96-99|0|max 100000|68719476736|/system.slice|max|0
IOmax /system.slice/docker-aaa.scope|8:0|rbps=1048576 wbps=max riops=max wiops=500
IOweight /system.slice/docker-aaa.scope|default|100
IOweight /system.slice/docker-aaa.scope|8:0|200
CG /system.slice/docker-bbb.scope|4-7,100-103|0-1|400000 100000||||max
IOweight /system.slice/docker-bbb.scope|default|50
`
	configs := parseCgroupConfigs(out)
	assert.Len(t, configs, 2)

	// the leading slash is stripped so the name matches the report's CID field
	assert.Equal(t, "system.slice/docker-aaa.scope", configs[0].Name)
	assert.Equal(t, "0-3,96-99", configs[0].CPUs)
	assert.Equal(t, 8, configs[0].CPUCount)
	assert.Equal(t, float64(unlimited), configs[0].CPUQuota)
	assert.Equal(t, "0", configs[0].NUMANodes)
	assert.Equal(t, int64(68719476736), configs[0].MemoryLimit)
	// an ancestor imposes the limit, so it is attributed
	assert.Equal(t, "system.slice", configs[0].MemoryLimitFrom)
	assert.Equal(t, unlimited, configs[0].MemoryHigh)
	assert.Equal(t, int64(0), configs[0].SwapLimit)
	// io.max: the fields set are kept, the "max" fields read as unlimited
	assert.Equal(t, []IODeviceLimit{{Device: "8:0", RBPS: 1048576, WBPS: unlimited, RIOPS: unlimited, WIOPS: 500}}, configs[0].IOLimits)
	// io.weight: the "default" line sets the weight, other lines are per-device overrides
	assert.Equal(t, 100, configs[0].IOWeight)
	assert.Equal(t, []IODeviceWeight{{Device: "8:0", Weight: 200}}, configs[0].IOWeightDevices)

	// the IO records are attached to the cgroup they name, not to the most recent one
	assert.Equal(t, "system.slice/docker-bbb.scope", configs[1].Name)
	assert.Equal(t, 8, configs[1].CPUCount)
	assert.Equal(t, float64(4), configs[1].CPUQuota)
	assert.Equal(t, "0-1", configs[1].NUMANodes)
	assert.Equal(t, unlimited, configs[1].MemoryLimit)
	assert.Equal(t, unlimited, configs[1].SwapLimit)
	// io.max is empty when nothing is throttled
	assert.Empty(t, configs[1].IOLimits)
	assert.Equal(t, 50, configs[1].IOWeight)
	assert.Empty(t, configs[1].IOWeightDevices)
}

func TestParseCgroupConfigsIgnoresMalformedLines(t *testing.T) {
	out := `CG too|few|fields
IOweight only|two
IOweight /unknown.scope|default|100
garbage
CG /a.scope|0-1|0|max 100000|max||max|max
IOweight /a.scope|default|notanumber
`
	configs := parseCgroupConfigs(out)
	assert.Len(t, configs, 1)
	assert.Equal(t, "a.scope", configs[0].Name)
	assert.Equal(t, 2, configs[0].CPUCount)
	// an IO record naming a cgroup with no CG record is dropped rather than misattributed,
	// and an unparsable weight leaves the weight unset
	assert.Equal(t, 0, configs[0].IOWeight)
}

func TestSummarizeIOLimits(t *testing.T) {
	// "unlimited" only when io.max names no device at all
	assert.Equal(t, "unlimited", summarizeIOLimits(nil))
	// a device named in io.max is kept even when none of its fields are set, and every
	// field is shown, so the rendering has the same shape whatever is configured
	assert.Equal(t, "8:0 rbps=max wbps=max riops=max wiops=max", summarizeIOLimits([]IODeviceLimit{
		{Device: "8:0", RBPS: unlimited, WBPS: unlimited, RIOPS: unlimited, WIOPS: unlimited},
	}))
	// byte limits are rates, IO limits are counts, and unset fields read as "max"
	assert.Equal(t, "8:0 rbps=1.0 MiB/s wbps=max riops=max wiops=500", summarizeIOLimits([]IODeviceLimit{
		{Device: "8:0", RBPS: 1048576, WBPS: unlimited, RIOPS: unlimited, WIOPS: 500},
	}))
	assert.Equal(t, "8:0 rbps=1.0 MiB/s wbps=max riops=max wiops=max; 259:0 rbps=max wbps=2.0 MiB/s riops=max wiops=max",
		summarizeIOLimits([]IODeviceLimit{
			{Device: "8:0", RBPS: 1048576, WBPS: unlimited, RIOPS: unlimited, WIOPS: unlimited},
			{Device: "259:0", RBPS: unlimited, WBPS: 2097152, RIOPS: unlimited, WIOPS: unlimited},
		}))
}

func TestSummarizeIOWeight(t *testing.T) {
	// io.weight is absent unless the IO controller supports weights, leaving a blank cell
	assert.Equal(t, "", summarizeIOWeight(0, nil))
	// the default weight alone
	assert.Equal(t, "100", summarizeIOWeight(100, nil))
	// per-device overrides are listed after the default
	assert.Equal(t, "100 (8:0=200)", summarizeIOWeight(100, []IODeviceWeight{{Device: "8:0", Weight: 200}}))
	assert.Equal(t, "50 (8:0=200, 259:0=10)", summarizeIOWeight(50, []IODeviceWeight{
		{Device: "8:0", Weight: 200},
		{Device: "259:0", Weight: 10},
	}))
}

func TestMergeCgroupConfigs(t *testing.T) {
	existing := []CgroupConfig{{Name: "a", CPUCount: 4}, {Name: "b", CPUCount: 8}}
	// "b" is refreshed and "c" is newly monitored; "a" is no longer hot but is retained
	// because the report still presents its metrics
	merged := mergeCgroupConfigs(existing, []CgroupConfig{{Name: "b", CPUCount: 16}, {Name: "c", CPUCount: 2}})
	assert.Len(t, merged, 3)
	assert.Equal(t, 4, merged[0].CPUCount)
	assert.Equal(t, 16, merged[1].CPUCount)
	assert.Equal(t, 2, merged[2].CPUCount)
}

func TestCgroupConfigRows(t *testing.T) {
	configs := []CgroupConfig{
		{
			Name: "cg1", CPUs: "0-3,96-99", CPUCount: 8, CPUQuota: float64(unlimited),
			NUMANodes:   "0",
			MemoryLimit: 68719476736, MemoryLimitFrom: "system.slice",
			MemoryHigh: unlimited, SwapLimit: 0,
			IOLimits:        []IODeviceLimit{{Device: "8:0", RBPS: 1048576, WBPS: unlimited, RIOPS: unlimited, WIOPS: unlimited}},
			IOWeight:        100,
			IOWeightDevices: []IODeviceWeight{{Device: "8:0", Weight: 200}},
		},
		{
			Name: "cg2", CPUs: "4-7,100-103", CPUCount: 8, CPUQuota: 4,
			NUMANodes:   "0",
			MemoryLimit: unlimited, MemoryHigh: unlimited, SwapLimit: unlimited,
			IOWeight: 50,
		},
	}
	rows := cgroupConfigRows(configs, []string{"cg1", "cg2"})

	labels := make([]string, 0, len(rows))
	sources := make([]string, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, row[0])
		sources = append(sources, row[1])
	}
	// every field is present, in a fixed order, whatever the values turn out to be
	assert.Equal(t, []string{"CPUs", "CPU quota", "NUMA nodes", "Memory limit", "Memory high",
		"Swap limit", "IO limits", "IO weight"}, labels)
	// each row names the cgroup v2 file its value is read from
	assert.Equal(t, []string{"cpuset.cpus.effective", "cpu.max", "cpuset.mems.effective",
		"memory.max (incl. ancestors)", "memory.high", "memory.swap.max", "io.max",
		"io.weight"}, sources)

	assert.Equal(t, []string{"CPUs", "cpuset.cpus.effective", "0-3,96-99 (8)", "4-7,100-103 (8)"}, rows[0])
	assert.Equal(t, []string{"CPU quota", "cpu.max", "unlimited", "4 CPUs"}, rows[1])
	// the NUMA node list alone, without the nodes' memory size
	assert.Equal(t, []string{"NUMA nodes", "cpuset.mems.effective", "0", "0"}, rows[2])
	// the ancestor that imposes the limit is named
	assert.Equal(t, []string{"Memory limit", "memory.max (incl. ancestors)",
		"64.0 GiB (from system.slice)", "unlimited"}, rows[3])
	// kept even though it is unlimited for every cgroup
	assert.Equal(t, []string{"Memory high", "memory.high", "unlimited", "unlimited"}, rows[4])
	assert.Equal(t, []string{"Swap limit", "memory.swap.max", "0", "unlimited"}, rows[5])
	assert.Equal(t, []string{"IO limits", "io.max",
		"8:0 rbps=1.0 MiB/s wbps=max riops=max wiops=max", "unlimited"}, rows[6])
	assert.Equal(t, []string{"IO weight", "io.weight", "100 (8:0=200)", "50"}, rows[7])
}

// A CPU quota is an exact quota/period ratio, so it is rendered without rounding. A fixed
// precision would misstate the limit, e.g., 12.5 CPUs as "12" and 100 CPUs as "1e+02".
func TestCgroupConfigRowsCPUQuotaPrecision(t *testing.T) {
	configs := []CgroupConfig{
		{Name: "a", CPUQuota: 12.5, MemoryLimit: unlimited, MemoryHigh: unlimited, SwapLimit: unlimited},
		{Name: "b", CPUQuota: 100, MemoryLimit: unlimited, MemoryHigh: unlimited, SwapLimit: unlimited},
		{Name: "c", CPUQuota: 0.125, MemoryLimit: unlimited, MemoryHigh: unlimited, SwapLimit: unlimited},
	}
	rows := cgroupConfigRows(configs, []string{"a", "b", "c"})
	assert.Equal(t, []string{"CPU quota", "cpu.max", "12.5 CPUs", "100 CPUs", "0.125 CPUs"}, rows[1])
}

// A cgroup present in the report but absent from the collected configuration, e.g., one
// that went away before its configuration could be read, leaves blank cells rather than
// shifting the remaining values into the wrong columns.
func TestCgroupConfigRowsMissingCgroup(t *testing.T) {
	configs := []CgroupConfig{{Name: "cg2", CPUs: "0-1", CPUCount: 2, CPUQuota: float64(unlimited), MemoryLimit: unlimited, MemoryHigh: unlimited, SwapLimit: unlimited}}
	rows := cgroupConfigRows(configs, []string{"cg1", "cg2"})
	assert.Equal(t, []string{"CPUs", "cpuset.cpus.effective", "", "0-1 (2)"}, rows[0])
}

// No configuration at all, e.g., a cgroup v1 host, still produces every row, with every
// cell blank. Suppressing the configuration entirely in that case is the caller's job; see
// addGroupTemplateValues, which only sets the template value when a configuration was
// collected.
func TestCgroupConfigRowsNoConfigs(t *testing.T) {
	rows := cgroupConfigRows(nil, []string{"cg1", "cg2"})
	assert.Len(t, rows, 8)
	// the label and the source file are still present; only the values are blank
	assert.Equal(t, []string{"CPUs", "cpuset.cpus.effective", "", ""}, rows[0])
	assert.Equal(t, []string{"IO weight", "io.weight", "", ""}, rows[7])
}

// The report's template must always receive a GROUPCONFIGS value: an unset template value
// renders as nothing, which would produce invalid JavaScript in the report.
func TestGetHTMLRendersCgroupConfigs(t *testing.T) {
	names := []string{"CPU utilization %"}
	mc := MetricCollection{
		{
			names:        names,
			groupByField: "CID",
			groupByValue: "system.slice/docker-aaa.scope",
			rows:         []row{{timestamp: 5, cgroup: "system.slice/docker-aaa.scope", metrics: map[string]float64{names[0]: 21.1}}},
		},
		{
			names:        names,
			groupByField: "CID",
			groupByValue: "system.slice/docker-bbb.scope",
			rows:         []row{{timestamp: 5, cgroup: "system.slice/docker-bbb.scope", metrics: map[string]float64{names[0]: 54.1}}},
		},
	}
	metadata := Metadata{}
	metadata.CgroupConfigs = []CgroupConfig{
		{
			Name: "system.slice/docker-aaa.scope", CPUs: "0-3,96-99", CPUCount: 8,
			CPUQuota: float64(unlimited), NUMANodes: "0",
			MemoryLimit: 68719476736, MemoryHigh: unlimited, SwapLimit: unlimited,
		},
		{
			Name: "system.slice/docker-bbb.scope", CPUs: "4-7,100-103", CPUCount: 8,
			CPUQuota: float64(unlimited), NUMANodes: "0",
			MemoryLimit: 34359738368, MemoryHigh: unlimited, SwapLimit: unlimited,
		},
	}
	out, err := mc.getHTML(metadata, []MetricDefinition{{Name: names[0]}})
	assert.NoError(t, err)
	assert.NotContains(t, out, "const group_configs = \n", "template value must not render empty")
	assert.Contains(t, out, `"0-3,96-99 (8)"`)
	assert.Contains(t, out, `"64.0 GiB"`)
	assert.Contains(t, out, `"32.0 GiB"`)
}

// Without cgroup configuration the template value must still be valid JavaScript
func TestGetHTMLRendersWithoutCgroupConfigs(t *testing.T) {
	names := []string{"CPU utilization %"}
	mc := MetricCollection{
		{names: names, groupByField: "CID", groupByValue: "cg1", rows: []row{{timestamp: 5, metrics: map[string]float64{names[0]: 1}}}},
		{names: names, groupByField: "CID", groupByValue: "cg2", rows: []row{{timestamp: 5, metrics: map[string]float64{names[0]: 2}}}},
	}
	out, err := mc.getHTML(Metadata{}, []MetricDefinition{{Name: names[0]}})
	assert.NoError(t, err)
	assert.Contains(t, out, "const group_configs = []")
}
