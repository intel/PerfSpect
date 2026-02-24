// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package flamegraph

import "testing"

func parseFoldedForTest(t *testing.T, folded string) ProcessStacks {
	t.Helper()
	stacks := make(ProcessStacks)
	if err := stacks.parsePerfFolded(folded); err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	return stacks
}

func TestMergeSystemFoldedUsesDwarfWhenFpEmpty(t *testing.T) {
	fpFolded := ""
	dwarfFolded := "procA;foo;bar 3\nprocB;baz 2\n"

	merged, err := mergeSystemFolded(fpFolded, dwarfFolded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := parseFoldedForTest(t, merged)
	expected := parseFoldedForTest(t, dwarfFolded)
	if got.totalSamples() != expected.totalSamples() {
		t.Fatalf("expected %d samples, got %d", expected.totalSamples(), got.totalSamples())
	}
}

func TestMergeSystemFoldedUsesFpWhenDwarfEmpty(t *testing.T) {
	fpFolded := "procA;foo;bar 3\nprocB;baz 2\n"
	dwarfFolded := ""

	merged, err := mergeSystemFolded(fpFolded, dwarfFolded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := parseFoldedForTest(t, merged)
	expected := parseFoldedForTest(t, fpFolded)
	if got.totalSamples() != expected.totalSamples() {
		t.Fatalf("expected %d samples, got %d", expected.totalSamples(), got.totalSamples())
	}
}

func TestMergeSystemFoldedErrorsWhenBothEmpty(t *testing.T) {
	_, err := mergeSystemFolded("", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
