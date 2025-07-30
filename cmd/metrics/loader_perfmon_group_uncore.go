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

type UncoreGroup struct {
	MaxGeneralPurposeCounters int
	GeneralPurposeCounters    map[int]UncoreEvent
	MetricNames               []string
}

func NewUncoreGroup(maxGeneralPurposeCounters int) UncoreGroup {
	return UncoreGroup{
		MaxGeneralPurposeCounters: maxGeneralPurposeCounters,
		GeneralPurposeCounters:    make(map[int]UncoreEvent, 0),
	}
}

func (group UncoreGroup) ToGroupDefinition() GroupDefinition {
	// Convert the CoreGroup to a GroupDefinition
	groupDef := make(GroupDefinition, 0)
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

func (group UncoreGroup) FindEventByName(eventName string) UncoreEvent {
	for _, event := range group.GeneralPurposeCounters {
		if event.EventName == eventName {
			return event // Event found in the group
		}
	}
	// If we reach here, the event was not found in any of the counters
	return UncoreEvent{}
}

func (group UncoreGroup) Equal(other UncoreGroup) bool {
	if group.MaxGeneralPurposeCounters != other.MaxGeneralPurposeCounters {
		return false // Different max GP events
	}
	if len(group.GeneralPurposeCounters) != len(other.GeneralPurposeCounters) {
		return false // Different number of general purpose counters
	}
	// order of general purpose counters is not important
	for _, event := range group.GeneralPurposeCounters {
		// if event is not in the other group, they are not equal
		if otherEvent := other.FindEventByName(event.EventName); otherEvent == (UncoreEvent{}) {
			return false // Event not found in other group
		}
	}
	return true // All checks passed, groups are equal
}

func (group UncoreGroup) Copy() UncoreGroup {
	newGroup := NewUncoreGroup(group.MaxGeneralPurposeCounters)
	copy(newGroup.MetricNames, group.MetricNames)
	maps.Copy(newGroup.GeneralPurposeCounters, group.GeneralPurposeCounters)
	return newGroup
}

func (group UncoreGroup) Merge(other UncoreGroup) error {
	// Merge metric names
	group.MetricNames = util.UniqueAppend(group.MetricNames, other.MetricNames...)
	// Merge general purpose counters
	for _, event := range other.GeneralPurposeCounters {
		if err := group.AddEvent(event, false); err != nil {
			return fmt.Errorf("error adding event %s to group: %w", event.EventName, err)
		}
	}
	return nil
}

func (group UncoreGroup) AddEvent(event UncoreEvent, reorder bool) error {
	if group.FindEventByName(event.EventName) != (UncoreEvent{}) {
		// Event is already in the group, no need to insert it again
		return nil
	}
	// the new event's unit must match the unit of the other events already in the group
	for _, existingEvent := range group.GeneralPurposeCounters {
		if existingEvent != (UncoreEvent{}) && existingEvent.Unit != event.Unit {
			return fmt.Errorf("incompatible unit for %s, %s != %s", event.EventName, existingEvent.Unit, event.Unit)
		}
	}
	// get the list of valid counters for this event
	validCounters := event.Counter
	if validCounters == "" {
		return fmt.Errorf("event %s has no valid counters defined", event.EventName)
	}
	// check if the group has an open counter that is in the valid counters list
	for i := 0; i < group.MaxGeneralPurposeCounters; i++ {
		if _, ok := group.GeneralPurposeCounters[i]; !ok {
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
	return fmt.Errorf("no counter available for %s: %s", event.EventName, event.Counter)
}

func (group UncoreGroup) Print(w io.Writer) {
	fmt.Fprintf(w, "  Metric Names: %s\n", strings.Join(group.MetricNames, ", "))
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

func (group UncoreGroup) StringForPerf() (string, error) {
	// sort the events by their counter number
	keys := make([]int, 0, len(group.GeneralPurposeCounters))
	for k := range group.GeneralPurposeCounters {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var formattedEvents []string
	for _, k := range keys {
		// Format the event for perf
		formattedEvent, err := group.GeneralPurposeCounters[k].StringForPerf()
		if err != nil {
			return "", fmt.Errorf("error formatting event %s for perf: %w", group.GeneralPurposeCounters[k].EventName, err)
		}
		// Add the formatted event to the list
		formattedEvents = append(formattedEvents, formattedEvent)
	}
	if len(formattedEvents) == 0 {
		return "", fmt.Errorf("no valid events found in group")
	}
	return fmt.Sprintf("{%s}", strings.Join(formattedEvents, ",")), nil
}

func MergeUncoreGroups(uncoreGroups []UncoreGroup) ([]UncoreGroup, error) {
	for i := 0; i < len(uncoreGroups); i++ {
		for j := i + 1; j < len(uncoreGroups); j++ {
			fmt.Printf("Attempting to merge uncore group %d into group %d\n", j, i)
			tmpGroup := uncoreGroups[i].Copy() // Copy the group to avoid modifying the original
			err := tmpGroup.Merge(uncoreGroups[j])
			if err != nil {
				fmt.Printf("Failed to merge uncore group %d into group %d: %v\n", j, i, err)
				continue // Cannot merge these groups, try the next pair
			}
			fmt.Printf("Successfully merged uncore group %d into group %d\n", j, i)
			uncoreGroups[i] = tmpGroup // Update the group at index i with the merged group
			// remove the group at index j
			uncoreGroups = append(uncoreGroups[:j], uncoreGroups[j+1:]...)
			j-- // adjust index since we removed an element
		}
	}
	return uncoreGroups, nil
}

func EliminateDuplicateUncoreGroups(uncoreGroups []UncoreGroup) ([]UncoreGroup, error) {
	for i := 0; i < len(uncoreGroups); i++ {
		for j := i + 1; j < len(uncoreGroups); j++ {
			if uncoreGroups[i].Equal(uncoreGroups[j]) {
				fmt.Printf("Found duplicate uncore group %d and %d\n", i, j)
				// merge the metric names
				uncoreGroups[i].MetricNames = util.UniqueAppend(uncoreGroups[i].MetricNames, uncoreGroups[j].MetricNames...)
				// remove the group at index j
				uncoreGroups = append(uncoreGroups[:j], uncoreGroups[j+1:]...)
				j-- // adjust index since we removed an element
			}
		}
	}
	return uncoreGroups, nil
}

func ExpandUncoreGroups(uncoreGroups []UncoreGroup, uncoreDeviceIDs map[string][]int) ([]UncoreGroup, error) {
	var expandedGroups []UncoreGroup
	for _, group := range uncoreGroups {
		if len(group.GeneralPurposeCounters) == 0 {
			return nil, fmt.Errorf("group has no general purpose counters")
		}
		groupUnit := group.GeneralPurposeCounters[0].Unit // Assume all events in the group have the same unit
		if groupUnit == "" {
			return nil, fmt.Errorf("group has no unit defined")
		}
		// Create a new group for each uncore device ID
		for deviceType, deviceIDs := range uncoreDeviceIDs {
			if !strings.EqualFold(deviceType, groupUnit) {
				continue // Skip if the device type does not match the group unit
			}
			for _, deviceID := range deviceIDs {
				// Create a new group for this device ID
				newGroup := group.Copy()
				for counter, event := range group.GeneralPurposeCounters {
					// add the device ID to the event's name
					newName := fmt.Sprintf("%s.%d", event.EventName, deviceID)
					newEvent := event                                   // Create a copy of the event
					newEvent.EventName = newName                        // Update the event name with the device ID
					newGroup.GeneralPurposeCounters[counter] = newEvent // Update the event in the new group
				}
				expandedGroups = append(expandedGroups, newGroup)
			}
		}
	}
	return expandedGroups, nil
}
