// Package app defines application-wide types, constants, and context
// that are shared across multiple commands.
package app

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"os"
	"path/filepath"
	"perfspect/internal/script"
	"perfspect/internal/table"
)

// Name is the name of the application executable.
var Name = filepath.Base(os.Args[0])

// Context represents the application context that can be accessed from all commands.
type Context struct {
	Timestamp      string // Timestamp is the timestamp when the application was started.
	OutputDir      string // OutputDir is the directory where the application will write output files.
	LocalTempDir   string // LocalTempDir is the temp directory on the local host (created by the application).
	LogFilePath    string // LogFilePath is the path to the log file.
	TargetTempRoot string // TargetTempRoot is the path to a directory on the target host where the application can create temporary directories.
	Version        string // Version is the version of the application.
	Debug          bool   // Debug is true if the application is running in debug mode.
}

// Table name constants used across multiple commands.
const (
	TableNameInsights  = "Insights"
	TableNamePerfspect = "PerfSpect"
)

// Flag names for input and format flags used by reporting commands.
const (
	FlagInputName  = "input"
	FlagFormatName = "format"
)

// Global flag variables for reporting commands.
var (
	FlagInput  string
	FlagFormat []string
)

// Category represents a configuration category with associated tables and flags.
type Category struct {
	FlagName     string
	Tables       []table.TableDefinition
	FlagVar      *bool
	DefaultValue bool
	Help         string
}

// Flag names for flags defined in the root command, but sometimes used in other commands.
const (
	FlagDebugName          = "debug"
	FlagSyslogName         = "syslog"
	FlagLogStdOutName      = "log-stdout"
	FlagOutputDirName      = "output"
	FlagTargetTempRootName = "tempdir"
	FlagNoCheckUpdateName  = "noupdate"
)

// Flag represents a command-line flag with its name and help text.
type Flag struct {
	Name string
	Help string
}

// FlagGroup represents a group of related flags with a group name.
type FlagGroup struct {
	GroupName string
	Flags     []Flag
}

// SummaryFunc is a function type for generating summary table values from processed tables.
type SummaryFunc func([]table.TableValues, map[string]script.ScriptOutput) table.TableValues

// InsightsFunc is a function type for generating insights from processed tables.
type InsightsFunc SummaryFunc
