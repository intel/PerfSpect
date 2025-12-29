// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

type OtherGroup struct {
	GeneralPurposeCounters []OtherEvent
	MetricNames            []string
}

func NewOtherGroup(metadata Metadata) OtherGroup {
	return OtherGroup{
		GeneralPurposeCounters: make([]OtherEvent, metadata.NumGeneralPurposeCounters),
		MetricNames:            make([]string, 0),
	}
}

func (group OtherGroup) ToGroupDefinition() GroupDefinition {
	// Convert the CoreGroup to a GroupDefinition
	groupDef := make(GroupDefinition, 0)
	// Add general purpose counters
	for _, event := range group.GeneralPurposeCounters {
		if event.IsEmpty() {
			continue // Skip empty events
		}
		raw, err := event.StringForPerf()
		if err != nil {
			slog.Error("Error formatting event for perf", slog.String("event", event.EventName), slog.Any("error", err))
			continue
		}
		groupDef = append(groupDef, EventDefinition{
			Raw:  raw,
			Name: event.EventName,
		})
	}
	return groupDef
}

func (group *OtherGroup) AddEvent(event OtherEvent, _ bool) error {
	if event.IsEmpty() {
		return fmt.Errorf("cannot add empty event")
	}
	for i, existingEvent := range group.GeneralPurposeCounters {
		if existingEvent.IsEmpty() {
			// Found an empty slot, add the event here
			group.GeneralPurposeCounters[i] = event
			return nil
		}
	}
	return fmt.Errorf("no empty slot available in OtherGroup for event %s", event.EventName)
}

func (group OtherGroup) Print(w io.Writer) {
	fmt.Fprintf(w, "  Metric Names: %s\n", strings.Join(group.MetricNames, ", "))
	fmt.Fprintln(w, "  General Purpose Counters:")
	for i, event := range group.GeneralPurposeCounters {
		if event.IsEmpty() {
			continue // Skip empty events
		}
		fmt.Fprintf(w, "    Counter %d: %s\n", i, event.EventName)
	}
}

func (group OtherGroup) StringForPerf() (string, error) {
	var formattedEvents []string
	for _, event := range group.GeneralPurposeCounters {
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
