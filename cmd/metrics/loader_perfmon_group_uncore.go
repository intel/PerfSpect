// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"fmt"
	"io"
	"log/slog"
	"perfspect/internal/util"
	"strings"
)

type UncoreGroup struct {
	GeneralPurposeCounters []UncoreEvent
	MetricNames            []string
}

func NewUncoreGroup(metadata Metadata) UncoreGroup {
	return UncoreGroup{
		GeneralPurposeCounters: make([]UncoreEvent, metadata.NumGeneralPurposeCounters),
	}
}

func (group UncoreGroup) ToGroupDefinition() GroupDefinition {
	// Convert the CoreGroup to a GroupDefinition
	groupDef := make(GroupDefinition, 0)
	// Add general purpose counters
	for _, event := range group.GeneralPurposeCounters {
		if event.IsEmpty() {
			continue // Skip empty events
		}
		// Format the event for perf
		raw, err := event.StringForPerf()
		if err != nil {
			slog.Error("Error formatting event for perf", slog.String("event", event.EventName), slog.Any("error", err))
			continue
		}
		// Add the formatted event to the group definition
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
	// check if the events present in the other group are also present in the group
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

func (group UncoreGroup) Copy() UncoreGroup {
	newGroup := UncoreGroup{}
	newGroup.MetricNames = make([]string, len(group.MetricNames))
	copy(newGroup.MetricNames, group.MetricNames)
	newGroup.GeneralPurposeCounters = make([]UncoreEvent, len(group.GeneralPurposeCounters))
	copy(newGroup.GeneralPurposeCounters, group.GeneralPurposeCounters)
	return newGroup
}

func (group *UncoreGroup) Merge(other UncoreGroup) error {
	// Merge general purpose counters
	for _, event := range other.GeneralPurposeCounters {
		if event.IsEmpty() {
			continue // Skip empty events
		}
		if err := group.AddEvent(event, true); err != nil {
			return fmt.Errorf("error adding general purpose counter %s: %w", event.EventName, err)
		}
	}
	// Merge metric names
	group.MetricNames = util.UniqueAppend(group.MetricNames, other.MetricNames...)
	return nil
}

func (group *UncoreGroup) AddEvent(event UncoreEvent, reorder bool) error {
	if event.IsEmpty() {
		return fmt.Errorf("event is not initialized")
	}
	if group.FindEventByName(event.EventName) != (UncoreEvent{}) {
		// Event is already in the group, no need to insert it again
		return nil
	}
	// the new event's unit must match the unit of the other events already in the group
	for _, existingEvent := range group.GeneralPurposeCounters {
		if existingEvent.IsEmpty() {
			continue // Skip empty events
		}
		if existingEvent.Unit != event.Unit {
			return fmt.Errorf("incompatible unit for %s, %s != %s", event.EventName, existingEvent.Unit, event.Unit)
		}
	}
	// get the list of valid counters for this event
	validCounters := event.Counter
	if validCounters == "" {
		return fmt.Errorf("event %s has no valid counters defined", event.EventName)
	}
	// check if the group has an open counter that is in the valid counters list
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
				group.GeneralPurposeCounters[otherCounter] = existingEvent // move the existing event to the new counter
				group.GeneralPurposeCounters[counter] = event              // place the new event in the current counter
				return nil
			}
		}
	}
	// If we reach here, we couldn't find a valid counter for the event
	return fmt.Errorf("no counter available for %s: %s", event.EventName, event.Counter)
}

func (group UncoreGroup) Print(w io.Writer) {
	fmt.Fprintf(w, "  Metric Names: %s\n", strings.Join(group.MetricNames, ", "))
	fmt.Fprintln(w, "  General Purpose Counters:")
	for i, event := range group.GeneralPurposeCounters {
		if event.IsEmpty() {
			continue // Skip empty events
		}
		fmt.Fprintf(w, "    Counter %d: %s [%s]\n", i, event.EventName, event.Counter)
	}
}

func (group UncoreGroup) StringForPerf() (string, error) {
	var formattedEvents []string
	for i, event := range group.GeneralPurposeCounters {
		if event.IsEmpty() {
			continue // Skip empty events
		}
		// Format the event for perf
		formattedEvent, err := event.StringForPerf()
		if err != nil {
			return "", fmt.Errorf("error formatting event %s for perf: %w", group.GeneralPurposeCounters[i].EventName, err)
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
	// First, eliminate duplicate events across groups (only needs to be done once)
	seenEvents := make(map[string]int) // map of event name to group index where it first appears
	for i := range uncoreGroups {
		for j := range uncoreGroups[i].GeneralPurposeCounters {
			event := uncoreGroups[i].GeneralPurposeCounters[j]
			if event.IsEmpty() {
				continue
			}

			if firstGroupIdx, exists := seenEvents[event.EventName]; exists {
				// Event already exists in another group, remove from current group
				slog.Debug("removing duplicate uncore event from group",
					slog.String("event", event.EventName),
					slog.Int("firstGroup", firstGroupIdx),
					slog.Int("currentGroup", i))
				uncoreGroups[i].GeneralPurposeCounters[j] = UncoreEvent{}
			} else {
				// First time seeing this event, record it
				seenEvents[event.EventName] = i
			}
		}
	}

	// Then, keep merging until no more merges are possible
	merged := true
	for merged {
		merged = false
		i := 0
		for i < len(uncoreGroups) {
			j := i + 1
			for j < len(uncoreGroups) {
				tmpGroup := uncoreGroups[i].Copy()
				if err := tmpGroup.Merge(uncoreGroups[j]); err == nil {
					uncoreGroups[i] = tmpGroup
					// remove the group at index j
					uncoreGroups = append(uncoreGroups[:j], uncoreGroups[j+1:]...)
					merged = true // mark that we made a change
				} else {
					j++
				}
			}
			i++
		}
	}
	return uncoreGroups, nil
}

func EliminateDuplicateUncoreGroups(uncoreGroups []UncoreGroup) ([]UncoreGroup, error) {
	// if two groups have the same events, merge them into one group
	// combine the metric names of the groups
	i := 0
	for i < len(uncoreGroups) {
		j := i + 1
		for j < len(uncoreGroups) {
			if uncoreGroups[i].Equal(uncoreGroups[j]) {
				// merge the metric names
				uncoreGroups[i].MetricNames = util.UniqueAppend(uncoreGroups[i].MetricNames, uncoreGroups[j].MetricNames...)
				// remove the group at index j
				uncoreGroups = append(uncoreGroups[:j], uncoreGroups[j+1:]...)
			} else {
				j++ // only increment j if we didn't remove an element
			}
		}
		i++
	}
	return uncoreGroups, nil
}
