// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package report

import (
	"fmt"
	"sort"
	"strings"

	"perfspect/internal/common"
	"perfspect/internal/script"
)

func cveInfoFromOutput(outputs map[string]script.ScriptOutput) [][]string {
	vulns := make(map[string]string)
	// from spectre-meltdown-checker
	for _, pair := range common.ValsArrayFromRegexSubmatch(outputs[script.CveScriptName].Stdout, `(CVE-\d+-\d+): (.+)`) {
		vulns[pair[0]] = pair[1]
	}
	// sort the vulnerabilities by CVE ID
	var ids []string
	for id := range vulns {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	cves := make([][]string, 0)
	for _, id := range ids {
		cves = append(cves, []string{id, vulns[id]})
	}
	return cves
}

func cveSummaryFromOutput(outputs map[string]script.ScriptOutput) string {
	cves := cveInfoFromOutput(outputs)
	if len(cves) == 0 {
		return ""
	}
	var numOK int
	var numVuln int
	for _, cve := range cves {
		if strings.HasPrefix(cve[1], "OK") {
			numOK++
		} else {
			numVuln++
		}
	}
	return fmt.Sprintf("%d OK, %d Vulnerable", numOK, numVuln)
}
