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
			name: "Modern format with integer fields",
			input: `{
   "caches": [
      {
         "name": "L1d",
         "one-size": "48K",
         "all-size": "6M",
         "ways": 12,
         "type": "Data",
         "level": 1,
         "sets": 64,
         "phy-line": 1,
         "coherency-size": 64
      },
      {
         "name": "L3",
         "one-size": "320M",
         "all-size": "640M",
         "ways": 20,
         "type": "Unified",
         "level": 3,
         "sets": 262144,
         "phy-line": 1,
         "coherency-size": 64
      }
   ]
}`,
			expectedError:  false,
			expectedLength: 2,
			expectedL3: lscpuCacheEntry{
				Name:          "L3",
				OneSize:       "320M",
				AllSize:       "640M",
				Ways:          20,
				Type:          "Unified",
				Level:         3,
				Sets:          262144,
				PhyLine:       1,
				CoherencySize: 64,
			},
		},
		{
			name: "Legacy format with string fields",
			input: `{
   "caches": [
      {
         "name": "L1d",
         "one-size": "48K",
         "all-size": "6M",
         "ways": "12",
         "type": "Data",
         "level": "1",
         "sets": "64",
         "phy-line": "1",
         "coherency-size": "64"
      },
      {
         "name": "L3",
         "one-size": "320M",
         "all-size": "640M",
         "ways": "20",
         "type": "Unified",
         "level": "3",
         "sets": "262144",
         "phy-line": "1",
         "coherency-size": "64"
      }
   ]
}`,
			expectedError:  false,
			expectedLength: 2,
			expectedL3: lscpuCacheEntry{
				Name:          "L3",
				OneSize:       "320M",
				AllSize:       "640M",
				Ways:          20,
				Type:          "Unified",
				Level:         3,
				Sets:          262144,
				PhyLine:       1,
				CoherencySize: 64,
			},
		},
		{
			name: "Legacy format with some empty string fields",
			input: `{
   "caches": [
      {
         "name": "L3",
         "one-size": "320M",
         "all-size": "640M",
         "ways": "20",
         "type": "Unified",
         "level": "3",
         "sets": "",
         "phy-line": "",
         "coherency-size": "64"
      }
   ]
}`,
			expectedError:  false,
			expectedLength: 1,
			expectedL3: lscpuCacheEntry{
				Name:          "L3",
				OneSize:       "320M",
				AllSize:       "640M",
				Ways:          20,
				Type:          "Unified",
				Level:         3,
				Sets:          0, // empty string converts to 0
				PhyLine:       0, // empty string converts to 0
				CoherencySize: 64,
			},
		},
		{
			name: "Legacy format with invalid number strings",
			input: `{
   "caches": [
      {
         "name": "L3",
         "one-size": "320M",
         "all-size": "640M",
         "ways": "invalid",
         "type": "Unified",
         "level": "3",
         "sets": "not_a_number",
         "phy-line": "1",
         "coherency-size": "64"
      }
   ]
}`,
			expectedError:  false,
			expectedLength: 1,
			expectedL3: lscpuCacheEntry{
				Name:          "L3",
				OneSize:       "320M",
				AllSize:       "640M",
				Ways:          0, // invalid string converts to 0
				Type:          "Unified",
				Level:         3,
				Sets:          0, // invalid string converts to 0
				PhyLine:       1,
				CoherencySize: 64,
			},
		},
		{
			name:          "Empty input",
			input:         "",
			expectedError: true,
		},
		{
			name:          "Invalid JSON",
			input:         `{"caches": [invalid json}`,
			expectedError: true,
		},
		{
			name:           "Empty caches array",
			input:          `{"caches": []}`,
			expectedError:  false,
			expectedLength: 0,
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
