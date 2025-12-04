// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// table_helpers.go contains base helper functions that are used to extract values from the output of the scripts.

package table

import (
	"regexp"
	"strings"
)

// valFromRegexSubmatch searches for a regex pattern in the given output string and returns the first captured group.
// If no match is found, an empty string is returned.
func valFromRegexSubmatch(output string, regex string) string {
	re := regexp.MustCompile(regex)
	for line := range strings.SplitSeq(output, "\n") {
		match := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

// valsFromRegexSubmatch extracts the captured groups from each line in the output
// that matches the given regular expression.
// It returns a slice of strings containing the captured values.
func valsFromRegexSubmatch(output string, regex string) []string {
	var vals []string
	re := regexp.MustCompile(regex)
	for line := range strings.SplitSeq(output, "\n") {
		match := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) > 1 {
			vals = append(vals, match[1])
		}
	}
	return vals
}

// return all matches for all capture groups in regex
func valsArrayFromRegexSubmatch(output string, regex string) (vals [][]string) {
	re := regexp.MustCompile(regex)
	for line := range strings.SplitSeq(output, "\n") {
		match := re.FindStringSubmatch(line)
		if len(match) > 1 {
			vals = append(vals, match[1:])
		}
	}
	return
}

// valFromDmiDecodeRegexSubmatch extracts a value from the DMI decode output using a regular expression.
// It takes the DMI decode output, the DMI type, and the regular expression as input parameters.
// It returns the extracted value as a string.
func valFromDmiDecodeRegexSubmatch(dmiDecodeOutput string, dmiType string, regex string) string {
	return valFromRegexSubmatch(getDmiDecodeType(dmiDecodeOutput, dmiType), regex)
}

func valsArrayFromDmiDecodeRegexSubmatch(dmiDecodeOutput string, dmiType string, regexes ...string) (vals [][]string) {
	var res []*regexp.Regexp
	for _, r := range regexes {
		re := regexp.MustCompile(r)
		res = append(res, re)
	}
	for _, entry := range getDmiDecodeEntries(dmiDecodeOutput, dmiType) {
		row := make([]string, len(res))
		for _, line := range entry {
			for i, re := range res {
				match := re.FindStringSubmatch(strings.TrimSpace(line))
				if len(match) > 1 {
					row[i] = match[1]
				}
			}
		}
		vals = append(vals, row)
	}
	return
}

// getDmiDecodeType extracts the lines from the given `dmiDecodeOutput` that belong to the specified `dmiType`.
func getDmiDecodeType(dmiDecodeOutput string, dmiType string) string {
	var lines []string
	start := false
	for line := range strings.SplitSeq(dmiDecodeOutput, "\n") {
		if start && strings.HasPrefix(line, "Handle ") {
			start = false
		}
		if strings.Contains(line, "DMI type "+dmiType+",") {
			start = true
		}
		if start {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// getDmiDecodeEntries extracts the entries from the given `dmiDecodeOutput` that belong to the specified `dmiType`.
func getDmiDecodeEntries(dmiDecodeOutput string, dmiType string) (entries [][]string) {
	lines := strings.Split(dmiDecodeOutput, "\n")
	var entry []string
	typeMatch := false
	for _, line := range lines {
		if strings.HasPrefix(line, "Handle ") {
			if strings.Contains(line, "DMI type "+dmiType+",") {
				// type match
				typeMatch = true
				entry = []string{}
			} else {
				// not a type match
				typeMatch = false
			}
		}
		if !typeMatch {
			continue
		}
		if line == "" {
			// end of type match entry
			entries = append(entries, entry)
		} else {
			// a line in the entry
			entry = append(entry, line)
		}
	}
	return
}
