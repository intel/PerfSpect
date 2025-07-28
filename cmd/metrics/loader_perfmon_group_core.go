package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"io"
	"maps"
	"perfspect/internal/util"
	"slices"
	"strings"
)

type CoreGroup struct {
	MaxGeneralPurposeCounters int
	GeneralPurposeCounters    map[int]CoreEvent
	FixedPurposeCounters      []CoreEvent
	MetricNames               []string // List of metric names that this group represents
}

func NewCoreGroup(metadata Metadata) CoreGroup {
	// initialize core group by setting up the fixed purpose counters
	var fixedCounters []CoreEvent
	if metadata.SupportsFixedCycles {
		fixedCounters = append(fixedCounters, CoreEvent{EventName: "CPU_CLK_UNHALTED.THREAD", EventCode: "0x00", UMask: "0x02", SampleAfterValue: "2000003"})
	}
	if metadata.SupportsFixedInstructions {
		fixedCounters = append(fixedCounters, CoreEvent{EventName: "INST_RETIRED.ANY", EventCode: "0x00", UMask: "0x01", SampleAfterValue: "2000003"})
	}
	if metadata.SupportsFixedRefCycles {
		fixedCounters = append(fixedCounters, CoreEvent{EventName: "CPU_CLK_UNHALTED.REF_TSC", EventCode: "0x00", UMask: "0x03", SampleAfterValue: "2000003"})
	}
	return CoreGroup{
		MaxGeneralPurposeCounters: metadata.NumGeneralPurposeCounters,
		FixedPurposeCounters:      fixedCounters,
		GeneralPurposeCounters:    make(map[int]CoreEvent, 0),
		MetricNames:               make([]string, 0),
	}
}

func NewCoreGroupWithTMAEvents(metadata Metadata) CoreGroup {
	group := NewCoreGroup(metadata)
	// prepend TMA events to the fixed purpose counters
	tmaEvents := []CoreEvent{
		{EventName: "TOPDOWN.SLOTS:perf_metrics", EventCode: "0x00", UMask: "0x04", SampleAfterValue: "10000003"},
	}
	tmaEvents = append(tmaEvents, perfMetricsEvents...)
	group.FixedPurposeCounters = append(tmaEvents, group.FixedPurposeCounters...)
	return group
}

func (group CoreGroup) ToGroupDefinition() GroupDefinition {
	// Convert the CoreGroup to a GroupDefinition
	groupDef := make(GroupDefinition, 0)
	// Add fixed purpose counters
	for _, event := range group.FixedPurposeCounters {
		raw, err := event.StringForPerf()
		if err != nil {
			fmt.Printf("Error formatting event %s for perf: %v\n", event.EventName, err)
			continue
		}
		groupDef = append(groupDef, EventDefinition{
			Raw:  raw,
			Name: event.EventName,
		})
	}
	// Add general purpose counters
	for _, event := range group.GeneralPurposeCounters {
		raw, err := event.StringForPerf()
		if err != nil {
			fmt.Printf("Error formatting event %s for perf: %v\n", event.EventName, err)
			continue
		}
		groupDef = append(groupDef, EventDefinition{
			Raw:  raw,
			Name: event.EventName,
		})
	}
	return groupDef
}

func (group CoreGroup) FindEventByName(eventName string) CoreEvent {
	for _, event := range group.FixedPurposeCounters {
		if event.EventName == eventName {
			return event // Event found in fixed purpose counters
		}
	}
	for _, event := range group.GeneralPurposeCounters {
		if event.EventName == eventName {
			return event // Event found in general purpose counters
		}
	}
	// If we reach here, the event was not found in any of the counters
	return CoreEvent{}
}

func (group CoreGroup) Equal(other CoreGroup) bool {
	if group.MaxGeneralPurposeCounters != other.MaxGeneralPurposeCounters {
		return false // Different max GP events
	}
	if len(group.FixedPurposeCounters) != len(other.FixedPurposeCounters) {
		return false // Different number of fixed purpose counters
	}
	// order/placement of fixed purpose counters is important
	for i, event := range group.FixedPurposeCounters {
		if event != other.FixedPurposeCounters[i] {
			return false // Fixed purpose counter differs
		}
	}
	if len(group.GeneralPurposeCounters) != len(other.GeneralPurposeCounters) {
		return false // Different number of general purpose counters
	}
	// order of general purpose counters is not important
	for _, event := range group.GeneralPurposeCounters {
		// if event is not in the other group, they are not equal
		if otherEvent := other.FindEventByName(event.EventName); otherEvent == (CoreEvent{}) {
			return false // Event not found in other group
		}
	}
	return true // All checks passed, groups are equal
}

func (group CoreGroup) Copy() CoreGroup {
	newGroup := CoreGroup{}
	newGroup.GeneralPurposeCounters = make(map[int]CoreEvent, len(group.GeneralPurposeCounters))
	newGroup.FixedPurposeCounters = make([]CoreEvent, len(group.FixedPurposeCounters))
	newGroup.MetricNames = make([]string, len(group.MetricNames))
	newGroup.MaxGeneralPurposeCounters = group.MaxGeneralPurposeCounters
	copy(newGroup.MetricNames, group.MetricNames)
	copy(newGroup.FixedPurposeCounters, group.FixedPurposeCounters)
	maps.Copy(newGroup.GeneralPurposeCounters, group.GeneralPurposeCounters)
	return newGroup
}

func (group CoreGroup) CanMerge(other CoreGroup) bool {
	// Check if the groups have the same max GP events
	if group.MaxGeneralPurposeCounters != other.MaxGeneralPurposeCounters {
		return false
	}
	// Check if all events in other can be added to this group
	// Create a copy of group so we do not alter this group in this check
	tempGroup := group.Copy()
	for _, event := range other.FixedPurposeCounters {
		if err := tempGroup.AddEvent(event, false); err != nil {
			return false // Cannot add fixed purpose counter from other group
		}
	}
	for _, event := range other.GeneralPurposeCounters {
		if err := tempGroup.AddEvent(event, true); err != nil {
			return false // Cannot add general purpose counter from other group
		}
	}
	return true
}

func (group CoreGroup) HasTakenAloneEvent() bool {
	// Check if the group has an event tagged with "TakenAlone"
	for _, counter := range group.GeneralPurposeCounters {
		if counter.TakenAlone == "true" {
			return true
		}
	}
	return false
}

func (group CoreGroup) AddEvent(event CoreEvent, reorder bool) error {
	// If the event is already in the group, no need to insert it again
	if group.FindEventByName(event.EventName) != (CoreEvent{}) {
		return nil
	}
	// check if group already has an event that is tagged with "TakenAlone"
	if event.TakenAlone == "true" && group.HasTakenAloneEvent() {
		return fmt.Errorf("group already has an event tagged with TakenAlone, cannot add %s", event.EventName)
	}
	// only 2 offcore events are allowed in a group
	if event.Offcore == "1" {
		offcoreCount := 0
		for _, existingEvent := range group.GeneralPurposeCounters {
			if existingEvent.Offcore == "1" {
				offcoreCount++
			}
		}
		if offcoreCount >= 2 {
			return fmt.Errorf("group already has two OCR events, cannot add %s", event.EventName)
		}
	}
	// ignore TMA events, they are in there own special core group
	if strings.HasPrefix(event.EventName, "PERF_METRICS.") || event.EventName == "TOPDOWN.SLOTS:perf_metrics" {
		return nil
	}
	// get the list of valid counters for this event
	validCounters := event.Counter
	if validCounters == "" {
		return fmt.Errorf("event %s has no valid counters defined", event.EventName)
	}
	// does this event require a fixed counter that isn't already in the group?
	// this should only happen if the ":k" variant is being added
	if strings.HasPrefix(validCounters, "Fixed counter") {
		// find the non :k variant of the event name
		baseEventName := strings.TrimSuffix(event.EventName, ":k")
		for i, fixedEvent := range group.FixedPurposeCounters {
			if fixedEvent.EventName == baseEventName {
				group.FixedPurposeCounters[i] = event // replace the fixed purpose counter with the new event
				return nil
			}
		}
		// if we reach here, the fixed purpose counter was not found, that shouldn't happen
		return fmt.Errorf("fixed purpose counter for %s not found in group", baseEventName)
	} else {
		// otherwise, it is a general purpose event, check if we can place it in one of the general purpose counters
		if len(group.GeneralPurposeCounters) >= group.MaxGeneralPurposeCounters {
			return fmt.Errorf("group already has maximum number of general purpose events (%d), cannot add %s", group.MaxGeneralPurposeCounters, event.EventName)
		}
		for i := 0; i < group.MaxGeneralPurposeCounters; i++ {
			if _, ok := group.GeneralPurposeCounters[i]; !ok {
				// this counter is empty, check if it is a valid counter for this event
				counterIndex := fmt.Sprintf("%d", i)
				if strings.Contains(validCounters, counterIndex) {
					group.GeneralPurposeCounters[i] = event // place the event in this counter
					return nil
				}
			}
		}
		if reorder {
			// check if we can move an event that's already in the group to make room for the new event
			for counter, existingEvent := range group.GeneralPurposeCounters {
				// check if the new event can be placed in the current counter
				if !strings.Contains(validCounters, fmt.Sprintf("%d", counter)) {
					continue // not a valid counter for this event
				}
				// check if the existing event can be moved to another unoccupied counter
				for otherCounter := 0; otherCounter < group.MaxGeneralPurposeCounters; otherCounter++ {
					if otherCounter == counter {
						continue // skip the current counter
					}
					if _, ok := group.GeneralPurposeCounters[otherCounter]; ok {
						continue // skip occupied counters
					}
					// check if the existing event is compatible with the other counter
					if !strings.Contains(existingEvent.Counter, fmt.Sprintf("%d", otherCounter)) {
						continue // not a valid counter for this event
					}
					// we can move the event to a different counter
					fmt.Printf("Moving %s [%s] from counter %d to %d for %s [%s]\n", existingEvent.EventName, existingEvent.Counter, counter, otherCounter, event.EventName, event.Counter)
					group.GeneralPurposeCounters[otherCounter] = existingEvent // move the existing event to the new counter
					group.GeneralPurposeCounters[counter] = event              // place the new event in the current counter
					return nil
				}
			}
		}
	}
	// If we reach here, we couldn't find a valid counter for the event
	return fmt.Errorf("no counter available for %s: %s", event.EventName, event.Counter)
}

func (group CoreGroup) Print(w io.Writer) {
	fmt.Fprintf(w, "  Metric Names: %s\n", strings.Join(group.MetricNames, ", "))
	fmt.Fprintln(w, "  Fixed Purpose Counters:")
	for i, event := range group.FixedPurposeCounters {
		fmt.Fprintf(w, "    Counter %d: %s [%s]\n", i, event.EventName, event.Counter)
	}
	fmt.Fprintln(w, "  General Purpose Counters:")
	// print sorted by keys
	keys := make([]int, 0, len(group.GeneralPurposeCounters))
	for k := range group.GeneralPurposeCounters {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		event := group.GeneralPurposeCounters[k]
		fmt.Fprintf(w, "    Counter %d: %s [%s]\n", k, event.EventName, event.Counter)
	}
}

func (group CoreGroup) StringForPerf() (string, error) {
	var formattedEvents []string
	// add the fixed purpose counters first
	for _, event := range group.FixedPurposeCounters {
		// Format the event for perf
		formattedEvent, err := event.StringForPerf()
		if err != nil {
			return "", fmt.Errorf("error formatting event %s for perf: %w", event.EventName, err)
		}
		// Add the formatted event to the list
		formattedEvents = append(formattedEvents, formattedEvent)
	}
	// sort the events by their counter number
	keys := make([]int, 0, len(group.GeneralPurposeCounters))
	for k := range group.GeneralPurposeCounters {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	// now add the general purpose counters in sorted order
	for _, k := range keys {
		event := group.GeneralPurposeCounters[k]
		// Format the event for perf
		formattedEvent, err := event.StringForPerf()
		if err != nil {
			return "", fmt.Errorf("error formatting event %s for perf: %w", event.EventName, err)
		}
		// Add the formatted event to the list
		formattedEvents = append(formattedEvents, formattedEvent)
	}
	if len(formattedEvents) == 0 {
		return "", fmt.Errorf("no valid events found in group")
	}
	return fmt.Sprintf("{%s}", strings.Join(formattedEvents, ",")), nil
}

func MergeCoreGroups(coreGroups []CoreGroup) ([]CoreGroup, error) {
	for i := 0; i < len(coreGroups); i++ {
		for j := i + 1; j < len(coreGroups); j++ {
			if coreGroups[i].CanMerge(coreGroups[j]) {
				fmt.Printf("Merging core group %d into group %d\n", j, i)
				// merge the metric names
				coreGroups[i].MetricNames = util.UniqueAppend(coreGroups[i].MetricNames, coreGroups[j].MetricNames...)
				// merge the events
				for _, event := range coreGroups[j].FixedPurposeCounters {
					err := coreGroups[i].AddEvent(event, true)
					if err != nil {
						return nil, fmt.Errorf("error adding event %s to group %d: %w", event.EventName, i, err)
					}
				}
				for _, event := range coreGroups[j].GeneralPurposeCounters {
					err := coreGroups[i].AddEvent(event, true)
					if err != nil {
						return nil, fmt.Errorf("error adding event %s to group %d: %w", event.EventName, i, err)
					}
				}
				// remove the group at index j
				coreGroups = append(coreGroups[:j], coreGroups[j+1:]...)
				j-- // adjust index since we removed an element
			}
		}
	}
	return coreGroups, nil
}

func EliminateDuplicateCoreGroups(coreGroups []CoreGroup) ([]CoreGroup, error) {
	// if two groups have the same events, merge them into one group
	// combine the metric names of the groups
	for i := 0; i < len(coreGroups); i++ {
		for j := i + 1; j < len(coreGroups); j++ {
			if coreGroups[i].Equal(coreGroups[j]) {
				fmt.Printf("Found duplicate core group %d and %d\n", i, j)
				// merge the metric names
				coreGroups[i].MetricNames = util.UniqueAppend(coreGroups[i].MetricNames, coreGroups[j].MetricNames...)
				// remove the group at index j
				coreGroups = append(coreGroups[:j], coreGroups[j+1:]...)
				j-- // adjust index since we removed an element
			}
		}
	}
	return coreGroups, nil
}
