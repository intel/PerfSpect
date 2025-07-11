package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"testing"
)

func TestGetCacheLscpuParts(t *testing.T) {
	tests := []struct {
		input         string
		wantSize      float64
		wantUnits     string
		wantInstances int
		wantErr       bool
	}{
		{
			input:         "32 MiB (1 instance)",
			wantSize:      32,
			wantUnits:     "MiB",
			wantInstances: 1,
			wantErr:       false,
		},
		{
			input:         "1.5 GiB (2 instances)",
			wantSize:      1.5,
			wantUnits:     "GiB",
			wantInstances: 2,
			wantErr:       false,
		},
		{
			input:         "512 KiB (4 instances)",
			wantSize:      512,
			wantUnits:     "KiB",
			wantInstances: 4,
			wantErr:       false,
		},
		{
			input:         "256 KiB",
			wantSize:      256,
			wantUnits:     "KiB",
			wantInstances: 1,
			wantErr:       false,
		},
		{
			input:         "2 MiB",
			wantSize:      2,
			wantUnits:     "MiB",
			wantInstances: 1,
			wantErr:       false,
		},
		{
			input:         "bad format string",
			wantSize:      0,
			wantUnits:     "",
			wantInstances: 0,
			wantErr:       true,
		},
		{
			input:         "12.5 MiB (notanumber instances)",
			wantSize:      0,
			wantUnits:     "",
			wantInstances: 0,
			wantErr:       true,
		},
		{
			input:         "",
			wantSize:      0,
			wantUnits:     "",
			wantInstances: 0,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		size, units, instances, err := getCacheLscpuParts(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("getCacheLscpuParts(%q) expected error, got none", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("getCacheLscpuParts(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if size != tt.wantSize {
			t.Errorf("getCacheLscpuParts(%q) size = %v, want %v", tt.input, size, tt.wantSize)
		}
		if units != tt.wantUnits {
			t.Errorf("getCacheLscpuParts(%q) units = %v, want %v", tt.input, units, tt.wantUnits)
		}
		if instances != tt.wantInstances {
			t.Errorf("getCacheLscpuParts(%q) instances = %v, want %v", tt.input, instances, tt.wantInstances)
		}
	}
}
func TestGetCacheMBLscpu(t *testing.T) {
	tests := []struct {
		name        string
		lscpuOutput string
		cacheRegex  string
		wantMB      float64
		wantErr     bool
	}{
		{
			name: "L3 cache in MiB, 1 socket",
			lscpuOutput: `
Socket(s):             1
L3 cache:              32 MiB (1 instance)
`,
			cacheRegex: `^L3 cache:\s*(.+)$`,
			wantMB:     32,
			wantErr:    false,
		},
		{
			name: "L3 cache in MiB, 2 sockets",
			lscpuOutput: `
Socket(s):             2
L3 cache:              64 MiB (2 instances)
`,
			cacheRegex: `^L3 cache:\s*(.+)$`,
			wantMB:     32,
			wantErr:    false,
		},
		{
			name: "L3 cache in GiB, 2 sockets",
			lscpuOutput: `
Socket(s):             2
L3 cache:              2 GiB (2 instances)
`,
			cacheRegex: `^L3 cache:\s*(.+)$`,
			wantMB:     1024,
			wantErr:    false,
		},
		{
			name: "L2 cache in KiB, 4 sockets",
			lscpuOutput: `
Socket(s):             4
L2 cache:              1024 KiB (4 instances)
`,
			cacheRegex: `^L2 cache:\s*(.+)$`,
			wantMB:     0.25,
			wantErr:    false,
		},
		{
			name: "Cache size not found",
			lscpuOutput: `
Socket(s):             1
`,
			cacheRegex: `^L3 cache:\s*(.+)$`,
			wantMB:     0,
			wantErr:    true,
		},
		{
			name: "Socket line missing",
			lscpuOutput: `
L3 cache:              32 MiB (1 instance)
`,
			cacheRegex: `^L3 cache:\s*(.+)$`,
			wantMB:     0,
			wantErr:    true,
		},
		{
			name: "Socket value not a number",
			lscpuOutput: `
Socket(s):             notanumber
L3 cache:              32 MiB (1 instance)
`,
			cacheRegex: `^L3 cache:\s*(.+)$`,
			wantMB:     0,
			wantErr:    true,
		},
		{
			name: "Unknown units",
			lscpuOutput: `
Socket(s):             1
L3 cache:              32 XYB (1 instance)
`,
			cacheRegex: `^L3 cache:\s*(.+)$`,
			wantMB:     0,
			wantErr:    true,
		},
		{
			name: "Cache size parse error",
			lscpuOutput: `
Socket(s):             1
L3 cache:              bad format string
`,
			cacheRegex: `^L3 cache:\s*(.+)$`,
			wantMB:     0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMB, err := getCacheMBLscpu(tt.lscpuOutput, tt.cacheRegex)
			if tt.wantErr {
				if err == nil {
					t.Errorf("getCacheMBLscpu() expected error, got none")
				}
				return
			}
			if err != nil {
				t.Errorf("getCacheMBLscpu() unexpected error: %v", err)
				return
			}
			if gotMB != tt.wantMB {
				t.Errorf("getCacheMBLscpu() = %v, want %v", gotMB, tt.wantMB)
			}
		})
	}
}
