#!/usr/bin/env python3
# filterperfspectmetrics.py - generates a metrics file based on perfmon metrics and a perfspect metrics file
#
# Usage: filterperfspectmetrics.py <all metrics in perfspect style> <pre-existing perfspect metrics file>
#
# New metrics file in perfspect format is printed to stdout.

# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

import sys
import json

# generate metrics file, based on perfmon metrics (allFile)
# usedFile - current perfspect metrics file
# if metric from usedFile not found in perfmon list,
# add it to filtered output as is and add field "origin"="perfspect"
def generate_final_metrics_list(allFile, usedFile):
    with open(allFile, "r") as f:
        allMetrics = json.load(f)
    with open(usedFile, "r") as f:
        usedMetrics = json.load(f)
    result = []
    for m in allMetrics:
        found = find_metric(usedMetrics, m["name"])
        if not found:
            print(f"Not including metric {m['name']} from perfmon", file=sys.stderr)
    for m in usedMetrics:
        found = find_metric(allMetrics, m["name"])
        if found is not None:
            result.append(found)
        else:
            m["origin"] = "perfspect"
            print(f"Adding metric {m['name']} from perfspect", file=sys.stderr)
            result.append(m)

    print(f"PerfSpect metrics: {len(result)}", file=sys.stderr)
    json_object = json.dumps(result, indent=4)
    print(json_object)


# find metric by name
def find_metric(metrics, metric_name):
    for m in metrics:
        if m["name"] == metric_name:
            return m
    return None


# arg 1 - all perfmon metrics in "perfspect" style
# arg 2 - pre-existing perfspect metrics file
if __name__ == "__main__":
    if len(sys.argv) != 3:
        print(
            "Usage: filterperfspectmetrics.py <all metrics in perfspect style> <pre-existing perfspect metrics file>",
            file=sys.stderr,
        )
        sys.exit(1)

    generate_final_metrics_list(sys.argv[1], sys.argv[2])
