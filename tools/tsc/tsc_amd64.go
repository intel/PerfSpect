// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Time Stamp Counter helper functions.
package main

import (
	"time"
)

// GetTSCFreqMHz - gets the TSC frequency
func GetTSCFreqMHz() (freqMHz int) {
	start := GetTSCStart()
	time.Sleep(time.Millisecond * 1000)
	end := GetTSCEnd()
	freqMHz = int((end - start) / 1000000) // #nosec G115
	return
}

func GetTSCStart() uint64

func GetTSCEnd() uint64
