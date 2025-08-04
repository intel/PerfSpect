package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/Knetic/govaluate"
)

type MetricDefinition struct {
	Name        string                         `json:"name"`
	Expression  string                         `json:"expression"`
	Description string                         `json:"description"`
	Variables   map[string]int                 // parsed from Expression for efficiency, int represents group index
	Evaluable   *govaluate.EvaluableExpression // parse expression once, store here for use in metric evaluation
}

// EventDefinition represents a single perf event
type EventDefinition struct {
	Raw    string
	Name   string
	Device string
}

// GroupDefinition represents a group of perf events
type GroupDefinition []EventDefinition

type Loader interface {
	Load(metricDefinitionOverridePath string, eventDefinitionOverridePath string, selectedMetrics []string, metadata Metadata) (metrics []MetricDefinition, groups []GroupDefinition, err error)
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

func NewLoader(uarch string) (Loader, error) {
	switch strings.ToLower(uarch) {
	case "srf", "clx", "skx", "bdx", "bergamo", "genoa", "turin":
		slog.Debug("Using legacy loader for microarchitecture", slog.String("uarch", uarch))
		return newLegacyLoader(strings.ToLower(uarch)), nil
	case "gnr", "emr", "spr", "icx":
		slog.Debug("Using perfmon loader for microarchitecture", slog.String("uarch", uarch))
		return newPerfmonLoader(strings.ToLower(uarch)), nil
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
