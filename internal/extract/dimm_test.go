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
			name:         "dimmType0 - Inspur ICX",
			bankLocator:  "Not Specified",
			locator:      "CPU0_C0D0",
			expectedType: dimmType0,
		},
		{
			name:         "dimmType1 - Generic CPU_Letter_Digit",
			bankLocator:  "Not Specified",
			locator:      "CPU0_A0",
			expectedType: dimmType1,
		},
		{
			name:         "dimmType2 - MC format",
			bankLocator:  "Not Specified",
			locator:      "CPU0_MC0_DIMM_A0",
			expectedType: dimmType2,
		},
		{
			name:         "dimmType3 - NODE CHANNEL DIMM",
			bankLocator:  "NODE 0 CHANNEL 0 DIMM 0",
			locator:      "DIMM0",
			expectedType: dimmType3,
		},
		{
			name:         "dimmType4 - P_Node_Channel_Dimm",
			bankLocator:  "P0_Node0_Channel0_Dimm0",
			locator:      "DIMM0",
			expectedType: dimmType4,
		},
		{
			name:         "dimmType5 - _Node_Channel_Dimm",
			bankLocator:  "_Node0_Channel0_Dimm0",
			locator:      "DIMM0",
			expectedType: dimmType5,
		},
		{
			name:         "dimmType6 - SKX SDP (1-indexed)",
			bankLocator:  "NODE 1",
			locator:      "CPU1_DIMM_A1",
			expectedType: dimmType6,
		},
		{
			name:         "dimmType7 - ICX SDP (0-indexed)",
			bankLocator:  "NODE 0",
			locator:      "CPU0_DIMM_A1",
			expectedType: dimmType7,
		},
		{
			name:         "dimmType8 - NODE n + DIMM_Xn",
			bankLocator:  "NODE 1",
			locator:      "DIMM_A1",
			expectedType: dimmType8,
		},
		{
			name:         "dimmType9 - Gigabyte Milan DIMM_Pn_Xn",
			bankLocator:  "BANK 0",
			locator:      "DIMM_P0_A0",
			expectedType: dimmType9,
		},
		{
			name:         "dimmType10 - NUC SODIMM",
			bankLocator:  "CHANNEL A DIMM0",
			locator:      "SODIMM0",
			expectedType: dimmType10,
		},
		{
			name:         "dimmType11 - Alder Lake Controller",
			bankLocator:  "BANK 0",
			locator:      "Controller0-ChannelA-DIMM0",
			expectedType: dimmType11,
		},
		{
			name:         "dimmType12 - SuperMicro SPR P1-DIMMA1",
			bankLocator:  "Not Specified",
			locator:      "P1-DIMMA1",
			expectedType: dimmType12,
		},
		{
			name:         "dimmType13 - Birchstream CPU0_DIMM_A1",
			bankLocator:  "BANK 0",
			locator:      "CPU0_DIMM_A1",
			expectedType: dimmType13,
		},
		{
			name:         "dimmType14 - GNR AP/X3 CPU0_DIMM_A",
			bankLocator:  "BANK 0",
			locator:      "CPU0_DIMM_A",
			expectedType: dimmType14,
		},
		{
			name:         "dimmType15 - Forest City CPU0 CH0/D0",
			bankLocator:  "BANK 0",
			locator:      "CPU0 CH0/D0",
			expectedType: dimmType15,
		},
		{
			name:         "dimmType16 - Quanta GNR",
			bankLocator:  "_Node0_Channel0_Dimm1",
			locator:      "CPU0_A1",
			expectedType: dimmType16,
		},
		// --- Ordering-sensitive / ambiguous cases ---
		{
			name:         "ordering: CPU0_C0D0 matches type0, not type1",
			bankLocator:  "Not Specified",
			locator:      "CPU0_C0D0",
			expectedType: dimmType0,
		},
		{
			name:         "ordering: P1-DIMMA1 matches type12, not type1",
			bankLocator:  "Not Specified",
			locator:      "P1-DIMMA1",
			expectedType: dimmType12,
		},
		{
			name:         "ordering: Quanta GNR matches type16, not type5",
			bankLocator:  "_Node0_Channel0_Dimm1",
			locator:      "CPU0_A1",
			expectedType: dimmType16,
		},
		{
			// type16 requires Dimm[1-2]; Dimm0 doesn't match type16.
			// CPU0_A1 matches type1's CPU([0-9])_([A-Z])([0-9]) before reaching type5's bank locator check.
			name:         "ordering: _Node0_Channel0_Dimm0 with CPU0_A1 falls to type1",
			bankLocator:  "_Node0_Channel0_Dimm0",
			locator:      "CPU0_A1",
			expectedType: dimmType1,
		},
		{
			// type6 requires CPU[1-4] and NODE [1-8]; type7 requires CPU[0-7] and NODE [0-9]+
			name:         "ordering: SKX SDP CPU1_DIMM_A1 NODE 1 matches type6",
			bankLocator:  "NODE 1",
			locator:      "CPU1_DIMM_A1",
			expectedType: dimmType6,
		},
		{
			// CPU0 doesn't match type6's CPU[1-4], so falls to type7
			name:         "ordering: ICX SDP CPU0_DIMM_A1 NODE 0 matches type7",
			bankLocator:  "NODE 0",
			locator:      "CPU0_DIMM_A1",
			expectedType: dimmType7,
		},
		{
			// Birchstream CPU0_DIMM_A1 with non-NODE bank loc → not type6/7, falls to type13
			name:         "ordering: Birchstream CPU0_DIMM_A1 BANK 0 matches type13, not type7",
			bankLocator:  "BANK 0",
			locator:      "CPU0_DIMM_A1",
			expectedType: dimmType13,
		},
		// --- Multi-digit values (regression tests for [\d+] bug fix) ---
		{
			name:         "type16 - multi-digit node number",
			bankLocator:  "_Node10_Channel0_Dimm1",
			locator:      "CPU0_A1",
			expectedType: dimmType16,
		},
		{
			name:         "type16 - multi-digit channel number",
			bankLocator:  "_Node0_Channel12_Dimm2",
			locator:      "CPU10_B2",
			expectedType: dimmType16,
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
		// dimmType0: reLoc match[1]=socket, match[3]=slot
		{
			name:           "type0 - Inspur ICX CPU0_C0D0",
			bankLocator:    "Not Specified",
			locator:        "CPU0_C0D0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type0 - Inspur ICX CPU1_C2D1",
			bankLocator:    "Not Specified",
			locator:        "CPU1_C2D1",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// dimmType1: reLoc match[1]=socket, match[3]=slot
		{
			name:           "type1 - CPU0_A0",
			bankLocator:    "Not Specified",
			locator:        "CPU0_A0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type1 - CPU1_B3",
			bankLocator:    "Not Specified",
			locator:        "CPU1_B3",
			expectedSocket: 1,
			expectedSlot:   3,
		},
		// dimmType2: reLoc match[1]=socket, match[3]=slot
		{
			name:           "type2 - CPU0_MC0_DIMM_A0",
			bankLocator:    "Not Specified",
			locator:        "CPU0_MC0_DIMM_A0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type2 - CPU1_MC1_DIMM_B2",
			bankLocator:    "Not Specified",
			locator:        "CPU1_MC1_DIMM_B2",
			expectedSocket: 1,
			expectedSlot:   2,
		},
		// dimmType3: reBankLoc match[1]=socket, match[3]=slot
		{
			name:           "type3 - NODE 0 CHANNEL 0 DIMM 0",
			bankLocator:    "NODE 0 CHANNEL 0 DIMM 0",
			locator:        "DIMM0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type3 - NODE 1 CHANNEL 3 DIMM 1",
			bankLocator:    "NODE 1 CHANNEL 3 DIMM 1",
			locator:        "DIMM1",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// dimmType4: reBankLoc match[1]=socket, match[4]=slot
		{
			name:           "type4 - P0_Node0_Channel0_Dimm0",
			bankLocator:    "P0_Node0_Channel0_Dimm0",
			locator:        "DIMM0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type4 - P1_Node1_Channel2_Dimm1",
			bankLocator:    "P1_Node1_Channel2_Dimm1",
			locator:        "DIMM1",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// dimmType5: reBankLoc match[1]=socket, match[3]=slot
		{
			name:           "type5 - _Node0_Channel0_Dimm0",
			bankLocator:    "_Node0_Channel0_Dimm0",
			locator:        "DIMM0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type5 - _Node1_Channel2_Dimm1",
			bankLocator:    "_Node1_Channel2_Dimm1",
			locator:        "DIMM1",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// dimmType6: reLoc match[1]=socket-1, match[3]=slot-1
		{
			name:           "type6 - SKX SDP CPU1_DIMM_A1 NODE 1",
			bankLocator:    "NODE 1",
			locator:        "CPU1_DIMM_A1",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type6 - SKX SDP CPU2_DIMM_B2 NODE 2",
			bankLocator:    "NODE 2",
			locator:        "CPU2_DIMM_B2",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// dimmType7: reLoc match[1]=socket, match[3]=slot-1
		{
			name:           "type7 - ICX SDP CPU0_DIMM_A1 NODE 0",
			bankLocator:    "NODE 0",
			locator:        "CPU0_DIMM_A1",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			// CPU0 doesn't match type6's CPU[1-4], so falls to type7's CPU[0-7]
			name:           "type7 - ICX SDP CPU0_DIMM_C2 NODE 0",
			bankLocator:    "NODE 0",
			locator:        "CPU0_DIMM_C2",
			expectedSocket: 0,
			expectedSlot:   1,
		},
		// dimmType8: reBankLoc match[1]=socket-1, reLoc match[2]=slot-1
		{
			name:           "type8 - NODE 1 DIMM_A1",
			bankLocator:    "NODE 1",
			locator:        "DIMM_A1",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type8 - NODE 2 DIMM_B3",
			bankLocator:    "NODE 2",
			locator:        "DIMM_B3",
			expectedSocket: 1,
			expectedSlot:   2,
		},
		// dimmType9: reLoc match[1]=socket, match[2]=slot
		{
			name:           "type9 - Gigabyte Milan DIMM_P0_A0",
			bankLocator:    "BANK 0",
			locator:        "DIMM_P0_A0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type9 - Gigabyte Milan DIMM_P1_B1",
			bankLocator:    "BANK 0",
			locator:        "DIMM_P1_B1",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// dimmType10: socket=0, reBankLoc match[2]=slot
		{
			name:           "type10 - NUC CHANNEL A DIMM0",
			bankLocator:    "CHANNEL A DIMM0",
			locator:        "SODIMM0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type10 - NUC CHANNEL B DIMM1",
			bankLocator:    "CHANNEL B DIMM1",
			locator:        "SODIMM1",
			expectedSocket: 0,
			expectedSlot:   1,
		},
		// dimmType11: socket=0, reLoc match[2]=slot
		{
			name:           "type11 - Alder Lake Controller0-ChannelA-DIMM0",
			bankLocator:    "BANK 0",
			locator:        "Controller0-ChannelA-DIMM0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type11 - Alder Lake Controller1-ChannelA-DIMM1",
			bankLocator:    "BANK 0",
			locator:        "Controller1-ChannelA-DIMM1",
			expectedSocket: 0,
			expectedSlot:   1,
		},
		// dimmType12: reLoc match[1]=socket-1, match[3]=slot-1
		{
			name:           "type12 - SuperMicro P1-DIMMA1",
			bankLocator:    "Not Specified",
			locator:        "P1-DIMMA1",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type12 - SuperMicro P2-DIMMB2",
			bankLocator:    "Not Specified",
			locator:        "P2-DIMMB2",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// dimmType13: reLoc match[1]=socket, match[3]=slot-1
		{
			name:           "type13 - Birchstream CPU0_DIMM_A1",
			bankLocator:    "BANK 0",
			locator:        "CPU0_DIMM_A1",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type13 - Birchstream CPU1_DIMM_H2",
			bankLocator:    "BANK 7",
			locator:        "CPU1_DIMM_H2",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// dimmType14: reLoc match[1]=socket, slot=0
		{
			name:           "type14 - GNR AP/X3 CPU0_DIMM_A",
			bankLocator:    "BANK 0",
			locator:        "CPU0_DIMM_A",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type14 - GNR AP/X3 CPU1_DIMM_L",
			bankLocator:    "BANK 11",
			locator:        "CPU1_DIMM_L",
			expectedSocket: 1,
			expectedSlot:   0,
		},
		// dimmType15: reLoc match[1]=socket, match[3]=slot
		{
			name:           "type15 - Forest City CPU0 CH0/D0",
			bankLocator:    "BANK 0",
			locator:        "CPU0 CH0/D0",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type15 - Forest City CPU1 CH7/D1",
			bankLocator:    "BANK 7",
			locator:        "CPU1 CH7/D1",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// dimmType16: reBankLoc match[1]=socket, reLoc match[3]=slot-1
		{
			name:           "type16 - Quanta GNR _Node0_Channel0_Dimm1 CPU0_A1",
			bankLocator:    "_Node0_Channel0_Dimm1",
			locator:        "CPU0_A1",
			expectedSocket: 0,
			expectedSlot:   0,
		},
		{
			name:           "type16 - Quanta GNR _Node1_Channel2_Dimm2 CPU1_B2",
			bankLocator:    "_Node1_Channel2_Dimm2",
			locator:        "CPU1_B2",
			expectedSocket: 1,
			expectedSlot:   1,
		},
		// --- Multi-digit regression tests for [\d+] bug fix ---
		{
			// Socket comes from reBankLoc match[1] (Node number), slot from reBankLoc match[3] (Dimm-1)
			name:           "type16 - multi-digit node _Node10_Channel0_Dimm1 CPU0_A1",
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
		name               string
		dimms              [][]string
		channelsPerSocket  int
		expectedDerived    []DerivedDIMMFields
		expectErr          bool
		expectNil          bool // nil result, no error (parse failure logged)
	}{
		{
			name: "type1 - two sockets, two channels each, one slot",
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
			name: "type3 - NODE CHANNEL format",
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
