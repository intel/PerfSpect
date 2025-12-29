// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"os"
	"perfspect/cmd"
	"runtime/pprof"
)

func main() {
	// profile only if the environment variable is set
	if os.Getenv("PERFSPECT_PROFILE") != "" {
		// CPU profiling
		cpuFile, err := os.Create("cpu.prof")
		if err != nil {
			panic(err)
		}
		defer cpuFile.Close()

		if err := pprof.StartCPUProfile(cpuFile); err != nil {
			panic(err)
		}
		defer pprof.StopCPUProfile()

		// Memory profiling
		memFile, err := os.Create("mem.prof")
		if err != nil {
			panic(err)
		}
		defer memFile.Close()
		defer func() {
			if err := pprof.WriteHeapProfile(memFile); err != nil {
				panic(err)
			}
		}()
		defer func() {
			fmt.Printf("Profiling data written to cpu.prof and mem.prof\n")
			fmt.Printf("To analyze, use:\n")
			fmt.Printf("  go tool pprof cpu.prof\n")
			fmt.Printf("  go tool pprof mem.prof\n")
			fmt.Printf("For web visualization, use:\n")
			fmt.Printf("  go tool pprof -http=:8080 cpu.prof\n")
			fmt.Printf("  go tool pprof -inuse_space -http=:8081 mem.prof\n")
			fmt.Printf("  go tool pprof -alloc_space -http=:8081 mem.prof\n")
			fmt.Printf("  go tool pprof -alloc_objects -http=:8081 mem.prof\n")
		}()
	}
	cmd.Execute()
}
