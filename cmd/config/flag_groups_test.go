package config

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestValidateFlags(t *testing.T) {
	// Create a mock command
	cmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {},
	}
	// Mock flag groups and flags
	flagGroups = []flagGroup{}
	group := flagGroup{
		name:  "testGroup",
		flags: []flagDefinition{},
	}
	group.flags = append(group.flags, newStringFlag(cmd,
		"testFlag",
		"",
		nil,
		"A test flag",
		"valid value",
		func(cmd *cobra.Command) bool {
			value, _ := cmd.Flags().GetString("testFlag")
			return value == "validValue"
		}))
	flagGroups = append(flagGroups, group)

	// Test case: Invalid flag value
	t.Run("InvalidFlagValue", func(t *testing.T) {
		_ = cmd.Flags().Set("testFlag", "invalidValue")
		var stderr bytes.Buffer
		cmd.SetErr(&stderr)

		err := validateFlags(cmd, []string{})
		assert.Error(t, err)
	})

	// Test case: Valid flag value
	t.Run("ValidFlagValue", func(t *testing.T) {
		_ = cmd.Flags().Set("testFlag", "validValue")
		var stderr bytes.Buffer
		cmd.SetErr(&stderr)

		err := validateFlags(cmd, []string{})
		assert.NoError(t, err)
	})
}
