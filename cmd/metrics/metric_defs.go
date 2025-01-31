package metrics

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"perfspect/internal/util"

	"github.com/Knetic/govaluate"
)

type Variable struct {
	Name          string
	EventGroupIdx int // initialized to -1 to indicate that a group has not yet been identified
}

type MetricDefinition struct {
	Name          string                         `json:"name"`
	Expression    string                         `json:"expression"`
	NameTxn       string                         `json:"name-txn"`
	ExpressionTxn string                         `json:"expression-txn"`
	Variables     map[string]int                 // parsed from Expression for efficiency, int represents group index
	Evaluable     *govaluate.EvaluableExpression // parse expression once, store here for use in metric evaluation
}

// define const metric name prefix
const metricPrefix = "metric_"

// LoadMetricDefinitions reads and parses metric definitions from an architecture-specific metric
// definition file. When the override path argument is empty, the function will load metrics from
// the file associated with the platform's architecture found in the provided metadata. When
// a list of metric names is provided, only those metric definitions will be loaded.
func LoadMetricDefinitions(metricDefinitionOverridePath string, selectedMetrics []string, metadata Metadata) (metrics []MetricDefinition, err error) {
	var bytes []byte
	if metricDefinitionOverridePath != "" {
		if bytes, err = os.ReadFile(metricDefinitionOverridePath); err != nil {
			return
		}
	} else {
		uarch := strings.ToLower(strings.Split(metadata.Microarchitecture, "_")[0])
		// use alternate events/metrics when TMA fixed counters are not supported
		alternate := ""
		if (uarch == "icx" || uarch == "spr" || uarch == "emr") && !metadata.SupportsFixedTMA {
			alternate = "_nofixedtma"
		}
		metricFileName := fmt.Sprintf("%s%s.json", uarch, alternate)
		if bytes, err = resources.ReadFile(filepath.Join("resources", "metrics", metadata.Architecture, metadata.Vendor, metricFileName)); err != nil {
			return
		}
	}
	var metricsInFile []MetricDefinition
	if err = json.Unmarshal(bytes, &metricsInFile); err != nil {
		return
	}
	// if a list of metric names provided, reduce list to match
	if len(selectedMetrics) > 0 {
		// confirm provided metric names are valid (included in metrics defined in file)
		for _, metricName := range selectedMetrics {
			found := false
			for _, metric := range metricsInFile {
				if metricPrefix+metricName == metric.Name || metricName == metric.Name {
					found = true
					break
				}
			}
			if !found {
				err = fmt.Errorf("provided metric name not found: %s", metricName)
				return
			}
		}
		// build list of metrics based on provided list of metric names
		for _, metric := range metricsInFile {
			trimmedName := strings.TrimPrefix(metric.Name, metricPrefix)
			if !util.StringInList(trimmedName, selectedMetrics) {
				continue
			}
			metrics = append(metrics, metric)
		}
	} else {
		metrics = metricsInFile
	}
	return
}

// ConfigureMetrics prepares metrics for use by the evaluator, by e.g., replacing
// metric constants with known values and aligning metric variables to perf event
// groups
func ConfigureMetrics(loadedMetrics []MetricDefinition, uncollectableEvents []string, evaluatorFunctions map[string]govaluate.ExpressionFunction, metadata Metadata) (metrics []MetricDefinition, err error) {
	// get constants as strings
	tscFreq := fmt.Sprintf("%f", float64(metadata.TSCFrequencyHz))
	var tsc string
	if flagGranularity == granularitySystem {
		tsc = fmt.Sprintf("%f", float64(metadata.TSC))
	} else if flagGranularity == granularitySocket {
		tsc = fmt.Sprintf("%f", float64(metadata.TSC)/float64(metadata.SocketCount))
	} else if flagGranularity == granularityCPU {
		tsc = fmt.Sprintf("%f", float64(metadata.TSC)/(float64(metadata.SocketCount*metadata.CoresPerSocket*metadata.ThreadsPerCore)))
	} else {
		err = fmt.Errorf("unknown granularity: %s", flagGranularity)
		return
	}

	coresPerSocket := fmt.Sprintf("%f", float64(metadata.CoresPerSocket))
	chasPerSocket := fmt.Sprintf("%f", float64(len(metadata.UncoreDeviceIDs["cha"])))
	socketCount := fmt.Sprintf("%f", float64(metadata.SocketCount))
	hyperThreadingOn := fmt.Sprintf("%t", metadata.ThreadsPerCore > 1)
	threadsPerCore := fmt.Sprintf("%f", float64(metadata.ThreadsPerCore))
	// configure each metric
	for metricIdx := range loadedMetrics {
		tmpMetric := loadedMetrics[metricIdx]
		// abbreviate event names in metric expressions to match abbreviations used in uncollectableEvents
		tmpMetric.Expression = abbreviateEventName(tmpMetric.Expression)
		tmpMetric.ExpressionTxn = abbreviateEventName(tmpMetric.ExpressionTxn)
		// skip metrics that use uncollectable events
		foundUncollectable := false
		for _, uncollectableEvent := range uncollectableEvents {
			if strings.Contains(tmpMetric.Expression, uncollectableEvent) {
				slog.Warn("removing metric that uses uncollectable event", slog.String("metric", tmpMetric.Name), slog.String("event", uncollectableEvent))
				foundUncollectable = true
				break
			}
		}
		if foundUncollectable {
			continue
		}
		// swap in per-txn metric definition if transaction rate is provided
		if flagTransactionRate != 0 && tmpMetric.ExpressionTxn != "" {
			tmpMetric.Expression = tmpMetric.ExpressionTxn
			tmpMetric.Expression = strings.ReplaceAll(tmpMetric.Expression, "[TXN]", fmt.Sprintf("%f", flagTransactionRate))
			tmpMetric.Name = tmpMetric.NameTxn
		}
		// remove "metric_" prefix from metric names
		tmpMetric.Name = strings.TrimPrefix(tmpMetric.Name, metricPrefix)
		// transform if/else to ?/:
		var transformed string
		if transformed, err = transformConditional(tmpMetric.Expression); err != nil {
			return
		}
		if transformed != tmpMetric.Expression {
			slog.Debug("transformed metric", slog.String("original", tmpMetric.Name), slog.String("transformed", transformed))
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

// transformConditional transforms if/else to ternary conditional (? :) so expression evaluator can handle it
// simple:
// from: <expression 1> if <condition> else <expression 2>
// to:   <condition> ? <expression 1> : <expression 2>
// less simple:
// from: <expression 0> ((<expression 1>) if <condition> else (<expression 2>)) <expression 3>
// to:   <expression 0> (<condition> ? (<expression 1>) : <expression 2) <expression 3>
func transformConditional(origIn string) (out string, err error) {
	numIfs := strings.Count(origIn, "if")
	if numIfs == 0 {
		out = origIn
		return
	}
	in := origIn
	for i := 0; i < numIfs; i++ {
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
			if c == ')' {
				parens += 1
			} else if c == '(' {
				parens -= 1
			} else {
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
			if c == '(' {
				parens += 1
			} else if c == ')' {
				parens -= 1
			} else {
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
