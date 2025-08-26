package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"perfspect/internal/util"
	"regexp"
	"strings"

	"github.com/Knetic/govaluate"
)

// configureMetrics configures the metrics for use
func configureMetrics(metrics []MetricDefinition, uncollectableEvents []string, metadata Metadata) ([]MetricDefinition, error) {
	var err error
	// remove metrics that use uncollectable events
	if flagTransactionRate == 0 {
		uncollectableEvents = append(uncollectableEvents, "TXN") // if transaction rate is not set, remove TXN event
	}
	metrics, err = removeIfUncollectableEvents(metrics, uncollectableEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to remove uncollectable events: %w", err)
	}

	// replace constants variables with their values
	metrics, err = replaceConstants(metrics, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to replace constants: %w", err)
	}

	// transform metric expressions from perfmon format to perfspect format
	metrics, err = transformMetricExpressions(metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to transform metric expressions: %w", err)
	}

	// replace constant numbers masquerading as variables with their values
	metrics, err = replaceConstantNumbers(metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to replace constant numbers: %w", err)
	}

	// set evaluable expressions for each metric
	metrics, err = setEvaluableExpressions(metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to set evaluable expressions: %w", err)
	}

	// initialize metric variables
	metrics, err = initializeMetricVariables(metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metric variables: %w", err)
	}

	// initialize threshold variables
	metrics, err = initializeThresholdVariables(metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize threshold variables: %w", err)
	}

	// remove "metric_" prefix from metric names
	metrics, err = removeMetricsPrefix(metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to remove metrics prefix: %w", err)
	}

	return metrics, nil
}

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

// replaceConstants replaces constant variables in expressions with their values
func replaceConstants(metrics []MetricDefinition, metadata Metadata) ([]MetricDefinition, error) {
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
		return nil, fmt.Errorf("unknown granularity: %s", flagGranularity)
	}
	coresPerSocket := fmt.Sprintf("%f", float64(metadata.CoresPerSocket))
	chasPerSocket := fmt.Sprintf("%f", float64(len(metadata.UncoreDeviceIDs["cha"])))
	socketCount := fmt.Sprintf("%f", float64(metadata.SocketCount))
	hyperThreadingOn := fmt.Sprintf("%t", metadata.ThreadsPerCore > 1)
	threadsPerCore := fmt.Sprintf("%f", float64(metadata.ThreadsPerCore))

	for i := range metrics {
		metric := &metrics[i]
		// replace constants with their values
		metric.Expression = strings.ReplaceAll(metric.Expression, "[SYSTEM_TSC_FREQ]", tscFreq)
		metric.Expression = strings.ReplaceAll(metric.Expression, "[TSC]", tsc)
		metric.Expression = strings.ReplaceAll(metric.Expression, "[CORES_PER_SOCKET]", coresPerSocket)
		metric.Expression = strings.ReplaceAll(metric.Expression, "[CHAS_PER_SOCKET]", chasPerSocket)
		metric.Expression = strings.ReplaceAll(metric.Expression, "[SOCKET_COUNT]", socketCount)
		metric.Expression = strings.ReplaceAll(metric.Expression, "[HYPERTHREADING_ON]", hyperThreadingOn)
		metric.Expression = strings.ReplaceAll(metric.Expression, "[CONST_THREAD_COUNT]", threadsPerCore)
		metric.Expression = strings.ReplaceAll(metric.Expression, "[TXN]", fmt.Sprintf("%f", flagTransactionRate))
	}
	return metrics, nil
}

// removeIfUncollectableEvents removes metrics that use uncollectable events
func removeIfUncollectableEvents(metrics []MetricDefinition, uncollectableEvents []string) ([]MetricDefinition, error) {
	// remove metrics that use uncollectable events
	var filteredMetrics []MetricDefinition
	for i := range metrics {
		metric := &metrics[i]
		foundUncollectable := false
		for _, uncollectableEvent := range uncollectableEvents {
			if strings.Contains(metric.Expression, uncollectableEvent) {
				slog.Debug("removing metric that uses uncollectable event", slog.String("metric", metric.Name), slog.String("event", uncollectableEvent))
				foundUncollectable = true
				break
			}
		}
		if !foundUncollectable {
			filteredMetrics = append(filteredMetrics, *metric)
		}
	}
	return filteredMetrics, nil
}

func transformExpression(expression string) (string, error) {
	// transform if/else to ?/:
	transformed, err := perfmonToPerfspectConditional(expression)
	if err != nil {
		return "", fmt.Errorf("failed to transform metric expression: %w", err)
	}
	// replace "> =" with ">=" and "< =" with "<="
	transformed = strings.ReplaceAll(transformed, "> =", ">=")
	transformed = strings.ReplaceAll(transformed, "< =", "<=")
	// replace "&" with "&&" and "|" with "||"
	transformed = strings.ReplaceAll(transformed, " & ", " && ")
	transformed = strings.ReplaceAll(transformed, " | ", " || ")
	return transformed, nil
}

// transformMetricExpressions transforms metric expressions from perfmon format to perfspect format
// it replaces if/else with ternary conditional, replaces "> =" with ">=", and "< =" with "<="
func transformMetricExpressions(metrics []MetricDefinition) ([]MetricDefinition, error) {
	var transformedMetrics []MetricDefinition
	for i := range metrics {
		metric := &metrics[i]
		var err error
		metric.Expression, err = transformExpression(metric.Expression)
		if err != nil {
			return nil, fmt.Errorf("failed to transform metric expression: %w", err)
		}
		if metric.ThresholdExpression != "" {
			metric.ThresholdExpression, err = transformExpression(metric.ThresholdExpression)
			if err != nil {
				return nil, fmt.Errorf("failed to transform threshold expression: %w", err)
			}
		}
		// add the transformed metric to the list
		transformedMetrics = append(transformedMetrics, *metric)
	}
	return transformedMetrics, nil
}

// setEvaluableExpressions sets the EvaluableExpression for each metric
// this allows the expression to be evaluated later without parsing it again
func setEvaluableExpressions(metrics []MetricDefinition) ([]MetricDefinition, error) {
	evaluatorFunctions := getEvaluatorFunctions()
	for i := range metrics {
		metric := &metrics[i]
		var err error
		if metric.Evaluable, err = govaluate.NewEvaluableExpressionWithFunctions(metric.Expression, evaluatorFunctions); err != nil {
			slog.Error("failed to create evaluable expression for metric", slog.String("error", err.Error()), slog.String("name", metric.Name), slog.String("expression", metric.Expression))
			return nil, err
		}
		if metric.ThresholdExpression != "" {
			if metric.ThresholdEvaluable, err = govaluate.NewEvaluableExpressionWithFunctions(metric.ThresholdExpression, evaluatorFunctions); err != nil {
				slog.Error("failed to create threshold evaluable expression for metric", slog.String("error", err.Error()), slog.String("name", metric.Name), slog.String("threshold_expression", metric.ThresholdExpression))
				return nil, err
			}
		}
	}
	return metrics, nil
}

// replaceConstantNumbers replaces constant numbers masquerading as variables with their values, e.g., [20] -> 20
// there may be more than one with differing values in the expression, so use a regex to find them all
func replaceConstantNumbers(metrics []MetricDefinition) ([]MetricDefinition, error) {
	reConstantInt := regexp.MustCompile(`\[(\d+)\]`)
	for i := range metrics {
		metric := &metrics[i]
		for {
			// find the first match
			found := reConstantInt.FindStringSubmatchIndex(metric.Expression)
			if found == nil {
				break // no more matches
			}
			// match[2] is the start of the number, match[3] is the end of the number
			number := metric.Expression[found[2]:found[3]]
			// replace the whole match with the number
			metric.Expression = strings.ReplaceAll(metric.Expression, metric.Expression[found[0]:found[1]], number)
		}
	}
	return metrics, nil
}

// initializeMetricVariables initializes the Variables map for each metric
// it parses the expression and finds all variables in the form [variable_name]
// the variable name is stored in the map with a value of -1 to indicate it has not yet been determined
// the value will be set later when the group index is determined
func initializeMetricVariables(metrics []MetricDefinition) ([]MetricDefinition, error) {
	// get a list of the variables in the expression
	for i := range metrics {
		metric := &metrics[i]
		metric.Variables = make(map[string]int)
		expressionIdx := 0
		for {
			startVar := strings.IndexRune(metric.Expression[expressionIdx:], '[')
			if startVar == -1 { // no more vars in this expression
				break
			}
			endVar := strings.IndexRune(metric.Expression[expressionIdx:], ']')
			if endVar == -1 {
				return nil, fmt.Errorf("didn't find end of variable indicator (]) in expression: %s", metric.Expression[expressionIdx:])
			}
			// add the variable name to the map, set group index to -1 to indicate it has not yet been determined
			metric.Variables[metric.Expression[expressionIdx:][startVar+1:endVar]] = -1
			expressionIdx += endVar + 1
		}
	}
	return metrics, nil
}

func initializeThresholdVariables(metrics []MetricDefinition) ([]MetricDefinition, error) {
	// get a list of the variables in the threshold expression
	for i := range metrics {
		metric := &metrics[i]
		metric.ThresholdVariables = []string{}
		expressionIdx := 0
		for {
			startVar := strings.IndexRune(metric.ThresholdExpression[expressionIdx:], '[')
			if startVar == -1 { // no more vars in this expression
				break
			}
			endVar := strings.IndexRune(metric.ThresholdExpression[expressionIdx:], ']')
			if endVar == -1 {
				return nil, fmt.Errorf("didn't find end of variable indicator (]) in threshold expression: %s", metric.ThresholdExpression[expressionIdx:])
			}
			// add the variable name to the list if it is not already present
			varName := metric.ThresholdExpression[expressionIdx:][startVar+1 : endVar]
			metric.ThresholdVariables = util.UniqueAppend(metric.ThresholdVariables, varName)
			expressionIdx += endVar + 1
		}
	}
	return metrics, nil
}

// removeMetricsPrefix removes the "metric_" prefix from metric names
// this is done to make the names more readable and consistent with other metrics
// it is assumed that all metric names start with "metric_"
func removeMetricsPrefix(metrics []MetricDefinition) ([]MetricDefinition, error) {
	for i := range metrics {
		metric := &metrics[i]
		metric.Name = strings.TrimPrefix(metric.Name, "metric_")
	}
	return metrics, nil
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
