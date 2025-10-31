package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestParseLscpuCacheOutput(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedError  bool
		expectedLength int
		expectedL3     lscpuCacheEntry
	}{
		{
			name:           "Typical table output",
			input:          "NAME ONE-SIZE ALL-SIZE WAYS TYPE LEVEL SETS PHY-LINE COHERENCY-SIZE\nL1d 48K 8.1M 12 Data 1 64 1 64\nL1i 64K 10.8M 16 Instruction 1 64 1 64\nL2 2M 344M 16 Unified 2 2048 1 64\nL3 336M 672M 16 Unified 3 344064 1 64",
			expectedError:  false,
			expectedLength: 4,
			expectedL3: lscpuCacheEntry{
				Name:          "L3",
				OneSize:       "336M",
				AllSize:       "672M",
				Ways:          "16",
				Type:          "Unified",
				Level:         "3",
				Sets:          "344064",
				PhyLine:       "1",
				CoherencySize: "64",
			},
		},
		{
			name:           "Missing optional numeric columns",
			input:          "NAME ONE-SIZE ALL-SIZE WAYS TYPE LEVEL\nL3 320M 640M 20 Unified 3",
			expectedError:  false,
			expectedLength: 1,
			expectedL3: lscpuCacheEntry{
				Name:    "L3",
				OneSize: "320M",
				AllSize: "640M",
				Ways:    "20",
				Type:    "Unified",
				Level:   "3",
			},
		},
		{
			name:          "Empty input",
			input:         "",
			expectedError: true,
		},
		{
			name:          "Header only",
			input:         "Name  One Size  All Size  Ways  Type  Level  Sets  Phys-Line  Coherency-Size",
			expectedError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseLscpuCacheOutput(tt.input)
			if tt.expectedError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tt.expectedLength)
				if tt.expectedLength > 0 && tt.expectedL3.Name != "" {
					l3Cache, exists := result["L3"]
					require.True(t, exists, "L3 cache should exist in result")
					assert.Equal(t, tt.expectedL3, l3Cache)
				}
			}
		})
	}
}
