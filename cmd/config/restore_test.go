package config

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfigFile(t *testing.T) {
	// create a temporary config file
	content := `Configuration
=============
Cores per Socket:                86               --cores <N>
L3 Cache:                        336M             --llc <MB>
Package Power / TDP:             350W             --tdp <Watts>
Core SSE Frequency:              1-44/3.6, 45-52/3.5, 53-60/3.4, 61-72/3.2, 73-76/3.1, 77-86/3.0  --core-max <GHz>
Uncore Max Frequency (Compute):  2.2GHz           --uncore-max-compute <GHz>
Energy Performance Bias:         Performance (0)  --epb <0-15>
Energy Performance Preference:   inconsistent     --epp <0-255>
Scaling Governor:                powersave        --gov <performance|powersave>
L2 HW prefetcher:                Enabled          --pref-l2hw <enable|disable>
C6:                              Disabled         --c6 <enable|disable>
`

	tmpFile, err := os.CreateTemp("", "config_test_*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()

	// parse the file
	flagValues, err := parseConfigFile(tmpFile.Name())
	require.NoError(t, err)

	// convert slice to map for easier testing
	valueMap := make(map[string]string)
	for _, fv := range flagValues {
		valueMap[fv.flagName] = fv.value
	}

	// verify expected values
	assert.Equal(t, "86", valueMap["cores"])
	assert.Equal(t, "336", valueMap["llc"])
	assert.Equal(t, "350", valueMap["tdp"])
	// verify core-max with buckets is converted to core-sse-freq-buckets
	assert.Equal(t, "1-44/3.6, 45-52/3.5, 53-60/3.4, 61-72/3.2, 73-76/3.1, 77-86/3.0", valueMap["core-sse-freq-buckets"])
	assert.Equal(t, "2.2", valueMap["uncore-max-compute"])
	assert.Equal(t, "0", valueMap["epb"])
	assert.Equal(t, "powersave", valueMap["gov"])
	assert.Equal(t, "enable", valueMap["pref-l2hw"])
	assert.Equal(t, "disable", valueMap["c6"])

	// verify inconsistent EPP was skipped
	_, exists := valueMap["epp"]
	assert.False(t, exists, "EPP with 'inconsistent' value should be skipped")

	// verify order is preserved
	assert.Equal(t, "cores", flagValues[0].flagName)
	assert.Equal(t, "llc", flagValues[1].flagName)
	assert.Equal(t, "tdp", flagValues[2].flagName)
}

func TestConvertValue(t *testing.T) {
	tests := []struct {
		name      string
		flagName  string
		rawValue  string
		expected  string
		shouldErr bool
	}{
		{"LLC with M suffix", "llc", "336M", "336", false},
		{"LLC with MB suffix", "llc", "336MB", "336", false},
		{"TDP with W suffix", "tdp", "350W", "350", false},
		{"Frequency with GHz suffix", "core-max", "3.2GHz", "3.2", false},
		{"EPB with parentheses", "epb", "Performance (0)", "0", false},
		{"EPB with text and parentheses", "epb", "Best Performance (15)", "15", false},
		{"Governor lowercase", "gov", "performance", "performance", false},
		{"Prefetcher enabled", "pref-l2hw", "Enabled", "enable", false},
		{"Prefetcher disabled", "pref-l2hw", "Disabled", "disable", false},
		{"C6 enabled", "c6", "Enabled", "enable", false},
		{"ELC lowercase", "elc", "default", "default", false},
		{"ELC capitalized", "elc", "Default", "default", false},
		{"Core SSE freq buckets", "core-sse-freq-buckets", "1-44/3.6, 45-52/3.5, 53-60/3.4", "1-44/3.6, 45-52/3.5, 53-60/3.4", false},
		{"Core SSE freq buckets full", "core-sse-freq-buckets", "1-44/3.6, 45-52/3.5, 53-60/3.4, 61-72/3.2, 73-76/3.1, 77-86/3.0", "1-44/3.6, 45-52/3.5, 53-60/3.4, 61-72/3.2, 73-76/3.1, 77-86/3.0", false},
		{"Core SSE freq buckets invalid", "core-sse-freq-buckets", "invalid-format", "", true},
		{"Inconsistent value", "epp", "inconsistent", "", true},
		{"Unknown flag", "unknown-flag", "value", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertValue(tt.flagName, tt.rawValue)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseNumericWithUnit(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		units     []string
		expected  string
		shouldErr bool
	}{
		{"GHz unit", "3.2GHz", []string{"GHz"}, "3.2", false},
		{"MB unit", "336MB", []string{"M", "MB"}, "336", false},
		{"M unit", "336M", []string{"M", "MB"}, "336", false},
		{"W unit", "350W", []string{"W"}, "350", false},
		{"No unit but valid number", "350", []string{"W"}, "350", false},
		{"Invalid number", "abc", []string{"W"}, "", true},
		{"Missing unit", "350", []string{}, "350", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseNumericWithUnit(tt.value, tt.units...)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseParenthesizedNumber(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		expected  string
		shouldErr bool
	}{
		{"Simple parentheses", "Performance (0)", "0", false},
		{"Multiple words", "Best Performance (15)", "15", false},
		{"Two digit number", "Some Text (255)", "255", false},
		{"No parentheses", "Performance", "", true},
		{"Empty parentheses", "Performance ()", "", true},
		{"Non-numeric", "Performance (abc)", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseParenthesizedNumber(tt.value)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseEnableDisableOrOption(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		validOptions []string
		expected     string
		shouldErr    bool
	}{
		{"Enabled to enable", "Enabled", []string{"enable", "disable"}, "enable", false},
		{"Disabled to disable", "Disabled", []string{"enable", "disable"}, "disable", false},
		{"Lowercase enable", "enable", []string{"enable", "disable"}, "enable", false},
		{"Performance option", "performance", []string{"performance", "powersave"}, "performance", false},
		{"Powersave option", "powersave", []string{"performance", "powersave"}, "powersave", false},
		{"Invalid option", "invalid", []string{"enable", "disable"}, "", true},
		{"Case insensitive", "ENABLED", []string{"enable", "disable"}, "enable", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseEnableDisableOrOption(tt.value, tt.validOptions)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseAndPresentResults(t *testing.T) {
	tests := []struct {
		name           string
		stderrOutput   string
		flagValues     []flagValue
		expectedOutput []string // lines we expect to see in output
	}{
		{
			name:         "Example from function header comment",
			stderrOutput: "configuration update complete: set cores to 86, set llc to 336, set tdp to 350, set core-sse-freq-buckets to 1-44/3.6, 45-52/3.5, 53-60/3.4, 61-72/3.2, 73-76/3.1, 77-86/3.0, set epb to 6, set epp to 128, set gov to powersave, set elc to default, set uncore-max-compute to 2.2, set uncore-min-compute to 0.8, set uncore-max-io to 2.5, set uncore-min-io to 0.8, set pref-l2hw to enable, set pref-l2adj to enable, set pref-dcuhw to enable, set pref-dcuip to enable, set pref-dcunp to enable, set pref-amp to enable, set pref-llcpp to enable, set pref-aop to enable, set pref-homeless to enable, set pref-llc to disable, set c6 to enable, set c1-demotion to disable",
			flagValues: []flagValue{
				{fieldName: "Cores per Socket", flagName: "cores", value: "86"},
				{fieldName: "L3 Cache", flagName: "llc", value: "336"},
				{fieldName: "Package Power / TDP", flagName: "tdp", value: "350"},
				{fieldName: "Core SSE Frequency", flagName: "core-sse-freq-buckets", value: "1-44/3.6, 45-52/3.5, 53-60/3.4, 61-72/3.2, 73-76/3.1, 77-86/3.0"},
				{fieldName: "Uncore Max Frequency (Compute)", flagName: "uncore-max-compute", value: "2.2"},
				{fieldName: "Uncore Min Frequency (Compute)", flagName: "uncore-min-compute", value: "0.8"},
				{fieldName: "Uncore Max Frequency (I/O)", flagName: "uncore-max-io", value: "2.5"},
				{fieldName: "Uncore Min Frequency (I/O)", flagName: "uncore-min-io", value: "0.8"},
				{fieldName: "Energy Performance Bias", flagName: "epb", value: "6"},
				{fieldName: "Energy Performance Preference", flagName: "epp", value: "128"},
				{fieldName: "Scaling Governor", flagName: "gov", value: "powersave"},
				{fieldName: "Efficiency Latency Control", flagName: "elc", value: "default"},
				{fieldName: "L2 HW prefetcher", flagName: "pref-l2hw", value: "enable"},
				{fieldName: "L2 Adj prefetcher", flagName: "pref-l2adj", value: "enable"},
				{fieldName: "DCU HW prefetcher", flagName: "pref-dcuhw", value: "enable"},
				{fieldName: "DCU IP prefetcher", flagName: "pref-dcuip", value: "enable"},
				{fieldName: "DCU NP prefetcher", flagName: "pref-dcunp", value: "enable"},
				{fieldName: "AMP prefetcher", flagName: "pref-amp", value: "enable"},
				{fieldName: "LLCPP prefetcher", flagName: "pref-llcpp", value: "enable"},
				{fieldName: "AOP prefetcher", flagName: "pref-aop", value: "enable"},
				{fieldName: "Homeless prefetcher", flagName: "pref-homeless", value: "enable"},
				{fieldName: "LLC prefetcher", flagName: "pref-llc", value: "disable"},
				{fieldName: "C6", flagName: "c6", value: "enable"},
				{fieldName: "C1 Demotion", flagName: "c1-demotion", value: "disable"},
			},
			expectedOutput: []string{
				"✓ Cores per Socket",
				"✓ L3 Cache",
				"✓ Package Power / TDP",
				"✓ Core SSE Frequency",
				"✓ Uncore Max Frequency (Compute)",
				"✓ Uncore Min Frequency (Compute)",
				"✓ Uncore Max Frequency (I/O)",
				"✓ Uncore Min Frequency (I/O)",
				"✓ Energy Performance Bias",
				"✓ Energy Performance Preference",
				"✓ Scaling Governor",
				"✓ Efficiency Latency Control",
				"✓ L2 HW prefetcher",
				"✓ L2 Adj prefetcher",
				"✓ DCU HW prefetcher",
				"✓ DCU IP prefetcher",
				"✓ DCU NP prefetcher",
				"✓ AMP prefetcher",
				"✓ LLCPP prefetcher",
				"✓ AOP prefetcher",
				"✓ Homeless prefetcher",
				"✓ LLC prefetcher",
				"✓ C6",
				"✓ C1 Demotion",
			},
		},
		{
			name:         "Empty stderr output",
			stderrOutput: "",
			flagValues: []flagValue{
				{fieldName: "Cores per Socket", flagName: "cores", value: "86"},
			},
			expectedOutput: []string{}, // nothing should be printed
		},
		{
			name: "Mixed success and error messages on separate lines",
			stderrOutput: "gnr                   ⣾  preparing target\n" +
				"gnr                   ⣽  configuration update complete: set cores to 86, failed to set llc to 336, set tdp to 350\n",
			flagValues: []flagValue{
				{fieldName: "Cores per Socket", flagName: "cores", value: "86"},
				{fieldName: "L3 Cache", flagName: "llc", value: "336"},
				{fieldName: "Package Power / TDP", flagName: "tdp", value: "350"},
			},
			expectedOutput: []string{
				"✓ Cores per Socket",
				"✗ L3 Cache",
				"✓ Package Power / TDP",
			},
		},
		{
			name:         "Flag name with multiple hyphens",
			stderrOutput: "set uncore-max-compute to 2.2, set uncore-min-io to 0.8",
			flagValues: []flagValue{
				{fieldName: "Uncore Max Frequency (Compute)", flagName: "uncore-max-compute", value: "2.2"},
				{fieldName: "Uncore Min Frequency (I/O)", flagName: "uncore-min-io", value: "0.8"},
			},
			expectedOutput: []string{
				"✓ Uncore Max Frequency (Compute)",
				"✓ Uncore Min Frequency (I/O)",
			},
		},
		{
			name:         "No matching flags in output",
			stderrOutput: "some other message without flag updates",
			flagValues: []flagValue{
				{fieldName: "Cores per Socket", flagName: "cores", value: "86"},
			},
			expectedOutput: []string{
				"? Cores per Socket",
			},
		},
		{
			name:         "Flag with underscore and numbers",
			stderrOutput: "set pref_test123 to enable, failed to set flag_456 to disable",
			flagValues: []flagValue{
				{fieldName: "Pref Test", flagName: "pref_test123", value: "enable"},
				{fieldName: "Flag 456", flagName: "flag_456", value: "disable"},
			},
			expectedOutput: []string{
				"✓ Pref Test",
				"✗ Flag 456",
			},
		},
		{
			name:         "Core SSE freq buckets with commas in value",
			stderrOutput: "set core-sse-freq-buckets to 1-44/3.6, 45-52/3.5, 53-60/3.4, 61-72/3.2, 73-76/3.1, 77-86/3.0",
			flagValues: []flagValue{
				{fieldName: "Core SSE Frequency", flagName: "core-sse-freq-buckets", value: "1-44/3.6, 45-52/3.5, 53-60/3.4, 61-72/3.2, 73-76/3.1, 77-86/3.0"},
			},
			expectedOutput: []string{
				"✓ Core SSE Frequency",
			},
		},
		{
			name:         "Some flags updated, others not mentioned",
			stderrOutput: "set cores to 86, set tdp to 350",
			flagValues: []flagValue{
				{fieldName: "Cores per Socket", flagName: "cores", value: "86"},
				{fieldName: "L3 Cache", flagName: "llc", value: "336"},
				{fieldName: "Package Power / TDP", flagName: "tdp", value: "350"},
			},
			expectedOutput: []string{
				"✓ Cores per Socket",
				"? L3 Cache",
				"✓ Package Power / TDP",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Call the function
			parseAndPresentResults(tt.stderrOutput, tt.flagValues)

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			// Verify expected output
			if len(tt.expectedOutput) == 0 {
				// Should be empty or just whitespace
				if strings.TrimSpace(output) != "" {
					t.Errorf("Expected no output, got: %q", output)
				}
				return
			}

			// Check that each expected line is in the output
			for _, expected := range tt.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nFull output:\n%s", expected, output)
				}
			}

			// Verify the output contains "Configuration Results:" header
			if !strings.Contains(output, "Configuration Results:") {
				t.Errorf("Expected output to contain 'Configuration Results:' header")
			}
		})
	}
}
