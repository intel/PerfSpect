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

type MetricsConfigHeader struct {
	Copyright string `json:"Copyright"`
	Info      string `json:"Info"`
}
type PerfspectMetric struct {
	MetricName string `json:"MetricName"`
	Origin     string `json:"Origin"`
}
type MetricsConfig struct {
	Header                   MetricsConfigHeader `json:"Header"`
	PerfmonMetricsFile       string              `json:"PerfmonMetricsFile"`       // Path to the perfmon metrics file
	PerfmonCoreEventsFile    string              `json:"PerfmonCoreEventsFile"`    // Path to the perfmon core events file
	PerfmonUncoreEventsFile  string              `json:"PerfmonUncoreEventsFile"`  // Path to the perfmon uncore events file
	PerfmonRetireLatencyFile string              `json:"PerfmonRetireLatencyFile"` // Path to the perfmon retire latency file
	AlternateTMAMetricsFile  string              `json:"AlternateTMAMetricsFile"`  // Path to the alternate TMA metrics file
	PerfspectMetricsFile     string              `json:"PerfspectMetricsFile"`     // Path to the Perfspect metrics file
	ReportMetrics            []PerfspectMetric   `json:"ReportMetrics"`            // Metrics that are reported in the PerfSpect report
}

func (l *PerfmonLoader) Load(loaderConfig LoaderConfig) ([]MetricDefinition, []GroupDefinition, error) {
	// Load the metrics configuration from the JSON file
	config, err := l.loadMetricsConfig(loaderConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load metrics config: %w", err)
	}
	// Load the perfmon metric definitions from the JSON file
	perfmonMetricDefinitions, err := loadPerfmonMetricsFromFile(config.PerfmonMetricsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading perfmon metrics from file: %w", err)
	}
	// Load the perfmon core events from the JSON file
	coreEvents, err := NewCoreEvents(config.PerfmonCoreEventsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading perfmon core events: %w", err)
	}
	// Load the perfmon uncore events from the JSON file
	uncoreEvents, err := NewUncoreEvents(config.PerfmonUncoreEventsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading perfmon uncore events: %w", err)
	}
	// Load the other events (not core or uncore)
	otherEvents, err := NewOtherEvents()
	if err != nil {
		return nil, nil, fmt.Errorf("error loading other events: %w", err)
	}
	// Load the (optional) alternate TMA metrics from the JSON file
	var alternateTMAMetrics PerfmonMetrics
	if config.AlternateTMAMetricsFile != "" {
		alternateTMAMetrics, err = loadPerfmonMetricsFromFile(config.AlternateTMAMetricsFile)
		if err != nil {
			return nil, nil, fmt.Errorf("error loading alternate TMA metrics from file: %w", err)
		}
	}
	// Load the Perfspect metrics from the JSON file
	perfspectMetrics, err := loadPerfmonMetricsFromFile(config.PerfspectMetricsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading Perfspect metrics from file: %w", err)
	}
	// Filter the report metrics based on the selected metrics
	reportMetrics, err := filterReportMetrics(config.ReportMetrics, loaderConfig.SelectedMetrics, append(perfmonMetricDefinitions.Metrics, perfspectMetrics.Metrics...))
	if err != nil {
		return nil, nil, fmt.Errorf("error filtering report metrics: %w", err)
	}
	// Combine the PerfSpect-defined metrics with the metric definitions from perfmon and filter based on report metrics
	// Creates one list of all metrics to be used in the loader
	perfmonMetrics, err := loadPerfmonMetrics(reportMetrics, perfmonMetricDefinitions.Metrics, perfspectMetrics.Metrics, alternateTMAMetrics.Metrics, loaderConfig.Metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading perfmon metrics: %w", err)
	}
	// Remove metrics that use uncollectable events
	perfmonMetrics, err = removeUncollectableMetrics(perfmonMetrics, coreEvents, uncoreEvents, otherEvents, loaderConfig.Metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("error removing uncollectable metrics: %w", err)
	}
	// Load the metric definitions (this is the type that will be returned per the interface definition)
	metricDefs, err := perfmonToMetricDefs(perfmonMetrics)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading metrics from definitions: %w", err)
	}
	// Simplify OCR event names in metric expressions
	metricDefs = customizeOCREventNames(metricDefs)
	// Create event groups from the perfspect metrics
	coreGroups, uncoreGroups, otherGroups, uncollectableEvents, err := loadEventGroupsFromMetrics(
		perfmonMetrics,
		coreEvents,
		uncoreEvents,
		otherEvents,
		loaderConfig.Metadata,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading event groups from metrics: %v", err)
	}
	// Eliminate duplicate groups
	coreGroups, uncoreGroups, err = eliminateDuplicateGroups(coreGroups, uncoreGroups)
	if err != nil {
		return nil, nil, fmt.Errorf("error merging duplicate groups: %v", err)
	}
	// Merge groups that can be merged, i.e., if 2nd group's events fit in the first group
	coreGroups, uncoreGroups, err = mergeGroups(coreGroups, uncoreGroups, loaderConfig.Metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("error merging groups: %v", err)
	}
	slog.Debug("Number of core groups", slog.Int("count", len(coreGroups)))
	slog.Debug("Number of uncore groups", slog.Int("count", len(uncoreGroups)))
	slog.Debug("Number of other groups", slog.Int("count", len(otherGroups)))
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
	// Replace retire latencies variables with their values
	if config.PerfmonRetireLatencyFile != "" {
		metricDefs, err = replaceRetireLatencies(metricDefs, config.PerfmonRetireLatencyFile)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to replace retire latencies: %w", err)
		}
	}
	// Apply common modifications to metric expressions
	metricDefs, err = configureMetrics(metricDefs, uncollectableEvents, loaderConfig.Metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to configure metrics: %w", err)
	}
	return metricDefs, allGroups, nil
}

// uarchToResourceName maps from the CPU's microarchiteture, as defined in the cpus
// module, to the associated perfmon resource directory and config file name
func uarchToResourceName(uarch string) string {
	name := strings.ToLower(uarch)
	name = strings.Split(name, "_")[0] // Handle "GNR_X2", etc.
	name = strings.Split(name, "-")[0] // Handle GNR-D
	return name
}

func (l *PerfmonLoader) loadMetricsConfig(loaderConfig LoaderConfig) (MetricsConfig, error) {
	var config MetricsConfig
	var bytes []byte
	if loaderConfig.ConfigFileOverride != "" {
		var err error
		bytes, err = os.ReadFile(loaderConfig.ConfigFileOverride)
		if err != nil {
			return MetricsConfig{}, fmt.Errorf("error reading metric config override file: %w", err)
		}
	} else {
		var err error
		resourceName := uarchToResourceName(loaderConfig.Metadata.Microarchitecture)
		bytes, err = resources.ReadFile(filepath.Join("resources", "perfmon", resourceName, resourceName+".json"))
		if err != nil {
			return MetricsConfig{}, fmt.Errorf("error reading metrics config file: %w", err)
		}
	}
	if err := json.Unmarshal(bytes, &config); err != nil {
		return MetricsConfig{}, fmt.Errorf("error unmarshaling metrics config JSON: %w", err)
	}
	return config, nil
}

func loadPerfmonMetricsFromFile(pathWithSource string) (PerfmonMetrics, error) {
	var metrics PerfmonMetrics
	pathParts := strings.Split(pathWithSource, ":")
	if len(pathParts) != 2 || (pathParts[0] != "resources" && pathParts[0] != "file") {
		return PerfmonMetrics{}, fmt.Errorf("invalid path format, expected 'resources:<path>' or 'file:<path>' but got '%s'", pathWithSource)
	}
	var path string
	var bytes []byte
	var err error
	if pathParts[0] == "resources" {
		path = filepath.Join("resources", "perfmon", pathParts[1])
		bytes, err = resources.ReadFile(path)
	} else { // pathParts[0] == "file"
		path = pathParts[1]
		bytes, err = os.ReadFile(path) // #nosec G304
	}
	if err != nil {
		return PerfmonMetrics{}, fmt.Errorf("error reading file %s: %w", path, err)
	}
	if err := json.Unmarshal(bytes, &metrics); err != nil {
		return PerfmonMetrics{}, fmt.Errorf("error unmarshaling JSON from %s: %w", path, err)
	}
	return metrics, nil
}

func filterReportMetrics(reportMetrics []PerfspectMetric, selectedMetricNames []string, perfmonMetrics []PerfmonMetric) ([]PerfspectMetric, error) {
	if len(selectedMetricNames) == 0 {
		slog.Debug("No selected metrics provided, using all report metrics")
		return reportMetrics, nil
	}
	slog.Debug("Filtering report metrics based on selected metrics", slog.Any("selectedMetrics", selectedMetricNames))
	var filteredMetrics []PerfspectMetric
	for _, metricName := range selectedMetricNames {
		pm, pmFound := findPerfmonMetricByLegacyName(perfmonMetrics, "metric_"+metricName)
		if pmFound {
			found := false
			for _, metric := range reportMetrics {
				if strings.EqualFold(pm.MetricName, metric.MetricName) {
					filteredMetrics = append(filteredMetrics, metric)
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("metric not found in perfspect metrics: %s", metricName)
			}
		} else {
			return nil, fmt.Errorf("metric not found in perfmon metrics: %s", metricName)
		}
	}
	return filteredMetrics, nil
}

func loadPerfmonMetrics(reportMetrics []PerfspectMetric, perfmonMetrics []PerfmonMetric, configMetrics []PerfmonMetric, alternateTMAMetrics []PerfmonMetric, metadata Metadata) ([]PerfmonMetric, error) {
	var perfmonMetricsToReturn []PerfmonMetric
	allPerfmonMetrics := append(configMetrics, perfmonMetrics...)
	for _, metric := range reportMetrics {
		var perfmonMetric *PerfmonMetric
		var found bool
		if !metadata.SupportsFixedTMA {
			perfmonMetric, found = findPerfmonMetric(alternateTMAMetrics, metric.MetricName)
		}
		if !found {
			perfmonMetric, found = findPerfmonMetric(allPerfmonMetrics, metric.MetricName)
		}
		if !found {
			slog.Warn("Metric not found in metric definitions", "metric", metric.MetricName, "origin", metric.Origin)
			continue
		}
		// Add the metric to the list of metrics to return
		perfmonMetricsToReturn = append(perfmonMetricsToReturn, *perfmonMetric)
	}
	return perfmonMetricsToReturn, nil
}

// customizeOCREventNames scans through a slice of MetricDefinition objects and customizes the names of OCR events
// in their Expression fields. Specifically, it looks for substrings matching the pattern
// "[OCR.<event_name>:ocr_msr_val=<msr_value>]" and replaces them with a customized name in the format
// "<event_name>.<msr_value>". Only expressions containing ":ocr_msr_val=" are processed. The function returns
// the modified slice of MetricDefinition objects.
func customizeOCREventNames(metrics []MetricDefinition) []MetricDefinition {
	re := regexp.MustCompile(`(OCR\.[^\]]+):ocr_msr_val=([0-9a-fx]+)`)
	for i := range metrics {
		metric := &metrics[i]
		// example portion of expression: [OCR.DEMAND_RFO.L3_MISS:ocr_msr_val=0x103b8000]
		if !strings.Contains(metric.Expression, ":ocr_msr_val=") {
			continue // only customize OCR events with this format
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
	return metrics
}

// getExpression retrieves the expression for a given PerfmonMetric, replacing variables with their corresponding event or constant names.
// example formula: "( 1000000000 * (a / b) / (c / (d * socket_count) ) ) * DURATIONTIMEINSECONDS"
// desired output: "( 1000000000 * ([event1] / [event2]) / ([constant1] / ([constant2] * socket_count) ) ) * 1"
func getExpression(perfmonMetric PerfmonMetric) (string, error) {
	if perfmonMetric.Formula == "" {
		return "", fmt.Errorf("metric '%s' has no formula defined", perfmonMetric.MetricName)
	}
	expression := perfmonMetric.Formula
	replacers := make(map[string]string)
	for _, event := range perfmonMetric.Events {
		replacers[event["Alias"]] = fmt.Sprintf("[%s]", event["Name"])
	}
	for _, constant := range perfmonMetric.Constants {
		replacers[constant["Alias"]] = fmt.Sprintf("[%s]", constant["Name"])
	}
	for alias, replacement := range replacers {
		// Replace alias as whole words only (not substrings)
		expression = util.ReplaceWholeWord(expression, alias, replacement)
	}
	// replace common constants with their values
	commonEventReplacements := map[string]string{
		"DURATIONTIMEINSECONDS":        "1",
		"[DURATIONTIMEINMILLISECONDS]": "1000",
	}
	for commonEvent, alias := range commonEventReplacements {
		expression = strings.ReplaceAll(expression, commonEvent, alias)
	}
	// replace fixed counter perfmon event names with their corresponding perf
	// event names found in the fixedCounterEventNameTranslation map
	// example: "100 * ([CPU_CLK_UNHALTED.REF_TSC:k] / [TSC])"
	// becomes  "100 * ([ref-cycles:k] / [TSC])"
	expressionVarPattern := regexp.MustCompile(`\[[^\]]+\]`)
	for _, match := range expressionVarPattern.FindAllString(expression, -1) {
		// strip the brackets
		match = strings.Trim(match, "[]")
		// check if the match is in the translation map
		if perfEventName, ok := fixedCounterEventNameTranslation[match]; ok {
			expression = strings.ReplaceAll(expression, match, perfEventName)
		}
	}
	return expression, nil
}

func getThresholdExpression(perfmonMetric PerfmonMetric) (string, error) {
	if perfmonMetric.Threshold == nil {
		return "", nil // no threshold defined
	}
	// SRF metrics are currently defined using an older perfmon format that doesn't
	// define threshold metrics. We could parse them out manually, but for now we'll just skip them.
	if perfmonMetric.Threshold.ThresholdMetrics == nil {
		return "", nil // no threshold defined
	}
	expression := perfmonMetric.Threshold.Formula
	if expression == "" {
		return "", nil // no threshold defined
	}
	replacers := make(map[string]string)
	for _, thresholdMetric := range perfmonMetric.Threshold.ThresholdMetrics {
		replacers[thresholdMetric["Alias"]] = fmt.Sprintf("[%s]", thresholdMetric["Value"])
	}
	for alias, replacement := range replacers {
		// Replace alias as whole words only (not substrings)
		expression = util.ReplaceWholeWord(expression, alias, replacement)
	}
	return expression, nil
}

func perfmonToMetricDefs(perfmonMetrics []PerfmonMetric) ([]MetricDefinition, error) {
	var metrics []MetricDefinition
	for _, perfmonMetric := range perfmonMetrics {
		// get the expression for the metric
		expression, err := getExpression(perfmonMetric)
		if err != nil {
			slog.Warn("Failed getting expression for metric", "metric", perfmonMetric.MetricName, "error", err)
			continue
		}
		thresholdExpression, err := getThresholdExpression(perfmonMetric)
		if err != nil {
			slog.Warn("Failed getting threshold expression for metric", "metric", perfmonMetric.MetricName, "error", err)
			continue
		}
		// create a MetricDefinition from the perfmon metric
		metric := MetricDefinition{
			Name:                perfmonMetric.MetricName,
			LegacyName:          perfmonMetric.LegacyName,
			Description:         perfmonMetric.BriefDescription,
			Category:            perfmonMetric.Category,
			Level:               perfmonMetric.Level,
			Expression:          expression,
			ThresholdExpression: thresholdExpression,
		}
		// add the metric to the list of metrics
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func removeUncollectableMetrics(perfmonMetrics []PerfmonMetric, coreEvents CoreEvents, uncoreEvents UncoreEvents, otherEvents OtherEvents, metadata Metadata) ([]PerfmonMetric, error) {
	var collectableMetrics []PerfmonMetric
	for _, perfmonMetric := range perfmonMetrics {
		// collect the event names from the metric and check if any of them are not collectable
		var eventNames []string
		for _, event := range perfmonMetric.Events {
			eventNames = util.UniqueAppend(eventNames, event["Name"])
		}
		uncollectableEvents := getUncollectableEvents(eventNames, coreEvents, uncoreEvents, otherEvents, metadata)
		if len(uncollectableEvents) > 0 {
			slog.Warn("Metric contains uncollectable events", "metric", perfmonMetric.MetricName, "uncollectableEvents", uncollectableEvents)
			continue
		}
		// if the metric is collectable, add it to the list of collectable metrics
		collectableMetrics = append(collectableMetrics, perfmonMetric)
	}
	return collectableMetrics, nil
}

func loadEventGroupsFromMetrics(perfmonMetrics []PerfmonMetric, coreEvents CoreEvents, uncoreEvents UncoreEvents, otherEvents OtherEvents, metadata Metadata) ([]CoreGroup, []UncoreGroup, []OtherGroup, []string, error) {
	coreGroups := make([]CoreGroup, 0)
	uncoreGroups := make([]UncoreGroup, 0)
	otherGroups := make([]OtherGroup, 0)
	uncollectableEvents := make([]string, 0)

	for _, perfmonMetric := range perfmonMetrics {
		var metricEventNames []string
		for _, event := range perfmonMetric.Events {
			metricEventNames = util.UniqueAppend(metricEventNames, event["Name"])
		}
		// check if the metric has uncollectable events
		uncollectableMetricEvents := getUncollectableEvents(metricEventNames, coreEvents, uncoreEvents, otherEvents, metadata)
		// if there are uncollectable events, add them to the uncollectableEvents list
		uncollectableEvents = util.UniqueAppend(uncollectableEvents, uncollectableMetricEvents...)
		// skip metrics that have uncollectable events
		if len(uncollectableMetricEvents) > 0 {
			slog.Warn("Metric contains uncollectable events", "metric", perfmonMetric.MetricName, "uncollectableEvents", uncollectableMetricEvents)
			continue
		}
		metricCoreGroups, metricUncoreGroups, metricOtherGroups, err := groupsFromEventNames(
			perfmonMetric.MetricName,
			metricEventNames,
			coreEvents,
			uncoreEvents,
			otherEvents,
			metadata,
		)
		if err != nil {
			slog.Error("Error creating groups from event names", "metric", perfmonMetric.MetricName, "error", err)
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
			slog.Warn("Event not found in core or uncore events", "event", eventName)
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
		if metric.MetricName == metricName {
			return &metric, true
		}
	}
	return nil, false
}

// findPerfmonMetricByLegacyName -- Helper function to find a metric by legacy name
func findPerfmonMetricByLegacyName(metricsList []PerfmonMetric, legacyName string) (*PerfmonMetric, bool) {
	for _, metric := range metricsList {
		if strings.EqualFold(metric.LegacyName, legacyName) {
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
func loadRetireLatencies(pathWithSource string) (map[string]string, error) {
	pathParts := strings.Split(pathWithSource, ":")
	if len(pathParts) != 2 || (pathParts[0] != "resources" && pathParts[0] != "file") {
		return nil, fmt.Errorf("invalid path format, expected 'resources:<path>' or 'file:<path>' but got '%s'", pathWithSource)
	}
	var bytes []byte
	var err error
	if pathParts[0] == "resources" {
		retireLatenciesFile := filepath.Join("resources", "perfmon", pathParts[1])
		bytes, err = resources.ReadFile(retireLatenciesFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read retire latencies file %s: %w", retireLatenciesFile, err)
		}
	} else { // pathParts[0] == "file"
		bytes, err = os.ReadFile(pathParts[1]) // #nosec G304
		if err != nil {
			return nil, fmt.Errorf("failed to read retire latencies file %s: %w", pathParts[1], err)
		}
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
func replaceRetireLatencies(metrics []MetricDefinition, pathWithSource string) ([]MetricDefinition, error) {
	// load retire latencies
	retireLatencies, err := loadRetireLatencies(pathWithSource)
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
