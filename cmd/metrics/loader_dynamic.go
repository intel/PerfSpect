package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"perfspect/internal/util"
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
	Name   string `json:"Name"`
	Origin string `json:"Origin"`
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
	// Create event groups from the perfspect metrics
	coreGroups, uncoreGroups, otherGroups, err := loadEventGroupsFromMetrics(
		config.ReportMetrics,
		perfmonMetricDefinitions,
		config,
		coreEvents,
		uncoreEvents,
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

	return metrics, allGroups, nil
}

func loadMetricsFromDefinitions(perfmonMetrics PerfmonMetrics, metricsConfig MetricsConfig) ([]MetricDefinition, error) {
	var metrics []MetricDefinition
	for _, includedMetric := range metricsConfig.ReportMetrics {
		switch strings.ToLower(includedMetric.Origin) {
		case "perfmon":
			for _, perfmonMetric := range perfmonMetrics.Metrics {
				if perfmonMetric.LegacyName == includedMetric.Name {
					metric := MetricDefinition{
						Name:        includedMetric.Name,
						Description: perfmonMetric.BriefDescription,
						Expression:  perfmonMetric.Formula,
					}
					metrics = append(metrics, metric)
					break
				}
			}
		case "perfspect":
			for _, perfspectMetric := range metricsConfig.Metrics {
				if perfspectMetric.LegacyName == includedMetric.Name {
					metric := MetricDefinition{
						Name:        includedMetric.Name,
						Description: perfspectMetric.BriefDescription,
						Expression:  perfspectMetric.Formula,
					}
					metrics = append(metrics, metric)
					break
				}
			}
		default:
			return nil, fmt.Errorf("unknown metric origin: %s for metric: %s", includedMetric.Origin, includedMetric.Name)
		}
	}
	return metrics, nil
}

func loadEventGroupsFromMetrics(includedMetrics []PerfspectMetric, perfmonMetrics PerfmonMetrics, config MetricsConfig, coreEvents CoreEvents, uncoreEvents UncoreEvents, metadata Metadata) ([]CoreGroup, []UncoreGroup, []OtherGroup, error) {
	coreGroups := make([]CoreGroup, 0)
	uncoreGroups := make([]UncoreGroup, 0)
	otherGroups := make([]OtherGroup, 0)
	for _, perfspectMetric := range includedMetrics {
		var metricEventNames []string
		if perfspectMetric.Origin == "perfmon" {
			// find this metric name in the perfmon metrics
			var perfmonMetric *PerfmonMetric
			found := false
			for _, metric := range perfmonMetrics.Metrics {
				if metric.LegacyName == perfspectMetric.Name {
					perfmonMetric = &metric
					found = true
					break
				}
			}
			if !found {
				return nil, nil, nil, fmt.Errorf("metric %s not found in perfmon metrics", perfspectMetric.Name)
			}
			for _, event := range perfmonMetric.Events {
				metricEventNames = append(metricEventNames, event["Name"])
			}
		} else if perfspectMetric.Origin == "perfspect" {
			// find the metric name in the perfspect metrics
			var perfmonMetric *PerfmonMetric
			found := false
			for _, metric := range config.Metrics {
				if metric.LegacyName == perfspectMetric.Name {
					perfmonMetric = &metric
					found = true
					break
				}
			}
			if !found {
				return nil, nil, nil, fmt.Errorf("metric %s not found in perfspect metrics", perfspectMetric.Name)
			}
			for _, event := range perfmonMetric.Events {
				metricEventNames = append(metricEventNames, event["Name"])
			}
		} else {
			return nil, nil, nil, fmt.Errorf("unknown origin for metric %s: %s", perfspectMetric.Name, perfspectMetric.Origin)
		}
		metricCoreGroups, metricUncoreGroups, metricOtherGroups, err := groupsFromEventNames(
			perfspectMetric.Name,
			metricEventNames,
			coreEvents,
			uncoreEvents,
			metadata,
		)
		if err != nil {
			fmt.Printf("Error grouping events for metric %s: %v\n", perfspectMetric.Name, err)
			continue
		}
		coreGroups = append(coreGroups, metricCoreGroups...)
		uncoreGroups = append(uncoreGroups, metricUncoreGroups...)
		otherGroups = append(otherGroups, metricOtherGroups...)
	}
	return coreGroups, uncoreGroups, otherGroups, nil
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

func (md Metadata) isCollectableCoreEvent(event CoreEvent) bool {
	if !md.SupportsFixedTMA && (strings.HasPrefix(event.EventName, "TOPDOWN.SLOTS") || strings.HasPrefix(event.EventName, "PERF_METRICS")) {
		return false // TOPDOWN.SLOTS and PERF_METRICS.* events are not supported
	}
	pebsEventNames := []string{"INT_MISC.UNKNOWN_BRANCH_CYCLES", "UOPS_RETIRED.MS"}
	if !md.SupportsPEBS {
		for _, pebsEventName := range pebsEventNames {
			if strings.Contains(event.EventName, pebsEventName) {
				return false // PEBS events are not supported
			}
		}
	}
	if !md.SupportsOCR && (strings.HasPrefix(event.EventName, "OCR") || strings.HasPrefix(event.EventName, "OFFCORE_REQUESTS_OUTSTANDING")) {
		return false // OCR events are not supported
	}
	if !md.SupportsRefCycles && strings.Contains(event.EventName, "ref-cycles") {
		return false // ref-cycles events are not supported
	}
	return true
}

func (md Metadata) isCollectableUncoreEvent(event UncoreEvent) bool {
	if !md.SupportsUncore {
		return false // uncore events are not supported
	}
	deviceExists := false
	for uncoreDeviceName := range md.UncoreDeviceIDs {
		if strings.EqualFold(strings.Split(event.Unit, " ")[0], uncoreDeviceName) {
			deviceExists = true
			break
		}
	}
	if !deviceExists {
		fmt.Printf("Warning: Uncore event %s has unit %s, but no such uncore device found in target capabilities\n", event.EventName, event.Unit)
		return false // uncore device not found
	}
	return true
}

func (md Metadata) isSupportedPerfEvent(eventName string) bool {
	// Check if the event name is in the list of supported perf event names
	return strings.Contains(md.PerfSupportedEvents, eventName)
}

var constants []string = []string{
	"TSC",
}

func groupsFromEventNames(metricName string, eventNames []string, coreEvents CoreEvents, uncoreEvents UncoreEvents, metadata Metadata) ([]CoreGroup, []UncoreGroup, []OtherGroup, error) {
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
			if !metadata.isCollectableCoreEvent(coreEvent) {
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
				if !metadata.isCollectableUncoreEvent(uncoreEvent) {
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
				// not a core event or uncore event
				// TODO: load these from file
				uncategorizedEvents := []string{
					"power/energy-pkg/",
					"power/energy-ram/",
					"cstate_core/c6-residency/",
					"cstate_pkg/c6-residency/",
				}
				if metadata.isSupportedPerfEvent(eventName) {
					for _, uncategorizedEvent := range uncategorizedEvents {
						if strings.Contains(eventName, uncategorizedEvent) {
							// add a group for this uncategorized event
							otherGroup := NewOtherGroup(metadata.NumGeneralPurposeCounters)
							otherGroup.MetricNames = util.UniqueAppend(otherGroup.MetricNames, metricName)
							err := otherGroup.AddEvent(OtherEvent{EventName: uncategorizedEvent}, false)
							if err != nil {
								fmt.Printf("Failed to add other event %s to group for metric %s: %v\n", uncategorizedEvent, metricName, err)
							} else {
								otherGroups = append(otherGroups, otherGroup)
							}

						}
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
