// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package cpus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewX86Identifier(t *testing.T) {
	id := NewX86Identifier("6", "143", "4", "c0", "4")
	assert.Equal(t, "6", id.Family)
	assert.Equal(t, "143", id.Model)
	assert.Equal(t, "4", id.Stepping)
	assert.Equal(t, "c0", id.Capid4)
	assert.Equal(t, "4", id.Devices)
	assert.Equal(t, X86Architecture, id.Architecture)
}

func TestNewARMIdentifier(t *testing.T) {
	id := NewARMIdentifier("0x41", "0xd0c", "AWS Graviton2")
	assert.Equal(t, "0x41", id.Implementer)
	assert.Equal(t, "0xd0c", id.Part)
	assert.Equal(t, "AWS Graviton2", id.DmidecodePart)
	assert.Equal(t, ARMArchitecture, id.Architecture)
}

func TestNewCPUIdentifier(t *testing.T) {
	id := NewCPUIdentifier("6", "143", "4", "c0", "4", "0x41", "0xd0c", "AWS Graviton2", X86Architecture)
	assert.Equal(t, "6", id.Family)
	assert.Equal(t, "143", id.Model)
	assert.Equal(t, "4", id.Stepping)
	assert.Equal(t, "0x41", id.Implementer)
	assert.Equal(t, X86Architecture, id.Architecture)
}

func TestGetCPU_X86_ICX(t *testing.T) {
	// Test Ice Lake (ICX) - Family 6, Model 106
	id := NewX86Identifier("6", "106", "6", "", "")
	cpu, err := GetCPU(id)
	require.NoError(t, err)
	assert.Equal(t, UarchICX, cpu.MicroArchitecture)
	assert.Equal(t, 8, cpu.MemoryChannelCount)
	assert.Equal(t, 2, cpu.LogicalThreadCount)
}

func TestGetCPU_X86_SKX(t *testing.T) {
	// Test Skylake (SKX) - Family 6, Model 85
	id := NewX86Identifier("6", "85", "4", "", "")
	cpu, err := GetCPU(id)
	require.NoError(t, err)
	assert.Equal(t, UarchSKX, cpu.MicroArchitecture)
	assert.Equal(t, 6, cpu.MemoryChannelCount)
	assert.Equal(t, 2, cpu.LogicalThreadCount)
}

func TestGetCPU_ARM_Graviton2(t *testing.T) {
	// Test AWS Graviton2
	id := NewARMIdentifier("0x41", "0xd0c", "AWS Graviton2")
	cpu, err := GetCPU(id)
	require.NoError(t, err)
	assert.Equal(t, UarchGraviton2, cpu.MicroArchitecture)
}

func TestGetCPU_ARM_Graviton3(t *testing.T) {
	// Test AWS Graviton3
	id := NewARMIdentifier("0x41", "0xd40", "AWS Graviton3")
	cpu, err := GetCPU(id)
	require.NoError(t, err)
	assert.Equal(t, UarchGraviton3, cpu.MicroArchitecture)
}

func TestGetCPU_UnknownX86(t *testing.T) {
	// Test unknown x86 CPU
	id := NewX86Identifier("99", "999", "0", "", "")
	_, err := GetCPU(id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CPU match not found")
}

func TestGetCPU_UnknownARM(t *testing.T) {
	// Test unknown ARM CPU
	id := NewARMIdentifier("0xff", "0xfff", "Unknown")
	_, err := GetCPU(id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CPU match not found")
}

func TestGetCPU_NoArchitecture(t *testing.T) {
	// Test with insufficient information to determine architecture
	id := CPUIdentifier{}
	_, err := GetCPU(id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to determine CPU architecture")
}

func TestGetCPUByMicroArchitecture(t *testing.T) {
	tests := []struct {
		name          string
		uarch         string
		expectError   bool
		expectedUarch string
	}{
		{
			name:          "exact match - ICX",
			uarch:         UarchICX,
			expectError:   false,
			expectedUarch: UarchICX,
		},
		{
			name:          "exact match - SPR",
			uarch:         UarchSPR,
			expectError:   false,
			expectedUarch: UarchSPR,
		},
		{
			name:          "case insensitive - icx",
			uarch:         "icx",
			expectError:   false,
			expectedUarch: UarchICX,
		},
		{
			name:        "unknown microarchitecture",
			uarch:       "UNKNOWN",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu, err := GetCPUByMicroArchitecture(tt.uarch)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedUarch, cpu.MicroArchitecture)
			}
		})
	}
}

func TestIsIntelCPUFamily(t *testing.T) {
	tests := []struct {
		name     string
		family   int
		expected bool
	}{
		{"family 6", 6, true},
		{"family 19", 19, true},
		{"family 15", 15, false},
		{"family 0", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIntelCPUFamily(tt.family)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsIntelCPUFamilyStr(t *testing.T) {
	tests := []struct {
		name     string
		family   string
		expected bool
	}{
		{"family 6", "6", true},
		{"family 19", "19", true},
		{"family 15", "15", false},
		{"invalid", "invalid", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIntelCPUFamilyStr(tt.family)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSPRMicroArchitecture(t *testing.T) {
	tests := []struct {
		name          string
		capid4        string
		expectedUarch string
		expectError   bool
	}{
		{
			name:          "SPR XCC - bits 11",
			capid4:        "c0", // binary: 11000000, bits [7:6] = 11 (3)
			expectedUarch: UarchSPR_XCC,
			expectError:   false,
		},
		{
			name:          "SPR MCC - bits 01",
			capid4:        "40", // binary: 01000000, bits [7:6] = 01 (1)
			expectedUarch: UarchSPR_MCC,
			expectError:   false,
		},
		{
			name:          "SPR default - no capid4",
			capid4:        "",
			expectedUarch: UarchSPR,
			expectError:   false,
		},
		{
			name:          "SPR default - invalid capid4",
			capid4:        "invalid",
			expectedUarch: "",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uarch, err := getSPRMicroArchitecture(tt.capid4)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedUarch, uarch)
			}
		})
	}
}

func TestGetEMRMicroArchitecture(t *testing.T) {
	tests := []struct {
		name          string
		capid4        string
		expectedUarch string
		expectError   bool
	}{
		{
			name:          "EMR XCC - bits 11",
			capid4:        "c0",
			expectedUarch: UarchEMR_XCC,
			expectError:   false,
		},
		{
			name:          "EMR MCC - bits 01",
			capid4:        "40",
			expectedUarch: UarchEMR_MCC,
			expectError:   false,
		},
		{
			name:          "EMR default - no capid4",
			capid4:        "",
			expectedUarch: UarchEMR,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uarch, err := getEMRMicroArchitecture(tt.capid4)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedUarch, uarch)
			}
		})
	}
}

func TestGetGNRMicroArchitecture(t *testing.T) {
	tests := []struct {
		name          string
		devices       string
		expectedUarch string
	}{
		{"GNR X3 - 5 devices", "5", UarchGNR_X3},
		{"GNR X3 - 10 devices", "10", UarchGNR_X3},
		{"GNR X2 - 4 devices", "4", UarchGNR_X2},
		{"GNR X2 - 8 devices", "8", UarchGNR_X2},
		{"GNR X1 - 3 devices", "3", UarchGNR_X1},
		{"GNR X1 - 6 devices", "6", UarchGNR_X1},
		{"GNR default - no devices", "", UarchGNR},
		{"GNR default - invalid devices", "invalid", UarchGNR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uarch, err := getGNRMicroArchitecture(tt.devices)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedUarch, uarch)
		})
	}
}

func TestGetSRFMicroArchitecture(t *testing.T) {
	tests := []struct {
		name          string
		devices       string
		expectedUarch string
	}{
		{"SRF SP - 3 devices", "3", UarchSRF_SP},
		{"SRF SP - 6 devices", "6", UarchSRF_SP},
		{"SRF AP - 4 devices", "4", UarchSRF_AP},
		{"SRF AP - 8 devices", "8", UarchSRF_AP},
		{"SRF default - no devices", "", UarchSRF},
		{"SRF default - invalid devices", "invalid", UarchSRF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uarch, err := getSRFMicroArchitecture(tt.devices)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedUarch, uarch)
		})
	}
}

func TestGetCPU_SPR_WithCapid4(t *testing.T) {
	// Test SPR XCC with capid4
	id := NewX86Identifier("6", "143", "4", "c0", "")
	cpu, err := GetCPU(id)
	require.NoError(t, err)
	assert.Equal(t, UarchSPR_XCC, cpu.MicroArchitecture)
}

func TestGetCPU_GNR_WithDevices(t *testing.T) {
	// Test GNR X2 with 4 devices
	id := NewX86Identifier("6", "173", "0", "", "4")
	cpu, err := GetCPU(id)
	require.NoError(t, err)
	assert.Equal(t, UarchGNR_X2, cpu.MicroArchitecture)
}

func TestGetCPU_AMD_Milan(t *testing.T) {
	// Test AMD Milan - Family 25, Model 1
	id := NewX86Identifier("25", "1", "1", "", "")
	cpu, err := GetCPU(id)
	require.NoError(t, err)
	assert.Equal(t, UarchMilan, cpu.MicroArchitecture)
}

func TestGetCPU_AMD_Genoa(t *testing.T) {
	// Test AMD Genoa - Family 25, Model 17
	id := NewX86Identifier("25", "17", "0", "", "")
	cpu, err := GetCPU(id)
	require.NoError(t, err)
	assert.Equal(t, UarchGenoa, cpu.MicroArchitecture)
}
