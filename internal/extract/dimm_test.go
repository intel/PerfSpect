// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"testing"
)

func TestGetDIMMParseInfo(t *testing.T) {
	tests := []struct {
		name         string
		bankLocator  string
		locator      string
		expectedType dimmType
	}{
		// --- One positive case per dimmType ---
		{
			name:         "Inspur ICX",
			bankLocator:  "Not Specified",
			locator:      "CPU0_C0D0",
			expectedType: dimmTypeInspurICX,
		},
		{
			name:         "Generic CPU_Letter_Digit",
			bankLocator:  "Not Specified",
			locator:      "CPU0_A0",
			expectedType: dimmTypeGenericCPULetterDigit,
		},
		{
			name:         "MC format",
			bankLocator:  "Not Specified",
			locator:      "CPU0_MC0_DIMM_A0",
			expectedType: dimmTypeMCFormat,
		},
		{
			name:         "NODE CHANNEL DIMM",
			bankLocator:  "NODE 0 CHANNEL 0 DIMM 0",
			locator:      "DIMM0",
			expectedType: dimmTypeNodeChannelDimm,
		},
		{
			name:         "P_Node_Channel_Dimm",
			bankLocator:  "P0_Node0_Channel0_Dimm0",
			locator:      "DIMM0",
			expectedType: dimmTypePNodeChannelDimm,
		},
		{
			name:         "_Node_Channel_Dimm",
			bankLocator:  "_Node0_Channel0_Dimm0",
			locator:      "DIMM0",
			expectedType: dimmTypeNodeChannelDimmAlt,
		},
		{
			name:         "SKX SDP (1-indexed)",
			bankLocator:  "NODE 1",
			locator:      "CPU1_DIMM_A1",
			expectedType: dimmTypeSKXSDP,
		},
		{
			name:         "ICX SDP (0-indexed)",
			bankLocator:  "NODE 0",
			locator:      "CPU0_DIMM_A1",
			expectedType: dimmTypeICXSDP,
		},
		{
			name:         "NODE n + DIMM_Xn",
			bankLocator:  "NODE 1",
			locator:      "DIMM_A1",
			expectedType: dimmTypeNodeDIMM,
		},
		{
			name:         "Gigabyte Milan DIMM_Pn_Xn",
			bankLocator:  "BANK 0",
			locator:      "DIMM_P0_A0",
			expectedType: dimmTypeGigabyteMilan,
		},
		{
			name:         "NUC SODIMM",
			bankLocator:  "CHANNEL A DIMM0",
			locator:      "SODIMM0",
			expectedType: dimmTypeNUC,
		},
		{
			name:         "Alder Lake Controller",
			bankLocator:  "BANK 0",
			locator:      "Controller0-ChannelA-DIMM0",
			expectedType: dimmTypeAlderLake,
		},
		{
			name:         "SuperMicro SPR P1-DIMMA1",
			bankLocator:  "Not Specified",
			locator:      "P1-DIMMA1",
			expectedType: dimmTypeSuperMicroSPR,
		},
		{
			name:         "Birchstream CPU0_DIMM_A1",
			bankLocator:  "BANK 0",
			locator:      "CPU0_DIMM_A1",
			expectedType: dimmTypeBirchstream,
		},
		{
			name:         "GNR AP/X3 CPU0_DIMM_A",
			bankLocator:  "BANK 0",
			locator:      "CPU0_DIMM_A",
			expectedType: dimmTypeBirchstreamGNRAP,
		},
		{
			name:         "Forest City CPU0 CH0/D0",
			bankLocator:  "BANK 0",
			locator:      "CPU0 CH0/D0",
			expectedType: dimmTypeForestCity,
		},
		{
			name:         "Quanta GNR",
			bankLocator:  "_Node0_Channel0_Dimm1",
			locator:      "CPU0_A1",
			expectedType: dimmTypeQuantaGNR,
		},
		// --- Ordering-sensitive / ambiguous cases ---
		{
			name:         "ordering: CPU0_C0D0 matches InspurICX, not GenericCPULetterDigit",
			bankLocator:  "Not Specified",
			locator:      "CPU0_C0D0",
			expectedType: dimmTypeInspurICX,
		},
		{
			name:         "ordering: P1-DIMMA1 matches SuperMicroSPR, not GenericCPULetterDigit",
			bankLocator:  "Not Specified",
			locator:      "P1-DIMMA1",
			expectedType: dimmTypeSuperMicroSPR,
		},
		{
			name:         "ordering: Quanta GNR matches QuantaGNR, not NodeChannelDimmAlt",
			bankLocator:  "_Node0_Channel0_Dimm1",
			locator:      "CPU0_A1",
			expectedType: dimmTypeQuantaGNR,
		},
		{
			// QuantaGNR requires Dimm[1-2]; Dimm0 doesn't match.
			// CPU0_A1 matches GenericCPULetterDigit before reaching NodeChannelDimmAlt's bank locator check.
			name:         "ordering: _Node0_Channel0_Dimm0 with CPU0_A1 falls to GenericCPULetterDigit",
			bankLocator:  "_Node0_Channel0_Dimm0",
			locator:      "CPU0_A1",
			expectedType: dimmTypeGenericCPULetterDigit,
		},
		{
			// SKXSDP requires CPU[1-4] and NODE [1-8]; ICXSDP requires CPU[0-7] and NODE [0-9]+
			name:         "ordering: SKX SDP CPU1_DIMM_A1 NODE 1 matches SKXSDP",
			bankLocator:  "NODE 1",
			locator:      "CPU1_DIMM_A1",
			expectedType: dimmTypeSKXSDP,
		},
		{
			// CPU0 doesn't match SKXSDP's CPU[1-4], so falls to ICXSDP
			name:         "ordering: ICX SDP CPU0_DIMM_A1 NODE 0 matches ICXSDP",
			bankLocator:  "NODE 0",
			locator:      "CPU0_DIMM_A1",
			expectedType: dimmTypeICXSDP,
		},
		{
			// Birchstream CPU0_DIMM_A1 with non-NODE bank loc -> not SKXSDP/ICXSDP, falls to Birchstream
			name:         "ordering: Birchstream CPU0_DIMM_A1 BANK 0 matches Birchstream, not ICXSDP",
			bankLocator:  "BANK 0",
			locator:      "CPU0_DIMM_A1",
			expectedType: dimmTypeBirchstream,
		},
		// --- Multi-digit values (regression tests for [\d+] bug fix) ---
		{
			name:         "QuantaGNR - multi-digit node number",
			bankLocator:  "_Node10_Channel0_Dimm1",
			locator:      "CPU0_A1",
			expectedType: dimmTypeQuantaGNR,
		},
		{
			name:         "QuantaGNR - multi-digit channel number",
			bankLocator:  "_Node0_Channel12_Dimm2",
			locator:      "CPU10_B2",
			expectedType: dimmTypeQuantaGNR,
		},
		// --- Unknown / no match ---
		{
			name:         "unknown format returns dimmTypeUNKNOWN",
			bankLocator:  "UNKNOWN",
			locator:      "UNKNOWN",
			expectedType: dimmTypeUNKNOWN,
		},
		{
			name:         "empty strings return dimmTypeUNKNOWN",
			bankLocator:  "",
			locator:      "",
			expectedType: dimmTypeUNKNOWN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dt, _, _ := getDIMMParseInfo(tt.bankLocator, tt.locator)
			if dt != tt.expectedType {
				t.Errorf("getDIMMParseInfo(%q, %q) = dimmType %d, want dimmType %d",
					tt.bankLocator, tt.locator, dt, tt.expectedType)
			}
		})
	}
}

func TestGetDIMMSocketSlot(t *testing.T) {
	tests := []struct {
		name           string
		bankLocator    string
		locator        string
		expectedSocket int
		expectedSlot   int
		expectErr      bool
	}{
		// InspurICX: locMatch[1]=socket, locMatch[3]=slot
		{
			name:           "InspurICX - CPU0_C0D0",
			bankLocator:    "Not Specified",
			locator:        "CPU0_C0D0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "InspurICX - CPU1_C2D1",
			bankLocator:    "Not Specified",
			locator:        "CPU1_C2D1",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// GenericCPULetterDigit: locMatch[1]=socket, locMatch[3]=slot
		{
			name:           "GenericCPULetterDigit - CPU0_A0",
			bankLocator:    "Not Specified",
			locator:        "CPU0_A0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "GenericCPULetterDigit - CPU1_B3",
			bankLocator:    "Not Specified",
			locator:        "CPU1_B3",
			expectedSocket: 1,
			expectedSlot:   3,
		},
		// MCFormat: locMatch[1]=socket, locMatch[3]=slot
		{
			name:           "MCFormat - CPU0_MC0_DIMM_A0",
			bankLocator:    "Not Specified",
			locator:        "CPU0_MC0_DIMM_A0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "MCFormat - CPU1_MC1_DIMM_B2",
			bankLocator:    "Not Specified",
			locator:        "CPU1_MC1_DIMM_B2",
			expectedSocket: 1,
			expectedSlot:   2,
		},
		// NodeChannelDimm: bankLocMatch[1]=socket, bankLocMatch[3]=slot
		{
			name:           "NodeChannelDimm - NODE 0 CHANNEL 0 DIMM 0",
			bankLocator:    "NODE 0 CHANNEL 0 DIMM 0",
			locator:        "DIMM0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "NodeChannelDimm - NODE 1 CHANNEL 3 DIMM 1",
			bankLocator:    "NODE 1 CHANNEL 3 DIMM 1",
			locator:        "DIMM1",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// PNodeChannelDimm: bankLocMatch[1]=socket, bankLocMatch[4]=slot
		{
			name:           "PNodeChannelDimm - P0_Node0_Channel0_Dimm0",
			bankLocator:    "P0_Node0_Channel0_Dimm0",
			locator:        "DIMM0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "PNodeChannelDimm - P1_Node1_Channel2_Dimm1",
			bankLocator:    "P1_Node1_Channel2_Dimm1",
			locator:        "DIMM1",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// NodeChannelDimmAlt: bankLocMatch[1]=socket, bankLocMatch[3]=slot
		{
			name:           "NodeChannelDimmAlt - _Node0_Channel0_Dimm0",
			bankLocator:    "_Node0_Channel0_Dimm0",
			locator:        "DIMM0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "NodeChannelDimmAlt - _Node1_Channel2_Dimm1",
			bankLocator:    "_Node1_Channel2_Dimm1",
			locator:        "DIMM1",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// SKXSDP: locMatch[1]=socket-1, locMatch[3]=slot-1
		{
			name:           "SKXSDP - CPU1_DIMM_A1 NODE 1",
			bankLocator:    "NODE 1",
			locator:        "CPU1_DIMM_A1",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "SKXSDP - CPU2_DIMM_B2 NODE 2",
			bankLocator:    "NODE 2",
			locator:        "CPU2_DIMM_B2",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// ICXSDP: locMatch[1]=socket, locMatch[3]=slot-1
		{
			name:           "ICXSDP - CPU0_DIMM_A1 NODE 0",
			bankLocator:    "NODE 0",
			locator:        "CPU0_DIMM_A1",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			// CPU0 doesn't match SKXSDP's CPU[1-4], so falls to ICXSDP's CPU[0-7]
			name:           "ICXSDP - CPU0_DIMM_C2 NODE 0",
			bankLocator:    "NODE 0",
			locator:        "CPU0_DIMM_C2",
			expectedSocket: 0,
			expectedSlot:   1,
		},
		// NodeDIMM: bankLocMatch[1]=socket-1, locMatch[2]=slot-1
		{
			name:           "NodeDIMM - NODE 1 DIMM_A1",
			bankLocator:    "NODE 1",
			locator:        "DIMM_A1",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "NodeDIMM - NODE 2 DIMM_B3",
			bankLocator:    "NODE 2",
			locator:        "DIMM_B3",
			expectedSocket: 1,
			expectedSlot:   2,
		},
		// GigabyteMilan: locMatch[1]=socket, locMatch[2]=slot
		{
			name:           "GigabyteMilan - DIMM_P0_A0",
			bankLocator:    "BANK 0",
			locator:        "DIMM_P0_A0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "GigabyteMilan - DIMM_P1_B1",
			bankLocator:    "BANK 0",
			locator:        "DIMM_P1_B1",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// NUC: socket=0, bankLocMatch[2]=slot
		{
			name:           "NUC - CHANNEL A DIMM0",
			bankLocator:    "CHANNEL A DIMM0",
			locator:        "SODIMM0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "NUC - CHANNEL B DIMM1",
			bankLocator:    "CHANNEL B DIMM1",
			locator:        "SODIMM1",
			expectedSocket: 0,
			expectedSlot:   1,
		},
		// AlderLake: socket=0, locMatch[2]=slot
		{
			name:           "AlderLake - Controller0-ChannelA-DIMM0",
			bankLocator:    "BANK 0",
			locator:        "Controller0-ChannelA-DIMM0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "AlderLake - Controller1-ChannelA-DIMM1",
			bankLocator:    "BANK 0",
			locator:        "Controller1-ChannelA-DIMM1",
			expectedSocket: 0,
			expectedSlot:   1,
		},
		// SuperMicroSPR: locMatch[1]=socket-1, locMatch[3]=slot-1
		{
			name:           "SuperMicroSPR - P1-DIMMA1",
			bankLocator:    "Not Specified",
			locator:        "P1-DIMMA1",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "SuperMicroSPR - P2-DIMMB2",
			bankLocator:    "Not Specified",
			locator:        "P2-DIMMB2",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// Birchstream: locMatch[1]=socket, locMatch[3]=slot-1
		{
			name:           "Birchstream - CPU0_DIMM_A1",
			bankLocator:    "BANK 0",
			locator:        "CPU0_DIMM_A1",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "Birchstream - CPU1_DIMM_H2",
			bankLocator:    "BANK 7",
			locator:        "CPU1_DIMM_H2",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// BirchstreamGNRAP: locMatch[1]=socket, slot=0
		{
			name:           "BirchstreamGNRAP - CPU0_DIMM_A",
			bankLocator:    "BANK 0",
			locator:        "CPU0_DIMM_A",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "BirchstreamGNRAP - CPU1_DIMM_L",
			bankLocator:    "BANK 11",
			locator:        "CPU1_DIMM_L",
			expectedSocket: 1,
			expectedSlot:   0,
		},
		// ForestCity: locMatch[1]=socket, locMatch[3]=slot
		{
			name:           "ForestCity - CPU0 CH0/D0",
			bankLocator:    "BANK 0",
			locator:        "CPU0 CH0/D0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "ForestCity - CPU1 CH7/D1",
			bankLocator:    "BANK 7",
			locator:        "CPU1 CH7/D1",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// QuantaGNR: bankLocMatch[1]=socket, bankLocMatch[3]=slot-1
		{
			name:           "QuantaGNR - _Node0_Channel0_Dimm1 CPU0_A1",
			bankLocator:    "_Node0_Channel0_Dimm1",
			locator:        "CPU0_A1",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "QuantaGNR - _Node1_Channel2_Dimm2 CPU1_B2",
			bankLocator:    "_Node1_Channel2_Dimm2",
			locator:        "CPU1_B2",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// --- Multi-digit regression tests for [\d+] bug fix ---
		{
			// Socket comes from bankLocMatch[1] (Node number), slot from bankLocMatch[3] (Dimm-1)
			name:           "QuantaGNR - multi-digit node _Node10_Channel0_Dimm1 CPU0_A1",
			bankLocator:    "_Node10_Channel0_Dimm1",
			locator:        "CPU0_A1",
			expectedSocket: 10,
			expectedSlot:   0,
		},
		// --- Error case ---
		{
			name:        "unknown format returns error",
			bankLocator: "UNKNOWN",
			locator:     "UNKNOWN",
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dt, reBankLoc, reLoc := getDIMMParseInfo(tt.bankLocator, tt.locator)
			socket, slot, err := getDIMMSocketSlot(dt, reBankLoc, reLoc, tt.bankLocator, tt.locator)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error for (%q, %q), got socket=%d slot=%d",
						tt.bankLocator, tt.locator, socket, slot)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for (%q, %q): %v", tt.bankLocator, tt.locator, err)
				return
			}
			if socket != tt.expectedSocket {
				t.Errorf("getDIMMSocketSlot(%q, %q) socket = %d, want %d",
					tt.bankLocator, tt.locator, socket, tt.expectedSocket)
			}
			if slot != tt.expectedSlot {
				t.Errorf("getDIMMSocketSlot(%q, %q) slot = %d, want %d",
					tt.bankLocator, tt.locator, slot, tt.expectedSlot)
			}
		})
	}
}

func TestDeriveDIMMInfoOther(t *testing.T) {
	// Helper to build a minimal DIMM row (only BankLocatorIdx and LocatorIdx matter,
	// but SizeIdx is used by the caller; fill enough fields to avoid index panics).
	makeDIMMRow := func(bankLoc, loc string) []string {
		row := make([]string, DerivedSlotIdx+1)
		row[BankLocatorIdx] = bankLoc
		row[LocatorIdx] = loc
		row[SizeIdx] = "8192 MB"
		return row
	}

	tests := []struct {
		name              string
		dimms             [][]string
		channelsPerSocket int
		expectedDerived   []DerivedDIMMFields
		expectErr         bool
		expectNil         bool // nil result, no error (parse failure logged)
	}{
		{
			name: "GenericCPULetterDigit - two sockets, two channels each, one slot",
			dimms: [][]string{
				makeDIMMRow("Not Specified", "CPU0_A0"),
				makeDIMMRow("Not Specified", "CPU0_B0"),
				makeDIMMRow("Not Specified", "CPU1_A0"),
				makeDIMMRow("Not Specified", "CPU1_B0"),
			},
			channelsPerSocket: 2,
			expectedDerived: []DerivedDIMMFields{
				{Socket: "0", Channel: "0", Slot: "0"},
				{Socket: "0", Channel: "1", Slot: "0"},
				{Socket: "1", Channel: "0", Slot: "0"},
				{Socket: "1", Channel: "1", Slot: "0"},
			},
		},
		{
			name: "NodeChannelDimm - NODE CHANNEL format",
			dimms: [][]string{
				makeDIMMRow("NODE 0 CHANNEL 0 DIMM 0", "DIMM0"),
				makeDIMMRow("NODE 0 CHANNEL 0 DIMM 1", "DIMM1"),
				makeDIMMRow("NODE 0 CHANNEL 1 DIMM 0", "DIMM2"),
				makeDIMMRow("NODE 1 CHANNEL 0 DIMM 0", "DIMM3"),
			},
			channelsPerSocket: 2,
			expectedDerived: []DerivedDIMMFields{
				{Socket: "0", Channel: "0", Slot: "0"},
				{Socket: "0", Channel: "0", Slot: "1"},
				{Socket: "0", Channel: "1", Slot: "0"},
				{Socket: "1", Channel: "0", Slot: "0"},
			},
		},
		{
			name:      "empty dimms returns error",
			dimms:     [][]string{},
			expectErr: true,
		},
		{
			name: "unknown format returns error",
			dimms: [][]string{
				makeDIMMRow("UNKNOWN", "UNKNOWN"),
			},
			channelsPerSocket: 2,
			expectErr:         true,
		},
		{
			// First DIMM identifies as GenericCPULetterDigit, but second DIMM
			// doesn't match that format, triggering the return nil, nil path.
			name: "mismatched format in subsequent DIMM returns nil",
			dimms: [][]string{
				makeDIMMRow("Not Specified", "CPU0_A0"),
				makeDIMMRow("Not Specified", "UNKNOWN_FORMAT"),
			},
			channelsPerSocket: 2,
			expectNil:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := deriveDIMMInfoOther(tt.dimms, tt.channelsPerSocket)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got result: %+v", result)
				}
				return
			}
			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil result, got: %+v", result)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(result) != len(tt.expectedDerived) {
				t.Fatalf("result length = %d, want %d", len(result), len(tt.expectedDerived))
			}
			for i, expected := range tt.expectedDerived {
				if result[i] != expected {
					t.Errorf("dimm[%d] = %+v, want %+v", i, result[i], expected)
				}
			}
		})
	}
}

func TestGetDIMMSocketSlotMatchBothPartialMatch(t *testing.T) {
	// For matchBoth formats (e.g., QuantaGNR), if only one of the two patterns
	// matches a subsequent DIMM row, getDIMMSocketSlot must return an error
	// rather than passing a nil match slice to extractFunc (which would panic).
	//
	// This simulates deriveDIMMInfoOther identifying the format from dimms[0],
	// then encountering a later DIMM where only one pattern matches.
	bankLocPat := dimmFormats[1].bankLocPat // QuantaGNR
	locPat := dimmFormats[1].locPat

	// locator matches but bankLocator does not
	_, _, err := getDIMMSocketSlot(dimmTypeQuantaGNR, bankLocPat, locPat, "NOT_A_NODE", "CPU0_A1")
	if err == nil {
		t.Error("expected error when bankLocator doesn't match for matchBoth format, got nil")
	}

	// bankLocator matches but locator does not
	_, _, err = getDIMMSocketSlot(dimmTypeQuantaGNR, bankLocPat, locPat, "_Node0_Channel0_Dimm1", "UNKNOWN")
	if err == nil {
		t.Error("expected error when locator doesn't match for matchBoth format, got nil")
	}
}
