package progress

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"testing"
)

func TestNewMultiSpinner(t *testing.T) {
	spinner := NewMultiSpinner()
	if spinner == nil {
		t.Fatal("failed to create a spinner")
	}
}

func TestMultiSpinner(t *testing.T) {
	spinner := NewMultiSpinner()
	if spinner == nil {
		t.Fatal("failed to create a spinner")
	}
	if spinner.AddSpinner("A") != nil {
		t.Fatal("failed to add spinner")
	}
	if spinner.AddSpinner("B") != nil {
		t.Fatal("failed to add spinner")
	}
	if spinner.AddSpinner("A") == nil {
		t.Fatal("added spinner with same label")
	}
	spinner.Start()

	if spinner.Status("A", "FOO") != nil {
		t.Fatal("failed to update spinner status")
	}
	if spinner.Status("B", "BAR") != nil {
		t.Fatal("failed to update spinner status")
	}
	if spinner.Status("C", "WOOPS") == nil {
		t.Fatal("updated status of non-existent spinner")
	}
	spinner.Finish()
}
