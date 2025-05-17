package config

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"perfspect/internal/target"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// setOutput is a struct that holds the output of a flagDefinition set function
type setOutput struct {
	goRoutineID int
	err         error
}

// flagDefinition is a struct that defines a command line flag.
type flagDefinition struct {
	pflag                 *pflag.Flag
	uintSetFunc           func(uint, target.Target, string, chan setOutput, int)
	intSetFunc            func(int, target.Target, string, chan setOutput, int)
	floatSetFunc          func(float64, target.Target, string, chan setOutput, int)
	stringSetFunc         func(string, target.Target, string, chan setOutput, int)
	boolSetFunc           func(bool, target.Target, string, chan setOutput, int)
	validationFunc        func(cmd *cobra.Command) bool
	validationDescription string
}

// HasSetFunc checks if any set function is defined for the flag.
func (f *flagDefinition) HasSetFunc() bool {
	return f.uintSetFunc != nil || f.intSetFunc != nil || f.floatSetFunc != nil || f.stringSetFunc != nil || f.boolSetFunc != nil
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

// newUintFlag creates a new uint flag and adds it to the command.
func newUintFlag(cmd *cobra.Command, name string, defaultValue uint, setFunc func(uint, target.Target, string, chan setOutput, int), help string, validationDescription string, validationFunc func(cmd *cobra.Command) bool) flagDefinition {
	cmd.Flags().Uint(name, defaultValue, help)
	pFlag := cmd.Flags().Lookup(name)
	return flagDefinition{
		pflag:                 pFlag,
		uintSetFunc:           setFunc,
		validationFunc:        validationFunc,
		validationDescription: validationDescription,
	}
}

// newFloat64Flag creates a new float64 flag and adds it to the command.
func newFloat64Flag(cmd *cobra.Command, name string, defaultValue float64, setFunc func(float64, target.Target, string, chan setOutput, int), help string, validationDescription string, validationFunc func(cmd *cobra.Command) bool) flagDefinition {
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
func newStringFlag(cmd *cobra.Command, name string, defaultValue string, setFunc func(string, target.Target, string, chan setOutput, int), help string, validationDescription string, validationFunc func(cmd *cobra.Command) bool) flagDefinition {
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
func newBoolFlag(cmd *cobra.Command, name string, defaultValue bool, setFunc func(bool, target.Target, string, chan setOutput, int), help string, validationDescription string, validationFunc func(cmd *cobra.Command) bool) flagDefinition {
	cmd.Flags().Bool(name, defaultValue, help)
	pFlag := cmd.Flags().Lookup(name)
	return flagDefinition{
		pflag:                 pFlag,
		boolSetFunc:           setFunc,
		validationFunc:        validationFunc,
		validationDescription: validationDescription,
	}
}
