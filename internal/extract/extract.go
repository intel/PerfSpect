// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Package extract provides helper functions for extracting values from script outputs
// to populate table fields for reports.
package extract

import (
	"log/slog"
	"regexp"
	"strings"
)

// ValFromRegexSubmatch searches for a regex pattern in the given output string and returns the first captured group.
// If no match is found, an empty string is returned.
func ValFromRegexSubmatch(output string, regex string) string {
	re := regexp.MustCompile(regex)
	for line := range strings.SplitSeq(output, "\n") {
		match := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

// ValsFromRegexSubmatch extracts the captured groups from each line in the output
// that matches the given regular expression.
// It returns a slice of strings containing the captured values.
func ValsFromRegexSubmatch(output string, regex string) []string {
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

// ValsArrayFromRegexSubmatch returns all matches for all capture groups in regex
func ValsArrayFromRegexSubmatch(output string, regex string) (vals [][]string) {
	re := regexp.MustCompile(regex)
	for line := range strings.SplitSeq(output, "\n") {
		match := re.FindStringSubmatch(line)
		if len(match) > 1 {
			vals = append(vals, match[1:])
		}
	}
	return
}

// ValFromDmiDecodeRegexSubmatch extracts a value from the DMI decode output using a regular expression.
// It takes the DMI decode output, the DMI type, and the regular expression as input parameters.
// It returns the extracted value as a string.
func ValFromDmiDecodeRegexSubmatch(dmiDecodeOutput string, dmiType string, regex string) string {
	return ValFromRegexSubmatch(GetDmiDecodeType(dmiDecodeOutput, dmiType), regex)
}

// ValsArrayFromDmiDecodeRegexSubmatch extracts multiple values from DMI decode entries.
func ValsArrayFromDmiDecodeRegexSubmatch(dmiDecodeOutput string, dmiType string, regexes ...string) (vals [][]string) {
	var res []*regexp.Regexp
	for _, r := range regexes {
		re := regexp.MustCompile(r)
		res = append(res, re)
	}
	for _, entry := range GetDmiDecodeEntries(dmiDecodeOutput, dmiType) {
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

// GetDmiDecodeType extracts the lines from the given `dmiDecodeOutput` that belong to the specified `dmiType`.
func GetDmiDecodeType(dmiDecodeOutput string, dmiType string) string {
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

// GetDmiDecodeEntries extracts the entries from the given `dmiDecodeOutput` that belong to the specified `dmiType`.
func GetDmiDecodeEntries(dmiDecodeOutput string, dmiType string) (entries [][]string) {
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

// GetSectionsFromOutput parses output into sections, where the section name
// is the key in a map and the section content is the value
// sections are delimited by lines of the form ########## <section name> ##########
func GetSectionsFromOutput(output string) map[string]string {
	sections := make(map[string]string)
	re := regexp.MustCompile(`^########## (.+?) ##########$`)
	var sectionName string
	for line := range strings.SplitSeq(output, "\n") {
		// check if the line is a section header
		match := re.FindStringSubmatch(line)
		if match != nil {
			// if the section name isn't in the map yet, add it
			if _, ok := sections[match[1]]; !ok {
				sections[match[1]] = ""
			}
			// save the section name
			sectionName = match[1]
			continue
		}
		if sectionName != "" {
			sections[sectionName] += line + "\n"
		}
	}
	return sections
}

// SectionValueFromOutput returns the content of a section from the output
// if the section doesn't exist, returns an empty string
func SectionValueFromOutput(output string, sectionName string) string {
	sections := GetSectionsFromOutput(output)
	if len(sections) == 0 {
		slog.Warn("no sections in output")
		return ""
	}
	if _, ok := sections[sectionName]; !ok {
		slog.Warn("section not found in output", slog.String("section", sectionName))
		return ""
	}
	if sections[sectionName] == "" {
		slog.Warn("No content for section:", slog.String("section", sectionName))
		return ""
	}
	return sections[sectionName]
}
