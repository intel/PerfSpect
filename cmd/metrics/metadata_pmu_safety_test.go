// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import "testing"

// The cases are the four targets the predicate was derived from, so a change that starts
// treating a healthy PMU as broken -- or stops recognizing the broken one -- fails here.
func TestPerfEnumerationFaultsKernelFromOutput(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		wantFaults bool
	}{
		{
			// The target this exists for: 4 fixed counters advertised, so counter 3
			// (TOPDOWN.SLOTS) is claimed, while the kernel exposes no slots event.
			name:       "m6i.16xlarge, Ice Lake guest with an unusable fixed counter 3",
			stdout:     "fixed_purpose_counters=4\nslots_event_exposed=no\n",
			wantFaults: true,
		},
		{
			// Same silicon and kernel, bare metal: the counter is advertised and usable.
			name:       "bare metal Ice Lake, counter 3 advertised and exposed",
			stdout:     "fixed_purpose_counters=4\nslots_event_exposed=yes\n",
			wantFaults: false,
		},
		{
			// Sapphire Rapids guest: only 2 fixed counters, so counter 3 is never claimed
			// and perf refuses the event outright rather than programming it.
			name:       "m7i.24xlarge, counter 3 not advertised",
			stdout:     "fixed_purpose_counters=2\nslots_event_exposed=no\n",
			wantFaults: false,
		},
		{
			// Skylake guest: no slots counter exists on that microarchitecture at all.
			name:       "m5.24xlarge, Skylake with 3 fixed counters",
			stdout:     "fixed_purpose_counters=3\nslots_event_exposed=no\n",
			wantFaults: false,
		},
		{
			// Fails open: an unreadable dmesg must not narrow a healthy target's events.
			name:       "counter count unavailable",
			stdout:     "fixed_purpose_counters=unknown\nslots_event_exposed=no\n",
			wantFaults: false,
		},
		{
			name:       "empty output",
			stdout:     "",
			wantFaults: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			faults, reason := perfEnumerationFaultsKernelFromOutput(test.stdout)
			if faults != test.wantFaults {
				t.Errorf("perfEnumerationFaultsKernelFromOutput() = %v, want %v", faults, test.wantFaults)
			}
			if faults && reason == "" {
				t.Error("expected a reason to log when reporting that enumeration faults")
			}
		})
	}
}
