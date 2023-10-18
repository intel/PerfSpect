#!/usr/bin/env python3

###########################################################################################################
# Copyright (C) 2021-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

import logging
import os
import platform
import shlex
import shutil
import subprocess
import sys

from argparse import ArgumentParser
from src.common import configure_logging, crash
from src.perf_helpers import get_perf_list


def fix_path(script):
    return os.path.join(
        getattr(sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__))),
        script,
    )


def attach_perf_map_agent():
    # look for java processes
    try:
        pids = (
            subprocess.check_output(shlex.split("pgrep java"), encoding="UTF-8")
            .strip()
            .split("\n")
        )
    except subprocess.CalledProcessError:
        return

    if len(pids) > 0 and pids[0] != "":
        logging.info("detected java processes: " + str(pids))

        # setup tmp folder for storing perf-map-agent
        if not os.path.exists("/tmp/perfspect"):
            os.mkdir("/tmp/perfspect")
            shutil.copy(fix_path("attach-main.jar"), "/tmp/perfspect")
            shutil.copy(fix_path("libperfmap.so"), "/tmp/perfspect")
            os.chmod("/tmp/perfspect/attach-main.jar", 0o666)
            os.chmod("/tmp/perfspect/libperfmap.so", 0o666)

        for pid in pids:
            uid = subprocess.check_output(
                shlex.split("awk '/^Uid:/{print $2}' /proc/" + pid + "/status"),
                encoding="UTF-8",
            )
            gid = subprocess.check_output(
                shlex.split("awk '/^Gid:/{print $2}' /proc/" + pid + "/status"),
                encoding="UTF-8",
            )
            JAVA_HOME = subprocess.check_output(
                shlex.split('sed "s:bin/java::"'),
                input=subprocess.check_output(
                    shlex.split("readlink -f /usr/bin/java"), encoding="UTF-8"
                ),
                encoding="UTF-8",
            ).strip()
            current_dir = os.getcwd()
            try:
                os.chdir("/tmp/perfspect/")
                subprocess.check_call(
                    shlex.split(
                        f"sudo -u \\#{uid} -g \\#{gid} {JAVA_HOME}bin/java -cp /tmp/perfspect/attach-main.jar:{JAVA_HOME}lib/tools.jar net.virtualvoid.perf.AttachOnce {pid}"
                    ),
                    encoding="UTF-8",  # type: ignore
                )
                logging.info("Successfully attached perf-map-agent to: " + pid)
            except subprocess.CalledProcessError:
                logging.info("Failed to attach perf-map-agent to: " + pid)
            os.chdir(current_dir)


if __name__ == "__main__":
    configure_logging(".")

    parser = ArgumentParser(
        description="hotspot: PMU based flamegraphs for hotspot analysis"
    )
    parser.add_argument(
        "-t",
        "--timeout",
        required=True,
        type=int,
        help="collection time",
    )
    args = parser.parse_args()
    if os.geteuid() != 0:
        crash("Must run as root, please re-run")
    if platform.system() != "Linux":
        crash("PerfSpect currently supports Linux only")
    get_perf_list()

    events = ["instructions", "cycles", "branch-misses", "cache-misses"]

    logging.info("collecting...")

    attach_perf_map_agent()

    subprocess.run(
        shlex.split(
            "sudo perf record -a -g -F 99 -e "
            + ",".join(events)
            + " sleep "
            + str(args.timeout)
        )
    )

    logging.info("postprocessing...")

    script = subprocess.run(
        shlex.split("perf script"),
        stdout=subprocess.PIPE,
    )
    cycles_collapse = ""
    with open("cycles.col", "w") as c:
        cycles_collapse = subprocess.run(
            shlex.split(fix_path("stackcollapse-perf.pl") + ' --event-filter="cycles"'),
            input=script.stdout,
            stdout=c,
        )
    for event, subtitle, differential in [
        ["branch-misses", "What is being stalled by poor prefetching", False],
        ["cache-misses", "What is being stalled by poor caching", False],
        ["instructions", "CPI: blue = vectorized, red = stalled", True],
    ]:
        with open(event + ".svg", "w") as f:
            collapse = ""
            with open(event + ".col", "w") as e:
                collapse = subprocess.run(
                    shlex.split(
                        fix_path("stackcollapse-perf.pl")
                        + ' --event-filter="'
                        + event
                        + '"'
                    ),
                    input=script.stdout,
                    stdout=e,
                )
            if differential:
                with open("diff.col", "w") as e:
                    collapse = subprocess.run(
                        shlex.split(
                            fix_path("difffolded.pl") + " " + event + ".col cycles.col"
                        ),
                        stdout=e,
                    )
            with open("diff.col" if differential else event + ".col", "r") as e:
                flamegraph = subprocess.run(
                    shlex.split(
                        fix_path("flamegraph.pl")
                        + ' --title="'
                        + event
                        + '" --subtitle="'
                        + subtitle
                        + '"'
                    ),
                    stdin=e,
                    stdout=f,
                )
            os.remove(event + ".col")
            if differential:
                os.remove("diff.col")
        os.chmod(event + ".svg", 0o666)
        logging.info("generated " + event + ".svg")

    os.remove("cycles.col")
