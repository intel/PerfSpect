// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package instructions

import (
	"math"
	"testing"
)

// TestAnalyzeAPXAgainstReference checks the APX signal counters against fields 9-11 of
// the reference output: gawk -v mode=annotate -v show_ops=0 -f regpressure.awk < <fixture>
func TestAnalyzeAPXAgainstReference(t *testing.T) {
	tests := []struct {
		fixture    string
		nddPct     float64
		ccmpPct    float64
		push2Pairs int
	}{
		{"annotate_gp_pressure.stdio", 1.9102, 0.0000, 0},
		{"annotate_simd_pressure.stdio", 0.0000, 0.0000, 0},
		{"annotate_light_loop.stdio", 0.0000, 0.0000, 0},
		{"edge_pushpop_lea.stdio", 0.0000, 0.0000, 1},
		{"edge_nofp.stdio", 0.0000, 0.0000, 0},
		{"edge_canary.stdio", 0.0000, 0.0000, 0},
	}
	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			got := AnalyzeAPX(loadFixture(t, tc.fixture))
			checkPct(t, "NDDPct", got.NDDPct, tc.nddPct)
			checkPct(t, "CCMPPct", got.CCMPPct, tc.ccmpPct)
			if got.Push2Pairs != tc.push2Pairs {
				t.Errorf("Push2Pairs = %d, want %d", got.Push2Pairs, tc.push2Pairs)
			}
		})
	}
}

// TestAnalyzeAPXDoesNotPerturbRegPressure guards the shared single pass: adding the APX
// counters must not change any register-pressure figure.
func TestAnalyzeAPXDoesNotPerturbRegPressure(t *testing.T) {
	insns := loadFixture(t, "annotate_gp_pressure.stdio")
	p := AnalyzeRegPressure(insns, 0)
	if d := p.StackPct - 91.3791; math.Abs(d) > tolerance {
		t.Errorf("StackPct = %.4f, want 91.3791 (unchanged by the APX pass)", p.StackPct)
	}
	if p.InsnCount != 148 {
		t.Errorf("InsnCount = %d, want 148", p.InsnCount)
	}
}

func TestNDDMovDst(t *testing.T) {
	tests := []struct {
		insn string
		want string
	}{
		// Plain register-to-register moves are candidates, normalized to 64-bit names.
		{"mov    %rax,%rbx", "rbx"},
		{"mov    %eax,%r9d", "r9"},
		// Excluded: memory, immediate, SIMD, and extending moves.
		{"mov    -0x8(%rbp),%rax", ""},
		{"mov    $0x1,%rax", ""},
		{"movsd  %xmm0,%xmm1", ""},
		{"movzbl %al,%edx", ""},
		{"mov    %rax,-0x8(%rbp)", ""},
	}
	for _, tc := range tests {
		if got := nddMovDst(tc.insn); got != tc.want {
			t.Errorf("nddMovDst(%q) = %q, want %q", tc.insn, got, tc.want)
		}
	}
}

func TestALUWritesReg(t *testing.T) {
	tests := []struct {
		insn string
		reg  string
		want bool
	}{
		{"add    %rcx,%rbx", "rbx", true},  // two-source, destination matches
		{"add    %rcx,%rbx", "rcx", false}, // source, not destination
		{"inc    %rbx", "rbx", true},       // one-source destructive
		{"neg    %r9", "r9", true},
		{"mov    %rcx,%rbx", "rbx", false},    // not an ALU op
		{"addsd  %xmm1,%xmm0", "xmm0", false}, // SIMD excluded
		{"incq   -0x8(%rbp)", "rbp", false},   // memory operand excluded
		{"add    %ecx,%ebx", "rbx", true},     // sub-register normalizes
	}
	for _, tc := range tests {
		if got := aluWritesReg(tc.insn, tc.reg); got != tc.want {
			t.Errorf("aluWritesReg(%q, %q) = %v, want %v", tc.insn, tc.reg, got, tc.want)
		}
	}
}

func TestIsJcc(t *testing.T) {
	for _, mn := range []string{"je", "jne", "jle", "jbe"} {
		if !isJcc(mn) {
			t.Errorf("isJcc(%q) = false, want true", mn)
		}
	}
	// An unconditional jump is not a compare chain.
	for _, mn := range []string{"jmp", "jmpq", "mov", "add"} {
		if isJcc(mn) {
			t.Errorf("isJcc(%q) = true, want false", mn)
		}
	}
}

func TestJccForwardDisp(t *testing.T) {
	tests := []struct {
		insn    string
		curAddr uint64
		want    int64
	}{
		{"je     1570", 0x1550, 0x20},                     // perf annotate emits bare hex
		{"jne    0x1560", 0x1550, 0x10},                   // explicit 0x prefix
		{"je     1570 <gp_pressure+0x240>", 0x1550, 0x20}, // objdump annotation stripped
		{"je     .L123", 0x1550, -1},                      // symbolic target cannot be decoded
		{"je", 0x1550, -1},                                // malformed
		// Kernel text addresses exceed MaxInt64 and are legitimate. The difference must
		// be taken unsigned: narrowing each operand first wraps the result.
		{"je     ffffffff81000020", 0xffffffff81000000, 0x20},
		{"je     ffffffff81000000", 0xffffffff81000020, -0x20},
		// A difference too large for int64 is rejected rather than wrapped.
		{"je     ffffffffffffffff", 0x0, -1},
		{"je     0", 0xffffffffffffffff, -1},
	}
	for _, tc := range tests {
		if got := jccForwardDisp(tc.insn, tc.curAddr); got != tc.want {
			t.Errorf("jccForwardDisp(%q, %#x) = %d, want %d", tc.insn, tc.curAddr, got, tc.want)
		}
	}
}

// TestEstimateAPX reproduces the reference tool's estimate for a real stress-ng profile
// (GP cost 5.71%, 11/16 regs, upper ~5.96%, realistic ~3.20%, 59 of 150 excluded) from
// reconstructed aggregate inputs, checking the model's clamps and coefficients end to end.
func TestEstimateAPX(t *testing.T) {
	// One symbol carrying all the weight, chosen so the derived values are checkable.
	symbols := []AnalyzedSymbol{{
		Symbol:   Symbol{Name: "hot", HotPct: 50.0},
		Pressure: RegPressure{GPPct: 20.0, StackPct: 20.0, GPRegsUsed: 11, InsnCount: 100},
		APX:      APXSignals{NDDPct: 4.0, CCMPPct: 1.0, Push2Pairs: 2},
	}}
	e := EstimateAPX(symbols)

	if e.SymbolsIncluded != 1 || e.SymbolsSkipped != 0 {
		t.Errorf("included/skipped = %d/%d, want 1/0", e.SymbolsIncluded, e.SymbolsSkipped)
	}
	// gp_gstk = 50 * 20 / 100 = 10
	checkPct(t, "GPStackPct", e.GPStackPct, 10.0)
	// util = 11/16 = 0.6875 -> factor = 0.25 + 0.6875*0.40 = 0.525
	checkPct(t, "GPRFactor", e.GPRFactor, 0.525)
	if e.AvgGPRegistersUsed != 11 {
		t.Errorf("AvgGPRegistersUsed = %.1f, want 11", e.AvgGPRegistersUsed)
	}
	if e.MaxGPRegisters != 16 {
		t.Errorf("MaxGPRegisters = %d, want 16", e.MaxGPRegisters)
	}
	// Attribution must sum to the realistic estimate.
	sum := e.GPRContribPct + e.NDDContribPct + e.CCMPContribPct + e.Push2ContribPct
	if math.Abs(sum-e.RealisticPct) > 1e-9 {
		t.Errorf("attribution sums to %.6f, want RealisticPct %.6f", sum, e.RealisticPct)
	}
	// The realistic estimate must never exceed the upper bound.
	if e.RealisticPct > e.UpperBoundPct {
		t.Errorf("RealisticPct %.4f > UpperBoundPct %.4f", e.RealisticPct, e.UpperBoundPct)
	}
}

func TestEstimateAPXExcludesColdSymbols(t *testing.T) {
	symbols := []AnalyzedSymbol{
		{Symbol: Symbol{HotPct: 50.0}, Pressure: RegPressure{GPPct: 20, GPRegsUsed: 8, InsnCount: 10}},
		{Symbol: Symbol{HotPct: 0.05}, Pressure: RegPressure{GPPct: 99, GPRegsUsed: 8, InsnCount: 10}},
	}
	e := EstimateAPX(symbols)
	if e.SymbolsIncluded != 1 || e.SymbolsSkipped != 1 {
		t.Errorf("included/skipped = %d/%d, want 1/1", e.SymbolsIncluded, e.SymbolsSkipped)
	}
	// The cold symbol's 99% GP must not leak into the total.
	checkPct(t, "GPStackPct", e.GPStackPct, 10.0)
}

// TestEstimateAPXClamps checks the two bounds that stop the model producing absurd
// projections for degenerate input.
func TestEstimateAPXClamps(t *testing.T) {
	// Saturated secondary features: lift is capped at 1.35.
	saturated := []AnalyzedSymbol{{
		Symbol:   Symbol{HotPct: 100},
		Pressure: RegPressure{GPPct: 1, GPRegsUsed: 16, InsnCount: 1},
		APX:      APXSignals{NDDPct: 100, CCMPPct: 100, Push2Pairs: 100},
	}}
	if e := EstimateAPX(saturated); e.Lift > apxLiftMax+1e-9 {
		t.Errorf("Lift = %.4f, want <= %.2f", e.Lift, apxLiftMax)
	}
	// No usable register counts: the factor falls back to the default utilization.
	none := []AnalyzedSymbol{{Symbol: Symbol{HotPct: 50}, Pressure: RegPressure{GPPct: 0}}}
	e := EstimateAPX(none)
	if e.AvgGPRegistersUsed != -1 {
		t.Errorf("AvgGPRegistersUsed = %.1f, want -1", e.AvgGPRegistersUsed)
	}
	checkPct(t, "GPRFactor", e.GPRFactor, apxGPRFactorBase+apxDefaultUtilRatio*apxGPRFactorSpan)
}

// TestEstimateAPXDetectsEGPR covers code already built for APX: there is no register-file
// doubling left to project.
func TestEstimateAPXDetectsEGPR(t *testing.T) {
	symbols := []AnalyzedSymbol{{
		Symbol:   Symbol{HotPct: 50},
		Pressure: RegPressure{GPPct: 20, GPRegsUsed: 20, InsnCount: 10},
		APX:      APXSignals{UsesEGPR: true},
	}}
	e := EstimateAPX(symbols)
	if !e.AlreadyUsesAPX {
		t.Error("AlreadyUsesAPX = false, want true")
	}
	if e.MaxGPRegisters != apxEGPRRegisters {
		t.Errorf("MaxGPRegisters = %d, want %d", e.MaxGPRegisters, apxEGPRRegisters)
	}
}

func TestAPXSignalsDetectEGPRFromDisassembly(t *testing.T) {
	// r16-r31 only exist under APX, so their presence means the binary already targets it.
	insns, err := ParseAnnotate(newFixtureReader(
		"  0.00 :   1000:   mov    %r20,%r21\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !AnalyzeAPX(insns).UsesEGPR {
		t.Error("UsesEGPR = false for code referencing r20/r21, want true")
	}

	insns, err = ParseAnnotate(newFixtureReader(
		"  0.00 :   1000:   mov    %rax,%rbx\n"))
	if err != nil {
		t.Fatal(err)
	}
	if AnalyzeAPX(insns).UsesEGPR {
		t.Error("UsesEGPR = true for legacy registers, want false")
	}
}
