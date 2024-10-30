package report

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Intel Discrete GPUs (sorted by devid)
// references:
//   https://pci-ids.ucw.cz/read/PC/8086
//   https://dgpu-docs.intel.com/devices/hardware-table.html
//
//   The devid field will be interpreted as a regular expression.

type GPUDef struct {
	Model string
	MfgID string
	DevID string
}

var IntelGPUs = []GPUDef{
	{
		Model: "ATS-P",
		MfgID: "8086",
		DevID: "201",
	},
	{
		Model: "Ponte Vecchio 2T",
		MfgID: "8086",
		DevID: "BD0",
	},
	{
		Model: "Ponte Vecchio 1T",
		MfgID: "8086",
		DevID: "BD5",
	},
	{
		Model: "Intel® Iris® Xe MAX Graphics (DG1)",
		MfgID: "8086",
		DevID: "4905",
	},
	{
		Model: "Intel® Iris® Xe Pod (DG1)",
		MfgID: "8086",
		DevID: "4906",
	},
	{
		Model: "SG1",
		MfgID: "8086",
		DevID: "4907",
	},
	{
		Model: "Intel® Iris® Xe Graphics (DG1)",
		MfgID: "8086",
		DevID: "4908",
	},
	{
		Model: "Intel® Iris® Xe MAX 100 (DG1)",
		MfgID: "8086",
		DevID: "4909",
	},
	{
		Model: "DG2",
		MfgID: "8086",
		DevID: "(4F80|4F81|4F82)",
	},
	{
		Model: "Intel® Arc ™ A770M Graphics",
		MfgID: "8086",
		DevID: "5690",
	},
	{
		Model: "Intel® Arc ™ A730M Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "5691",
	},
	{
		Model: "Intel® Arc ™ A550M Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "5692",
	},
	{
		Model: "Intel® Arc ™ A370M Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "5693",
	},
	{
		Model: "Intel® Arc ™ A350M Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "5694",
	},
	{
		Model: "Intel® Arc ™ A770 Graphics",
		MfgID: "8086",
		DevID: "56A0",
	},
	{
		Model: "Intel® Arc ™ A750 Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "56A1",
	},
	{
		Model: "Intel® Arc ™ A380 Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "56A5",
	},
	{
		Model: "Intel® Arc ™ A310 Graphics (Alchemist)",
		MfgID: "8086",
		DevID: "56A6",
	},
	{
		Model: "Intel® Data Center GPU Flex 170",
		MfgID: "8086",
		DevID: "56C0",
	},
	{
		Model: "Intel® Data Center GPU Flex 140",
		MfgID: "8086",
		DevID: "56C1",
	},
	{
		Model: "Intel® Data Center GPU Flex 170V",
		MfgID: "8086",
		DevID: "56C2",
	},
}
