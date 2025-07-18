package metrics

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type CoreEvent struct {
	EventCode          string `json:"EventCode"`
	UMask              string `json:"UMask"`
	EventName          string `json:"EventName"`
	BriefDescription   string `json:"BriefDescription"`
	PublicDescription  string `json:"PublicDescription"`
	Counter            string `json:"Counter"`
	PEBScounters       string `json:"PEBScounters"`
	SampleAfterValue   string `json:"SampleAfterValue"`
	MSRIndex           string `json:"MSRIndex"`
	MSRValue           string `json:"MSRValue"`
	Precise            string `json:"Precise"`
	CollectPEBSRecords string `json:"CollectPEBSRecords"`
	TakenAlone         string `json:"TakenAlone"`
	CounterMask        string `json:"CounterMask"`
	Invert             string `json:"Invert"`
	EdgeDetect         string `json:"EdgeDetect"`
	DataLA             string `json:"Data_LA"`
	L1HitIndication    string `json:"L1_Hit_Indication"`
	Errata             string `json:"Errata"`
	Offcore            string `json:"Offcore"`
	Deprecated         string `json:"Deprecated"`
	PDISTCounter       string `json:"PDISTCounter"`
	Speculative        string `json:"Speculative"`
}

type CoreEvents struct {
	Header map[string]string `json:"Header"`
	Events []CoreEvent       `json:"Events"`
}

var perfMetricsEvents []CoreEvent = []CoreEvent{
	{EventName: "PERF_METRICS.RETIRING", EventCode: "0x00", UMask: "0x80", SampleAfterValue: "10000003"},
	{EventName: "PERF_METRICS.BAD_SPECULATION", EventCode: "0x00", UMask: "0x81", SampleAfterValue: "10000003"},
	{EventName: "PERF_METRICS.FRONTEND_BOUND", EventCode: "0x00", UMask: "0x82", SampleAfterValue: "10000003"},
	{EventName: "PERF_METRICS.BACKEND_BOUND", EventCode: "0x00", UMask: "0x83", SampleAfterValue: "10000003"},
	{EventName: "PERF_METRICS.HEAVY_OPERATIONS", EventCode: "0x00", UMask: "0x84", SampleAfterValue: "10000003"},
	{EventName: "PERF_METRICS.BRANCH_MISPREDICTS", EventCode: "0x00", UMask: "0x85", SampleAfterValue: "10000003"},
	{EventName: "PERF_METRICS.FETCH_LATENCY", EventCode: "0x00", UMask: "0x86", SampleAfterValue: "10000003"},
	{EventName: "PERF_METRICS.MEMORY_BOUND", EventCode: "0x00", UMask: "0x87", SampleAfterValue: "10000003"},
}

func NewCoreEvents(path string) (CoreEvents, error) {
	var events CoreEvents
	bytes, err := resources.ReadFile(path)
	if err != nil {
		return events, fmt.Errorf("error reading file %s: %w", path, err)
	}
	if err := json.Unmarshal(bytes, &events); err != nil {
		return events, fmt.Errorf("error unmarshaling JSON from %s: %w", path, err)
	}
	return events, nil
}

func (events CoreEvents) FindEventByName(eventName string) CoreEvent {
	// check if the event is a perf metrics event
	for _, perfEvent := range perfMetricsEvents {
		if perfEvent.EventName == eventName {
			// If the event is a perf metrics event, we return it directly.
			return perfEvent
		}
	}
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
	return CoreEvent{}
}

// perfmon event name to perf event name
var fixedCounterEventNameTranslation = map[string]string{
	"INST_RETIRED.ANY":                "instructions",
	"INST_RETIRED.ANY_P:SUP":          "instructions:k",
	"CPU_CLK_UNHALTED.THREAD":         "cpu-cycles",
	"CPU_CLK_UNHALTED.THREAD_P:SUP":   "cpu-cycles:k",
	"CPU_CLK_UNHALTED.REF_TSC":        "ref-cycles",
	"CPU_CLK_UNHALTED.REF_TSC_P:SUP":  "ref-cycles:k",
	"TOPDOWN.SLOTS:perf_metrics":      "topdown.slots",
	"PERF_METRICS.BAD_SPECULATION":    "topdown-bad-spec",
	"PERF_METRICS.BACKEND_BOUND":      "topdown-be-bound",
	"PERF_METRICS.BRANCH_MISPREDICTS": "topdown-br-mispredict",
	"PERF_METRICS.FRONTEND_BOUND":     "topdown-fe-bound",
	"PERF_METRICS.FETCH_LATENCY":      "topdown-fetch-lat",
	"PERF_METRICS.HEAVY_OPERATIONS":   "topdown-heavy-ops",
	"PERF_METRICS.MEMORY_BOUND":       "topdown-mem-bound",
	"PERF_METRICS.RETIRING":           "topdown-retiring",
}

func (event CoreEvent) StringForPerf() (string, error) {
	if event == (CoreEvent{}) {
		return "", fmt.Errorf("event is not initialized")
	}
	if translatedName, ok := fixedCounterEventNameTranslation[event.EventName]; ok {
		return translatedName, nil
	}
	var parts []string
	if event.EventCode != "" {
		// unit/event
		unit := "cpu"
		eventCode := event.EventCode
		// special handling of OCR events that have EventCode "0x2A,0x2B"
		// for lack of a better way to handle this, we will just use the first part
		if strings.Contains(event.EventCode, ",") {
			eventCode = strings.Split(event.EventCode, ",")[0]
		}
		parts = append(parts, fmt.Sprintf("%s/event=%s", strings.ToLower(unit), eventCode))
	}
	// umask
	if event.UMask != "" {
		parts = append(parts, fmt.Sprintf("umask=%s", event.UMask))
	}
	// cmask
	if event.CounterMask != "" {
		cmask, err := strconv.ParseInt(event.CounterMask, 10, 64)
		if err != nil {
			return "", fmt.Errorf("error parsing CounterMask %s for event %s: %w", event.CounterMask, event.EventName, err)
		}
		parts = append(parts, fmt.Sprintf("cmask=0x%02x", cmask))
	}
	//period
	if event.SampleAfterValue != "" {
		parts = append(parts, fmt.Sprintf("period=%s", event.SampleAfterValue))
	}
	// offcore_rsp, name
	if event.Offcore == "1" {
		// some offcore events have a MSR value appended to their name, like this:
		// OCR.DEMAND_RFO.L3_MISS:ocr_msr_val=0x103b8000. If we see this, set oc_rsp
		// to the ocr_msr_val and remove it from the name.
		name := event.EventName
		msr := event.MSRValue
		if strings.Contains(name, ":ocr_msr_val=") {
			// get the msr value
			parts := strings.Split(name, ":ocr_msr_val=")
			if len(parts) != 2 {
				return "", fmt.Errorf("error parsing offcore event name %s: expected format 'name:ocr_msr_val=msr_value'", name)
			}
			name = parts[0]
			msr = parts[1]
		}
		parts = append(parts, fmt.Sprintf("offcore_rsp=%s", msr))
		parts = append(parts, fmt.Sprintf("name='%s'/", name))
	} else {
		// name
		parts = append(parts, fmt.Sprintf("name='%s'/", event.EventName))
	}
	return strings.Join(parts, ","), nil
}
