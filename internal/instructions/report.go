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

// reReportRow matches rows of "perf report -t '|'". The explicit separator is required:
// comm (e.g. "GC Thread#5"), DSO ("[JIT] tid <n>") and C++ symbols can all contain spaces,
// so whitespace splitting mis-assigns fields.
var reReportRow = regexp.MustCompile(`^[[:space:]]*([0-9]+\.[0-9]+)%`)

// reLevelMarker strips perf's level marker from the head of the symbol field.
var reLevelMarker = regexp.MustCompile(`^[[:space:]]*\[([a-zA-Z.?])\][[:space:]]*`)

// ParseReport reads "perf report --stdio --no-children --sort comm,dso,sym" output,
// returning hot symbols in perf's order (descending overhead). Only HotPct, Name, DSO and
// Comm are populated; Insns is filled in later from each symbol's "perf annotate" output.
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

// GlobalStackPct is the symbol's share of total profile weight on stack-memory instructions
// (HotPct scaled by StackPct). Rank by it: neither HotPct nor StackPct alone orders symbols
// by total time spent on spill and reload.
func GlobalStackPct(hotPct float64, p RegPressure) float64 {
	return hotPct * p.StackPct / 100
}
