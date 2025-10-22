package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// table.go provides functions for accessing and processing table definitions.

import (
	"fmt"
	"log/slog"
	"perfspect/internal/cpus"
	"perfspect/internal/script"
	"perfspect/internal/target"
	"slices"
)

// GetTableByName retrieves a table definition by its name.
func GetTableByName(name string) TableDefinition {
	if table, ok := tableDefinitions[name]; ok {
		return table
	}
	panic(fmt.Sprintf("table not found: %s", name))
}

// IsTableForTarget checks if the given table is applicable for the specified target
func IsTableForTarget(tableName string, myTarget target.Target) bool {
	table := GetTableByName(tableName)
	if len(table.Architectures) > 0 {
		architecture, err := myTarget.GetArchitecture()
		if err != nil {
			slog.Error("failed to get architecture for target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
			return false
		}
		if !slices.Contains(table.Architectures, architecture) {
			return false
		}
	}
	if len(table.Vendors) > 0 {
		vendor, err := myTarget.GetVendor()
		if err != nil {
			slog.Error("failed to get vendor for target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
			return false
		}
		if !slices.Contains(table.Vendors, vendor) {
			return false
		}
	}
	if len(table.MicroArchitectures) > 0 {
		family, err := myTarget.GetFamily()
		if err != nil {
			slog.Error("failed to get family for target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
			return false
		}
		model, err := myTarget.GetModel()
		if err != nil {
			slog.Error("failed to get model for target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
			return false
		}
		stepping, err := myTarget.GetStepping()
		if err != nil {
			slog.Error("failed to get stepping for target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
			return false
		}
		cpu, err := cpus.GetCPU(family, model, stepping)
		if err != nil {
			slog.Error("failed to get CPU for target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
			return false
		}
		if !slices.Contains(table.MicroArchitectures, cpu.GetMicroArchitecture()) && !slices.Contains(table.MicroArchitectures, cpu.GetShortMicroArchitecture()) {
			return false
		}
	}
	return true
}

// ProcessTables processes the given tables and script outputs to generate table values.
// It collects values for each field in the tables and returns a slice of TableValues.
// If any error occurs during processing, it is returned along with the table values.
func ProcessTables(tableNames []string, scriptOutputs map[string]script.ScriptOutput) (allTableValues []TableValues, err error) {
	for _, tableName := range tableNames {
		allTableValues = append(allTableValues, GetValuesForTable(tableName, scriptOutputs))
	}
	return
}
