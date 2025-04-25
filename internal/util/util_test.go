package util

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0-alpha.1", "1.0.0-beta.1", -1},
		{"1.0.0-beta.1", "1.0.0-rc.1", -1},
		{"1.0.0-rc.1", "1.0.0", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha.2", -1},
		{"1.0.0-alpha.2", "1.0.0-alpha.1", 1},
		{"1.0.0-alpha.1", "1.0.0-alpha.1", 0},
	}

	for _, test := range tests {
		result, err := CompareVersions(test.v1, test.v2)
		if err != nil {
			t.Fatalf("failed to compare versions: %v", err)
		}
		if result != test.expected {
			t.Errorf("expected %d, got %d for versions %s and %s", test.expected, result, test.v1, test.v2)
		}
	}
}
