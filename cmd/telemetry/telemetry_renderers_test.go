// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package telemetry

import (
	"fmt"
	"testing"
)

func TestComputeAxisMax(t *testing.T) {
	tests := []struct {
		name      string
		data      [][]float64
		wantEmpty bool // true means expect "" (no constraint)
		wantAbove float64
		wantBelow float64
	}{
		{
			name:      "normal data, no outliers",
			data:      [][]float64{{10, 12, 11, 13, 10, 12, 11, 14, 10, 13}},
			wantEmpty: true,
		},
		{
			name:      "single extreme outlier",
			data:      [][]float64{{10, 12, 11, 13, 10, 12, 11, 14, 10, 10000}},
			wantEmpty: false,
			wantAbove: 13,
			wantBelow: 10000,
		},
		{
			name:      "all identical values",
			data:      [][]float64{{5, 5, 5, 5, 5, 5, 5, 5, 5, 5}},
			wantEmpty: true,
		},
		{
			name:      "too few data points",
			data:      [][]float64{{10, 20, 30}},
			wantEmpty: true,
		},
		{
			name:      "empty data",
			data:      [][]float64{},
			wantEmpty: true,
		},
		{
			name:      "multiple datasets, one with outlier",
			data:      [][]float64{{10, 12, 11, 13, 10}, {11, 14, 10, 13, 50000}},
			wantEmpty: false,
			wantAbove: 13,
			wantBelow: 50000,
		},
		{
			name:      "gradual increase, no outlier",
			data:      [][]float64{{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
			wantEmpty: true,
		},
		{
			name:      "all zeros",
			data:      [][]float64{{0, 0, 0, 0, 0}},
			wantEmpty: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeAxisMax(tt.data)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("computeAxisMax() = %q, want empty string", got)
				}
				return
			}
			if got == "" {
				t.Errorf("computeAxisMax() = empty string, want a constraining value")
				return
			}
			// Parse and check bounds
			var val float64
			n, err := fmt.Sscanf(got, "%f", &val)
			if err != nil || n != 1 {
				t.Errorf("computeAxisMax() = %q, could not parse as float", got)
				return
			}
			if val <= tt.wantAbove {
				t.Errorf("computeAxisMax() = %f, want > %f", val, tt.wantAbove)
			}
			if val >= tt.wantBelow {
				t.Errorf("computeAxisMax() = %f, want < %f", val, tt.wantBelow)
			}
		})
	}
}
