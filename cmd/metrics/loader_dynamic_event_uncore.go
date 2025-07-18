package metrics

import (
	"encoding/json"
	"fmt"
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
	UniqueID          string // This field is not in the JSON. We set it to a unique value after unmarshaling.
}

type UncoreEvents struct {
	Header map[string]string `json:"Header"`
	Events []UncoreEvent     `json:"Events"`
}

func NewUncoreEvents(path string) (UncoreEvents, error) {
	var events UncoreEvents
	bytes, err := resources.ReadFile(path)
	if err != nil {
		return events, fmt.Errorf("error reading file %s: %w", path, err)
	}
	if err := json.Unmarshal(bytes, &events); err != nil {
		return events, fmt.Errorf("error unmarshaling JSON from %s: %w", path, err)
	}
	for i := range events.Events {
		// Set the UniqueID for each event. We use this when generating perf strings
		// instead of the EventName to reduce the length of the perf command line.
		events.Events[i].UniqueID = fmt.Sprintf("uce_%03d", i)
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

func (event UncoreEvent) StringForPerf() (string, error) {
	if event == (UncoreEvent{}) {
		return "", fmt.Errorf("event is not initialized")
	}
	if event.EventCode == "" {
		return "", fmt.Errorf("event %s does not have an EventCode", event.EventName)
	}
	// name is the uniqueID.deviceID
	if event.UniqueID == "" {
		return "", fmt.Errorf("event %s does not have a UniqueID", event.EventName)
	}
	// parse the device ID out of the event name
	eventNameParts := strings.Split(event.EventName, ".")
	if len(eventNameParts) < 2 {
		return "", fmt.Errorf("event %s does not have a device ID in its name", event.EventName)
	}
	deviceID := eventNameParts[len(eventNameParts)-1]
	if deviceID == "" {
		return "", fmt.Errorf("event %s does not have a device ID", event.EventName)
	}
	var parts []string
	// unit/event
	parts = append(parts, fmt.Sprintf("uncore_%s_%s/event=%s", strings.ToLower(strings.Split(event.Unit, " ")[0]), deviceID, event.EventCode))
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
	parts = append(parts, fmt.Sprintf("name='%s.%s'/", event.UniqueID, deviceID))
	return strings.Join(parts, ","), nil
}
