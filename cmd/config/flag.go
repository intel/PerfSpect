package config

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"perfspect/internal/target"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type IntSetFunc func(int, target.Target, string) error
type FloatSetFunc func(float64, target.Target, string) error
type StringSetFunc func(string, target.Target, string) error
type BoolSetFunc func(bool, target.Target, string) error
type ValidationFunc func(cmd *cobra.Command) bool

// flagDefinition is a struct that defines a command line flag.
type flagDefinition struct {
	pflag                 *pflag.Flag
	intSetFunc            IntSetFunc
	floatSetFunc          FloatSetFunc
	stringSetFunc         StringSetFunc
	boolSetFunc           BoolSetFunc
	validationFunc        ValidationFunc
	validationDescription string
}

// HasSetFunc checks if any set function is defined for the flag.
func (f *flagDefinition) HasSetFunc() bool {
	return f.intSetFunc != nil || f.floatSetFunc != nil || f.stringSetFunc != nil || f.boolSetFunc != nil
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

// newIntFlag creates a new int flag and adds it to the command.
func newIntFlag(cmd *cobra.Command, name string, defaultValue int, setFunc IntSetFunc, help string, validationDescription string, validationFunc ValidationFunc) flagDefinition {
	cmd.Flags().Int(name, defaultValue, help)
	pFlag := cmd.Flags().Lookup(name)
	return flagDefinition{
		pflag:                 pFlag,
		intSetFunc:            setFunc,
		validationFunc:        validationFunc,
		validationDescription: validationDescription,
	}
}

// newFloat64Flag creates a new float64 flag and adds it to the command.
func newFloat64Flag(cmd *cobra.Command, name string, defaultValue float64, setFunc FloatSetFunc, help string, validationDescription string, validationFunc ValidationFunc) flagDefinition {
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
func newStringFlag(cmd *cobra.Command, name string, defaultValue string, setFunc StringSetFunc, help string, validationDescription string, validationFunc ValidationFunc) flagDefinition {
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
func newBoolFlag(cmd *cobra.Command, name string, defaultValue bool, setFunc BoolSetFunc, help string, validationDescription string, validationFunc ValidationFunc) flagDefinition {
	cmd.Flags().Bool(name, defaultValue, help)
	pFlag := cmd.Flags().Lookup(name)
	return flagDefinition{
		pflag:                 pFlag,
		boolSetFunc:           setFunc,
		validationFunc:        validationFunc,
		validationDescription: validationDescription,
	}
}
