package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"perfspect/internal/cpus"
	"perfspect/internal/util"
	"regexp"
	"slices"
	"strings"

	"github.com/casbin/govaluate"
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
		archDir, err = getUarchDir(metadata.Microarchitecture)
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

	evaluatorFunctions := getARMEvaluatorFunctions(metadata.ARMCPUID)
	for i := range componentMetricsInFile {
		// a couple ARM metrics don't have MetricExpr, skip those
		// if selectedMetrics is empty, include all metrics
		// otherwise, include only those in selectedMetrics
		// comparison is case insensitive
		if componentMetricsInFile[i].MetricExpr != "" && (len(selectedMetrics) == 0 || util.ContainsIgnoreCase(selectedMetrics, componentMetricsInFile[i].getName())) {
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

func getARMEvaluatorFunctions(CPUID string) map[string]govaluate.ExpressionFunction {
	functions := make(map[string]govaluate.ExpressionFunction)
	functions["strcmp_cpuid_str"] =
		// adapted from: https://elixir.bootlin.com/linux/v6.17-rc4/source/tools/perf/util/expr.c#L468
		func(args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("strcmp_cpuid_str requires one argument")
			}
			argStr, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("strcmp_cpuid_str argument must be a string")
			}
			res, err := compareCPUID(argStr, CPUID)
			if err != nil {
				return nil, err
			}
			// reverse the result to match the expected logic
			if res == 0 {
				return false, nil
			}
			return true, nil
		}
	return functions
}

// compareCPUID compares the argument string to the CPUID
// adapted from: https://elixir.bootlin.com/linux/v6.17-rc4/source/tools/perf/arch/arm64/util/header.c#L91
//
//	Return 0 if idstr is a higher or equal to version of the same part as
//	mapcpuid. Therefore, if mapcpuid has 0 for revision and variant then any
//	version of idstr will match as long as it's the same CPU type.
//
//	Return 1 if the CPU type is different or the version of idstr is lower.
func compareCPUID(mapCpuId string, idStr string) (int, error) {
	mapId, err := util.ParseHex(mapCpuId)
	if err != nil {
		return 0, fmt.Errorf("strcmp_cpuid_str argument must be a valid hex string")
	}
	mapIdVariant := mapId & (0xF << 20)
	mapIdRevision := mapId & 0xF
	id, err := util.ParseHex(idStr)
	if err != nil {
		return 0, fmt.Errorf("strcmp_cpuid_str argument must be a valid hex string")
	}
	idVariant := id & (0xF << 20)
	idRevision := id & 0xF
	// compare without version first
	idFields := ^uint64(0xF | (0xF << 20))
	if mapId&idFields != id&idFields {
		return 1, nil // CPU type is different
	}
	// id matches, now compare version
	if mapIdVariant > idVariant {
		return 0, nil
	}
	if mapIdVariant == idVariant && mapIdRevision >= idRevision {
		return 0, nil
	}
	return 1, nil
}

// loadEventDefinitions -- load all event files in resources/component/<arch>/
// skip metrics.json
func (l *ComponentLoader) loadEventDefinitions(metadata Metadata) (events []ComponentEvent, err error) {
	var archDir string
	archDir, err = getUarchDir(metadata.Microarchitecture)
	if err != nil {
		return nil, err
	}
	resourcesDir := filepath.Join("resources", "component", archDir)
	dirEntries, err := resources.ReadDir(resourcesDir)
	if err != nil {
		return
	}
	// sort for deterministic processing order
	slices.SortFunc(dirEntries, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})
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

		// Get variable names and sort them for deterministic order
		var variables []string
		for variable := range metric.Variables {
			variables = append(variables, variable)
		}
		slices.Sort(variables)

		for _, variable := range variables {
			// confirm variable is a valid event
			if _, exists := eventNames[variable]; !exists {
				slog.Warn("Metric variable does not correspond to a known event, skipping variable", slog.String("metric", metric.Name), slog.String("variable", variable))
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

	// merge small groups
	groups = mergeSmallGroups(groups, numGPCounters)

	return groups, nil
}

// mergeSmallGroups merges groups that have few events, ensuring that the merged group does not exceed numGPCounters
// CPU_CYCLES is a fixed counter and does not count against the numGPCounters limit
// events in a group are unique (no duplicates)
// all events in a group must be merged together
func mergeSmallGroups(groups []GroupDefinition, numGPCounters int) []GroupDefinition {
	// If there are 1 or 0 groups, no merging is needed
	if len(groups) <= 1 {
		return groups
	}

	// Sort groups by size for efficient merging (smallest first)
	// Important that this is a deterministic sort
	slices.SortFunc(groups, func(a, b GroupDefinition) int {
		if len(a) == 0 || len(b) == 0 {
			panic("empty group encountered during sorting in mergeSmallGroups")
		}
		aGPCount := countGPEvents(a)
		bGPCount := countGPEvents(b)
		if aGPCount != bGPCount {
			return aGPCount - bGPCount
		}
		return 1 // arbitrary but consistent
	})
	var mergedGroups []GroupDefinition
	processed := make([]bool, len(groups))

	// Process groups in increasing order of size
	for i := range groups {
		if processed[i] {
			continue
		}

		currentGroup := groups[i]
		currentGPCount := countGPEvents(currentGroup)
		processed[i] = true

		// Try to merge with other unprocessed groups
		for j := i + 1; j < len(groups); j++ {
			if processed[j] {
				continue
			}

			candidateGroup := groups[j]

			// Calculate how many new unique GP events would be added by this merge
			uniqueGPEventsToAdd := countUniqueGPEventsToAdd(currentGroup, candidateGroup)

			// Check if merging would exceed the GP counter limit
			if currentGPCount+uniqueGPEventsToAdd > numGPCounters {
				continue
			}

			// Merge the groups
			mergedGroup := mergeGroupsWithoutDuplicates(currentGroup, candidateGroup)
			// Continue with the merge if we successfully added events
			currentGroup = mergedGroup
			currentGPCount = countGPEvents(currentGroup)
			processed[j] = true
		}

		mergedGroups = append(mergedGroups, currentGroup)
	}

	return mergedGroups
}

// countGPEvents counts the number of general purpose events (excluding CPU_CYCLES)
func countGPEvents(group GroupDefinition) int {
	count := 0
	for _, event := range group {
		if event.Name != "CPU_CYCLES" {
			count++
		}
	}
	return count
}

// countUniqueGPEventsToAdd counts how many new unique GP events would be added
// when merging target into source (excluding CPU_CYCLES and already existing events)
func countUniqueGPEventsToAdd(source, target GroupDefinition) int {
	// Create a map of events that already exist in source
	existing := make(map[string]bool)
	for _, event := range source {
		existing[event.Name] = true
	}

	// Count unique GP events in target that don't exist in source
	count := 0
	for _, event := range target {
		if !existing[event.Name] && event.Name != "CPU_CYCLES" {
			count++
		}
	}
	return count
}

// mergeGroupsWithoutDuplicates merges two groups into one, avoiding duplicates
func mergeGroupsWithoutDuplicates(a, b GroupDefinition) GroupDefinition {
	merged := make(GroupDefinition, len(a))
	copy(merged, a)

	// Track existing event names
	existing := make(map[string]bool)
	for _, event := range merged {
		existing[event.Name] = true
	}

	// Add non-duplicate events from group b
	for _, event := range b {
		if !existing[event.Name] {
			merged = append(merged, event)
			existing[event.Name] = true
		}
	}

	return merged
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
	transformedExpression := strings.ReplaceAll(expression, "#slots", fmt.Sprintf("%d", metadata.ARMSlots))
	// replace if else with ?:
	transformedExpression, err := transformExpression(transformedExpression)
	if err != nil {
		slog.Error("Failed to transform expression", slog.String("expression", transformedExpression), slog.String("error", err.Error()))
		return nil
	}
	// govaluate doesn't like: strcmp_cpuid_str(0x410fd493)
	// "Cannot transition token types from NUMERIC [0] to VARIABLE [x410fd493]"
	// quote the hex number
	// look for (0x....) and replace with ("0x....")
	rxHex := regexp.MustCompile(`\((0x[0-9a-fA-F]+)\)`)
	transformedExpression = rxHex.ReplaceAllString(transformedExpression, `("$1")`)
	// govaluate doesn't like: strcmp_cpuid_str(0x410fd490) ^ 1
	// so we replace it with !strcmp_cpuid_str(0x410fd490)
	rxInvertedStrcmp := regexp.MustCompile(`strcmp_cpuid_str\("0x[0-9a-fA-F]+"\)\s*\^\s*1`)
	transformedExpression = rxInvertedStrcmp.ReplaceAllStringFunc(transformedExpression, func(match string) string {
		// Extract the argument inside strcmp_cpuid_str(...)
		rxArg := regexp.MustCompile(`strcmp_cpuid_str\("0x[0-9a-fA-F]+"\)`)
		argMatch := rxArg.FindString(match)
		if argMatch != "" {
			return "!" + argMatch
		}
		return match // Should not happen, but return the original match if extraction fails
	})

	if transformedExpression != expression {
		slog.Debug("Transformed metric expression", slog.String("original", expression), slog.String("transformed", transformedExpression))
	}

	// create govaluate expression
	expr, err := govaluate.NewEvaluableExpressionWithFunctions(transformedExpression, evaluatorFunctions)
	if err != nil {
		slog.Error("Failed to parse metric expression", slog.String("expression", transformedExpression), slog.String("error", err.Error()))
		return nil
	}
	return expr
}

// getUarchDir maps from the CPU's microarchitecture, as defined in
// the cpus module, to the directory where the associated events and metrics reside
func getUarchDir(uarch string) (string, error) {
	switch uarch {
	case cpus.UarchGraviton4, cpus.UarchAxion, cpus.UarchAmpereOneAC04, cpus.UarchAmpereOneAC04_1:
		return "neoverse-n2-v2", nil
	case cpus.UarchGraviton2:
		return "neoverse-n1", nil
	case cpus.UarchGraviton3:
		return "neoverse-v1", nil
	}
	return "", fmt.Errorf("unsupported component loader architecture: %s", uarch)
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
