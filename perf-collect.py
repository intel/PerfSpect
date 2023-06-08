#!/usr/bin/env python3

###########################################################################################################
# Copyright (C) 2021-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

import json
import logging
import os
import platform
import sys
import subprocess  # nosec
import shlex  # nosec
import time
from argparse import ArgumentParser
from src import perf_helpers
from src import prepare_perf_events as prep_events
from src.common import crash
from src import common


SUPPORTED_ARCHITECTURES = [
    "Broadwell",
    "Skylake",
    "Cascadelake",
    "Icelake",
    "SapphireRapids",
    "EmeraldRapids",
]


# meta data gathering
def write_metadata(
    outcsv,
    collection_events,
    arch,
    cpuname,
    cpuid_info,
    muxinterval,
    thread,
    socket,
    psi,
):
    tsc_freq = str(perf_helpers.get_tsc_freq())
    data = ""
    time_stamp = ""
    validate_file(outcsv)
    with open(outcsv, "r") as original:
        time_stamp = original.readline()
        data = original.read()
    with open(outcsv, "w") as modified:
        modified.write("### META DATA ###,\n")
        modified.write("SYSTEM_TSC_FREQ (MHz)," + tsc_freq + ",\n")
        modified.write("CORES_PER_SOCKET," + str(perf_helpers.get_cpu_count()) + ",\n")
        modified.write("SOCKET_COUNT," + str(perf_helpers.get_socket_count()) + ",\n")
        modified.write("HYPERTHREADING_ON," + str(perf_helpers.get_ht_status()) + ",\n")
        imc, upi = perf_helpers.get_imc_upi_count()
        cha = perf_helpers.get_cha_count()
        modified.write("IMC count," + str(imc) + ",\n")
        modified.write("CHAS_PER_SOCKET," + str(cha) + ",\n")
        modified.write("UPI count," + str(upi) + ",\n")
        modified.write("Architecture," + str(arch) + ",\n")
        modified.write("Model," + str(cpuname) + ",\n")
        modified.write("kernel version," + perf_helpers.get_version() + "\n")
        for _socket, _cpus in cpuid_info.items():
            modified.write("Socket:" + str(_socket) + ",")
            for c in _cpus:
                modified.write(str(c) + ";")
            modified.write("\n")
        modified.write("Perf event mux Interval ms," + str(muxinterval) + ",\n")
        threadmode = "enabled" if thread else "disabled"
        socketmode = "enabled" if socket else "disabled"
        if args.cid is not None:
            cgname = "enabled,"
            for cgroup in cgroups:
                cgname += cgroup + "=" + cgroup.replace("/", "-") + ","
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
                for path in cgroup_paths:
                    try:
                        with open(path, "r") as cpu_set_file:
                            cg_path_found = True
                            cpu_set = cpu_set_file.read()
                            cpu_set = cpu_set.strip()
                            cpu_set = cpu_set.replace(",", "+")
                            break
                    except FileNotFoundError:
                        continue

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
        modified.write("PSI," + json.dumps(psi) + "\n")
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


def get_psi():
    psi = []
    for resource in ["cpu", "memory", "io"]:
        with open("/proc/pressure/" + resource) as f:
            psi.append(f.readline().split()[4].split("=")[1])
    return psi


def supports_psi():
    psi = []
    for resource in ["cpu", "memory", "io"]:
        try:
            with open("/proc/pressure/" + resource) as _:
                psi.append(resource)
        except Exception:
            pass
    if len(psi) == 3:
        logging.info("PSI metrics supported")
        return True
    else:
        logging.info("PSI metrics not supported")
        return False


def resource_path(relative_path):
    """Get absolute path to resource, works for dev and for PyInstaller"""
    base_path = getattr(sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__)))
    return os.path.join(base_path, relative_path)


def validate_perfargs(perf):
    """validate perf command before executing"""
    if perf[0] != "perf":
        crash("Not a perf command, exiting!")


def validate_file(fname):
    """validate if file is accessible"""
    if not os.access(fname, os.R_OK):
        crash(str(fname) + " not accessible")


if __name__ == "__main__":
    common.configure_logging(".")
    if platform.system() != "Linux":
        crash("PerfSpect currently supports Linux only")

    # fix the pyinstaller path
    script_path = os.path.dirname(os.path.realpath(__file__))
    if "_MEI" in script_path:
        script_path = script_path.rsplit("/", 1)[0]
    default_output_file = "perfstat.csv"

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
        help="perf-collect on up to 5 cgroups. Provide comma separated cids like e19f4fb59,6edca29db (by default, selects the 5 containers using the most CPU)",
        type=str,
        nargs="?",
        const="",
    )
    runmode.add_argument(
        "--thread", help="Collect for thread metrics", action="store_true"
    )
    runmode.add_argument(
        "--socket", help="Collect for socket metrics", action="store_true"
    )
    parser.add_argument(
        "-m",
        "--muxinterval",
        type=int,
        default=125,
        help="event mux interval in milli seconds, default=125. Lower numbers can cause higher overhead",
    )
    parser.add_argument(
        "-o",
        "--outcsv",
        type=str,
        default=default_output_file,
        help="perf stat output in csv format, default=perfstat.csv",
    )
    parser.add_argument(
        "-v", "--verbose", help="Display debugging information", action="store_true"
    )
    parser.add_argument(
        "-V", "--version", help="display version info", action="store_true"
    )
    args = parser.parse_args()

    if args.version:
        print(perf_helpers.get_tool_version())
        sys.exit()

    if os.geteuid() != 0:
        crash("Must run PerfSpect as root, please re-run")

    # disable nmi watchdog before collecting perf
    nmi_watchdog = perf_helpers.disable_nmi_watchdog()
    initial_pmus = perf_helpers.pmu_contention_detect()
    interval = 5000
    collect_psi = False

    if args.thread:
        logging.info("Run mode: thread")
        collect_psi = supports_psi()
    elif args.socket:
        logging.info("Run mode: socket")
        collect_psi = supports_psi()
    elif args.pid is not None:
        logging.info("Run mode: pid")
    elif args.cid is not None:
        logging.info("Run mode: cid")
    else:
        logging.info("Run mode: system")
        collect_psi = supports_psi()

    if args.muxinterval > 1000:
        crash("Input argument muxinterval is too large, max is [1s or 1000ms]")

    # select architecture default event file if not supplied
    have_uncore = True
    procinfo = perf_helpers.get_cpuinfo()
    arch, cpuname = perf_helpers.get_arch_and_name(procinfo)
    if not arch:
        crash(
            f"Unrecognized CPU architecture. Supported architectures: {', '.join(SUPPORTED_ARCHITECTURES)}"
        )
    eventfile = None
    if arch == "broadwell":
        eventfile = "bdx.txt"
    elif arch == "skylake" or arch == "cascadelake":
        eventfile = "clx_skx.txt"
    elif arch == "icelake":
        eventfile = "icx.txt"
    elif arch == "sapphirerapids":
        eventfile = "spr.txt"
    elif arch == "emeraldrapids":
        eventfile = "spr.txt"
        have_uncore = False

    if eventfile is None:
        crash(f"failed to match architecture ({arch}) to event file name.")

    # Convert path of event file to relative path if being packaged by pyInstaller into a binary
    if getattr(sys, "frozen", False):
        basepath = getattr(sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__)))
        eventfilename = eventfile
        eventfile = os.path.join(basepath, eventfile)
    elif __file__:
        eventfile = script_path + "/events/" + eventfile
        eventfilename = eventfile
    else:
        crash("Unknown application type")

    # get perf events to collect
    sys_devs = perf_helpers.get_sys_devices()
    if (
        "uncore_cha" not in sys_devs
        and "uncore_cbox" not in sys_devs
        and "uncore_upi" not in sys_devs
        and "uncore_qpi" not in sys_devs
        and "uncore_imc" not in sys_devs
    ):
        logging.info("disabling uncore (possibly in a vm?)")
        have_uncore = False
        if arch == "icelake":
            logging.warning(
                "Due to lack of vPMU support, TMA L1 events will not be collected"
            )
        if arch == "sapphirerapids" or arch == "emeraldrapids":
            logging.warning(
                "Due to lack of vPMU support, TMA L1 & L2 events will not be collected"
            )
    events, collection_events = prep_events.prepare_perf_events(
        eventfile,
        (
            args.pid is not None
            or args.cid is not None
            or args.thread
            or args.socket
            or not have_uncore
        ),
        args.pid is not None or args.cid is not None,
    )

    if not perf_helpers.validate_outfile(args.outcsv):
        crash(
            "Output filename not accepted. Filename should be a .csv without special characters"
        )

    mux_intervals = perf_helpers.get_perf_event_mux_interval()
    if args.muxinterval > 0:
        perf_helpers.set_perf_event_mux_interval(False, args.muxinterval, mux_intervals)

    # parse cgroups
    cgroups = []
    if args.cid is not None:
        cgroups = perf_helpers.get_cgroups(args.cid)

    if args.thread or args.socket or args.pid is not None or args.cid is not None:
        logging.info("Not collecting uncore events in this run mode")

    # log some metadata
    logging.info("Architecture: " + arch)
    logging.info("Model: " + cpuname)
    logging.info("Kernel version: " + perf_helpers.get_version())
    logging.info("Cores per socket: " + str(perf_helpers.get_cpu_count()))
    logging.info("Socket: " + str(perf_helpers.get_socket_count()))
    logging.info("Hyperthreading on: " + str(perf_helpers.get_ht_status()))
    imc, upi = perf_helpers.get_imc_upi_count()
    logging.info("IMC count: " + str(imc))
    logging.info("CHA per socket: " + str(perf_helpers.get_cha_count()))
    logging.info("UPI count: " + str(upi))
    logging.info("PerfSpect version: " + perf_helpers.get_tool_version())
    logging.info("/sys/devices/: " + str(sys_devs))

    # build perf stat command
    collection_type = "-a" if not args.thread and not args.socket else "-a -A"
    cmd = f"perf stat -I {interval} -x , {collection_type} -o {args.outcsv}"
    if args.pid:
        cmd += f" --pid {args.pid}"

    if args.cid is not None:
        perf_format = prep_events.get_cgroup_events_format(
            cgroups, events, len(collection_events)
        )
        cmd += f" {perf_format}"
    else:
        cmd += f" -e {events}"

    if args.timeout:
        cmd += f" sleep {args.timeout}"
    elif args.app:
        cmd += f" {args.app}"

    perfargs = shlex.split(cmd)
    validate_perfargs(perfargs)
    perf_helpers.pmu_contention_detect(msrs=initial_pmus, detect=True)
    if args.verbose:
        logging.info(cmd)
    try:
        psi = []
        start = time.time()
        perf = subprocess.Popen(perfargs)  # nosec
        while perf.poll() is None:
            if collect_psi:
                psi.append(get_psi())
            time.sleep(interval / 1000)
        end = time.time()
        if end - start < 7:
            logging.warning(
                "PerfSpect was run for a short duration, some events might be zero or blank because they never got scheduled"
            )

    except subprocess.SubprocessError as e:
        perf.kill()  # type: ignore
        crash("Failed to start perf\n" + str(e))
    except KeyboardInterrupt:
        perf.kill()  # type: ignore
    except Exception as e:
        perf.kill()  # type: ignore
        crash(str(e) + "\nperf encountered errors")

    logging.info("Collection complete!")

    cpuid_info = perf_helpers.get_cpuid_info(procinfo)
    if collect_psi:
        psi.append(get_psi())
    write_metadata(
        args.outcsv,
        collection_events,
        arch,
        cpuname,
        cpuid_info,
        args.muxinterval,
        args.thread,
        args.socket,
        list(map(list, zip(*psi))),
    )

    os.chmod(args.outcsv, 0o666)  # nosec

    # reset nmi_watchdog to what it was before running perfspect
    if nmi_watchdog != 0:
        perf_helpers.enable_nmi_watchdog()

    perf_helpers.set_perf_event_mux_interval(True, 1, mux_intervals)

    logging.info("perf stat dumped to %s" % args.outcsv)
