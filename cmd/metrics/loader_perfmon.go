package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"perfspect/internal/util"
	"regexp"
	"slices"
	"strings"
)

type PerfmonMetricsHeader map[string]string

type PerfmonMetricThreshold struct {
	ThresholdMetrics []map[string]string `json:"ThresholdMetrics"`
	Formula          string              `json:"Formula"`
	BaseFormula      string              `json:"BaseFormula"`
	ThresholdIssues  string              `json:"ThresholdIssues"`
}

type PerfmonMetric struct {
	MetricName       string                  `json:"MetricName"`
	LegacyName       string                  `json:"LegacyName"`
	ParentCategory   string                  `json:"ParentCategory"`
	Level            int                     `json:"Level"`
	BriefDescription string                  `json:"BriefDescription"`
	UnitOfMeasure    string                  `json:"UnitOfMeasure"`
	Events           []map[string]string     `json:"Events"`
	Constants        []map[string]string     `json:"Constants"`
	Formula          string                  `json:"Formula"`
	BaseFormula      string                  `json:"BaseFormula"`
	Category         string                  `json:"Category"`
	CountDomain      string                  `json:"CountDomain"`
	Threshold        *PerfmonMetricThreshold `json:"Threshold"`
	ResolutionLevels string                  `json:"ResolutionLevels"`
	MetricGroup      string                  `json:"MetricGroup"`
	LocateWith       string                  `json:"LocateWith"`
}

type PerfmonMetrics struct {
	Header  PerfmonMetricsHeader `json:"Header"`
	Metrics []PerfmonMetric      `json:"Metrics"`
}

func LoadPerfmonMetrics(path string) (PerfmonMetrics, error) {
	var metrics PerfmonMetrics
	bytes, err := resources.ReadFile(path)
	if err != nil {
		return PerfmonMetrics{}, fmt.Errorf("error reading file %s: %w", path, err)
	}
	if err := json.Unmarshal(bytes, &metrics); err != nil {
		return PerfmonMetrics{}, fmt.Errorf("error unmarshaling JSON from %s: %w", path, err)
	}
	return metrics, nil
}

type MetricsConfigHeader struct {
	Copyright string `json:"Copyright"`
	Info      string `json:"Info"`
}
type PerfspectMetric struct {
	MetricName string `json:"MetricName"`
	LegacyName string `json:"LegacyName"`
	Origin     string `json:"Origin"`
}
type MetricsConfig struct {
	Header                   MetricsConfigHeader `json:"Header"`
	PerfmonMetricsFile       string              `json:"PerfmonMetricsFile"`       // Path to the perfmon metrics file
	PerfmonCoreEventsFile    string              `json:"PerfmonCoreEventsFile"`    // Path to the perfmon core events file
	PerfmonUncoreEventsFile  string              `json:"PerfmonUncoreEventsFile"`  // Path to the perfmon uncore events file
	PerfmonRetireLatencyFile string              `json:"PerfmonRetireLatencyFile"` // Path to the perfmon retire latency file
	Metrics                  []PerfmonMetric     `json:"Metrics"`                  // Metrics defined by PerfSpect
	AlternateTMAMetrics      []PerfmonMetric     `json:"AlternateTMAMetrics"`      // Alternate TMA metrics that can be used in place of the main TMA metrics
	ReportMetrics            []PerfspectMetric   `json:"ReportMetrics"`            // Metrics that are reported in the PerfSpect report
}

func (l *PerfmonLoader) loadMetricsConfig(metricConfigOverridePath string, metadata Metadata) (MetricsConfig, error) {
	var config MetricsConfig
	var bytes []byte
	if metricConfigOverridePath != "" {
		var err error
		bytes, err = os.ReadFile(metricConfigOverridePath)
		if err != nil {
			return MetricsConfig{}, fmt.Errorf("error reading metric config override file: %w", err)
		}
	} else {
		var err error
		bytes, err = resources.ReadFile(filepath.Join("resources", "metrics", metadata.Architecture, metadata.Vendor, strings.ToLower(l.microarchitecture), strings.ToLower(l.microarchitecture)+".json"))
		if err != nil {
			return MetricsConfig{}, fmt.Errorf("error reading metrics config file: %w", err)
		}
	}
	if err := json.Unmarshal(bytes, &config); err != nil {
		return MetricsConfig{}, fmt.Errorf("error unmarshaling metrics config JSON: %w", err)
	}
	return config, nil
}

func (l *PerfmonLoader) Load(metricConfigOverridePath string, _ string, selectedMetrics []string, metadata Metadata) ([]MetricDefinition, []GroupDefinition, error) {
	// Load the metrics configuration from the JSON file
	config, err := l.loadMetricsConfig(metricConfigOverridePath, metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load metrics config: %w", err)
	}
	// Load the perfmon metric definitions from the JSON file
	perfmonMetricDefinitions, err := LoadPerfmonMetrics(filepath.Join("resources", "metrics", metadata.Architecture, metadata.Vendor, strings.ToLower(l.microarchitecture), config.PerfmonMetricsFile))
	if err != nil {
		return nil, nil, fmt.Errorf("error loading perfmon metrics: %w", err)
	}
	// Load the perfmon core events from the JSON file
	coreEvents, err := NewCoreEvents(filepath.Join("resources", "metrics", metadata.Architecture, metadata.Vendor, strings.ToLower(l.microarchitecture), config.PerfmonCoreEventsFile))
	if err != nil {
		return nil, nil, fmt.Errorf("error loading perfmon core events: %w", err)
	}
	// Load the perfmon uncore events from the JSON file
	uncoreEvents, err := NewUncoreEvents(filepath.Join("resources", "metrics", metadata.Architecture, metadata.Vendor, strings.ToLower(l.microarchitecture), config.PerfmonUncoreEventsFile))
	if err != nil {
		return nil, nil, fmt.Errorf("error loading perfmon uncore events: %w", err)
	}
	// Load the other events (not core or uncore)
	otherEvents, err := NewOtherEvents()
	if err != nil {
		return nil, nil, fmt.Errorf("error loading other events: %w", err)
	}
	// remove metrics that use uncollectable events
	includedMetrics, err := removeUncollectableMetrics(config.ReportMetrics, perfmonMetricDefinitions.Metrics, config.Metrics, config.AlternateTMAMetrics, coreEvents, uncoreEvents, otherEvents, metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("error removing uncollectable metrics: %w", err)
	}
	// Filter the metrics based on the selected metrics and load the metrics definitions
	metrics, err := loadMetricsFromDefinitions(includedMetrics, perfmonMetricDefinitions.Metrics, config.Metrics, config.AlternateTMAMetrics, metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading metrics from definitions: %w", err)
	}
	// abbreviate uncore event names in metric expressions
	metrics, err = abbreviateUncoreEventNames(metrics, uncoreEvents)
	if err != nil {
		return nil, nil, fmt.Errorf("error abbreviating uncore event names: %w", err)
	}
	// simplify OCR event names in metric expressions
	metrics, err = customizeOCREventNames(metrics)
	if err != nil {
		return nil, nil, fmt.Errorf("error simplifying OCR event names: %w", err)
	}
	// Create event groups from the perfspect metrics
	coreGroups, uncoreGroups, otherGroups, uncollectableEvents, err := loadEventGroupsFromMetrics(
		includedMetrics,
		perfmonMetricDefinitions.Metrics,
		config.Metrics,
		config.AlternateTMAMetrics,
		coreEvents,
		uncoreEvents,
		otherEvents,
		metadata,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading event groups from metrics: %v", err)
	}
	fmt.Printf("Number of core groups: %d, uncore groups: %d, other groups: %d\n", len(coreGroups), len(uncoreGroups), len(otherGroups))
	// eliminate duplicate groups
	coreGroups, uncoreGroups, err = eliminateDuplicateGroups(coreGroups, uncoreGroups)
	if err != nil {
		return nil, nil, fmt.Errorf("error merging duplicate groups: %v", err)
	}
	fmt.Printf("Number of core groups after eliminating duplicates: %d, uncore groups: %d\n", len(coreGroups), len(uncoreGroups))
	// merge groups that can be merged, i.e., if 2nd group's events fit in the first group
	coreGroups, uncoreGroups, err = mergeGroups(coreGroups, uncoreGroups, metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("error merging groups: %v", err)
	}
	fmt.Printf("Number of core groups after merging: %d, uncore groups: %d\n", len(coreGroups), len(uncoreGroups))
	// expand uncore groups for uncore devices
	uncoreGroups, err = ExpandUncoreGroups(uncoreGroups, metadata.UncoreDeviceIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("error expanding uncore groups: %v", err)
	}
	fmt.Printf("Number of uncore groups after expanding for uncore devices: %d\n", len(uncoreGroups))
	// Merge all groups into a single slice of GroupDefinition
	allGroups := make([]GroupDefinition, 0)
	for _, group := range coreGroups {
		allGroups = append(allGroups, group.ToGroupDefinition())
	}
	for _, group := range uncoreGroups {
		allGroups = append(allGroups, group.ToGroupDefinition())
	}
	for _, group := range otherGroups {
		allGroups = append(allGroups, group.ToGroupDefinition())
	}
	// Print the groups for debugging
	for i, group := range coreGroups {
		fmt.Printf("Core Group %d:\n", i)
		group.Print(os.Stdout)
	}
	for i, group := range uncoreGroups {
		fmt.Printf("Uncore Group %d:\n", i)
		group.Print(os.Stdout)
	}
	for i, group := range otherGroups {
		fmt.Printf("Other Group %d:\n", i)
		group.Print(os.Stdout)
	}

	// replace retire latencies variables with their values
	if config.PerfmonRetireLatencyFile != "" {
		metrics, err = replaceRetireLatencies(metrics, filepath.Join("resources", "metrics", metadata.Architecture, metadata.Vendor, strings.ToLower(l.microarchitecture), config.PerfmonRetireLatencyFile))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to replace retire latencies: %w", err)
		}
	}

	// apply common modifications to metric expressions
	metrics, err = configureMetrics(metrics, uncollectableEvents, metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to configure metrics: %w", err)
	}
	return metrics, allGroups, nil
}

func abbreviateUncoreEventNames(metrics []MetricDefinition, uncoreEvents UncoreEvents) ([]MetricDefinition, error) {
	for i := range metrics {
		metric := &metrics[i]
		for _, uncoreEvent := range uncoreEvents.Events {
			re, err := regexp.Compile(fmt.Sprintf(`\b%s\b`, uncoreEvent.EventName))
			if err != nil {
				return nil, fmt.Errorf("failed to compile regex for uncore event %s: %w", uncoreEvent.EventName, err)
			}
			for {
				index := re.FindStringIndex(metric.Expression)
				if index == nil {
					break // no more matches found
				}
				// replace this occurrence of the original with the replacement
				metric.Expression = metric.Expression[:index[0]] + uncoreEvent.UniqueID + metric.Expression[index[1]:]
			}
		}
	}
	return metrics, nil
}

func customizeOCREventNames(metrics []MetricDefinition) ([]MetricDefinition, error) {
	for i := range metrics {
		metric := &metrics[i]
		// example portion of expression: [OCR.DEMAND_RFO.L3_MISS:ocr_msr_val=0x103b8000]
		if !strings.Contains(metric.Expression, ":ocr_msr_val=") {
			continue // only customize OCR events with this format
		}
		re, err := regexp.Compile(`(OCR\.[^\]]+):ocr_msr_val=([0-9a-fx]+)`)
		if err != nil {
			return nil, fmt.Errorf("failed to compile regex for OCR event: %w", err)
		}
		for {
			index := re.FindStringSubmatchIndex(metric.Expression)
			if index == nil {
				break // no more matches found
			}
			// extract the event name and MSR value from the match
			eventName := metric.Expression[index[2]:index[3]]
			msrValue := metric.Expression[index[4]:index[5]]
			// replace the OCR event with its customized name
			customizedName := fmt.Sprintf("%s.%s", eventName, msrValue)
			metric.Expression = metric.Expression[:index[0]] + customizedName + metric.Expression[index[1]:]
		}
	}
	return metrics, nil
}

// getExpression retrieves the expression for a given PerfmonMetric, replacing variables with their corresponding event or constant names.
// example formula: "( 1000000000 * (a / b) / (c / (d * socket_count) ) ) * DURATIONTIMEINSECONDS"
// desired output: "( 1000000000 * ([event1] / [event2]) / ([constant1] / ([constant2] * socket_count) ) ) * 1"
func getExpression(perfmonMetric PerfmonMetric) (string, error) {
	expression := perfmonMetric.Formula
	replacers := make(map[string]string)
	for _, event := range perfmonMetric.Events {
		replacers[event["Alias"]] = fmt.Sprintf("[%s]", event["Name"])
	}
	for _, constant := range perfmonMetric.Constants {
		replacers[constant["Alias"]] = fmt.Sprintf("[%s]", constant["Name"])
	}
	for alias, replacement := range replacers {
		// regex to match alias as a whole word
		// this prevents replacing substrings that are part of other words
		re, err := regexp.Compile(fmt.Sprintf(`\b%s\b`, alias))
		if err != nil {
			return "", fmt.Errorf("failed to compile regex for alias %s: %w", alias, err)
		}
		for {
			index := re.FindStringIndex(expression)
			if index == nil {
				break // no more matches found
			}
			// replace the first occurrence of the alias with the replacement
			expression = expression[:index[0]] + replacement + expression[index[1]:]
		}
	}
	// replace common constants with their values
	commonEventReplacements := map[string]string{
		"DURATIONTIMEINSECONDS":        "1",
		"[DURATIONTIMEINMILLISECONDS]": "1000",
	}
	for commonEvent, alias := range commonEventReplacements {
		expression = strings.ReplaceAll(expression, commonEvent, alias)
	}
	// replace fixed counter perfmon event names with their corresponding perf event names
	for perfmonEventName, perfEventName := range fixedCounterEventNameTranslation {
		// regex to match event name as a whole word
		// this prevents replacing substrings that are part of other words
		re, err := regexp.Compile(fmt.Sprintf(`\b%s\b`, perfmonEventName))
		if err != nil {
			return "", fmt.Errorf("failed to compile regex for perfmonEventName %s: %w", perfmonEventName, err)
		}
		for {
			index := re.FindStringIndex(expression)
			if index == nil {
				break // no more matches found
			}
			// replace the first occurrence of the alias with the replacement
			expression = expression[:index[0]] + perfEventName + expression[index[1]:]
		}
	}
	return expression, nil
}

func loadMetricsFromDefinitions(includedMetrics []PerfspectMetric, perfmonMetrics []PerfmonMetric, perfspectMetrics []PerfmonMetric, alternateTMAMetrics []PerfmonMetric, metadata Metadata) ([]MetricDefinition, error) {
	var metrics []MetricDefinition
	for _, includedMetric := range includedMetrics {
		var perfmonMetric *PerfmonMetric
		var found bool
		// find the metric in the perfmon metrics or perfspect metrics based on the origin
		switch strings.ToLower(includedMetric.Origin) {
		case "perfmon":
			if !metadata.SupportsFixedTMA {
				perfmonMetric, found = findPerfmonMetric(alternateTMAMetrics, includedMetric.LegacyName)
			}
			if !found {
				perfmonMetric, found = findPerfmonMetric(perfmonMetrics, includedMetric.LegacyName)
			}
		case "perfspect":
			perfmonMetric, found = findPerfmonMetric(perfspectMetrics, includedMetric.LegacyName)
		default:
			slog.Warn("Unknown metric origin", "origin", includedMetric.Origin, "metric", includedMetric.LegacyName)
			continue
		}
		if !found {
			slog.Warn("Metric not found in metric definitions", "metric", includedMetric.LegacyName, "origin", includedMetric.Origin)
			continue
		}
		// get the expression for the metric
		expression, err := getExpression(*perfmonMetric)
		if err != nil {
			slog.Warn("Failed getting expression for metric", "metric", perfmonMetric.LegacyName, "error", err)
			continue
		}
		// create a MetricDefinition from the perfmon metric
		metric := MetricDefinition{
			Name:        includedMetric.LegacyName,
			Description: perfmonMetric.BriefDescription,
			Expression:  expression,
		}
		// add the metric to the list of metrics
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func removeUncollectableMetrics(includedMetrics []PerfspectMetric, perfmonMetrics []PerfmonMetric, perfspectMetrics []PerfmonMetric, alternateTMAMetrics []PerfmonMetric, coreEvents CoreEvents, uncoreEvents UncoreEvents, otherEvents OtherEvents, metadata Metadata) ([]PerfspectMetric, error) {
	var collectableMetrics []PerfspectMetric
	for _, includedMetric := range includedMetrics {
		var perfmonMetric *PerfmonMetric
		var found bool
		// find the metric in the perfmon metrics or perfspect metrics based on the origin
		switch strings.ToLower(includedMetric.Origin) {
		case "perfmon":
			if !metadata.SupportsFixedTMA {
				perfmonMetric, found = findPerfmonMetric(alternateTMAMetrics, includedMetric.LegacyName)
			}
			if !found {
				perfmonMetric, found = findPerfmonMetric(perfmonMetrics, includedMetric.LegacyName)
			}
		case "perfspect":
			perfmonMetric, found = findPerfmonMetric(perfspectMetrics, includedMetric.LegacyName)
		default:
			slog.Warn("Unknown metric origin", "origin", includedMetric.Origin, "metric", includedMetric.LegacyName)
			continue
		}
		if !found {
			slog.Warn("Metric not found in metric definitions", "metric", includedMetric.LegacyName, "origin", includedMetric.Origin)
			continue
		}
		// collect the event names from the metric and check if any of them are uncollectable
		var eventNames []string
		for _, event := range perfmonMetric.Events {
			eventNames = util.UniqueAppend(eventNames, event["Name"])
		}
		uncollectableEvents := getUncollectableEvents(eventNames, coreEvents, uncoreEvents, otherEvents, metadata)
		if len(uncollectableEvents) > 0 {
			slog.Warn("Metric contains uncollectable events", "metric", includedMetric.LegacyName, "uncollectableEvents", uncollectableEvents)
			continue
		}
		// if the metric is collectable, add it to the list of collectable metrics
		collectableMetrics = append(collectableMetrics, includedMetric)
	}
	return collectableMetrics, nil
}

func loadEventGroupsFromMetrics(includedMetrics []PerfspectMetric, perfmonMetrics []PerfmonMetric, perfspectMetrics []PerfmonMetric, alternateTMAMetrics []PerfmonMetric, coreEvents CoreEvents, uncoreEvents UncoreEvents, otherEvents OtherEvents, metadata Metadata) ([]CoreGroup, []UncoreGroup, []OtherGroup, []string, error) {
	coreGroups := make([]CoreGroup, 0)
	uncoreGroups := make([]UncoreGroup, 0)
	otherGroups := make([]OtherGroup, 0)
	uncollectableEvents := make([]string, 0)

	for _, includedMetric := range includedMetrics {
		var metricEventNames []string
		var perfmonMetric *PerfmonMetric
		var found bool
		// find the metric in the perfmon metrics or perfspect metrics based on the origin
		// and collect the event names from the metric
		switch strings.ToLower(includedMetric.Origin) {
		case "perfmon":
			if !metadata.SupportsFixedTMA {
				perfmonMetric, found = findPerfmonMetric(alternateTMAMetrics, includedMetric.LegacyName)
			}
			if !found {
				perfmonMetric, found = findPerfmonMetric(perfmonMetrics, includedMetric.LegacyName)
			}
		case "perfspect":
			perfmonMetric, found = findPerfmonMetric(perfspectMetrics, includedMetric.LegacyName)
		default:
			return nil, nil, nil, nil, fmt.Errorf("unknown metric origin: %s for metric: %s", includedMetric.Origin, includedMetric.LegacyName)
		}
		if !found {
			return nil, nil, nil, nil, fmt.Errorf("metric %s not found in %s metrics", includedMetric.LegacyName, includedMetric.Origin)
		}
		for _, event := range perfmonMetric.Events {
			metricEventNames = util.UniqueAppend(metricEventNames, event["Name"])
		}
		// check if the metric has uncollectable events
		uncollectableMetricEvents := getUncollectableEvents(metricEventNames, coreEvents, uncoreEvents, otherEvents, metadata)
		// if there are uncollectable events, add them to the uncollectableEvents list
		uncollectableEvents = util.UniqueAppend(uncollectableEvents, uncollectableMetricEvents...)
		// skip metrics that have uncollectable events
		if len(uncollectableMetricEvents) > 0 {
			fmt.Printf("Warning: Metric %s contains uncollectable events: %v\n", includedMetric.LegacyName, uncollectableMetricEvents)
			continue
		}
		metricCoreGroups, metricUncoreGroups, metricOtherGroups, err := groupsFromEventNames(
			includedMetric.LegacyName,
			metricEventNames,
			coreEvents,
			uncoreEvents,
			otherEvents,
			metadata,
		)
		if err != nil {
			fmt.Printf("Error grouping events for metric %s: %v\n", includedMetric.LegacyName, err)
			continue
		}
		// Add the groups to the main lists
		coreGroups = append(coreGroups, metricCoreGroups...)
		uncoreGroups = append(uncoreGroups, metricUncoreGroups...)
		otherGroups = append(otherGroups, metricOtherGroups...)
	}
	return coreGroups, uncoreGroups, otherGroups, uncollectableEvents, nil
}

func getUncollectableEvents(eventNames []string, coreEvents CoreEvents, uncoreEvents UncoreEvents, otherEvents OtherEvents, metadata Metadata) []string {
	uncollectableEvents := make([]string, 0)
	for _, eventName := range eventNames {
		coreEvent := coreEvents.FindEventByName(eventName)
		if !coreEvent.IsEmpty() {
			if !coreEvent.IsCollectable(metadata) {
				uncollectableEvents = util.UniqueAppend(uncollectableEvents, coreEvent.EventName)
			}
			continue
		}
		uncoreEvent := uncoreEvents.FindEventByName(eventName)
		if uncoreEvent != (UncoreEvent{}) {
			if !uncoreEvent.IsCollectable(metadata) {
				uncollectableEvents = util.UniqueAppend(uncollectableEvents, uncoreEvent.EventName)
			}
			continue
		}
		otherEvent := otherEvents.FindEventByName(eventName)
		if otherEvent != (OtherEvent{}) {
			if !otherEvent.IsCollectable(metadata) {
				uncollectableEvents = util.UniqueAppend(uncollectableEvents, otherEvent.EventName)
			}
			continue
		}
		if !slices.Contains(constants, eventName) { // ignore constants, they'll be handled separately
			fmt.Printf("Warning: Event %s not found in core or uncore events\n", eventName)
			uncollectableEvents = util.UniqueAppend(uncollectableEvents, eventName) // if the event is not found in either core or uncore events, we consider it uncollectable
		}
	}
	return uncollectableEvents
}

func eliminateDuplicateGroups(coreGroups []CoreGroup, uncoreGroups []UncoreGroup) ([]CoreGroup, []UncoreGroup, error) {
	coreGroups, err := EliminateDuplicateCoreGroups(coreGroups)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to eliminate duplicate core groups: %w", err)
	}
	uncoreGroups, err = EliminateDuplicateUncoreGroups(uncoreGroups)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to elminate duplicate uncore groups: %w", err)
	}
	return coreGroups, uncoreGroups, nil
}

func mergeGroups(coreGroups []CoreGroup, uncoreGroups []UncoreGroup, metadata Metadata) ([]CoreGroup, []UncoreGroup, error) {
	coreGroups, err := MergeCoreGroups(coreGroups, metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("error merging core groups: %w", err)
	}
	uncoreGroups, err = MergeUncoreGroups(uncoreGroups)
	if err != nil {
		return nil, nil, fmt.Errorf("error merging uncore groups: %w", err)
	}
	return coreGroups, uncoreGroups, nil
}

var constants []string = []string{
	"TSC",
}

func groupsFromEventNames(metricName string, eventNames []string, coreEvents CoreEvents, uncoreEvents UncoreEvents, otherEvents OtherEvents, metadata Metadata) ([]CoreGroup, []UncoreGroup, []OtherGroup, error) {
	var coreGroups []CoreGroup
	var uncoreGroups []UncoreGroup
	var otherGroups []OtherGroup
	coreGroup := NewCoreGroup(metadata)
	uncoreGroup := NewUncoreGroup(metadata)
	for _, eventName := range eventNames {
		// Skip constants, they are not events
		if slices.Contains(constants, eventName) {
			continue
		}
		if strings.Contains(eventName, "retire_latency") {
			// skip <event>:retire_latency
			continue
		}
		coreEvent := coreEvents.FindEventByName(eventName)
		if !coreEvent.IsEmpty() { // this is a core event
			// if the event has been customized with :c<val>, :e<val>, or both, we create a new event with
			// customizations in the name
			if strings.Contains(eventName, ":") {
				// Create a copy of the event with the customized name
				coreEvent.EventName = eventName
			}
			coreGroup.MetricNames = util.UniqueAppend(coreGroup.MetricNames, metricName)
			err := coreGroup.AddEvent(coreEvent, false, metadata)
			if err != nil {
				fmt.Printf("Creating additional core group for metric %s, event %s: %v\n", metricName, eventName, err)
				coreGroups = append(coreGroups, coreGroup)
				coreGroup = NewCoreGroup(metadata) // Reset coreGroup for the next set of events
				coreGroup.MetricNames = util.UniqueAppend(coreGroup.MetricNames, metricName)
				err = coreGroup.AddEvent(coreEvent, false, metadata) // Add the event to the new group
				if err != nil {
					return nil, nil, nil, fmt.Errorf("error adding event %s to new core group: %w", eventName, err)
				}
			}
		} else {
			uncoreEvent := uncoreEvents.FindEventByName(eventName)
			if !uncoreEvent.IsEmpty() { // this is an uncore event
				uncoreGroup.MetricNames = util.UniqueAppend(uncoreGroup.MetricNames, metricName)
				err := uncoreGroup.AddEvent(uncoreEvent, false)
				if err != nil {
					fmt.Printf("Creating additional uncore group for metric %s, event %s: %v\n", metricName, eventName, err)
					uncoreGroups = append(uncoreGroups, uncoreGroup)
					uncoreGroup = NewUncoreGroup(metadata) // Reset uncoreGroup for the next set of events
					uncoreGroup.MetricNames = util.UniqueAppend(uncoreGroup.MetricNames, metricName)
					err = uncoreGroup.AddEvent(uncoreEvent, false) // Add the event
					if err != nil {
						return nil, nil, nil, fmt.Errorf("error adding event %s to new uncore group: %w", eventName, err)
					}
				}
			} else {
				otherEvent := otherEvents.FindEventByName(eventName)
				if !otherEvent.IsEmpty() { // this is an other event
					otherGroup := NewOtherGroup(metadata)
					otherGroup.MetricNames = util.UniqueAppend(otherGroup.MetricNames, metricName)
					err := otherGroup.AddEvent(otherEvent, false)
					if err != nil {
						return nil, nil, nil, fmt.Errorf("error adding other event %s to group for metric %s: %w", eventName, metricName, err)
					} else {
						otherGroups = append(otherGroups, otherGroup)
					}
				}
			}
		}
	}
	// if there are any events in the core group, add it to the groups
	coreGroupAdded := false
	for _, event := range coreGroup.FixedPurposeCounters {
		if !event.IsEmpty() {
			coreGroups = append(coreGroups, coreGroup)
			coreGroupAdded = true
			break
		}
	}
	if !coreGroupAdded {
		for _, event := range coreGroup.GeneralPurposeCounters {
			if !event.IsEmpty() {
				coreGroups = append(coreGroups, coreGroup)
				break
			}
		}
	}
	// if there are any events in the uncore group, add it to the groups
	for _, event := range uncoreGroup.GeneralPurposeCounters {
		if !event.IsEmpty() {
			uncoreGroups = append(uncoreGroups, uncoreGroup)
			break
		}
	}
	return coreGroups, uncoreGroups, otherGroups, nil
}

// findPerfmonMetric -- Helper function to find a metric by name
func findPerfmonMetric(metricsList []PerfmonMetric, metricName string) (*PerfmonMetric, bool) {
	for _, metric := range metricsList {
		if metric.LegacyName == metricName {
			return &metric, true
		}
	}
	return nil, false
}

//
// Retire Latency Files
//

type PlatformInfo struct {
	ModelName      string `json:"Model name"`
	CPUFamily      string `json:"CPU family"`
	Model          string `json:"Model"`
	ThreadsPerCore string `json:"Thread(s) per core"`
	CoresPerSocket string `json:"Core(s) per socket"`
	Sockets        string `json:"Socket(s)"`
	Stepping       string `json:"Stepping"`
	L3Cache        string `json:"L3 cache"`
	NUMANodes      string `json:"NUMA node(s)"`
	TMAVersion     string `json:"TMA version"`
}

type MetricStats struct {
	Min  float64 `json:"MIN"`
	Max  float64 `json:"MAX"`
	Mean float64 `json:"MEAN"`
}

type RetireLatency struct {
	Platform PlatformInfo           `json:"Platform"`
	Data     map[string]MetricStats `json:"Data"`
}

// loadRetireLatencies loads the retire latencies from a JSON file based on the microarchitecture
// it returns a map of event names to their retire latencies
// the retire latency is the mean value of the metric stats
func loadRetireLatencies(retireLatenciesFile string) (map[string]string, error) {
	var bytes []byte
	var err error
	if bytes, err = resources.ReadFile(retireLatenciesFile); err != nil {
		slog.Error("failed to read retire latencies file", slog.String("file", retireLatenciesFile), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to read retire latencies file %s: %w", retireLatenciesFile, err)
	}
	var retireLatency RetireLatency
	if err = json.Unmarshal(bytes, &retireLatency); err != nil {
		slog.Error("failed to unmarshal retire latencies", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to unmarshal retire latencies: %w", err)
	}
	// create a map of retire latencies
	retireLatencies := make(map[string]string)
	for event, stats := range retireLatency.Data {
		// use the mean value for the retire latency
		retireLatencies[event] = fmt.Sprintf("%f", stats.Mean)
	}
	slog.Debug("loaded retire latencies", slog.Any("latencies", retireLatencies))
	return retireLatencies, nil
}

// replaceRetireLatencies replaces retire latencies in metrics with their values
func replaceRetireLatencies(metrics []MetricDefinition, retireLatenciesFile string) ([]MetricDefinition, error) {
	// load retire latencies
	retireLatencies, err := loadRetireLatencies(retireLatenciesFile)
	if err != nil {
		slog.Error("failed to load retire latencies", slog.String("error", err.Error()))
		return nil, err
	}
	// replace retire latencies in metrics
	for i := range metrics {
		metric := &metrics[i]
		for retireEvent, retireLatency := range retireLatencies {
			// replace <event>:retire_latency with value
			metric.Expression = strings.ReplaceAll(metric.Expression, fmt.Sprintf("[%s:retire_latency]", retireEvent), retireLatency)
		}
	}
	return metrics, nil
}
