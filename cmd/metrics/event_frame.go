package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Linux perf event output, i.e., from 'perf stat' parsing and processing helper functions

import (
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
	if allEvents, err = parseEvents(rawEvents, eventGroupDefinitions); err != nil {
		return
	}
	// coalesce events to one or more lists based on scope and granularity
	var coalescedEvents [][]Event
	if coalescedEvents, err = coalesceEvents(allEvents, scope, granularity, metadata); err != nil {
		return
	}
	// create one EventFrame per list of Events
	for _, events := range coalescedEvents {
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
		// TODO: can we collapse uncore groups as we're parsing (above)?
		if eventFrame, err = collapseUncoreGroupsInFrame(eventFrame); err != nil {
			return
		}
		eventFrames = append(eventFrames, eventFrame)
	}
	return
}

// parseEvents parses the raw event data into a list of Event
func parseEvents(rawEvents [][]byte, eventGroupDefinitions []GroupDefinition) ([]Event, error) {
	events := make([]Event, 0, len(rawEvents))
	groupIdx := 0
	eventIdx := -1
	previousEvent := ""
	var eventsNotCounted []string
	var eventsNotSupported []string
	for _, rawEvent := range rawEvents {
		event, err := parseEventJSON(rawEvent) // nosemgrep
		if err != nil {
			slog.Error(err.Error(), slog.String("event", string(rawEvent)))
			return nil, err
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
		}
		if event.Event != previousEvent {
			eventIdx++
			previousEvent = event.Event
		}
		if eventIdx == len(eventGroupDefinitions[groupIdx]) { // last event in group
			groupIdx++
			if groupIdx == len(eventGroupDefinitions) {
				// if in cgroup scope, we receive one set of events for each cgroup
				if flagScope == scopeCgroup {
					groupIdx = 0
				} else {
					return nil, fmt.Errorf("event group definitions not aligning with raw events")
				}
			}
			eventIdx = 0
		}
		event.Group = groupIdx
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

// coalesceEvents separates the events into a number of event lists by granularity and scope
func coalesceEvents(allEvents []Event, scope string, granularity string, metadata Metadata) (coalescedEvents [][]Event, err error) {
	switch scope {
	case scopeSystem:
		switch granularity {
		case granularitySystem:
			coalescedEvents = append(coalescedEvents, allEvents)
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
			coalescedEvents = append(coalescedEvents, newEvents...)
			return
		case granularityCPU:
			// create one list of Events per CPU
			numCPUs := metadata.SocketCount * metadata.CoresPerSocket * metadata.ThreadsPerCore
			// note: if some cores have been off-lined, this may cause an issue because 'perf' seems
			// to still report events for those cores
			newEvents := make([][]Event, numCPUs)
			for i := range numCPUs {
				newEvents[i] = make([]Event, 0, len(allEvents)/numCPUs)
			}
			for _, event := range allEvents {
				var cpu int
				if cpu, err = strconv.Atoi(event.CPU); err != nil {
					return
				}
				// handle case where perf returns events for off-lined cores
				if cpu > len(newEvents)-1 {
					cpusToAdd := len(newEvents) + 1 - cpu
					for range cpusToAdd {
						newEvents = append(newEvents, make([]Event, 0, len(allEvents)/numCPUs))
					}
				}
				newEvents[cpu] = append(newEvents[cpu], event)
			}
			coalescedEvents = append(coalescedEvents, newEvents...)
		default:
			err = fmt.Errorf("unsupported granularity: %s", granularity)
			return
		}
	case scopeProcess:
		coalescedEvents = append(coalescedEvents, allEvents)
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
		coalescedEvents = append(coalescedEvents, allCgroupEvents...)
	default:
		err = fmt.Errorf("unsupported scope: %s", scope)
		return
	}
	return
}

// collapseUncoreGroupsInFrame merges repeated (per-device) uncore groups into a single
// group by summing the values for events that only differ by device ID.
//
// uncore events are received in repeated perf groups like this:
// group:
// 5.005032332,49,,UNC_CHA_TOR_INSERTS.IA_MISS_CRD.0,2806917160,25.00,,
// 5.005032332,2720,,UNC_CHA_TOR_INSERTS.IA_MISS_DRD_REMOTE.0,2806917160,25.00,,
// 5.005032332,1061494,,UNC_CHA_TOR_OCCUPANCY.IA_MISS_DRD_REMOTE.0,2806917160,25.00,,
// group:
// 5.005032332,49,,UNC_CHA_TOR_INSERTS.IA_MISS_CRD.1,2806585867,25.00,,
// 5.005032332,2990,,UNC_CHA_TOR_INSERTS.IA_MISS_DRD_REMOTE.1,2806585867,25.00,,
// 5.005032332,1200063,,UNC_CHA_TOR_OCCUPANCY.IA_MISS_DRD_REMOTE.1,2806585867,25.00,,
//
// For the example above, we will have this:
// 5.005032332,98,,UNC_CHA_TOR_INSERTS.IA_MISS_CRD,2806585867,25.00,,
// 5.005032332,5710,,UNC_CHA_TOR_INSERTS.IA_MISS_DRD_REMOTE,2806585867,25.00,,
// 5.005032332,2261557,,UNC_CHA_TOR_OCCUPANCY.IA_MISS_DRD_REMOTE,2806585867,25.00,,
// Note: uncore event names start with "UNC"
// Note: we assume that uncore events are not mixed into groups that have other event types, e.g., cpu events
func collapseUncoreGroupsInFrame(inFrame EventFrame) (outFrame EventFrame, err error) {
	outFrame = inFrame
	outFrame.EventGroups = []EventGroup{}
	var idxUncoreMatches []int
	for inGroupIdx, inGroup := range inFrame.EventGroups {
		// skip groups that have been collapsed
		if slices.Contains(idxUncoreMatches, inGroupIdx) {
			continue
		}
		idxUncoreMatches = []int{}
		foundUncore := false
		for eventName := range inGroup.EventValues {
			// only check the first entry
			if strings.HasPrefix(eventName, "UNC") {
				foundUncore = true
			}
			break
		}
		if foundUncore {
			// we need to know how many of the following groups (if any) match the current group
			// so they can be merged together into a single group
			for i := inGroupIdx + 1; i < len(inFrame.EventGroups); i++ {
				if isMatchingGroup(inGroup, inFrame.EventGroups[i]) {
					// keep track of the groups that match so we can skip processing them since
					// they will be merged into a single group
					idxUncoreMatches = append(idxUncoreMatches, i)
				} else {
					break
				}
			}
			var outGroup EventGroup
			if outGroup, err = collapseUncoreGroups(inFrame.EventGroups, inGroupIdx, len(idxUncoreMatches)); err != nil {
				return
			}
			outFrame.EventGroups = append(outFrame.EventGroups, outGroup)
		} else {
			outFrame.EventGroups = append(outFrame.EventGroups, inGroup)
		}
	}
	return
}

// isMatchingGroup - groups are considered matching if they include the same event names (ignoring .ID suffix)
func isMatchingGroup(groupA, groupB EventGroup) bool {
	if len(groupA.EventValues) != len(groupB.EventValues) {
		return false
	}
	aNames := make([]string, 0, len(groupA.EventValues))
	bNames := make([]string, 0, len(groupB.EventValues))
	for eventAName := range groupA.EventValues {
		parts := strings.Split(eventAName, ".")
		newName := strings.Join(parts[:len(parts)-1], ".")
		aNames = append(aNames, newName)
	}
	for eventBName := range groupB.EventValues {
		parts := strings.Split(eventBName, ".")
		newName := strings.Join(parts[:len(parts)-1], ".")
		bNames = append(bNames, newName)
	}
	slices.Sort(aNames)
	slices.Sort(bNames)
	for nameIdx, name := range aNames {
		if name != bNames[nameIdx] {
			return false
		}
	}
	return true
}

// collapseUncoreGroups collapses a list of groups into a single group
func collapseUncoreGroups(inGroups []EventGroup, firstIdx int, count int) (outGroup EventGroup, err error) {
	outGroup.GroupID = inGroups[firstIdx].GroupID
	outGroup.Percentage = inGroups[firstIdx].Percentage
	outGroup.EventValues = make(map[string]float64)
	for i := firstIdx; i <= firstIdx+count; i++ {
		for name, value := range inGroups[i].EventValues {
			parts := strings.Split(name, ".")
			newName := strings.Join(parts[:len(parts)-1], ".")
			if _, ok := outGroup.EventValues[newName]; !ok {
				outGroup.EventValues[newName] = 0
			}
			outGroup.EventValues[newName] += value
		}
	}
	return
}

// parseEventJSON parses JSON formatted event into struct
// example: {"interval" : 5.005113019, "cpu": "0", "counter-value" : "22901873.000000", "unit" : "", "cgroup" : "...1cb2de.scope", "event" : "L1D.REPLACEMENT", "event-runtime" : 80081151765, "pcnt-running" : 6.00, "metric-value" : 0.000000, "metric-unit" : "(null)"}
func parseEventJSON(rawEvent []byte) (Event, error) {
	var event Event
	if err := json.Unmarshal(rawEvent, &event); err != nil {
		err = fmt.Errorf("unrecognized event format: \"%s\"", rawEvent)
		return event, err
	}
	if !strings.Contains(event.CounterValue, "not counted") && !strings.Contains(event.CounterValue, "not supported") {
		var err error
		if event.Value, err = strconv.ParseFloat(event.CounterValue, 64); err != nil {
			slog.Error("failed to parse event value", slog.String("event", event.Event), slog.String("value", event.CounterValue))
			event.Value = math.NaN()
		}
	}
	return event, nil
}
