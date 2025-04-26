package target

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"testing"
)

func TestNew(t *testing.T) {
	targets := []Target{}
	localTarget := NewLocalTarget()
	if localTarget == nil {
		t.Fatal("failed to create a local target")
	}
	remoteTarget := NewRemoteTarget("label", "hostname", "22", "user", "key")
	if remoteTarget == nil {
		t.Fatal("failed to create a remote target")
	}
	targets = append(targets, localTarget)
	targets = append(targets, remoteTarget)
	for _, target := range targets {
		if target.GetName() == "" {
			t.Fatal("failed to get target name")
		}
	}
}
