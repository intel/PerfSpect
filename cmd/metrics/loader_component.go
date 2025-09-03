package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Knetic/govaluate"
)

func (l *ComponentLoader) Load(loaderConfig LoaderConfig) ([]MetricDefinition, []GroupDefinition, error) {
	metricDefinitions, err := l.loadMetricDefinitions(loaderConfig.MetricDefinitionOverride, loaderConfig.SelectedMetrics, loaderConfig.Metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load metric definitions: %w", err)
	}
	eventDefinitions, err := l.loadEventDefinitions(loaderConfig.Metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load event definitions: %w", err)
	}
	metricDefinitions, err = l.filterUncollectableMetrics(metricDefinitions, eventDefinitions, loaderConfig.Metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to filter uncollectable metrics: %w", err)
	}
	groupDefinitions, err := l.formEventGroups(metricDefinitions, eventDefinitions)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to form event groups: %w", err)
	}
	return metricDefinitions, groupDefinitions, nil
}

type ComponentMetric struct {
	ArchStdEvent     string `json:"ArchStdEvent"`
	MetricName       string `json:"MetricName"`
	MetricExpr       string `json:"MetricExpr"`
	BriefDescription string `json:"BriefDescription"`
	MetricGroup      string `json:"MetricGroup"`
	ScaleUnit        string `json:"ScaleUnit"`
}

func (cm *ComponentMetric) getName() string {
	if cm.ArchStdEvent != "" {
		return cm.ArchStdEvent
	}
	return cm.MetricName
}
func (cm *ComponentMetric) getLegacyName() string {
	return cm.getName()
}

type ComponentEvent struct {
	ArchStdEvent      string `json:"ArchStdEvent"`
	PublicDescription string `json:"PublicDescription"`
}

func (l *ComponentLoader) loadMetricDefinitions(metricDefinitionOverridePath string, selectedMetrics []string, metadata Metadata) (metrics []MetricDefinition, err error) {
	var bytes []byte
	if metricDefinitionOverridePath != "" {
		bytes, err = os.ReadFile(metricDefinitionOverridePath) // #nosec G304
		if err != nil {
			return
		}
	} else {
		var archDir string
		archDir, err = getArchDir(metadata.Microarchitecture)
		if err != nil {
			return nil, err
		}
		if bytes, err = resources.ReadFile(filepath.Join("resources", "component", archDir, "metrics.json")); err != nil {
			return
		}
	}
	var componentMetricsInFile []ComponentMetric
	if err = json.Unmarshal(bytes, &componentMetricsInFile); err != nil {
		return
	}
	var allMetricNames []string
	for i := range componentMetricsInFile {
		allMetricNames = append(allMetricNames, componentMetricsInFile[i].getName())
	}
	for i := range componentMetricsInFile {
		if len(selectedMetrics) == 0 || slices.Contains(allMetricNames, componentMetricsInFile[i].getName()) {
			var m MetricDefinition
			m.Name = componentMetricsInFile[i].getName()
			m.LegacyName = componentMetricsInFile[i].getLegacyName()
			m.Expression = componentMetricsInFile[i].MetricExpr
			m.Description = componentMetricsInFile[i].BriefDescription
			m.Category = componentMetricsInFile[i].MetricGroup
			m.Variables = initializeComponentMetricVariables(m.Expression)
			m.Evaluable = initializeComponentMetricEvaluable(m.Expression, m.Variables)
			metrics = append(metrics, m)
		}
	}
	return metrics, nil
}

// loadEventDefinitions -- load all event files in resources/component/<arch>/
// skip metrics.json
func (l *ComponentLoader) loadEventDefinitions(metadata Metadata) (events []ComponentEvent, err error) {
	var archDir string
	archDir, err = getArchDir(metadata.Microarchitecture)
	if err != nil {
		return nil, err
	}
	resourcesDir := filepath.Join("resources", "component", archDir)
	dirEntries, err := resources.ReadDir(resourcesDir)
	if err != nil {
		return
	}
	for _, entry := range dirEntries {
		if entry.IsDir() || entry.Name() == "metrics.json" {
			continue
		}
		var fileBytes []byte
		fileBytes, err = resources.ReadFile(filepath.Join(resourcesDir, entry.Name()))
		if err != nil {
			return
		}
		var fileEvents []ComponentEvent
		if err = json.Unmarshal(fileBytes, &fileEvents); err != nil {
			return
		}
		events = append(events, fileEvents...)
	}
	return events, nil
}

func (l *ComponentLoader) filterUncollectableMetrics(metrics []MetricDefinition, events []ComponentEvent, metadata Metadata) (filteredMetrics []MetricDefinition, err error) {
	uncollectableEvents, err := l.identifyUncollectableEvents(events, metadata)
	if err != nil {
		return
	}
	for _, metric := range metrics {
		for variable := range metric.Variables {
			if slices.Contains(uncollectableEvents, variable) {
				slog.Info("Excluding metric due to uncollectable event", slog.String("metric", metric.Name), slog.String("event", variable))
				goto nextMetric
			}
		}
		filteredMetrics = append(filteredMetrics, metric)
	nextMetric:
	}
	return filteredMetrics, nil
}

func (l *ComponentLoader) identifyUncollectableEvents(events []ComponentEvent, metadata Metadata) (uncollectableEvents []string, err error) {
	// For now, assume all events are collectable
	// In the future, we may want to check for specific events that are not collectable on certain platforms
	return uncollectableEvents, nil
}

func (l *ComponentLoader) formEventGroups(metrics []MetricDefinition, events []ComponentEvent) (groups []GroupDefinition, err error) {
	return groups, nil
}

func initializeComponentMetricVariables(expression string) map[string]int {
	// parse expression to find variables
	variables := make(map[string]int)

	// Define operators and special tokens to exclude
	operators := map[string]bool{
		"if":   true,
		"else": true,
	}

	// Split by common delimiters
	delimiters := []rune{
		' ', '+', '-', '*', '/', '(', ')', ',', ';', '^', '|', '=', '<', '>', '!', '?', ':',
	}
	tokens := strings.FieldsFunc(expression, func(r rune) bool {
		return slices.Contains(delimiters, r)
	})

	for _, token := range tokens {
		if token == "" {
			continue
		}

		// Skip tokens that are integer values or operators
		if isInteger(token) || operators[token] {
			continue
		}

		if _, exists := variables[token]; !exists {
			variables[token] = -1 // initialize to -1, will be set the first time the metric is evaluated
		}
	}
	return variables
}

// isInteger checks if a string is a valid integer
func isInteger(s string) bool {
	// Check if the string is a decimal integer
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func initializeComponentMetricEvaluable(expression string, variables map[string]int) *govaluate.EvaluableExpression {
	// re-form expression into govaluate format

	// create govaluate expression
	expr, err := govaluate.NewEvaluableExpression(expression)
	if err != nil {
		slog.Error("Failed to parse metric expression", slog.String("expression", expression), slog.String("error", err.Error()))
		return nil
	}
	return expr
}

func getArchDir(uarch string) (string, error) {
	if strings.ToLower(uarch) == "neoverse-n2" || strings.ToLower(uarch) == "neoverse-v2" {
		return "neoverse-n2-v2", nil
	} else {
		return "", fmt.Errorf("unsupported component loader architecture")
	}

}
