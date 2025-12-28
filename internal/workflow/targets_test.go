package workflow

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFieldFromDfpOutput(t *testing.T) {
	tests := []struct {
		name        string
		dfOutput    string
		fieldName   string
		expected    string
		expectError bool
	}{
		{
			name: "Valid field extraction",
			dfOutput: `Filesystem     1024-blocks     Used  Available Capacity Mounted on
/dev/sda2       1858388360 17247372 1747419536       1% /`,
			fieldName:   "Available",
			expected:    "1747419536",
			expectError: false,
		},
		{
			name: "Field not found",
			dfOutput: `Filesystem     1024-blocks     Used  Available Capacity Mounted on
/dev/sda2       1858388360 17247372 1747419536       1% /`,
			fieldName:   "NonExistentField",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Invalid df output format",
			dfOutput:    `Filesystem     1024-blocks     Used  Available Capacity Mounted on`,
			fieldName:   "Available",
			expected:    "",
			expectError: true,
		},
		{
			name: "Field index out of range",
			dfOutput: `Filesystem     1024-blocks     Used  Available Capacity Mounted on
/dev/sda2       1858388360 17247372`,
			fieldName:   "Capacity",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Empty df output",
			dfOutput:    ``,
			fieldName:   "Available",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fieldFromDfpOutput(tt.dfOutput, tt.fieldName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
func TestParseMountOutput(t *testing.T) {
	tests := []struct {
		name        string
		mountOutput string
		expected    []mountRecord
		expectError bool
	}{
		{
			name: "Valid mount output",
			mountOutput: `sysfs on /sys type sysfs (rw,nosuid,nodev,noexec,relatime)
tmpfs on /run type tmpfs (rw,nosuid,nodev,mode=755)`,
			expected: []mountRecord{
				{
					fileSystem: "sysfs",
					mountPoint: "/sys",
					typeName:   "sysfs",
					options:    []string{"rw", "nosuid", "nodev", "noexec", "relatime"},
				},
				{
					fileSystem: "tmpfs",
					mountPoint: "/run",
					typeName:   "tmpfs",
					options:    []string{"rw", "nosuid", "nodev", "mode=755"},
				},
			},
			expectError: false,
		},
		{
			name: "Invalid mount output format",
			mountOutput: `invalid output line
tmpfs on /run type tmpfs (rw,nosuid,nodev,mode=755)`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "Empty mount output",
			mountOutput: ``,
			expected:    nil,
			expectError: false,
		},
		{
			name: "Unexpected format in one line",
			mountOutput: `sysfs on /sys type sysfs (rw,nosuid,nodev,noexec,relatime)
invalid line format`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "Single valid mount record",
			mountOutput: `proc on /proc type proc (rw,nosuid,nodev,noexec,relatime)`,
			expected: []mountRecord{
				{
					fileSystem: "proc",
					mountPoint: "/proc",
					typeName:   "proc",
					options:    []string{"rw", "nosuid", "nodev", "noexec", "relatime"},
				},
			},
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseMountOutput(tt.mountOutput)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
func TestSanitizeTargetName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Valid name with allowed characters",
			input:    "valid_name-123.txt",
			expected: "valid_name-123.txt",
		},
		{
			name:     "Name with invalid characters",
			input:    "invalid@name#123!",
			expected: "invalid_name_123_",
		},
		{
			name:     "Name with spaces",
			input:    "name with spaces",
			expected: "name_with_spaces",
		},
		{
			name:     "Empty name",
			input:    "",
			expected: "",
		},
		{
			name:     "Name with only invalid characters",
			input:    "@#$%^&*()",
			expected: "_________",
		},
		{
			name:     "Name with mixed valid and invalid characters",
			input:    "valid@name#123!.txt",
			expected: "valid_name_123_.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeTargetName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
