// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package instructions

import (
	"regexp"
	"sort"
	"strings"
)

// Pressure tier thresholds, expressed as a percentage of annotated sample weight landing
// on stack-memory instructions.
const (
	TierHighThreshold = 15.0
	TierMedThreshold  = 5.0
)

// Pressure tier labels.
const (
	TierHigh = "HIGH"
	TierMed  = "MED"
	TierLow  = "LOW"
)

// HotOp is a single stack-memory instruction and the sample weight attributed to it.
type HotOp struct {
	Mnemonic string
	Text     string
	Pct      float64
}

// RegPressure is the register-pressure profile of one symbol.
//
// All percentages are shares of the symbol's annotated sample weight. Note that StackPct
// can exceed the sum of SpillPct and ReloadPct would suggest, and can exceed 100: a
// read-modify-write instruction such as "xor %r15,-0x50(%rbp)" both spills and reloads,
// so it is counted in both and contributes twice to StackPct. This matches the reference
// implementation.
type RegPressure struct {
	StackPct   float64 // share of weight on stack-memory instructions
	SpillPct   float64 // share of weight on stores to the frame
	ReloadPct  float64 // share of weight on loads from the frame
	GPPct      float64 // share of weight on GP-register spill/reload
	SIMDPct    float64 // share of weight on vector-register spill/reload
	UnknownPct float64 // share of weight where the access direction is ambiguous

	GPRegsUsed   int // distinct GP registers referenced anywhere in the symbol
	SIMDRegsUsed int // distinct vector registers referenced anywhere in the symbol
	InsnCount    int // instructions considered (excludes headers, blanks, canary traffic)

	// UsesFramePointer records whether rbp was detected as a frame pointer. When false,
	// rbp-relative accesses are not treated as stack traffic.
	UsesFramePointer bool

	// TopOps lists the heaviest stack-memory instructions, descending by weight.
	TopOps []HotOp
}

// Tier classifies the symbol's register pressure for display.
func (p RegPressure) Tier() string {
	switch {
	case p.StackPct >= TierHighThreshold:
		return TierHigh
	case p.StackPct >= TierMedThreshold:
		return TierMed
	default:
		return TierLow
	}
}

var (
	reSkipHeader  = regexp.MustCompile(`^[[:space:]]*(Percent|Disassembly|/\*)`)
	reSectionHead = regexp.MustCompile(`^[[:space:]]*<`)
	reBlank       = regexp.MustCompile(`^[[:space:]]*$`)
)

// AnalyzeRegPressure computes the register-pressure profile of one symbol's annotated
// disassembly. topOps caps the number of hot stack instructions returned; pass 0 to skip
// collecting them.
func AnalyzeRegPressure(insns []Instruction, topOps int) RegPressure {
	p, _ := analyzeInstructions(insns, topOps)
	return p
}

// AnalyzeAPX counts the APX opportunity signals in one symbol's annotated disassembly.
func AnalyzeAPX(insns []Instruction) APXSignals {
	_, a := analyzeInstructions(insns, 0)
	return a
}

// analyzeInstructions walks a symbol's disassembly once, deriving both the
// register-pressure profile and the APX signals.
//
// The two share a single pass deliberately: they must agree on exactly which lines count
// as instructions, since both divide by that population. Splitting the filtering across
// two loops would let them drift apart.
func analyzeInstructions(insns []Instruction, topOps int) (RegPressure, APXSignals) {
	useRBP := detectFramePointer(insns)
	apx := newAPXState()

	var (
		annTotal  float64
		stackTot  float64
		spillTot  float64
		reloadTot float64
		gpTot     float64
		simdTot   float64
		unkTot    float64
		insnCount int
	)
	gpSeen := make(map[string]struct{})
	simdSeen := make(map[string]struct{})
	opWeight := make(map[HotOp]float64)

	// Stack-canary state. The canary is loaded via a segment override, stashed at a fixed
	// frame slot, and compared on exit. None of that is allocator spill traffic, so the
	// slot is identified and then excluded.
	canaryOff := ""
	sawCanaryLoad := false

	for _, in := range insns {
		raw := in.Raw
		if reBlank.MatchString(raw) || reSkipHeader.MatchString(raw) {
			continue
		}
		if reSectionHead.MatchString(raw) {
			// A new function or section header. Multi-instruction APX patterns must not
			// straddle it, so the lookahead is cleared.
			apx.reset()
			continue
		}
		if !strings.Contains(raw, ":") {
			continue
		}

		text := in.Text
		if text == "" || startsWithComment(text) {
			continue
		}

		// Canary detection runs against the raw line: deriving Text strips through the
		// final colon, which removes the "%fs:" that identifies the load.
		if isCanaryLine(raw) {
			sawCanaryLoad = true
			continue
		}

		mn := mnemonic(text)
		if mn == unknownMnemonic {
			continue
		}

		// The instruction immediately after the canary load stores it to its frame slot.
		// Record that offset so later accesses to it can be excluded.
		if sawCanaryLoad {
			sawCanaryLoad = false
			if off := canaryOffset(text); off != "" {
				canaryOff = off
				continue
			}
		}
		if canaryOff != "" && isCanaryOffsetAccess(text, canaryOff) {
			continue
		}

		w := in.Pct
		annTotal += w
		insnCount++
		collectRegs(text, useRBP, gpSeen, simdSeen)
		apx.observe(text, mn, w, in.Addr, in.HasAddr)

		if !hasStackMem(text, useRBP) {
			continue
		}

		dir := memDir(text, useRBP)
		simd := isSIMD(text)
		gp := hasGPReg(text, dir)

		switch {
		case isRMW(mn) && dir != memNone:
			// Read-modify-write: one instruction, both a reload and a spill.
			stackTot += w * 2
			spillTot += w
			reloadTot += w
			if simd {
				simdTot += w * 2
			} else if gp {
				gpTot += w * 2
			}
		case dir == memStore:
			stackTot += w
			spillTot += w
			if simd {
				simdTot += w
			} else if gp {
				gpTot += w
			}
		case dir == memLoad:
			stackTot += w
			reloadTot += w
			if simd {
				simdTot += w
			} else if gp {
				gpTot += w
			}
		default:
			stackTot += w
			unkTot += w
		}

		if topOps > 0 {
			opWeight[HotOp{Mnemonic: mn, Text: text}] += w
		}
	}

	base := annTotal
	if base <= 0 {
		base = 1
	}

	p := RegPressure{
		StackPct:         stackTot / base * 100,
		SpillPct:         spillTot / base * 100,
		ReloadPct:        reloadTot / base * 100,
		GPPct:            gpTot / base * 100,
		SIMDPct:          simdTot / base * 100,
		UnknownPct:       unkTot / base * 100,
		GPRegsUsed:       len(gpSeen),
		SIMDRegsUsed:     len(simdSeen),
		InsnCount:        insnCount,
		UsesFramePointer: useRBP,
	}
	if topOps > 0 {
		p.TopOps = rankOps(opWeight, annTotal, topOps)
	}
	return p, apx.signals(annTotal, insnCount)
}

// rankOps returns the heaviest ops, descending by weight, with percentages relative to
// the symbol's total annotated weight.
func rankOps(weights map[HotOp]float64, annTotal float64, limit int) []HotOp {
	if len(weights) == 0 {
		return nil
	}
	ops := make([]HotOp, 0, len(weights))
	for op, w := range weights {
		if annTotal > 0 {
			op.Pct = w / annTotal * 100
		}
		ops = append(ops, op)
	}
	// Sort by weight descending, then by text so equal weights order deterministically.
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Pct != ops[j].Pct {
			return ops[i].Pct > ops[j].Pct
		}
		return ops[i].Text < ops[j].Text
	})
	if len(ops) > limit {
		ops = ops[:limit]
	}
	return ops
}

// detectFramePointer reports whether the symbol establishes rbp as a frame pointer, by
// looking for "push %rbp" followed by "mov %rsp,%rbp". Without that prologue, rbp is an
// ordinary general-purpose register and rbp-relative accesses are not stack traffic.
func detectFramePointer(insns []Instruction) bool {
	sawPush := false
	for _, in := range insns {
		line := in.Text
		if line == "" || startsWithComment(line) {
			continue
		}
		if rePushRBP.MatchString(line) {
			sawPush = true
			continue
		}
		if sawPush && reMovRSPRBP.MatchString(line) {
			return true
		}
	}
	return false
}

// collectRegs records every distinct register the instruction references. rsp is always
// excluded, and rbp is excluded when it serves as the frame pointer, since neither is
// available to the register allocator in that role.
func collectRegs(s string, useRBP bool, gpSeen, simdSeen map[string]struct{}) {
	for _, m := range reAnyReg.FindAllStringSubmatch(s, -1) {
		r := m[1]
		if sm := reSIMDNo.FindStringSubmatch(r); sm != nil {
			simdSeen[sm[1]] = struct{}{}
			continue
		}
		norm := normalizeGPReg(r)
		if norm == "" || norm == "rsp" {
			continue
		}
		if useRBP && norm == "rbp" {
			continue
		}
		gpSeen[norm] = struct{}{}
	}
}

// startsWithComment reports whether the text is a comment or label rather than an
// instruction.
func startsWithComment(s string) bool {
	if s == "" {
		return false
	}
	switch s[0] {
	case '#', '<', ';':
		return true
	case '/':
		return strings.HasPrefix(s, "//")
	}
	return false
}
