package util

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

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
func TestIsValidHex(t *testing.T) {
	tests := []struct {
		hexStr   string
		expected bool
	}{
		{"0x1a2b3c", true},  // Valid hex with "0x" prefix
		{"0X1A2B3C", true},  // Valid hex with "0X" prefix
		{"1a2b3c", true},    // Valid hex without prefix
		{"1A2B3C", true},    // Valid uppercase hex without prefix
		{"0x", false},       // Invalid hex, only prefix
		{"", false},         // Empty string
		{"0xGHIJKL", false}, // Invalid hex with non-hex characters
		{"GHIJKL", false},   // Invalid hex without prefix
		{"12345", true},     // Valid numeric hex
		{"0x12345", true},   // Valid numeric hex with
		{" 12345 ", false},  // Invalid hex with spaces
	}

	for _, test := range tests {
		result := IsValidHex(test.hexStr)
		if result != test.expected {
			t.Errorf("expected %v, got %v for hex string %s", test.expected, result, test.hexStr)
		}
	}
}
func TestHexToIntList(t *testing.T) {
	tests := []struct {
		hexStr   string
		expected []int
		err      bool
	}{
		{"0x1a2b3c", []int{26, 43, 60}, false}, // Valid hex with "0x" prefix
		{"1a2b3c", []int{26, 43, 60}, false},   // Valid hex without prefix
		{"0X1A2B3C", []int{26, 43, 60}, false}, // Valid hex with "0X" prefix
		{"1A2B3C", []int{26, 43, 60}, false},   // Valid uppercase hex without prefix
		{"0x123", []int{1, 35}, false},         // Valid hex with odd length
		{"123", []int{1, 35}, false},           // Valid hex without prefix and odd length
		{"0x", nil, true},                      // Invalid hex, only prefix
		{"", nil, true},                        // Empty string
		{"0xGHIJKL", nil, true},                // Invalid hex with non-hex characters
		{"GHIJKL", nil, true},                  // Invalid hex without prefix
		{"12345", []int{1, 35, 69}, false},     // Valid numeric hex
		{"0x12345", []int{1, 35, 69}, false},   // Valid numeric hex with prefix
		{" 12345 ", nil, true},                 // Invalid hex with spaces
	}

	for _, test := range tests {
		result, err := HexToIntList(test.hexStr)
		if (err != nil) != test.err {
			t.Errorf("expected error: %v, got: %v for hex string %s", test.err, err != nil, test.hexStr)
		}
		if !test.err && !slices.Equal(result, test.expected) {
			t.Errorf("expected %v, got %v for hex string %s", test.expected, result, test.hexStr)
		}
	}
}
func TestIntRangeToIntList(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
		err      bool
	}{
		{"1-5", []int{1, 2, 3, 4, 5}, false},            // Valid range
		{"10-15", []int{10, 11, 12, 13, 14, 15}, false}, // Valid range
		{"5-5", []int{5}, false},                        // Single value range
		{"", []int{}, true},                             // Empty input
		{"5-3", nil, true},                              // Invalid range (start > end)
		{"abc-def", nil, true},                          // Invalid input format
		{"1-", nil, true},                               // Missing end value
		{"-5", nil, true},                               // Missing start value
		{"1-5-10", nil, true},                           // Invalid format with extra dash
		{"1-abc", nil, true},                            // Invalid end value
		{"abc-5", nil, true},                            // Invalid start value
		{"3", []int{3}, false},                          // Single value without range
	}

	for _, test := range tests {
		result, err := IntRangeToIntList(test.input)
		if (err != nil) != test.err {
			t.Errorf("expected error: %v, got: %v for input %s, err: %v", test.err, err != nil, test.input, err)
		}
		if !test.err && !slices.Equal(result, test.expected) {
			t.Errorf("expected %v, got %v for input %s", test.expected, result, test.input)
		}
	}
}
func TestSelectiveIntRangeToIntList(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
		err      bool
	}{
		{"1-3,5,7-9", []int{1, 2, 3, 5, 7, 8, 9}, false},             // Valid mixed ranges and single values
		{"10-12,15,20-22", []int{10, 11, 12, 15, 20, 21, 22}, false}, // Valid mixed ranges
		{"5", []int{5}, false},                                       // Single value
		{"1-3,5-5,7", []int{1, 2, 3, 5, 7}, false},                   // Mixed ranges with single value range
		{"", nil, true},            // Empty input
		{"1-3,abc,7-9", nil, true}, // Invalid input with non-numeric value
		{"1-3,5-2,7-9", nil, true}, // Invalid range (start > end)
		{"1-3,,7-9", nil, true},    // Invalid format with empty segment
		{"1-3,7-9-", nil, true},    // Invalid format with trailing dash
		{"1-3,7-abc", nil, true},   // Invalid range with non-numeric end
	}

	for _, test := range tests {
		result, err := SelectiveIntRangeToIntList(test.input)
		if (err != nil) != test.err {
			t.Errorf("expected error: %v, got: %v for input %s, err: %v", test.err, err != nil, test.input, err)
		}
		if !test.err && !slices.Equal(result, test.expected) {
			t.Errorf("expected %v, got %v for input %s", test.expected, result, test.input)
		}
	}
}
func TestIntSliceToStringSlice(t *testing.T) {
	tests := []struct {
		input    []int
		expected []string
	}{
		{[]int{1, 2, 3}, []string{"1", "2", "3"}},                   // Simple case
		{[]int{-1, 0, 1}, []string{"-1", "0", "1"}},                 // Negative, zero, and positive integers
		{[]int{}, []string{}},                                       // Empty slice
		{[]int{123, 456, 789}, []string{"123", "456", "789"}},       // Larger numbers
		{[]int{-123, -456, -789}, []string{"-123", "-456", "-789"}}, // Negative larger numbers
	}

	for _, test := range tests {
		result := IntSliceToStringSlice(test.input)
		if !slices.Equal(result, test.expected) {
			t.Errorf("expected %v, got %v for input %v", test.expected, result, test.input)
		}
	}
}
func TestIsValidSemver(t *testing.T) {
	tests := []struct {
		version string
		valid   bool
	}{
		{"1.0.0", true},
		{"v1.0.0", true},
		{"1.2.3", true},
		{"0.1.0", true},
		{"1.2.3-alpha", true},
		{"1.2.3+build.1", true},
		{"1.2.3-alpha+build.1", true},
		{"1.2.3-alpha.1", true},
		{"1.2.3-0.3.7", true},
		{"1.2.3-x.7.z.92", true},
		{"1.2.3-rc.1+build.1", true},
		{"v1.2.3-rc.1+build.1", true},
		{"1.2", false},
		{"1.2.3.4", false},
		{"1.2.3-", false},
		{"1.2.3+", false},
		{"1.2.3-01", true},    // leading zero allowed in prerelease
		{"1.2.3-rc.01", true}, // leading zero allowed in prerelease
		{"1.2.3-rc..1", false},
		{"1.2.3-rc_1", false}, // underscore not allowed
		{"1.2.3-rc@1", false}, // @ not allowed
		{"v", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsValidSemver(tt.version); got != tt.valid {
			t.Errorf("IsValidSemver(%q) = %v, want %v", tt.version, got, tt.valid)
		}
	}
}
func TestRandUint(t *testing.T) {
	const max uint64 = 100
	const iterations = 1000

	// Test that returned value is always in [0, max)
	for range iterations {
		val := RandUint(max)
		if val >= max {
			t.Errorf("RandInt(%d) returned %d, want in [0, %d)", max, val, max)
		}
	}

	// Test that values are distributed (not always the same)
	seen := make(map[uint64]bool)
	for range iterations {
		val := RandUint(max)
		seen[val] = true
	}
	if len(seen) < int(max/2) {
		t.Errorf("RandInt(%d) seems not random enough, got only %d unique values", max, len(seen))
	}

	// Test with max = 1 (should always return 0)
	for range 10 {
		val := RandUint(1)
		if val != 0 {
			t.Errorf("RandInt(1) returned %d, want 0", val)
		}
	}

	// Test with max = 0 (should always return 0, but avoid panic)
	for range 10 {
		val := RandUint(0)
		if val != 0 {
			t.Errorf("RandInt(0) returned %d, want 0", val)
		}
	}
}
func TestGetChildren(t *testing.T) {
	// This test assumes that the current process has no child processes at the start.
	selfPid := os.Getpid()
	children, err := GetChildren(selfPid)
	if err != nil {
		t.Fatalf("GetChildren returned error: %v", err)
	}
	if len(children) != 0 {
		t.Errorf("expected no children, got %v", children)
	}

	// Now fork a child process (using os/exec) and check that it appears in the list.
	// We'll use a short-lived sleep process.
	proc, err := os.StartProcess("/bin/sleep", []string{"sleep", "1"}, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		t.Fatalf("failed to start child process: %v", err)
	}
	defer func() {
		_, _ = proc.Wait()
	}()

	// Give the child process a moment to start and appear in /proc
	found := false
	for range 10 {
		children, err = GetChildren(selfPid)
		if err != nil {
			t.Fatalf("GetChildren returned error: %v", err)
		}
		if slices.Contains(children, proc.Pid) {
			found = true
		}
		if found {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !found {
		t.Errorf("child process PID %d not found in GetChildren(%d): got %v", proc.Pid, selfPid, children)
	}
}
func TestInt64ToUint64(t *testing.T) {
	tests := []struct {
		name    string
		input   int64
		want    uint64
		wantErr bool
	}{
		{"positive value", 12345, 12345, false},
		{"zero", 0, 0, false},
		{"max int64", int64(^uint64(0) >> 1), uint64(^uint64(0) >> 1), false},
		{"negative value", -1, 0, true},
		{"min int64", int64(-1 << 63), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Int64ToUint64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Int64ToUint64(%d) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Int64ToUint64(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
func TestNumUint64Bits(t *testing.T) {
	tests := []struct {
		input    uint64
		expected int
	}{
		{0, 0},
		{1, 1},
		{2, 1},
		{3, 2},
		{4, 1},
		{7, 3},
		{8, 1},
		{15, 4},
		{16, 1},
		{255, 8},
		{256, 1},
		{1023, 10},
		{1024, 1},
		{0xfff, 12},
		{0xffff, 16},
		{uint64(^uint64(0)), 64}, // all bits set
	}

	for _, tt := range tests {
		got := NumUint64Bits(tt.input)
		if got != tt.expected {
			t.Errorf("NumUint64Bits(%d) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}
func TestUint64FromNumLowerBits(t *testing.T) {
	tests := []struct {
		numBits  int
		expected uint64
		wantErr  bool
	}{
		{0, 0, false},
		{1, 1, false},
		{2, 3, false},
		{3, 7, false},
		{4, 15, false},
		{8, 255, false},
		{16, 65535, false},
		{32, 4294967295, false},
		{63, 0x7FFFFFFFFFFFFFFF, false},
		{64, 0xFFFFFFFFFFFFFFFF, false},
		{-1, 0, true},
		{65, 0, true},
		{100, 0, true},
	}
	for _, tt := range tests {
		got, err := Uint64FromNumLowerBits(tt.numBits)
		if (err != nil) != tt.wantErr {
			t.Errorf("Uint64FromNumLowerBits(%d) error = %v, wantErr %v", tt.numBits, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.expected {
			t.Errorf("Uint64FromNumLowerBits(%d) = %d, want %d", tt.numBits, got, tt.expected)
		}
	}
}
func TestIsUint64BitSet(t *testing.T) {
	tests := []struct {
		name    string
		x       uint64
		bit     int
		want    bool
		wantErr bool
	}{
		{"bit 0 set", 1, 0, true, false},
		{"bit 1 set", 2, 1, true, false},
		{"bit 2 not set", 2, 2, false, false},
		{"bit 63 set", 1 << 63, 63, true, false},
		{"bit 63 not set", 1, 63, false, false},
		{"all bits set", ^uint64(0), 0, true, false},
		{"all bits set, bit 63", ^uint64(0), 63, true, false},
		{"bit out of range negative", 1, -1, false, true},
		{"bit out of range high", 1, 64, false, true},
		{"zero value, bit 0", 0, 0, false, false},
		{"zero value, bit 63", 0, 63, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsUint64BitSet(tt.x, tt.bit)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsUint64BitSet(%d, %d) error = %v, wantErr %v", tt.x, tt.bit, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsUint64BitSet(%d, %d) = %v, want %v", tt.x, tt.bit, got, tt.want)
			}
		})
	}
}
func TestMergeOrderedUnique(t *testing.T) {
	type testCase[T comparable] struct {
		name     string
		input    [][]T
		expected []T
	}

	stringTests := []testCase[string]{
		{
			name:     "empty input",
			input:    [][]string{},
			expected: []string{},
		},
		{
			name:     "single slice",
			input:    [][]string{{"a", "b", "c"}},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "two slices, no overlap",
			input:    [][]string{{"a", "b"}, {"c", "d"}},
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:     "two slices, some overlap",
			input:    [][]string{{"a", "b"}, {"b", "c"}},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "multiple slices, complex order",
			input:    [][]string{{"a", "b"}, {"b", "c", "d"}, {"d", "e", "f"}, {"a", "f", "g"}},
			expected: []string{"a", "b", "c", "d", "e", "f", "g"},
		},
		{
			name:     "insertion after previous",
			input:    [][]string{{"a", "b"}, {"a", "c", "b"}},
			expected: []string{"a", "c", "b"},
		},
		{
			name:     "all duplicates",
			input:    [][]string{{"a", "b"}, {"a", "b"}, {"a", "b"}},
			expected: []string{"a", "b"},
		},
		{
			name:     "empty inner slices",
			input:    [][]string{{}, {}, {}},
			expected: []string{},
		},
		{
			name:     "first slice empty, others non-empty",
			input:    [][]string{{}, {"a", "b"}, {"b", "c"}},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "second slice empty",
			input:    [][]string{{"a", "b"}, {}, {"c"}},
			expected: []string{"a", "b", "c"},
		},
	}

	intTests := []testCase[int]{
		{
			name:     "integers, no overlap",
			input:    [][]int{{1, 2}, {3, 4}},
			expected: []int{1, 2, 3, 4},
		},
		{
			name:     "integers, with overlap",
			input:    [][]int{{1, 2}, {2, 3, 4}, {4, 5}},
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name:     "integers, all duplicates",
			input:    [][]int{{1, 2}, {1, 2}, {1, 2}},
			expected: []int{1, 2},
		},
		{
			name:     "integers, insertion after previous",
			input:    [][]int{{1, 2}, {1, 3, 2}},
			expected: []int{1, 3, 2},
		},
	}

	for _, tc := range stringTests {
		t.Run("string/"+tc.name, func(t *testing.T) {
			got := MergeOrderedUnique(tc.input)
			if !slices.Equal(got, tc.expected) {
				t.Errorf("MergeOrderedUnique(%v) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}

	for _, tc := range intTests {
		t.Run("int/"+tc.name, func(t *testing.T) {
			got := MergeOrderedUnique(tc.input)
			if !slices.Equal(got, tc.expected) {
				t.Errorf("MergeOrderedUnique(%v) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}
func TestCreateFlatTGZ(t *testing.T) {
	// Create a temporary directory for test files and tarball
	tempDir := t.TempDir()

	// Create some test files with known content
	files := []string{}
	contents := []string{"hello world", "foo bar", "baz qux"}
	for i, content := range contents {
		filePath := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		files = append(files, filePath)
	}

	// Path for the tarball
	tarballPath := filepath.Join(tempDir, "test.tar.gz")

	// Call CreateFlatTGZ
	if err := CreateFlatTGZ(files, tarballPath); err != nil {
		t.Fatalf("CreateFlatTGZ failed: %v", err)
	}

	// Open and read the tarball, check contents
	tarball, err := os.Open(tarballPath)
	if err != nil {
		t.Fatalf("failed to open tarball: %v", err)
	}
	defer tarball.Close()

	gzipReader, err := gzip.NewReader(tarball)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	foundFiles := map[string]string{}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("error reading tarball: %v", err)
		}
		data, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("failed to read file from tarball: %v", err)
		}
		foundFiles[header.Name] = string(data)
	}

	// Check that all files are present and contents match
	for i, content := range contents {
		base := filepath.Base(files[i])
		got, ok := foundFiles[base]
		if !ok {
			t.Errorf("file %s not found in tarball", base)
		}
		if got != content {
			t.Errorf("file %s content mismatch: got %q, want %q", base, got, content)
		}
	}

	// Test error when file does not exist
	badTarball := filepath.Join(tempDir, "bad.tar.gz")
	err = CreateFlatTGZ([]string{filepath.Join(tempDir, "doesnotexist.txt")}, badTarball)
	if err == nil {
		t.Errorf("expected error for non-existent file, got nil")
	}

	// Test error when tarball path is invalid
	err = CreateFlatTGZ(files, "/invalid/path/to/tarball.tar.gz")
	if err == nil {
		t.Errorf("expected error for invalid tarball path, got nil")
	}
}
