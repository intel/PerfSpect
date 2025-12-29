// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"reflect"
	"testing"
)

func TestInitializeComponentMetricVariables(t *testing.T) {
	tests := []struct {
		expression string
		want       map[string]int
	}{
		{
			expression: "event1 + event2",
			want:       map[string]int{"event1": -1, "event2": -1},
		},
		{
			expression: "event1 - event2 * event3",
			want:       map[string]int{"event1": -1, "event2": -1, "event3": -1},
		},
		{
			expression: "event1/(event2+event3)",
			want:       map[string]int{"event1": -1, "event2": -1, "event3": -1},
		},
		{
			expression: "event1",
			want:       map[string]int{"event1": -1},
		},
		{
			expression: "event1 + event1",
			want:       map[string]int{"event1": -1},
		},
		{
			expression: "",
			want:       map[string]int{},
		},
		{
			expression: "event1, event2; event3",
			want:       map[string]int{"event1": -1, "event2": -1, "event3": -1},
		},
		{
			expression: "event1 + 42",
			want:       map[string]int{"event1": -1}, // Integer values should not be treated as variables
		},
		// Real-world MetricExpr examples from metrics.json
		{
			expression: "(100 * ((STALL_SLOT_BACKEND / (CPU_CYCLES * #slots)) - ((BR_MIS_PRED * 3) / CPU_CYCLES)))",
			want: map[string]int{
				"STALL_SLOT_BACKEND": -1, "CPU_CYCLES": -1, "BR_MIS_PRED": -1,
			},
		},
		{
			expression: "STALL_BACKEND / CPU_CYCLES * 100",
			want:       map[string]int{"STALL_BACKEND": -1, "CPU_CYCLES": -1},
		},
		{
			expression: "(100 * (((1 - (OP_RETIRED / OP_SPEC)) * (1 - (((STALL_SLOT) if (strcmp_cpuid_str(0x410fd493) | strcmp_cpuid_str(0x410fd490) ^ 1) else (STALL_SLOT - CPU_CYCLES)) / (CPU_CYCLES * #slots)))) + ((BR_MIS_PRED * 4) / CPU_CYCLES)))",
			want: map[string]int{
				"OP_RETIRED": -1, "OP_SPEC": -1, "STALL_SLOT": -1,
				"CPU_CYCLES": -1, "BR_MIS_PRED": -1,
			},
		},
		{
			expression: "BR_MIS_PRED_RETIRED / BR_RETIRED",
			want:       map[string]int{"BR_MIS_PRED_RETIRED": -1, "BR_RETIRED": -1},
		},
		{
			expression: "BR_MIS_PRED_RETIRED / INST_RETIRED * 1000",
			want:       map[string]int{"BR_MIS_PRED_RETIRED": -1, "INST_RETIRED": -1},
		},
		{
			expression: "(BR_IMMED_SPEC + BR_INDIRECT_SPEC) / INST_SPEC * 100",
			want: map[string]int{
				"BR_IMMED_SPEC": -1, "BR_INDIRECT_SPEC": -1, "INST_SPEC": -1,
			},
		},
		{
			expression: "CRYPTO_SPEC / INST_SPEC * 100",
			want:       map[string]int{"CRYPTO_SPEC": -1, "INST_SPEC": -1},
		},
		{
			expression: "DTLB_WALK / INST_RETIRED * 1000",
			want:       map[string]int{"DTLB_WALK": -1, "INST_RETIRED": -1},
		},
		{
			expression: "DTLB_WALK / L1D_TLB",
			want:       map[string]int{"DTLB_WALK": -1, "L1D_TLB": -1},
		},
		{
			expression: "(100 * ((((STALL_SLOT_FRONTEND) if (strcmp_cpuid_str(0x410fd493) | strcmp_cpuid_str(0x410fd490) ^ 1) else (STALL_SLOT_FRONTEND - CPU_CYCLES)) / (CPU_CYCLES * #slots)) - (BR_MIS_PRED / CPU_CYCLES)))",
			want: map[string]int{
				"STALL_SLOT_FRONTEND": -1, "CPU_CYCLES": -1, "BR_MIS_PRED": -1,
			},
		},
		{
			expression: "STALL_FRONTEND / CPU_CYCLES * 100",
			want:       map[string]int{"STALL_FRONTEND": -1, "CPU_CYCLES": -1},
		},
		{
			expression: "DP_SPEC / INST_SPEC * 100",
			want:       map[string]int{"DP_SPEC": -1, "INST_SPEC": -1},
		},
		{
			expression: "INST_RETIRED / CPU_CYCLES",
			want:       map[string]int{"INST_RETIRED": -1, "CPU_CYCLES": -1},
		},
		{
			expression: "ITLB_WALK / INST_RETIRED * 1000",
			want:       map[string]int{"ITLB_WALK": -1, "INST_RETIRED": -1},
		},
		{
			expression: "ITLB_WALK / L1I_TLB",
			want:       map[string]int{"ITLB_WALK": -1, "L1I_TLB": -1},
		},
		{
			expression: "L1D_CACHE_REFILL / L1D_CACHE",
			want:       map[string]int{"L1D_CACHE_REFILL": -1, "L1D_CACHE": -1},
		},
		{
			expression: "L1D_CACHE_REFILL / INST_RETIRED * 1000",
			want:       map[string]int{"L1D_CACHE_REFILL": -1, "INST_RETIRED": -1},
		},
		{
			expression: "(100 * ((OP_RETIRED / OP_SPEC) * (1 - (((STALL_SLOT) if (strcmp_cpuid_str(0x410fd493) | strcmp_cpuid_str(0x410fd490) ^ 1) else (STALL_SLOT - CPU_CYCLES)) / (CPU_CYCLES * #slots)))))",
			want: map[string]int{
				"OP_RETIRED": -1, "OP_SPEC": -1, "STALL_SLOT": -1,
				"CPU_CYCLES": -1,
			},
		},
		{
			expression: "SVE_INST_SPEC / INST_SPEC * 100",
			want:       map[string]int{"SVE_INST_SPEC": -1, "INST_SPEC": -1},
		},
	}

	for _, tt := range tests {
		got := initializeComponentMetricVariables(tt.expression)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("initializeComponentMetricVariables(%q) = %v, want %v", tt.expression, got, tt.want)
		}
	}
}
