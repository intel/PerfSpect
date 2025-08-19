package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Linux perf event output, i.e., from 'perf stat' parsing and processing helper functions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"strings"
)

// EventGroup represents a group of perf events and their values
type EventGroup struct {
	EventValues map[string]float64 // event name -> event value
	GroupID     int
	Percentage  float64
}

// EventFrame represents the list of EventGroups collected with a specific timestamp
// and sometimes present cgroup
type EventFrame struct {
	EventGroups []EventGroup
	Timestamp   float64
	Socket      string
	CPU         string
	Cgroup      string
}

// Event represents the structure of an event output by perf stat...with
// a few exceptions
type Event struct {
	Interval     float64 `json:"interval"`
	CPU          string  `json:"cpu"`
	CounterValue string  `json:"counter-value"`
	Unit         string  `json:"unit"`
	Cgroup       string  `json:"cgroup"`
	Event        string  `json:"event"`
	EventRuntime int     `json:"event-runtime"`
	PcntRunning  float64 `json:"pcnt-running"`
	Value        float64 // parsed value
	Group        int     // event group index
	Socket       string  // only relevant if granularity is socket
}

// GetEventFrames organizes raw events received from perf into one or more frames (groups of events) that
// will be used for calculating metrics.
//
// The raw events received from perf will differ based on the scope of collection. Current options
// are system-wide, process, cgroup(s). Cgroup scoped data is received intermixed, i.e., multiple
// cgroups' data is represented in the rawEvents list. Process scoped data is received for only
// one process at a time.
//
// The frames produced will differ based on the intended metric granularity. Current options are
// system, socket, cpu (thread/logical CPU), but only when in system scope. Process and cgroup scope
// only support system-level granularity.
func GetEventFrames(rawEvents [][]byte, eventGroupDefinitions []GroupDefinition, scope string, granularity string, metadata Metadata) (eventFrames []EventFrame, err error) {
	// parse raw events into list of Event
	var allEvents []Event
	if allEvents, err = parseEvents(rawEvents); err != nil {
		return
	}

	// bucket events into one or more lists based on scope and granularity
	var bucketedEvents [][]Event
	if bucketedEvents, err = bucketEvents(allEvents, scope, granularity, metadata); err != nil {
		return
	}

	// aggregate uncore events
	var aggregatedEvents [][]Event
	if aggregatedEvents, err = aggregateUncoreEvents(bucketedEvents); err != nil {
		return nil, fmt.Errorf("failed to aggregate uncore events: %v", err)
	}

	// assign events to groups based on event group definitions
	for _, events := range aggregatedEvents {
		err = assignEventsToGroups(events, eventGroupDefinitions)
		if err != nil {
			return
		}
	}

	// create one EventFrame per list of Events
	for _, events := range aggregatedEvents {
		// organize events into groups
		group := EventGroup{EventValues: make(map[string]float64)}
		var lastGroupID int
		var eventFrame EventFrame
		for eventIdx, event := range events {
			if eventIdx == 0 {
				lastGroupID = event.Group
				eventFrame.Timestamp = event.Interval
				switch flagGranularity {
				case granularityCPU:
					eventFrame.CPU = event.CPU
				case granularitySocket:
					eventFrame.Socket = event.Socket
				}
				if flagScope == scopeCgroup {
					eventFrame.Cgroup = event.Cgroup
				}
			}
			if event.Group != lastGroupID {
				eventFrame.EventGroups = append(eventFrame.EventGroups, group)
				group = EventGroup{EventValues: make(map[string]float64)}
				lastGroupID = event.Group
			}
			group.GroupID = event.Group
			group.Percentage = event.PcntRunning
			group.EventValues[event.Event] = event.Value
		}
		// add the last group
		eventFrame.EventGroups = append(eventFrame.EventGroups, group)
		eventFrames = append(eventFrames, eventFrame)
	}
	return
}

// parseEvents parses the raw event data into a list of Event
func parseEvents(rawEvents [][]byte) ([]Event, error) {
	events := make([]Event, 0, len(rawEvents))
	var eventsNotCounted []string
	var eventsNotSupported []string
	for _, rawEvent := range rawEvents {
		var event Event
		if err := json.Unmarshal(rawEvent, &event); err != nil {
			err = fmt.Errorf("unrecognized event format: %w", err)
			slog.Error(err.Error(), slog.String("event", string(rawEvent)))
			return nil, err
		}
		// sometimes perf will prepend "cpu/" to the topdown event names, e.g., cpu/topdown-retiring/, we clean it up here to match metric formulas
		if strings.HasPrefix(event.Event, "cpu/") && strings.Contains(event.Event, "topdown") && strings.HasSuffix(event.Event, "/") {
			event.Event = strings.TrimPrefix(event.Event, "cpu/")
			event.Event = strings.TrimSuffix(event.Event, "/")
		}
		switch event.CounterValue {
		case "<not counted>":
			slog.Debug("event not counted", slog.String("event", string(rawEvent)))
			eventsNotCounted = append(eventsNotCounted, event.Event)
			event.Value = math.NaN()
		case "<not supported>":
			slog.Debug("event not supported", slog.String("event", string(rawEvent)))
			eventsNotSupported = append(eventsNotSupported, event.Event)
			event.Value = math.NaN()
		default:
			var err error
			if event.Value, err = strconv.ParseFloat(event.CounterValue, 64); err != nil {
				slog.Error("failed to parse event value", slog.String("event", event.Event), slog.String("value", event.CounterValue))
				event.Value = math.NaN()
			}
		}
		events = append(events, event)
	}
	if len(eventsNotCounted) > 0 {
		slog.Warn("events not counted", slog.String("events", strings.Join(eventsNotCounted, ",")))
	}
	if len(eventsNotSupported) > 0 {
		slog.Warn("events not supported", slog.String("events", strings.Join(eventsNotSupported, ",")))
	}
	return events, nil
}

// aggregateUncoreEvents sums the values of uncore events with the same name and interval
// and removes duplicates, leaving only one event per name and interval.
func aggregateUncoreEvents(bucketedEvents [][]Event) ([][]Event, error) {
	if flagGranularity != granularitySocket {
		return bucketedEvents, nil // disaggregated uncore events are only present in socket granularity
	}
	for bucketIdx, events := range bucketedEvents {
		if len(events) == 0 {
			continue
		}
		// Use a map to track unique uncore events by (event_name, interval)
		uncoreMap := make(map[string]int) // key: "event_name:interval" -> index in filtered slice
		var filteredEvents []Event
		for _, event := range events {
			if strings.HasPrefix(event.Event, "UNC") {
				// Create unique key for this uncore event
				key := fmt.Sprintf("%s:%.9f", event.Event, event.Interval)
				// Check if this uncore event is already in the map
				if existingIdx, exists := uncoreMap[key]; exists {
					// Aggregate with existing event
					filteredEvents[existingIdx].Value += event.Value
				} else {
					// Add new unique uncore event
					uncoreMap[key] = len(filteredEvents)
					filteredEvents = append(filteredEvents, event)
				}
			} else {
				// Keep non-uncore events as-is
				filteredEvents = append(filteredEvents, event)
			}
		}
		// Update the original slice
		bucketedEvents[bucketIdx] = filteredEvents
	}
	return bucketedEvents, nil
}

// assignEventsToGroups assigns each event to a group based on the event group definitions.
// It modifies the events in place by setting the Group field of each event.
func assignEventsToGroups(events []Event, eventGroupDefinitions []GroupDefinition) error {
	if len(events) == 0 {
		return fmt.Errorf("no events to assign to groups")
	}
	if len(eventGroupDefinitions) == 0 {
		return fmt.Errorf("no event group definitions provided")
	}
	groupIdx := 0
	eventIdx := -1
	previousEvent := ""
	for i := range events {
		if events[i].Event != previousEvent {
			eventIdx++
			previousEvent = events[i].Event
		}
		if eventIdx == len(eventGroupDefinitions[groupIdx]) { // last event in group
			groupIdx++
			if groupIdx == len(eventGroupDefinitions) {
				return fmt.Errorf("event group definitions not aligning with raw events")
			}
			eventIdx = 0
		}
		events[i].Group = groupIdx
	}
	return nil
}

// bucketEvents separates the events into a number of event lists by granularity and scope
func bucketEvents(allEvents []Event, scope string, granularity string, metadata Metadata) (bucketedEvents [][]Event, err error) {
	switch scope {
	case scopeSystem:
		switch granularity {
		case granularitySystem:
			bucketedEvents = append(bucketedEvents, allEvents)
			return
		case granularitySocket:
			// create one list of Events per Socket
			newEvents := make([][]Event, metadata.SocketCount)
			for i := range metadata.SocketCount {
				newEvents[i] = make([]Event, 0, len(allEvents)/metadata.SocketCount)
			}
			// incoming events are labeled with cpu number
			// we need to map cpu number to socket number, and accumulate the values from each cpu event into a socket event
			// we assume that the events are ordered by cpu number and events are present for each cpu
			var currentEvent string
			for _, event := range allEvents {
				var eventCPU int
				if eventCPU, err = strconv.Atoi(event.CPU); err != nil {
					err = fmt.Errorf("failed to parse cpu number: %s", event.CPU)
					return
				}
				// if cpu exists in map, add event to the eventSocket, use the !ok go idiom to check if the key exists
				if eventSocket, ok := metadata.CPUSocketMap[eventCPU]; ok {
					if eventSocket > len(newEvents)-1 {
						err = fmt.Errorf("cpu %d is mapped to socket %d, which is greater than the number of sockets %d", eventCPU, eventSocket, len(newEvents)-1)
						return
					}
					// if first event or the event name changed, add the event to the list of socket events
					if len(newEvents[eventSocket]) == 0 || newEvents[eventSocket][len(newEvents[eventSocket])-1].Event != currentEvent || event.Event != currentEvent {
						newEvents[eventSocket] = append(newEvents[eventSocket], event)
						newEvents[eventSocket][len(newEvents[eventSocket])-1].Socket = fmt.Sprintf("%d", eventSocket)
						newEvents[eventSocket][len(newEvents[eventSocket])-1].CPU = ""
						currentEvent = event.Event
					} else {
						// if the event name is the same as the last socket event, add the new event's value to the last socket event's value
						newEvents[eventSocket][len(newEvents[eventSocket])-1].Value += event.Value
					}
				} else {
					err = fmt.Errorf("cpu %d is not mapped to a socket", eventCPU)
					return
				}
			}
			bucketedEvents = append(bucketedEvents, newEvents...)
			return
		case granularityCPU:
			// create one list of Events per CPU dynamically as we encounter them
			cpuEventsMap := make(map[int][]Event)

			for _, event := range allEvents {
				var cpu int
				if cpu, err = strconv.Atoi(event.CPU); err != nil {
					return nil, fmt.Errorf("failed to parse cpu number: %s", event.CPU)
				}

				// dynamically create CPU bucket if it doesn't exist
				if _, exists := cpuEventsMap[cpu]; !exists {
					cpuEventsMap[cpu] = make([]Event, 0)
				}

				cpuEventsMap[cpu] = append(cpuEventsMap[cpu], event)
			}

			// convert map to slice, maintaining consistent ordering by CPU number
			cpuNumbers := make([]int, 0, len(cpuEventsMap))
			for cpu := range cpuEventsMap {
				cpuNumbers = append(cpuNumbers, cpu)
			}
			slices.Sort(cpuNumbers)

			for _, cpu := range cpuNumbers {
				bucketedEvents = append(bucketedEvents, cpuEventsMap[cpu])
			}
		default:
			err = fmt.Errorf("unsupported granularity: %s", granularity)
			return
		}
	case scopeProcess:
		bucketedEvents = append(bucketedEvents, allEvents)
		return
	case scopeCgroup:
		// expand events list to one list per cgroup
		var allCgroupEvents [][]Event
		var cgroups []string
		for _, event := range allEvents {
			var cgroupIdx int
			if cgroupIdx = slices.Index(cgroups, event.Cgroup); cgroupIdx == -1 {
				cgroups = append(cgroups, event.Cgroup)
				cgroupIdx = len(cgroups) - 1
				allCgroupEvents = append(allCgroupEvents, []Event{})
			}
			allCgroupEvents[cgroupIdx] = append(allCgroupEvents[cgroupIdx], event)
		}
		bucketedEvents = append(bucketedEvents, allCgroupEvents...)
	default:
		err = fmt.Errorf("unsupported scope: %s", scope)
		return
	}
	return
}

// extractInterval parses the interval value from a JSON perf event line
// Returns the interval as a float64, or -1 if parsing fails
func extractInterval(line []byte) float64 {
	// Look for the interval field in the JSON: "interval" : 5.005073756
	intervalPattern := []byte(`"interval" : `)
	intervalStart := bytes.Index(line, intervalPattern)
	if intervalStart == -1 {
		return -1
	}

	// Move to the start of the number
	intervalStart += len(intervalPattern)
	if intervalStart >= len(line) {
		return -1
	}

	// Find the end of the number (comma, space, or closing brace)
	intervalEnd := intervalStart
	for intervalEnd < len(line) {
		ch := line[intervalEnd]
		if ch == ',' || ch == ' ' || ch == '}' {
			break
		}
		intervalEnd++
	}
	if intervalEnd == intervalStart {
		return -1
	}

	// Parse the number directly from bytes
	interval, err := strconv.ParseFloat(string(line[intervalStart:intervalEnd]), 64)
	if err != nil {
		return -1
	}

	return interval
}
