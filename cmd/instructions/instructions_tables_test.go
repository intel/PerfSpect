// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package instructions

import (
	"strings"
	"testing"

	"perfspect/internal/script"
)

// TestAPXEstimateWithNothingAnalyzable covers the case where the collection produced no
// annotatable symbols, which happens whenever the hot code is stripped or JIT-compiled
// without an agent.
//
// The model would otherwise report a 0.00% gain next to a realization factor and a
// register-utilization figure, all derived from no data. That reads as a measurement
// saying APX would not help here, rather than as an absence of evidence.
func TestAPXEstimateWithNothingAnalyzable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		outputs map[string]script.ScriptOutput
	}{
		{"no script output at all", map[string]script.ScriptOutput{}},
		{"script ran but found nothing", map[string]script.ScriptOutput{
			script.HotCodeAnnotateScriptName: {Stdout: "########## perf_event ##########\ncycles:upp\n"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fields := apxEstimateTableValues(tc.outputs)
			if len(fields) != 1 {
				t.Fatalf("got %d fields, want a single explanatory one; a full table of "+
					"zeroes would imply APX was evaluated and found not to help", len(fields))
			}
			v := fields[0].Values[0]
			if !strings.Contains(v, "not available") {
				t.Errorf("field value = %q, want it to say the estimate is not available", v)
			}
			// It must not present a number that could be mistaken for a result.
			if strings.Contains(v, "0.00%") {
				t.Errorf("field value = %q, want no percentage at all", v)
			}
		})
	}
}

// TestAPXEstimatePopulatedWhenSymbolsExist is the counterpart: given real annotated
// output, the full estimate is reported rather than the placeholder.
func TestAPXEstimatePopulatedWhenSymbolsExist(t *testing.T) {
	// Minimal sectioned output with one annotated symbol carrying stack traffic.
	stdout := strings.Join([]string{
		"########## perf_event ##########",
		"cycles:upp",
		"########## perf_report ##########",
		" 50.00% |prog|prog|[.] hot",
		"########## annotate_0 ##########",
		"sym=hot",
		"dso=prog",
		" Percent |	Source code & Disassembly of prog for cycles:upp",
		"    0.00 :   1000:   push   %rbp",
		"    0.00 :   1001:   mov    %rsp,%rbp",
		"   50.00 :   1004:   mov    %rax,-0x8(%rbp)",
		"   50.00 :   1008:   mov    -0x8(%rbp),%rbx",
		"",
	}, "\n")
	fields := apxEstimateTableValues(map[string]script.ScriptOutput{
		script.HotCodeAnnotateScriptName: {Stdout: stdout},
	})
	if len(fields) < 5 {
		t.Fatalf("got %d fields, want the full estimate", len(fields))
	}
	var names []string
	for _, f := range fields {
		names = append(names, f.Name)
	}
	joined := strings.Join(names, ",")
	for _, want := range []string{"Estimated Gain (realistic)", "GP Spill/Reload Cost", "Caveat"} {
		if !strings.Contains(joined, want) {
			t.Errorf("fields %v, want one named %q", names, want)
		}
	}
}
