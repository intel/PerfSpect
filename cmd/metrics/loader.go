// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"fmt"
	"log/slog"
	"perfspect/internal/cpus"

	"github.com/casbin/govaluate"
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
}
type Loader interface {
	Load(config LoaderConfig) (metrics []MetricDefinition, groups []GroupDefinition, err error)
}

type BaseLoader struct {
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

// NewLoader creates the right type of Loader for each CPU microarchitecture
// Input is the CPU microarchitecture name as defined in the cpus module.
// If useLegacyLoader is true, the legacy loader will be used regardless of microarchitecture.
func NewLoader(uarch string, useLegacyLoader bool) (Loader, error) {
	if useLegacyLoader {
		slog.Debug("Using legacy loader due to override", slog.String("uarch", uarch))
		return newLegacyLoader(), nil
	}
	switch uarch {
	case cpus.UarchCLX, cpus.UarchSKX, cpus.UarchBDX, cpus.UarchBergamo, cpus.UarchGenoa, cpus.UarchTurinZen5, cpus.UarchTurinZen5c:
		slog.Debug("Using legacy loader for microarchitecture", slog.String("uarch", uarch))
		return newLegacyLoader(), nil
	case cpus.UarchGNR, cpus.UarchGNR_X1, cpus.UarchGNR_X2, cpus.UarchGNR_X3, cpus.UarchGNR_D, cpus.UarchSRF, cpus.UarchSRF_SP, cpus.UarchSRF_AP, cpus.UarchEMR, cpus.UarchEMR_MCC, cpus.UarchEMR_XCC, cpus.UarchSPR, cpus.UarchSPR_MCC, cpus.UarchSPR_XCC, cpus.UarchICX:
		slog.Debug("Using perfmon loader for microarchitecture", slog.String("uarch", uarch))
		return newPerfmonLoader(), nil
	case cpus.UarchGraviton2, cpus.UarchGraviton3, cpus.UarchGraviton4, cpus.UarchAxion, cpus.UarchAltraFamily, cpus.UarchAmpereOneAC03, cpus.UarchAmpereOneAC04, cpus.UarchAmpereOneAC04_1:
		slog.Debug("Using component loader for microarchitecture", slog.String("uarch", uarch))
		return newComponentLoader(), nil
	default:
		return nil, fmt.Errorf("unsupported microarchitecture: %s", uarch)
	}
}

func newLegacyLoader() *LegacyLoader {
	return &LegacyLoader{
		BaseLoader: BaseLoader{},
	}
}

func newPerfmonLoader() *PerfmonLoader {
	return &PerfmonLoader{
		BaseLoader: BaseLoader{},
	}
}

func newComponentLoader() *ComponentLoader {
	return &ComponentLoader{
		BaseLoader: BaseLoader{},
	}
}
