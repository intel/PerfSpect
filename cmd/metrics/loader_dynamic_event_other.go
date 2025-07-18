package metrics

type OtherEvent struct {
	EventName string `json:"EventName"`
}

func (event OtherEvent) StringForPerf() (string, error) {
	// For other events, we just return the event name as is.
	// This is used for events that are not part of core or uncore events.
	return event.EventName, nil
}
