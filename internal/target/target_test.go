package target

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"testing"
)

func TestNew(t *testing.T) {
	localTarget := NewLocalTarget()
	if localTarget == nil {
		t.Fatal("failed to create a local target")
	}
	remoteTarget := NewRemoteTarget("label", "hostname", "22", "user", "key")
	if remoteTarget == nil {
		t.Fatal("failed to create a remote target")
	}
}
