package report

import (
	"reflect"
	"strings"
	"testing"
)

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

const turbostatOutput = `TIME: 15:04:05
INTERVAL: 2
Package Core    CPU     Avg_MHz Busy%   Bzy_MHz TSC_MHz IPC     IRQ     SMI     POLL    C1      C1E     C6      POLL%   C1%     C1E%    C6%     CPU%c1  CPU%c6  CoreTmp CoreThr PkgTmp  Pkg%pc2 Pkg%pc6 PkgWatt RAMWatt PKG_%   RAM_%   UncMHz
-       -       -       363     9.41    3863    1997    2.15    132732  0       77      68      2530    9318    0.00    0.00    0.15    90.29   0.28    80.88   57      0       57      0.00    0.00    431.32  24.17   0.00    0.00    2400
0       0       0       2244    58.05   3866    2000    2.12    5298    0       0       3       906     1451    0.00    0.00    14.06   27.97   14.07   27.66   45      0       57      0.00    0.00    223.53  7.38    0.00    0.00    2400
0       0       128     0       0.01    3842    2000    0.57    16      0       0       0       4       13      0.00    0.00    0.07    99.92   14.07
0       1       1       951     24.63   3860    2000    2.06    3721    0       0       0       1107    2067    0.00    0.00    20.19   55.29   20.16   54.97   50      0
0       1       129     0       0.01    3875    2000    0.47    19      0       0       0       4       18      0.00    0.00    0.08    99.92   20.16
1       0       64      2       0.05    3789    2000    0.39    433     0       0       0       408     30      0.00    0.00    3.75    96.20   0.99    18.81   46      0       53      0.00    0.00    208.40  16.83   0.00    0.00    2400
1       0       192     3096    80.09   3866    2000    2.15    4205    0       0       0       0       162     0.00    0.00    0.00    19.92   0.99
1       1       65      80      2.06    3862    2000    3.12    130     0       0       0       2       26      0.00    0.00    0.02    97.92   0.02    97.90   46      0
1       1       193     1       0.02    3885    2000    0.30    26      0       0       0       0       27      0.00    0.00    0.00    99.99   0.02
Package Core    CPU     Avg_MHz Busy%   Bzy_MHz TSC_MHz IPC     IRQ     SMI     POLL    C1      C1E     C6      POLL%   C1%     C1E%    C6%     CPU%c1  CPU%c6  CoreTmp CoreThr PkgTmp  Pkg%pc2 Pkg%pc6 PkgWatt RAMWatt PKG_%   RAM_%   UncMHz
-       -       -       363     9.41    3863    1997    2.15    132732  0       77      68      2530    9318    0.00    0.00    0.15    90.29   0.28    80.88   57      0       22      0.00    0.00    223.32  24.17   0.00    0.00    2400
0       0       0       2244    58.05   3866    2000    2.12    5298    0       0       3       906     1451    0.00    0.00    14.06   27.97   14.07   27.66   45      0       57      0.00    0.00    223.53  7.38    0.00    0.00    2400
0       0       128     0       0.01    3842    2000    0.57    16      0       0       0       4       13      0.00    0.00    0.07    99.92   14.07
0       1       1       951     24.63   3860    2000    2.06    3721    0       0       0       1107    2067    0.00    0.00    20.19   55.29   20.16   54.97   50      0
0       1       129     0       0.01    3875    2000    0.47    19      0       0       0       4       18      0.00    0.00    0.08    99.92   20.16
1       0       64      2       0.05    3789    2000    0.39    433     0       0       0       408     30      0.00    0.00    3.75    96.20   0.99    18.81   46      0       53      0.00    0.00    208.40  16.83   0.00    0.00    2400
1       0       192     3096    80.09   3866    2000    2.15    4205    0       0       0       0       162     0.00    0.00    0.00    19.92   0.99
1       1       65      80      2.06    3862    2000    3.12    130     0       0       0       2       26      0.00    0.00    0.02    97.92   0.02    97.90   46      0
1       1       193     1       0.02    3885    2000    0.30    26      0       0       0       0       27      0.00    0.00    0.00    99.99   0.02
Package Core    CPU     Avg_MHz Busy%   Bzy_MHz TSC_MHz IPC     IRQ     SMI     POLL    C1      C1E     C6      POLL%   C1%     C1E%    C6%     CPU%c1  CPU%c6  CoreTmp CoreThr PkgTmp  Pkg%pc2 Pkg%pc6 PkgWatt RAMWatt PKG_%   RAM_%   UncMHz
-       -       -       363     9.41    3863    1997    2.15    132732  0       77      68      2530    9318    0.00    0.00    0.15    90.29   0.28    80.88   57      0       90      0.00    0.00    300.00  24.17   0.00    0.00    2400
0       0       0       2244    58.05   3866    2000    2.12    5298    0       0       3       906     1451    0.00    0.00    14.06   27.97   14.07   27.66   45      0       57      0.00    0.00    223.53  7.38    0.00    0.00    2400
0       0       128     0       0.01    3842    2000    0.57    16      0       0       0       4       13      0.00    0.00    0.07    99.92   14.07
0       1       1       951     24.63   3860    2000    2.06    3721    0       0       0       1107    2067    0.00    0.00    20.19   55.29   20.16   54.97   50      0
0       1       129     0       0.01    3875    2000    0.47    19      0       0       0       4       18      0.00    0.00    0.08    99.92   20.16
1       0       64      2       0.05    3789    2000    0.39    433     0       0       0       408     30      0.00    0.00    3.75    96.20   0.99    18.81   46      0       53      0.00    0.00    208.40  16.83   0.00    0.00    2400
1       0       192     3096    80.09   3866    2000    2.15    4205    0       0       0       0       162     0.00    0.00    0.00    19.92   0.99
1       1       65      80      2.06    3862    2000    3.12    130     0       0       0       2       26      0.00    0.00    0.02    97.92   0.02    97.90   46      0
1       1       193     1       0.02    3885    2000    0.30    26      0       0       0       0       27      0.00    0.00    0.00    99.99   0.02
`

func TestTurbostatSummaryRows(t *testing.T) {
	tests := []struct {
		name            string
		turbostatOutput string
		fieldNames      []string
		wantFirst       []string
		wantLen         int
		expectErr       bool
	}{
		{
			name:            "Extract Avg_MHz and Busy% from summary rows",
			turbostatOutput: turbostatOutput,
			fieldNames:      []string{"Avg_MHz", "Busy%"},
			wantFirst:       []string{"15:04:05", "363", "9.41"},
			wantLen:         3,
			expectErr:       false,
		},
		{
			name:            "Extract Bzy_MHz from summary rows",
			turbostatOutput: turbostatOutput,
			fieldNames:      []string{"Bzy_MHz"},
			wantFirst:       []string{"15:04:05", "3863"},
			wantLen:         3,
			expectErr:       false,
		},
		{
			name:            "Extract multiple fields from summary row",
			turbostatOutput: turbostatOutput,
			fieldNames:      []string{"Avg_MHz", "Busy%", "Bzy_MHz"},
			wantFirst:       []string{"15:04:05", "363", "9.41", "3863"},
			wantLen:         3,
			expectErr:       false,
		},
		{
			name:            "Missing field in header",
			turbostatOutput: turbostatOutput,
			fieldNames:      []string{"NotAField"},
			wantFirst:       nil,
			wantLen:         0,
			expectErr:       true,
		},
		{
			name:            "Empty fieldNames",
			turbostatOutput: turbostatOutput,
			fieldNames:      []string{},
			wantFirst:       nil,
			wantLen:         0,
			expectErr:       true,
		},
		{
			name:            "No summary rows in output",
			turbostatOutput: "No summary rows here",
			fieldNames:      []string{"Avg_MHz", "Busy%"},
			wantFirst:       nil,
			wantLen:         0,
			expectErr:       true,
		},
		{
			name:            "No output",
			turbostatOutput: "",
			fieldNames:      []string{"Avg_MHz", "Busy%"},
			wantFirst:       nil,
			wantLen:         0,
			expectErr:       true,
		},
		{
			name:            "Only time and interval, no turbostat data",
			turbostatOutput: strings.Join(strings.Split(turbostatOutput, "\n")[0:2], "\n"), // Only header and no data
			fieldNames:      []string{"Avg_MHz", "Busy%"},
			wantFirst:       nil,
			wantLen:         0,
			expectErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := turbostatSummaryRows(tt.turbostatOutput, tt.fieldNames)
			if (err != nil) != tt.expectErr {
				t.Errorf("turbostatSummaryRows() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if tt.expectErr {
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("turbostatSummaryRows() got %d rows, want %d", len(got), tt.wantLen)
			}
			if tt.wantLen > 0 && !reflect.DeepEqual(got[0], tt.wantFirst) {
				t.Errorf("turbostatSummaryRows() first row = %v, want %v", got[0], tt.wantFirst)
			}
		})
	}
}
func TestMaxTotalPackagePowerFromOutput(t *testing.T) {
	tests := []struct {
		name            string
		turbostatOutput string
		want            string
	}{
		{
			name:            "Typical output with summary rows",
			turbostatOutput: turbostatOutput,
			want:            "300.00 Watts",
		},
		{
			name: "Multiple summary rows, max is not first",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgWatt
-       -       -       100.5
-       -       -       200.75
-       -       -       150.25
`,
			want: "200.75 Watts",
		},
		{
			name: "Multiple summary rows, no TIME",
			turbostatOutput: `
INTERVAL: 1
Package Core    CPU     PkgWatt
-       -       -       100.5
-       -       -       200.75
-       -       -       150.25
`,
			want: "200.75 Watts",
		},
		{
			name: "Multiple summary rows, no INTERVAL",
			turbostatOutput: `
TIME: 12:00:00
Package Core    CPU     PkgWatt
-       -       -       100.5
-       -       -       200.75
-       -       -       150.25
`,
			want: "200.75 Watts",
		},
		{
			name: "No summary rows",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgWatt
0       0       0       100.5
0       1       1       200.75
`,
			want: "",
		},
		{
			name: "Malformed PkgWatt value",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgWatt
-       -       -       notanumber
-       -       -       123.45
`,
			want: "123.45 Watts",
		},
		{
			name: "No PkgWatt column",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     SomethingElse
-       -       -       999
`,
			want: "",
		},
		{
			name:            "Empty output",
			turbostatOutput: "",
			want:            "",
		},
		{
			name: "Only headers, no data",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgWatt
`,
			want: "",
		},
		{
			name: "Zero PkgWatt values",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgWatt
-       -       -       0
-       -       -       0
`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxTotalPackagePowerFromOutput(tt.turbostatOutput)
			if got != tt.want {
				t.Errorf("maxTotalPackagePowerFromOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}
func TestMinTotalPackagePowerFromOutput(t *testing.T) {
	tests := []struct {
		name            string
		turbostatOutput string
		want            string
	}{
		{
			name:            "Typical output with summary rows",
			turbostatOutput: turbostatOutput,
			want:            "223.32 Watts",
		},
		{
			name: "Multiple summary rows, min is not first",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgWatt
-       -       -       100.5
-       -       -       200.75
-       -       -       50.25
`,
			want: "50.25 Watts",
		},
		{
			name: "No summary rows",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgWatt
0       0       0       100.5
0       1       1       200.75
`,
			want: "",
		},
		{
			name: "Malformed PkgWatt value",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgWatt
-       -       -       notanumber
-       -       -       123.45
-       -       -       99.99
`,
			want: "99.99 Watts",
		},
		{
			name: "No PkgWatt column",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     SomethingElse
-       -       -       999
`,
			want: "",
		},
		{
			name:            "Empty output",
			turbostatOutput: "",
			want:            "",
		},
		{
			name: "Only headers, no data",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgWatt
`,
			want: "",
		},
		{
			name: "Zero PkgWatt values",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgWatt
-       -       -       0
-       -       -       0
`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := minTotalPackagePowerFromOutput(tt.turbostatOutput)
			if got != tt.want {
				t.Errorf("minTotalPackagePowerFromOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}
func TestMaxPackageTemperatureFromOutput(t *testing.T) {
	tests := []struct {
		name            string
		turbostatOutput string
		want            string
	}{
		{
			name:            "Typical output with summary rows",
			turbostatOutput: turbostatOutput,
			want:            "90 C",
		},
		{
			name: "Multiple summary rows, max is not first",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgTmp
-       -       -       45
-       -       -       99
-       -       -       77
`,
			want: "99 C",
		},
		{
			name: "No summary rows",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgTmp
0       0       0       45
0       1       1       99
`,
			want: "",
		},
		{
			name: "Malformed PkgTmp value",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgTmp
-       -       -       notanumber
-       -       -       88
`,
			want: "88 C",
		},
		{
			name: "No PkgTmp column",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     SomethingElse
-       -       -       999
`,
			want: "",
		},
		{
			name:            "Empty output",
			turbostatOutput: "",
			want:            "",
		},
		{
			name: "Only headers, no data",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgTmp
`,
			want: "",
		},
		{
			name: "Zero PkgTmp values",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     PkgTmp
-       -       -       0
-       -       -       0
`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxPackageTemperatureFromOutput(tt.turbostatOutput)
			if got != tt.want {
				t.Errorf("maxPackageTemperatureFromOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}
