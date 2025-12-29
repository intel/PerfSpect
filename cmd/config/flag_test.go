// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package config

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestFlagDefinition_GetName(t *testing.T) {
	// Create a mock pflag.Flag
	mockFlag := &pflag.Flag{
		Name: "test-flag",
	}

	// Create a flagDefinition instance with the mock flag
	flagDef := flagDefinition{
		pflag: mockFlag,
	}

	// Call GetName and verify the result
	result := flagDef.GetName()
	assert.Equal(t, "test-flag", result, "GetName should return the correct flag name")
}
func TestFlagDefinition_GetType(t *testing.T) {
	flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flagSet.String("test-flag", "default", "help")
	// Lookup the flag to get the pflag.Flag instance
	mockFlag := flagSet.Lookup("test-flag")
	if mockFlag == nil {
		t.Fatalf("Failed to create mock flag")
	}
	// Create a flagDefinition instance with the mock flag
	flagDef := flagDefinition{
		pflag: mockFlag,
	}
	// Call GetType and verify the result
	result := flagDef.GetType()
	assert.Equal(t, "string", result, "GetType should return the correct flag type")
}

func TestFlagDefinition_GetValueAsString(t *testing.T) {
	flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flagSet.String("test-flag", "default", "help")
	// Lookup the flag to get the pflag.Flag instance
	mockFlag := flagSet.Lookup("test-flag")
	if mockFlag == nil {
		t.Fatalf("Failed to create mock flag")
	}
	// Create a flagDefinition instance with the mock flag
	flagDef := flagDefinition{
		pflag: mockFlag,
	}
	// Call GetValueAsString and verify the result
	result := flagDef.GetValueAsString()
	assert.Equal(t, "default", result, "GetValueAsString should return the correct flag value as string")
}
