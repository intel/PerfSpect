#!/usr/bin/env python3

###########################################################################################################
# Copyright (C) 2021-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

from __future__ import print_function
import logging
import os
import platform
import re
import sys
import subprocess  # nosec
import shlex  # nosec
from argparse import ArgumentParser
from src import perf_helpers
from src import prepare_perf_events as prep_events

logging.basicConfig(
    format="%(asctime)s %(levelname)s: %(message)s",
    datefmt="%H:%M:%S",
    level=logging.NOTSET,
    handlers=[logging.FileHandler("debug.log"), logging.StreamHandler(sys.stdout)],
)
log = logging.getLogger(__name__)

SUPPORTED_ARCHITECTURES = [
    "Broadwell",
    "Skylake",
    "Cascadelake",
    "Icelake",
    "SapphireRapids",
]


# meta data gathering
def write_metadata(
    outcsv,
    collection_events,
    arch,
    cpuname,
    cpuid_info,
    interval,
    muxinterval,
    thread,
    socket,
    metadata_only=False,
):
    tsc_freq = str(perf_helpers.get_tsc_freq())
    data = ""
    time_stamp = ""
    validate_file(outcsv)
    with open(outcsv, "r") as original:
        time_stamp = original.readline()
        if metadata_only and time_stamp.startswith("### META DATA ###"):
            log.warning("Not prepending metadata, already present in %s " % (outcsv))
            return
        data = original.read()
    with open(outcsv, "w") as modified:
        modified.write("### META DATA ###,\n")
        modified.write("TSC Frequency(MHz)," + tsc_freq + ",\n")
        modified.write("CPU count," + str(perf_helpers.get_cpu_count()) + ",\n")
        modified.write("SOCKET count," + str(perf_helpers.get_socket_count()) + ",\n")
        modified.write("HT count," + str(perf_helpers.get_ht_count()) + ",\n")
        imc, cha, upi = perf_helpers.get_imc_cacheagent_count()
        modified.write("IMC count," + str(imc) + ",\n")
        modified.write("CHA count," + str(cha) + ",\n")
        modified.write("UPI count," + str(upi) + ",\n")
        modified.write("Sampling Interval," + str(interval) + ",\n")
        modified.write("Architecture," + str(arch) + ",\n")
        modified.write("Model," + str(cpuname) + ",\n")
        modified.write("kernel version," + perf_helpers.get_version() + "\n")
        for socket, cpus in cpuid_info.items():
            modified.write("Socket:" + str(socket) + ",")
            for c in cpus:
                modified.write(str(c) + ";")
            modified.write("\n")
        modified.write("Perf event mux Interval ms," + str(muxinterval) + ",\n")
        threadmode = "enabled" if thread else "disabled"
        socketmode = "enabled" if socket else "disabled"
        if args.cid is not None:
            cgname = "enabled," + perf_helpers.get_comm_from_cid(
                args.cid.split(","), cgroups
            )
        else:
            cgname = "disabled"
        modified.write("cgroups=" + str(cgname) + "\n")

        cpusets = ""
        if args.cid is not None:
            for cgroup in cgroups:
                cgroup_paths = [
                    "/sys/fs/cgroup/cpuset/" + cgroup + "/cpuset.cpus",  # cgroup v1
                    "/sys/fs/cgroup/" + cgroup + "/cpuset.cpus",  # cgroup v2
                ]
                cg_path_found = False
                for _ in cgroup_paths:
                    try:
                        cpu_set_file = open(
                            "/sys/fs/cgroup/cpuset/" + cgroup + "/cpuset.cpus", "r"
                        )
                        cg_path_found = True
                        # no need to check other paths
                        break
                    except FileNotFoundError:
                        # check next path
                        continue

                if cg_path_found:
                    cpu_set = cpu_set_file.read()
                    cpu_set_file.close()
                    cpu_set = cpu_set.strip()

                if not cg_path_found or cpu_set == "":
                    # A missing path or an empty cpu-set in v2 indicates that the container is running on all CPUs
                    cpu_set = "0-" + str(
                        int(
                            perf_helpers.get_cpu_count()
                            * perf_helpers.get_socket_count()
                            * perf_helpers.get_ht_count()
                            - 1
                        )
                    )

                cpusets += "," + cpu_set
        else:
            cpusets = ",disabled"

        modified.write("cpusets" + cpusets + ",\n")
        modified.write("Percore mode," + threadmode + ",\n")
        modified.write("Persocket mode," + socketmode + ",\n")
        modified.write("PerfSpect version," + perf_helpers.get_tool_version() + ",\n")
        modified.write("### PERF EVENTS ###" + ",\n")
        for e in collection_events:
            modified.write(e + "\n")
        modified.write("\n")
        modified.write("### PERF DATA ###" + ",\n")
        if time_stamp:
            zone = subprocess.check_output(  # nosec
                ["date", "+%Z"], universal_newlines=True  # nosec
            ).split()  # nosec
            epoch = str(perf_helpers.get_epoch(time_stamp))
            modified.write(
                time_stamp.rstrip() + " " + zone[0] + " EPOCH " + epoch + "\n"
            )
        modified.write(data)


def resource_path(relative_path):
    """Get absolute path to resource, works for dev and for PyInstaller"""
    base_path = getattr(sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__)))
    return os.path.join(base_path, relative_path)


def validate_perfargs(perf):
    """validate perf command before executing"""
    if perf[0] != "perf":
        log.error("Not a perf command, exiting!")
        sys.exit(1)


def validate_file(fname):
    """validate if file is accessible"""
    if not os.access(fname, os.R_OK):
        log.error(str(fname) + " not accessible")
        sys.exit(1)


def is_safe_file(fname, substr):
    """verify if file name/format is accurate"""
    if not fname.endswith(substr):
        log.error(str(fname) + " isn't appropriate format")
        sys.exit(1)


if __name__ == "__main__":
    if platform.system() != "Linux":
        log.error("PerfSpect currently supports Linux only")
        sys.exit(1)

    # fix the pyinstaller path
    script_path = os.path.dirname(os.path.realpath(__file__))
    if "_MEI" in script_path:
        script_path = script_path.rsplit("/", 1)[0]
    result_dir = script_path + "/results"
    default_output_file = result_dir + "/perfstat.csv"

    parser = ArgumentParser(description="perf-collect: Time series dump of PMUs")
    duration = parser.add_mutually_exclusive_group()
    runmode = parser.add_mutually_exclusive_group()
    duration.add_argument(
        "-t", "--timeout", type=int, default=None, help="perf event collection time"
    )
    duration.add_argument(
        "-a",
        "--app",
        type=str,
        default=None,
        help="Application to run with perf-collect, perf collection ends after workload completion",
    )
    runmode.add_argument(
        "-p", "--pid", type=str, default=None, help="perf-collect on selected PID(s)"
    )
    runmode.add_argument(
        "-c",
        "--cid",
        type=str,
        default=None,
        help="perf-collect on selected container ids",
    )
    runmode.add_argument(
        "--thread", help="Collect for thread metrics", action="store_true"
    )
    runmode.add_argument(
        "--socket", help="Collect for socket metrics", action="store_true"
    )
    parser.add_argument(
        "-V", "--version", help="display version info", action="store_true"
    )
    parser.add_argument(
        "-i",
        "--interval",
        type=float,
        default=1,
        help="interval in seconds for time series dump, default=1",
    )
    parser.add_argument(
        "-m",
        "--muxinterval",
        type=int,
        default=10,
        help="event mux interval in milli seconds, default=0 i.e. will use the system default",
    )
    parser.add_argument(
        "-o",
        "--outcsv",
        type=str,
        default=default_output_file,
        help="perf stat output in csv format, default=results/perfstat.csv",
    )
    parser.add_argument(
        "-v",
        "--verbose",
        help="Display debugging information",
        action="store_true",
    )
    args = parser.parse_args()

    if args.version:
        print(perf_helpers.get_tool_version())
        sys.exit(0)

    if os.geteuid() != 0:
        log.error("Must run PerfSpect as root, please re-run")
        sys.exit(1)

    # disable nmi watchdog before collecting perf
    nmi_watchdog = 0
    try:
        with open("/proc/sys/kernel/nmi_watchdog", "r+") as f_nmi:
            nmi_watchdog = f_nmi.read()
            if int(nmi_watchdog) != 0:
                f_nmi.write("0")
                log.info("nmi_watchdog disabled")
    except FileNotFoundError:
        pass

    initial_pmus = perf_helpers.pmu_contention_detect()
    interval = int(args.interval * 1000)

    if args.muxinterval > 1000:
        log.error("Input argument muxinterval is too large, max is [1s or 1000ms]")
        sys.exit(1)
    if args.interval < 0.1 or args.interval > 300:
        log.error(
            "Input argument dump interval is too large or too small, range is [0.1 to 300s]"
        )
        sys.exit(1)

    # select architecture default event file if not supplied
    procinfo = perf_helpers.get_cpuinfo()
    arch, cpuname = perf_helpers.get_arch_and_name(procinfo)
    if not arch:
        log.error(
            f"Unrecognized CPU architecture. Supported architectures: {', '.join(SUPPORTED_ARCHITECTURES)}"
        )
        sys.exit(1)
    eventfile = None
    if arch == "broadwell":
        eventfile = "bdx.txt"
    elif arch == "skylake":
        eventfile = "skx.txt"
    elif arch == "cascadelake":
        eventfile = "clx.txt"
    elif arch == "icelake":
        eventfile = "icx.txt"
    elif arch == "sapphirerapids":
        eventfile = "spr.txt"

    # Convert path of event file to relative path if being packaged by pyInstaller into a binary
    if getattr(sys, "frozen", False):
        basepath = getattr(sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__)))
        eventfilename = eventfile
        is_safe_file(eventfile, ".txt")
        eventfile = os.path.join(basepath, eventfile)
    elif __file__:
        eventfile = script_path + "/events/" + eventfile
        eventfilename = eventfile
    else:
        log.error("Unknown application type")
        sys.exit(1)

    if args.outcsv == default_output_file:
        # create results dir
        if not os.path.exists(result_dir):
            os.mkdir(result_dir)
            perf_helpers.fix_path_ownership(result_dir)
    else:
        if not perf_helpers.validate_outfile(args.outcsv):
            log.error(
                "Output filename not accepted. Filename should be a .csv without special characters"
            )
            sys.exit(1)

    mux_intervals = perf_helpers.get_perf_event_mux_interval()
    if args.muxinterval > 0:
        perf_helpers.set_perf_event_mux_interval(False, args.muxinterval, mux_intervals)

    # parse cgroups
    cgroups = []
    if args.cid is not None:
        cgroups = perf_helpers.get_cgroups_from_cids(args.cid.split(","))
        num_cgroups = len(cgroups)

    try:
        reg = r"^[0-9]*\.[0-9][0-9]*"
        kernel = perf_helpers.get_version().split("Linux version")[1].lstrip()
        significant_kernel_version = re.match(reg, kernel).group(0)
        full_kernel_version = kernel
    except Exception as e:
        log.exception(e)
        log.info("Unable to get kernel version")
        sys.exit(1)

    # get perf events to collect
    collection_events = []
    imc, cha, upi = perf_helpers.get_imc_cacheagent_count()
    have_uncore = True
    if imc == 0 and cha == 0 and upi == 0:
        log.info("disabling uncore (possibly in a vm?)")
        have_uncore = False
    events, collection_events = prep_events.prepare_perf_events(
        eventfile,
        (
            (args.pid or args.cid or args.thread or args.socket) is not None
            or not have_uncore
        ),
    )

    collection_type = "-a" if args.thread is False and args.socket is False else "-a -A"
    # start perf stat
    if args.pid and args.timeout:
        log.info("Only CPU/core events will be enabled with pid option")
        cmd = "perf stat -I %d -x , --pid %s -e %s -o %s sleep %d" % (
            interval,
            args.pid,
            events,
            args.outcsv,
            args.timeout,
        )

    elif args.pid:
        log.info("Only CPU/core events will be enabled with pid option")
        cmd = "perf stat -I %d -x , --pid %s -e %s -o %s" % (
            interval,
            args.pid,
            events,
            args.outcsv,
        )
    elif args.cid and args.timeout:
        log.info("Only CPU/core events will be enabled with cid option")
        perf_format = prep_events.get_cgroup_events_format(
            cgroups, events, len(collection_events)
        )
        cmd = "perf stat -I %d -x , %s -a -o %s sleep %d" % (
            interval,
            perf_format,
            args.outcsv,
            args.timeout,
        )
    elif args.cid:
        log.info("Only CPU/core events will be enabled with cid option")
        perf_format = prep_events.get_cgroup_events_format(
            cgroups, events, len(collection_events)
        )
        cmd = "perf stat -I %d -x , %s -o %s" % (interval, perf_format, args.outcsv)
    elif args.app:
        cmd = "perf stat %s -I %d -x , -e %s -o %s %s" % (
            collection_type,
            interval,
            events,
            args.outcsv,
            args.app,
        )
    elif args.timeout:
        cmd = "perf stat %s -I %d -x , -e %s -o %s sleep %d" % (
            collection_type,
            interval,
            events,
            args.outcsv,
            args.timeout,
        )
    else:
        cmd = "perf stat %s -I %d -x , -e %s -o %s" % (
            collection_type,
            interval,
            events,
            args.outcsv,
        )
    perfargs = shlex.split(cmd)
    validate_perfargs(perfargs)
    perf_helpers.pmu_contention_detect(msrs=initial_pmus, detect=True)
    if args.verbose:
        log.info(cmd)
    try:
        log.info("Collecting perf stat for events in : %s" % eventfilename)
        subprocess.call(perfargs)  # nosec
        log.info("Collection complete! Calculating TSC frequency now")
    except KeyboardInterrupt:
        log.info("Collection stopped! Caculating TSC frequency now")
    except Exception:
        log.error("perf encountered errors")
        sys.exit(1)

    cpuid_info = perf_helpers.get_cpuid_info(procinfo)
    write_metadata(
        args.outcsv,
        collection_events,
        arch,
        cpuname,
        cpuid_info,
        args.interval,
        args.muxinterval,
        args.thread,
        args.socket,
        False,
    )

    # reset nmi_watchdog to what it was before running perfspect
    with open("/proc/sys/kernel/nmi_watchdog", "w") as f_nmi:
        if int(nmi_watchdog) != 0:
            f_nmi.write(nmi_watchdog)
            log.info("nmi_watchdog re-enabled")

    perf_helpers.set_perf_event_mux_interval(True, 1, mux_intervals)

    log.info("perf stat dumped to %s" % args.outcsv)
    perf_helpers.fix_path_ownership(result_dir, True)
