package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"testing"
)

func TestParseCacheSizeToMB(t *testing.T) {
	tests := []struct {
		input     string
		fieldName string
		want      float64
		wantErr   bool
	}{
		{"32K", "test", 32.0 / 1024.0, false},
		{"1024K", "test", 1.0, false},
		{"2M", "test", 2.0, false},
		{"2MB", "test", 2.0, false},
		{"1G", "test", 1024.0, false},
		{"1GB", "test", 1024.0, false},
		{"0K", "test", 0.0, false},
		{"", "empty", 0.0, true},
		{"100", "no_suffix", 0.0, true},
		{"abcK", "invalid_number", 0.0, true},
		{"5T", "unknown_suffix", 0.0, true},
		{"  4M  ", "spaces", 4.0, false},
		{"8kB", "case", 8.0 / 1024.0, false},
		{"3m", "case", 3.0, false},
		{"2g", "case", 2048.0, false},
	}

	for _, tt := range tests {
		got, err := parseCacheSizeToMB(tt.input, tt.fieldName)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseCacheSizeToMB(%q, %q) error = %v, wantErr %v", tt.input, tt.fieldName, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("parseCacheSizeToMB(%q, %q) = %v, want %v", tt.input, tt.fieldName, got, tt.want)
		}
	}
}
