package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Linux perf event/group multiplexing interval helper functions

import (
	"fmt"
	"strconv"
	"strings"

	"perfspect/internal/script"
	"perfspect/internal/target"
)

// GetMuxIntervals - get a map of sysfs device file names to current mux value for the associated device
func GetMuxIntervals(myTarget target.Target, localTempDir string) (intervals map[string]int, err error) {
	bash := "for file in $(find /sys/devices -type f -name perf_event_mux_interval_ms); do echo $file $(cat $file); done"
	scriptOutput, err := script.RunScript(myTarget, script.ScriptDefinition{Name: "get mux intervals", ScriptTemplate: bash, Superuser: false}, localTempDir)
	if err != nil {
		return
	}
	intervals = make(map[string]int)
	for line := range strings.SplitSeq(scriptOutput.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			if interval, err := strconv.Atoi(fields[1]); err == nil {
				intervals[fields[0]] = interval
			}
		}
	}
	return
}

// SetMuxIntervals - write the given intervals (values in ms) to the given sysfs device file names (key)
func SetMuxIntervals(myTarget target.Target, intervals map[string]int, localTempDir string) (err error) {
	var bash string
	for device := range intervals {
		bash += fmt.Sprintf("echo %d > %s; ", intervals[device], device)
	}
	scriptOutput, err := script.RunScript(myTarget, script.ScriptDefinition{Name: "set mux intervals", ScriptTemplate: bash, Superuser: true}, localTempDir) // nosemgrep
	if err != nil {
		err = fmt.Errorf("failed to set mux interval on device: %s, %d, %v", scriptOutput.Stderr, scriptOutput.Exitcode, err)
		return
	}
	return
}

// SetAllMuxIntervals - writes the given interval (ms) to all perf mux sysfs device files
func SetAllMuxIntervals(myTarget target.Target, interval int, localTempDir string) (err error) {
	bash := fmt.Sprintf("for file in $(find /sys/devices -type f -name perf_event_mux_interval_ms); do echo %d > $file; done", interval)
	_, err = script.RunScript(myTarget, script.ScriptDefinition{Name: "set all mux intervals", ScriptTemplate: bash, Superuser: true}, localTempDir)
	if err != nil {
		return
	}
	return
}
