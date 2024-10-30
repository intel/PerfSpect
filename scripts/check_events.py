#!/usr/bin/env python3
# check_events.py - checks if all events used in metrics are present in the events file
#
# Usage: check_events.py <perfspect_metrics.json> <perfspect_events.txt> [perfmon_events.json]

# Copyright (C) 2021-2024 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

import sys
import json


def get_event(line):
    if line != "" and not line.startswith("#"):
        if line.find("name=") >= 0:
            x = line[line.find("name=") + 5 :]
            x = x[x.find("'") + 1 :]
            x = x[0 : x.find("'")]
            if x.find(":") > 0:
                x = x[0 : x.find(":")]
        else:
            x = line[0:-2]
    else:
        x = None
    return x


# check_events.py perfspect_metrics.json perfspect_events.txt <perfmon_events.json>
def main():
    if len(sys.argv) < 3:
        print(
            "Usage: check_events.py <perfspect_metrics.json> <perfspect_events.txt> [perfmon_events.json]",
            file=sys.stderr,
        )
        sys.exit(1)
    metrics_file = sys.argv[1]
    events_file = sys.argv[2]

    with open(metrics_file, "r") as f:
        metrics = json.load(f)
    metric_list = {}
    used_events = {}  # event: count
    for m in metrics:
        metric = m["name"]
        formula = m["expression"]
        m_events = []
        start_bracket = formula.find("[")
        while start_bracket >= 0:
            end_bracket = formula.find("]")
            event = formula[start_bracket + 1 : end_bracket]
            if event not in [
                "SYSTEM_TSC_FREQ",
                "TSC",
                "CHAS_PER_SOCKET",
                "SOCKET_COUNT",
                "CORES_PER_SOCKET",
                "HYPERTHREADING_ON",
                "CONST_THREAD_COUNT",
            ]:
                if event.find(":") > 0 and event[-1] != "k":
                    event = event[0 : event.find(":")]
                if not event.startswith("const_"):
                    used_events[event] = used_events.get(event, 0) + 1
                m_events.append(event)
            formula = formula[end_bracket + 1 :]
            start_bracket = formula.find("[")
        metric_list[metric] = m_events

    event_list = []
    with open(events_file, "r") as f:
        for line in f:
            event = get_event(line)
            if event is not None and event != "" and event_list.count(event) == 0:
                event_list.append(event)

    taken_alone = []  # events that cannot be in the same group
    all_taken_alone = []
    if len(sys.argv) > 3:
        perfmon_events_file = sys.argv[3]
        with open(perfmon_events_file, "r") as f:
            perfmon_events = json.load(f)
        for pm_event in perfmon_events["Events"]:
            if pm_event["TakenAlone"] == 1:
                all_taken_alone.append(pm_event["EventName"])
            if pm_event["TakenAlone"] == 1 and pm_event["EventName"] in event_list:
                taken_alone.append(pm_event["EventName"])

    missing_events = [x for x in used_events.keys() if not x in event_list]
    unused_events = [x for x in event_list if not x in used_events.keys()]
    missing_events_str = "\n".join(missing_events)
    unused_events_str = "\n".join(unused_events)
    taken_alone_str = "\n".join(taken_alone)
    all_taken_alone_str = "\n".join(all_taken_alone)
    print(f"Missing events:\n{missing_events_str}\n")
    print(f"Unused events: \n{unused_events_str}")
    if len(sys.argv) > 3:
        print(f"\n'TakenAlone' events found in perfspect events: \n{taken_alone_str}\n")
        print(
            f"All 'TakenAlone' events found in perfmon events: \n{all_taken_alone_str}"
        )


if __name__ == "__main__":
    main()
