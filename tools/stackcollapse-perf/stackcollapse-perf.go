package main

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// This code is a port of the Perl script stackcollapse-perf.pl from Brendan
// Gregg's Flamegraph project -- github.com/brendangregg/FlameGraph.
// All credit to Brendan Gregg for the original implementation and the flamegraph concept.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Config holds configuration options for processing stacks.
// It includes options for annotating kernel and JIT symbols, including process names, PIDs, TIDs, and addresses,
// as well as options for tidying Java and generic function names, filtering events, and showing inline or context information.
type Config struct {
	AnnotateKernel bool
	AnnotateJit    bool
	IncludePname   bool
	IncludePid     bool
	IncludeTid     bool
	IncludeAddrs   bool
	TidyJava       bool
	TidyGeneric    bool
	EventFilter    string
	ShowInline     bool
	ShowContext    bool
	SrcLineInInput bool
}

// StackAggregator aggregates stack traces and their counts.
// It provides a method to remember stacks and their associated counts.
type StackAggregator struct {
	collapsed map[string]int
}

// NewStackAggregator creates and returns a new StackAggregator instance.
func NewStackAggregator() *StackAggregator {
	return &StackAggregator{collapsed: make(map[string]int)}
}

// RememberStack adds a stack trace and its count to the aggregator.
func (sa *StackAggregator) RememberStack(stack string, count int) {
	sa.collapsed[stack] += count
}

func main() {
	var input *os.File
	var err error

	// Check if a file path is provided as a command-line argument
	if len(os.Args) > 1 {
		input, err = os.Open(os.Args[1]) // Open the file
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file: %s\n", err)
			os.Exit(1)
		}
		defer input.Close()
	} else {
		input = os.Stdin // Default to standard input
	}

	var config = Config{
		AnnotateKernel: false,
		AnnotateJit:    false,
		IncludePname:   true,
		IncludePid:     false,
		IncludeTid:     false,
		IncludeAddrs:   false,
		TidyJava:       true,
		TidyGeneric:    true,
		EventFilter:    "",
		ShowInline:     false,
		ShowContext:    false,
		SrcLineInInput: false,
	}

	err = ProcessStacks(input, os.Stdout, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing stacks: %s\n", err)
		os.Exit(1)
	}
}

// Regular expressions for parsing the perf output
var (
	eventLineRegex = regexp.MustCompile(`^(\S.+?)\s+(\d+)\/*(\d+)*\s+`)
	eventTypeRegex = regexp.MustCompile(`:\s*(\d+)*\s+(\S+):\s*$`)
	stackLineRegex = regexp.MustCompile(`^\s*(\w+)\s*(.+) \((.*)\)`)
	// inlineRegex = regexp.MustCompile(`(perf-\d+.map|kernel\.|\[[^\]]+\])`)
	stripSymbolsRegex   = regexp.MustCompile(`\+0x[\da-f]+$`)
	stripIdRegex        = regexp.MustCompile(`\.\(.*\)\.`)
	stripAnonymousRegex = regexp.MustCompile(`\([^a]*anonymous namespace[^)]*\)`)
	jitRegex            = regexp.MustCompile(`/tmp/perf-\d+\.map`)
)

// ProcessStacks processes stack traces from the input reader and writes the collapsed stacks to the output writer.
// It uses the provided configuration to control the processing behavior.
func ProcessStacks(input io.Reader, output io.Writer, config Config) error {
	aggregator := NewStackAggregator()
	scanner := bufio.NewScanner(input)

	var stack []string
	var processName string
	var period int

	// main loop, read lines from stdin
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "#") {
			continue
		}

		if line == "" && processName == "" {
			continue
		}

		if line == "" { // check for End of stack
			if config.IncludePname {
				stack = append([]string{processName}, stack...)
			}
			if stack != nil {
				aggregator.RememberStack(strings.Join(stack, ";"), period)
			}
			stack = nil
			processName = ""
			continue
		}
		if err := handleEventRecord(line, &processName, &period, config); err != nil {
			fmt.Fprintf(output, "Error: %s\n", err)
			continue
		} else if err := handleStackLine(line, &stack, processName, config); err != nil {
			fmt.Fprintf(output, "Error: %s\n", err)
			continue
		}
	}

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %s\n", err)
		return err
	}

	// Output results
	keys := make([]string, 0, len(aggregator.collapsed))
	for k := range aggregator.collapsed {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Fprintf(output, "%s %d\n", k, aggregator.collapsed[k])
	}

	return nil
}

// handleEventRecord parses an event record line and updates the process name and period based on the configuration.
func handleEventRecord(line string, processName *string, period *int, config Config) error {
	matches := eventLineRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil
	}

	comm, pid, tid := matches[1], matches[2], matches[3]
	if tid == "" {
		tid = pid
		pid = "?"
	}

	if eventMatches := eventTypeRegex.FindStringSubmatch(line); eventMatches != nil {
		eventPeriod := eventMatches[1]
		if eventPeriod == "" {
			*period = 1
		} else {
			eventPeriodInt, err := strconv.Atoi(eventPeriod)
			if err != nil {
				return fmt.Errorf("failed to parse event period: %s, error: %v", eventPeriod, err)
			}
			*period = eventPeriodInt
		}
		event := eventMatches[2]

		if config.EventFilter == "" {
			config.EventFilter = event
		} else if event != config.EventFilter {
			return fmt.Errorf("event type mismatch: %s != %s", event, config.EventFilter)
		}
	}

	if config.IncludeTid {
		*processName = fmt.Sprintf("%s-%s/%s", comm, pid, tid)
	} else if config.IncludePid {
		*processName = fmt.Sprintf("%s-%s", comm, pid)
	} else {
		*processName = comm
	}
	*processName = strings.ReplaceAll(*processName, " ", "_")
	return nil
}

// handleStackLine parses a stack line and appends the function name to the stack based on the configuration.
func handleStackLine(line string, stack *[]string, pname string, config Config) error {
	matches := stackLineRegex.FindStringSubmatch(line)
	if matches == nil || pname == "" {
		return nil
	}

	pc, rawFunc, mod := matches[1], matches[2], matches[3]

	// skip for now as showInline is always false
	// if showInline && !inlineRegex.MatchString(mod) {
	// 	inlineRes := inline(pc, rawFunc, mod)
	// if inlineRes != "" && inlineRes != "??" && inlineRes != "??:??:0" {
	// 	// prepend the inline result to the stack
	// 	stack = append([]string{inlineRes}, stack...)
	// 	continue
	// }
	//}

	// strip symbol offsets from rawFunc
	// symbol offsets match this regex: \+0x[\da-f]+$
	rawFunc = stripSymbolsRegex.ReplaceAllString(rawFunc, "")

	// skip process names
	if strings.HasPrefix(rawFunc, "(") {
		return nil
	}

	*stack = append(processFunctionName(rawFunc, mod, pc, config), *stack...)
	return nil
}

// processFunctionName processes a raw function name, module, and program counter (PC) based on the configuration.
// It returns a slice of processed function names.
func processFunctionName(rawFunc, mod, pc string, config Config) []string {
	// var isUnknown bool
	var inline []string
	for funcname := range strings.SplitSeq(rawFunc, "->") {
		if funcname == "[unknown]" { // use module name instead, if known
			if mod != "[unknown]" {
				funcname = filepath.Base(mod)
			} else {
				funcname = "unknown"
				// isUnknown = true
			}

			if config.IncludeAddrs {
				funcname = fmt.Sprintf("[%s <%s>]", funcname, pc)
			} else {
				funcname = fmt.Sprintf("[%s]", funcname)
			}
		}
		if config.TidyGeneric {
			funcname = strings.ReplaceAll(funcname, ";", ":")
			if matches := stripIdRegex.FindStringSubmatch(funcname); matches != nil {
				index := stripAnonymousRegex.FindStringIndex(funcname)
				if index != nil {
					funcname = funcname[0:index[0]]
				}
			}
			funcname = strings.ReplaceAll(funcname, "\"", "")
			funcname = strings.ReplaceAll(funcname, "'", "")
		}
		if config.TidyJava {
			if strings.Contains(funcname, "/") {
				// strip the leading L
				funcname = strings.TrimPrefix(funcname, "L")
			}
		}
		// annotations
		if len(inline) > 0 {
			if !strings.Contains(funcname, "_[i]") {
				funcname = fmt.Sprintf("%s_[i]", funcname)
			} else if config.AnnotateKernel && (strings.HasPrefix(funcname, "[") || strings.HasSuffix(funcname, "vmlinux")) && !strings.Contains(mod, "unknown") {
				funcname = fmt.Sprintf("%s_[k]", funcname)
			} else if config.AnnotateJit && jitRegex.MatchString(funcname) {
				if !strings.Contains(funcname, "_[j]") {
					funcname = fmt.Sprintf("%s_[j]", funcname)
				}
			}
		}

		// source lines
		// skip for now since srcLineInInput is always false
		// 	if srcLineInInput && !isUnknown {
		// }

		inline = append(inline, funcname)
	}
	return inline
}
