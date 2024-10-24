package report

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Intel Accelerators (sorted by devid)
// references:
//   https://pci-ids.ucw.cz/read/PC/8086

type Accelerator struct {
	MfgID       string
	DevID       string
	Name        string
	FullName    string
	Description string
}

var accelDefs = []Accelerator{
	{
		MfgID:       "8086",
		DevID:       "(2710|2714)",
		Name:        "DLB",
		FullName:    "Intel Dynamic Load Balancer",
		Description: "hardware managed system of queues and arbiters connecting producers and consumers",
	},
	{
		MfgID:       "8086",
		DevID:       "B25",
		Name:        "DSA",
		FullName:    "Intel Data Streaming Accelerator",
		Description: "a high-performance data copy and transformation accelerator",
	},
	{
		MfgID:       "8086",
		DevID:       "CFE",
		Name:        "IAA",
		FullName:    "Intel Analytics Accelerator",
		Description: "accelerates compression and decompression for big data applications and in-memory analytic databases",
	},
	{
		MfgID:       "8086",
		DevID:       "(4940|4942|4944)",
		Name:        "QAT (on CPU)",
		FullName:    "Intel Quick Assist Technology",
		Description: "accelerates data encryption and compression for applications from networking to enterprise, cloud to storage, and content delivery to database",
	},
	{
		MfgID:       "8086",
		DevID:       "37C8",
		Name:        "QAT (on chipset)",
		FullName:    "Intel Quick Assist Technology",
		Description: "accelerates data encryption and compression for applications from networking to enterprise, cloud to storage, and content delivery to database",
	},
	{
		MfgID:       "8086",
		DevID:       "57C2",
		Name:        "vRAN Boost",
		FullName:    "Intel vRAN Boost",
		Description: "accelerates vRAN workloads",
	},
}
