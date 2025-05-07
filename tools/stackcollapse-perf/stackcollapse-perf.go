package main

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// This code is a port of the Perl script stackcollapse-perf.pl from Brendan
// Gregg's Flamegraph project -- github.com/brendangregg/FlameGraph.
// All credit to Brendan Gregg for the original implementation and the flamegraph concept.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// In the original Perl code, the following
// are command line arguments. For our use, we don't need to set them as flags,
// but we can keep them for compatibility with the original
var annotateKernel = false
var annotateJit = false

// var annotateAll = false
var includePname = true
var includePid = false
var includeTid = false
var includeAddrs = false
var tidyJava = true
var tidyGeneric = true

// var targetPname = ""
var eventFilter = ""

// var showInline = false
// var showContext = false
// var srcLineInInput = false

type StackAggregator struct {
	collapsed map[string]int
}

func NewStackAggregator() *StackAggregator {
	return &StackAggregator{collapsed: make(map[string]int)}
}

func (sa *StackAggregator) RememberStack(stack string, count int) {
	sa.collapsed[stack] += count
}

func main() {
	aggregator := NewStackAggregator()

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
	scanner := bufio.NewScanner(input)

	var stack []string
	var pname string
	var mPeriod int
	var mTid string
	var mPid string

	eventRegex := regexp.MustCompile(`^(\S.+?)\s+(\d+)\/*(\d+)*\s+`)
	eventTypeRegex := regexp.MustCompile(`:\s*(\d+)*\s+(\S+):\s*$`)
	stackLineRegex := regexp.MustCompile(`^\s*(\w+)\s*(.+) \((.*)\)`)
	// inlineRegex := regexp.MustCompile(`(perf-\d+.map|kernel\.|\[[^\]]+\])`)
	stripSymbolsRegex := regexp.MustCompile(`\+0x[\da-f]+$`)
	stripIdRegex := regexp.MustCompile(`\.\(.*\)\.`)
	stripAnonymousRegex := regexp.MustCompile(`\([^a]*anonymous namespace[^)]*\)`)
	jitRegex := regexp.MustCompile(`/tmp/perf-\d+\.map`)

	var eventDefaulted bool
	var eventWarning bool

	// main loop, read lines from stdin
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "# cmdline") {
			// loop through the command line arguments in reverse order
			for i := len(os.Args) - 1; i > 0; i-- {
				if !strings.HasPrefix(os.Args[i], "-") {
					// not used -> target_pname = filepath.Base(os.Args[i])
					break
				}
			}
		}

		// Skip remaining comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// End of stack
		if line == "" {
			if pname == "" {
				continue
			}
			if includePname {
				// prepend the process name to the stack
				stack = append([]string{pname}, stack...)
			}

			if stack != nil {
				aggregator.RememberStack(strings.Join(stack, ";"), mPeriod)
			}
			stack = nil
			pname = ""
			continue
		}

		// Event record start
		if matches := eventRegex.FindStringSubmatch(line); matches != nil {
			comm, pid, tid, period := matches[1], matches[2], matches[3], ""
			if tid == "" {
				tid = pid
				pid = "?"
			}

			if eventMatches := eventTypeRegex.FindStringSubmatch(line); eventMatches != nil {
				period = eventMatches[1]
				event := eventMatches[2]

				if eventFilter == "" {
					eventFilter = event
					eventDefaulted = true
				} else if event != eventFilter {
					if eventDefaulted && !eventWarning {
						fmt.Fprintf(os.Stderr, "Filtering for events of type: %s\n", event)
						eventWarning = true
					}
					continue
				}
			}

			if period == "" {
				period = "1"
			}
			periodInt, err := strconv.Atoi(period)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing period: %s\n", err)
				continue
			}
			mPid, mTid, mPeriod = pid, tid, periodInt

			if includeTid {
				pname = fmt.Sprintf("%s-%s/%s", comm, mPid, mTid)
			} else if includePid {
				pname = fmt.Sprintf("%s-%s", comm, mPid)
			} else {
				pname = comm
			}
			pname = strings.ReplaceAll(pname, " ", "_")
			continue
			// Stack line
		} else if matches := stackLineRegex.FindStringSubmatch(line); matches != nil {
			if pname == "" {
				continue
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
				continue
			}
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

					if includeAddrs {
						funcname = fmt.Sprintf("[%s <%s>]", funcname, pc)
					} else {
						funcname = fmt.Sprintf("[%s]", funcname)
					}
				}
				if tidyGeneric {
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
				if tidyJava {
					if strings.Contains(funcname, "/") {
						// strip the leading L
						funcname = strings.TrimPrefix(funcname, "L")
					}
				}
				// annotations
				if len(inline) > 0 {
					if !strings.Contains(funcname, "_[i]") {
						funcname = fmt.Sprintf("%s_[i]", funcname)
					} else if annotateKernel && (strings.HasPrefix(funcname, "[") || strings.HasSuffix(funcname, "vmlinux")) && !strings.Contains(mod, "unknown") {
						funcname = fmt.Sprintf("%s_[k]", funcname)
					} else if annotateJit && jitRegex.MatchString(funcname) {
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

			// prepend inline array to the stack array
			if len(inline) > 0 {
				stack = append(inline, stack...)
			}

		} else {
			fmt.Fprintf(os.Stderr, "Unknown line format: %s\n", line)
		}
	}

	// Output results
	keys := make([]string, 0, len(aggregator.collapsed))
	for k := range aggregator.collapsed {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Printf("%s %d\n", k, aggregator.collapsed[k])
	}
}
