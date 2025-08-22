package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type UncoreEvent struct {
	Unit              string `json:"Unit"`
	EventCode         string `json:"EventCode"`
	UMask             string `json:"UMask"`
	PortMask          string `json:"PortMask"`
	FCMask            string `json:"FCMask"`
	UMaskExt          string `json:"UMaskExt"`
	EventName         string `json:"EventName"`
	BriefDescription  string `json:"BriefDescription"`
	PublicDescription string `json:"PublicDescription"`
	Counter           string `json:"Counter"`
	ELLC              string `json:"ELLC"`
	Filter            string `json:"Filter"`
	ExtSel            string `json:"ExtSel"`
	Deprecated        string `json:"Deprecated"`
	FilterValue       string `json:"FILTER_VALUE"`
	CounterType       string `json:"CounterType"`
}

type UncoreEvents struct {
	Header map[string]string `json:"Header"`
	Events []UncoreEvent     `json:"Events"`
}

func NewUncoreEvents(pathWithSource string) (UncoreEvents, error) {
	var events UncoreEvents
	pathParts := strings.Split(pathWithSource, ":")
	if len(pathParts) != 2 || (pathParts[0] != "resources" && pathParts[0] != "file") {
		return UncoreEvents{}, fmt.Errorf("invalid path format, expected 'resources:<path>' or 'file:<path>' but got '%s'", pathWithSource)
	}
	var path string
	var bytes []byte
	var err error
	if pathParts[0] == "resources" {
		path = filepath.Join("resources", "perfmon", pathParts[1])
		bytes, err = resources.ReadFile(path)
	} else { // pathParts[0] == "file"
		path = pathParts[1]
		bytes, err = os.ReadFile(path) // #nosec G304
	}
	if err != nil {
		return events, fmt.Errorf("error reading file %s: %w", path, err)
	}
	if err := json.Unmarshal(bytes, &events); err != nil {
		return events, fmt.Errorf("error unmarshaling JSON from %s: %w", path, err)
	}
	return events, nil
}

func (events UncoreEvents) FindEventByName(eventName string) UncoreEvent {
	// Check if event is customized with :c<val>, :e<val>, or both. If it is, then we need to remove them
	// from the name to match the event name in the events lists.
	// examples: INST_RETIRED.ANY:c0:e1, CPU_CLK_UNHALTED.THREAD:c0
	// Get the base event name
	name := strings.Split(eventName, ":")[0]
	for _, event := range events.Events {
		if event.EventName == name {
			return event
		}
	}
	return UncoreEvent{}
}

func (event UncoreEvent) IsEmpty() bool {
	return event == UncoreEvent{}
}

func (event UncoreEvent) IsCollectable(metadata Metadata) bool {
	if !metadata.SupportsUncore {
		slog.Debug("Uncore events not supported on target", slog.String("event", event.EventName))
		return false // uncore events are not supported
	}
	if flagScope == scopeProcess || flagScope == scopeCgroup || flagGranularity == granularityCPU {
		slog.Debug("Uncore events not supported in process scope, cgroup scope, or cpu granularity", slog.String("event", event.EventName))
		return false
	}
	deviceExists := false
	for uncoreDeviceName := range metadata.UncoreDeviceIDs {
		if strings.EqualFold(strings.Split(event.Unit, " ")[0], uncoreDeviceName) {
			deviceExists = true
			break
		}
	}
	if !deviceExists {
		slog.Warn("Uncore event unit not found on target", "name", event.EventName, "unit", event.Unit)
		return false // uncore device not found
	}
	return true
}

func (event UncoreEvent) StringForPerf() (string, error) {
	if event.IsEmpty() {
		return "", fmt.Errorf("event is not initialized")
	}
	if event.EventCode == "" {
		return "", fmt.Errorf("event %s does not have an EventCode", event.EventName)
	}
	var parts []string
	// unit/event
	parts = append(parts, fmt.Sprintf("%s/event=%s", strings.ToLower(strings.Split(event.Unit, " ")[0]), event.EventCode))
	// umask
	if event.UMask != "" {
		umaskVal, err := strconv.ParseInt(event.UMask, 0, 64)
		if err != nil {
			return "", fmt.Errorf("error parsing UMask %s for event %s: %w", event.UMask, event.EventName, err)
		}
		umaskHex := fmt.Sprintf("%02x", umaskVal)
		if event.UMaskExt != "" {
			umaskExtVal, err := strconv.ParseInt(event.UMaskExt, 0, 64)
			if err != nil {
				return "", fmt.Errorf("error parsing UMaskExt %s for event %s: %w", event.UMaskExt, event.EventName, err)
			}
			var umaskExtHex string
			if umaskExtVal > 0 {
				umaskExtHex = fmt.Sprintf("%x", umaskExtVal)
			}
			parts = append(parts, fmt.Sprintf("umask=0x%s%s", umaskExtHex, umaskHex))
		}
	}
	parts = append(parts, fmt.Sprintf("name='%s'/", event.EventName))
	return strings.Join(parts, ","), nil
}
