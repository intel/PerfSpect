// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"regexp"
	"slices"
	"strings"
)

// parseContainerCgroup extracts the host cgroup path (without leading slash) from a
// /proc/<pid>/cgroup file, supporting both cgroup v2 ("0::/...") and cgroup v1
// (preferring the "perf_event" controller, then "cpu"/"cpuacct"). Nested sub-cgroups
// after the container ID segment (such as "/init.scope") are trimmed so perf monitors
// the whole container.
func parseContainerCgroup(procCgroupContent string, containerID string) string {
	var v2Path string
	var perfEventPath string
	var cpuPath string
	var fallbackPath string

	for line := range strings.SplitSeq(procCgroupContent, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		hierarchyID := parts[0]
		controllers := parts[1]
		cgPath := parts[2]
		if cgPath == "" || cgPath == "/" {
			continue
		}
		norm := normalizeCgroupPath(cgPath, containerID)
		if norm == "" {
			continue
		}
		if hierarchyID == "0" && controllers == "" {
			v2Path = norm
			continue
		}
		for ctrl := range strings.SplitSeq(controllers, ",") {
			switch ctrl {
			case "perf_event":
				if perfEventPath == "" {
					perfEventPath = norm
				}
			case "cpu", "cpuacct":
				if cpuPath == "" {
					cpuPath = norm
				}
			}
		}
		if fallbackPath == "" {
			fallbackPath = norm
		}
	}

	if perfEventPath != "" {
		return perfEventPath
	}
	candidates := []string{v2Path, cpuPath, fallbackPath}
	if containerID != "" {
		for _, cand := range candidates {
			if cand != "" && cgroupPathContainsContainerID(cand, containerID) {
				return cand
			}
		}
	}
	for _, cand := range candidates {
		if cand != "" {
			return cand
		}
	}
	return ""
}

func segmentMatchesContainerID(seg string, containerID string) bool {
	if containerID == "" {
		return false
	}
	if len(containerID) >= 12 {
		return strings.Contains(seg, containerID)
	}
	return seg == containerID ||
		seg == containerID+".scope" ||
		strings.HasSuffix(seg, "-"+containerID+".scope") ||
		strings.HasSuffix(seg, ":"+containerID)
}

func cgroupPathContainsContainerID(cgroupPath string, containerID string) bool {
	if cgroupPath == "" || containerID == "" {
		return false
	}
	for seg := range strings.SplitSeq(cgroupPath, "/") {
		if segmentMatchesContainerID(seg, containerID) {
			return true
		}
	}
	return false
}

func normalizeCgroupPath(rawPath string, containerID string) string {
	if strings.ContainsAny(rawPath, "\r\n") {
		return ""
	}
	p := strings.TrimSpace(rawPath)
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return ""
	}
	parts := strings.Split(p, "/")
	if slices.Contains(parts, "..") {
		return ""
	}
	if containerID != "" {
		for i, seg := range parts {
			if segmentMatchesContainerID(seg, containerID) {
				return strings.Join(parts[:i+1], "/")
			}
		}
	}
	if len(parts) > 1 && parts[len(parts)-1] == "init.scope" {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, "/")
}

var nonPerfV1ControllerPrefixes = []string{
	"pids/",
	"memory/",
	"blkio/",
	"devices/",
	"freezer/",
	"net_cls/",
	"net_prio/",
	"net_cls,net_prio/",
	"net_prio,net_cls/",
	"hugetlb/",
	"rdma/",
	"cpuset/",
	"misc/",
	"systemd/",
}

var cpuV1ControllerPrefixes = []string{
	"cpu,cpuacct/",
	"cpuacct,cpu/",
	"cpuacct/",
	"cpu/",
}

func selectCgroupFromFindOutput(findOut string, containerID string) string {
	var perfV1Best string
	var v2Best string
	var cpuV1Best string

	for line := range strings.SplitSeq(findOut, "\n") {
		d := strings.TrimSpace(line)
		if d == "" {
			continue
		}
		if after, ok := strings.CutPrefix(d, "/sys/fs/cgroup/perf_event/"); ok {
			norm := normalizeCgroupPath(after, containerID)
			if norm != "" && (perfV1Best == "" || len(norm) < len(perfV1Best)) {
				perfV1Best = norm
			}
			continue
		}
		rel := strings.TrimPrefix(d, "/sys/fs/cgroup/")
		var matchedCPU bool
		for _, prefix := range cpuV1ControllerPrefixes {
			if after, ok := strings.CutPrefix(rel, prefix); ok {
				norm := normalizeCgroupPath(after, containerID)
				if norm != "" && (cpuV1Best == "" || len(norm) < len(cpuV1Best)) {
					cpuV1Best = norm
				}
				matchedCPU = true
				break
			}
		}
		if matchedCPU {
			continue
		}
		skip := false
		for _, prefix := range nonPerfV1ControllerPrefixes {
			if strings.HasPrefix(rel, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		norm := normalizeCgroupPath(rel, containerID)
		if norm != "" && (v2Best == "" || len(norm) < len(v2Best)) {
			v2Best = norm
		}
	}
	if perfV1Best != "" {
		return perfV1Best
	}
	if v2Best != "" {
		return v2Best
	}
	return cpuV1Best
}

func deduplicateContainers(containers []Container) []Container {
	merged := make([]Container, 0, len(containers))
	byID := make(map[string]int)
	byCgroup := make(map[string]int)

	mergeInto := func(dst *Container, src Container) {
		if (dst.Name == "" || dst.Name == dst.ID || dst.Name == shortContainerID(dst.ID)) && src.Name != "" && src.Name != src.ID {
			dst.Name = src.Name
		}
		if dst.PodName == "" && src.PodName != "" {
			dst.PodName = src.PodName
		}
		if dst.Namespace == "" && src.Namespace != "" {
			dst.Namespace = src.Namespace
		}
		if dst.PID == 0 && src.PID > 0 {
			dst.PID = src.PID
		}
		if dst.CgroupPath == "" && src.CgroupPath != "" {
			dst.CgroupPath = src.CgroupPath
		}
		if src.CPUUsage > dst.CPUUsage {
			dst.CPUUsage = src.CPUUsage
		}
		if dst.ID == "" && src.ID != "" {
			dst.ID = src.ID
		}
	}

	for _, c := range containers {
		idx := -1
		if c.ID != "" {
			if existingIdx, ok := byID[c.ID]; ok {
				idx = existingIdx
			}
		}
		if idx == -1 && c.CgroupPath != "" {
			if existingIdx, ok := byCgroup[c.CgroupPath]; ok {
				idx = existingIdx
			}
		}
		if idx != -1 {
			mergeInto(&merged[idx], c)
			if merged[idx].ID != "" {
				byID[merged[idx].ID] = idx
			}
			if merged[idx].CgroupPath != "" {
				byCgroup[merged[idx].CgroupPath] = idx
			}
			continue
		}
		idx = len(merged)
		merged = append(merged, c)
		if c.ID != "" {
			byID[c.ID] = idx
		}
		if c.CgroupPath != "" {
			byCgroup[c.CgroupPath] = idx
		}
	}
	return merged
}

func filterContainersByRegex(containers []Container, re *regexp.Regexp) []Container {
	if re == nil {
		return containers
	}
	filtered := make([]Container, 0, len(containers))
	for _, c := range containers {
		if re.MatchString(c.Name) {
			filtered = append(filtered, c)
			continue
		}
		if c.PodName != "" {
			if re.MatchString(c.PodName + "/" + c.Name) {
				filtered = append(filtered, c)
				continue
			}
			if c.Namespace != "" && re.MatchString(c.Namespace+"/"+c.PodName+"/"+c.Name) {
				filtered = append(filtered, c)
				continue
			}
		}
	}
	return filtered
}

func selectTopContainers(containers []Container, count int) []Container {
	if len(containers) == 0 || count <= 0 {
		return nil
	}
	sorted := slices.Clone(containers)
	slices.SortStableFunc(sorted, func(a, b Container) int {
		if a.CPUUsage != b.CPUUsage {
			if a.CPUUsage > b.CPUUsage {
				return -1
			}
			return 1
		}
		if a.Name != b.Name {
			return strings.Compare(a.Name, b.Name)
		}
		return strings.Compare(a.ID, b.ID)
	})
	if len(sorted) > count {
		sorted = sorted[:count]
	}
	return sorted
}
