package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/Knetic/govaluate"
)

// MetricDefinition is the common (across loader implementations) representation of a single metric
type MetricDefinition struct {
	Name                string
	LegacyName          string
	Expression          string
	Description         string
	Category            string
	Level               int
	ThresholdExpression string
	// Evaluation fields - used during metric expression evaluation
	//
	// Variables - map of variable names found in Expression to the indices of the event
	// group from which the value will be taken from for metric evaluation. These indices
	// are set the first time the metric is evaluated.
	Variables map[string]int
	// Evaluable - parsed expression from govaluate. These are set once when the metric
	// definitions are loaded and parsed, so that the expression does not need to be
	// parsed each time the metric is evaluated.
	Evaluable *govaluate.EvaluableExpression
	// ThresholdVariables - list of variable names found in ThresholdExpression.
	ThresholdVariables []string
	// ThresholdEvaluable - parsed threshold expression from govaluate. These are set once when the metric
	// definitions are loaded and parsed, so that the expression does not need to be
	// parsed each time the metric is evaluated.
	ThresholdEvaluable *govaluate.EvaluableExpression
}

// EventDefinition is the common (across loader implementations) representation of a single perf event
type EventDefinition struct {
	Raw    string // the event string in perf format
	Name   string // the event name
	Device string // the event device (e.g., "cpu" from cpu/event=0x3c/,umask=...), currently used by legacy loader only
}

// GroupDefinition represents a group of perf events
type GroupDefinition []EventDefinition

// LoaderConfig encapsulates all configuration options for loaders
type LoaderConfig struct {
	// Common configuration
	SelectedMetrics []string
	Metadata        Metadata

	// Override configurations
	MetricDefinitionOverride string // For direct metric file override (legacy loader)
	EventDefinitionOverride  string // For direct event file override  (legacy loader)
	ConfigFileOverride       string // For config file that points to multiple files (perfmon loader)
}
type Loader interface {
	Load(config LoaderConfig) (metrics []MetricDefinition, groups []GroupDefinition, err error)
}

type BaseLoader struct {
	microarchitecture string
}

type LegacyLoader struct {
	BaseLoader
}

type PerfmonLoader struct {
	BaseLoader
}

type ComponentLoader struct {
	BaseLoader
}

func NewLoader(uarch string) (Loader, error) {
	switch strings.ToLower(uarch) {
	case "clx", "skx", "bdx", "bergamo", "genoa", "turin":
		slog.Debug("Using legacy loader for microarchitecture", slog.String("uarch", uarch))
		return newLegacyLoader(strings.ToLower(uarch)), nil
	case "gnr", "srf", "emr", "spr", "icx":
		slog.Debug("Using perfmon loader for microarchitecture", slog.String("uarch", uarch))
		return newPerfmonLoader(strings.ToLower(uarch)), nil
	case "neoverse-n2", "neoverse-v2", "neoverse-n1", "neoverse-v1":
		slog.Debug("Using component loader for microarchitecture", slog.String("uarch", uarch))
		return newComponentLoader(strings.ToLower(uarch)), nil
	default:
		return nil, fmt.Errorf("unsupported microarchitecture: %s", uarch)
	}
}

func newLegacyLoader(uarch string) *LegacyLoader {
	return &LegacyLoader{
		BaseLoader: BaseLoader{
			microarchitecture: uarch,
		},
	}
}

func newPerfmonLoader(uarch string) *PerfmonLoader {
	return &PerfmonLoader{
		BaseLoader: BaseLoader{
			microarchitecture: uarch,
		},
	}
}

func newComponentLoader(uarch string) *ComponentLoader {
	return &ComponentLoader{
		BaseLoader: BaseLoader{
			microarchitecture: uarch,
		},
	}
}
