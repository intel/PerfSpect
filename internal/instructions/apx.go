// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package instructions

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// APXSignals counts the code patterns Intel APX could encode more efficiently. These are
// proxies for opportunity, not measurements of a speedup; see EstimateAPX.
type APXSignals struct {
	// NDDPct is the share of sample weight on a confirmed copy-then-modify pair: a
	// register-to-register mov immediately followed by an ALU op on the copy. APX's
	// non-destructive destination encoding removes the mov.
	NDDPct float64
	// CCMPPct is the share of instructions forming a short forward conditional branch
	// over a cmp/test, which APX's conditional compare can fold into one instruction.
	CCMPPct float64
	// Push2Pairs counts consecutive general-purpose pushes in the prologue, which APX's
	// PUSH2/POP2 can halve. A raw count, not a percentage.
	Push2Pairs int
	// UsesEGPR reports whether the code references r16-r31, meaning it is already built
	// for APX and has 32 general-purpose registers rather than 16.
	UsesEGPR bool
}

var (
	rePlainMov     = regexp.MustCompile(`^mov[bwlq]?$`)
	reNDDAlu       = regexp.MustCompile(`^(add|sub|and|or|xor|adc|sbb|imul|sar|shr|shl|sal|ror|rol|rcl|rcr)[bwlq]?$`)
	reNDDAlu1Src   = regexp.MustCompile(`^(inc|dec|neg|not)[bwlq]?$`)
	reJmp          = regexp.MustCompile(`^jmp[ql]?$`)
	reJcc          = regexp.MustCompile(`^j[a-z]+$`)
	rePushPrologue = regexp.MustCompile(`^push[ql]?$`)
	reEndbr        = regexp.MustCompile(`^endbr(32|64)?$`)
	reStackPtrRef  = regexp.MustCompile(`[er]?sp`)

	// Destination operand of a two-operand AT&T instruction: after the last comma.
	reDstOperand = regexp.MustCompile(`,[[:space:]]*%([a-zA-Z][a-zA-Z0-9]*)([[:space:]].*)?$`)
	// Sole register operand of a one-operand instruction.
	reSoleOperand = regexp.MustCompile(`[[:space:]]+%([a-zA-Z][a-zA-Z0-9]*)([[:space:]].*)?$`)
	// Branch target: perf annotate emits bare hex, objdump may add <sym+off>.
	reBranchTarget = regexp.MustCompile(`^(0[xX])?([0-9a-fA-F]+)$`)

	// Extended general-purpose registers introduced by APX.
	reEGPR = regexp.MustCompile(`%r(1[6-9]|2[0-9]|3[01])\b`)
)

// ccmpMaxDisplacement bounds how far forward a conditional branch may jump and still be
// considered a foldable compare chain.
const ccmpMaxDisplacement = 32

// nddMovDst returns the normalized destination register of a plain register-to-register
// mov, or "" if the instruction is not a candidate for APX's non-destructive destination
// form. Sign/zero-extending moves, memory operands, immediates and SIMD are all excluded.
func nddMovDst(s string) string {
	if !rePlainMov.MatchString(mnemonic(s)) {
		return ""
	}
	if isSIMD(s) || hasStackMemAny(s) {
		return ""
	}
	if strings.ContainsAny(s, "$([") {
		return "" // immediate or memory operand
	}
	if !hasGPReg(s, memStore) || !hasGPReg(s, memLoad) {
		return "" // needs a GP register on both sides
	}
	m := reDstOperand.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return normalizeGPReg(m[1])
}

// aluWritesReg reports whether the instruction is an APX-eligible destructive ALU op whose
// destination is reg. Paired with nddMovDst, this identifies a copy-then-modify sequence.
func aluWritesReg(s, reg string) bool {
	mn := mnemonic(s)
	twoSrc := reNDDAlu.MatchString(mn)
	oneSrc := reNDDAlu1Src.MatchString(mn)
	if !twoSrc && !oneSrc {
		return false
	}
	if isSIMD(s) {
		return false
	}

	var m []string
	if oneSrc {
		if hasStackMemAny(s) {
			return false // operating on memory, not on the copied register
		}
		m = reSoleOperand.FindStringSubmatch(s)
	} else {
		m = reDstOperand.FindStringSubmatch(s)
	}
	if m == nil {
		return false
	}
	return normalizeGPReg(m[1]) == reg
}

// isJcc reports whether the mnemonic is a conditional branch. An unconditional jmp is not
// a compare chain.
func isJcc(mn string) bool { return !reJmp.MatchString(mn) && reJcc.MatchString(mn) }

// isCmpTest reports whether the mnemonic seeds a compare chain.
func isCmpTest(mn string) bool { return reCmpTest.MatchString(mn) }

// jccForwardDisp returns the branch displacement in bytes, or -1 when the target cannot be
// decoded. perf annotate emits bare hex targets; objdump may append a <sym+off> comment.
func jccForwardDisp(s string, curAddr uint64) int64 {
	target := s
	// Drop the mnemonic.
	if i := strings.IndexAny(target, " \t"); i >= 0 {
		target = target[i+1:]
	} else {
		return -1
	}
	// Drop any <sym+offset> annotation.
	if i := strings.Index(target, "<"); i >= 0 {
		target = target[:i]
	}
	target = strings.TrimSpace(target)

	m := reBranchTarget.FindStringSubmatch(target)
	if m == nil {
		return -1
	}
	val, err := strconv.ParseUint(m[2], 16, 64)
	if err != nil {
		return -1
	}
	// Kernel text addresses exceed MaxInt64, so the difference is taken unsigned and
	// narrowed only once it is known to fit.
	if val >= curAddr {
		if d := val - curAddr; d <= math.MaxInt64 {
			return int64(d)
		}
		return -1
	}
	if d := curAddr - val; d <= math.MaxInt64 {
		return -int64(d)
	}
	return -1
}

// hasStackMemAny reports whether the instruction references rsp- or rbp-relative memory,
// regardless of whether rbp is a frame pointer. The APX heuristics care only about whether
// an operand is in memory at all, not about whose frame it belongs to.
func hasStackMemAny(s string) bool { return hasStackMem(s, true) }

// apxState carries the lookahead needed to recognize multi-instruction APX patterns.
type apxState struct {
	// Copy-then-modify lookahead: the destination register of the preceding mov and the
	// weight to credit if the next instruction modifies it.
	nddPendingReg string
	nddPendingW   float64
	nddPairsTotal float64

	// Compare-chain lookahead: whether the preceding instruction was a short forward jcc.
	prevWasJcc  bool
	prevJccDisp int64
	ccmpChains  int

	// Prologue tracking for PUSH2/POP2.
	inPrologue     bool
	prologuePushes int

	usesEGPR bool
}

func newAPXState() *apxState {
	return &apxState{inPrologue: true, prevJccDisp: -1}
}

// reset clears lookahead at a section boundary so patterns cannot straddle two functions.
func (st *apxState) reset() {
	st.nddPendingReg = ""
	st.nddPendingW = 0
	st.prevWasJcc = false
	st.prevJccDisp = -1
	st.inPrologue = true
	st.prologuePushes = 0
}

// observe folds one classifiable instruction into the APX pattern state.
func (st *apxState) observe(text, mn string, w float64, addr uint64, hasAddr bool) {
	if reEGPR.MatchString(text) {
		st.usesEGPR = true
	}

	// NDD: did the previous mov's destination get modified right away?
	if st.nddPendingReg != "" {
		if aluWritesReg(text, st.nddPendingReg) {
			st.nddPairsTotal += st.nddPendingW
		}
		// Only the immediately following instruction counts.
		st.nddPendingReg = ""
		st.nddPendingW = 0
	}
	if dst := nddMovDst(text); dst != "" && dst != "rsp" && dst != "rbp" && w > 0 {
		st.nddPendingReg = dst
		st.nddPendingW = w
	}

	// PUSH2: consecutive GP pushes before the frame is set up.
	if st.inPrologue {
		switch {
		case rePushPrologue.MatchString(mn) && !strings.ContainsAny(text, "$(") && hasGPReg(text, memNone):
			st.prologuePushes++
		case reNDDAlu.MatchString(mn) && reStackPtrRef.MatchString(text):
			// stack adjustment, still in the prologue
		case rePlainMov.MatchString(mn) && reStackPtrRef.MatchString(text):
			// frame pointer setup, still in the prologue
		case reEndbr.MatchString(mn):
			// CET landing pad, does not close the prologue
		default:
			st.inPrologue = false
		}
	}

	// CCMP: a short forward conditional branch followed by the cmp/test it guards.
	if st.prevWasJcc {
		if isCmpTest(mn) && st.prevJccDisp > 0 && st.prevJccDisp <= ccmpMaxDisplacement {
			st.ccmpChains++
		}
		st.prevWasJcc = false
		st.prevJccDisp = -1
	}
	if isJcc(mn) && hasAddr {
		if d := jccForwardDisp(text, addr); d > 0 && d <= ccmpMaxDisplacement {
			st.prevWasJcc = true
			st.prevJccDisp = d
		}
	}
}

// signals converts the accumulated state into the reported percentages. annTotal is the
// symbol's total sample weight and insnCount its classifiable instruction count.
func (st *apxState) signals(annTotal float64, insnCount int) APXSignals {
	var s APXSignals
	if annTotal > 0 {
		s.NDDPct = st.nddPairsTotal / annTotal * 100
	}
	if insnCount > 0 {
		s.CCMPPct = float64(st.ccmpChains) / float64(insnCount) * 100
	}
	s.Push2Pairs = st.prologuePushes / 2
	s.UsesEGPR = st.usesEGPR
	return s
}

// Coefficients of the APX benefit model, ported from the reference implementation. They
// are judgement calls in that tool, not measurements: see APXEstimate.
const (
	// APXMinSamplePct excludes symbols too cold for their percentages to mean anything.
	APXMinSamplePct = 0.1
	// The realized fraction of GP spill traffic APX recovers, interpolated by how
	// saturated the register file already is, then clamped.
	apxGPRFactorBase = 0.25
	apxGPRFactorSpan = 0.40
	apxGPRFactorMax  = 0.65
	// Multiplicative uplift contributed by each secondary feature at full saturation.
	apxNDDWeight   = 0.22
	apxCCMPWeight  = 0.08
	apxPush2Weight = 0.03
	apxLiftMax     = 1.35
	// Assumed register-file utilization when no symbol reported a usable count.
	apxDefaultUtilRatio = 0.50
	// Register file sizes without and with APX.
	apxBaseGPRegisters = 16
	apxEGPRRegisters   = 32
)

// APXEstimate projects what Intel APX might recover from the profiled code. It is a model
// scaling observed GP spill traffic by assumed coefficients, not validated on APX hardware:
// read it as whether a workload is worth investigating, not as a predicted speedup.
type APXEstimate struct {
	// MaxGPRegisters is the register file the profiled code currently targets: 32 if it
	// already uses r16-r31, otherwise 16.
	MaxGPRegisters int
	// AlreadyUsesAPX reports whether the hot code references extended registers, in which
	// case there is no doubling left to gain.
	AlreadyUsesAPX bool

	// GPStackPct is the share of total profile weight spent on GP spill and reload.
	// It is the ceiling the model works from.
	GPStackPct float64
	// UpperBoundPct assumes every GP spill is recovered, plus the secondary uplift.
	UpperBoundPct float64
	// RealisticPct applies the realization factor, and is the headline figure.
	RealisticPct float64

	// GPRFactor is the realization factor applied to GPStackPct.
	GPRFactor float64
	// Lift is the multiplier contributed by NDD, CCMP and PUSH2 together.
	Lift float64
	// AvgGPRegistersUsed is the spill-weighted mean of distinct GP registers per symbol,
	// or -1 when no symbol reported one.
	AvgGPRegistersUsed float64

	// Contributions attribute RealisticPct across the features, and sum to it.
	GPRContribPct   float64
	NDDContribPct   float64
	CCMPContribPct  float64
	Push2ContribPct float64

	// SymbolsIncluded and SymbolsSkipped record the population the model saw.
	SymbolsIncluded int
	SymbolsSkipped  int
}

// EstimateAPX applies the benefit model across the analyzed symbols.
func EstimateAPX(symbols []AnalyzedSymbol) APXEstimate {
	var (
		gpGstk, nddGstk, ccmpGstk, p2Gstk float64
		weightedGPSum, gpGstkWeight       float64
		est                               APXEstimate
	)
	est.MaxGPRegisters = apxBaseGPRegisters

	for _, s := range symbols {
		if s.HotPct < APXMinSamplePct {
			est.SymbolsSkipped++
			continue
		}
		est.SymbolsIncluded++
		if s.APX.UsesEGPR {
			est.AlreadyUsesAPX = true
		}

		gpGstkI := s.HotPct * s.Pressure.GPPct / 100
		gpGstk += gpGstkI
		nddGstk += s.HotPct * s.APX.NDDPct / 100
		ccmpGstk += s.HotPct * s.APX.CCMPPct / 100
		if s.Pressure.InsnCount > 0 && s.APX.Push2Pairs > 0 {
			p2Gstk += s.HotPct * (float64(s.APX.Push2Pairs) / float64(s.Pressure.InsnCount))
		}
		// Weight the register-file utilization by each symbol's spill contribution, so
		// hot spill-heavy code dominates the realization factor.
		if gpGstkI > 0 && s.Pressure.GPRegsUsed >= 0 {
			weightedGPSum += gpGstkI * float64(s.Pressure.GPRegsUsed)
			gpGstkWeight += gpGstkI
		}
	}

	if est.AlreadyUsesAPX {
		est.MaxGPRegisters = apxEGPRRegisters
	}

	utilRatio := apxDefaultUtilRatio
	est.AvgGPRegistersUsed = -1
	if gpGstkWeight > 0 {
		est.AvgGPRegistersUsed = weightedGPSum / gpGstkWeight
		utilRatio = est.AvgGPRegistersUsed / float64(est.MaxGPRegisters)
	}

	est.GPRFactor = min(max(apxGPRFactorBase+utilRatio*apxGPRFactorSpan, apxGPRFactorBase), apxGPRFactorMax)
	gprBase := gpGstk * est.GPRFactor

	// Secondary features are expressed relative to the GP spill traffic they accompany.
	denom := max(gpGstk, 0.01)
	nddLift := min(nddGstk/denom, 1.0) * apxNDDWeight
	ccmpLift := min(ccmpGstk/denom, 1.0) * apxCCMPWeight
	p2Lift := min(p2Gstk/denom, 1.0) * apxPush2Weight
	est.Lift = min(1.0+nddLift+ccmpLift+p2Lift, apxLiftMax)

	est.GPStackPct = gpGstk
	est.RealisticPct = gprBase * est.Lift
	est.UpperBoundPct = gpGstk * est.Lift
	est.GPRContribPct = gprBase
	est.NDDContribPct = gprBase * nddLift
	est.CCMPContribPct = gprBase * ccmpLift
	est.Push2ContribPct = gprBase * p2Lift
	return est
}
