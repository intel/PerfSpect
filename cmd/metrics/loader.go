package metrics

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Knetic/govaluate"
)

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
	case "gnrxxx", "srf", "emr", "spr", "icx", "clx", "skx", "bdx", "bergamo", "genoa", "turin":
		return newStaticLoader(strings.ToLower(uarch)), nil
	case "gnr":
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

//
// Retire Latency Files
//

type PlatformInfo struct {
	ModelName      string `json:"Model name"`
	CPUFamily      string `json:"CPU family"`
	Model          string `json:"Model"`
	ThreadsPerCore string `json:"Thread(s) per core"`
	CoresPerSocket string `json:"Core(s) per socket"`
	Sockets        string `json:"Socket(s)"`
	Stepping       string `json:"Stepping"`
	L3Cache        string `json:"L3 cache"`
	NUMANodes      string `json:"NUMA node(s)"`
	TMAVersion     string `json:"TMA version"`
}

type MetricStats struct {
	Min  float64 `json:"MIN"`
	Max  float64 `json:"MAX"`
	Mean float64 `json:"MEAN"`
}

type RetireLatency struct {
	Platform PlatformInfo           `json:"Platform"`
	Data     map[string]MetricStats `json:"Data"`
}

func loadRetireLatencies(metadata Metadata) (retireLatencies map[string]string, err error) {
	uarch := strings.ToLower(strings.Split(metadata.Microarchitecture, "_")[0])
	uarch = strings.Split(uarch, " ")[0]
	filename := fmt.Sprintf("%s_retire_latency.json", uarch)
	var bytes []byte
	if bytes, err = resources.ReadFile(filepath.Join("resources", "metrics", metadata.Architecture, metadata.Vendor, filename)); err != nil {
		// not all architectures have retire latencies defined
		err = nil
		return
	}
	var retireLatency RetireLatency
	if err = json.Unmarshal(bytes, &retireLatency); err != nil {
		slog.Error("failed to unmarshal retire latencies", slog.String("error", err.Error()))
		return
	}
	// create a map of retire latencies
	retireLatencies = make(map[string]string)
	for event, stats := range retireLatency.Data {
		// use the mean value for the retire latency
		retireLatencies[event] = fmt.Sprintf("%f", stats.Mean)
	}
	slog.Debug("loaded retire latencies", slog.Any("latencies", retireLatencies))
	return
}

//
// common functions
//

// perfmonToPerfspectConditional transforms if/else to ternary conditional (? :) so expression evaluator can handle it
// simple:
// from: <expression 1> if <condition> else <expression 2>
// to:   <condition> ? <expression 1> : <expression 2>
// less simple:
// from: <expression 0> ((<expression 1>) if <condition> else (<expression 2>)) <expression 3>
// to:   <expression 0> (<condition> ? (<expression 1>) : <expression 2) <expression 3>
func perfmonToPerfspectConditional(origIn string) (out string, err error) {
	numIfs := strings.Count(origIn, "if")
	if numIfs == 0 {
		out = origIn
		return
	}
	in := origIn
	for i := range numIfs {
		if i > 0 {
			in = out
		}
		var idxIf, idxElse, idxExpression1, idxExpression3 int
		if idxIf = strings.Index(in, "if"); idxIf == -1 {
			err = fmt.Errorf("didn't find expected if: %s", in)
			return
		}
		if idxElse = strings.Index(in, "else"); idxElse == -1 {
			err = fmt.Errorf("if without else in expression: %s", in)
			return
		}
		// find the beginning of expression 1 (also end of expression 0)
		var parens int
		for i := idxIf - 1; i >= 0; i-- {
			c := in[i]
			switch c {
			case ')':
				parens += 1
			case '(':
				parens -= 1
			default:
				continue
			}
			if parens < 0 {
				idxExpression1 = i + 1
				break
			}
		}
		// find the end of expression 2 (also beginning of expression 3)
		parens = 0
		for i, c := range in[idxElse+5:] {
			switch c {
			case '(':
				parens += 1
			case ')':
				parens -= 1
			default:
				continue
			}
			if parens < 0 {
				idxExpression3 = i + idxElse + 6
				break
			}
		}
		if idxExpression3 == 0 {
			idxExpression3 = len(in)
		}
		expression0 := in[:idxExpression1]
		expression1 := in[idxExpression1 : idxIf-1]
		condition := in[idxIf+3 : idxElse-1]
		expression2 := in[idxElse+5 : idxExpression3]
		expression3 := in[idxExpression3:]
		var space0, space3 string
		if expression0 != "" {
			space0 = " "
		}
		if expression3 != "" {
			space3 = " "
		}
		out = fmt.Sprintf("%s%s%s ? %s : %s%s%s", expression0, space0, condition, expression1, expression2, space3, expression3)
	}
	return
}

// configureMetrics prepares metrics for use by the evaluator, by e.g., replacing
// metric constants with known values and aligning metric variables to perf event
// groups
func configureMetrics(loadedMetrics []MetricDefinition, uncollectableEvents []string, metadata Metadata) (metrics []MetricDefinition, err error) {
	// get constants as strings
	tscFreq := fmt.Sprintf("%f", float64(metadata.TSCFrequencyHz))
	var tsc string
	switch flagGranularity {
	case granularitySystem:
		tsc = fmt.Sprintf("%f", float64(metadata.TSC))
	case granularitySocket:
		tsc = fmt.Sprintf("%f", float64(metadata.TSC)/float64(metadata.SocketCount))
	case granularityCPU:
		tsc = fmt.Sprintf("%f", float64(metadata.TSC)/(float64(metadata.SocketCount*metadata.CoresPerSocket*metadata.ThreadsPerCore)))
	default:
		err = fmt.Errorf("unknown granularity: %s", flagGranularity)
		return
	}
	coresPerSocket := fmt.Sprintf("%f", float64(metadata.CoresPerSocket))
	chasPerSocket := fmt.Sprintf("%f", float64(len(metadata.UncoreDeviceIDs["cha"])))
	socketCount := fmt.Sprintf("%f", float64(metadata.SocketCount))
	hyperThreadingOn := fmt.Sprintf("%t", metadata.ThreadsPerCore > 1)
	threadsPerCore := fmt.Sprintf("%f", float64(metadata.ThreadsPerCore))
	// load retire latency constants
	var retireLatencies map[string]string
	if retireLatencies, err = loadRetireLatencies(metadata); err != nil {
		slog.Error("failed to load retire latencies", slog.String("error", err.Error()))
		return
	}
	// configure each metric
	reConstantInt := regexp.MustCompile(`\[(\d+)\]`)
	evaluatorFunctions := getEvaluatorFunctions()
	for metricIdx := range loadedMetrics {
		tmpMetric := loadedMetrics[metricIdx]
		// skip metrics that use uncollectable events
		foundUncollectable := false
		for _, uncollectableEvent := range uncollectableEvents {
			if strings.Contains(tmpMetric.Expression, uncollectableEvent) {
				slog.Debug("removing metric that uses uncollectable event", slog.String("metric", tmpMetric.Name), slog.String("event", uncollectableEvent))
				foundUncollectable = true
				break
			}
		}
		if foundUncollectable {
			continue
		}
		// transform if/else to ?/:
		var transformed string
		if transformed, err = perfmonToPerfspectConditional(tmpMetric.Expression); err != nil {
			return
		}
		// replace "> =" with ">=" and "< =" with "<="
		transformed = strings.ReplaceAll(transformed, "> =", ">=")
		transformed = strings.ReplaceAll(transformed, "< =", "<=")
		if transformed != tmpMetric.Expression {
			slog.Debug("transformed metric", slog.String("metric name", tmpMetric.Name), slog.String("transformed", transformed))
			tmpMetric.Expression = transformed
		}
		// replace constants with their values
		tmpMetric.Expression = strings.ReplaceAll(tmpMetric.Expression, "[SYSTEM_TSC_FREQ]", tscFreq)
		tmpMetric.Expression = strings.ReplaceAll(tmpMetric.Expression, "[TSC]", tsc)
		tmpMetric.Expression = strings.ReplaceAll(tmpMetric.Expression, "[CORES_PER_SOCKET]", coresPerSocket)
		tmpMetric.Expression = strings.ReplaceAll(tmpMetric.Expression, "[CHAS_PER_SOCKET]", chasPerSocket)
		tmpMetric.Expression = strings.ReplaceAll(tmpMetric.Expression, "[SOCKET_COUNT]", socketCount)
		tmpMetric.Expression = strings.ReplaceAll(tmpMetric.Expression, "[HYPERTHREADING_ON]", hyperThreadingOn)
		tmpMetric.Expression = strings.ReplaceAll(tmpMetric.Expression, "[CONST_THREAD_COUNT]", threadsPerCore)
		tmpMetric.Expression = strings.ReplaceAll(tmpMetric.Expression, "[TXN]", fmt.Sprintf("%f", flagTransactionRate))
		// replace retire latencies
		for retireEvent, retireLatency := range retireLatencies {
			// replace <event>:retire_latency with value
			tmpMetric.Expression = strings.ReplaceAll(tmpMetric.Expression, fmt.Sprintf("[%s:retire_latency]", retireEvent), retireLatency)
		}
		// replace constant numbers masquerading as variables with their values, e.g., [20] -> 20
		// there may be more than one with differing values in the expression, so use a regex to find them all
		for {
			// find the first match
			found := reConstantInt.FindStringSubmatchIndex(tmpMetric.Expression)
			if found == nil {
				break // no more matches
			}
			// match[2] is the start of the number, match[3] is the end of the number
			number := tmpMetric.Expression[found[2]:found[3]]
			// replace the whole match with the number
			tmpMetric.Expression = strings.ReplaceAll(tmpMetric.Expression, tmpMetric.Expression[found[0]:found[1]], number)
		}
		// get a list of the variables in the expression
		tmpMetric.Variables = make(map[string]int)
		expressionIdx := 0
		for {
			startVar := strings.IndexRune(tmpMetric.Expression[expressionIdx:], '[')
			if startVar == -1 { // no more vars in this expression
				break
			}
			endVar := strings.IndexRune(tmpMetric.Expression[expressionIdx:], ']')
			if endVar == -1 {
				err = fmt.Errorf("didn't find end of variable indicator (]) in expression: %s", tmpMetric.Expression[expressionIdx:])
				return
			}
			// add the variable name to the map, set group index to -1 to indicate it has not yet been determined
			tmpMetric.Variables[tmpMetric.Expression[expressionIdx:][startVar+1:endVar]] = -1
			expressionIdx += endVar + 1
		}
		if tmpMetric.Evaluable, err = govaluate.NewEvaluableExpressionWithFunctions(tmpMetric.Expression, evaluatorFunctions); err != nil {
			slog.Error("failed to create evaluable expression for metric", slog.String("error", err.Error()), slog.String("metric name", tmpMetric.Name), slog.String("metric expression", tmpMetric.Expression))
			return
		}
		metrics = append(metrics, tmpMetric)
	}
	return
}

// getEvaluatorFunctions defines functions that can be called in metric expressions
func getEvaluatorFunctions() (functions map[string]govaluate.ExpressionFunction) {
	functions = make(map[string]govaluate.ExpressionFunction)
	functions["max"] = func(args ...any) (any, error) {
		var leftVal float64
		var rightVal float64
		switch t := args[0].(type) {
		case int:
			leftVal = float64(t)
		case float64:
			leftVal = t
		}
		switch t := args[1].(type) {
		case int:
			rightVal = float64(t)
		case float64:
			rightVal = t
		}
		return max(leftVal, rightVal), nil
	}
	functions["min"] = func(args ...any) (any, error) {
		var leftVal float64
		var rightVal float64
		switch t := args[0].(type) {
		case int:
			leftVal = float64(t)
		case float64:
			leftVal = t
		}
		switch t := args[1].(type) {
		case int:
			rightVal = float64(t)
		case float64:
			rightVal = t
		}
		return min(leftVal, rightVal), nil
	}
	return
}
