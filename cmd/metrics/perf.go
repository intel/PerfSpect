package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"strings"
)

// getPerfCommand is responsible for assembling the command that will be
// executed to collect event data
func getPerfCommand(eventGroups []GroupDefinition, pids []string, cids []string, cpuRange string) (string, error) {
	var duration int
	switch flagScope {
	case scopeSystem:
		duration = flagDuration
	case scopeProcess:
		if flagDuration > 0 {
			duration = flagDuration
		} else if len(flagPidList) == 0 { // don't refresh if PIDs are specified
			duration = flagRefresh // refresh hot processes every flagRefresh seconds
		}
	case scopeCgroup:
		duration = 0
	}

	args, err := getPerfCommandArgs(pids, cids, duration, eventGroups, cpuRange)
	if err != nil {
		err = fmt.Errorf("failed to assemble perf args: %v", err)
		return "", err
	}
	return strings.Join(append([]string{"perf"}, args...), " "), nil
}

// getPerfCommandArgs returns the command arguments for the 'perf stat' command
// based on the provided parameters.
//
// Parameters:
//   - pids: The process IDs for which to collect performance data. If flagScope is
//     set to "process", the data will be collected only for these processes.
//   - cgroups: The list of cgroups for which to collect performance data. If
//     flagScope is set to "cgroup", the data will be collected only for these cgroups.
//   - timeout: The timeout value in seconds. If flagScope is not set to "cgroup"
//     and timeout is not 0, the 'sleep' command will be added to the arguments
//     with the specified timeout value.
//   - eventGroups: The list of event groups to collect. Each event group is a
//     collection of events to be monitored.
//
// Returns:
// - args: The command arguments for the 'perf stat' command.
// - err: An error, if any.
func getPerfCommandArgs(pids []string, cgroups []string, timeout int, eventGroups []GroupDefinition, cpuRange string) (args []string, err error) {
	// -I: print interval in ms
	// -j: json formatted event output
	args = append(args, "stat", "-I", fmt.Sprintf("%d", flagPerfPrintInterval*1000), "-j")

	// -C: collect only for these cpus
	if cpuRange != "" {
		args = append(args, "-C", cpuRange) // collect only for these cpus
	}
	switch flagScope {
	case scopeSystem:
		args = append(args, "-a") // system-wide collection
		if flagGranularity == granularityCPU || flagGranularity == granularitySocket {
			args = append(args, "-A") // no aggregation
		}
	case scopeProcess:
		args = append(args, "-p", strings.Join(pids, ",")) // collect only for these processes
	case scopeCgroup:
		args = append(args, "--for-each-cgroup", strings.Join(cgroups, ",")) // collect only for these cgroups
	}
	// -e: event groups to collect
	//args = append(args, "-e")
	//var groups []string
	for _, group := range eventGroups {
		var events []string
		for _, event := range group {
			events = append(events, event.Raw)
		}
		formattedGroup := fmt.Sprintf("'{%s}'", strings.Join(events, ","))
		args = append(args, "-e", formattedGroup)
		//groups = append(groups, fmt.Sprintf("{%s}", strings.Join(events, ",")))
	}
	//args = append(args, fmt.Sprintf("'%s'", strings.Join(groups, ",")))
	if len(argsApplication) > 0 {
		// add application args
		args = append(args, "--")
		args = append(args, argsApplication...)
	} else if flagScope != scopeCgroup && timeout != 0 {
		// add timeout
		args = append(args, "sleep", fmt.Sprintf("%d", timeout))
	}
	return
}
