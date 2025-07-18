package metrics

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

type OtherGroup struct {
	MaxGeneralPurposeCounters int
	GeneralPurposeCounters    map[int]OtherEvent
	MetricNames               []string
}

func NewOtherGroup(maxGeneralPurposeCounters int) OtherGroup {
	return OtherGroup{
		MaxGeneralPurposeCounters: maxGeneralPurposeCounters,
		GeneralPurposeCounters:    make(map[int]OtherEvent, 0),
	}
}

func (group OtherGroup) ToGroupDefinition() GroupDefinition {
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

func (group OtherGroup) AddEvent(event OtherEvent, _ bool) error {
	group.GeneralPurposeCounters[len(group.GeneralPurposeCounters)] = event
	return nil
}

func (group OtherGroup) Print(w io.Writer) {
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
		fmt.Fprintf(w, "    Counter %d: %s\n", k, event.EventName)
	}
}

func (group OtherGroup) StringForPerf() (string, error) {
	var formattedEvents []string
	for _, event := range group.GeneralPurposeCounters {
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
