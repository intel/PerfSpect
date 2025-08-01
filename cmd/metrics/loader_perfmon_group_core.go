package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"io"
	"perfspect/internal/util"
	"strconv"
	"strings"
)

type CoreGroup struct {
	GeneralPurposeCounters []CoreEvent
	FixedPurposeCounters   []CoreEvent
	MetricNames            []string // List of metric names that this group represents
}

func NewCoreGroup(metadata Metadata) CoreGroup {
	// initialize core group by setting up the fixed purpose counters
	var fixedCounters []CoreEvent
	// if metadata.SupportsFixedCycles {
	// 	fixedCounters = append(fixedCounters, CoreEvent{EventName: "CPU_CLK_UNHALTED.THREAD", EventCode: "0x00", UMask: "0x02", SampleAfterValue: "2000003", Counter: "Fixed counter 0"})
	// }
	// if metadata.SupportsFixedInstructions {
	// 	fixedCounters = append(fixedCounters, CoreEvent{EventName: "INST_RETIRED.ANY", EventCode: "0x00", UMask: "0x01", SampleAfterValue: "2000003", Counter: "Fixed counter 1"})
	// }
	// if metadata.SupportsFixedRefCycles {
	// 	fixedCounters = append(fixedCounters, CoreEvent{EventName: "CPU_CLK_UNHALTED.REF_TSC", EventCode: "0x00", UMask: "0x03", SampleAfterValue: "2000003", Counter: "Fixed counter 2"})
	// }
	return CoreGroup{
		FixedPurposeCounters:   fixedCounters,
		GeneralPurposeCounters: make([]CoreEvent, metadata.NumGeneralPurposeCounters),
		MetricNames:            make([]string, 0),
	}
}

func NewCoreGroupWithTMAEvents(metadata Metadata) CoreGroup {
	group := NewCoreGroup(metadata)
	// prepend TMA events to the fixed purpose counters
	tmaEvents := []CoreEvent{
		{EventName: "TOPDOWN.SLOTS:perf_metrics", EventCode: "0x00", UMask: "0x04", SampleAfterValue: "10000003", Counter: "Fixed counter 3"},
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
		if event.IsEmpty() {
			continue // Skip empty events
		}
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
		if event.IsEmpty() {
			continue // Skip empty events
		}
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
	// check if the events present in the group are also present in the other group
	for _, event := range group.GeneralPurposeCounters {
		if event.IsEmpty() {
			continue // Skip empty events
		}
		// if event is not in the other group, they are not equal
		if otherEvent := other.FindEventByName(event.EventName); otherEvent.IsEmpty() {
			return false // Event not found in other group
		}
	}
	// check if the events present in the other group are also present in this group
	for _, event := range other.GeneralPurposeCounters {
		if event.IsEmpty() {
			continue // Skip empty events
		}
		// if event is not in the group, they are not equal
		if groupEvent := group.FindEventByName(event.EventName); groupEvent.IsEmpty() {
			return false // Event not found in group
		}
	}
	return true // All checks passed, groups are equal
}

func (group CoreGroup) Copy() CoreGroup {
	newGroup := CoreGroup{}
	newGroup.MetricNames = make([]string, len(group.MetricNames))
	copied := copy(newGroup.MetricNames, group.MetricNames)
	if copied != len(group.MetricNames) {
		fmt.Printf("Warning: copied %d metric names, expected %d\n", copied, len(group.MetricNames))
	}
	newGroup.FixedPurposeCounters = make([]CoreEvent, len(group.FixedPurposeCounters))
	copied = copy(newGroup.FixedPurposeCounters, group.FixedPurposeCounters)
	if copied != len(group.FixedPurposeCounters) {
		fmt.Printf("Warning: copied %d fixed purpose counters, expected %d\n", copied, len(group.FixedPurposeCounters))
	}
	newGroup.GeneralPurposeCounters = make([]CoreEvent, len(group.GeneralPurposeCounters))
	copied = copy(newGroup.GeneralPurposeCounters, group.GeneralPurposeCounters)
	if copied != len(group.GeneralPurposeCounters) {
		fmt.Printf("Warning: copied %d general purpose counters, expected %d\n", copied, len(group.GeneralPurposeCounters))
	}
	return newGroup
}

func (group *CoreGroup) Merge(other CoreGroup, metadata Metadata) error {
	// Merge fixed purpose counters
	for _, event := range other.FixedPurposeCounters {
		if event.IsEmpty() {
			continue // Skip empty events
		}
		if err := group.AddEvent(event, false, metadata); err != nil {
			return fmt.Errorf("error adding fixed purpose counter %s: %w", event.EventName, err)
		}
	}
	// Merge general purpose counters
	for _, event := range other.GeneralPurposeCounters {
		if event.IsEmpty() {
			continue // Skip empty events
		}
		if err := group.AddEvent(event, true, metadata); err != nil {
			return fmt.Errorf("error adding general purpose counter %s: %w", event.EventName, err)
		}
	}
	// Merge metric names
	group.MetricNames = util.UniqueAppend(group.MetricNames, other.MetricNames...)
	return nil
}

func (group CoreGroup) HasTakenAloneEvent() bool {
	// Check if the group has an event tagged with "TakenAlone"
	for i := range group.FixedPurposeCounters {
		event := &group.FixedPurposeCounters[i]
		if event.TakenAlone == "true" {
			return true
		}
	}
	return false
}

func (group *CoreGroup) AddEvent(event CoreEvent, reorder bool, metadata Metadata) error {
	if event.IsEmpty() {
		return fmt.Errorf("event is not initialized")
	}
	// If the event is already in the group, no need to insert it again
	if !group.FindEventByName(event.EventName).IsEmpty() {
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
	if strings.HasPrefix(validCounters, "Fixed counter") {
		fixedCounter := strings.TrimPrefix(validCounters, "Fixed counter ")
		fixedCounterIndex, err := strconv.Atoi(fixedCounter)
		if err != nil {
			return fmt.Errorf("invalid fixed counter index %s for event %s: %w", fixedCounter, event.EventName, err)
		}
		if metadata.SupportsFixedInstructions && fixedCounterIndex == 0 {
			group.FixedPurposeCounters = append(group.FixedPurposeCounters, event)
			return nil
		}
		if metadata.SupportsFixedCycles && fixedCounterIndex == 1 {
			group.FixedPurposeCounters = append(group.FixedPurposeCounters, event)
			return nil
		}
		if metadata.SupportsFixedRefCycles && fixedCounterIndex == 2 {
			group.FixedPurposeCounters = append(group.FixedPurposeCounters, event)
			return nil
		}
		// fall through to add the event to a general purpose counter
		validCounters = ""
		for i := range len(group.GeneralPurposeCounters) {
			validCounters += fmt.Sprintf("%d,", i)
		}
	}
	// otherwise, it is a general purpose event, check if we can place it in one of the general purpose counters
	for i := range group.GeneralPurposeCounters {
		if counter := group.GeneralPurposeCounters[i]; counter.IsEmpty() {
			// this counter is empty, check if it is a valid counter for this event
			if strings.Contains(validCounters, fmt.Sprintf("%d", i)) {
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
			for otherCounter := 0; otherCounter < len(group.GeneralPurposeCounters); otherCounter++ {
				if otherCounter == counter {
					continue // skip the current counter
				}
				if !group.GeneralPurposeCounters[otherCounter].IsEmpty() {
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
	// If we reach here, we couldn't find a valid counter for the event
	return fmt.Errorf("no counter available for %s: %s", event.EventName, event.Counter)
}

func (group CoreGroup) Print(w io.Writer) {
	fmt.Fprintf(w, "  Metric Names: %s\n", strings.Join(group.MetricNames, ", "))
	fmt.Fprintln(w, "  Fixed Purpose Counters:")
	for i := range group.FixedPurposeCounters {
		event := &group.FixedPurposeCounters[i]
		if event.IsEmpty() {
			continue // Skip empty events
		}
		fmt.Fprintf(w, "    Counter %d: %s [%s]\n", i, event.EventName, event.Counter)
	}
	fmt.Fprintln(w, "  General Purpose Counters:")
	for i := range group.GeneralPurposeCounters {
		event := &group.GeneralPurposeCounters[i]
		if event.IsEmpty() {
			continue // Skip empty events
		}
		fmt.Fprintf(w, "    Counter %d: %s [%s]\n", i, event.EventName, event.Counter)
	}
}

func (group CoreGroup) StringForPerf() (string, error) {
	var formattedEvents []string
	// add the fixed purpose counters first
	for _, event := range group.FixedPurposeCounters {
		if event.IsEmpty() {
			continue // Skip empty events
		}
		// Format the event for perf
		formattedEvent, err := event.StringForPerf()
		if err != nil {
			return "", fmt.Errorf("error formatting event %s for perf: %w", event.EventName, err)
		}
		// Add the formatted event to the list
		formattedEvents = append(formattedEvents, formattedEvent)
	}
	for _, event := range group.FixedPurposeCounters {
		if event.IsEmpty() {
			continue // Skip empty events
		}
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

func MergeCoreGroups(coreGroups []CoreGroup, metadata Metadata) ([]CoreGroup, error) {
	i := 0
	for i < len(coreGroups) { // this style of for loop is used to allow for removal of elements
		j := i + 1
		for j < len(coreGroups) { // len(coreGroups) is recalculated on each iteration
			fmt.Printf("Attempting to merge core group %d into group %d\n", j, i)
			tmpGroup := coreGroups[i].Copy() // Copy the group to avoid modifying the original
			if err := tmpGroup.Merge(coreGroups[j], metadata); err == nil {
				fmt.Printf("Successfully merged core group %d into group %d\n", j, i)
				coreGroups[i] = tmpGroup // Update the group at index i with the merged group
				// remove the group at index j
				coreGroups = append(coreGroups[:j], coreGroups[j+1:]...)
			} else {
				fmt.Printf("Failed to merge core group %d into group %d: %v\n", j, i, err)
				j++ // Cannot merge these groups, try the next pair
			}
		}
		i++
	}
	return coreGroups, nil
}

func EliminateDuplicateCoreGroups(coreGroups []CoreGroup) ([]CoreGroup, error) {
	// if two groups have the same events, merge them into one group
	// combine the metric names of the groups
	i := 0
	for i < len(coreGroups) {
		j := i + 1
		for j < len(coreGroups) {
			if coreGroups[i].Equal(coreGroups[j]) {
				fmt.Printf("Found duplicate core group %d and %d\n", i, j)
				// merge the metric names
				coreGroups[i].MetricNames = util.UniqueAppend(coreGroups[i].MetricNames, coreGroups[j].MetricNames...)
				// remove the group at index j
				coreGroups = append(coreGroups[:j], coreGroups[j+1:]...)
			} else {
				j++ // only increment j if we didn't remove an element
			}
		}
		i++
	}
	return coreGroups, nil
}
