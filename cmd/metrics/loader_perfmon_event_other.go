package metrics

type OtherEvent struct {
	EventName string `json:"EventName"`
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

func (event OtherEvent) IsCollectable(metadata Metadata) bool {
	return true
}

func (event OtherEvent) StringForPerf() (string, error) {
	// For other events, we just return the event name as is.
	// This is used for events that are not part of core or uncore events.
	return event.EventName, nil
}
