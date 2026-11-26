// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"fmt"
	"slices"
	"strings"
)

var displayNameSanitizer = strings.NewReplacer(",", "_", `"`, "_", "|", "_", "\r", "_", "\n", "_", "\t", "_")

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
