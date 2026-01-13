// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package main

// This code is a port of the Perl script stackcollapse-perf.pl from Brendan
// Gregg's Flamegraph project -- github.com/brendangregg/FlameGraph.
// All credit to Brendan Gregg for the original implementation and the flamegraph concept.

import (
	"bufio"
	"flag"
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
	var config Config

	flag.BoolVar(&config.AnnotateKernel, "kernel", false, "annotate kernel functions with a _[k]")
	flag.BoolVar(&config.AnnotateJit, "jit", false, "annotate jit functions with a _[j]")
	var annotateAll bool
	flag.BoolVar(&annotateAll, "all", false, "all annotations (--kernel --jit)")
	flag.BoolVar(&config.IncludePname, "pname", true, "include process names in stacks")
	flag.BoolVar(&config.IncludePid, "pid", false, "include PID with process names")
	flag.BoolVar(&config.IncludeTid, "tid", false, "include TID and PID with process names")
	flag.BoolVar(&config.IncludeAddrs, "addrs", false, "include raw addresses where symbols can't be found")
	flag.BoolVar(&config.TidyJava, "java", true, "condense Java signatures")
	flag.BoolVar(&config.TidyGeneric, "generic", true, "clean up function names a little")
	flag.StringVar(&config.EventFilter, "event-filter", "", "event name filter")
	flag.BoolVar(&config.ShowInline, "inline", false, "un-inline using addr2line")
	flag.BoolVar(&config.ShowContext, "context", false, "adds source context to --inline")
	flag.BoolVar(&config.SrcLineInInput, "srcline", false, "parses output of 'perf script -F+srcline' and adds source context")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "USAGE: %s [options] [infile] > outfile\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n[1] perf script must emit both PID and TIDs for these to work; eg, Linux < 4.1:\n")
		fmt.Fprintf(os.Stderr, "    perf script -f comm,pid,tid,cpu,time,event,ip,sym,dso,trace\n")
		fmt.Fprintf(os.Stderr, "    for Linux >= 4.1:\n")
		fmt.Fprintf(os.Stderr, "    perf script -F comm,pid,tid,cpu,time,event,ip,sym,dso,trace\n")
		fmt.Fprintf(os.Stderr, "    If you save this output add --header on Linux >= 3.14 to include perf info.\n")
	}

	flag.Parse()

	if annotateAll {
		config.AnnotateKernel = true
		config.AnnotateJit = true
	}

	if config.ShowInline {
		fmt.Fprintf(os.Stderr, "--inline is not implemented\n")
		os.Exit(1)
	}
	if config.SrcLineInInput {
		fmt.Fprintf(os.Stderr, "--srcline is not implemented\n")
		os.Exit(1)
	}

	var input *os.File
	var err error

	args := flag.Args()
	if len(args) > 0 {
		input, err = os.Open(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file: %s\n", err)
			os.Exit(1)
		}
		defer input.Close()
	} else {
		input = os.Stdin
	}

	err = ProcessStacks(input, os.Stdout, os.Stderr, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing stacks: %s\n", err)
		os.Exit(1)
	}
}

// Regular expressions for parsing the perf output
var (
	eventLineRegex         = regexp.MustCompile(`^(\S.+?)\s+(\d+)\/*(\d+)*\s+`)
	eventTypeRegex         = regexp.MustCompile(`:\s*(\d+)*\s+(\S+):\s*$`)
	stackLineRegex         = regexp.MustCompile(`^\s*(\w+)\s*(.+) \((.*)\)`)
	stripSymbolOffsetRegex = regexp.MustCompile(`\+0x[\da-f]+$`)
	goMethodRegex          = regexp.MustCompile(`\.\(.*\)\.`)
	jitRegex               = regexp.MustCompile(`/tmp/perf-\d+\.map`)
)

// ProcessStacks processes stack traces from the input reader and writes the collapsed stacks to the output writer.
// It uses the provided configuration to control the processing behavior.
func ProcessStacks(input io.Reader, output io.Writer, errorOutput io.Writer, config Config) error {
	var stack []string
	var processName string
	var period int
	aggregator := NewStackAggregator()
	scanner := bufio.NewScanner(input)
	eventFilter := config.EventFilter // if not set, it will be set to the first event encountered
	skipStackLines := false           // whether to skip stack lines based on event filtering

	// main loop, read lines from stdin
	for scanner.Scan() {
		line := scanner.Text()
		// skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}
		// skip empty lines that are not after a stack
		if line == "" && processName == "" {
			continue
		}
		// check for end of stack
		if line == "" {
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
		// check for event record
		if eventLineRegex.MatchString(line) {
			skipStackLines = false
			var err error
			var event string
			processName, period, event, err = handleEventRecord(line, config)
			if err != nil {
				fmt.Fprintf(errorOutput, "Error: %s\n", err)
				skipStackLines = true
				continue
			}
			if eventFilter == "" {
				eventFilter = event // default to first event
			} else if event != eventFilter {
				fmt.Fprintf(errorOutput, "Skipping event %s, filtering for %s\n", event, eventFilter)
				skipStackLines = true // need to skip stack lines for this event
			}
			continue
		}
		// check for stack line
		if stackLineRegex.MatchString(line) && !skipStackLines {
			err := handleStackLine(line, &stack, processName, config)
			if err != nil {
				fmt.Fprintf(errorOutput, "Error: %s\n", err)
			}
			continue
		}
	}
	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(errorOutput, "Error reading input: %s\n", err)
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
func handleEventRecord(line string, config Config) (processName string, period int, event string, err error) {
	matches := eventLineRegex.FindStringSubmatch(line)
	if matches == nil {
		return
	}

	comm, pid, tid := matches[1], matches[2], matches[3]
	if tid == "" {
		tid = pid
		pid = "?"
	}

	if eventMatches := eventTypeRegex.FindStringSubmatch(line); eventMatches != nil {
		eventPeriod := eventMatches[1]
		if eventPeriod == "" {
			period = 1
		} else {
			var eventPeriodInt int
			eventPeriodInt, err = strconv.Atoi(eventPeriod)
			if err != nil {
				err = fmt.Errorf("failed to parse event period: %s, error: %v", eventPeriod, err)
				return
			}
			period = eventPeriodInt
		}
		event = eventMatches[2]
	}

	if config.IncludeTid {
		processName = fmt.Sprintf("%s-%s/%s", comm, pid, tid)
	} else if config.IncludePid {
		processName = fmt.Sprintf("%s-%s", comm, pid)
	} else {
		processName = comm
	}
	processName = strings.ReplaceAll(processName, " ", "_")
	return
}

// handleStackLine parses a stack line and appends the function name to the stack based on the configuration.
func handleStackLine(line string, stack *[]string, pname string, config Config) error {
	matches := stackLineRegex.FindStringSubmatch(line)
	if matches == nil || pname == "" {
		return nil
	}

	pc, rawFunc, mod := matches[1], matches[2], matches[3]

	rawFunc = stripSymbolOffsetRegex.ReplaceAllString(rawFunc, "")

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
	var inline []string
	for funcname := range strings.SplitSeq(rawFunc, "->") {
		if funcname == "[unknown]" { // use module name instead, if known
			if mod != "[unknown]" {
				funcname = filepath.Base(mod)
			} else {
				funcname = "unknown"
			}

			if config.IncludeAddrs {
				funcname = fmt.Sprintf("[%s <%s>]", funcname, pc)
			} else {
				funcname = fmt.Sprintf("[%s]", funcname)
			}
		}
		if config.TidyGeneric {
			funcname = strings.ReplaceAll(funcname, ";", ":")
			if !goMethodRegex.MatchString(funcname) {
				funcname = stripParenArgsUnlessAnonymous(funcname)
			}
			funcname = strings.ReplaceAll(funcname, "\"", "")
			funcname = strings.ReplaceAll(funcname, "'", "")
		}
		if config.TidyJava {
			if strings.Contains(funcname, "/") {
				funcname = strings.TrimPrefix(funcname, "L")
			}
		}
		// annotations
		if len(inline) > 0 {
			if !strings.Contains(funcname, "_[i]") {
				funcname = fmt.Sprintf("%s_[i]", funcname)
			}
		} else if config.AnnotateKernel && (strings.HasPrefix(mod, "[") || strings.HasSuffix(mod, "vmlinux")) && !strings.Contains(mod, "unknown") {
			funcname = fmt.Sprintf("%s_[k]", funcname)
		} else if config.AnnotateJit && jitRegex.MatchString(mod) {
			if !strings.Contains(funcname, "_[j]") {
				funcname = fmt.Sprintf("%s_[j]", funcname)
			}
		}

		inline = append(inline, funcname)
	}
	return inline
}

// stripParenArgsUnlessAnonymous removes everything after the first '(' unless it is immediately followed by 'anonymous namespace'.
func stripParenArgsUnlessAnonymous(s string) string {
	idx := strings.Index(s, "(")
	if idx == -1 {
		return s
	}
	// Check if what follows is 'anonymous namespace'
	if strings.HasPrefix(s[idx:], "(anonymous namespace") {
		return s
	}
	return s[:idx]
}
