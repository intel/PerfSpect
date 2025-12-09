// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Package table provides functions for accessing and processing table definitions.
package table

import (
	"fmt"
	"log/slog"
	"perfspect/internal/script"

	"github.com/xuri/excelize/v2"
)

// Field represents the values for a field in a table
type Field struct {
	Name        string
	Description string // optional description of the field
	Values      []string
}

// TableValues combines the table definition with the resulting fields and their values
type TableValues struct {
	TableDefinition
	Fields   []Field
	Insights []Insight
}

// Insight represents an insight about the data in a table
type Insight struct {
	Recommendation string
	Justification  string
}

type FieldsRetriever func(map[string]script.ScriptOutput) []Field
type InsightsRetriever func(map[string]script.ScriptOutput, TableValues) []Insight
type HTMLTableRenderer func(TableValues, string) string
type HTMLMultiTargetTableRenderer func([]TableValues, []string) string
type TextTableRenderer func(TableValues) string
type XlsxTableRenderer func(TableValues, *excelize.File, string, *int)

// TableDefinition defines the structure of a table in the report
type TableDefinition struct {
	Name               string
	ScriptNames        []string
	Architectures      []string // architectures, i.e., x86_64, aarch64. If empty, it will be present for all architectures.
	Vendors            []string // vendors, e.g., GenuineIntel, AuthenticAMD. If empty, it will be present for all vendors.
	MicroArchitectures []string // microarchitectures, e.g., EMR, ICX. If empty, it will be present for all microarchitectures.
	// Fields function is called to retrieve field values from the script outputs
	FieldsFunc  FieldsRetriever
	MenuLabel   string // add to tables that will be displayed in the menu
	HasRows     bool   // table is meant to be displayed in row form, i.e., a field may have multiple values
	NoDataFound string // message to display when no data is found
	// insights function is used to retrieve insights about the data in the table
	InsightsFunc InsightsRetriever
}

// ProcessTables processes the given tables and script outputs to generate table values.
// It collects values for each field in the tables and returns a slice of TableValues.
// If any error occurs during processing, it is returned along with the table values.
func ProcessTables(tables []TableDefinition, scriptOutputs map[string]script.ScriptOutput) (allTableValues []TableValues, err error) {
	for _, table := range tables {
		allTableValues = append(allTableValues, GetValuesForTable(table, scriptOutputs))
	}
	return
}

// GetFieldIndex returns the index of a field with the given name in the TableValues structure.
// Returns:
//   - int: The index of the field if found and valid, -1 otherwise
//   - error: nil if successful, an error describing the issue otherwise
func GetFieldIndex(fieldName string, tableValues TableValues) (int, error) {
	for i, field := range tableValues.Fields {
		if field.Name == fieldName {
			if len(field.Values) == 0 {
				return -1, fmt.Errorf("field [%s] does not have associated value(s)", field.Name)
			}
			return i, nil
		}
	}
	return -1, fmt.Errorf("field [%s] not found in table [%s]", fieldName, tableValues.Name)
}

// GetValuesForTable returns the fields and their values for the table with the given name
func GetValuesForTable(table TableDefinition, outputs map[string]script.ScriptOutput) TableValues {
	// ValuesFunc can't be nil
	if table.FieldsFunc == nil {
		panic(fmt.Sprintf("table %s, ValuesFunc cannot be nil", table.Name))
	}
	// call the table's FieldsFunc to get the table's fields and values
	fields := table.FieldsFunc(outputs)
	tableValues := TableValues{
		TableDefinition: table,
		Fields:          fields,
	}
	// sanity check
	if err := validateTableValues(tableValues); err != nil {
		slog.Error("table validation failed", "table", table.Name, "error", err)
		return TableValues{
			TableDefinition: table,
			Fields:          []Field{},
		}
	}
	// call the table's InsightsFunc to get insights about the data in the table
	if table.InsightsFunc != nil {
		tableValues.Insights = table.InsightsFunc(outputs, tableValues)
	}
	return tableValues
}

func validateTableValues(tableValues TableValues) error {
	if tableValues.Name == "" {
		return fmt.Errorf("table name cannot be empty")
	}
	// no field values is a valid state
	if len(tableValues.Fields) == 0 {
		return nil
	}
	// field names cannot be empty
	for i, field := range tableValues.Fields {
		if field.Name == "" {
			return fmt.Errorf("table %s, field %d, name cannot be empty", tableValues.Name, i)
		}
	}
	// the number of entries in each field must be the same
	numEntries := len(tableValues.Fields[0].Values)
	for i, field := range tableValues.Fields {
		if len(field.Values) != numEntries {
			return fmt.Errorf("table %s, field %d, %s, number of entries must be the same for all fields, expected %d, got %d", tableValues.Name, i, field.Name, numEntries, len(field.Values))
		}
	}
	return nil
}
