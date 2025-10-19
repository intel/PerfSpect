package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"testing"

	"perfspect/internal/script"
)

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

	nics := parseNicInfo(sampleOutput)

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

func TestNicTableValuesWithCardPort(t *testing.T) {
	// Sample output simulating the scenario from the issue
	sampleOutput := `Interface: eth2
bus-info: 0000:32:00.0
Vendor: Intel Corporation
Model: Ethernet Controller 10G X550T
Speed: 1000Mb/s
Link detected: yes
----------------------------------------
Interface: eth3
bus-info: 0000:32:00.1
Vendor: Intel Corporation
Model: Ethernet Controller 10G X550T
Speed: Unknown!
Link detected: no
----------------------------------------
Interface: eth0
bus-info: 0000:c0:00.0
Vendor: Intel Corporation
Model: Ethernet Controller E810-C for QSFP
Speed: 100000Mb/s
Link detected: yes
----------------------------------------
Interface: eth1
bus-info: 0000:c0:00.1
Vendor: Intel Corporation
Model: Ethernet Controller E810-C for QSFP
Speed: 100000Mb/s
Link detected: yes
----------------------------------------`

	outputs := map[string]script.ScriptOutput{
		script.NicInfoScriptName: {Stdout: sampleOutput},
	}

	fields := nicTableValues(outputs)

	// Find the "Card / Port" field
	var cardPortField Field
	found := false
	for _, field := range fields {
		if field.Name == "Card / Port" {
			cardPortField = field
			found = true
			break
		}
	}

	if !found {
		t.Fatal("Card / Port field not found in NIC table")
	}

	// Verify we have 4 entries
	if len(cardPortField.Values) != 4 {
		t.Fatalf("Expected 4 Card / Port values, got %d", len(cardPortField.Values))
	}

	// Find the Name field to match values
	var nameField Field
	for _, field := range fields {
		if field.Name == "Name" {
			nameField = field
			break
		}
	}

	// Verify card/port assignments
	expectedCardPort := map[string]string{
		"eth2": "1 / 1",
		"eth3": "1 / 2",
		"eth0": "2 / 1",
		"eth1": "2 / 2",
	}

	for i, name := range nameField.Values {
		expected := expectedCardPort[name]
		actual := cardPortField.Values[i]
		if actual != expected {
			t.Errorf("NIC %s: expected Card / Port %q, got %q", name, expected, actual)
		}
	}
}
