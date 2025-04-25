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
    # example: cpu/event=0x71,umask=0x00,name='TOPDOWN_FE_BOUND.ALL'/
    # if event name is not found in perfmon json file, it is added to a list of not found events
    result = []
    notfound = []
    for e in events:
        e = e.strip()
        for p in perfmon["Events"]:
            if p["EventName"] == e:
                # event = f"cpu/event={p['EventCode']},umask={p['UMask']},cmask={p['CMask']},name='{p['EventName']}'/"
                event = f"cpu/event={p['EventCode']},umask={p['UMask']},name='{p['EventName']}'/"
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
