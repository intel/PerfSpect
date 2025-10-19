package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"testing"
)

func TestAssignCardAndPort(t *testing.T) {
	tests := []struct {
		name     string
		nics     []nicInfo
		expected map[string]string // map of NIC name to expected "Card / Port"
	}{
		{
			name: "Two cards with two ports each",
			nics: []nicInfo{
				{Name: "eth2", Bus: "0000:32:00.0"},
				{Name: "eth3", Bus: "0000:32:00.1"},
				{Name: "eth0", Bus: "0000:c0:00.0"},
				{Name: "eth1", Bus: "0000:c0:00.1"},
			},
			expected: map[string]string{
				"eth2": "1 / 1",
				"eth3": "1 / 2",
				"eth0": "2 / 1",
				"eth1": "2 / 2",
			},
		},
		{
			name: "Single card with four ports",
			nics: []nicInfo{
				{Name: "eth0", Bus: "0000:19:00.0"},
				{Name: "eth1", Bus: "0000:19:00.1"},
				{Name: "eth2", Bus: "0000:19:00.2"},
				{Name: "eth3", Bus: "0000:19:00.3"},
			},
			expected: map[string]string{
				"eth0": "1 / 1",
				"eth1": "1 / 2",
				"eth2": "1 / 3",
				"eth3": "1 / 4",
			},
		},
		{
			name: "Three different cards",
			nics: []nicInfo{
				{Name: "eth0", Bus: "0000:19:00.0"},
				{Name: "eth1", Bus: "0000:1a:00.0"},
				{Name: "eth2", Bus: "0000:1b:00.0"},
			},
			expected: map[string]string{
				"eth0": "1 / 1",
				"eth1": "2 / 1",
				"eth2": "3 / 1",
			},
		},
		{
			name: "Empty bus address should not assign card/port",
			nics: []nicInfo{
				{Name: "eth0", Bus: ""},
			},
			expected: map[string]string{
				"eth0": " / ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assignCardAndPort(tt.nics)
			for _, nic := range tt.nics {
				expected := tt.expected[nic.Name]
				actual := nic.Card + " / " + nic.Port
				if actual != expected {
					t.Errorf("NIC %s: expected %q, got %q", nic.Name, expected, actual)
				}
			}
		})
	}
}

func TestExtractFunction(t *testing.T) {
	tests := []struct {
		busAddr  string
		expected int
	}{
		{"0000:32:00.0", 0},
		{"0000:32:00.1", 1},
		{"0000:32:00.3", 3},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.busAddr, func(t *testing.T) {
			result := extractFunction(tt.busAddr)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}
