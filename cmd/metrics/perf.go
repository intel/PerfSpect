package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"os/exec"
	"path"
	"perfspect/internal/script"
	"perfspect/internal/target"
	"perfspect/internal/util"
	"strconv"
	"strings"
)

// extractPerf extracts the perf binary from the resources to the local temporary directory.
func extractPerf(myTarget target.Target, localTempDir string) (string, error) {
	// get the target architecture
	arch, err := myTarget.GetArchitecture()
	if err != nil {
		return "", fmt.Errorf("failed to get target architecture: %w", err)
	}
	// extract the perf binary
	return util.ExtractResource(script.Resources, path.Join("resources", arch, "perf"), localTempDir)
}

// getPerfPath determines the path to the `perf` binary for the given target.
// If the target is a local target, it uses the provided localPerfPath.
// If the target is remote, it checks if `perf` version 6.1 or later is available on the target.
// If available, it uses the `perf` binary on the target.
// If not available, it pushes the local `perf` binary to the target's temporary directory and uses that.
//
// Parameters:
// - myTarget: The target system where the `perf` binary is needed.
// - localPerfPath: The local path to the `perf` binary.
//
// Returns:
// - perfPath: The path to the `perf` binary on the target.
// - err: An error if any occurred during the process.
func getPerfPath(myTarget target.Target, localPerfPath string) (string, error) {
	var perfPath string
	if _, ok := myTarget.(*target.LocalTarget); ok {
		perfPath = localPerfPath
	} else {
		hasPerf := false
		cmd := exec.Command("perf", "--version")
		output, _, _, err := myTarget.RunCommand(cmd, 0, true)
		if err == nil && strings.Contains(output, "perf version") {
			// get the version number
			version := strings.Split(strings.TrimSpace(output), " ")[2]
			// split version into major and minor and build numbers
			versionParts := strings.Split(version, ".")
			if len(versionParts) >= 2 {
				major, _ := strconv.Atoi(versionParts[0])
				minor, _ := strconv.Atoi(versionParts[1])
				if major > 6 || (major == 6 && minor >= 1) {
					hasPerf = true
				}
			}
		}
		if hasPerf {
			perfPath = "perf"
		} else {
			targetTempDir := myTarget.GetTempDirectory()
			if targetTempDir == "" {
				panic("targetTempDir is empty")
			}
			if err = myTarget.PushFile(localPerfPath, targetTempDir); err != nil {
				slog.Error("failed to push perf binary to remote directory", slog.String("error", err.Error()))
				return "", err
			}
			perfPath = path.Join(targetTempDir, "perf")
		}
	}
	return perfPath, nil
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
func getPerfCommandArgs(pids string, cgroups []string, timeout uint, eventGroups []GroupDefinition) (args []string, err error) {
	// -I: print interval in ms
	// -j: json formatted event output
	args = append(args, "stat", "-I", fmt.Sprintf("%d", flagPerfPrintInterval*1000), "-j")
	if flagScope == scopeSystem {
		args = append(args, "-a") // system-wide collection
		if flagGranularity == granularityCPU || flagGranularity == granularitySocket {
			args = append(args, "-A") // no aggregation
		}
	} else if flagScope == scopeProcess {
		args = append(args, "-p", pids) // collect only for these processes
	} else if flagScope == scopeCgroup {
		args = append(args, "--for-each-cgroup", strings.Join(cgroups, ",")) // collect only for these cgroups
	}
	// -e: event groups to collect
	args = append(args, "-e")
	var groups []string
	for _, group := range eventGroups {
		var events []string
		for _, event := range group {
			events = append(events, event.Raw)
		}
		groups = append(groups, fmt.Sprintf("{%s}", strings.Join(events, ",")))
	}
	args = append(args, fmt.Sprintf("'%s'", strings.Join(groups, ",")))
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

// getPerfCommand is responsible for assembling the command that will be
// executed to collect event data
func getPerfCommand(myTarget target.Target, perfPath string, eventGroups []GroupDefinition, localTempDir string) (processes []Process, perfCommand *exec.Cmd, err error) {
	if flagScope == scopeSystem {
		var args []string
		if args, err = getPerfCommandArgs("", []string{}, flagDuration, eventGroups); err != nil {
			err = fmt.Errorf("failed to assemble perf args: %v", err)
			return
		}
		perfCommand = exec.Command(perfPath, args...) // #nosec G204 // nosemgrep
	} else if flagScope == scopeProcess {
		if len(flagPidList) > 0 {
			if processes, err = GetProcesses(myTarget, flagPidList); err != nil {
				return
			}
			if len(processes) == 0 {
				err = fmt.Errorf("failed to find processes associated with designated PIDs: %v", flagPidList)
				return
			}
		} else {
			if processes, err = GetHotProcesses(myTarget, flagCount, flagFilter); err != nil {
				return
			}
			if len(processes) == 0 {
				if flagFilter == "" {
					err = fmt.Errorf("failed to find \"hot\" processes")
					return
				} else {
					err = fmt.Errorf("failed to find processes matching filter: %s", flagFilter)
					return
				}
			}
		}
		var timeout uint
		if flagDuration > 0 {
			timeout = flagDuration
		} else if len(flagPidList) == 0 { // don't refresh if PIDs are specified
			timeout = flagRefresh // refresh hot processes every flagRefresh seconds
		}
		pidList := make([]string, 0, len(processes))
		for _, process := range processes {
			pidList = append(pidList, process.pid)
		}
		var args []string
		if args, err = getPerfCommandArgs(strings.Join(pidList, ","), []string{}, timeout, eventGroups); err != nil {
			err = fmt.Errorf("failed to assemble perf args: %v", err)
			return
		}
		perfCommand = exec.Command(perfPath, args...) // #nosec G204 // nosemgrep
	} else if flagScope == scopeCgroup {
		var cgroups []string
		if len(flagCidList) > 0 {
			if cgroups, err = GetCgroups(myTarget, flagCidList, localTempDir); err != nil {
				return
			}
		} else {
			if cgroups, err = GetHotCgroups(myTarget, flagCount, flagFilter, localTempDir); err != nil {
				return
			}
		}
		if len(cgroups) == 0 {
			err = fmt.Errorf("no CIDs selected")
			return
		}
		var args []string
		if args, err = getPerfCommandArgs("", cgroups, 0, eventGroups); err != nil {
			err = fmt.Errorf("failed to assemble perf args: %v", err)
			return
		}
		perfCommand = exec.Command(perfPath, args...) // #nosec G204 // nosemgrep
	}
	return
}
