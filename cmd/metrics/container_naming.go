// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var (
	displayNameSanitizer = strings.NewReplacer(",", "_", `"`, "_", "|", "_", "\r", "_", "\n", "_", "\t", "_")

	k8sDeploymentPodRe      = regexp.MustCompile(`^(.+)-[a-z0-9]{5,10}-[bcdfghjklmnpqrstvwxz2456789]{5}$`)
	k8sStatefulSetPodRe     = regexp.MustCompile(`^(.+)-[0-9]+$`)
	k8sHashPodRe            = regexp.MustCompile(`^(.+)-([bcdfghjklmnpqrstvwxz2456789]{5})$`)
	cgroupLeafContainerIDRe = regexp.MustCompile(`^(?:.*[-:])?([0-9a-f]{64})(?:\.scope)?$`)
)

func inferKubernetesWorkload(podName string) string {
	if podName == "" {
		return ""
	}
	if m := k8sDeploymentPodRe.FindStringSubmatch(podName); m != nil {
		return m[1]
	}
	if m := k8sStatefulSetPodRe.FindStringSubmatch(podName); m != nil {
		return m[1]
	}
	if m := k8sHashPodRe.FindStringSubmatch(podName); m != nil {
		return m[1]
	}
	return podName
}

func shortenKubernetesPodName(podName string, workload string) string {
	if podName == "" {
		return ""
	}
	if workload == "" {
		workload = inferKubernetesWorkload(podName)
	}
	if m := k8sDeploymentPodRe.FindStringSubmatch(podName); m != nil && m[1] == workload {
		if idx := strings.LastIndex(podName, "-"); idx != -1 {
			return "…" + podName[idx:]
		}
	}
	if !k8sStatefulSetPodRe.MatchString(podName) {
		if m := k8sHashPodRe.FindStringSubmatch(podName); m != nil && m[1] == workload {
			return "…-" + m[2]
		}
	}
	runes := []rune(podName)
	if len(runes) > 22 {
		return string(runes[:10]) + "…" + string(runes[len(runes)-8:])
	}
	return podName
}

func inferContainerIDFromCgroupPath(cgroupPath string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(cgroupPath), "/")
	if trimmed == "" {
		return ""
	}
	leaf := trimmed
	if idx := strings.LastIndex(trimmed, "/"); idx != -1 {
		leaf = trimmed[idx+1:]
	}
	if m := cgroupLeafContainerIDRe.FindStringSubmatch(leaf); m != nil {
		return m[1]
	}
	return ""
}

func inferContainerMetadataFromDisplayName(displayName string, cgroupPath string) ContainerMetadata {
	parts := strings.Split(displayName, "/")
	meta := ContainerMetadata{
		ID:          inferContainerIDFromCgroupPath(cgroupPath),
		DisplayName: displayName,
		CgroupPath:  cgroupPath,
	}
	switch len(parts) {
	case 3:
		meta.Namespace = parts[0]
		meta.PodName = parts[1]
		meta.Name = parts[2]
	case 2:
		meta.PodName = parts[0]
		meta.Name = parts[1]
		if strings.Contains(cgroupPath, "kubepods") {
			meta.Namespace = "default"
		}
	default:
		meta.Name = displayName
	}
	if meta.PodName != "" {
		meta.Workload = inferKubernetesWorkload(meta.PodName)
		meta.ShortPod = shortenKubernetesPodName(meta.PodName, meta.Workload)
	}
	return meta
}

func assignContainerDisplayNames(containers []Container, existingByCgroup map[string]string) []Container {
	result := slices.Clone(containers)

	// Seed usedLabels with all previously claimed display names so new containers
	// on refresh never steal a label that already belongs to another cgroup.
	usedLabels := make(map[string]bool, len(existingByCgroup)+len(result))
	for _, label := range existingByCgroup {
		if label != "" {
			usedLabels[label] = true
		}
	}

	candidates := make([]string, len(result))
	candidateCounts := make(map[string]int, len(result))
	isExisting := make([]bool, len(result))
	for i, c := range result {
		if c.CgroupPath != "" {
			if existingLabel := existingByCgroup[c.CgroupPath]; existingLabel != "" {
				result[i].DisplayName = existingLabel
				isExisting[i] = true
				usedLabels[existingLabel] = true
				continue
			}
		}
		base := c.Name
		if base == "" {
			base = shortContainerID(c.ID)
		}
		cand := base
		if c.PodName != "" {
			if c.Namespace != "" {
				cand = c.Namespace + "/" + c.PodName + "/" + base
			} else {
				cand = c.PodName + "/" + base
			}
		}
		candidates[i] = sanitizeContainerDisplayName(cand)
		candidateCounts[candidates[i]]++
	}

	// Disambiguate collisions (among newly discovered containers or against previously
	// claimed labels) using short ID and a deterministic sequence suffix if needed.
	for i, c := range result {
		if isExisting[i] {
			continue
		}
		label := candidates[i]
		if candidateCounts[label] > 1 || usedLabels[label] {
			if idSuffix := shortContainerID(c.ID); idSuffix != "" && !strings.Contains(label, idSuffix) {
				label = sanitizeContainerDisplayName(fmt.Sprintf("%s (%s)", label, idSuffix))
			}
		}
		if usedLabels[label] {
			baseLabel := label
			for n := 2; ; n++ {
				candidate := fmt.Sprintf("%s #%d", baseLabel, n)
				if !usedLabels[candidate] {
					label = candidate
					break
				}
			}
		}
		usedLabels[label] = true
		result[i].DisplayName = label
	}
	return result
}

func sanitizeContainerDisplayName(name string) string {
	return strings.TrimSpace(displayNameSanitizer.Replace(name))
}

func shortContainerID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}
