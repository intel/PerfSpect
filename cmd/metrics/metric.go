package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// metric generation type defintions and helper functions

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/Knetic/govaluate"
	mapset "github.com/deckarep/golang-set/v2"
)

// Metric represents a metric (name, value) derived from perf events
type Metric struct {
	Name  string
	Value float64
}

// MetricFrame represents the metrics values and associated metadata
type MetricFrame struct {
	Metrics   []Metric
	Timestamp float64
	Socket    string
	CPU       string
	Cgroup    string
	PID       string
	Cmd       string
}

// ProcessEvents is responsible for producing metrics from raw perf events
func ProcessEvents(perfEvents [][]byte, eventGroupDefinitions []GroupDefinition, metricDefinitions []MetricDefinition, processes []Process, previousTimestamp float64, metadata Metadata) (metricFrames []MetricFrame, timeStamp float64, err error) {
	var eventFrames []EventFrame
	if eventFrames, err = GetEventFrames(perfEvents, eventGroupDefinitions, flagScope, flagGranularity, metadata); err != nil { // arrange the events into groups
		err = fmt.Errorf("failed to put perf events into groups: %v", err)
		return
	}
	metricFrames = make([]MetricFrame, 0, len(eventFrames))
	for _, eventFrame := range eventFrames {
		timeStamp = eventFrame.Timestamp
		var metricFrame MetricFrame
		metricFrame.Metrics = make([]Metric, 0, len(metricDefinitions))
		metricFrame.Timestamp = eventFrame.Timestamp
		metricFrame.Socket = eventFrame.Socket
		metricFrame.CPU = eventFrame.CPU
		metricFrame.Cgroup = eventFrame.Cgroup
		var pidList []string
		var cmdList []string
		for _, process := range processes {
			pidList = append(pidList, process.pid)
			cmdList = append(cmdList, process.cmd)
		}
		metricFrame.PID = strings.Join(pidList, ",")
		metricFrame.Cmd = strings.Join(cmdList, ",")
		// produce metrics from event groups
		for _, metricDef := range metricDefinitions {
			metric := Metric{Name: metricDef.Name, Value: math.NaN()}
			var variables map[string]any
			if variables, err = getExpressionVariableValues(metricDef, eventFrame, previousTimestamp, metadata); err != nil {
				slog.Debug("failed to get expression variable values", slog.String("error", err.Error()))
				err = nil
			} else {
				var result any
				if result, err = evaluateExpression(metricDef, variables); err != nil {
					slog.Debug("failed to evaluate expression", slog.String("error", err.Error()))
					err = nil
				} else {
					metric.Value = result.(float64)
				}
			}
			metricFrame.Metrics = append(metricFrame.Metrics, metric)
			var prettyVars []string
			for variableName := range variables {
				prettyVars = append(prettyVars, fmt.Sprintf("%s=%f", variableName, variables[variableName]))
			}
			slog.Debug("processed metric", slog.String("name", metricDef.Name), slog.String("expression", metricDef.Expression), slog.String("vars", strings.Join(prettyVars, ", ")))
		}
		metricFrames = append(metricFrames, metricFrame)
	}
	return
}

// GetEvaluatorFunctions defines functions that can be called in metric expressions
func GetEvaluatorFunctions() (functions map[string]govaluate.ExpressionFunction) {
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

// lock to protect metric variable map that holds the event group where a variable value will be retrieved
var metricVariablesLock = sync.RWMutex{}

// for each variable in a metric, set the best group from which to get its value
func loadMetricBestGroups(metric MetricDefinition, frame EventFrame) (err error) {
	// one thread at a time through this function, since it updates the metric variables map and this only needs to be done one time
	metricVariablesLock.Lock()
	defer metricVariablesLock.Unlock()
	// only load event groups one time for each metric
	loadGroups := false
	for variableName := range metric.Variables {
		if metric.Variables[variableName] == -1 { // group not yet set
			loadGroups = true
			break
		}
		if metric.Variables[variableName] == -2 { // tried previously and failed, don't try again
			err = fmt.Errorf("metric variable group assignment previously failed, skipping: %s", variableName)
			return
		}
	}
	if !loadGroups {
		return // nothing to do, already loaded
	}
	allVariableNames := mapset.NewSetFromMapKeys(metric.Variables)
	remainingVariableNames := allVariableNames.Clone()
	for {
		if remainingVariableNames.Cardinality() == 0 { // found matches for all
			break
		}
		// find group with the greatest number of event names that match the remaining variable names
		bestGroupIdx := -1
		bestMatches := 0
		var matchedNames mapset.Set[string]
		for groupIdx, group := range frame.EventGroups {
			groupEventNames := mapset.NewSetFromMapKeys(group.EventValues)
			intersection := remainingVariableNames.Intersect(groupEventNames)
			// if an event value is NaN, remove it from the intersection map with hopes we'll find a better match
			for _, name := range intersection.ToSlice() {
				if math.IsNaN(group.EventValues[name]) {
					intersection.Remove(name)
				}
			}
			if intersection.Cardinality() > bestMatches {
				bestGroupIdx = groupIdx
				bestMatches = intersection.Cardinality()
				matchedNames = intersection.Clone()
				if bestMatches == remainingVariableNames.Cardinality() {
					break
				}
			}
		}
		if bestGroupIdx == -1 { // no matches
			for _, variableName := range remainingVariableNames.ToSlice() {
				metric.Variables[variableName] = -2 // we tried and failed
			}
			err = fmt.Errorf("metric variables (%s) not found for metric: %s", strings.Join(remainingVariableNames.ToSlice(), ", "), metric.Name)
			break
		}
		// for each of the matched names, set the value and the group from which to retrieve the value next time
		for _, name := range matchedNames.ToSlice() {
			metric.Variables[name] = bestGroupIdx
		}
		remainingVariableNames = remainingVariableNames.Difference(matchedNames)
	}
	return
}

// get the variable values that will be used to evaluate the metric's expression
func getExpressionVariableValues(metric MetricDefinition, frame EventFrame, previousTimestamp float64, metadata Metadata) (variables map[string]any, err error) {
	variables = make(map[string]any)
	if err = loadMetricBestGroups(metric, frame); err != nil {
		err = fmt.Errorf("at least one of the variables couldn't be assigned to a group: %v", err)
		return
	}
	// set the variable values to be used in the expression evaluation
	for variableName := range metric.Variables {
		if metric.Variables[variableName] == -2 {
			err = fmt.Errorf("variable value set to -2 (shouldn't happen): %s", variableName)
			return
		}
		// set the variable value to the event value divided by the perf collection time to normalize the value to 1 second
		if len(frame.EventGroups) <= metric.Variables[variableName] {
			err = fmt.Errorf("event groups have changed")
			return
		}
		variables[variableName] = frame.EventGroups[metric.Variables[variableName]].EventValues[variableName] / (frame.Timestamp - previousTimestamp)
		// adjust cstate_core/c6-residency value if hyperthreading is enabled
		// why here? so we don't have to change the perfmon metric formula
		if metadata.ThreadsPerCore > 1 && variableName == "cstate_core/c6-residency/" {
			variables[variableName] = variables[variableName].(float64) * float64(metadata.ThreadsPerCore)
		}
	}
	return
}

// function to call evaluator so that we can catch panics that come from the evaluator
func evaluateExpression(metric MetricDefinition, variables map[string]any) (result any, err error) {
	defer func() {
		if errx := recover(); errx != nil {
			err = errx.(error)
		}
	}()
	if result, err = metric.Evaluable.Evaluate(variables); err != nil {
		err = fmt.Errorf("%v : %s : %s", err, metric.Name, metric.Expression)
	}
	return
}

// write json formatted events to raw file
func writeEventsToFile(path string, events [][]byte) (err error) {
	rawFile, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // #nosec G304 G302
	if err != nil {
		slog.Error("failed to open raw file for writing", slog.String("error", err.Error()))
		return
	}
	defer rawFile.Close()
	for _, rawEvent := range events {
		rawEvent = append(rawEvent, []byte("\n")...)
		if _, err = rawFile.Write(rawEvent); err != nil {
			slog.Error("failed to write event to raw file", slog.String("error", err.Error()))
			return
		}
	}
	return
}
