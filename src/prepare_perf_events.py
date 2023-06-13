#!/usr/bin/env python3

###########################################################################################################
# Copyright (C) 2020-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

import logging
import os
import re
import src.perf_helpers as helper
from src.common import crash


# test if the event can be collected, check supported events in perf list
def is_collectable_event(event, perf_list):
    tmp_list = event.split("/")
    name = helper.get_dev_name(tmp_list[0])
    unc_name = "uncore_" + name
    sys_devs = helper.get_sys_devices()
    dev_support = False
    if name in sys_devs or unc_name in sys_devs:
        dev_support = True
    if (
        dev_support
        and len(tmp_list) > 1
        and (tmp_list[1].startswith("umask") or tmp_list[1].startswith("event"))
    ):
        return True

    test_event = event.split(";")[0]
    test_event = test_event.split(",")[0]
    test_event = test_event.split(":")[0]
    pattern = r"s*" + test_event
    if re.search(pattern, perf_list, re.MULTILINE):
        return True
    else:
        return False


# expand uncore event names
def expand_unc(line):
    line = line.strip()
    name = line.split("/")[0]
    unc_name = "uncore_" + name
    unc_count = 0
    sys_devs = helper.get_sys_devices()
    if unc_name in sys_devs:
        unc_count = int(sys_devs[unc_name])
    if unc_count > 1:
        line = line.replace(name, unc_name + "_0")
        if "name=" in line:
            prettyname = (line.split("'"))[1].strip()
            line = line.replace(prettyname, prettyname + ".0")
    return line, unc_count


# check if CPU/core event
def is_cpu_event(line):
    line = line.strip()
    tmp_list = line.split("/")
    # assumes event name without a PMU qualifier is a core event
    if (
        (len(tmp_list) == 1 or tmp_list[0] == "cpu" or tmp_list[0].startswith("cstate"))
        and "OCR." not in line
        and "power/" not in line
    ):
        return True
    return False


# save the last group names in a list when it is cha or imc
# test for cha or imc event. append with count value
# once reaches new group, start looping through all imc/cha counts to finish up
def enumerate_uncore(group, pattern, n, default_range=True):
    uncore_group = ""
    ids = []
    if default_range:
        ids = range(n)
    else:
        ids = helper.get_channel_ids()
    for i in range(n - 1):
        old = pattern + str(ids[i])
        new = pattern + str(ids[i + 1])
        group = group.replace(old, new)
        oldname = group.split("'")
        for j, n in enumerate(oldname):
            if "name=" in n:
                tmp = oldname[j + 1]
                idx = tmp.rfind(".")
                oldname[j + 1] = tmp[: idx + 1] + str(ids[i + 1])

        group = "'".join(oldname)
        uncore_group += group
    return uncore_group


# For multiple cgroup collection we need this format : “-e e1 -e e2 -G foo,foo -G bar,bar"
def get_cgroup_events_format(cgroups, events, num_events):
    eventlist = ""
    grouplist = ""
    # "-e" flags: Create event groups as many number of cgroups
    for _ in range(len(cgroups)):
        eventlist += " -e " + events

    # "-G" flags: Repeat cgroup name for as many events in each event group
    for cgroup in cgroups:
        grouplist = grouplist.rstrip(",") + " -G "
        for _ in range(num_events):
            grouplist += cgroup + ","

    perf_format = eventlist + grouplist.rstrip(",")
    return perf_format


def filter_events(event_file, cpu_only, PID_CID_mode, TMA_supported):
    if not os.path.isfile(event_file):
        crash("event file not found")
    collection_events = []
    unsupported_events = []
    perf_list = helper.get_perf_list()
    with open(event_file, "r") as fin:
        for line in fin:
            line = line.strip()
            if (
                line == ""
                or line.startswith("#")
                or (cpu_only and not is_cpu_event(line))
            ):
                continue
            if PID_CID_mode and line.startswith("cstate_"):
                continue
            if not TMA_supported and (
                "name='TOPDOWN.SLOTS'" in line or "name='PERF_METRICS." in line
            ):
                continue
            if not is_collectable_event(line, perf_list):
                # not a collectable event
                unsupported_events.append(line)
                # if this is the last event in the group, mark the previous event as the last (with a ';')
                if line.endswith(";") and len(collection_events) > 1:
                    end_event = collection_events[-1]
                    collection_events[-1] = end_event[:-1] + ";"
            else:
                collection_events.append(line)
        if any("cpu-cycles" in event for event in unsupported_events):
            crash("PMU's not available. Run in a full socket VM or baremetal")
        if len(unsupported_events) > 0:
            logging.warning(
                f"Perf unsupported events not counted: {unsupported_events}"
            )
    return collection_events, unsupported_events


def prepare_perf_events(event_file, cpu_only, PID_CID_mode, TMA_supported):
    start_group = "'{"
    end_group = "}'"
    group = ""
    prev_group = ""
    new_group = True

    collection_events, unsupported_events = filter_events(
        event_file, cpu_only, PID_CID_mode, TMA_supported
    )
    core_event = []
    uncore_event = []
    event_names = []
    for line in collection_events:
        if cpu_only:
            if is_cpu_event(line):
                event = line + ":c"
                core_event.append(event)
        else:
            if is_cpu_event(line):
                event = line + ":c"
                core_event.append(event)
            else:
                event = line + ":u"
                uncore_event.append(event)
        event_names.append(event)
        line, unc_count = expand_unc(line)
        if new_group:
            group += start_group
            prev_group = start_group
            new_group = False
        if line.endswith(";"):
            group += line[:-1] + end_group + ","
            prev_group += line[:-1] + end_group + ","
            new_group = True
        else:
            group += line
            prev_group += line

        # enumerate all uncore units
        if new_group and (unc_count > 1):
            name = helper.get_dev_name(line.split("/")[0].strip())
            default_range = name != "uncore_imc"
            group += enumerate_uncore(prev_group, name + "_", unc_count, default_range)

    group = group[:-1]
    if len(event_names) == 0:
        crash("No supported events found on this platform.")
    # being conservative not letting the collection to proceed if fixed counters aren't supported on the platform
    if len(unsupported_events) >= len(core_event):
        crash(
            "Most core counters aren't supported on this platform, unable to collect PMUs"
        )
    return group, event_names
