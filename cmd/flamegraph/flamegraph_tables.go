package flamegraph

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"math"
	"perfspect/internal/common"
	"perfspect/internal/script"
	"perfspect/internal/table"
	"regexp"
	"strconv"
	"strings"
)

// flamegraph table names
const (
	CallStackFrequencyTableName = "Call Stack Frequency"
)

// flamegraph tables
var tableDefinitions = map[string]table.TableDefinition{
	CallStackFrequencyTableName: {
		Name:      CallStackFrequencyTableName,
		MenuLabel: CallStackFrequencyTableName,
		ScriptNames: []string{
			script.CollapsedCallStacksScriptName,
		},
		FieldsFunc: callStackFrequencyTableValues},
}

func callStackFrequencyTableValues(outputs map[string]script.ScriptOutput) []table.Field {
	fields := []table.Field{
		{Name: "Native Stacks", Values: []string{nativeFoldedFromOutput(outputs)}},
		{Name: "Java Stacks", Values: []string{javaFoldedFromOutput(outputs)}},
		{Name: "Maximum Render Depth", Values: []string{maxRenderDepthFromOutput(outputs)}},
	}
	return fields
}

func javaFoldedFromOutput(outputs map[string]script.ScriptOutput) string {
	if outputs[script.CollapsedCallStacksScriptName].Stdout == "" {
		slog.Warn("collapsed call stack output is empty")
		return ""
	}
	sections := common.GetSectionsFromOutput(outputs[script.CollapsedCallStacksScriptName].Stdout)
	if len(sections) == 0 {
		slog.Warn("no sections in collapsed call stack output")
		return ""
	}
	javaFolded := make(map[string]string)
	re := regexp.MustCompile(`^async-profiler (\d+) (.*)$`)
	for header, stacks := range sections {
		match := re.FindStringSubmatch(header)
		if match == nil {
			continue
		}
		pid := match[1]
		processName := match[2]
		if stacks == "" {
			slog.Warn("no stacks for java process", slog.String("header", header))
			continue
		}
		if strings.HasPrefix(stacks, "Failed to inject profiler") {
			slog.Error("profiling data error", slog.String("header", header))
			continue
		}
		_, ok := javaFolded[processName]
		if processName == "" {
			processName = "java (" + pid + ")"
		} else if ok {
			processName = processName + " (" + pid + ")"
		}
		javaFolded[processName] = stacks
	}
	folded, err := mergeJavaFolded(javaFolded)
	if err != nil {
		slog.Error("failed to merge java stacks", slog.String("error", err.Error()))
	}
	return folded
}

func nativeFoldedFromOutput(outputs map[string]script.ScriptOutput) string {
	if outputs[script.CollapsedCallStacksScriptName].Stdout == "" {
		slog.Warn("collapsed call stack output is empty")
		return ""
	}
	sections := common.GetSectionsFromOutput(outputs[script.CollapsedCallStacksScriptName].Stdout)
	if len(sections) == 0 {
		slog.Warn("no sections in collapsed call stack output")
		return ""
	}
	var dwarfFolded, fpFolded string
	for header, content := range sections {
		switch header {
		case "perf_dwarf":
			dwarfFolded = content
		case "perf_fp":
			fpFolded = content
		}
	}
	if dwarfFolded == "" && fpFolded == "" {
		return ""
	}
	folded, err := mergeSystemFolded(fpFolded, dwarfFolded)
	if err != nil {
		slog.Error("failed to merge native stacks", slog.String("error", err.Error()))
	}
	return folded
}

func maxRenderDepthFromOutput(outputs map[string]script.ScriptOutput) string {
	if outputs[script.CollapsedCallStacksScriptName].Stdout == "" {
		slog.Warn("collapsed call stack output is empty")
		return ""
	}
	sections := common.GetSectionsFromOutput(outputs[script.CollapsedCallStacksScriptName].Stdout)
	if len(sections) == 0 {
		slog.Warn("no sections in collapsed call stack output")
		return ""
	}
	for header, content := range sections {
		if header == "maximum depth" {
			return strings.TrimSpace(content)
		}
	}
	return ""
}

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
			fmt.Fprintf(&sb, "%s;%s %d\n", processName, stack, stackCount)
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

func callStackFrequencyTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	out := `<style>

/* Custom page header */
.fgheader {
	padding-bottom: 15px;
	padding-right: 15px;
	padding-left: 15px;
	border-bottom: 1px solid #e5e5e5;
}

/* Make the masthead heading the same height as the navigation */
.fgheader h3 {
    margin-top: 0;
    margin-bottom: 0;
    line-height: 40px;
}

/* Customize container */
.fgcontainer {
	max-width: 990px;
}
</style>
`
	out += renderFlameGraph("Native", tableValues, "Native Stacks")
	out += renderFlameGraph("Java", tableValues, "Java Stacks")
	return out
}
