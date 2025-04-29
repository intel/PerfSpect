package util

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"slices"
	"testing"
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
