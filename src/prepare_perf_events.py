###########################################################################################################
# Copyright (C) 2021 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

import os
import re
import subprocess  # nosec
import src.perf_helpers as helper


# test if the event can be collected, check supported events in perf list
# def filter_func(event, imc_count, cha_count, cbox_count, upi_count, perf_list):
def filter_func(event, perf_list):
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
def expand_unc(line, grouping):
    line = line.strip()
    name = line.split("/")[0]
    unc_name = "uncore_" + name
    unc_count = 0
    sys_devs = helper.get_sys_devices()
    if unc_name in sys_devs:
        unc_count = int(sys_devs[unc_name])
    if grouping and (unc_count > 1):
        line = line.replace(name, unc_name + "_0")
        if "name=" in line:
            prettyname = (line.split("'"))[1].strip()
            line = line.replace(prettyname, prettyname + ".0")
    return line, unc_count


# check if CPU/core event
def check_cpu_event(line):
    line = line.strip()
    tmp_list = line.split("/")
    # assumes event name without a PMU qualifier is a core event
    if len(tmp_list) == 1 or tmp_list[0] == "cpu":
        return True
    return False


# expand event names - deprecated
def expand_event_name(line, grouping):
    line = line.strip()
    loop_imc = False
    loop_cha = False
    loop_cbox = False
    loop_upi = False
    if grouping and line.startswith("imc"):
        line = line.replace("imc", "uncore_imc_0")
        if "name=" in line:
            name = (line.split("'"))[1]
            line = line.replace(name, name + ".0")
        loop_imc = True
    if grouping and line.startswith("cha"):
        if "name=" in line:
            name = (line.split("'"))[1]
            line = line.replace(name, name + ".0")
        line = line.replace("cha", "uncore_cha_0")
        loop_cha = True
    if grouping and line.startswith("cbox"):
        if "name=" in line:
            name = (line.split("'"))[1]
            line = line.replace(name, name + ".0")
        line = line.replace("cbox", "uncore_cbox_0")
        loop_cbox = True
    if grouping and line.startswith("upi"):
        if "name=" in line:
            name = (line.split("'"))[1]
            line = line.replace(name, name + ".0")
        line = line.replace("upi", "uncore_upi_0")
        loop_upi = True
    if grouping and line.startswith("qpi"):
        if "name=" in line:
            name = (line.split("'"))[1]
            line = line.replace(name, name + ".0")
        line = line.replace("qpi", "uncore_qpi_0")
        loop_upi = True
    return (line, loop_imc, loop_cha, loop_cbox, loop_upi)


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


# get number of cgroups in the list
def get_num_cgroups(cgroups):
    return len(cgroups.split(","))


# fix events that aren't compatible with older kernels
def fix_events_for_older_kernels(eventfile, kernel_version):
    reg = r".*?\-(\d*)(\.|\-).*"
    match = re.match(reg, kernel_version)
    kernel_part_number = match.group(1)
    if int(kernel_part_number) >= 1000:
        if not os.path.isfile(eventfile):
            raise SystemExit("event file not found")

        with open(eventfile, "r") as fin:
            lines = fin.readlines()
        with open(eventfile, "w") as f:
            for line in lines:
                if (
                    line.strip("\n")
                    == "cpu/event=0x80,umask=0x4,cmask=0x1,edge=0x1,name='ICACHE_16B.c1_e1_IFDATA_STALL'/,"
                ):
                    f.write(
                        "cpu/event=0x80,umask=0x4,cmask=0x1,edge=0x1,name='ICACHE_16B_c1_e1_IFDATA_STALL'/,\n"
                    )
                    continue
                if (
                    line.strip("\n")
                    != "cstate_pkg/c6-residency,name='FREERUN_CORE_C6_RESIDENCY'/;"
                ):
                    f.write(line)


# For multiple cgroup collection we need this format : “-e e1 -e e2 -G foo,foo -G bar,bar"
def get_cgroup_events_format(cgroups, events, num_events):
    eventlist = ""
    grouplist = ""
    # Find total number of cgroups
    num_cgroups = len(cgroups)
    # cgroups = cgroups.split(",")
    # "-e" flags: Create event groups as many number of cgroups
    for i in range(num_cgroups):
        eventlist += " -e " + events

    # "-G" flags: Repeat cgroup name for as many events in each event group
    for cgroup in cgroups:
        grouplist = grouplist.rstrip(",") + " -G "
        for i in range(num_events):
            grouplist += cgroup + ","

    perf_format = eventlist + grouplist.rstrip(",")
    return perf_format


def prepare_perf_events(event_file, grouping, cpu_only):

    if not os.path.isfile(event_file):
        raise SystemExit("event file not found")

    start_group = "'{"
    end_group = "}'"
    group = ""
    prev_group = ""
    new_group = True

    if not grouping:
        start_group = ""
        end_group = ""

    collection_events = []

    with open(event_file, "r") as fin:
        # get supported perf events
        try:
            perf_list = subprocess.check_output(  # nosec
                ["perf", "list"], universal_newlines=True
            )
        except FileNotFoundError:
            raise SystemExit("perf not found; please install linux perf utility")
        unsupported_events = []
        for line in fin:
            if (line != "\n") and (not line.startswith("#")):
                line = line.strip()
                if cpu_only and (not check_cpu_event(line)):
                    continue
                if not filter_func(line, perf_list):
                    unsupported_events.append(line)
                    if line.endswith(";") and (len(collection_events) > 1):
                        end_event = str(collection_events[-1])
                        collection_events[-1] = end_event[:-1] + ";"
                else:
                    collection_events.append(line)
        if len(unsupported_events) > 0:
            print(
                "These events are not supported with current version of perf, will not be collected!"
            )
            for e in unsupported_events:
                print("%s" % e)
        if not grouping:
            templist = []
            for e in collection_events:
                # remove any ';' at the end that indicates end of group
                e = e.strip()[:-1] + ","
                if e not in templist:
                    templist.append(e)
            collection_events = templist

    event_names = []
    for line in collection_events:
        event = line + ":c" if check_cpu_event(line) else line + ":u"
        event_names.append(event)
        # line, loop_imc, loop_cha, loop_cbox, loop_upi = expand_event_name(line, grouping)
        line, unc_count = expand_unc(line, grouping)
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

    fin.close()
    group = group[:-1]

    return group, event_names
