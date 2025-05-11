package main

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bytes"
	"strings"
	"testing"
)

// keep for debugging
// func TestProcessStacksFromFile(t *testing.T) {
// 	filePath := filepath.Join("testdata", "perf_fp_stacks")
// 	file, err := os.Open(filePath)
// 	if err != nil {
// 		t.Fatalf("failed to open test file: %v", err)
// 	}
// 	defer file.Close()

// 	output := &bytes.Buffer{}
// 	config := Config{
// 		IncludePname: true,
// 		IncludePid:   false,
// 		IncludeTid:   false,
// 		IncludeAddrs: false,
// 		TidyJava:     true,
// 		TidyGeneric:  true,
// 	}

// 	err = ProcessStacks(file, output, config)
// 	if err != nil {
// 		t.Fatalf("unexpected error: %v", err)
// 	}

// 	if output.Len() == 0 {
// 		t.Errorf("expected output, got empty result")
// 	}
// }

func TestProcessStacks(t *testing.T) {
	input := strings.NewReader(`
stress-ng-cpu 1230556 [121] 6223127.073349:  293637623 cycles:P: 
	    61e248df6091 [unknown] (/usr/bin/stress-ng)

stress-ng-cpu 1230793 [098] 6223127.074783:  307465331 cycles:P: 
	ffffffffa7c00f0b asm_sysvec_apic_timer_interrupt+0x1b ([kernel.kallsyms])
	    760c9702dc5d [unknown] (/usr/lib/x86_64-linux-gnu/libm.so.6)
	    760c96fda3a2 sincosf64x+0x122 (/usr/lib/x86_64-linux-gnu/libm.so.6)

	`)
	output := &bytes.Buffer{}

	config := Config{
		IncludePname: true,
		IncludePid:   false,
		IncludeTid:   false,
		IncludeAddrs: false,
		TidyJava:     true,
		TidyGeneric:  true,
	}

	err := ProcessStacks(input, output, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "stress-ng-cpu;[stress-ng] 293637623\nstress-ng-cpu;sincosf64x;[libm.so.6];asm_sysvec_apic_timer_interrupt 307465331\n"
	if output.String() != expected {
		t.Errorf("expected %q, got %q", expected, output.String())
	}
}

func TestHandleEventRecord(t *testing.T) {
	line := "stress-ng-cpu 1230556 [121] 6223127.073349:  293637623 cycles:P: "
	var processName string
	var period int
	config := Config{
		IncludePname: true,
		IncludePid:   false,
		IncludeTid:   false,
		IncludeAddrs: false,
		TidyJava:     true,
		TidyGeneric:  true,
	}

	processName, period, err := handleEventRecord(line, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedProcessName := "stress-ng-cpu"
	if processName != "stress-ng-cpu" {
		t.Errorf("expected processName to be '%s', got %q", expectedProcessName, processName)
	}
	expectedPeriod := 293637623
	if period != expectedPeriod {
		t.Errorf("expected period to be %d, got %d", expectedPeriod, period)
	}
}

func TestHandleStackLine(t *testing.T) {
	line := "0x1234 someFunction (module)"
	var stack []string
	processName := "main"
	config := Config{
		IncludePname: true,
		IncludePid:   false,
		IncludeTid:   false,
		IncludeAddrs: false,
		TidyJava:     true,
		TidyGeneric:  true,
	}

	err := handleStackLine(line, &stack, processName, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stack) != 1 || stack[0] != "someFunction" {
		t.Errorf("expected stack to contain 'someFunction', got %v", stack)
	}
}

func TestProcessFunctionName(t *testing.T) {
	tests := []struct {
		rawFunc  string
		mod      string
		pc       string
		config   Config
		expected []string
	}{
		{
			rawFunc:  "[unknown]",
			mod:      "[kernel]",
			pc:       "0x1234",
			config:   Config{IncludeAddrs: true},
			expected: []string{"[[kernel] <0x1234>]"},
		},
		{
			rawFunc:  "Lcom/example/MyClass",
			mod:      "[unknown]",
			pc:       "0x5678",
			config:   Config{TidyJava: true},
			expected: []string{"com/example/MyClass"},
		},
		{
			rawFunc:  "someFunction;bar\"hello'world(remove me).foo",
			mod:      "module.so",
			pc:       "0x9abc",
			config:   Config{TidyGeneric: true},
			expected: []string{"someFunction:barhelloworld"},
		},
	}

	for _, test := range tests {
		result := processFunctionName(test.rawFunc, test.mod, test.pc, test.config)
		if len(result) != len(test.expected) {
			t.Errorf("expected %d results, got %d", len(test.expected), len(result))
			continue
		}
		for i := range result {
			if result[i] != test.expected[i] {
				t.Errorf("expected %q, got %q", test.expected[i], result[i])
			}
		}
	}
}
func TestStripParenArgsUnlessAnonymous(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"foo(int, float)", "foo"},
		{"bar()", "bar"},
		{"baz", "baz"},
		{"qux(anonymous namespace)", "qux(anonymous namespace)"},
		{"func(anonymous namespace::Type)", "func(anonymous namespace::Type)"},
		{"func(anonymous namespace", "func(anonymous namespace"},
		{"func(int) (anonymous namespace)", "func"},
		{"func()", "func"},
		{"func", "func"},
		{"func(abc", "func"},
	}

	for _, test := range tests {
		result := stripParenArgsUnlessAnonymous(test.input)
		if result != test.expected {
			t.Errorf("stripParenArgsUnlessAnonymous(%q) = %q; want %q", test.input, result, test.expected)
		}
	}
}
