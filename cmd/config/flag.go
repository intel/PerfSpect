package config

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"perfspect/internal/target"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagDefinition is a struct that defines a command line flag.
type flagDefinition struct {
	pflag                 *pflag.Flag
	intSetFunc            func(int, target.Target, string) error
	floatSetFunc          func(float64, target.Target, string) error
	stringSetFunc         func(string, target.Target, string) error
	boolSetFunc           func(bool, target.Target, string) error
	validationFunc        func(cmd *cobra.Command) bool
	validationDescription string
}

// GetName returns the name of the flag.
func (f *flagDefinition) GetName() string {
	return f.pflag.Name
}

// GetType returns the type of the flag.
func (f *flagDefinition) GetType() string {
	return f.pflag.Value.Type()
}

// GetValueAsString returns the value of the flag as a string.
func (f *flagDefinition) GetValueAsString() string {
	return f.pflag.Value.String()
}

// newIntFlag creates a new integer flag and adds it to the command.
func newIntFlag(cmd *cobra.Command, name string, defaultValue int, setFunc func(int, target.Target, string) error, help string, validationDescription string, validationFunc func(cmd *cobra.Command) bool) flagDefinition {
	cmd.Flags().Int(name, defaultValue, help)
	pFlag := cmd.Flags().Lookup(name)
	return flagDefinition{
		pflag:                 pFlag,
		intSetFunc:            setFunc,
		validationFunc:        validationFunc,
		validationDescription: validationDescription,
	}
}

// newInt64Flag creates a new int64 flag and adds it to the command.
func newFloat64Flag(cmd *cobra.Command, name string, defaultValue float64, setFunc func(float64, target.Target, string) error, help string, validationDescription string, validationFunc func(cmd *cobra.Command) bool) flagDefinition {
	cmd.Flags().Float64(name, defaultValue, help)
	pFlag := cmd.Flags().Lookup(name)
	return flagDefinition{
		pflag:                 pFlag,
		floatSetFunc:          setFunc,
		validationFunc:        validationFunc,
		validationDescription: validationDescription,
	}
}

// newStringFlag creates a new string flag and adds it to the command.
func newStringFlag(cmd *cobra.Command, name string, defaultValue string, setFunc func(string, target.Target, string) error, help string, validationDescription string, validationFunc func(cmd *cobra.Command) bool) flagDefinition {
	cmd.Flags().String(name, defaultValue, help)
	pFlag := cmd.Flags().Lookup(name)
	return flagDefinition{
		pflag:                 pFlag,
		stringSetFunc:         setFunc,
		validationFunc:        validationFunc,
		validationDescription: validationDescription,
	}
}

// newBoolFlag creates a new boolean flag and adds it to the command.
func newBoolFlag(cmd *cobra.Command, name string, defaultValue bool, setFunc func(bool, target.Target, string) error, help string, validationDescription string, validationFunc func(cmd *cobra.Command) bool) flagDefinition {
	cmd.Flags().Bool(name, defaultValue, help)
	pFlag := cmd.Flags().Lookup(name)
	return flagDefinition{
		pflag:                 pFlag,
		boolSetFunc:           setFunc,
		validationFunc:        validationFunc,
		validationDescription: validationDescription,
	}
}
