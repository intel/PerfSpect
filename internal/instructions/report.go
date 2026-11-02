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

// Rows from "perf report -t '|'" are pipe-delimited:
//
//	12.20% |stress-ng-cpu|stress-ng            |[.] ccitt_crc16
//	42.02% |java         |jitted-4148375-749.so|[.] CryptoBench.crypt(java.lang.String)
//	 0.26% |GC Thread#5  |libjvm.so            |[.] G1ParScanThreadState::trim_queue(...)
//
// An explicit separator is essential rather than convenient. Every one of the three
// name fields can contain spaces: Spark names threads "GC Thread#5" and "task l", perf
// names JIT regions "[JIT] tid <n>", and C++ symbols are full signatures. Column
// alignment alone cannot be split reliably, and guessing mis-assigns the DSO.
var reReportRow = regexp.MustCompile(`^[[:space:]]*([0-9]+\.[0-9]+)%`)

// reLevelMarker strips perf's level marker from the head of the symbol field.
var reLevelMarker = regexp.MustCompile(`^[[:space:]]*\[([a-zA-Z.?])\][[:space:]]*`)

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
		fields := strings.Split(line, "|")
		if len(fields) < 4 {
			continue
		}
		m := reReportRow.FindStringSubmatch(fields[0])
		if m == nil {
			continue
		}
		pct, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}

		kind := SymbolKindOther
		sym := fields[3]
		if mk := reLevelMarker.FindStringSubmatch(sym); mk != nil {
			kind = symbolKindFromMarker(mk[1])
			sym = sym[len(mk[0]):]
		}
		sym = strings.TrimSpace(sym)
		if sym == "" {
			continue
		}

		symbols = append(symbols, Symbol{
			HotPct: pct,
			Comm:   strings.TrimSpace(fields[1]),
			DSO:    strings.TrimSpace(fields[2]),
			Kind:   kind,
			Name:   sym,
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
