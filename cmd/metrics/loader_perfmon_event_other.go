package metrics

import (
	"fmt"
	"log/slog"
	"strings"
)

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

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
