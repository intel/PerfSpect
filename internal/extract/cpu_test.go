package extract

import (
	"perfspect/internal/script"
	"testing"
)

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

func TestHyperthreadingFromOutput(t *testing.T) {
	tests := []struct {
		name        string
		lscpuOutput string
		wantResult  string
	}{
		{
			name: "Hyperthreading enabled - 2 threads per core",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             1
Core(s) per socket:    8
CPU(s):                16
Thread(s) per core:    2
On-line CPU(s) list:   0-15
`,
			wantResult: "Enabled",
		},
		{
			name: "Hyperthreading disabled - 1 thread per core",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             1
Core(s) per socket:    8
CPU(s):                8
Thread(s) per core:    1
On-line CPU(s) list:   0-7
`,
			wantResult: "Disabled",
		},
		{
			name: "Hyperthreading enabled - detected by CPU count vs core count",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             2
Core(s) per socket:    8
CPU(s):                32
On-line CPU(s) list:   0-31
`,
			wantResult: "Enabled",
		},
		{
			name: "Hyperthreading disabled - CPU count equals core count",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             2
Core(s) per socket:    8
CPU(s):                16
On-line CPU(s) list:   0-15
`,
			wantResult: "Disabled",
		},
		{
			name: "Online CPUs less than total CPUs - use online count",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             1
Core(s) per socket:    8
CPU(s):                16
Thread(s) per core:    2
On-line CPU(s) list:   0-7
`,
			wantResult: "Enabled",
		},
		{
			name: "Missing threads per core - fallback to CPU vs core comparison",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             1
Core(s) per socket:    8
CPU(s):                16
On-line CPU(s) list:   0-15
`,
			wantResult: "Enabled",
		},
		{
			name: "Error parsing CPU count",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             1
Core(s) per socket:    8
CPU(s):                invalid
Thread(s) per core:    2
On-line CPU(s) list:   0-15
`,
			wantResult: "",
		},
		{
			name: "Error parsing socket count",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             invalid
Core(s) per socket:    8
CPU(s):                16
Thread(s) per core:    2
On-line CPU(s) list:   0-15
`,
			wantResult: "",
		},
		{
			name: "Error parsing cores per socket",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             1
Core(s) per socket:    invalid
CPU(s):                16
Thread(s) per core:    2
On-line CPU(s) list:   0-15
`,
			wantResult: "",
		},
		{
			name: "Invalid online CPU list - should continue with total CPU count",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             1
Core(s) per socket:    8
CPU(s):                16
Thread(s) per core:    2
On-line CPU(s) list:   invalid-range
`,
			wantResult: "Enabled",
		},
		{
			name: "Single core CPU - disabled result",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             1
Core(s) per socket:    1
CPU(s):                1
Thread(s) per core:    1
On-line CPU(s) list:   0
`,
			wantResult: "Disabled",
		},
		{
			name: "4 threads per core - enabled",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             1
Core(s) per socket:    8
CPU(s):                32
Thread(s) per core:    4
On-line CPU(s) list:   0-31
`,
			wantResult: "Enabled",
		},
		{
			name: "Missing CPU family - getCPUExtended will fail",
			lscpuOutput: `
Model:                 143
Stepping:              8
Socket(s):             1
Core(s) per socket:    8
CPU(s):                16
Thread(s) per core:    2
On-line CPU(s) list:   0-15
`,
			wantResult: "",
		},
		{
			name: "Dual socket system with hyperthreading",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             2
Core(s) per socket:    16
CPU(s):                64
Thread(s) per core:    2
On-line CPU(s) list:   0-63
`,
			wantResult: "Enabled",
		},
		{
			name: "Quad socket system without hyperthreading",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             4
Core(s) per socket:    12
CPU(s):                48
Thread(s) per core:    1
On-line CPU(s) list:   0-47
`,
			wantResult: "Disabled",
		},
		{
			name: "Offlined cores with hyperthreading disabled and no threads per core",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             1
Core(s) per socket:    8
CPU(s):                64
On-line CPU(s) list:   0-7
`,
			wantResult: "Disabled",
		},
		{
			name: "Offlined cores with hyperthreading enabled and no threads per core",
			lscpuOutput: `
CPU family:            6
Model:                 143
Stepping:              8
Socket(s):             1
Core(s) per socket:    8
CPU(s):                64
On-line CPU(s) list:   0-7,32-39
`,
			wantResult: "Enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs := map[string]script.ScriptOutput{
				script.LscpuScriptName: {
					Stdout:   tt.lscpuOutput,
					Stderr:   "",
					Exitcode: 0,
				},
			}

			result := HyperthreadingFromOutput(outputs)
			if result != tt.wantResult {
				t.Errorf("hyperthreadingFromOutput() = %q, want %q", result, tt.wantResult)
			}
		})
	}
}
