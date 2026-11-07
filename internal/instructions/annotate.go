// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package instructions

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Instruction is one line of sample-weighted disassembly from "perf annotate --stdio".
type Instruction struct {
	// Addr is the instruction address, valid only when HasAddr is true.
	Addr    uint64
	HasAddr bool
	// Pct is the share of the enclosing symbol's samples attributed to this instruction.
	// perf reports percentages local to the symbol, so these sum to ~100 across a symbol.
	Pct float64
	// Text is the instruction itself: everything after the final colon, trimmed.
	Text string
	// Raw is the unmodified source line. Needed because segment overrides such as
	// "%fs:0x28" are stripped when deriving Text, and canary detection depends on them.
	Raw string
}

// Symbol is one hot symbol: its identity and overall weight from pass 1 (perf report),
// plus its annotated disassembly from pass 2 (perf annotate) once that has been read.
type Symbol struct {
	Name string
	DSO  string
	Comm string
	Kind SymbolKind
	// HotPct is the symbol's share of all samples in the profile, from pass 1.
	HotPct float64
	// Insns is the symbol's disassembly, empty until pass 2 has been parsed for it.
	Insns []Instruction
	// Annotated records whether pass 2 produced any disassembly. A symbol can be hot but
	// unannotatable, most often because its binary is stripped.
	Annotated bool
}

var (
	// Instruction address: a run of 4+ hex digits followed by a colon. Anchored on a
	// word start so a percentage such as "10.00  :" is not mistaken for an address.
	reAnnAddr = regexp.MustCompile(`(^|[[:space:]])([0-9a-fA-F]{4,})[[:space:]]*:`)
	// Leading numeric run, for the per-line sample percentage.
	reLeadingNum = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?`)
)

// ParseAnnotate reads "perf annotate --stdio --no-source" output for one symbol. Every
// line is kept, headers and blanks included: analysis needs the raw sequence to detect
// the frame-pointer prologue and section boundaries.
func ParseAnnotate(r io.Reader) ([]Instruction, error) {
	var insns []Instruction
	scanner := bufio.NewScanner(r)
	// perf annotate lines are short, but raise the limit so a pathological disassembly
	// line cannot abort the parse.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		raw := scanner.Text()
		insn := Instruction{Raw: raw}

		if m := reAnnAddr.FindStringSubmatch(raw); m != nil {
			if addr, err := strconv.ParseUint(m[2], 16, 64); err == nil {
				insn.Addr = addr
				insn.HasAddr = true
			}
		}

		insn.Pct = parsePercent(raw)
		insn.Text = instructionText(raw)
		insns = append(insns, insn)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return insns, nil
}

// parsePercent extracts the leading sample percentage from a line shaped
// "  12.34 :   4a1b:   mov ...". A line whose first token is not numeric weighs 0.
func parsePercent(raw string) float64 {
	fields := reSplitSpace.Split(raw, -1)
	pct := ""
	if len(fields) > 0 {
		pct = fields[0]
	}
	if pct == "" && len(fields) > 1 {
		pct = fields[1]
	}
	m := reLeadingNum.FindString(pct)
	if m == "" {
		return 0
	}
	v, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0
	}
	return v
}

// instructionText returns the portion of the line after the final colon, which is the
// instruction proper. The final colon is used rather than the first because the address
// field itself contains one, and because perf prefixes a percentage with a colon too.
func instructionText(raw string) string {
	if i := strings.LastIndex(raw, ":"); i >= 0 {
		return strings.TrimLeft(raw[i+1:], " \t\n\v\f\r")
	}
	return strings.TrimLeft(raw, " \t\n\v\f\r")
}
