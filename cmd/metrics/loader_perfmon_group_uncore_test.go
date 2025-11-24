package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeUncoreGroups_EliminateDuplicateEvents(t *testing.T) {
	tests := []struct {
		name           string
		inputGroups    []UncoreGroup
		expectedEvents map[string]int // map of event name to expected count across all groups
		wantErr        bool
	}{
		{
			name: "duplicate events across two groups",
			inputGroups: []UncoreGroup{
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "UNC_CHA_TOR_INSERTS.IA_MISS", Unit: "cha", EventCode: "0x35", Counter: "0,1,2,3"},
						{EventName: "UNC_CHA_TOR_OCCUPANCY.IA_MISS", Unit: "cha", EventCode: "0x36", Counter: "0"},
					},
					MetricNames: []string{"metric1"},
				},
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "UNC_CHA_TOR_INSERTS.IA_MISS", Unit: "cha", EventCode: "0x35", Counter: "0,1,2,3"}, // duplicate
						{EventName: "UNC_CHA_CLOCKTICKS", Unit: "cha", EventCode: "0x01", Counter: "0,1,2,3"},
					},
					MetricNames: []string{"metric2"},
				},
			},
			expectedEvents: map[string]int{
				"UNC_CHA_TOR_INSERTS.IA_MISS":   1, // should appear only once after deduplication
				"UNC_CHA_TOR_OCCUPANCY.IA_MISS": 1,
				"UNC_CHA_CLOCKTICKS":            1,
			},
			wantErr: false,
		},
		{
			name: "duplicate events across three groups",
			inputGroups: []UncoreGroup{
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "UNC_M_CAS_COUNT.RD", Unit: "imc", EventCode: "0x04", Counter: "0,1,2,3"},
					},
					MetricNames: []string{"metric1"},
				},
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "UNC_M_CAS_COUNT.RD", Unit: "imc", EventCode: "0x04", Counter: "0,1,2,3"}, // duplicate
						{EventName: "UNC_M_CAS_COUNT.WR", Unit: "imc", EventCode: "0x04", Counter: "0,1,2,3"},
					},
					MetricNames: []string{"metric2"},
				},
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "UNC_M_CAS_COUNT.RD", Unit: "imc", EventCode: "0x04", Counter: "0,1,2,3"}, // duplicate
						{EventName: "UNC_M_CLOCKTICKS", Unit: "imc", EventCode: "0x01", Counter: "0,1,2,3"},
					},
					MetricNames: []string{"metric3"},
				},
			},
			expectedEvents: map[string]int{
				"UNC_M_CAS_COUNT.RD": 1, // should appear only once
				"UNC_M_CAS_COUNT.WR": 1,
				"UNC_M_CLOCKTICKS":   1,
			},
			wantErr: false,
		},
		{
			name: "no duplicate events",
			inputGroups: []UncoreGroup{
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "UNC_CHA_TOR_INSERTS.IA_MISS", Unit: "cha", EventCode: "0x35", Counter: "0,1,2,3"},
					},
					MetricNames: []string{"metric1"},
				},
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "UNC_CHA_CLOCKTICKS", Unit: "cha", EventCode: "0x01", Counter: "0,1,2,3"},
					},
					MetricNames: []string{"metric2"},
				},
			},
			expectedEvents: map[string]int{
				"UNC_CHA_TOR_INSERTS.IA_MISS": 1,
				"UNC_CHA_CLOCKTICKS":          1,
			},
			wantErr: false,
		},
		{
			name: "empty events should not count as duplicates",
			inputGroups: []UncoreGroup{
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "UNC_CHA_TOR_INSERTS.IA_MISS", Unit: "cha", EventCode: "0x35", Counter: "0,1,2,3"},
						{}, // empty event
					},
					MetricNames: []string{"metric1"},
				},
				{
					GeneralPurposeCounters: []UncoreEvent{
						{}, // empty event
						{EventName: "UNC_CHA_CLOCKTICKS", Unit: "cha", EventCode: "0x01", Counter: "0,1,2,3"},
					},
					MetricNames: []string{"metric2"},
				},
			},
			expectedEvents: map[string]int{
				"UNC_CHA_TOR_INSERTS.IA_MISS": 1,
				"UNC_CHA_CLOCKTICKS":          1,
			},
			wantErr: false,
		},
		{
			name: "duplicate in same group with duplicates across groups",
			inputGroups: []UncoreGroup{
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "UNC_CHA_TOR_INSERTS.IA_MISS", Unit: "cha", EventCode: "0x35", Counter: "0,1,2,3"},
						{EventName: "UNC_CHA_TOR_OCCUPANCY.IA_MISS", Unit: "cha", EventCode: "0x36", Counter: "0"},
					},
					MetricNames: []string{"metric1"},
				},
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "UNC_CHA_TOR_INSERTS.IA_MISS", Unit: "cha", EventCode: "0x35", Counter: "0,1,2,3"}, // duplicate from group 0
						{EventName: "UNC_CHA_TOR_OCCUPANCY.IA_MISS", Unit: "cha", EventCode: "0x36", Counter: "0"},     // duplicate from group 0
					},
					MetricNames: []string{"metric2"},
				},
			},
			expectedEvents: map[string]int{
				"UNC_CHA_TOR_INSERTS.IA_MISS":   1,
				"UNC_CHA_TOR_OCCUPANCY.IA_MISS": 1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a deep copy of input groups to avoid modifying the test data
			inputCopy := make([]UncoreGroup, len(tt.inputGroups))
			for i, group := range tt.inputGroups {
				inputCopy[i] = group.Copy()
			}

			result, err := MergeUncoreGroups(inputCopy)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Count occurrences of each event across all groups
			eventCounts := make(map[string]int)
			for _, group := range result {
				for _, event := range group.GeneralPurposeCounters {
					if !event.IsEmpty() {
						eventCounts[event.EventName]++
					}
				}
			}

			// Verify each expected event appears the correct number of times
			for eventName, expectedCount := range tt.expectedEvents {
				actualCount, found := eventCounts[eventName]
				assert.True(t, found, "Expected event %s not found in result", eventName)
				assert.Equal(t, expectedCount, actualCount, "Event %s appears %d times, expected %d", eventName, actualCount, expectedCount)
			}

			// Verify no unexpected events are present
			for eventName := range eventCounts {
				_, expected := tt.expectedEvents[eventName]
				assert.True(t, expected, "Unexpected event %s found in result", eventName)
			}
		})
	}
}

func TestMergeUncoreGroups_KeepsFirstOccurrence(t *testing.T) {
	// Verify that when duplicates are removed, the first occurrence is kept
	inputGroups := []UncoreGroup{
		{
			GeneralPurposeCounters: []UncoreEvent{
				{EventName: "EVENT_A", Unit: "cha", EventCode: "0x01", Counter: "0,1,2,3"},
			},
			MetricNames: []string{"metric1"},
		},
		{
			GeneralPurposeCounters: []UncoreEvent{
				{EventName: "EVENT_A", Unit: "cha", EventCode: "0x01", Counter: "0,1,2,3"}, // duplicate, should be removed
				{EventName: "EVENT_B", Unit: "cha", EventCode: "0x02", Counter: "0,1,2,3"},
			},
			MetricNames: []string{"metric2"},
		},
		{
			GeneralPurposeCounters: []UncoreEvent{
				{EventName: "EVENT_A", Unit: "cha", EventCode: "0x01", Counter: "0,1,2,3"}, // duplicate, should be removed
			},
			MetricNames: []string{"metric3"},
		},
	}

	result, err := MergeUncoreGroups(inputGroups)
	require.NoError(t, err)

	// Find the group containing EVENT_A
	foundInGroup := -1
	for i, group := range result {
		for _, event := range group.GeneralPurposeCounters {
			if event.EventName == "EVENT_A" {
				foundInGroup = i
				break
			}
		}
		if foundInGroup >= 0 {
			break
		}
	}

	// EVENT_A should be found
	require.GreaterOrEqual(t, foundInGroup, 0, "EVENT_A should be present in at least one group")

	// Count total occurrences
	count := 0
	for _, group := range result {
		for _, event := range group.GeneralPurposeCounters {
			if event.EventName == "EVENT_A" {
				count++
			}
		}
	}

	assert.Equal(t, 1, count, "EVENT_A should appear exactly once after deduplication")
}

func TestMergeUncoreGroups_MergingBehavior(t *testing.T) {
	// Test that groups are still merged when possible after deduplication
	tests := []struct {
		name              string
		inputGroups       []UncoreGroup
		maxExpectedGroups int
		minExpectedGroups int
		description       string
	}{
		{
			name: "compatible groups should merge after deduplication",
			inputGroups: []UncoreGroup{
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "EVENT_A", Unit: "cha", EventCode: "0x01", Counter: "0,1,2,3"},
						{}, // empty slot
						{}, // empty slot
						{}, // empty slot
					},
					MetricNames: []string{"metric1"},
				},
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "EVENT_A", Unit: "cha", EventCode: "0x01", Counter: "0,1,2,3"}, // duplicate, will be removed
						{EventName: "EVENT_B", Unit: "cha", EventCode: "0x02", Counter: "0,1,2,3"}, // should merge with first group
						{}, // empty slot
						{}, // empty slot
					},
					MetricNames: []string{"metric2"},
				},
			},
			maxExpectedGroups: 1,
			minExpectedGroups: 1,
			description:       "After removing EVENT_A duplicate, groups should merge into one",
		},
		{
			name: "incompatible units prevent merging",
			inputGroups: []UncoreGroup{
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "EVENT_A", Unit: "cha", EventCode: "0x01", Counter: "0,1,2,3"},
					},
					MetricNames: []string{"metric1"},
				},
				{
					GeneralPurposeCounters: []UncoreEvent{
						{EventName: "EVENT_B", Unit: "imc", EventCode: "0x02", Counter: "0,1,2,3"}, // different unit
					},
					MetricNames: []string{"metric2"},
				},
			},
			maxExpectedGroups: 2,
			minExpectedGroups: 2,
			description:       "Groups with different units cannot merge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MergeUncoreGroups(tt.inputGroups)
			require.NoError(t, err)

			assert.LessOrEqual(t, len(result), tt.maxExpectedGroups,
				"Result has more groups than expected: %s", tt.description)
			assert.GreaterOrEqual(t, len(result), tt.minExpectedGroups,
				"Result has fewer groups than expected: %s", tt.description)
		})
	}
}

func TestMergeUncoreGroups_PreservesMetricNames(t *testing.T) {
	// Verify that metric names are preserved during deduplication and merging
	inputGroups := []UncoreGroup{
		{
			GeneralPurposeCounters: []UncoreEvent{
				{EventName: "EVENT_A", Unit: "cha", EventCode: "0x01", Counter: "0,1,2,3"},
			},
			MetricNames: []string{"metric1"},
		},
		{
			GeneralPurposeCounters: []UncoreEvent{
				{EventName: "EVENT_A", Unit: "cha", EventCode: "0x01", Counter: "0,1,2,3"}, // duplicate
				{EventName: "EVENT_B", Unit: "cha", EventCode: "0x02", Counter: "0,1,2,3"},
			},
			MetricNames: []string{"metric2"},
		},
	}

	result, err := MergeUncoreGroups(inputGroups)
	require.NoError(t, err)

	// Collect all metric names from result groups
	allMetrics := make(map[string]bool)
	for _, group := range result {
		for _, metric := range group.MetricNames {
			allMetrics[metric] = true
		}
	}

	// Both metrics should be preserved
	assert.True(t, allMetrics["metric1"], "metric1 should be preserved")
	assert.True(t, allMetrics["metric2"], "metric2 should be preserved")
}

func TestMergeUncoreGroups_EmptyInput(t *testing.T) {
	// Test with empty input
	result, err := MergeUncoreGroups([]UncoreGroup{})
	require.NoError(t, err)
	assert.Empty(t, result, "Empty input should return empty result")
}

func TestMergeUncoreGroups_SingleGroup(t *testing.T) {
	// Test with a single group
	inputGroups := []UncoreGroup{
		{
			GeneralPurposeCounters: []UncoreEvent{
				{EventName: "EVENT_A", Unit: "cha", EventCode: "0x01", Counter: "0,1,2,3"},
			},
			MetricNames: []string{"metric1"},
		},
	}

	result, err := MergeUncoreGroups(inputGroups)
	require.NoError(t, err)
	assert.Len(t, result, 1, "Single group should remain as single group")
	assert.Equal(t, "EVENT_A", result[0].GeneralPurposeCounters[0].EventName)
}

func TestMergeUncoreGroups_AllEmptyEvents(t *testing.T) {
	// Test with groups containing only empty events
	inputGroups := []UncoreGroup{
		{
			GeneralPurposeCounters: []UncoreEvent{
				{}, // empty
				{}, // empty
			},
			MetricNames: []string{"metric1"},
		},
		{
			GeneralPurposeCounters: []UncoreEvent{
				{}, // empty
			},
			MetricNames: []string{"metric2"},
		},
	}

	result, err := MergeUncoreGroups(inputGroups)
	require.NoError(t, err)

	// Count non-empty events
	nonEmptyCount := 0
	for _, group := range result {
		for _, event := range group.GeneralPurposeCounters {
			if !event.IsEmpty() {
				nonEmptyCount++
			}
		}
	}

	assert.Equal(t, 0, nonEmptyCount, "No non-empty events should be present in result")
}
