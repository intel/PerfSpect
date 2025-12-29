// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"testing"
)

func TestAssignCardAndPort(t *testing.T) {
	tests := []struct {
		name     string
		nics     []NicInfo
		expected map[string]string // map of NIC name to expected "Card / Port"
	}{
		{
			name: "Two cards with two ports each",
			nics: []NicInfo{
				{Name: "eth2", Bus: "0000:32:00.0"},
				{Name: "eth3", Bus: "0000:32:00.1"},
				{Name: "eth0", Bus: "0000:c0:00.0"},
				{Name: "eth1", Bus: "0000:c0:00.1"},
			},
			expected: map[string]string{
				"eth2": "1 / 1",
				"eth3": "1 / 2",
				"eth0": "2 / 1",
				"eth1": "2 / 2",
			},
		},
		{
			name: "Single card with four ports",
			nics: []NicInfo{
				{Name: "eth0", Bus: "0000:19:00.0"},
				{Name: "eth1", Bus: "0000:19:00.1"},
				{Name: "eth2", Bus: "0000:19:00.2"},
				{Name: "eth3", Bus: "0000:19:00.3"},
			},
			expected: map[string]string{
				"eth0": "1 / 1",
				"eth1": "1 / 2",
				"eth2": "1 / 3",
				"eth3": "1 / 4",
			},
		},
		{
			name: "Three different cards",
			nics: []NicInfo{
				{Name: "eth0", Bus: "0000:19:00.0"},
				{Name: "eth1", Bus: "0000:1a:00.0"},
				{Name: "eth2", Bus: "0000:1b:00.0"},
			},
			expected: map[string]string{
				"eth0": "1 / 1",
				"eth1": "2 / 1",
				"eth2": "3 / 1",
			},
		},
		{
			name: "Empty bus address should not assign card/port",
			nics: []NicInfo{
				{Name: "eth0", Bus: ""},
			},
			expected: map[string]string{
				"eth0": " / ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assignCardAndPort(tt.nics)
			for _, nic := range tt.nics {
				expected := tt.expected[nic.Name]
				actual := nic.Card + " / " + nic.Port
				if actual != expected {
					t.Errorf("NIC %s: expected %q, got %q", nic.Name, expected, actual)
				}
			}
		})
	}
}

func TestExtractFunction(t *testing.T) {
	tests := []struct {
		busAddr  string
		expected int
	}{
		{"0000:32:00.0", 0},
		{"0000:32:00.1", 1},
		{"0000:32:00.3", 3},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.busAddr, func(t *testing.T) {
			result := extractFunction(tt.busAddr)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestParseNicInfoWithCardPort(t *testing.T) {
	// Sample output simulating the scenario from the issue
	sampleOutput := `Interface: eth2
Vendor ID: 8086
Model ID: 1593
Vendor: Intel Corporation
Model: Ethernet Controller 10G X550T
Speed: 1000Mb/s
Link detected: yes
bus-info: 0000:32:00.0
driver: ixgbe
version: 5.1.0-k
firmware-version: 0x800009e0
MAC Address: aa:bb:cc:dd:ee:00
NUMA Node: 0
CPU Affinity:
IRQ Balance: Enabled
rx-usecs: 1
tx-usecs: 1
Adaptive RX: off  TX: off
----------------------------------------
Interface: eth3
Vendor ID: 8086
Model ID: 1593
Vendor: Intel Corporation
Model: Ethernet Controller 10G X550T
Speed: Unknown!
Link detected: no
bus-info: 0000:32:00.1
driver: ixgbe
version: 5.1.0-k
firmware-version: 0x800009e0
MAC Address: aa:bb:cc:dd:ee:01
NUMA Node: 0
CPU Affinity:
IRQ Balance: Enabled
rx-usecs: 1
tx-usecs: 1
Adaptive RX: off  TX: off
----------------------------------------
Interface: eth0
Vendor ID: 8086
Model ID: 37d2
Vendor: Intel Corporation
Model: Ethernet Controller E810-C for QSFP
Speed: 100000Mb/s
Link detected: yes
bus-info: 0000:c0:00.0
driver: ice
version: K_5.19.0-41-generic_5.1.9
firmware-version: 4.40 0x8001c967 1.3534.0
MAC Address: aa:bb:cc:dd:ee:82
NUMA Node: 1
CPU Affinity:
IRQ Balance: Enabled
rx-usecs: 1
tx-usecs: 1
Adaptive RX: off  TX: off
----------------------------------------
Interface: eth1
Vendor ID: 8086
Model ID: 37d2
Vendor: Intel Corporation
Model: Ethernet Controller E810-C for QSFP
Speed: 100000Mb/s
Link detected: yes
bus-info: 0000:c0:00.1
driver: ice
version: K_5.19.0-41-generic_5.1.9
firmware-version: 4.40 0x8001c967 1.3534.0
MAC Address: aa:bb:cc:dd:ee:83
NUMA Node: 1
CPU Affinity:
IRQ Balance: Enabled
rx-usecs: 1
tx-usecs: 1
Adaptive RX: off  TX: off
----------------------------------------`

	nics := ParseNicInfo(sampleOutput)

	if len(nics) != 4 {
		t.Fatalf("Expected 4 NICs, got %d", len(nics))
	}

	// Expected card/port assignments based on the issue example
	expectedCardPort := map[string]struct {
		card string
		port string
	}{
		"eth2": {"1", "1"}, // 0000:32:00.0
		"eth3": {"1", "2"}, // 0000:32:00.1
		"eth0": {"2", "1"}, // 0000:c0:00.0
		"eth1": {"2", "2"}, // 0000:c0:00.1
	}

	for _, nic := range nics {
		expected, exists := expectedCardPort[nic.Name]
		if !exists {
			t.Errorf("Unexpected NIC name: %s", nic.Name)
			continue
		}
		if nic.Card != expected.card {
			t.Errorf("NIC %s: expected card %s, got %s", nic.Name, expected.card, nic.Card)
		}
		if nic.Port != expected.port {
			t.Errorf("NIC %s: expected port %s, got %s", nic.Name, expected.port, nic.Port)
		}
	}
}

func TestParseNicInfo(t *testing.T) {
	nics := ParseNicInfo(nicinfo)
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
	if first.AdaptiveRX != "off" {
		t.Errorf("expected AdaptiveRX 'off', got '%s'", first.AdaptiveRX)
	}
	if first.AdaptiveTX != "off" {
		t.Errorf("expected AdaptiveTX 'off', got '%s'", first.AdaptiveTX)
	}
	if first.RxUsecs != "200" {
		t.Errorf("expected RxUsecs '200', got '%s'", first.RxUsecs)
	}
	if first.TxUsecs != "150" {
		t.Errorf("expected TxUsecs '150', got '%s'", first.TxUsecs)
	}
	if first.IsVirtual {
		t.Errorf("expected IsVirtual to be false for first NIC")
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
	if second.AdaptiveRX != "on" {
		t.Errorf("expected AdaptiveRX 'on', got '%s'", second.AdaptiveRX)
	}
	if second.AdaptiveTX != "on" {
		t.Errorf("expected AdaptiveTX 'on', got '%s'", second.AdaptiveTX)
	}
	if second.RxUsecs != "100" {
		t.Errorf("expected RxUsecs '100', got '%s'", second.RxUsecs)
	}
	if second.TxUsecs != "100" {
		t.Errorf("expected TxUsecs '100', got '%s'", second.TxUsecs)
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

func TestParseNicInfoWithVirtualFunction(t *testing.T) {
	nicinfoWithVF := `
Interface: eth0
Vendor: Intel Corporation
Vendor ID: 8086
Model: Ethernet Adaptive Virtual Function
Model ID: 1889
Speed: 10000Mb/s
Link detected: yes
driver: iavf
version: 6.13.7-061307-generic
firmware-version: N/A
bus-info: 0000:c0:11.0
MAC Address: 00:11:22:33:44:55
NUMA Node: 1
Virtual Function: yes
CPU Affinity: 100:0-63;
IRQ Balance: Enabled
Adaptive RX: on  TX: on
rx-usecs: 100
tx-usecs: 100
----------------------------------------
Interface: eth1
Vendor: Intel Corporation
Vendor ID: 8086
Model: Ethernet Controller E810-C
Model ID: 1592
Speed: 25000Mb/s
Link detected: yes
driver: ice
version: 6.13.7-061307-generic
firmware-version: 4.20
bus-info: 0000:c0:00.0
MAC Address: aa:bb:cc:dd:ee:ff
NUMA Node: 1
Virtual Function: no
CPU Affinity: 200:0-63;
IRQ Balance: Enabled
Adaptive RX: off  TX: off
rx-usecs: 50
tx-usecs: 50
----------------------------------------
`
	nics := ParseNicInfo(nicinfoWithVF)
	if len(nics) != 2 {
		t.Fatalf("expected 2 NICs, got %d", len(nics))
	}

	// Test virtual function
	vf := nics[0]
	if vf.Name != "eth0" {
		t.Errorf("expected Name 'eth0', got '%s'", vf.Name)
	}
	if !vf.IsVirtual {
		t.Errorf("expected IsVirtual to be true for eth0")
	}
	if vf.Model != "Ethernet Adaptive Virtual Function" {
		t.Errorf("expected Model 'Ethernet Adaptive Virtual Function', got '%s'", vf.Model)
	}

	// Test physical function
	pf := nics[1]
	if pf.Name != "eth1" {
		t.Errorf("expected Name 'eth1', got '%s'", pf.Name)
	}
	if pf.IsVirtual {
		t.Errorf("expected IsVirtual to be false for eth1")
	}
	if pf.Model != "Ethernet Controller E810-C" {
		t.Errorf("expected Model 'Ethernet Controller E810-C', got '%s'", pf.Model)
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
Coalesce parameters for ens7f0np0:
Adaptive RX: off  TX: off
stats-block-usecs: 0
sample-interval: 0
pkt-rate-low: 0
pkt-rate-high: 0

rx-usecs: 200
rx-frames: 0
rx-usecs-irq: 0
rx-frames-irq: 0

tx-usecs: 150
tx-frames: 0
tx-usecs-irq: 0
tx-frames-irq: 0
MAC Address: 04:32:01:f3:e1:a4
NUMA Node: 0
Virtual Function: no
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
Coalesce parameters for ens7f1np1:
Adaptive RX: on  TX: on
stats-block-usecs: 0
sample-interval: 0
pkt-rate-low: 0
pkt-rate-high: 0

rx-usecs: 100
rx-frames: 0
rx-usecs-irq: 0
rx-frames-irq: 0

tx-usecs: 100
tx-frames: 0
tx-usecs-irq: 0
tx-frames-irq: 0
MAC Address: 04:32:01:f3:e1:a5
NUMA Node: 0
Virtual Function: no
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
Virtual Function: no
CPU Affinity:
IRQ Balance: Disabled
----------------------------------------
`
