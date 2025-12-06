package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"encoding/json"
	"perfspect/internal/table"
)

func createJsonReport(allTableValues []table.TableValues) (out []byte, err error) {
	type outRecord map[string]string
	type outTable []outRecord
	type outReport map[string]outTable
	oReport := make(outReport)
	for _, tableValues := range allTableValues {
		if len(tableValues.Fields) == 0 || len(tableValues.Fields[0].Values) == 0 {
			oReport[tableValues.Name] = outTable{}
			continue
		}
		var oTable outTable
		for recordIdx := range len(tableValues.Fields[0].Values) {
			oRecord := make(outRecord)
			for _, field := range tableValues.Fields {
				oRecord[field.Name] = field.Values[recordIdx]
			}
			oTable = append(oTable, oRecord)
		}
		oReport[tableValues.Name] = oTable
	}
	return json.MarshalIndent(oReport, "", " ")
}
