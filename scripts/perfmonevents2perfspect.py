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

# arg1 - perfmon event json file (exported from perfmon.intel.com)
# arg2 - file containing list of event names, one event name per line
if __name__ == "__main__":
    if len(sys.argv) < 3:
        print(
            "Usage: perfmonevents2perfspect.py <perfmon json file> <event names file>",
            file=sys.stderr,
        )
        sys.exit(1)

    # read perfmon json file
    with open(sys.argv[1], "r") as f:
        perfmon = json.load(f)

    # read list of event names
    with open(sys.argv[2], "r") as f:
        events = f.readlines()

    # for each event name, find corresponding event in perfmon json and create a perfspect formatted event
    # example: cpu/event=0x71,umask=0x00,period=1000003,name='TOPDOWN_FE_BOUND.ALL'/
    # if event name is not found in perfmon json file, it is added to a list of not found events
    result = []
    notfound = []
    for e in events:
        e = e.strip()
        for p in perfmon["Events"]:
            if p["EventName"] == e:
                eventParts = []
                unit = p.get("Unit", "cpu")
                unit = unit.split(" ")[0]  # take the first part of the unit if it has multiple parts
                unit = unit.lower()
                eventParts.append(f"{unit}/event={p['EventCode']}")
                eventParts.append(f"umask={p['UMask']}")
                if p.get("SampleAfterValue", "") != "":
                    eventParts.append(f"period={p['SampleAfterValue']}")
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
                if p.get("TakenAlone", "0") == "1":
                    takenAlone = "[*]"
                if counter != "" or takenAlone != "":
                    comment = f"     # {counter}{takenAlone}"
                if comment != "":
                    event = event.ljust(80) + comment
                result.append(event)
                break
        else:
            notfound.append(e)

    # print perfspect formatted events to stdout
    for r in result:
        print(r)

    # print the not found event names to stderr
    print("\nNot Found Events:", file=sys.stderr)
    for n in notfound:
        print(n, file=sys.stderr)
