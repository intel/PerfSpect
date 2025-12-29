// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
)

func (l *LegacyLoader) Load(loaderConfig LoaderConfig) ([]MetricDefinition, []GroupDefinition, error) {
	loadedMetricDefinitions, err := l.loadMetricDefinitions(loaderConfig.MetricDefinitionOverride, loaderConfig.SelectedMetrics, loaderConfig.Metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load metric definitions: %w", err)
	}
	loadedEventGroups, uncollectableEvents, err := l.loadEventGroups(loaderConfig.EventDefinitionOverride, loaderConfig.Metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load event group definitions: %w", err)
	}
	configuredMetricDefinitions, err := configureMetrics(loadedMetricDefinitions, uncollectableEvents, loaderConfig.Metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to configure metrics: %w", err)
	}
	return configuredMetricDefinitions, loadedEventGroups, nil
}

// getUarchFileName maps the CPU's microarchitecture, as defined in the cpus
// module, to the resource file name used by the legacy loader
func getUarchFileName(uarch string) string {
	filename := strings.ToLower(uarch)
	filename = strings.Split(filename, " ")[0] // Handle "Turin (Zen 5)" case
	return filename
}

// loadMetricDefinitions reads and parses metric definitions from an architecture-specific metric
// definition file. When the override path argument is empty, the function will load metrics from
// the file associated with the platform's architecture found in the provided metadata. When
// a list of metric names is provided, only those metric definitions will be loaded.
func (l *LegacyLoader) loadMetricDefinitions(metricDefinitionOverridePath string, selectedMetrics []string, metadata Metadata) (metrics []MetricDefinition, err error) {
	var bytes []byte
	if metricDefinitionOverridePath != "" {
		bytes, err = os.ReadFile(metricDefinitionOverridePath) // #nosec G304
		if err != nil {
			return
		}
	} else {
		metricFileName := fmt.Sprintf("%s.json", getUarchFileName(metadata.Microarchitecture))
		if bytes, err = resources.ReadFile(filepath.Join("resources", "legacy", "metrics", metadata.Architecture, metadata.Vendor, metricFileName)); err != nil {
			return
		}
	}
	var metricsInFile []MetricDefinition
	if err = json.Unmarshal(bytes, &metricsInFile); err != nil {
		return
	}
	// set LegacyName to Name for all metrics
	for i := range metricsInFile {
		metricsInFile[i].LegacyName = metricsInFile[i].Name
	}
	// if a list of metric names provided, reduce list to match
	if len(selectedMetrics) > 0 {
		// confirm provided metric names are valid (included in metrics defined in file)
		// and build list of metrics based on provided list of metric names
		metricMap := make(map[string]MetricDefinition)
		for _, metric := range metricsInFile {
			metricMap[strings.ToLower(metric.Name)] = metric
		}
		for _, selectedMetricName := range selectedMetrics {
			if _, ok := metricMap[strings.ToLower(selectedMetricName)]; !ok {
				err = fmt.Errorf("provided metric name not found: %s", selectedMetricName)
				return
			}
			metrics = append(metrics, metricMap[strings.ToLower(selectedMetricName)])
		}
	} else {
		metrics = metricsInFile
	}
	return
}

// loadEventGroups reads the events defined in the architecture specific event definition file, then
// expands them to include the per-device uncore events
func (l *LegacyLoader) loadEventGroups(eventDefinitionOverridePath string, metadata Metadata) (groups []GroupDefinition, uncollectableEvents []string, err error) {
	var file fs.File
	if eventDefinitionOverridePath != "" {
		file, err = os.Open(eventDefinitionOverridePath) // #nosec G304
		if err != nil {
			return
		}
	} else {
		eventFileName := fmt.Sprintf("%s.txt", getUarchFileName(metadata.Microarchitecture))
		if file, err = resources.Open(filepath.Join("resources", "legacy", "events", metadata.Architecture, metadata.Vendor, eventFileName)); err != nil {
			return
		}
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	uncollectable := mapset.NewSet[string]()
	var group GroupDefinition
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		// strip end of line comment
		if idx := strings.Index(line, "#"); idx != -1 {
			line = line[:idx]
		}
		// remove trailing spaces
		line = strings.TrimSpace(line)
		var event EventDefinition
		if event, err = parseEventDefinition(line[:len(line)-1]); err != nil {
			return
		}
		// check if the event is collectable
		if isCollectableEvent(event, metadata) {
			group = append(group, event)
		} else {
			uncollectable.Add(event.Name)
		}
		if line[len(line)-1] == ';' {
			// end of group detected
			if len(group) > 0 {
				groups = append(groups, group)
			} else {
				slog.Debug("No collectable events in group", slog.String("ending", line))
			}
			group = GroupDefinition{} // clear the list
		}
	}
	if err = scanner.Err(); err != nil {
		return
	}
	uncollectableEvents = uncollectable.ToSlice()

	if uncollectable.Cardinality() != 0 {
		slog.Debug("Events not collectable on target", slog.String("events", uncollectable.String()))
	}
	return
}

// isCollectableEvent confirms if given event can be collected on the platform
func isCollectableEvent(event EventDefinition, metadata Metadata) bool {
	// fixed-counter TMA
	if !metadata.SupportsFixedTMA && (event.Name == "TOPDOWN.SLOTS" || strings.HasPrefix(event.Name, "PERF_METRICS.")) {
		slog.Debug("Fixed counter TMA not supported on target", slog.String("event", event.Name))
		return false
	}
	// short-circuit for cpu events that aren't off-core response events
	if event.Device == "cpu" && !(strings.HasPrefix(event.Name, "OCR") || strings.HasPrefix(event.Name, "OFFCORE_REQUESTS_OUTSTANDING")) {
		return true
	}
	// off-core response events
	if event.Device == "cpu" && (strings.HasPrefix(event.Name, "OCR") || strings.HasPrefix(event.Name, "OFFCORE_REQUESTS_OUTSTANDING")) {
		if !(metadata.SupportsOCR && metadata.SupportsUncore) {
			slog.Debug("Off-core response events not supported on target", slog.String("event", event.Name))
			return false
		} else if flagScope == scopeProcess || flagScope == scopeCgroup {
			slog.Debug("Off-core response events not supported in process or cgroup scope", slog.String("event", event.Name))
			return false
		}
		return true
	}
	// uncore events
	// if using CPU granularity, don't collect uncore events
	if flagGranularity == granularityCPU && strings.HasPrefix(event.Name, "UNC") {
		slog.Debug("Uncore events not supported with specified granularity", slog.String("event", event.Name))
		return false
	}
	// if uncore metrics not supported, don't collect uncore events
	if !metadata.SupportsUncore && strings.HasPrefix(event.Name, "UNC") {
		slog.Debug("Uncore events not supported on target", slog.String("event", event.Name))
		return false
	}
	// exclude uncore events when
	// - their corresponding device is not found
	// - not in system-wide collection scope
	if event.Device != "cpu" && event.Device != "" {
		if flagScope == scopeProcess || flagScope == scopeCgroup {
			slog.Debug("Uncore events not supported in process or cgroup scope", slog.String("event", event.Name))
			return false
		}
		deviceExists := false
		for uncoreDeviceName := range metadata.UncoreDeviceIDs {
			if event.Device == uncoreDeviceName {
				deviceExists = true
				break
			}
		}
		if !deviceExists {
			slog.Debug("Uncore device not found", slog.String("device", event.Device))
			return false
		} else if !strings.Contains(event.Raw, "umask") && !strings.Contains(event.Raw, "event") {
			slog.Debug("Uncore event missing umask or event", slog.String("event", event.Name))
			return false
		}
		return true
	}
	// if we got this far, event.Device is empty
	// is ref-cycles supported?
	if !metadata.SupportsRefCycles && strings.Contains(event.Name, "ref-cycles") {
		slog.Debug("ref-cycles not supported on target", slog.String("event", event.Name))
		return false
	}
	// no cstate and power events when collecting at process or cgroup scope
	if (flagScope == scopeProcess || flagScope == scopeCgroup) &&
		(strings.Contains(event.Name, "cstate_") || strings.Contains(event.Name, "power/energy")) {
		slog.Debug("Cstate and power events not supported in process or cgroup scope", slog.String("event", event.Name))
		return false
	}
	// no system-level events when collecting at CPU granularity e.g. power, cstates
	if (flagGranularity == granularityCPU) &&
		(strings.Contains(event.Name, "power/energy") || strings.Contains(event.Name, "cstate_pkg")) {
		slog.Debug("Power events not supported in CPU granularity", slog.String("event", event.Name))
		return false
	}
	// finally, if it isn't in the perf list output, it isn't collectable
	name := strings.Split(event.Name, ":")[0]
	if !strings.Contains(metadata.PerfSupportedEvents, name) {
		slog.Debug("Event not supported by perf", slog.String("event", name))
		return false
	}
	return true
}

// parseEventDefinition parses one line from the event definition file into a representative structure
func parseEventDefinition(line string) (eventDef EventDefinition, err error) {
	eventDef.Raw = line
	fields := strings.Split(line, ",")
	if len(fields) == 1 {
		eventDef.Name = fields[0]
	} else if len(fields) > 1 {
		nameField := fields[len(fields)-1]
		if nameField[:5] != "name=" {
			err = fmt.Errorf("unrecognized event format, name field not found: %s", line)
			return
		}
		eventDef.Name = nameField[6 : len(nameField)-2]
		eventDef.Device = strings.Split(fields[0], "/")[0]
	} else {
		err = fmt.Errorf("unrecognized event format: %s", line)
		return
	}
	return
}
