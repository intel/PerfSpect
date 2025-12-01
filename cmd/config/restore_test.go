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
All-Core Max Frequency:          3.2GHz           --core-max <GHz>
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
	assert.Equal(t, "3.2", valueMap["core-max"])
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
			stderrOutput: "configuration update complete: set gov to powersave, set c1-demotion to disable, set tdp to 350, set c6 to enable, set epb to 0, set core-max to 3.2, set cores to 86, set elc to default, failed to set pref-l2hw to enable, set pref-dcuhw to enable, set pref-llc to disable, set pref-aop to enable, set pref-l2adj to enable, set uncore-max-compute to 2.2, failed to set llc to 336, set pref-dcunp to enable, set pref-homeless to enable, set pref-amp to enable, set pref-dcuip to enable, set pref-llcpp to enable, set uncore-max-io to 2.5, set uncore-min-compute to 0.8, set uncore-min-io to 0.8",
			flagValues: []flagValue{
				{flagName: "cores", value: "86"},
				{flagName: "llc", value: "336"},
				{flagName: "tdp", value: "350"},
				{flagName: "core-max", value: "3.2"},
				{flagName: "uncore-max-compute", value: "2.2"},
				{flagName: "uncore-min-compute", value: "0.8"},
				{flagName: "uncore-max-io", value: "2.5"},
				{flagName: "uncore-min-io", value: "0.8"},
				{flagName: "epb", value: "0"},
				{flagName: "gov", value: "powersave"},
				{flagName: "elc", value: "default"},
				{flagName: "pref-l2hw", value: "enable"},
				{flagName: "pref-l2adj", value: "enable"},
				{flagName: "pref-dcuhw", value: "enable"},
				{flagName: "pref-dcuip", value: "enable"},
				{flagName: "pref-dcunp", value: "enable"},
				{flagName: "pref-amp", value: "enable"},
				{flagName: "pref-llcpp", value: "enable"},
				{flagName: "pref-aop", value: "enable"},
				{flagName: "pref-homeless", value: "enable"},
				{flagName: "pref-llc", value: "disable"},
				{flagName: "c6", value: "enable"},
				{flagName: "c1-demotion", value: "disable"},
			},
			expectedOutput: []string{
				"✓ Set cores to 86",
				"✗ Failed to set llc to 336",
				"✓ Set tdp to 350",
				"✓ Set core-max to 3.2",
				"✓ Set uncore-max-compute to 2.2",
				"✓ Set uncore-min-compute to 0.8",
				"✓ Set uncore-max-io to 2.5",
				"✓ Set uncore-min-io to 0.8",
				"✓ Set epb to 0",
				"✓ Set gov to powersave",
				"✓ Set elc to default",
				"✗ Failed to set pref-l2hw to enable",
				"✓ Set pref-l2adj to enable",
				"✓ Set pref-dcuhw to enable",
				"✓ Set pref-dcuip to enable",
				"✓ Set pref-dcunp to enable",
				"✓ Set pref-amp to enable",
				"✓ Set pref-llcpp to enable",
				"✓ Set pref-aop to enable",
				"✓ Set pref-homeless to enable",
				"✓ Set pref-llc to disable",
				"✓ Set c6 to enable",
				"✓ Set c1-demotion to disable",
			},
		},
		{
			name:         "Empty stderr output",
			stderrOutput: "",
			flagValues: []flagValue{
				{flagName: "cores", value: "86"},
			},
			expectedOutput: []string{}, // nothing should be printed
		},
		{
			name: "Mixed success and error messages on separate lines",
			stderrOutput: "gnr                   ⣾  preparing target\n" +
				"gnr                   ⣽  configuration update complete: set cores to 86, failed to set llc to 336, set tdp to 350\n",
			flagValues: []flagValue{
				{flagName: "cores", value: "86"},
				{flagName: "llc", value: "336"},
				{flagName: "tdp", value: "350"},
			},
			expectedOutput: []string{
				"✓ Set cores to 86",
				"✗ Failed to set llc to 336",
				"✓ Set tdp to 350",
			},
		},
		{
			name:         "Flag name with multiple hyphens",
			stderrOutput: "set uncore-max-compute to 2.2, set uncore-min-io to 0.8",
			flagValues: []flagValue{
				{flagName: "uncore-max-compute", value: "2.2"},
				{flagName: "uncore-min-io", value: "0.8"},
			},
			expectedOutput: []string{
				"✓ Set uncore-max-compute to 2.2",
				"✓ Set uncore-min-io to 0.8",
			},
		},
		{
			name:         "No matching flags in output",
			stderrOutput: "some other message without flag updates",
			flagValues: []flagValue{
				{flagName: "cores", value: "86"},
			},
			expectedOutput: []string{
				"? cores: status unknown",
			},
		},
		{
			name:         "Flag with underscore and numbers",
			stderrOutput: "set pref_test123 to enable, failed to set flag_456 to disable",
			flagValues: []flagValue{
				{flagName: "pref_test123", value: "enable"},
				{flagName: "flag_456", value: "disable"},
			},
			expectedOutput: []string{
				"✓ Set pref_test123 to enable",
				"✗ Failed to set flag_456 to disable",
			},
		},
		{
			name:         "Some flags updated, others not mentioned",
			stderrOutput: "set cores to 86, set tdp to 350",
			flagValues: []flagValue{
				{flagName: "cores", value: "86"},
				{flagName: "llc", value: "336"},
				{flagName: "tdp", value: "350"},
			},
			expectedOutput: []string{
				"✓ Set cores to 86",
				"? llc: status unknown",
				"✓ Set tdp to 350",
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
			io.Copy(&buf, r)
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
