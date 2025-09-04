package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
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
	groupDefinitions, err := l.formEventGroups(metricDefinitions, eventDefinitions, loaderConfig.Metadata)
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
	evaluatorFunctions := getARMEvaluatorFunctions()

	for i := range componentMetricsInFile {
		if len(selectedMetrics) == 0 || slices.Contains(allMetricNames, componentMetricsInFile[i].getName()) {
			var m MetricDefinition
			m.Name = componentMetricsInFile[i].getName()
			m.LegacyName = componentMetricsInFile[i].getLegacyName()
			m.Expression = componentMetricsInFile[i].MetricExpr
			m.Description = componentMetricsInFile[i].BriefDescription
			m.Category = componentMetricsInFile[i].MetricGroup
			m.Variables = initializeComponentMetricVariables(m.Expression)
			m.Evaluable = initializeComponentMetricEvaluable(m.Expression, evaluatorFunctions, metadata)
			if m.Evaluable != nil {
				metrics = append(metrics, m)
			}
		}
	}
	return metrics, nil
}

// TODO:
func getARMEvaluatorFunctions() map[string]govaluate.ExpressionFunction {
	functions := make(map[string]govaluate.ExpressionFunction)
	functions["strcmp_cpuid_str"] = func(args ...any) (any, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("strcmp_cpuid_str requires one argument")
		}
		return true, nil // Placeholder: always return true for now
	}
	return functions
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
	// TODO:
	// For now, assume all events are collectable
	return uncollectableEvents, nil
}

func (l *ComponentLoader) formEventGroups(metrics []MetricDefinition, events []ComponentEvent, metadata Metadata) (groups []GroupDefinition, err error) {
	numGPCounters := metadata.NumGeneralPurposeCounters // groups can have at most this many events (plus fixed counters)
	eventNames := make(map[string]bool)
	for _, event := range events {
		eventNames[event.ArchStdEvent] = true
	}

	for _, metric := range metrics {
		var metricGroups []GroupDefinition
		if len(metric.Variables) == 0 {
			// no variables, skip it
			continue
		}
		// metric has variables. Each variable is an event that will be added to a group
		// only numGPCounters events can be added to a group, so if there are more variables than that,
		// multiple groups will be created for the metric
		// CPU_CYCLES is a fixed counter, so it does not count against the numGPCounters limit
		var currentGroup GroupDefinition
		var currentGPCount int // Track the current number of GP counters used

		for variable := range metric.Variables {
			// confirm variable is a valid event
			if _, exists := eventNames[variable]; !exists {
				slog.Error("Metric variable does not correspond to a known event, skipping variable", slog.String("metric", metric.Name), slog.String("variable", variable))
				continue
			}
			// Add the event to the current group
			currentGroup = append(currentGroup, EventDefinition{Name: variable, Raw: variable, Device: "cpu"})

			// Only increment the GP counter count if this isn't a fixed counter
			if variable != "CPU_CYCLES" {
				currentGPCount++
			}

			// If we've reached the max number of GP counters, finalize this group
			if currentGPCount >= numGPCounters {
				metricGroups = append(metricGroups, currentGroup)
				currentGroup = nil
				currentGPCount = 0
			}
		}
		if len(currentGroup) > 0 {
			metricGroups = append(metricGroups, currentGroup)
		}
		// add metricGroups to groups to be returned
		groups = append(groups, metricGroups...)
	}

	// eliminate duplicate and overlapping groups
	groups = deduplicateGroups(groups)

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

	constants := map[string]bool{
		"#slots": true,
	}

	functions := map[string]bool{
		"strcmp_cpuid_str": true,
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

		// Skip some tokens
		if isInteger(token) || isHex(token) || operators[token] || constants[token] || functions[token] {
			continue
		}

		if _, exists := variables[token]; !exists {
			variables[token] = -1 // initialize to -1, will be set the first time the metric is evaluated
		}
	}
	return variables
}

func isHex(s string) bool {
	// Check if the string starts with "0x" or "0X"
	if len(s) < 3 || !(strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X")) {
		return false
	}
	// Check if the rest of the string is a valid hexadecimal number
	for _, c := range s[2:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
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

func initializeComponentMetricEvaluable(expression string, evaluatorFunctions map[string]govaluate.ExpressionFunction, metadata Metadata) *govaluate.EvaluableExpression {
	// replace #slots with metadata.ARMSlots
	expression = strings.ReplaceAll(expression, "#slots", fmt.Sprintf("%d", metadata.ARMSlots))
	// replace if else with ?:
	expression, err := transformExpression(expression)
	if err != nil {
		slog.Error("Failed to transform expression", slog.String("expression", expression), slog.String("error", err.Error()))
		return nil
	}
	// govaluate doesn't like: strcmp_cpuid_str(0x410fd493)
	// "Cannot transition token types from NUMERIC [0] to VARIABLE [x410fd493]"
	// quote the hex number
	// look for (0x....) and replace with ("0x....")
	rxHex := regexp.MustCompile(`\((0x[0-9a-fA-F]+)\)`)
	expression = rxHex.ReplaceAllStringFunc(expression, func(s string) string {
		return "(" + `"` + rxTrailingChars.ReplaceAllString(s, "") + `"` + ")"
	})

	// create govaluate expression
	expr, err := govaluate.NewEvaluableExpressionWithFunctions(expression, evaluatorFunctions)
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

// deduplicateGroups eliminates duplicate and overlapping groups
// Two groups are considered duplicates if they contain exactly the same events (regardless of order)
// A group is considered overlapping with another if one is a subset of the other
func deduplicateGroups(groups []GroupDefinition) []GroupDefinition {
	if len(groups) <= 1 {
		return groups
	}

	// Create a map to detect duplicates
	// The key is a sorted string representation of event names in the group
	seen := make(map[string]int) // Map from event set signature to index in dedupGroups
	var dedupGroups []GroupDefinition

	// For each group
	for _, group := range groups {
		// Create a sorted signature of the events in this group
		var eventNames []string
		for _, event := range group {
			eventNames = append(eventNames, event.Name)
		}
		slices.Sort(eventNames)
		signature := strings.Join(eventNames, ",")

		// If we've seen this exact group before, skip it
		if _, exists := seen[signature]; exists {
			continue
		}

		// Check for overlapping groups (is this group a subset of an existing group?)
		isSubset := false
		for i, existingGroup := range dedupGroups {
			// Create a set of events from the existing group
			existingEvents := make(map[string]bool)
			for _, event := range existingGroup {
				existingEvents[event.Name] = true
			}

			// Check if current group is a subset of the existing group
			allEventsInExisting := true
			for _, event := range group {
				if !existingEvents[event.Name] {
					allEventsInExisting = false
					break
				}
			}

			if allEventsInExisting && len(group) < len(existingGroup) {
				isSubset = true
				break
			}

			// Check if existing group is a subset of current group
			if len(existingGroup) < len(group) {
				allEventsInCurrent := true
				currentEvents := make(map[string]bool)
				for _, event := range group {
					currentEvents[event.Name] = true
				}

				for _, event := range existingGroup {
					if !currentEvents[event.Name] {
						allEventsInCurrent = false
						break
					}
				}

				// If existing group is a subset of current group, replace it
				if allEventsInCurrent {
					dedupGroups[i] = group
					seen[signature] = i
					isSubset = true
					break
				}
			}
		}

		// If this group is not a subset and doesn't contain a subset, add it
		if !isSubset {
			seen[signature] = len(dedupGroups)
			dedupGroups = append(dedupGroups, group)
		}
	}

	return dedupGroups
}
