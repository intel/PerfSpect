#!/usr/bin/env python3
# perfmonevents2perfspect.py - generates perfspect formatted events from perfmon events json file and a file of event names
#
# Usage: perfmonevents2perfspect.py <perfmon json file> <event names file>
#
# New perfspect formatted events are printed to stdout.


# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

import sys
import json

def format_event(p):
    """
    Format a single event from perfmon to perfspect format.
    """
    eventParts = []
    unit = p.get("Unit", "cpu")
    unit = unit.split(" ")[0]  # take the first part of the unit if it has multiple parts
    unit = unit.lower()
    # <unit>/event
    eventParts.append(f"{unit}/event={p['EventCode']}")
    # umask
    if p.get("UMaskExt", "") != "": # only uncore events
        umask_value = int(p["UMask"], 16)  # convert hex to int
        umask_hex = f"{umask_value:02x}"
        umaskext_value = int(p["UMaskExt"], 16)  # convert hex to int
        umaskext_hex = f"{umaskext_value:x}"
        eventParts.append(f"umask=0x{umaskext_hex}{umask_hex}")
    else:
        eventParts.append(f"umask={p['UMask']}")
    # cmask
    if p.get("CounterMask", "") != "": # only core events
        if p["CounterMask"] != "0":
            # convert to int
            cmask_value = int(p["CounterMask"])
            # convert to hex, pad with zeros to 2 digits for consistency
            cmask_hex = f"0x{cmask_value:02x}"
            eventParts.append(f"cmask={cmask_hex}")
    # period
    if p.get("SampleAfterValue", "") != "": # only core events
        eventParts.append(f"period={p['SampleAfterValue']}")
    # offcore_rsp
    if p.get("Offcore", "") != "":  # only core events
        if p["Offcore"] == "1":
            eventParts.append(f"offcore_rsp={p['MSRValue']}")
    # name
    eventParts.append(f"name='{p['EventName']}'")
    event = ",".join(eventParts)
    event += "/"
    # Add optional comment with counter and takenAlone info
    # if counter is not "0,1,2,3,4,5,6,7" or takenAlone is "1", add them to the comment
    counter = ""
    takenAlone = ""
    comment = ""
    if p["Counter"] != "0,1,2,3,4,5,6,7":
        counter = p['Counter']
    if p.get("TakenAlone", "") != "": # only core events
        if p["TakenAlone"] == "1":
            takenAlone = "[*]"
    if counter != "" or takenAlone != "":
        comment = f"     # {counter}{takenAlone}"
    if comment != "":
        event = event.ljust(100) + comment
    return event

def format_all_events(events):
    """
    Format all events from perfmon to perfspect format.
    """
    result = []
    for p in events:
        result.append(format_event(p))
    return result

def format_event_list(events, event_names):
    """
    Format a list of events from perfmon to perfspect format.
    """
    result = []
    not_found = []
    for e in event_names:
        e = e.strip()
        for p in events:
            if p["EventName"] == e:
                result.append(format_event(p))
                break
        else:
            not_found.append(e)
    return result, not_found

# arg1 - perfmon core event json file (downloaded from github.com/intel/perfmon)
# arg2 - file containing list of event names, one event name per line [optional]
# if 2nd argument is not provided, all events from perfmon json file are used
if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(
            "Usage: perfmonevents2perfspect.py <perfmon json file> [<event names file>]",
            file=sys.stderr,
        )
        sys.exit(1)

    # read perfmon json file
    with open(sys.argv[1], "r") as f:
        perfmon = json.load(f)

    # read list of event names
    if len(sys.argv) < 3:
        # if no event names file is provided, use all events from perfmon json
        result = format_all_events(perfmon["Events"])
        not_found = []
    else:
        # read event names from file
        with open(sys.argv[2], "r") as f:
            events = f.readlines()
            events = [e.strip() for e in events if e.strip()]
        result, not_found = format_event_list(perfmon["Events"], events)

    # print perfspect formatted events to stdout
    for r in result:
        print(r)

    # print the not found event names to stderr
    if not_found:
        print("\nNot Found Events:", file=sys.stderr)
        for n in not_found:
            print(n, file=sys.stderr)
