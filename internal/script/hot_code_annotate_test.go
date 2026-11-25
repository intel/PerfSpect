// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package script

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// selectionSnippet cuts the symbol-selection block out of the rendered hot code annotate
// script, so it can run against a fixture report without perf.
func selectionSnippet(t *testing.T) string {
	t.Helper()
	rendered := GetParameterizedScriptByName(HotCodeAnnotateScriptName, map[string]string{
		"Duration": "1", "Frequency": "1", "PIDs": "", "TopSymbols": "1", "Jobs": "1",
		"IncludeKernel": "false", "JIT": "false", "Symfs": "",
	}).ScriptTemplate
	const start = `SYMLIST="$WORKDIR/symlist.tsv"`
	const end = `if [ -s "$UNRESOLVED" ]; then`
	a := strings.Index(rendered, start)
	b := strings.Index(rendered, end)
	if a < 0 || b < a {
		t.Fatalf("selection block not found in rendered script (start %d, end %d)", a, b)
	}
	return rendered[a:b]
}

// runSelection runs the selection block on report with the given --top and returns the
// symlist and unresolved files.
func runSelection(t *testing.T, report string, top string) (symlist, unresolved string) {
	t.Helper()
	for _, tool := range []string{"bash", "awk", "sort", "head", "cut"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available: %v", tool, err)
		}
	}
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.txt")
	if err := os.WriteFile(reportPath, []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}
	script := "WORKDIR=\"$1\"\nREPORT_OUT=\"$2\"\ntop=\"$3\"\n" + selectionSnippet(t)
	cmd := exec.Command("bash", "-c", script, "selection", dir, reportPath, top) // #nosec G204
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("selection block failed: %v\n%s", err, out)
	}
	sl, err := os.ReadFile(filepath.Join(dir, "symlist.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	un, err := os.ReadFile(filepath.Join(dir, "unresolved.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	return string(sl), string(un)
}

// TestHotCodeSelectionSumsAcrossComms checks that symbols are ranked by HOT% summed over
// every comm they appear under, not by their heaviest single perf report row.
func TestHotCodeSelectionSumsAcrossComms(t *testing.T) {
	report := strings.Join([]string{
		"#",
		"# Overhead|Command|Shared Object|Symbol",
		"#",
		"     5.00%|main|app|[.] solo_hot",
		"     4.00%|main|app|[.] solo_warm",
		"     3.00%|worker-1|app|[.] spread",
		"     3.00%|worker-2|app|[.] spread",
		"     3.00%|worker-3|app|[.] spread",
		"     2.50%|java|[JIT] tid 42|[.] 0x00007f00deadbeef",
		"     1.00%|main|libstdc++.so.6|[.] std::vector<int, std::allocator<int> >::push_back(int const&)",
		"",
	}, "\n")

	t.Run("top 2 keeps the spread symbol", func(t *testing.T) {
		symlist, unresolved := runSelection(t, report, "2")
		// Ranking by single row would pick solo_hot and solo_warm; spread sums to 9%.
		want := "app\tspread\napp\tsolo_hot\n"
		if symlist != want {
			t.Errorf("symlist = %q, want %q", symlist, want)
		}
		// The JIT row is unannotatable: recorded for coverage, not selected.
		if want := "2.50\t[JIT] tid 42\t0x00007f00deadbeef\n"; unresolved != want {
			t.Errorf("unresolved = %q, want %q", unresolved, want)
		}
	})

	t.Run("all symbols, names with spaces intact", func(t *testing.T) {
		symlist, _ := runSelection(t, report, "10")
		want := "app\tspread\napp\tsolo_hot\napp\tsolo_warm\n" +
			"libstdc++.so.6\tstd::vector<int, std::allocator<int> >::push_back(int const&)\n"
		if symlist != want {
			t.Errorf("symlist = %q, want %q", symlist, want)
		}
	})

	t.Run("equal sums keep report order", func(t *testing.T) {
		tied := "     2.00%|main|app|[.] first\n     2.00%|main|app|[.] second\n"
		symlist, _ := runSelection(t, tied, "2")
		if want := "app\tfirst\napp\tsecond\n"; symlist != want {
			t.Errorf("symlist = %q, want %q", symlist, want)
		}
	})
}
