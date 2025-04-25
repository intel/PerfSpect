package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import "encoding/json"

func createJsonReport(allTableValues []TableValues) (out []byte, err error) {
	type outRecord map[string]string
	type outTable []outRecord
	type outReport map[string]outTable
	oReport := make(outReport)
	for _, tableValues := range allTableValues {
		var oTable outTable
		if len(tableValues.Fields) == 0 {
			oReport[tableValues.Name] = oTable
			continue
		}
		numRecords := len(tableValues.Fields[0].Values)
		if numRecords > 0 {
			for recordIdx := range numRecords {
				oRecord := make(outRecord)
				for _, field := range tableValues.Fields {
					oRecord[field.Name] = field.Values[recordIdx]
				}
				oTable = append(oTable, oRecord)
			}
		} else {
			// insert an empty record
			oRecord := make(outRecord)
			for _, field := range tableValues.Fields {
				oRecord[field.Name] = ""
			}
			oTable = append(oTable, oRecord)
		}
		oReport[tableValues.Name] = oTable
	}
	return json.MarshalIndent(oReport, "", " ")
}
