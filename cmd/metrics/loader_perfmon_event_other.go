// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"fmt"
	"log/slog"
	"strings"
)

type OtherEvent struct {
	EventName string
}

type OtherEvents []OtherEvent

func NewOtherEvents() (OtherEvents, error) {
	var events OtherEvents = []OtherEvent{
		{EventName: "power/energy-pkg/"},
		{EventName: "power/energy-ram/"},
		{EventName: "cstate_core/c6-residency/"},
		{EventName: "cstate_pkg/c6-residency/"},
	}
	return events, nil
}

func (events OtherEvents) FindEventByName(eventName string) OtherEvent {
	for _, event := range events {
		if event.EventName == eventName {
			return event
		}
	}
	return OtherEvent{} // return an empty OtherEvent if not found
}

func (event OtherEvent) IsEmpty() bool {
	return event == OtherEvent{}
}

func (event OtherEvent) IsCollectable(metadata Metadata) bool {
	if flagScope == scopeProcess || flagScope == scopeCgroup {
		slog.Debug("Other events not supported in process or cgroup scope", slog.String("event", event.EventName))
		return false // other events are not supported in process or cgroup scope
	}
	// no system-level events when collecting at CPU granularity e.g. power, cstates
	if (flagGranularity == granularityCPU) &&
		(strings.Contains(event.EventName, "power/energy") || strings.Contains(event.EventName, "cstate_pkg")) {
		slog.Debug("System level events not supported in CPU granularity", slog.String("event", event.EventName))
		return false
	}
	// check if the event is supported by perf
	// TODO: this substring check can produce a false positive, e.g., "ST_SPEC" matches a line
	// reading "INST_SPEC". util.HasLineIgnoreCase requires a whole-line match and is what the
	// component (ARM) loader uses. Switching to it here needs verification on an Intel target
	// first: PerfSupportedEvents holds entries like "cstate_core/c6-residency", so a stricter
	// match could start rejecting events that work today.
	if !strings.Contains(metadata.PerfSupportedEvents, event.EventName) {
		slog.Debug("Other event is not supported by perf", slog.String("event", event.EventName))
		return false // other events are not supported
	}
	return true
}

func (event OtherEvent) StringForPerf() (string, error) {
	if event.IsEmpty() {
		return "", fmt.Errorf("event is not initialized")
	}
	// For other events, we just return the event name as is.
	// This is used for events that are not part of core or uncore events.
	return event.EventName, nil
}
