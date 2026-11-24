// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package workflow

import (
	"os"
	"path/filepath"
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
		{
			name:     "Name referring to the current directory",
			input:    ".",
			expected: "_",
		},
		{
			name:     "Name referring to the parent directory",
			input:    "..",
			expected: "__",
		},
		{
			name:     "Name attempting path traversal",
			input:    "../../etc",
			expected: ".._.._etc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeTargetName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestValidateTargetHost confirms the host name rules. A host name becomes an argument of the local
// ssh process and, when a target is not named, a component of the target's output path, so a leading
// dash and a path separator must both be rejected. See validateTargetHost for the reasoning.
func TestValidateTargetHost(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "Empty host", input: "", wantErr: false},
		{name: "Host name", input: "my-host.example.com", wantErr: false},
		{name: "IPv4 address", input: "192.168.1.1", wantErr: false},
		{name: "Single character host", input: "h", wantErr: false},
		{name: "Host beginning with a dash", input: "-host", wantErr: true},
		{name: "Host ending with a dash", input: "host-", wantErr: true},
		{name: "Host with a path separator", input: "../../etc", wantErr: true},
		{name: "Host with a space", input: "host name", wantErr: true},
		{name: "Host with a semicolon", input: "host;name", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTargetHost(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTargetFromFile(t *testing.T) {
	tests := []struct {
		name    string
		input   targetFromYAML
		wantErr bool
	}{
		{
			name:    "Valid target",
			input:   targetFromYAML{Name: "target1", Host: "host1", Port: "22", User: "user1"},
			wantErr: false,
		},
		{
			name:    "Valid target with only a host",
			input:   targetFromYAML{Host: "host1"},
			wantErr: false,
		},
		{
			name:    "Missing host",
			input:   targetFromYAML{Name: "target1", User: "user1"},
			wantErr: true,
		},
		{
			name:    "Port is not a number",
			input:   targetFromYAML{Host: "host1", Port: "notanumber"},
			wantErr: true,
		},
		{
			name:    "Port is out of range",
			input:   targetFromYAML{Host: "host1", Port: "65536"},
			wantErr: true,
		},
		{
			name:    "Key file does not exist",
			input:   targetFromYAML{Host: "host1", Key: "/no/such/key/file"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTargetFromFile(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestGetTargetsFromFileRejectsInvalidTargets confirms that an entry with invalid connection details
// yields a target error rather than a usable target, and that the returned targets and errors stay
// parallel, which the callers of GetTargets require.
func TestGetTargetsFromFileRejectsInvalidTargets(t *testing.T) {
	tempDir := t.TempDir()
	// every entry is invalid, so that no connection is attempted and the test cannot hang on name
	// resolution; the second entry is unnamed, so its name comes from its host
	yaml := `targets:
  - name: target1
    host: "host name with a space"
    port:
    user:
    key:
    pwd:
  - host: host2
    port: 65536
`
	targetsFilePath := filepath.Join(tempDir, "targets.yaml")
	if err := os.WriteFile(targetsFilePath, []byte(yaml), 0600); err != nil {
		t.Fatalf("failed to write targets file: %v", err)
	}

	targets, targetErrs, err := getTargetsFromFile(targetsFilePath, tempDir)
	assert.NoError(t, err)
	// the targets and their errors remain parallel, as the caller requires
	assert.Len(t, targets, 2)
	assert.Len(t, targetErrs, 2)
	// both targets are rejected, and both keep a usable name for display
	assert.Error(t, targetErrs[0])
	assert.Equal(t, "target1", targets[0].GetName())
	assert.Error(t, targetErrs[1])
	assert.Equal(t, "host2", targets[1].GetName())
}
