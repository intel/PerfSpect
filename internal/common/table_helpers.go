// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// table_helpers.go contains base helper functions that are used to extract values from the output of the scripts.

package common

import (
	"fmt"
	"log/slog"
	"perfspect/internal/cpus"
	"perfspect/internal/script"
	"perfspect/internal/util"
	"regexp"
	"strconv"
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
// example:
// ########## <section A name> ##########
// <section content>
// <section content>
// ########## <section B name> ##########
// <section content>
//
// returns a map of section name to section content
// if the output is empty or contains no section headers, returns an empty map
// if a section contains no content, the value for that section is an empty string
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
// if the section exists but has no content, returns an empty string
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

// UarchFromOutput returns the architecture of the CPU that matches family, model, stepping,
// capid4, and devices information from the output or an empty string, if no match is found.
func UarchFromOutput(outputs map[string]script.ScriptOutput) string {
	family := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU family:\s*(.+)$`)
	model := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model:\s*(.+)$`)
	stepping := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Stepping:\s*(.+)$`)
	capid4 := ValFromRegexSubmatch(outputs[script.LspciBitsScriptName].Stdout, `^([0-9a-fA-F]+)`)
	devices := ValFromRegexSubmatch(outputs[script.LspciDevicesScriptName].Stdout, `^([0-9]+)`)
	cpu, err := cpus.GetCPU(cpus.NewX86Identifier(family, model, stepping, capid4, devices))
	if err != nil {
		slog.Error("error getting CPU characteristics", slog.String("error", err.Error()))
		return ""
	}
	return cpu.MicroArchitecture
}

func HyperthreadingFromOutput(outputs map[string]script.ScriptOutput) string {
	family := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU family:\s*(.+)$`)
	model := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Model:\s*(.+)$`)
	stepping := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Stepping:\s*(.+)$`)
	sockets := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(s\):\s*(.+)$`)
	coresPerSocket := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Core\(s\) per socket:\s*(.+)$`)
	cpuCount := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^CPU\(.*:\s*(.+?)$`)
	onlineCpus := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^On-line CPU\(s\) list:\s*(.+)$`)
	threadsPerCore := ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Thread\(s\) per core:\s*(.+)$`)

	numCPUs, err := strconv.Atoi(cpuCount) // logical CPUs
	if err != nil {
		slog.Error("error parsing cpus from lscpu")
		return ""
	}
	onlineCpusList, err := util.SelectiveIntRangeToIntList(onlineCpus) // logical online CPUs
	numOnlineCpus := len(onlineCpusList)
	if err != nil {
		slog.Error("error parsing online cpus from lscpu")
		numOnlineCpus = 0 // set to 0 to indicate parsing failed, will use numCPUs instead
	}
	numThreadsPerCore, err := strconv.Atoi(threadsPerCore) // logical threads per core
	if err != nil {
		slog.Error("error parsing threads per core from lscpu")
		numThreadsPerCore = 0
	}
	numSockets, err := strconv.Atoi(sockets)
	if err != nil {
		slog.Error("error parsing sockets from lscpu")
		return ""
	}
	numCoresPerSocket, err := strconv.Atoi(coresPerSocket) // physical cores
	if err != nil {
		slog.Error("error parsing cores per sockets from lscpu")
		return ""
	}
	cpu, err := cpus.GetCPU(cpus.NewX86Identifier(family, model, stepping, "", ""))
	if err != nil {
		slog.Warn("error getting CPU characteristics", slog.String("error", err.Error()))
		return ""
	}
	if numOnlineCpus > 0 && numOnlineCpus < numCPUs {
		// if online CPUs list is available, use it to determine the number of CPUs
		// supersedes lscpu output of numCPUs which counts CPUs on the system, not online CPUs
		numCPUs = numOnlineCpus
	}
	if cpu.LogicalThreadCount < 2 {
		return "N/A"
	} else if numThreadsPerCore == 1 {
		// if threads per core is 1, hyperthreading is disabled
		return "Disabled"
	} else if numThreadsPerCore >= 2 {
		// if threads per core is greater than or equal to 2, hyperthreading is enabled
		return "Enabled"
	} else if numCPUs > numCoresPerSocket*numSockets {
		// if the threads per core attribute is not available, we can still check if hyperthreading is enabled
		// by checking if the number of logical CPUs is greater than the number of physical cores
		return "Enabled"
	} else {
		return "Disabled"
	}
}

func OperatingSystemFromOutput(outputs map[string]script.ScriptOutput) string {
	os := ValFromRegexSubmatch(outputs[script.EtcReleaseScriptName].Stdout, `^PRETTY_NAME=\"(.+?)\"`)
	centos := ValFromRegexSubmatch(outputs[script.EtcReleaseScriptName].Stdout, `^(CentOS Linux release .*)`)
	if centos != "" {
		os = centos
	}
	return os
}

func TDPFromOutput(outputs map[string]script.ScriptOutput) string {
	msrHex := strings.TrimSpace(outputs[script.PackagePowerLimitName].Stdout)
	msr, err := strconv.ParseInt(msrHex, 16, 0)
	if err != nil || msr == 0 {
		return ""
	}
	return fmt.Sprint(msr/8) + "W"
}
