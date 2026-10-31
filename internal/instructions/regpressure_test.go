// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package instructions

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tolerance for comparing against the reference implementation, which prints 4 decimals.
const tolerance = 1e-4

// TestAnalyzeRegPressureAgainstReference checks the port against regpressure.awk run over
// the same fixtures. Expected values are that implementation's full-precision output:
//
//	gawk -v mode=annotate -v show_ops=0 -f regpressure.awk < annotate_<sym>.stdio
//
// The fixtures come from a purpose-built workload whose register pressure is known
// independently: gp_pressure spills general-purpose registers, simd_pressure spills
// vector registers, and light_loop has no stack traffic at all.
func TestAnalyzeRegPressureAgainstReference(t *testing.T) {
	tests := []struct {
		fixture   string
		stack     float64
		spill     float64
		reload    float64
		gp        float64
		simd      float64
		unknown   float64
		gpRegs    int
		simdRegs  int
		insnCount int
		tier      string
	}{
		{
			fixture: "annotate_gp_pressure.stdio",
			stack:   91.3791, spill: 33.3433, reload: 58.0358,
			gp: 91.3791, simd: 0.0000, unknown: 0.0000,
			gpRegs: 14, simdRegs: 0, insnCount: 148, tier: TierHigh,
		},
		{
			fixture: "annotate_simd_pressure.stdio",
			stack:   3.5200, spill: 3.5200, reload: 0.0000,
			gp: 0.0000, simd: 3.5200, unknown: 0.0000,
			gpRegs: 6, simdRegs: 16, insnCount: 192, tier: TierLow,
		},
		{
			fixture: "annotate_light_loop.stdio",
			stack:   0.0000, spill: 0.0000, reload: 0.0000,
			gp: 0.0000, simd: 0.0000, unknown: 0.0000,
			gpRegs: 8, simdRegs: 0, insnCount: 26, tier: TierLow,
		},
	}

	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			insns := loadFixture(t, tc.fixture)
			got := AnalyzeRegPressure(insns, 5)

			checkPct(t, "StackPct", got.StackPct, tc.stack)
			checkPct(t, "SpillPct", got.SpillPct, tc.spill)
			checkPct(t, "ReloadPct", got.ReloadPct, tc.reload)
			checkPct(t, "GPPct", got.GPPct, tc.gp)
			checkPct(t, "SIMDPct", got.SIMDPct, tc.simd)
			checkPct(t, "UnknownPct", got.UnknownPct, tc.unknown)

			if got.GPRegsUsed != tc.gpRegs {
				t.Errorf("GPRegsUsed = %d, want %d", got.GPRegsUsed, tc.gpRegs)
			}
			if got.SIMDRegsUsed != tc.simdRegs {
				t.Errorf("SIMDRegsUsed = %d, want %d", got.SIMDRegsUsed, tc.simdRegs)
			}
			if got.InsnCount != tc.insnCount {
				t.Errorf("InsnCount = %d, want %d", got.InsnCount, tc.insnCount)
			}
			if got.Tier() != tc.tier {
				t.Errorf("Tier() = %q, want %q", got.Tier(), tc.tier)
			}
		})
	}
}

// TestAnalyzeRegPressureEdgeCases covers classifier branches the real workload never
// reaches: frame-pointer-less code, stack canaries, push/pop, lea exclusion,
// read-modify-write double counting, vector widths, immediate sources and zero-sample
// symbols. Fixtures are synthetic; expected values were produced by running the reference
// implementation over the same files, so this is a differential test against it rather
// than an assertion of what the numbers ought to be.
//
// Regenerate the fixtures with: testdata/make-edge-fixtures.sh testdata
func TestAnalyzeRegPressureEdgeCases(t *testing.T) {
	tests := []struct {
		fixture   string
		stack     float64
		spill     float64
		reload    float64
		gp        float64
		simd      float64
		unknown   float64
		gpRegs    int
		simdRegs  int
		insnCount int
		note      string
	}{
		{
			fixture: "edge_nofp.stdio",
			stack:   60.0000, spill: 40.0000, reload: 20.0000,
			gp: 60.0000, simd: 0, unknown: 0,
			gpRegs: 4, simdRegs: 0, insnCount: 8,
			note: "no frame-pointer prologue: rbp-relative access must not count as stack traffic",
		},
		{
			fixture: "edge_canary.stdio",
			stack:   100.0000, spill: 50.0000, reload: 50.0000,
			gp: 100.0000, simd: 0, unknown: 0,
			gpRegs: 2, simdRegs: 0, insnCount: 6,
			note: "canary load, its store and the exit compare are all excluded",
		},
		{
			fixture: "edge_pushpop_lea.stdio",
			stack:   50.0000, spill: 25.0000, reload: 25.0000,
			gp: 50.0000, simd: 0, unknown: 0,
			gpRegs: 4, simdRegs: 0, insnCount: 8,
			note: "push is a spill and pop a reload; lea on a stack address is neither",
		},
		{
			fixture: "edge_rmw_cmp.stdio",
			stack:   150.0000, spill: 50.0000, reload: 100.0000,
			gp: 150.0000, simd: 0, unknown: 0,
			gpRegs: 2, simdRegs: 0, insnCount: 8,
			note: "RMW counts as both spill and reload, so StackPct exceeds 100; cmp/test are loads",
		},
		{
			fixture: "edge_simd.stdio",
			stack:   100.0000, spill: 60.0000, reload: 40.0000,
			gp: 0, simd: 100.0000, unknown: 0,
			gpRegs: 1, simdRegs: 5, insnCount: 9,
			note: "xmm/ymm/zmm all attribute to SIMD; a GP index register does not make it GP",
		},
		{
			fixture: "edge_widths.stdio",
			stack:   100.0000, spill: 100.0000, reload: 0,
			gp: 100.0000, simd: 0, unknown: 0,
			gpRegs: 7, simdRegs: 0, insnCount: 11,
			note: "8/16/32-bit sources normalize onto 7 distinct 64-bit registers",
		},
		{
			fixture: "edge_imm.stdio",
			stack:   175.0000, spill: 100.0000, reload: 75.0000,
			gp: 0, simd: 0, unknown: 0,
			gpRegs: 0, simdRegs: 0, insnCount: 8,
			note: "immediate sources are not registers, so stack traffic has no GP attribution",
		},
		{
			fixture: "edge_zero.stdio",
			stack:   0, spill: 0, reload: 0,
			gp: 0, simd: 0, unknown: 0,
			gpRegs: 1, simdRegs: 0, insnCount: 5,
			note: "all weights zero: exercises the denominator guard",
		},
	}

	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			got := AnalyzeRegPressure(loadFixture(t, tc.fixture), 0)

			checkPct(t, "StackPct", got.StackPct, tc.stack)
			checkPct(t, "SpillPct", got.SpillPct, tc.spill)
			checkPct(t, "ReloadPct", got.ReloadPct, tc.reload)
			checkPct(t, "GPPct", got.GPPct, tc.gp)
			checkPct(t, "SIMDPct", got.SIMDPct, tc.simd)
			checkPct(t, "UnknownPct", got.UnknownPct, tc.unknown)

			if got.GPRegsUsed != tc.gpRegs {
				t.Errorf("GPRegsUsed = %d, want %d (%s)", got.GPRegsUsed, tc.gpRegs, tc.note)
			}
			if got.SIMDRegsUsed != tc.simdRegs {
				t.Errorf("SIMDRegsUsed = %d, want %d (%s)", got.SIMDRegsUsed, tc.simdRegs, tc.note)
			}
			if got.InsnCount != tc.insnCount {
				t.Errorf("InsnCount = %d, want %d (%s)", got.InsnCount, tc.insnCount, tc.note)
			}
		})
	}
}

// TestGPSIMDDiscrimination asserts the property the fixtures were built to verify: the
// GP-heavy symbol must attribute its pressure entirely to GP registers and the SIMD-heavy
// symbol entirely to vector registers, with no leakage either way.
func TestGPSIMDDiscrimination(t *testing.T) {
	gp := AnalyzeRegPressure(loadFixture(t, "annotate_gp_pressure.stdio"), 0)
	if gp.SIMDPct != 0 {
		t.Errorf("gp_pressure leaked %.4f%% into SIMDPct, want 0", gp.SIMDPct)
	}
	if gp.GPPct <= 0 {
		t.Errorf("gp_pressure GPPct = %.4f, want > 0", gp.GPPct)
	}

	simd := AnalyzeRegPressure(loadFixture(t, "annotate_simd_pressure.stdio"), 0)
	if simd.GPPct != 0 {
		t.Errorf("simd_pressure leaked %.4f%% into GPPct, want 0", simd.GPPct)
	}
	if simd.SIMDPct <= 0 {
		t.Errorf("simd_pressure SIMDPct = %.4f, want > 0", simd.SIMDPct)
	}
}

// TestTopOpsIdentifiesSpillInstructions checks that the heaviest stack instructions are
// the expected spill forms: GP spills through rbp for one symbol, movsd of xmm registers
// for the other.
func TestTopOpsIdentifiesSpillInstructions(t *testing.T) {
	gp := AnalyzeRegPressure(loadFixture(t, "annotate_gp_pressure.stdio"), 5)
	if len(gp.TopOps) == 0 {
		t.Fatal("gp_pressure returned no TopOps")
	}
	for _, op := range gp.TopOps {
		if !strings.Contains(op.Text, "(%rbp)") {
			t.Errorf("gp_pressure TopOp %q does not reference the frame", op.Text)
		}
		if strings.Contains(op.Text, "xmm") {
			t.Errorf("gp_pressure TopOp %q references a vector register", op.Text)
		}
	}
	// Weights must be ordered descending.
	for i := 1; i < len(gp.TopOps); i++ {
		if gp.TopOps[i].Pct > gp.TopOps[i-1].Pct {
			t.Errorf("TopOps not sorted descending at %d: %.4f > %.4f",
				i, gp.TopOps[i].Pct, gp.TopOps[i-1].Pct)
		}
	}

	simd := AnalyzeRegPressure(loadFixture(t, "annotate_simd_pressure.stdio"), 5)
	if len(simd.TopOps) == 0 {
		t.Fatal("simd_pressure returned no TopOps")
	}
	for _, op := range simd.TopOps {
		if !strings.Contains(op.Text, "xmm") {
			t.Errorf("simd_pressure TopOp %q does not reference a vector register", op.Text)
		}
	}
}

// TestLightLoopHasNoStackTraffic is the negative control: a loop with few live values must
// report no stack traffic and no TopOps at all.
func TestLightLoopHasNoStackTraffic(t *testing.T) {
	got := AnalyzeRegPressure(loadFixture(t, "annotate_light_loop.stdio"), 5)
	if got.StackPct != 0 {
		t.Errorf("StackPct = %.4f, want 0", got.StackPct)
	}
	if len(got.TopOps) != 0 {
		t.Errorf("TopOps = %v, want none", got.TopOps)
	}
}

// TestFramePointerDetection verifies the prologue is recognized in the fixtures, which is
// a precondition for rbp-relative accesses being counted as stack traffic at all.
func TestFramePointerDetection(t *testing.T) {
	for _, f := range []string{"annotate_gp_pressure.stdio", "annotate_simd_pressure.stdio"} {
		if !detectFramePointer(loadFixture(t, f)) {
			t.Errorf("%s: frame pointer not detected, but the binary is built -fno-omit-frame-pointer", f)
		}
	}
}

func TestAnalyzeRegPressureEmptyInput(t *testing.T) {
	got := AnalyzeRegPressure(nil, 5)
	if got.StackPct != 0 || got.InsnCount != 0 {
		t.Errorf("empty input produced %+v, want zero value", got)
	}
	if got.Tier() != TierLow {
		t.Errorf("Tier() = %q, want %q", got.Tier(), TierLow)
	}
}

func checkPct(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %.4f, want %.4f (delta %.6f)", name, got, want, math.Abs(got-want))
	}
}

func loadFixture(t *testing.T, name string) []Instruction {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	insns, err := ParseAnnotate(f)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return insns
}

// readFixture returns a testdata file's contents as a string.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(b)
}
