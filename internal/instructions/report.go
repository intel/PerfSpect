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

// SymbolKind distinguishes where a hot symbol lives, taken from perf's level marker.
type SymbolKind string

const (
	// SymbolKindUser is a userspace symbol, perf's "[.]".
	SymbolKindUser SymbolKind = "user"
	// SymbolKindKernel is a kernel symbol, perf's "[k]".
	SymbolKindKernel SymbolKind = "kernel"
	// SymbolKindOther covers perf's remaining markers, e.g. "[g]", "[u]", "[?]".
	SymbolKindOther SymbolKind = "other"
)

// perf report --stdio --no-children --sort comm,dso,sym emits rows shaped:
//
//	12.20%  stress-ng-cpu  stress-ng          [.] ccitt_crc16
//	 7.49%  java           [JIT] tid 4029471  [.] 0x000077a2dc2e16eb
//
// Neither the DSO nor the symbol can be taken as a whitespace-delimited field: perf names
// JIT regions "[JIT] tid <n>", which contains spaces, and C++ symbols contain spaces too.
// The single-character level marker is the only reliable anchor, so the DSO is matched
// lazily up to it and the symbol greedily after it. Bracketed pseudo-DSOs such as
// "[kernel.kallsyms]" cannot be mistaken for the marker, which holds exactly one
// character between its brackets.
var reReportRow = regexp.MustCompile(
	`^[[:space:]]*([0-9]+\.[0-9]+)%[[:space:]]+(\S+)[[:space:]]+(.+?)[[:space:]]+\[([a-zA-Z.?])\][[:space:]]+(.+?)[[:space:]]*$`)

// ParseReport reads "perf report --stdio --no-children --sort comm,dso,sym" output and
// returns the hot symbols it lists, in the order perf emitted them (descending by
// overhead). Comment and header lines are ignored.
//
// Only HotPct, Name, DSO and Comm are populated; Insns is filled in later by pairing each
// symbol with its own "perf annotate" output.
func ParseReport(r io.Reader) ([]Symbol, error) {
	var symbols []Symbol
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		m := reReportRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pct, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		symbols = append(symbols, Symbol{
			HotPct: pct,
			Comm:   m[2],
			DSO:    m[3],
			Kind:   symbolKindFromMarker(m[4]),
			Name:   m[5],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return symbols, nil
}

func symbolKindFromMarker(marker string) SymbolKind {
	switch marker {
	case ".":
		return SymbolKindUser
	case "k":
		return SymbolKindKernel
	default:
		return SymbolKindOther
	}
}

// IsUnresolved reports whether perf failed to attribute the symbol to a name, leaving a
// raw instruction address instead. Such entries cannot be annotated.
func (s Symbol) IsUnresolved() bool {
	return strings.HasPrefix(s.Name, "0x") || s.DSO == "[unknown]"
}

// GlobalStackPct is the symbol's share of total profile weight that lands on stack-memory
// instructions: HotPct scaled by the symbol-local StackPct.
//
// This is the column to rank by. A symbol can be very hot with little stack traffic, or
// carry heavy spill code while being too cold to matter; neither HotPct nor StackPct alone
// orders symbols by how much total time is actually spent on spill and reload.
func GlobalStackPct(hotPct float64, p RegPressure) float64 {
	return hotPct * p.StackPct / 100
}
