// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandConsolidatedFrequencies(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		bucketSizes   []int
		expected      []float64
		expectError   bool
		errorContains string
	}{
		{
			name:        "three consolidated ranges",
			input:       "1-40/3.5, 41-60/3.4, 61-86/3.2",
			bucketSizes: []int{20, 40, 60, 80, 86, 86, 86, 86},
			expected:    []float64{3.5, 3.5, 3.4, 3.2, 3.2, 3.2, 3.2, 3.2},
			expectError: false,
		},
		{
			name:        "two consolidated ranges",
			input:       "1-43/3.5, 44-86/3.2",
			bucketSizes: []int{20, 40, 60, 80, 86, 86, 86, 86},
			expected:    []float64{3.5, 3.5, 3.2, 3.2, 3.2, 3.2, 3.2, 3.2},
			expectError: false,
		},
		{
			name:        "eight separate ranges (no consolidation)",
			input:       "1-20/3.5, 21-40/3.4, 41-60/3.3, 61-80/3.2, 81-82/3.1, 83-84/3.0, 85-86/2.9, 87-88/2.8",
			bucketSizes: []int{20, 40, 60, 80, 82, 84, 86, 88},
			expected:    []float64{3.5, 3.4, 3.3, 3.2, 3.1, 3.0, 2.9, 2.8},
			expectError: false,
		},
		{
			name:        "single consolidated range",
			input:       "1-86/3.5",
			bucketSizes: []int{20, 40, 60, 80, 86, 86, 86, 86},
			expected:    []float64{3.5, 3.5, 3.5, 3.5, 3.5, 3.5, 3.5, 3.5},
			expectError: false,
		},
		{
			name:        "decimal frequencies",
			input:       "1-50/3.75, 51-86/2.25",
			bucketSizes: []int{20, 40, 60, 80, 86, 86, 86, 86},
			expected:    []float64{3.75, 3.75, 3.75, 2.25, 2.25, 2.25, 2.25, 2.25},
			expectError: false,
		},
		{
			name:          "wrong number of bucket sizes",
			input:         "1-40/3.5, 41-60/3.4",
			bucketSizes:   []int{20, 40, 60, 80},
			expected:      nil,
			expectError:   true,
			errorContains: "expected 8 bucket sizes",
		},
		{
			name:          "invalid format - missing slash",
			input:         "1-40 3.5, 41-60/3.4",
			bucketSizes:   []int{20, 40, 60, 80, 86, 86, 86, 86},
			expected:      nil,
			expectError:   true,
			errorContains: "invalid format",
		},
		{
			name:          "invalid format - missing dash in range",
			input:         "1/3.5, 41-60/3.4",
			bucketSizes:   []int{20, 40, 60, 80, 86, 86, 86, 86},
			expected:      nil,
			expectError:   true,
			errorContains: "invalid range format",
		},
		{
			name:          "invalid frequency value",
			input:         "1-40/abc, 41-60/3.4",
			bucketSizes:   []int{20, 40, 60, 80, 86, 86, 86, 86},
			expected:      nil,
			expectError:   true,
			errorContains: "invalid frequency",
		},
		{
			name:          "invalid core number",
			input:         "1-abc/3.5, 41-60/3.4",
			bucketSizes:   []int{20, 40, 60, 80, 86, 86, 86, 86},
			expected:      nil,
			expectError:   true,
			errorContains: "invalid end core",
		},
		{
			name:        "six consolidated ranges with smaller bucket sizes",
			input:       "1-44/3.6, 45-52/3.5, 53-60/3.4, 61-72/3.2, 73-76/3.1, 77-86/3.0",
			bucketSizes: []int{22, 26, 30, 34, 36, 38, 40, 43},
			expected:    []float64{3.6, 3.6, 3.6, 3.6, 3.6, 3.6, 3.6, 3.6},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandConsolidatedFrequencies(tt.input, tt.bucketSizes)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, len(tt.expected), len(result), "result should have 8 frequencies")
				for i := range tt.expected {
					assert.InDelta(t, tt.expected[i], result[i], 0.01, "frequency at index %d should match", i)
				}
			}
		})
	}
}

func TestExpandConsolidatedFrequencies_EdgeCases(t *testing.T) {
	t.Run("buckets with same end values", func(t *testing.T) {
		// Some buckets may have the same end value (e.g., when there are fewer active buckets)
		input := "1-60/3.5, 61-86/3.2"
		bucketSizes := []int{20, 40, 60, 86, 86, 86, 86, 86}
		expected := []float64{3.5, 3.5, 3.5, 3.2, 3.2, 3.2, 3.2, 3.2}

		result, err := expandConsolidatedFrequencies(input, bucketSizes)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, len(expected), len(result))
		for i := range expected {
			assert.InDelta(t, expected[i], result[i], 0.01)
		}
	})

	t.Run("very small buckets", func(t *testing.T) {
		input := "1-2/3.5, 3-4/3.4, 5-6/3.3, 7-8/3.2, 9-10/3.1, 11-12/3.0, 13-14/2.9, 15-16/2.8"
		bucketSizes := []int{2, 4, 6, 8, 10, 12, 14, 16}
		expected := []float64{3.5, 3.4, 3.3, 3.2, 3.1, 3.0, 2.9, 2.8}

		result, err := expandConsolidatedFrequencies(input, bucketSizes)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, len(expected), len(result))
		for i := range expected {
			assert.InDelta(t, expected[i], result[i], 0.01)
		}
	})
}
