package metrics

import (
	"fmt"
	"strings"

	"github.com/Knetic/govaluate"
)

type Variable struct {
	Name          string
	EventGroupIdx int // initialized to -1 to indicate that a group has not yet been identified
}

type MetricDefinition struct {
	Name        string                         `json:"name"`
	Expression  string                         `json:"expression"`
	Description string                         `json:"description"`
	Variables   map[string]int                 // parsed from Expression for efficiency, int represents group index
	Evaluable   *govaluate.EvaluableExpression // parse expression once, store here for use in metric evaluation
}

// EventDefinition represents a single perf event
type EventDefinition struct {
	Raw    string
	Name   string
	Device string
}

// GroupDefinition represents a group of perf events
type GroupDefinition []EventDefinition

type Loader interface {
	Load(metricDefinitionOverridePath string, eventDefinitionOverridePath string, selectedMetrics []string, metadata Metadata) (metrics []MetricDefinition, groups []GroupDefinition, err error)
}

type BaseLoader struct {
	microarchitecture string
}

type StaticLoader struct {
	BaseLoader
}

type DynamicLoader struct {
	BaseLoader
}

func NewLoader(uarch string) (Loader, error) {
	switch strings.ToLower(uarch) {
	case "gnrxxx", "srf", "emrxx", "spr", "icx", "clx", "skx", "bdx", "bergamo", "genoa", "turin":
		return newStaticLoader(strings.ToLower(uarch)), nil
	case "gnr", "emr":
		return newDynamicLoader(strings.ToLower(uarch)), nil
	default:
		return nil, fmt.Errorf("unsupported microarchitecture: %s", uarch)
	}
}

func newStaticLoader(uarch string) *StaticLoader {
	return &StaticLoader{
		BaseLoader: BaseLoader{
			microarchitecture: uarch,
		},
	}
}

func newDynamicLoader(uarch string) *DynamicLoader {
	return &DynamicLoader{
		BaseLoader: BaseLoader{
			microarchitecture: uarch,
		},
	}
}
