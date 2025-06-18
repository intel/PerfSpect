#!/usr/bin/env python3
# perfmonmetrics2perfspect.py - converts a metrics file to the YAML file format used by PerfSpect 3.0+.
# 
# The input metrics file can be one of the following:
# - perfmon metrics json file from github.com/intel/perfmon
# - xml file from perfmon.intel.com or EMON/EDP release
#
# Usage: perfmonmetrics2perfspect.py <metric file>
#
# New metrics file in perfspect format is printed to stdout.

# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

import sys
import json

import xml.etree.ElementTree as ET

def replace_vars_in_formula(vars, formula):
    varMap = {
        "[INST_RETIRED.ANY]": "[instructions]",
        "[CPU_CLK_UNHALTED.THREAD]": "[cpu-cycles]",
        "[CPU_CLK_UNHALTED.REF]": "[ref-cycles]",
        "[CPU_CLK_UNHALTED.REF_TSC]": "[ref-cycles]",
        "DURATIONTIMEINSECONDS": "1",
        "[DURATIONTIMEINMILLISECONDS]": "1000",
        "[TOPDOWN.SLOTS:perf_metrics]": "[TOPDOWN.SLOTS]",
        "[OFFCORE_REQUESTS_OUTSTANDING.ALL_DATA_RD:c4]": "[OFFCORE_REQUESTS_OUTSTANDING.DATA_RD:c4]",
        "[system.tsc_freq]": "[SYSTEM_TSC_FREQ]",
        "[system.cha_count/system.socket_count]": "[CHAS_PER_SOCKET]",
        "[system.socket_count]": "[SOCKET_COUNT]",
    }
    newFormula = ""
    i = 0
    while i < len(formula):
        if formula[i].isalpha() or formula[i] == "_":
            x = formula[i]
            k = i + 1
            while k < len(formula) and (formula[k].isalpha() or formula[k] == "_"):
                x += formula[k]
                k += 1
            if vars.get(x) is not None:
                newFormula = newFormula + "[" + vars[x] + "]"
            else:
                newFormula = newFormula + formula[i:k]
            i = k
        else:
            newFormula += formula[i]
            i += 1
    for v in varMap:
        newFormula = newFormula.replace(v, varMap[v])
    return newFormula

def translate_perfmon_json_metrics_to_perfspect(inFile):
    with open(inFile, "r") as f:
        mf = json.load(f)

    if mf.get("Metrics") is None:
        print(f"ERROR: No metrics were found in {inFile}", file=sys.stderr)
        return

    print(f"Metrics in {inFile}: {len(mf['Metrics'])}", file=sys.stderr)
    vars = {}
    result = []
    for m in mf["Metrics"]:
        # if m.get("Category") != "TMA":
        #     continue
        # if m.get("Category") == "TMA" and m.get("Level") > 4:
        #     continue
        # if m.get("LegacyName") and m["LegacyName"].startswith("metric_TMA_Info_"):
        #     continue
        # if m.get("LegacyName") and m["LegacyName"].startswith("metric_TMA_Bottleneck_"):
        #     continue
        vars.clear()
        metric = {}
        # strip metric_ prefix from the name
        if m.get("LegacyName") is None:
            print(f"ERROR: Metric {m['Name']} has no LegacyName", file=sys.stderr)
            continue
        if m["LegacyName"].startswith("metric_"):
            metric["name"] = m["LegacyName"][len("metric_") :]
        else:
            metric["name"] = m["LegacyName"]
        # not yet :metric["description"] = m.get("BriefDescription", "")
        # extract the events and constants
        for e in m["Events"]:
            vars[e["Alias"]] = e["Name"]
        for c in m["Constants"]:
            vars[c["Alias"]] = c["Name"]
        # count the parentheses in the formula
        if m["Formula"].count("(") != m["Formula"].count(")"):
            print(
                f"ERROR: Perfmon metric {m['Name']} has mismatched parentheses in Formula: {m['Formula']}",
                file=sys.stderr,
            )
            continue
        # convert the formula
        metric["expression"] = replace_vars_in_formula(vars, m["Formula"])
        # count the parentheses in the expression
        if metric["expression"].count("(") != metric["expression"].count(")"):
            print(
                f"ERROR: PerfSpect metric {m['Name']} has mismatched parentheses in expression: {metric['expression']}",
                file=sys.stderr,
            )
            continue
        result.append(metric)
    return result

# this function has the following known limitations:
# - it does not convert the max notation, e.g., [(val1), (val2)].max
# - it does not convert the list index notation, e.g., val[0][0]
def translate_perfmon_xml_metrics_to_perfspect(inFile):
    tree = ET.parse(inFile)
    root = tree.getroot()
    vars = {}
    result = []
    for m in root:
        vars.clear()
        metric = {}
        metric["name"] = m.attrib["name"]
        # extract the events and constants
        for e in m.findall("event"):
            vars[e.attrib["alias"]] = e.text
        for c in m.findall("constant"):
            vars[c.attrib["alias"]] = c.text
        # convert the formula
        formula = m.find("formula").text
        metric["expression"] = replace_vars_in_formula(vars, formula)
        result.append(metric)

    return result

# translate perfmon metrics file to perfspect style metrics file
# inFile - perfmon_metrics.json file
def translate_perfmon_metrics_to_perfspect(inFile):
    # the file can be either a json file or an xml file
    fileType = inFile.split(".")[-1]
    if fileType == "json":
        result = translate_perfmon_json_metrics_to_perfspect(inFile)
    elif fileType == "xml":
        result = translate_perfmon_xml_metrics_to_perfspect(inFile)
    else:
        print(f"ERROR: Unsupported file type {fileType}", file=sys.stderr)
        return

    print(f"Generated metrics: {len(result)}", file=sys.stderr)
    json_object = json.dumps(result, indent=4)
    print(json_object)


# arg1 - perfmon metrics json file from github.com/intel/perfmon or xml file from perfmon.intel.com or EMON/EDP release
if __name__ == "__main__":
    if len(sys.argv) != 2:
        print(
            "Usage: perfmonmetrics2perfspect.py <perfmon metric json or xml file>",
            file=sys.stderr,
        )
        sys.exit(1)

    translate_perfmon_metrics_to_perfspect(sys.argv[1])
