// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Package instructions analyzes the sample-weighted disassembly of hot code produced by
// "perf annotate --stdio", classifying individual instructions to answer questions about
// generated-code quality and instruction-set headroom.
//
// The register-pressure analysis is a port of the regpressure tool by Vaibhav Shankar,
// used with the author's permission. The classification rules below intentionally mirror
// that implementation, including its quirks, so results can be diffed against it.
//
// Assumes x86-64 AT&T syntax, which is what "perf annotate" emits by default.
package instructions

import (
	"regexp"
	"strings"
)

// unknownMnemonic is the sentinel for a line whose mnemonic could not be parsed.
const unknownMnemonic = "(unk)"

var (
	// mnemonic extraction: leading identifier, optionally after a lock/rep prefix.
	reMnemonic = regexp.MustCompile(`^[[:alpha:]][[:alnum:]_.]*`)
	rePrefix   = regexp.MustCompile(`^(lock|rep[ez]?|repne)$`)

	reLea      = regexp.MustCompile(`^lea[bwlq]?$`)
	reCmpTest  = regexp.MustCompile(`^(cmp|test)[bwlq]?$`)
	rePrefetch = regexp.MustCompile(`^prefetch`)
	rePush     = regexp.MustCompile(`^push[bwlq]?$`)
	rePop      = regexp.MustCompile(`^pop[bwlq]?$`)

	// read-modify-write mnemonics: these both load and store their memory operand.
	reRMW = regexp.MustCompile(`^(add|sub|inc|dec|and|or|xor|adc|sbb|` +
		`btc|bts|btr|neg|not|rol|ror|rcl|rcr|shl|shr|sar|sal|xchg|cmpxchg|xadd)[bwlq]?$`)

	// push/pop touch the stack regardless of operand form.
	rePushPopOperand = regexp.MustCompile(`(push|pop)[bwlq]?[ \t]`)

	// AT&T stack memory operands, e.g. -0x18(%rbp), 0x8(%rsp), (%rsp), -8(%rsp,%rax,4).
	reStackMemSP = regexp.MustCompile(`(-?0x[0-9a-fA-F]+|-?[0-9]+)?\(%[er]?sp(,[^)]+)?\)`)
	reStackMemBP = regexp.MustCompile(`(-?0x[0-9a-fA-F]+|-?[0-9]+)?\(%[er]?bp(,[^)]+)?\)`)
	// Intel-syntax bracket forms, tolerated because perf can be configured to emit them.
	reBracketSP = regexp.MustCompile(`\[[^]]*\b[er]sp\b[^]]*\]`)
	reBracketBP = regexp.MustCompile(`\[[^]]*\b[er]bp\b[^]]*\]`)

	// store: memory operand appears after a comma (AT&T destination).
	reStoreSP     = regexp.MustCompile(`,[[:space:]]*(-?0x[0-9a-fA-F]+|-?[0-9]+)?\(%[er]?sp(,[^)]+)?\)`)
	reStoreBP     = regexp.MustCompile(`,[[:space:]]*(-?0x[0-9a-fA-F]+|-?[0-9]+)?\(%[er]?bp(,[^)]+)?\)`)
	reLoadSP      = regexp.MustCompile(`(-?0x[0-9a-fA-F]+|-?[0-9]+)?\(%[er]?sp(,[^)]+)?\)[[:space:]]*,`)
	reLoadBP      = regexp.MustCompile(`(-?0x[0-9a-fA-F]+|-?[0-9]+)?\(%[er]?bp(,[^)]+)?\)[[:space:]]*,`)
	reStoreBrktSP = regexp.MustCompile(`\[[^]]*\b[er]sp\b[^]]*\][^,]*,`)
	reStoreBrktBP = regexp.MustCompile(`\[[^]]*\b[er]bp\b[^]]*\][^,]*,`)
	reLoadBrktSP  = regexp.MustCompile(`,[^[]*\[[^]]*\b[er]sp\b[^]]*\]`)
	reLoadBrktBP  = regexp.MustCompile(`,[^[]*\[[^]]*\b[er]bp\b[^]]*\]`)

	reSIMD = regexp.MustCompile(`(xmm|ymm|zmm)[0-9]+`)

	// Stack-canary detection. The canary is loaded through %fs:/%gs: and then compared
	// against a fixed stack slot; neither is register-allocator spill traffic.
	reCanarySegment = regexp.MustCompile(`%[fg]s:`)
	reCanaryOffset  = regexp.MustCompile(`(-?0x[0-9a-fA-F]+|-?[0-9]+)\(%[er]?sp`)

	// Frame-pointer prologue: "push %rbp" followed by "mov %rsp,%rbp".
	rePushRBP   = regexp.MustCompile(`^push[ql]?[[:space:]]+%rbp[[:space:]]*$`)
	reMovRSPRBP = regexp.MustCompile(`^mov[ql]?[[:space:]]+%rsp[[:space:]]*,[[:space:]]*%rbp`)

	// GP register references, widest form first.
	reGP64  = regexp.MustCompile(`%r(ax|bx|cx|dx|si|di|bp|sp|[89]|[12][0-9]|1[0-5]|3[01])`)
	reGP32e = regexp.MustCompile(`%e(ax|bx|cx|dx|si|di|bp|sp)`)
	reGP32r = regexp.MustCompile(`%r([89]|[12][0-9]|1[0-5]|3[01])d`)
	reGP16  = regexp.MustCompile(`%((ax|bx|cx|dx|si|di|bp|sp)|r([89]|[12][0-9]|1[0-5]|3[01])w)`)
	reGP8a  = regexp.MustCompile(`%[abcd][lh]`)
	reGP8b  = regexp.MustCompile(`%(sil|dil|bpl|spl)`)
	reGP8c  = regexp.MustCompile(`%r([89]|[12][0-9]|1[0-5]|3[01])b`)

	// Register-name capture for the live-register census.
	reAnyReg = regexp.MustCompile(`%([a-zA-Z][a-zA-Z0-9]*)`)
	reSIMDNo = regexp.MustCompile(`^[xyz]mm([0-9]+)$`)

	reSplitSpace = regexp.MustCompile(`[[:space:]]+`)
)

// memDirection describes whether an instruction's stack-memory operand is read or written.
type memDirection int

const (
	memNone  memDirection = 0 // not a memory access
	memLoad  memDirection = 1 // reads memory  -> register reload
	memStore memDirection = 2 // writes memory -> register spill
)

// mnemonic extracts the instruction mnemonic, skipping a lock/rep prefix if present.
// Returns unknownMnemonic when no identifier can be found.
func mnemonic(s string) string {
	tmp := strings.TrimLeft(s, " \t\n\v\f\r")
	m := reMnemonic.FindString(tmp)
	if m == "" {
		return unknownMnemonic
	}
	if rePrefix.MatchString(m) {
		rest := strings.TrimLeft(tmp[len(m):], " \t\n\v\f\r")
		if m2 := reMnemonic.FindString(rest); m2 != "" {
			return m2
		}
	}
	return m
}

// isRMW reports whether the mnemonic both reads and writes its memory operand.
func isRMW(mn string) bool { return reRMW.MatchString(mn) }

// isSIMD reports whether the instruction references a vector register.
func isSIMD(s string) bool { return reSIMD.MatchString(s) }

// isCanaryLine reports whether the instruction loads through a segment override, which is
// how the stack canary is read.
func isCanaryLine(s string) bool { return reCanarySegment.MatchString(s) }

// canaryOffset returns the stack offset the canary is stored at, or "" if not found.
func canaryOffset(s string) string {
	if m := reCanaryOffset.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// isCanaryOffsetAccess reports whether the instruction touches the known canary slot.
func isCanaryOffsetAccess(s, off string) bool {
	if off == "" {
		return false
	}
	return strings.Contains(s, off+"(%rsp)") || strings.Contains(s, off+"(%esp)")
}

// hasStackMem reports whether the instruction references rsp- or rbp-relative stack memory.
// rbp-relative accesses only count when rbp is in use as a real frame pointer; otherwise
// rbp is just another general-purpose register.
func hasStackMem(s string, useRBP bool) bool {
	if reLea.MatchString(mnemonic(s)) {
		return false // lea computes an address without accessing memory
	}
	if rePushPopOperand.MatchString(s) {
		return true
	}
	if reStackMemSP.MatchString(s) || reBracketSP.MatchString(s) {
		return true
	}
	if useRBP && (reStackMemBP.MatchString(s) || reBracketBP.MatchString(s)) {
		return true
	}
	return false
}

// memDir classifies the direction of an instruction's stack-memory access.
func memDir(s string, useRBP bool) memDirection {
	mn := mnemonic(s)
	switch {
	case reLea.MatchString(mn):
		return memNone // address computation only
	case reCmpTest.MatchString(mn):
		return memLoad // cmp/test never write memory
	case rePrefetch.MatchString(mn):
		return memNone // cache hint
	case rePush.MatchString(mn):
		return memStore
	case rePop.MatchString(mn):
		return memLoad
	}

	if reStoreBrktSP.MatchString(s) || (useRBP && reStoreBrktBP.MatchString(s)) {
		return memStore
	}
	if reLoadBrktSP.MatchString(s) || (useRBP && reLoadBrktBP.MatchString(s)) {
		return memLoad
	}
	if reStoreSP.MatchString(s) || (useRBP && reStoreBP.MatchString(s)) {
		return memStore
	}
	if reLoadSP.MatchString(s) || (useRBP && reLoadBP.MatchString(s)) {
		return memLoad
	}
	// Single-operand form: the memory reference is the only operand, treated as a read.
	if !strings.Contains(s, ",") {
		if reStackMemSP.MatchString(s) || (useRBP && reStackMemBP.MatchString(s)) {
			return memLoad
		}
	}
	return memNone
}

// hasGPReg reports whether the operand implicated by dir names a general-purpose register.
// For a store the relevant operand is the AT&T source (before the first comma); for a load
// it is the destination (after the last comma). memNone examines the whole instruction.
func hasGPReg(s string, dir memDirection) bool {
	if rePushPopOperand.MatchString(s) {
		return true
	}

	var operand string
	switch dir {
	case memStore:
		if i := strings.Index(s, ","); i >= 0 {
			operand = s[:i]
		} else {
			operand = s
		}
	case memLoad:
		if i := strings.LastIndex(s, ","); i >= 0 {
			operand = s[i+1:]
		} else {
			operand = ""
		}
	default:
		operand = s
	}

	if strings.Contains(operand, "$") {
		return false // immediate, not a register
	}
	return reGP64.MatchString(operand) ||
		reGP32e.MatchString(operand) || reGP32r.MatchString(operand) ||
		reGP16.MatchString(operand) ||
		reGP8a.MatchString(operand) || reGP8b.MatchString(operand) || reGP8c.MatchString(operand)
}

// Register-name normalization patterns. "rN" covers r8-r31 so that APX's extended GP
// register file is recognized as well as the legacy set.
var (
	reNamed64 = regexp.MustCompile(`^r(ax|bx|cx|dx|si|di|bp|sp)$`)
	reNum64   = regexp.MustCompile(`^r([89]|1[0-5]|1[6-9]|2[0-9]|3[01])$`)
	reNamed32 = regexp.MustCompile(`^e(ax|bx|cx|dx|si|di|bp|sp)$`)
	reNum32   = regexp.MustCompile(`^r([89]|1[0-5]|1[6-9]|2[0-9]|3[01])d$`)
	reNamed16 = regexp.MustCompile(`^(ax|bx|cx|dx|si|di|bp|sp)$`)
	reNum16   = regexp.MustCompile(`^r([89]|1[0-5]|1[6-9]|2[0-9]|3[01])w$`)
	reNum8    = regexp.MustCompile(`^r([89]|1[0-5]|1[6-9]|2[0-9]|3[01])b$`)
)

// normalizeGPReg maps a register name of any width onto its 64-bit name, e.g. eax -> rax,
// r9d -> r9, sil -> rsi. Returns "" when the name is not a general-purpose register.
func normalizeGPReg(r string) string {
	switch {
	case reNamed64.MatchString(r), reNum64.MatchString(r):
		return r
	case reNamed32.MatchString(r):
		return "r" + r[1:]
	case reNum32.MatchString(r), reNum16.MatchString(r), reNum8.MatchString(r):
		return r[:len(r)-1]
	case reNamed16.MatchString(r):
		return "r" + r
	}
	switch r {
	case "al", "ah":
		return "rax"
	case "bl", "bh":
		return "rbx"
	case "cl", "ch":
		return "rcx"
	case "dl", "dh":
		return "rdx"
	case "sil":
		return "rsi"
	case "dil":
		return "rdi"
	case "bpl":
		return "rbp"
	case "spl":
		return "rsp"
	}
	return ""
}
