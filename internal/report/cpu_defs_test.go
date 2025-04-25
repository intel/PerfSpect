package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"testing"
)

func TestGetCPU(t *testing.T) {
	// should fail
	_, err := GetCPU("0", "0", "0")
	if err == nil {
		t.Fatal(err)
	}

	cpu, err := GetCPU("6", "85", "4") //SKX
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "SKX" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}

	cpu, err = GetCPU("6", "85", "7") //CLX
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "CLX" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}

	cpu, err = GetCPU("6", "85", "6") //CLX
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "CLX" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}

	cpu, err = GetCPU("6", "108", "0") //ICX
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "ICX" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}

	cpu, err = GetCPU("6", "71", "0") //BDW
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "BDW" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}

	cpu, err = getCPUExtended("6", "173", "", "", "10") // GNR_X3
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "GNR_X3" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}
	if cpu.MemoryChannelCount != 12 {
		t.Fatal(fmt.Errorf("Incorrect channel count: %d", cpu.MemoryChannelCount))
	}

	cpu, err = getCPUExtended("6", "173", "", "", "8") // GNR_X2
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "GNR_X2" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}
	if cpu.MemoryChannelCount != 8 {
		t.Fatal(fmt.Errorf("Incorrect channel count: %d", cpu.MemoryChannelCount))
	}

	cpu, err = getCPUExtended("6", "173", "", "", "6") // GNR_X1
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "GNR_X1" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}
	if cpu.MemoryChannelCount != 8 {
		t.Fatal(fmt.Errorf("Incorrect channel count: %d", cpu.MemoryChannelCount))
	}

	cpu, err = GetCPU("6", "173", "") // GNR with no differentiation
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "GNR" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}

	cpu, err = getCPUExtended("6", "207", "", "c0", "") // EMR XCC
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "EMR_XCC" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}
	if cpu.MemoryChannelCount != 8 {
		t.Fatal(fmt.Errorf("Incorrect channel count: %d", cpu.MemoryChannelCount))
	}

	cpu, err = getCPUExtended("6", "207", "", "40", "") // EMR MCC
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "EMR_MCC" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}
	if cpu.MemoryChannelCount != 8 {
		t.Fatal(fmt.Errorf("Incorrect channel count: %d", cpu.MemoryChannelCount))
	}

	cpu, err = GetCPU("6", "207", "") // EMR with no differentiation
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "EMR" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}
	if cpu.MemoryChannelCount != 8 {
		t.Fatal(fmt.Errorf("Incorrect channel count: %d", cpu.MemoryChannelCount))
	}

	cpu, err = GetCPU("25", "1", "") // Milan
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "Milan" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}
	if cpu.MemoryChannelCount != 8 {
		t.Fatal(fmt.Errorf("Incorrect channel count: %d", cpu.MemoryChannelCount))
	}

	cpu, err = GetCPU("25", "17", "") // Genoa
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "Genoa" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}
	if cpu.MemoryChannelCount != 12 {
		t.Fatal(fmt.Errorf("Incorrect channel count: %d", cpu.MemoryChannelCount))
	}

	cpu, err = GetCPU("6", "69", "99") //HSW
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "HSW" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}

	cpu, err = GetCPU("6", "70", "") //HSW
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "HSW" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}

	cpu, err = GetCPU("", "1", "r3p1") // N1
	if err != nil {
		t.Fatal(err)
	}
	if cpu.MicroArchitecture != "Neoverse N1" {
		t.Fatal(fmt.Errorf("Found the wrong CPU: %s", cpu.MicroArchitecture))
	}
}
