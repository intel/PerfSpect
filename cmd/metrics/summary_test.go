package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExcludeFinalSample(t *testing.T) {
	tests := []struct {
		name          string
		inputRows     []row
		expectedCount int
		expectedMaxTS float64
	}{
		{
			name: "exclude single final timestamp",
			inputRows: []row{
				{timestamp: 5.0, metrics: map[string]float64{"metric1": 100.0}},
				{timestamp: 10.0, metrics: map[string]float64{"metric1": 200.0}},
				{timestamp: 15.0, metrics: map[string]float64{"metric1": 150.0}},
				{timestamp: 20.0, metrics: map[string]float64{"metric1": 50.0}}, // this should be excluded
			},
			expectedCount: 3,
			expectedMaxTS: 15.0,
		},
		{
			name: "exclude multiple rows with same final timestamp",
			inputRows: []row{
				{timestamp: 5.0, socket: "0", metrics: map[string]float64{"metric1": 100.0}},
				{timestamp: 10.0, socket: "0", metrics: map[string]float64{"metric1": 200.0}},
				{timestamp: 15.0, socket: "0", metrics: map[string]float64{"metric1": 150.0}},
				{timestamp: 15.0, socket: "1", metrics: map[string]float64{"metric1": 160.0}}, // same timestamp, different socket
			},
			expectedCount: 2,
			expectedMaxTS: 10.0,
		},
		{
			name: "single sample - should not exclude",
			inputRows: []row{
				{timestamp: 5.0, metrics: map[string]float64{"metric1": 100.0}},
			},
			expectedCount: 1,
			expectedMaxTS: 5.0,
		},
		{
			name: "two samples - exclude last one",
			inputRows: []row{
				{timestamp: 5.0, metrics: map[string]float64{"metric1": 100.0}},
				{timestamp: 10.0, metrics: map[string]float64{"metric1": 50.0}},
			},
			expectedCount: 1,
			expectedMaxTS: 5.0,
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
				{timestamp: 5.0, socket: "0", metrics: map[string]float64{"metric1": 100.0}},
				{timestamp: 10.0, socket: "0", metrics: map[string]float64{"metric1": 200.0}},
				{timestamp: 15.0, socket: "0", metrics: map[string]float64{"metric1": 50.0}}, // should be excluded
			},
		},
		MetricGroup{
			names:        []string{"metric1"},
			groupByField: "SKT",
			groupByValue: "1",
			rows: []row{
				{timestamp: 5.0, socket: "1", metrics: map[string]float64{"metric1": 110.0}},
				{timestamp: 10.0, socket: "1", metrics: map[string]float64{"metric1": 210.0}},
				{timestamp: 15.0, socket: "1", metrics: map[string]float64{"metric1": 60.0}}, // should be excluded
			},
		},
	}

	mc.excludeFinalSample()

	// Both groups should have 2 rows remaining
	assert.Equal(t, 2, len(mc[0].rows), "socket 0 should have 2 rows")
	assert.Equal(t, 2, len(mc[1].rows), "socket 1 should have 2 rows")

	// Verify max timestamps
	assert.Equal(t, 10.0, mc[0].rows[1].timestamp, "socket 0 max timestamp should be 10.0")
	assert.Equal(t, 10.0, mc[1].rows[1].timestamp, "socket 1 max timestamp should be 10.0")
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
