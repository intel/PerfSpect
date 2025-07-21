package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"perfspect/internal/util"
	"regexp"
	"slices"
	"strings"
)

type PerfmonMetricHeader map[string]string
type PerfmonMetric struct {
	MetricName       string              `json:"MetricName"`
	LegacyName       string              `json:"LegacyName"`
	Level            int                 `json:"Level"`
	BriefDescription string              `json:"BriefDescription"`
	UnitOfMeasure    string              `json:"UnitOfMeasure"`
	Events           []map[string]string `json:"Events"`    // Each event is a map with string keys and values
	Constants        []map[string]string `json:"Constants"` // Each constant is a map with string keys and values
	Formula          string              `json:"Formula"`
	Category         string              `json:"Category"`
	ResolutionLevels string              `json:"ResolutionLevels"` // List of resolution levels
	MetricGroup      string              `json:"MetricGroup"`
}
type PerfmonMetrics struct {
	Header  PerfmonMetricHeader `json:"Header"`
	Metrics []PerfmonMetric     `json:"Metrics"`
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
	AlternateTMAMetricsFile  string              `json:"AlternateTMAMetricsFile"`  // Path to the alternate TMA metrics file
	Metrics                  []PerfmonMetric     `json:"Metrics"`                  // Metrics defined by PerfSpect
	ReportMetrics            []PerfspectMetric   `json:"ReportMetrics"`            // Metrics that are reported in the PerfSpect report
}

func (l *DynamicLoader) loadMetricsConfig(metricConfigOverridePath string, metadata Metadata) (MetricsConfig, error) {
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

func (l *DynamicLoader) Load(metricConfigOverridePath string, _ string, selectedMetrics []string, metadata Metadata) ([]MetricDefinition, []GroupDefinition, error) {
	config, err := l.loadMetricsConfig(metricConfigOverridePath, metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load metrics config: %w", err)
	}
	// Load the perfmon metric definitions from the JSON file
	perfmonMetricDefinitions, err := LoadPerfmonMetrics(filepath.Join("resources", "metrics", metadata.Architecture, metadata.Vendor, strings.ToLower(l.microarchitecture), config.PerfmonMetricsFile))
	if err != nil {
		return nil, nil, fmt.Errorf("error loading perfmon metrics: %w", err)
	}
	metrics, err := loadMetricsFromDefinitions(perfmonMetricDefinitions, config)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading metrics from definitions: %w", err)
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
	otherEvents, err := NewOtherEvents()
	if err != nil {
		return nil, nil, fmt.Errorf("error loading other events: %w", err)
	}
	// Create event groups from the perfspect metrics
	coreGroups, uncoreGroups, otherGroups, uncollectableEvents, err := loadEventGroupsFromMetrics(
		config.ReportMetrics,
		perfmonMetricDefinitions.Metrics,
		config.Metrics,
		coreEvents,
		uncoreEvents,
		otherEvents,
		metadata,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading event groups from metrics: %v", err)
	}
	// eliminate duplicate groups
	coreGroups, uncoreGroups, err = eliminateDuplicateGroups(coreGroups, uncoreGroups)
	if err != nil {
		return nil, nil, fmt.Errorf("error merging duplicate groups: %v", err)
	}
	// merge groups that can be merged, i.e., if 2nd group's events fit in the first group
	coreGroups, uncoreGroups, err = mergeGroups(coreGroups, uncoreGroups)
	if err != nil {
		return nil, nil, fmt.Errorf("error merging groups: %v", err)
	}
	// expand uncore groups for uncore devices
	uncoreGroups, err = ExpandUncoreGroups(uncoreGroups, metadata.UncoreDeviceIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("error expanding uncore groups: %v", err)
	}

	// Merge all groups into a single slice of GroupDefinition
	allGroups := make([]GroupDefinition, 0)
	for i, group := range coreGroups {
		perfGroup, _ := group.StringForPerf()
		fmt.Printf("echo \"core group %d\"\nsudo ./perf stat -e '%s' sleep 1\n\n", i, perfGroup)
		allGroups = append(allGroups, group.ToGroupDefinition())
	}
	for i, group := range uncoreGroups {
		perfGroup, _ := group.StringForPerf()
		fmt.Printf("echo \"uncore group %d\"\nsudo ./perf stat -e '%s' sleep 1\n\n", i, perfGroup)
		allGroups = append(allGroups, group.ToGroupDefinition())
	}
	for i, group := range otherGroups {
		perfGroup, _ := group.StringForPerf()
		fmt.Printf("echo \"other group %d\"\nsudo ./perf stat -e '%s' sleep 1\n\n", i, perfGroup)
		allGroups = append(allGroups, group.ToGroupDefinition())
	}

	// abbreviate uncore event names in metric expressions
	metrics, err = abbreviateUncoreEventNames(metrics, uncoreEvents)
	if err != nil {
		return nil, nil, fmt.Errorf("error abbreviating uncore event names: %w", err)
	}

	metrics, err = configureMetrics(metrics, uncollectableEvents, metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to configure metrics: %w", err)
	}
	return metrics, allGroups, nil
}

func abbreviateUncoreEventNames(metrics []MetricDefinition, uncoreEvents UncoreEvents) ([]MetricDefinition, error) {
	abbreviatedMetrics := make([]MetricDefinition, 0, len(metrics))
	for i := range metrics {
		metric := metrics[i]
		abbreviatedExpression := metric.Expression
		for _, uncoreEvent := range uncoreEvents.Events {
			re, err := regexp.Compile(fmt.Sprintf(`\b%s\b`, uncoreEvent.EventName))
			if err != nil {
				return nil, fmt.Errorf("failed to compile regex for uncore event %s: %w", uncoreEvent.EventName, err)
			}
			for {
				index := re.FindStringIndex(abbreviatedExpression)
				if index == nil {
					break // no more matches found
				}
				// replace the first occurrence of the alias with the replacement
				abbreviatedExpression = abbreviatedExpression[:index[0]] + uncoreEvent.UniqueID + abbreviatedExpression[index[1]:]
			}
		}
		metric.Expression = abbreviatedExpression
		abbreviatedMetrics = append(abbreviatedMetrics, metric)
	}
	return abbreviatedMetrics, nil
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

func loadMetricsFromDefinitions(perfmonMetrics PerfmonMetrics, metricsConfig MetricsConfig) ([]MetricDefinition, error) {
	var metrics []MetricDefinition
	for _, includedMetric := range metricsConfig.ReportMetrics {
		var perfmonMetric *PerfmonMetric
		var found bool

		switch strings.ToLower(includedMetric.Origin) {
		case "perfmon":
			perfmonMetric, found = findPerfmonMetric(perfmonMetrics.Metrics, includedMetric.MetricName)
		case "perfspect":
			perfmonMetric, found = findPerfmonMetric(metricsConfig.Metrics, includedMetric.MetricName)
		default:
			return nil, fmt.Errorf("unknown metric origin: %s for metric: %s", includedMetric.Origin, includedMetric.MetricName)
		}

		if !found {
			return nil, fmt.Errorf("metric %s not found in %s metrics", includedMetric.MetricName, includedMetric.Origin)
		}

		expression, err := getExpression(*perfmonMetric)
		if err != nil {
			return nil, fmt.Errorf("error getting expression for metric %s: %w", perfmonMetric.MetricName, err)
		}

		metric := MetricDefinition{
			Name:        includedMetric.MetricName,
			Description: perfmonMetric.BriefDescription,
			Expression:  expression,
		}
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

// loadEventGroupsFromMetrics
func loadEventGroupsFromMetrics(includedMetrics []PerfspectMetric, perfmonMetrics []PerfmonMetric, perfspectMetrics []PerfmonMetric, coreEvents CoreEvents, uncoreEvents UncoreEvents, otherEvents OtherEvents, metadata Metadata) ([]CoreGroup, []UncoreGroup, []OtherGroup, []string, error) {
	coreGroups := make([]CoreGroup, 0)
	uncoreGroups := make([]UncoreGroup, 0)
	otherGroups := make([]OtherGroup, 0)
	uncollectableEvents := make([]string, 0)

	if flagTransactionRate == 0 {
		uncollectableEvents = append(uncollectableEvents, "TXN")
	}

	for _, includedMetric := range includedMetrics {
		var metricEventNames []string
		var perfmonMetric *PerfmonMetric
		var found bool

		switch strings.ToLower(includedMetric.Origin) {
		case "perfmon":
			perfmonMetric, found = findPerfmonMetric(perfmonMetrics, includedMetric.MetricName)
		case "perfspect":
			perfmonMetric, found = findPerfmonMetric(perfspectMetrics, includedMetric.MetricName)
		default:
			return nil, nil, nil, nil, fmt.Errorf("unknown metric origin: %s for metric: %s", includedMetric.Origin, includedMetric.MetricName)
		}
		if !found {
			return nil, nil, nil, nil, fmt.Errorf("metric %s not found in %s metrics", includedMetric.MetricName, includedMetric.Origin)
		}
		for _, event := range perfmonMetric.Events {
			metricEventNames = util.UniqueAppend(metricEventNames, event["Name"])
		}
		uncollectableMetricEvents := getUncollectableEvents(metricEventNames, coreEvents, uncoreEvents, otherEvents, metadata)
		if len(uncollectableMetricEvents) > 0 {
			fmt.Printf("Warning: Metric %s contains uncollectable events: %v\n", includedMetric.MetricName, uncollectableMetricEvents)
			uncollectableEvents = util.UniqueAppend(uncollectableEvents, uncollectableMetricEvents...)
			// don't create groups for this metric
			continue
		}
		metricCoreGroups, metricUncoreGroups, metricOtherGroups, err := groupsFromEventNames(
			includedMetric.MetricName,
			metricEventNames,
			coreEvents,
			uncoreEvents,
			otherEvents,
			metadata,
		)
		if err != nil {
			fmt.Printf("Error grouping events for metric %s: %v\n", includedMetric.MetricName, err)
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
		if coreEvent != (CoreEvent{}) {
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

func mergeGroups(coreGroups []CoreGroup, uncoreGroups []UncoreGroup) ([]CoreGroup, []UncoreGroup, error) {
	coreGroups, err := MergeCoreGroups(coreGroups)
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
	uncoreGroup := NewUncoreGroup(metadata.NumGeneralPurposeCounters)
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
		if coreEvent != (CoreEvent{}) { // this is a core event
			if !coreEvent.IsCollectable(metadata) {
				return nil, nil, nil, fmt.Errorf("core event %s is not collectable on target", eventName)
			}
			// if the event has been customized with :c<val>, :e<val>, or both, we create a new event with
			// customizations in the name
			if strings.Contains(eventName, ":") {
				// Create a copy of the event with the customized name
				coreEvent.EventName = eventName
			}
			coreGroup.MetricNames = util.UniqueAppend(coreGroup.MetricNames, metricName)
			err := coreGroup.AddEvent(coreEvent, false)
			if err != nil {
				fmt.Printf("Creating additional core group for metric %s, event %s: %v\n", metricName, eventName, err)
				coreGroups = append(coreGroups, coreGroup)
				coreGroup = NewCoreGroup(metadata)         // Reset coreGroup for the next set of events
				err = coreGroup.AddEvent(coreEvent, false) // Add the event to the new group
				if err != nil {
					return nil, nil, nil, fmt.Errorf("error adding event %s to new core group: %w", eventName, err)
				}
			}
		} else {
			uncoreEvent := uncoreEvents.FindEventByName(eventName)
			if uncoreEvent != (UncoreEvent{}) { // this is an uncore event
				if !uncoreEvent.IsCollectable(metadata) {
					return nil, nil, nil, fmt.Errorf("uncore event %s is not collectable on target", eventName)
				}
				uncoreGroup.MetricNames = util.UniqueAppend(uncoreGroup.MetricNames, metricName)
				err := uncoreGroup.AddEvent(uncoreEvent, false)
				if err != nil {
					fmt.Printf("Creating additional uncore group for metric %s, event %s: %v\n", metricName, eventName, err)
					uncoreGroups = append(uncoreGroups, uncoreGroup)
					uncoreGroup = NewUncoreGroup(metadata.NumGeneralPurposeCounters) // Reset uncoreGroup for the next set of events
					err = uncoreGroup.AddEvent(uncoreEvent, false)                   // Add the event
					if err != nil {
						return nil, nil, nil, fmt.Errorf("error adding event %s to new uncore group: %w", eventName, err)
					}
				}
			} else {
				otherEvent := otherEvents.FindEventByName(eventName)
				if otherEvent != (OtherEvent{}) { // this is an other event
					if !otherEvent.IsCollectable(metadata) {
						return nil, nil, nil, fmt.Errorf("other event %s is not collectable on target", eventName)
					}
					otherGroup := NewOtherGroup(metadata.NumGeneralPurposeCounters)
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
	if len(coreGroup.FixedPurposeCounters) != 0 || len(coreGroup.GeneralPurposeCounters) != 0 {
		coreGroups = append(coreGroups, coreGroup)
	}
	// if there are any events in the uncore group, add it to the groups
	if len(uncoreGroup.GeneralPurposeCounters) != 0 {
		uncoreGroups = append(uncoreGroups, uncoreGroup)
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
