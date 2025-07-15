package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"perfspect/internal/script"
	"reflect"
	"testing"
)

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

			result := hyperthreadingFromOutput(outputs)
			if result != tt.wantResult {
				t.Errorf("hyperthreadingFromOutput() = %q, want %q", result, tt.wantResult)
			}
		})
	}
}

func TestGetFrequenciesFromMSR(t *testing.T) {
	tests := []struct {
		name      string
		msr       string
		want      []int
		expectErr bool
	}{
		{
			name:      "Valid MSR with multiple frequencies",
			msr:       "0x1A2B3C4D",
			want:      []int{0x4D, 0x3C, 0x2B, 0x1A},
			expectErr: false,
		},
		{
			name:      "Valid MSR with single frequency",
			msr:       "0x1A",
			want:      []int{0x1A},
			expectErr: false,
		},
		{
			name:      "Empty MSR string",
			msr:       "",
			want:      nil,
			expectErr: true,
		},
		{
			name:      "Invalid MSR string",
			msr:       "invalid_hex",
			want:      nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getFrequenciesFromHex(tt.msr)
			if (err != nil) != tt.expectErr {
				t.Errorf("getFrequenciesFromMSR() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getFrequenciesFromMSR() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestGetBucketSizesFromMSR(t *testing.T) {
	tests := []struct {
		name      string
		msr       string
		want      []int
		expectErr bool
	}{
		{
			name:      "Valid MSR with 8 bucket sizes",
			msr:       "0x0102030405060708",
			want:      []int{8, 7, 6, 5, 4, 3, 2, 1},
			expectErr: false,
		},
		{
			name:      "Valid MSR with reversed order",
			msr:       "0x0807060504030201",
			want:      []int{1, 2, 3, 4, 5, 6, 7, 8},
			expectErr: false,
		},
		{
			name:      "Invalid MSR string",
			msr:       "invalid_hex",
			want:      nil,
			expectErr: true,
		},
		{
			name:      "MSR with less than 8 bucket sizes",
			msr:       "0x01020304",
			want:      nil,
			expectErr: true,
		},
		{
			name:      "MSR with more than 8 bucket sizes",
			msr:       "0x010203040506070809",
			want:      nil,
			expectErr: true,
		},
		{
			name:      "Empty MSR string",
			msr:       "",
			want:      nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getBucketSizesFromHex(tt.msr)
			if (err != nil) != tt.expectErr {
				t.Errorf("getBucketSizesFromMSR() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getBucketSizesFromMSR() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestExpandTurboFrequencies(t *testing.T) {
	tests := []struct {
		name      string
		buckets   [][]string
		isa       string
		want      []string
		expectErr bool
	}{
		{
			name: "Valid input with single bucket",
			buckets: [][]string{
				{"Cores", "SSE", "AVX2"},
				{"1-4", "3.5", "3.2"},
			},
			isa:       "SSE",
			want:      []string{"3.5", "3.5", "3.5", "3.5"},
			expectErr: false,
		},
		{
			name: "Valid input with multiple buckets",
			buckets: [][]string{
				{"Cores", "SSE", "AVX2"},
				{"1-2", "3.5", "3.2"},
				{"3-4", "3.6", "3.3"},
			},
			isa:       "SSE",
			want:      []string{"3.5", "3.5", "3.6", "3.6"},
			expectErr: false,
		},
		{
			name: "ISA column not found",
			buckets: [][]string{
				{"Cores", "SSE", "AVX2"},
				{"1-4", "3.5", "3.2"},
			},
			isa:       "AVX512",
			want:      nil,
			expectErr: true,
		},
		{
			name: "Empty buckets",
			buckets: [][]string{
				{},
			},
			isa:       "SSE",
			want:      nil,
			expectErr: true,
		},
		{
			name: "Invalid bucket range",
			buckets: [][]string{
				{"Cores", "SSE", "AVX2"},
				{"1-", "3.5", "3.2"},
			},
			isa:       "SSE",
			want:      nil,
			expectErr: true,
		},
		{
			name: "Empty frequency value",
			buckets: [][]string{
				{"Cores", "SSE", "AVX2"},
				{"1-4", "", "3.2"},
			},
			isa:       "SSE",
			want:      nil,
			expectErr: true,
		},
		{
			name: "Whitespace in bucket range",
			buckets: [][]string{
				{"Cores", "SSE", "AVX2"},
				{" 1-4 ", "3.5", "3.2"},
			},
			isa:       "SSE",
			want:      []string{"3.5", "3.5", "3.5", "3.5"},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandTurboFrequencies(tt.buckets, tt.isa)
			if (err != nil) != tt.expectErr {
				t.Errorf("expandTurboFrequencies() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expandTurboFrequencies() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestGetSectionsFromOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   map[string]string
	}{
		{
			name: "Valid sections with content",
			output: `########## Section A ##########
Content A1
Content A2
########## Section B ##########
Content B1
Content B2
########## Section C ##########
Content C1`,
			want: map[string]string{
				"Section A": "Content A1\nContent A2\n",
				"Section B": "Content B1\nContent B2\n",
				"Section C": "Content C1\n",
			},
		},
		{
			name: "Valid sections with empty content",
			output: `########## Section A ##########
########## Section B ##########
########## Section C ##########`,
			want: map[string]string{
				"Section A": "",
				"Section B": "",
				"Section C": "",
			},
		},
		{
			name:   "No sections",
			output: "No section headers here",
			want:   map[string]string{},
		},
		{
			name:   "Empty output",
			output: ``,
			want:   map[string]string{},
		},
		{
			name:   "Empty lines in output",
			output: "\n\n\n",
			want:   map[string]string{},
		},
		{
			name: "Section with trailing newlines",
			output: `########## Section A ##########

Content A1

########## Section B ##########
Content B1`,
			want: map[string]string{
				"Section A": "\nContent A1\n\n",
				"Section B": "Content B1\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSectionsFromOutput(tt.output)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getSectionsFromOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestSectionValueFromOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		sectionName string
		want        string
	}{
		{
			name: "Section A exists with content",
			output: `########## Section A ##########
Content A1
Content A2
########## Section B ##########
Content B1
Content B2`,
			sectionName: "Section A",
			want:        "Content A1\nContent A2\n",
		},
		{
			name: "Section B exists with content",
			output: `########## Section A ##########
Content A1
Content A2
########## Section B ##########
Content B1
Content B2`,
			sectionName: "Section B",
			want:        "Content B1\nContent B2\n",
		},
		{
			name: "Section exists with no content",
			output: `########## Section A ##########
########## Section B ##########
Content B1`,
			sectionName: "Section A",
			want:        "",
		},
		{
			name: "Section does not exist",
			output: `########## Section A ##########
Content A1
########## Section B ##########
Content B1`,
			sectionName: "Section C",
			want:        "",
		},
		{
			name:        "Empty output",
			output:      "",
			sectionName: "Section A",
			want:        "",
		},
		{
			name: "Section with trailing newlines",
			output: `########## Section A ##########

Content A1

########## Section B ##########
Content B1`,
			sectionName: "Section A",
			want:        "\nContent A1\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sectionValueFromOutput(tt.output, tt.sectionName)
			if got != tt.want {
				t.Errorf("sectionValueFromOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestParseNicInfo(t *testing.T) {
	nics := parseNicInfo(nicinfo)
	if len(nics) != 3 {
		t.Errorf("expected 3 NICs, got %d", len(nics))
	}

	// Test first NIC
	first := nics[0]
	if first.Name != "ens7f0np0" {
		t.Errorf("expected Name 'ens7f0np0', got '%s'", first.Name)
	}
	if first.Vendor != "Broadcom Inc. and subsidiaries" {
		t.Errorf("expected Vendor 'Broadcom Inc. and subsidiaries', got '%s'", first.Vendor)
	}
	if first.Model == "" {
		t.Errorf("expected non-empty Model")
	}
	if first.Speed != "1000Mb/s" {
		t.Errorf("expected Speed '1000Mb/s', got '%s'", first.Speed)
	}
	if first.Link != "yes" {
		t.Errorf("expected Link 'yes', got '%s'", first.Link)
	}
	if first.Bus != "0000:4c:00.0" {
		t.Errorf("expected Bus '0000:4c:00.0', got '%s'", first.Bus)
	}
	if first.Driver != "bnxt_en" {
		t.Errorf("expected Driver 'bnxt_en', got '%s'", first.Driver)
	}
	if first.DriverVersion == "" {
		t.Errorf("expected non-empty DriverVersion")
	}
	if first.FirmwareVersion == "" {
		t.Errorf("expected non-empty FirmwareVersion")
	}
	if first.MACAddress != "04:32:01:f3:e1:a4" {
		t.Errorf("expected MACAddress '04:32:01:f3:e1:a4', got '%s'", first.MACAddress)
	}
	if first.NUMANode != "0" {
		t.Errorf("expected NUMANode '0', got '%s'", first.NUMANode)
	}
	if first.CPUAffinity == "" {
		t.Errorf("expected non-empty CPUAffinity")
	}
	if first.IRQBalance != "Disabled" {
		t.Errorf("expected IRQBalance 'Disabled', got '%s'", first.IRQBalance)
	}

	// Spot check second NIC
	second := nics[1]
	if second.Name != "ens7f1np1" {
		t.Errorf("expected Name 'ens7f1np1', got '%s'", second.Name)
	}
	if second.Model != "BCM57416 NetXtreme-E Dual-Media 10G RDMA Ethernet Controller" {
		t.Errorf("expected Model 'BCM57416 NetXtreme-E Dual-Media 10G RDMA Ethernet Controller', got '%s'", second.Model)
	}
	if second.Link != "no" {
		t.Errorf("expected Link 'no', got '%s'", second.Link)
	}

	// Spot check third NIC
	third := nics[2]
	if third.Name != "enx2aecf92702ac" {
		t.Errorf("expected Name 'enx2aecf92702ac', got '%s'", third.Name)
	}
	if third.Vendor != "Netchip Technology, Inc." {
		t.Errorf("expected Vendor 'Netchip Technology, Inc.', got '%s'", third.Vendor)
	}
}

var nicinfo = `
Interface: ens7f0np0
Vendor: Broadcom Inc. and subsidiaries
Model: BCM57416 NetXtreme-E Dual-Media 10G RDMA Ethernet Controller (NetXtreme-E Dual-port 10GBASE-T Ethernet OCP 3.0 Adapter (BCM957416N4160C))
Settings for ens7f0np0:
        Supported ports: [ TP ]
        Supported link modes:   1000baseT/Full
                                10000baseT/Full
        Supported pause frame use: Symmetric Receive-only
        Supports auto-negotiation: Yes
        Supported FEC modes: Not reported
        Advertised link modes:  1000baseT/Full
                                10000baseT/Full
        Advertised pause frame use: No
        Advertised auto-negotiation: Yes
        Advertised FEC modes: Not reported
        Speed: 1000Mb/s
        Lanes: 1
        Duplex: Full
        Auto-negotiation: on
        Port: Twisted Pair
        PHYAD: 12
        Transceiver: internal
        MDI-X: Unknown
        Supports Wake-on: g
        Wake-on: g
        Current message level: 0x00002081 (8321)
                               drv tx_err hw
        Link detected: yes
driver: bnxt_en
version: 6.13.7-061307-generic
firmware-version: 227.0.134.0/pkg 227.1.111.0
expansion-rom-version:
bus-info: 0000:4c:00.0
supports-statistics: yes
supports-test: yes
supports-eeprom-access: yes
supports-register-dump: yes
supports-priv-flags: no
MAC Address: 04:32:01:f3:e1:a4
NUMA Node: 0
CPU Affinity: 124:0-143;125:0-143;126:0-143;127:0-143;128:0-143;129:0-143;130:0-143;131:0-143;132:0-143;133:0-143;134:0-143;135:0-143;136:0-143;137:0-143;138:0-143;139:0-143;140:0-143;141:0-143;142:0-143;143:0-143;144:0-143;145:0-143;146:0-143;147:0-143;148:0-143;149:0-143;150:0-143;151:0-143;152:0-143;153:0-143;154:0-143;155:0-143;156:0-143;157:0-143;158:0-143;159:0-143;160:0-143;161:0-143;162:0-143;163:0-143;164:0-143;165:0-143;166:0-143;167:0-143;168:0-143;169:0-143;170:0-143;171:0-143;172:0-143;173:0-143;174:0-143;175:0-143;176:0-143;177:0-143;178:0-143;179:0-143;180:0-143;181:0-143;182:0-143;184:0-143;185:0-143;186:0-143;187:0-143;188:0-143;189:0-143;190:0-143;191:0-143;192:0-143;193:0-143;194:0-143;195:0-143;196:0-143;197:0-143;198:0-143;
IRQ Balance: Disabled
----------------------------------------
Interface: ens7f1np1
Vendor: Broadcom Inc. and subsidiaries
Model: BCM57416 NetXtreme-E Dual-Media 10G RDMA Ethernet Controller (NetXtreme-E Dual-port 10GBASE-T Ethernet OCP 3.0 Adapter (BCM957416N4160C))
Settings for ens7f1np1:
        Supported ports: [ TP ]
        Supported link modes:   1000baseT/Full
                                10000baseT/Full
        Supported pause frame use: Symmetric Receive-only
        Supports auto-negotiation: Yes
        Supported FEC modes: Not reported
        Advertised link modes:  1000baseT/Full
                                10000baseT/Full
        Advertised pause frame use: Symmetric
        Advertised auto-negotiation: Yes
        Advertised FEC modes: Not reported
        Speed: Unknown!
        Duplex: Unknown! (255)
        Auto-negotiation: on
        Port: Twisted Pair
        PHYAD: 13
        Transceiver: internal
        MDI-X: Unknown
        Supports Wake-on: g
        Wake-on: g
        Current message level: 0x00002081 (8321)
                               drv tx_err hw
        Link detected: no
driver: bnxt_en
version: 6.13.7-061307-generic
firmware-version: 227.0.134.0/pkg 227.1.111.0
expansion-rom-version:
bus-info: 0000:4c:00.1
supports-statistics: yes
supports-test: yes
supports-eeprom-access: yes
supports-register-dump: yes
supports-priv-flags: no
MAC Address: 04:32:01:f3:e1:a5
NUMA Node: 0
CPU Affinity: 454:0-143;455:0-143;456:0-143;457:0-143;458:0-143;459:0-143;460:0-143;461:0-143;462:0-143;463:0-143;464:0-143;465:0-143;466:0-143;467:0-143;468:0-143;469:0-143;470:0-143;471:0-143;472:0-143;473:0-143;474:0-143;475:0-143;476:0-143;477:0-143;478:0-143;479:0-143;480:0-143;481:0-143;482:0-143;483:0-143;484:0-143;485:0-143;486:0-143;487:0-143;488:0-143;489:0-143;490:0-143;491:0-143;492:0-143;493:0-143;494:0-143;495:0-143;496:0-143;497:0-143;498:0-143;499:0-143;500:0-143;501:0-143;502:0-143;503:0-143;504:0-143;505:0-143;506:0-143;507:0-143;508:0-143;509:0-143;510:0-143;511:0-143;512:0-143;513:0-143;514:0-143;515:0-143;516:0-143;517:0-143;518:0-143;519:0-143;520:0-143;521:0-143;522:0-143;523:0-143;524:0-143;525:0-143;526:0-143;527:0-143;
IRQ Balance: Disabled
----------------------------------------
Interface: enx2aecf92702ac
Vendor: Netchip Technology, Inc.
Model: Linux-USB Ethernet/RNDIS Gadget
Settings for enx2aecf92702ac:
        Supported ports: [  ]
        Supported link modes:   Not reported
        Supported pause frame use: No
        Supports auto-negotiation: No
        Supported FEC modes: Not reported
        Advertised link modes:  Not reported
        Advertised pause frame use: No
        Advertised auto-negotiation: No
        Advertised FEC modes: Not reported
        Speed: 425Mb/s
        Duplex: Half
        Auto-negotiation: off
        Port: Twisted Pair
        PHYAD: 0
        Transceiver: internal
        MDI-X: Unknown
        Current message level: 0x00000007 (7)
                               drv probe link
        Link detected: yes
driver: cdc_ether
version: 6.13.7-061307-generic
firmware-version: CDC Ethernet Device
expansion-rom-version:
bus-info: usb-0000:2c:00.0-3.1
supports-statistics: no
supports-test: no
supports-eeprom-access: no
supports-register-dump: no
supports-priv-flags: no
MAC Address: 2a:ec:f9:27:02:ac
NUMA Node:
CPU Affinity:
IRQ Balance: Disabled
----------------------------------------
`
