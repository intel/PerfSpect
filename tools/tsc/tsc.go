// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package main

import "fmt"

func main() {
	freq := GetTSCFreqMHz()
	fmt.Printf("%d", freq)
}
