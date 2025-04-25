package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ProcessStacks ...
// [processName][callStack]=count
type ProcessStacks map[string]Stacks

type Stacks map[string]int

// example folded stack:
// swapper;secondary_startup_64_no_verify;start_secondary;cpu_startup_entry;arch_cpu_idle_enter 10523019

func (p *ProcessStacks) parsePerfFolded(folded string) (err error) {
	re := regexp.MustCompile(`^([\w,\-, ,\.]+);(.+) (\d+)$`)
	for line := range strings.SplitSeq(folded, "\n") {
		match := re.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		processName := match[1]
		stack := match[2]
		count, err := strconv.Atoi(match[3])
		if err != nil {
			continue
		}
		if _, ok := (*p)[processName]; !ok {
			(*p)[processName] = make(Stacks)
		}
		(*p)[processName][stack] = count
	}
	return
}

func (p *ProcessStacks) parseAsyncProfilerFolded(folded string, processName string) (err error) {
	for line := range strings.SplitSeq(folded, "\n") {
		splitAt := strings.LastIndex(line, " ")
		if splitAt == -1 {
			continue
		}
		stack := line[:splitAt]
		count, err := strconv.Atoi(line[splitAt+1:])
		if err != nil {
			continue
		}
		if _, ok := (*p)[processName]; !ok {
			(*p)[processName] = make(Stacks)
		}
		(*p)[processName][stack] = count
	}
	return
}

func (p *ProcessStacks) totalSamples() (count int) {
	count = 0
	for _, stacks := range *p {
		for _, stackCount := range stacks {
			count += stackCount
		}
	}
	return
}

func (p *ProcessStacks) scaleCounts(ratio float64) {
	for processName, stacks := range *p {
		for stack, stackCount := range stacks {
			(*p)[processName][stack] = int(math.Round(float64(stackCount) * ratio))
		}
	}
}

func (p *ProcessStacks) averageDepth(processName string) (average float64) {
	if _, ok := (*p)[processName]; !ok {
		average = 0
		return
	}
	total := 0
	count := 0
	for stack := range (*p)[processName] {
		total += len(strings.Split(stack, ";"))
		count += 1
	}
	if count == 0 {
		return
	}
	average = float64(total) / float64(count)
	return
}

func (p *ProcessStacks) dumpFolded() (folded string) {
	var sb strings.Builder
	for processName, stacks := range *p {
		for stack, stackCount := range stacks {
			sb.WriteString(fmt.Sprintf("%s;%s %d\n", processName, stack, stackCount))
		}
	}
	folded = sb.String()
	return
}

// helper functions below

// mergeJavaFolded -- merge profiles from N java processes
func mergeJavaFolded(javaFolded map[string]string) (merged string, err error) {
	javaStacks := make(ProcessStacks)
	for processName, stacks := range javaFolded {
		err = javaStacks.parseAsyncProfilerFolded(stacks, processName)
		if err != nil {
			continue
		}
	}
	merged = javaStacks.dumpFolded()
	return
}

// mergeSystemFolded -- merge the two sets of system perf stacks into one set
// For every process, get the average depth of stacks from Fp and Dwarf.
// The stacks with the deepest average (per process) will be retained in the
// merged set.
// The Dwarf stack counts will be scaled to the FP stack counts.
func mergeSystemFolded(perfFp string, perfDwarf string) (merged string, err error) {
	fpStacks := make(ProcessStacks)
	err = fpStacks.parsePerfFolded(perfFp)
	if err != nil {
		return
	}
	dwarfStacks := make(ProcessStacks)
	err = dwarfStacks.parsePerfFolded(perfDwarf)
	if err != nil {
		return
	}
	fpSampleCount := fpStacks.totalSamples()
	dwarfSampleCount := dwarfStacks.totalSamples()
	if fpSampleCount == 0 || dwarfSampleCount == 0 {
		err = fmt.Errorf("sample counts cannot be zero")
		return
	}
	fpToDwarfScalingRatio := float64(fpSampleCount) / float64(dwarfSampleCount)
	dwarfStacks.scaleCounts(fpToDwarfScalingRatio)

	// for every process in fpStacks, get the average stack depth from
	// fpStacks and dwarfStacks, choose the deeper stack for the merged set
	mergedStacks := make(ProcessStacks)
	for processName := range fpStacks {
		fpDepth := fpStacks.averageDepth(processName)
		dwarfDepth := dwarfStacks.averageDepth(processName)
		if fpDepth >= dwarfDepth {
			mergedStacks[processName] = fpStacks[processName]
		} else {
			mergedStacks[processName] = dwarfStacks[processName]
		}
	}

	merged = mergedStacks.dumpFolded()
	return
}
