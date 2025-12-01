package config

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"os"
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
