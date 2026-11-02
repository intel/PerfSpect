// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package instructions

import (
	"strings"
	"testing"
)

const sampleReport = `# To display the perf.data header info, please use --header/--header-only options.
#
# Samples: 40K of event 'cycles:upp'
#
# Overhead  Command        Shared Object  Symbol
# ........  .............  .............  .....................
#
    12.20%  stress-ng-cpu  stress-ng      [.] ccitt_crc16
     5.74%  stress-ng-cpu  libm.so.6      [.] __kernel_rem_pio2
     0.30%  stress-ng-cpu  [kernel.kallsyms]  [k] native_write_msr
     0.01%  stress-ng      [unknown]      [.] 0xffffffff9120124c
`

func TestParseReport(t *testing.T) {
	syms, err := ParseReport(strings.NewReader(sampleReport))
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 4 {
		t.Fatalf("parsed %d symbols, want 4", len(syms))
	}

	if syms[0].Name != "ccitt_crc16" || syms[0].DSO != "stress-ng" ||
		syms[0].Comm != "stress-ng-cpu" || syms[0].HotPct != 12.20 {
		t.Errorf("first symbol = %+v, want ccitt_crc16/stress-ng/stress-ng-cpu/12.20", syms[0])
	}
	if syms[0].Kind != SymbolKindUser {
		t.Errorf("Kind = %q, want %q", syms[0].Kind, SymbolKindUser)
	}
	if syms[2].Kind != SymbolKindKernel {
		t.Errorf("kernel symbol Kind = %q, want %q", syms[2].Kind, SymbolKindKernel)
	}
	// Ordering must be preserved: perf emits descending by overhead and callers rely on it.
	for i := 1; i < len(syms); i++ {
		if syms[i].HotPct > syms[i-1].HotPct {
			t.Errorf("order not preserved at %d: %.2f > %.2f", i, syms[i].HotPct, syms[i-1].HotPct)
		}
	}
}

func TestParseReportIgnoresHeadersAndJunk(t *testing.T) {
	syms, err := ParseReport(strings.NewReader("# comment\n\nnot a row\n   garbage %\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 0 {
		t.Errorf("parsed %d symbols from junk, want 0", len(syms))
	}
}

func TestSymbolIsUnresolved(t *testing.T) {
	tests := []struct {
		sym  Symbol
		want bool
	}{
		{Symbol{Name: "ccitt_crc16", DSO: "stress-ng"}, false},
		{Symbol{Name: "0xffffffff9120124c", DSO: "stress-ng"}, true},
		{Symbol{Name: "something", DSO: "[unknown]"}, true},
	}
	for _, tc := range tests {
		if got := tc.sym.IsUnresolved(); got != tc.want {
			t.Errorf("IsUnresolved(%+v) = %v, want %v", tc.sym, got, tc.want)
		}
	}
}

func TestGlobalStackPct(t *testing.T) {
	// A hot symbol with little stack traffic must rank below a cooler one that is dense
	// with it; this is the whole reason GSTK% exists.
	hotButClean := GlobalStackPct(75.27, RegPressure{StackPct: 3.52})
	coolButDense := GlobalStackPct(23.43, RegPressure{StackPct: 91.38})
	if coolButDense <= hotButClean {
		t.Errorf("GSTK%% failed to reorder: dense=%.4f clean=%.4f", coolButDense, hotButClean)
	}
	if got, want := hotButClean, 2.649504; got < want-1e-5 || got > want+1e-5 {
		t.Errorf("GlobalStackPct = %.6f, want %.6f", got, want)
	}
}

// buildScriptOutput assembles output in the sectioned form the collection script emits.
func buildScriptOutput(t *testing.T, annotateFixture string) string {
	t.Helper()
	body := readFixture(t, annotateFixture)
	var sb strings.Builder
	sb.WriteString("########## perf_event ##########\ncycles:upp\n")
	sb.WriteString("########## perf_report ##########\n")
	sb.WriteString("# Overhead  Command   Shared Object  Symbol\n")
	sb.WriteString("    23.43%  workload  workload       [.] gp_pressure\n")
	sb.WriteString("########## annotate_0 ##########\n")
	sb.WriteString("sym=gp_pressure\ndso=workload\n")
	sb.WriteString(body)
	return sb.String()
}

func TestAnalyzeEndToEnd(t *testing.T) {
	result, err := Analyze(buildScriptOutput(t, "annotate_gp_pressure.stdio"), 5)
	if err != nil {
		t.Fatal(err)
	}
	if result.PerfEvent != "cycles:upp" {
		t.Errorf("PerfEvent = %q, want cycles:upp", result.PerfEvent)
	}
	if len(result.Analyzed) != 1 {
		t.Fatalf("analyzed %d symbols, want 1", len(result.Analyzed))
	}
	got := result.Analyzed[0]
	if got.Name != "gp_pressure" || got.DSO != "workload" {
		t.Errorf("symbol = %s/%s, want gp_pressure/workload", got.Name, got.DSO)
	}
	if got.HotPct != 23.43 {
		t.Errorf("HotPct = %.2f, want 23.43", got.HotPct)
	}
	// Known values for this fixture, matching the reference implementation.
	if d := got.Pressure.StackPct - 91.3791; d > 1e-4 || d < -1e-4 {
		t.Errorf("StackPct = %.4f, want 91.3791", got.Pressure.StackPct)
	}
	// GSTK% = HOT% x STACK% / 100
	if d := got.GlobalStackPct - 21.4100; d > 1e-3 || d < -1e-3 {
		t.Errorf("GlobalStackPct = %.4f, want ~21.4100", got.GlobalStackPct)
	}
	if !got.Annotated {
		t.Error("Annotated = false, want true")
	}
	if len(result.Unannotated) != 0 {
		t.Errorf("Unannotated = %v, want none", result.Unannotated)
	}
}

// TestAnalyzeReportsStrippedSymbols covers the case that matters most in practice: a hot
// symbol whose binary has no symbols produces an empty annotate section, and must be
// surfaced as unannotatable rather than silently dropped.
func TestAnalyzeReportsStrippedSymbols(t *testing.T) {
	out := strings.Join([]string{
		"########## perf_event ##########",
		"cycles:upp",
		"########## perf_report ##########",
		"# Overhead  Command        Shared Object  Symbol",
		"    99.00%  stress-ng-cpu  stress-ng      [.] ccitt_crc16",
		"########## annotate_0 ##########",
		"sym=ccitt_crc16",
		"dso=stress-ng",
		"",
	}, "\n")

	result, err := Analyze(out, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Analyzed) != 0 {
		t.Errorf("Analyzed = %d, want 0", len(result.Analyzed))
	}
	if len(result.Unannotated) != 1 {
		t.Fatalf("Unannotated = %d, want 1", len(result.Unannotated))
	}
	if result.Unannotated[0].Name != "ccitt_crc16" {
		t.Errorf("Unannotated symbol = %q, want ccitt_crc16", result.Unannotated[0].Name)
	}
	if got := result.LostHotPct(); got != 99.00 {
		t.Errorf("LostHotPct = %.2f, want 99.00", got)
	}
	dsos := result.UnannotatedDSOs()
	if len(dsos) != 1 || dsos[0].DSO != "stress-ng" || dsos[0].HotPct != 99.00 {
		t.Errorf("UnannotatedDSOs = %+v, want stress-ng at 99.00", dsos)
	}
}

func TestAnalyzeRanksByGlobalStackPct(t *testing.T) {
	// Two symbols: one hot but clean, one cooler but spill-dense. The dense one must sort
	// first because it accounts for more total time in stack traffic.
	gp := readFixture(t, "annotate_gp_pressure.stdio")
	simd := readFixture(t, "annotate_simd_pressure.stdio")
	out := strings.Join([]string{
		"########## perf_event ##########", "cycles:upp",
		"########## perf_report ##########",
		"# Overhead  Command   Shared Object  Symbol",
		"    75.27%  workload  workload       [.] simd_pressure",
		"    23.43%  workload  workload       [.] gp_pressure",
		"########## annotate_0 ##########", "sym=simd_pressure", "dso=workload", simd,
		"########## annotate_1 ##########", "sym=gp_pressure", "dso=workload", gp,
	}, "\n")

	result, err := Analyze(out, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Analyzed) != 2 {
		t.Fatalf("analyzed %d, want 2", len(result.Analyzed))
	}
	if result.Analyzed[0].Name != "gp_pressure" {
		t.Errorf("first ranked = %q, want gp_pressure (higher GSTK%% despite lower HOT%%)",
			result.Analyzed[0].Name)
	}
	if result.Analyzed[1].Name != "simd_pressure" {
		t.Errorf("second ranked = %q, want simd_pressure", result.Analyzed[1].Name)
	}
}

func TestAnalyzeEmptyInput(t *testing.T) {
	result, err := Analyze("", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Analyzed) != 0 || len(result.Unannotated) != 0 || result.PerfEvent != "" {
		t.Errorf("empty input produced %+v, want zero value", result)
	}
}

func TestSplitAnnotateSection(t *testing.T) {
	sym, dso, disasm := splitAnnotateSection("sym=foo\ndso=bar\n  0.00 :  1000:  nop\n")
	if sym != "foo" || dso != "bar" {
		t.Errorf("sym/dso = %q/%q, want foo/bar", sym, dso)
	}
	if !strings.Contains(disasm, "nop") {
		t.Errorf("disasm = %q, want it to contain the instruction", disasm)
	}
	if strings.Contains(disasm, "sym=") || strings.Contains(disasm, "dso=") {
		t.Errorf("disasm = %q, want header lines stripped", disasm)
	}
}

// TestAnalyzeReportsUnresolvedSymbols covers the stripped-binary case as it actually
// arrives: perf cannot resolve the symbol at all and reports a raw address, so the
// collection script lists it as unresolved rather than attempting to annotate it. These
// must be counted against coverage, not dropped, or a report covering only the
// symbol-bearing libraries would look complete.
func TestAnalyzeReportsUnresolvedSymbols(t *testing.T) {
	out := strings.Join([]string{
		"########## perf_event ##########", "cycles:upp",
		"########## perf_report ##########",
		"# Overhead  Command        Shared Object  Symbol",
		"     5.83%  stress-ng-cpu  stress-ng      [.] 0x0000000000072292",
		"########## unresolved ##########",
		"5.83\tstress-ng\t0x0000000000072292",
		"4.26\tstress-ng\t0x000000000007228a",
		"1.50\tother-app\t0x0000000000001234",
	}, "\n")

	result, err := Analyze(out, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Analyzed) != 0 {
		t.Errorf("Analyzed = %d, want 0", len(result.Analyzed))
	}
	if len(result.Unannotated) != 3 {
		t.Fatalf("Unannotated = %d, want 3", len(result.Unannotated))
	}
	if got, want := result.LostHotPct(), 11.59; got < want-1e-9 || got > want+1e-9 {
		t.Errorf("LostHotPct = %.2f, want %.2f", got, want)
	}
	// DSOs are aggregated and ranked by lost weight so the worst offender reads first.
	dsos := result.UnannotatedDSOs()
	if len(dsos) != 2 {
		t.Fatalf("UnannotatedDSOs = %d, want 2", len(dsos))
	}
	if dsos[0].DSO != "stress-ng" {
		t.Errorf("worst DSO = %q, want stress-ng", dsos[0].DSO)
	}
	if got, want := dsos[0].HotPct, 10.09; got < want-1e-9 || got > want+1e-9 {
		t.Errorf("stress-ng lost weight = %.2f, want %.2f", got, want)
	}
}

func TestParseUnresolvedIgnoresMalformedRows(t *testing.T) {
	got := parseUnresolved("not-a-number\tdso\tsym\n\nonly-two\tfields\n5.0\tdso\tsym\n")
	if len(got) != 1 {
		t.Fatalf("parsed %d rows, want 1", len(got))
	}
	if got[0].HotPct != 5.0 || got[0].DSO != "dso" || got[0].Name != "sym" {
		t.Errorf("row = %+v, want 5.0/dso/sym", got[0])
	}
}

// TestParseReportHandlesSpacesInDSO covers perf's JIT and pseudo-DSO naming. A JIT region
// is named "[JIT] tid <n>", so neither the DSO nor the symbol can be taken as a
// whitespace-delimited field. Getting this wrong silently drops every JIT row, which makes
// a Java profile look like it lost no weight at all.
func TestParseReportHandlesSpacesInDSO(t *testing.T) {
	in := strings.Join([]string{
		"# Overhead  Command  Shared Object  Symbol",
		"     7.49%  java             [JIT] tid 4029471      [.] 0x000077a2dc2e16eb",
		"     1.64%  java             [vdso]                 [.] 0x0000000000000b00",
		"     0.30%  java             [kernel.kallsyms]      [k] native_write_msr",
		"    12.20%  stress-ng-cpu    stress-ng              [.] ccitt_crc16",
		"     0.10%  prog             libfoo.so              [.] Ns::C::m(int, long) const",
	}, "\n")

	syms, err := ParseReport(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 5 {
		t.Fatalf("parsed %d rows, want 5", len(syms))
	}

	want := []struct {
		dso, name string
		kind      SymbolKind
	}{
		{"[JIT] tid 4029471", "0x000077a2dc2e16eb", SymbolKindUser},
		{"[vdso]", "0x0000000000000b00", SymbolKindUser},
		{"[kernel.kallsyms]", "native_write_msr", SymbolKindKernel},
		{"stress-ng", "ccitt_crc16", SymbolKindUser},
		// A C++ signature contains spaces and must survive intact.
		{"libfoo.so", "Ns::C::m(int, long) const", SymbolKindUser},
	}
	for i, w := range want {
		if syms[i].DSO != w.dso {
			t.Errorf("row %d DSO = %q, want %q", i, syms[i].DSO, w.dso)
		}
		if syms[i].Name != w.name {
			t.Errorf("row %d Name = %q, want %q", i, syms[i].Name, w.name)
		}
		if syms[i].Kind != w.kind {
			t.Errorf("row %d Kind = %q, want %q", i, syms[i].Kind, w.kind)
		}
	}
	// The JIT row must be recognized as unresolved so its weight is accounted for.
	if !syms[0].IsUnresolved() {
		t.Error("JIT row IsUnresolved() = false, want true")
	}
}
