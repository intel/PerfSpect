package extract

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
0       0       0       2244    58.05   3866    2000    2.12    5298    0       0       3       906     1451    0.00    0.00    14.06   27.97   14.07   27.66   45      0       57      0.00    0.00    223.53  7.38    0.00    0.00    2350
0       0       128     0       0.01    3842    2000    0.57    16      0       0       0       4       13      0.00    0.00    0.07    99.92   14.07
0       1       1       951     24.63   3860    2000    2.06    3721    0       0       0       1107    2067    0.00    0.00    20.19   55.29   20.16   54.97   50      0
0       1       129     0       0.01    3875    2000    0.47    19      0       0       0       4       18      0.00    0.00    0.08    99.92   20.16
1       0       64      2       0.05    3789    2000    0.39    433     0       0       0       408     30      0.00    0.00    3.75    96.20   0.99    18.81   46      0       53      0.00    0.00    208.40  16.83   0.00    0.00    2300
1       0       192     3096    80.09   3866    2000    2.15    4205    0       0       0       0       162     0.00    0.00    0.00    19.92   0.99
1       1       65      80      2.06    3862    2000    3.12    130     0       0       0       2       26      0.00    0.00    0.02    97.92   0.02    97.90   46      0
1       1       193     1       0.02    3885    2000    0.30    26      0       0       0       0       27      0.00    0.00    0.00    99.99   0.02
Package Core    CPU     Avg_MHz Busy%   Bzy_MHz TSC_MHz IPC     IRQ     SMI     POLL    C1      C1E     C6      POLL%   C1%     C1E%    C6%     CPU%c1  CPU%c6  CoreTmp CoreThr PkgTmp  Pkg%pc2 Pkg%pc6 PkgWatt RAMWatt PKG_%   RAM_%   UncMHz
-       -       -       363     9.41    3863    1997    2.15    132732  0       77      68      2530    9318    0.00    0.00    0.15    90.29   0.28    80.88   57      0       22      0.00    0.00    223.32  24.17   0.00    0.00    2400
0       0       0       2244    58.05   3866    2000    2.12    5298    0       0       3       906     1451    0.00    0.00    14.06   27.97   14.07   27.66   45      0       59      0.00    0.00    229.53  7.38    0.00    0.00    2400
0       0       128     0       0.01    3842    2000    0.57    16      0       0       0       4       13      0.00    0.00    0.07    99.92   14.07
0       1       1       951     24.63   3860    2000    2.06    3721    0       0       0       1107    2067    0.00    0.00    20.19   55.29   20.16   54.97   50      0
0       1       129     0       0.01    3875    2000    0.47    19      0       0       0       4       18      0.00    0.00    0.08    99.92   20.16
1       0       64      2       0.05    3789    2000    0.39    433     0       0       0       408     30      0.00    0.00    3.75    96.20   0.99    18.81   46      0       55      0.00    0.00    218.40  16.83   0.00    0.00    2400
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

func TestTurbostatPlatformRows(t *testing.T) {
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
			got, err := TurbostatPlatformRows(tt.turbostatOutput, tt.fieldNames)
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
			got := MaxTotalPackagePowerFromOutput(tt.turbostatOutput)
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
			got := MinTotalPackagePowerFromOutput(tt.turbostatOutput)
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
			got := MaxPackageTemperatureFromOutput(tt.turbostatOutput)
			if got != tt.want {
				t.Errorf("maxPackageTemperatureFromOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}
func TestTurbostatPackageRows(t *testing.T) {
	tests := []struct {
		name            string
		turbostatOutput string
		fieldNames      []string
		want            [][][]string
		wantErr         bool
	}{
		{
			name:            "package rows with UncoreMHz, PKGTmp, PkgWatt",
			turbostatOutput: turbostatOutput,
			fieldNames:      []string{"UncMHz", "PkgTmp", "PkgWatt"},
			want: [][][]string{
				{{"15:04:05", "2350", "57", "223.53"}, {"15:04:07", "2400", "59", "229.53"}, {"15:04:09", "2400", "57", "223.53"}},
				{{"15:04:05", "2300", "53", "208.40"}, {"15:04:07", "2400", "55", "218.40"}, {"15:04:09", "2400", "53", "208.40"}},
			},
			wantErr: false,
		},
		{
			name: "Typical output, two packages, one field",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     Avg_MHz Busy% FooBar
-       -       -       1000    10    999
0       0       0       1000    10    999
0       1       1       1100    11
1       0       2       2000    20    999
1       1       3       2100    21
`,
			fieldNames: []string{"Avg_MHz"},
			want: [][][]string{
				{{"12:00:00", "1000"}},
				{{"12:00:00", "2000"}},
			},
			wantErr: false,
		},
		{
			name: "Typical output, two packages, two fields",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     Avg_MHz Busy% FooBar
-       -       -       1000    10    999
0       0       0       1000    10    999
0       1       1       1100    11
1       0       2       2000    20    999
1       1       3       2100    21
`,
			fieldNames: []string{"Avg_MHz", "Busy%"},
			want: [][][]string{
				{{"12:00:00", "1000", "10"}},
				{{"12:00:00", "2000", "20"}},
			},
			wantErr: false,
		},
		{
			name: "Missing field in header",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     Avg_MHz Busy%
-       -       -       1000    10
0       0       0       1000    10
1       0       2       2000    20
`,
			fieldNames: []string{"NotAField"},
			want:       nil,
			wantErr:    true,
		},
		{
			name: "Empty fieldNames",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     Avg_MHz Busy%
-       -       -       1000    10
0       0       0       1000    10
1       0       2       2000    20
`,
			fieldNames: []string{},
			want:       nil,
			wantErr:    true,
		},
		{
			name: "No package rows",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     Avg_MHz Busy%
-       -       -       999     99
-       -       -       888     88
`,
			fieldNames: []string{"Avg_MHz"},
			want:       nil,
			wantErr:    true,
		},
		{
			name: "Malformed package number",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     Avg_MHz Busy%
X       0       0       1000    10
`,
			fieldNames: []string{"Avg_MHz"},
			want:       nil,
			wantErr:    true,
		},
		{
			name:            "Empty output",
			turbostatOutput: "",
			fieldNames:      []string{"Avg_MHz"},
			want:            nil,
			wantErr:         true,
		},
		{
			name: "Only headers, no data",
			turbostatOutput: `
TIME: 12:00:00
INTERVAL: 1
Package Core    CPU     Avg_MHz Busy%
`,
			fieldNames: []string{"Avg_MHz"},
			want:       nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TurbostatPackageRows(tt.turbostatOutput, tt.fieldNames)
			if (err != nil) != tt.wantErr {
				t.Errorf("turbostatPackageRows() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("turbostatPackageRows() = %v, want %v", got, tt.want)
			}
		})
	}
}
