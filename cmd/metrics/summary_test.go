// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/casbin/govaluate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExcludeFinalSample(t *testing.T) {
	tests := []struct {
		name          string
		inputRows     []row
		expectedCount int
		expectedMaxTS int
	}{
		{
			name: "exclude single final timestamp",
			inputRows: []row{
				{timestamp: 5, metrics: map[string]float64{"metric1": 100.0}},
				{timestamp: 10, metrics: map[string]float64{"metric1": 200.0}},
				{timestamp: 15, metrics: map[string]float64{"metric1": 150.0}},
				{timestamp: 20, metrics: map[string]float64{"metric1": 50.0}}, // this should be excluded
			},
			expectedCount: 3,
			expectedMaxTS: 15,
		},
		{
			name: "exclude multiple rows with same final timestamp",
			inputRows: []row{
				{timestamp: 5, socket: "0", metrics: map[string]float64{"metric1": 100.0}},
				{timestamp: 10, socket: "0", metrics: map[string]float64{"metric1": 200.0}},
				{timestamp: 15, socket: "0", metrics: map[string]float64{"metric1": 150.0}},
				{timestamp: 15, socket: "1", metrics: map[string]float64{"metric1": 160.0}}, // same timestamp, different socket
			},
			expectedCount: 2,
			expectedMaxTS: 10,
		},
		{
			name: "single sample - should not exclude",
			inputRows: []row{
				{timestamp: 5, metrics: map[string]float64{"metric1": 100.0}},
			},
			expectedCount: 1,
			expectedMaxTS: 5,
		},
		{
			name: "two samples - exclude last one",
			inputRows: []row{
				{timestamp: 5, metrics: map[string]float64{"metric1": 100.0}},
				{timestamp: 10, metrics: map[string]float64{"metric1": 50.0}},
			},
			expectedCount: 1,
			expectedMaxTS: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a MetricCollection with a single MetricGroup
			mc := MetricCollection{
				MetricGroup{
					names:        []string{"metric1"},
					rows:         tt.inputRows,
					groupByField: "",
					groupByValue: "",
				},
			}

			// Call excludeFinalSample
			mc.excludeFinalSample()

			// Verify the number of remaining rows
			assert.Equal(t, tt.expectedCount, len(mc[0].rows), "unexpected number of rows after exclusion")

			// Verify that no row has a timestamp greater than expectedMaxTS
			if len(mc[0].rows) > 0 {
				for _, row := range mc[0].rows {
					assert.LessOrEqual(t, row.timestamp, tt.expectedMaxTS, "found row with timestamp greater than expected maximum")
				}
			}
		})
	}
}

func TestExcludeFinalSampleMultipleGroups(t *testing.T) {
	// Test with multiple metric groups (e.g., multiple sockets)
	mc := MetricCollection{
		MetricGroup{
			names:        []string{"metric1"},
			groupByField: "SKT",
			groupByValue: "0",
			rows: []row{
				{timestamp: 5, socket: "0", metrics: map[string]float64{"metric1": 100.0}},
				{timestamp: 10, socket: "0", metrics: map[string]float64{"metric1": 200.0}},
				{timestamp: 15, socket: "0", metrics: map[string]float64{"metric1": 50.0}}, // should be excluded
			},
		},
		MetricGroup{
			names:        []string{"metric1"},
			groupByField: "SKT",
			groupByValue: "1",
			rows: []row{
				{timestamp: 5, socket: "1", metrics: map[string]float64{"metric1": 110.0}},
				{timestamp: 10, socket: "1", metrics: map[string]float64{"metric1": 210.0}},
				{timestamp: 15, socket: "1", metrics: map[string]float64{"metric1": 60.0}}, // should be excluded
			},
		},
	}

	mc.excludeFinalSample()

	// Both groups should have 2 rows remaining
	assert.Equal(t, 2, len(mc[0].rows), "socket 0 should have 2 rows")
	assert.Equal(t, 2, len(mc[1].rows), "socket 1 should have 2 rows")

	// Verify max timestamps
	assert.Equal(t, 10, mc[0].rows[1].timestamp, "socket 0 max timestamp should be 10")
	assert.Equal(t, 10, mc[1].rows[1].timestamp, "socket 1 max timestamp should be 10")
}

func TestExcludeFinalSampleEmptyCollection(t *testing.T) {
	// Test with empty MetricCollection
	mc := MetricCollection{}
	mc.excludeFinalSample() // should not panic
	assert.Equal(t, 0, len(mc), "empty collection should remain empty")
}

func TestExcludeFinalSampleEmptyRows(t *testing.T) {
	// Test with MetricGroup that has no rows
	mc := MetricCollection{
		MetricGroup{
			names:        []string{"metric1"},
			groupByField: "",
			groupByValue: "",
			rows:         []row{},
		},
	}
	mc.excludeFinalSample() // should not panic
	assert.Equal(t, 0, len(mc[0].rows), "empty rows should remain empty")
}

func TestProcessEventsMapsContainerNames(t *testing.T) {
	origScope := flagScope
	origGranularity := flagGranularity
	defer func() {
		flagScope = origScope
		flagGranularity = origGranularity
	}()
	flagScope = scopeContainer
	flagGranularity = granularitySystem

	expr, err := govaluate.NewEvaluableExpression("[instructions] / [cpu-cycles]")
	assert.NoError(t, err)
	metricDefs := []MetricDefinition{
		{
			Name:       "IPC",
			LegacyName: "IPC",
			Expression: "[instructions] / [cpu-cycles]",
			Variables:  map[string]int{"instructions": -1, "cpu-cycles": -1},
			Evaluable:  expr,
		},
	}
	groupDefs := []GroupDefinition{
		{
			{Name: "instructions", Raw: "instructions"},
			{Name: "cpu-cycles", Raw: "cpu-cycles"},
		},
	}
	metadata := Metadata{
		ContainerNames: map[string]string{
			"system.slice/docker-aaa.scope": "nginx",
		},
	}
	rawEvents := [][]byte{
		[]byte(`{"interval" : 5.0, "counter-value" : "200.0", "cgroup" : "system.slice/docker-aaa.scope", "event" : "instructions", "pcnt-running" : 100.0}`),
		[]byte(`{"interval" : 5.0, "counter-value" : "100.0", "cgroup" : "system.slice/docker-aaa.scope", "event" : "cpu-cycles", "pcnt-running" : 100.0}`),
		[]byte(`{"interval" : 5.0, "counter-value" : "300.0", "cgroup" : "/system.slice/docker-unmapped.scope", "event" : "instructions", "pcnt-running" : 100.0}`),
		[]byte(`{"interval" : 5.0, "counter-value" : "100.0", "cgroup" : "/system.slice/docker-unmapped.scope", "event" : "cpu-cycles", "pcnt-running" : 100.0}`),
	}

	frames, ts, err := ProcessEvents(rawEvents, groupDefs, metricDefs, nil, 0, metadata)
	assert.NoError(t, err)
	assert.Equal(t, 5.0, ts)
	assert.Len(t, frames, 2)
	assert.Equal(t, "nginx", frames[0].Cgroup)
	assert.InDelta(t, 2.0, frames[0].Metrics[0].Value, 1e-6)
	assert.Equal(t, "/system.slice/docker-unmapped.scope", frames[1].Cgroup)
	assert.InDelta(t, 3.0, frames[1].Metrics[0].Value, 1e-6)
}

func TestMetricCollectionCSVWithContainerNames(t *testing.T) {
	names := []string{"CPU utilization %"}

	t.Run("multi-container", func(t *testing.T) {
		mc := MetricCollection{
			{names: names, groupByField: "CID", groupByValue: "pod-a/webapp", rows: []row{{timestamp: 5, cgroup: "pod-a/webapp", metrics: map[string]float64{names[0]: 20.0}}}},
			{names: names, groupByField: "CID", groupByValue: "pod-b/webapp", rows: []row{{timestamp: 5, cgroup: "pod-b/webapp", metrics: map[string]float64{names[0]: 40.0}}}},
		}
		csvOut, err := mc.getCSV()
		assert.NoError(t, err)
		assert.Contains(t, csvOut, "metric,pod-a/webapp,pod-b/webapp\n")
		assert.Contains(t, csvOut, "CPU utilization %,20.000000,40.000000\n")
	})

	t.Run("single-container", func(t *testing.T) {
		mc := MetricCollection{
			{names: names, groupByField: "CID", groupByValue: "nginx", rows: []row{{timestamp: 5, cgroup: "nginx", metrics: map[string]float64{names[0]: 25.0}}}},
		}
		csvOut, err := mc.getCSV()
		assert.NoError(t, err)
		assert.Contains(t, csvOut, "CID,metric,mean,min,max,stddev\n")
		assert.Contains(t, csvOut, "nginx,CPU utilization %,25.000000,")
	})
}

func TestAddGroupTemplateValuesGroupMeta(t *testing.T) {
	names := []string{"CPU utilization %"}

	t.Run("non-container sets empty GROUPMETA", func(t *testing.T) {
		mc := MetricCollection{
			{names: names, groupByField: "CID", groupByValue: "cg1", rows: []row{{timestamp: 5, metrics: map[string]float64{names[0]: 10.0}}}},
			{names: names, groupByField: "CID", groupByValue: "cg2", rows: []row{{timestamp: 5, metrics: map[string]float64{names[0]: 20.0}}}},
		}
		tmplVals := make(map[string]string)
		err := mc.addGroupTemplateValues(tmplVals, []MetricDefinition{{Name: names[0]}}, Metadata{})
		assert.NoError(t, err)
		assert.Equal(t, `"CID"`, tmplVals["GROUPBYFIELD"])
		assert.Equal(t, "[]", tmplVals["GROUPMETA"])
	})

	t.Run("structured ContainerInfo preserves single and multi-namespace fields", func(t *testing.T) {
		metadata := Metadata{
			CommonMetadata: CommonMetadata{
				SocketCount:    1,
				CoresPerSocket: 2,
			},
			Scope:       scopeContainer,
			Granularity: granularitySystem,
			ContainerNames: map[string]string{
				"kubepods.slice/p1/c1": "prod/webapp-788bd774ff-5spvq/webapp",
				"kubepods.slice/p2/c2": "default/nginx-pod/nginx",
			},
			ContainerInfo: map[string]ContainerMetadata{
				"prod/webapp-788bd774ff-5spvq/webapp": {
					ID:          "1111222233334444555566667777888899990000aaaabbbbccccddddeeeeffff",
					DisplayName: "prod/webapp-788bd774ff-5spvq/webapp",
					Name:        "webapp",
					PodName:     "webapp-788bd774ff-5spvq",
					ShortPod:    "…-5spvq",
					Namespace:   "prod",
					Workload:    "webapp",
					Runtime:     "cri",
					CgroupPath:  "kubepods.slice/p1/c1",
				},
				"default/nginx-pod/nginx": {
					ID:          "aaaabbbbccccddddeeeeffff1111222233334444555566667777888899990000",
					DisplayName: "default/nginx-pod/nginx",
					Name:        "nginx",
					PodName:     "nginx-pod",
					ShortPod:    "nginx-pod",
					Namespace:   "default",
					Workload:    "nginx-pod",
					Runtime:     "cri",
					CgroupPath:  "kubepods.slice/p2/c2",
				},
			},
		}
		mc := MetricCollection{
			{names: names, groupByField: "CID", groupByValue: "prod/webapp-788bd774ff-5spvq/webapp", rows: []row{{timestamp: 5, metrics: map[string]float64{names[0]: 15.0}}}},
			{names: names, groupByField: "CID", groupByValue: "default/nginx-pod/nginx", rows: []row{{timestamp: 5, metrics: map[string]float64{names[0]: 25.0}}}},
		}
		tmplVals := make(map[string]string)
		err := mc.addGroupTemplateValues(tmplVals, []MetricDefinition{{Name: names[0]}}, metadata)
		assert.NoError(t, err)
		assert.Equal(t, `"CONTAINER"`, tmplVals["GROUPBYFIELD"])
		assert.Contains(t, tmplVals["GROUPMETA"], `"id":"1111222233334444555566667777888899990000aaaabbbbccccddddeeeeffff"`)
		assert.Contains(t, tmplVals["GROUPMETA"], `"shortPod":"…-5spvq"`)
		assert.Contains(t, tmplVals["GROUPMETA"], `"workload":"webapp"`)
		assert.Contains(t, tmplVals["GROUPMETA"], `"namespace":"prod"`)
		assert.Contains(t, tmplVals["GROUPMETA"], `"namespace":"default"`)

		out, err := mc.getHTML(metadata, []MetricDefinition{{Name: names[0]}})
		assert.NoError(t, err)
		assert.Contains(t, out, `const group_meta = [{"id":"1111222233334444555566667777888899990000aaaabbbbccccddddeeeeffff","displayName":"prod/webapp-788bd774ff-5spvq/webapp"`)

		// Verify metrics trim HTML metadata round-trip preserves Scope, Granularity, and ContainerInfo.ID.
		htmlPath := filepath.Join(t.TempDir(), "host_metrics_summary.html")
		require.NoError(t, os.WriteFile(htmlPath, []byte(out), 0644))
		roundTripped, err := loadMetadataFromHTMLSummary(htmlPath)
		require.NoError(t, err)
		assert.Equal(t, scopeContainer, roundTripped.Scope)
		assert.Equal(t, granularitySystem, roundTripped.Granularity)
		assert.Equal(t, metadata.ContainerNames, roundTripped.ContainerNames)
		assert.Equal(t, metadata.ContainerInfo, roundTripped.ContainerInfo)
	})

	t.Run("legacy metadata without ContainerInfo infers metadata from display name", func(t *testing.T) {
		metadata := Metadata{
			ContainerNames: map[string]string{
				"kubepods.slice/kubepods-burstable.slice/pod1/c1": "nginx",
				"kubepods.slice/kubepods-burstable.slice/pod2/c2": "webapp-788bd774ff-5spvq/webapp",
			},
		}
		mc := MetricCollection{
			{names: names, groupByField: "CID", groupByValue: "nginx", rows: []row{{timestamp: 5, metrics: map[string]float64{names[0]: 10.0}}}},
			{names: names, groupByField: "CID", groupByValue: "webapp-788bd774ff-5spvq/webapp", rows: []row{{timestamp: 5, metrics: map[string]float64{names[0]: 20.0}}}},
		}
		tmplVals := make(map[string]string)
		err := mc.addGroupTemplateValues(tmplVals, []MetricDefinition{{Name: names[0]}}, metadata)
		assert.NoError(t, err)
		assert.Equal(t, `"CONTAINER"`, tmplVals["GROUPBYFIELD"])
		assert.Contains(t, tmplVals["GROUPMETA"], `{"displayName":"nginx","name":"nginx","cgroupPath":"kubepods.slice/kubepods-burstable.slice/pod1/c1"}`)
		assert.Contains(t, tmplVals["GROUPMETA"], `{"displayName":"webapp-788bd774ff-5spvq/webapp","name":"webapp","podName":"webapp-788bd774ff-5spvq","shortPod":"…-5spvq","namespace":"default","workload":"webapp","cgroupPath":"kubepods.slice/kubepods-burstable.slice/pod2/c2"}`)
	})

	t.Run("deterministic cgroupByDisplayName precedence", func(t *testing.T) {
		metadata := Metadata{
			ContainerInfo: map[string]ContainerMetadata{
				"nginx": {
					DisplayName: "nginx",
					Name:        "nginx",
					CgroupPath:  "docker/info-path",
				},
			},
			ContainerNames: map[string]string{
				"docker/z-old-path": "nginx",
				"docker/a-new-path": "nginx",
				"docker/redis-path": "redis",
			},
			CgroupConfigs: []CgroupConfig{
				{Name: "docker/config-override", ContainerName: "redis"},
				{Name: "docker/only-config", ContainerName: "only-cfg"},
			},
		}
		mc := MetricCollection{
			{names: names, groupByField: "CID", groupByValue: "nginx", rows: []row{{timestamp: 5, metrics: map[string]float64{names[0]: 10.0}}}},
			{names: names, groupByField: "CID", groupByValue: "redis", rows: []row{{timestamp: 5, metrics: map[string]float64{names[0]: 20.0}}}},
			{names: names, groupByField: "CID", groupByValue: "only-cfg", rows: []row{{timestamp: 5, metrics: map[string]float64{names[0]: 30.0}}}},
		}
		tmplVals := make(map[string]string)
		require.NoError(t, mc.addGroupTemplateValues(tmplVals, []MetricDefinition{{Name: names[0]}}, metadata))
		assert.Contains(t, tmplVals["GROUPS"], `["nginx","1","docker/info-path"]`)
		assert.Contains(t, tmplVals["GROUPS"], `["redis","1","docker/redis-path"]`)
		assert.Contains(t, tmplVals["GROUPS"], `["only-cfg","1","docker/only-config"]`)
	})
}

func TestBaseHTMLBabel6Compatibility(t *testing.T) {
	htmlBytes, err := resources.ReadFile("resources/base.html")
	require.NoError(t, err)
	html := string(htmlBytes)

	startMarker := `<script type="text/babel">`
	startIdx := strings.Index(html, startMarker)
	require.NotEqual(t, -1, startIdx, "babel script block not found")
	scriptBody := html[startIdx+len(startMarker):]
	endIdx := strings.Index(scriptBody, `</script>`)
	require.NotEqual(t, -1, endIdx, "babel script closing tag not found")
	scriptBody = scriptBody[:endIdx]

	optionalChainingRe := regexp.MustCompile(`\?\.[A-Za-z_$(\[]`)
	assert.False(t, optionalChainingRe.MatchString(scriptBody), "Babel 6 does not support optional chaining (?.)")
	assert.NotContains(t, scriptBody, "??", "Babel 6 does not support nullish coalescing (??)")
	assert.False(t, regexp.MustCompile(`\.replaceAll\s*\(`).MatchString(scriptBody), "avoid String.prototype.replaceAll for older browser compatibility")
	assert.NotContains(t, scriptBody, "<>", "Babel 6 does not support JSX shorthand fragments (<>)")
	assert.NotContains(t, scriptBody, "</>", "Babel 6 does not support JSX shorthand fragments (</>)")
}
