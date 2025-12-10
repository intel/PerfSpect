// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package common

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"perfspect/internal/script"
)

type nicInfo struct {
	Name            string
	Vendor          string
	VendorID        string
	Model           string
	ModelID         string
	Speed           string
	Link            string
	Bus             string
	Driver          string
	DriverVersion   string
	FirmwareVersion string
	MACAddress      string
	NUMANode        string
	CPUAffinity     string
	AdaptiveRX      string
	AdaptiveTX      string
	RxUsecs         string
	TxUsecs         string
	Card            string
	Port            string
	MTU             string
	IsVirtual       bool
	TXQueues        string
	RXQueues        string
	XPSCPUs         map[string]string
	RPSCPUs         map[string]string
}

func ParseNicInfo(scriptOutput string) []nicInfo {
	var nics []nicInfo
	for nicOutput := range strings.SplitSeq(scriptOutput, "----------------------------------------") {
		if strings.TrimSpace(nicOutput) == "" {
			continue
		}
		var nic nicInfo
		nic.XPSCPUs = make(map[string]string)
		nic.RPSCPUs = make(map[string]string)
		// Map of prefixes to field pointers
		fieldMap := map[string]*string{
			"Interface: ":        &nic.Name,
			"Vendor: ":           &nic.Vendor,
			"Vendor ID: ":        &nic.VendorID,
			"Model: ":            &nic.Model,
			"Model ID: ":         &nic.ModelID,
			"Speed: ":            &nic.Speed,
			"Link detected: ":    &nic.Link,
			"bus-info: ":         &nic.Bus,
			"driver: ":           &nic.Driver,
			"version: ":          &nic.DriverVersion,
			"firmware-version: ": &nic.FirmwareVersion,
			"MAC Address: ":      &nic.MACAddress,
			"NUMA Node: ":        &nic.NUMANode,
			"CPU Affinity: ":     &nic.CPUAffinity,
			"rx-usecs: ":         &nic.RxUsecs,
			"tx-usecs: ":         &nic.TxUsecs,
			"MTU: ":              &nic.MTU,
			"TX Queues: ":        &nic.TXQueues,
			"RX Queues: ":        &nic.RXQueues,
		}
		for line := range strings.SplitSeq(nicOutput, "\n") {
			line = strings.TrimSpace(line)
			// Special parsing for "Adaptive RX: off  TX: off" format
			if strings.HasPrefix(line, "Adaptive RX: ") {
				parts := strings.Split(line, "TX: ")
				if len(parts) == 2 {
					nic.AdaptiveRX = strings.TrimSpace(strings.TrimPrefix(parts[0], "Adaptive RX: "))
					nic.AdaptiveTX = strings.TrimSpace(parts[1])
				}
				continue
			}
			// Check if this is a virtual function
			if value, ok := strings.CutPrefix(line, "Virtual Function: "); ok {
				nic.IsVirtual = (strings.TrimSpace(value) == "yes")
				continue
			}
			// Special parsing for xps_cpus and rps_cpus
			if strings.HasPrefix(line, "xps_cpus tx-") {
				parts := strings.SplitN(line, ": ", 2)
				if len(parts) == 2 {
					queue := strings.TrimPrefix(parts[0], "xps_cpus ")
					nic.XPSCPUs[queue] = hexBitmapToCPUList(parts[1])
				}
				continue
			}
			if strings.HasPrefix(line, "rps_cpus rx-") {
				parts := strings.SplitN(line, ": ", 2)
				if len(parts) == 2 {
					queue := strings.TrimPrefix(parts[0], "rps_cpus ")
					nic.RPSCPUs[queue] = hexBitmapToCPUList(parts[1])
				}
				continue
			}
			for prefix, fieldPtr := range fieldMap {
				if after, ok := strings.CutPrefix(line, prefix); ok {
					*fieldPtr = after
					break
				}
			}
		}
		// special case for model as it sometimes has additional information in parentheses
		modelParts := strings.Split(nic.Model, "(")
		if len(modelParts) > 0 {
			nic.Model = strings.TrimSpace(modelParts[0])
		}
		nics = append(nics, nic)
	}
	// Assign card and port information
	assignCardAndPort(nics)
	return nics
}

func hexBitmapToCPUList(hexBitmap string) string {
	if hexBitmap == "" {
		return ""
	}

	// Remove commas to form a single continuous hex string.
	// This assumes the comma-separated parts are in big-endian order.
	fullHexBitmap := strings.ReplaceAll(hexBitmap, ",", "")

	i := new(big.Int)
	// The string is a hex string, so the base is 16.
	if _, success := i.SetString(fullHexBitmap, 16); !success {
		// If parsing fails, it might not be a hex string. Return as is.
		return hexBitmap
	}

	var cpus []string
	// Iterate through the bits of the big integer.
	for bit := 0; bit < i.BitLen(); bit++ {
		if i.Bit(bit) == 1 {
			cpus = append(cpus, fmt.Sprintf("%d", bit))
		}
	}
	if len(cpus) == 0 {
		return ""
	}
	return strings.Join(cpus, ",")
}

// assignCardAndPort assigns card and port numbers to NICs based on their PCI addresses
func assignCardAndPort(nics []nicInfo) {
	if len(nics) == 0 {
		return
	}

	// Map to store card identifiers (domain:bus:device) to card numbers
	cardMap := make(map[string]int)
	// Map to track ports within each card
	portMap := make(map[string][]int) // card identifier -> list of indices in nics slice
	cardCounter := 1

	// First pass: identify cards and group NICs by card
	for i := range nics {
		if nics[i].Bus == "" {
			continue
		}
		// PCI address format: domain:bus:device.function (e.g., 0000:32:00.0)
		// Extract domain:bus:device as the card identifier
		parts := strings.Split(nics[i].Bus, ":")
		if len(parts) != 3 {
			continue
		}
		// Further split the last part to separate device from function
		deviceFunc := strings.Split(parts[2], ".")
		if len(deviceFunc) < 1 {
			continue
		}
		// Card identifier is domain:bus:device
		cardID := parts[0] + ":" + parts[1] + ":" + deviceFunc[0]

		// Assign card number if not already assigned
		if _, exists := cardMap[cardID]; !exists {
			cardMap[cardID] = cardCounter
			cardCounter++
		}
		// Add this NIC index to the card's port list
		portMap[cardID] = append(portMap[cardID], i)
	}

	// Second pass: assign card and port numbers
	for cardID, nicIndices := range portMap {
		cardNum := cardMap[cardID]
		// Sort NICs within a card by their function number
		sort.Slice(nicIndices, func(i, j int) bool {
			// Extract function numbers
			funcI := extractFunction(nics[nicIndices[i]].Bus)
			funcJ := extractFunction(nics[nicIndices[j]].Bus)
			return funcI < funcJ
		})
		// Assign port numbers
		for portNum, nicIdx := range nicIndices {
			nics[nicIdx].Card = fmt.Sprintf("%d", cardNum)
			nics[nicIdx].Port = fmt.Sprintf("%d", portNum+1)
		}
	}
}

// extractFunction extracts the function number from a PCI address
func extractFunction(busAddr string) int {
	parts := strings.Split(busAddr, ".")
	if len(parts) != 2 {
		return 0
	}
	funcNum, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0
	}
	return funcNum
}

func NICIrqMappingsFromOutput(outputs map[string]script.ScriptOutput) [][]string {
	nics := ParseNicInfo(outputs[script.NicInfoScriptName].Stdout)
	if len(nics) == 0 {
		return nil
	}
	nicIRQMappings := [][]string{}
	for _, nic := range nics {
		if nic.CPUAffinity == "" {
			continue // skip NICs without CPU affinity
		}
		affinities := strings.Split(strings.TrimSuffix(nic.CPUAffinity, ";"), ";")
		nicIRQMappings = append(nicIRQMappings, []string{nic.Name, strings.Join(affinities, " | ")})
	}
	return nicIRQMappings
}

func NICSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	nics := ParseNicInfo(outputs[script.NicInfoScriptName].Stdout)
	if len(nics) == 0 {
		return "N/A"
	}
	modelCount := make(map[string]int)
	for _, nic := range nics {
		modelCount[nic.Model]++
	}
	var summary []string
	for model, count := range modelCount {
		if model == "" {
			model = "Unknown NIC"
		}
		summary = append(summary, fmt.Sprintf("%dx %s", count, model))
	}
	return strings.Join(summary, ", ")
}
