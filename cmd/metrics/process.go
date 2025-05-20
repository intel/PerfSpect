package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Linux process information helper functions

import (
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"perfspect/internal/common"
	"perfspect/internal/script"
	"perfspect/internal/target"
)

type Process struct {
	pid  string
	ppid string
	comm string
	cmd  string
}

// pid,ppid,comm,cmd
var psRegex = `^\s*(\d+)\s+(\d+)\s+([\w\d\(\)\:\/_\-\:\.]+)\s+(.*)`

// GetProcesses - gets the list of processes associated with the given list of
// process IDs. An error occurs when a given PID is not found in the current
// set of running processes.
func GetProcesses(myTarget target.Target, pids []string) (processes []Process, err error) {
	for _, pid := range pids {
		if processExists(myTarget, pid) {
			var process Process
			if process, err = getProcess(myTarget, pid); err != nil {
				return
			}
			processes = append(processes, process)
		}
	}
	return
}

// GetCgroups - gets the list of full cgroup names associated with the given list of
// partial cgroup names. An error occurs when a given cgroup name is not found in the
// current set of process cgroups.
func GetCgroups(myTarget target.Target, cids []string, localTempDir string) (cgroups []string, err error) {
	for _, cid := range cids {
		var cgroup string
		if cgroup, err = getCgroup(myTarget, cid, localTempDir); err != nil {
			return
		}
		cgroups = append(cgroups, cgroup)
	}
	return
}

// GetHotProcesses - get maxProcesses processes with highest CPU utilization, matching
// filter if provided
func GetHotProcesses(myTarget target.Target, maxProcesses int, filter string) (processes []Process, err error) {
	if maxProcesses == 0 {
		err = fmt.Errorf("maxProcesses must be greater than 0")
		return
	}
	// run ps to get list of processes sorted by cpu utilization (descending)
	cmd := exec.Command("ps", "-a", "-x", "-h", "-o", "pid,ppid,comm,cmd", "--sort=-%cpu")
	stdout, stderr, exitcode, err := myTarget.RunCommand(cmd, 0, true)
	if err != nil {
		err = fmt.Errorf("failed to get hot processes: %s, %d, %v", stderr, exitcode, err)
		return
	}
	psOutput := stdout
	var reFilter *regexp.Regexp
	if filter != "" {
		if reFilter, err = regexp.Compile(filter); err != nil {
			return
		}
	}
	reProcess := regexp.MustCompile(psRegex)
	for line := range strings.SplitSeq(psOutput, "\n") {
		if line == "" {
			continue
		}
		match := reProcess.FindStringSubmatch(line)
		if match == nil {
			slog.Warn("Unrecognized ps output format", slog.String("line", line))
			continue
		}
		pid := match[1]
		ppid := match[2]
		comm := match[3]
		cmd := match[4]
		// skip processes that match the name of this program
		if strings.Contains(cmd, filepath.Base(common.AppName)) {
			slog.Debug("Skipping self", slog.String("PID", pid))
			continue
		}
		// skip processes that match the 'ps' command we ran above
		if strings.Contains(cmd, "ps -a -x -h -o pid,ppid,comm,cmd --sort=-%cpu") {
			slog.Debug("Skipping ps command", slog.String("PID", pid))
			continue
		}
		// if a filter was provided, skip processes that don't match
		if reFilter != nil && !reFilter.MatchString(cmd) {
			slog.Debug("Skipping process that doesn't match filter", slog.String("PID", pid), slog.String("Command", cmd))
			continue
		}
		processes = append(processes, Process{pid: pid, ppid: ppid, comm: comm, cmd: cmd})
		if len(processes) == maxProcesses {
			break
		}
	}
	var pids []string
	for _, process := range processes {
		pids = append(pids, process.pid)
	}
	slog.Debug("Hot PIDs", slog.String("PIDs", strings.Join(pids, ", ")))
	return
}

// GetHotCgroups - get maxCgroups cgroup names whose associated processes have the
// highest CPU utilization, matching filter if provided
func GetHotCgroups(myTarget target.Target, maxCgroups int, filter string, localTempDir string) (cgroups []string, err error) {
	hotCgroupsScript := script.ScriptDefinition{
		Name: "hot_cgroups",
		ScriptTemplate: fmt.Sprintf(`
# Directory to search for cgroups
search_dir="/sys/fs/cgroup"

# Find matching cgroups
matching_cgroups=$(find "$search_dir" -type d \( -name "docker*scope" -o -name "containerd*scope" \))

# Filter matching cgroups based on regex if provided
regex=%s
if [ -n "$regex" ]; then
    matching_cgroups=$(echo "$matching_cgroups" | grep -E "$regex")
fi

# Get CPU usage for each matching cgroup
declare -A cgroup_cpu_usage
for cgroup in $matching_cgroups; do
    if [ -f "$cgroup/cpu.stat" ]; then
        cpu_usage=$(grep 'usage_usec' "$cgroup/cpu.stat" | awk '{print $2}')
        if [ -n "$cpu_usage" ]; then
            cgroup_path=${cgroup#"$search_dir"}
            cgroup_cpu_usage["$cgroup_path"]=$cpu_usage
        fi
    fi
done

# Sort cgroups by CPU usage and get the top N
for cgroup in "${!cgroup_cpu_usage[@]}"; do
    echo "${cgroup_cpu_usage[$cgroup]} $cgroup"
done | sort -nr | head -n %d
`, filter, maxCgroups),
		Superuser: true,
	}
	output, err := script.RunScript(myTarget, hotCgroupsScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to get hot cgroups: %v", err)
		return
	}
	lines := strings.SplitSeq(output.Stdout, "\n")
	for line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		cgroups = append(cgroups, fields[1])
	}
	slog.Debug("Hot CIDs", slog.String("CIDs", strings.Join(cgroups, ", ")))
	return
}

func processExists(myTarget target.Target, pid string) (exists bool) {
	cmd := exec.Command("ps", "-p", pid)
	_, _, _, err := myTarget.RunCommand(cmd, 0, true)
	if err != nil {
		exists = false
		return
	}
	exists = true
	return
}

func getProcess(myTarget target.Target, pid string) (process Process, err error) {
	cmd := exec.Command("ps", "-q", pid, "h", "-o", "pid,ppid,comm,cmd", "ww")
	stdout, stderr, exitcode, err := myTarget.RunCommand(cmd, 0, true)
	if err != nil {
		err = fmt.Errorf("failed to get process: %s, %d, %v", stderr, exitcode, err)
		return
	}
	psOutput := stdout
	reProcess := regexp.MustCompile(psRegex)
	match := reProcess.FindStringSubmatch(psOutput)
	if match == nil {
		err = fmt.Errorf("Process not found, PID: %s, ps output: %s", pid, psOutput)
		return
	}
	process = Process{pid: match[1], ppid: match[2], comm: match[3], cmd: match[4]}
	return
}

func getCgroup(myTarget target.Target, cid string, localTempDir string) (cGroupName string, err error) {
	cgroupScript := script.ScriptDefinition{
		Name: "cgroup",
		ScriptTemplate: fmt.Sprintf(`
# Directory to search for cgroups
search_dir="/sys/fs/cgroup"

# Find the full cgroup path that matches the partial container ID
full_path=$(find "$search_dir" -type d | grep %s)

cgroup_path=${full_path#"$search_dir"}
echo $cgroup_path
`, cid),
		Superuser: true,
	}
	output, err := script.RunScript(myTarget, cgroupScript, localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to get cgroup: %v", err)
		return
	}
	cGroupName = strings.TrimSpace(output.Stdout)
	return
}
