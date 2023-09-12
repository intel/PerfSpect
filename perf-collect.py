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
    "SierraForest",
]


# meta data gathering
def write_metadata(
    outcsv,
    collection_events,
    arch,
    cpuname,
    cpuid_info,
    muxinterval,
    cpu,
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
        imc, cha, upi = perf_helpers.get_imc_cha_upi_count()
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
        cpumode = "enabled" if cpu else "disabled"
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
        modified.write("Percpu mode," + cpumode + ",\n")
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


def tma_supported():
    perf_out = ""
    try:
        perf = subprocess.Popen(
            shlex.split(
                "perf stat -a -e '{cpu/event=0x00,umask=0x04,period=10000003,name='TOPDOWN.SLOTS'/,cpu/event=0x00,umask=0x81,period=10000003,name='PERF_METRICS.BAD_SPECULATION'/}' sleep .1"
            ),
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        perf_out = perf.communicate()[0].decode()
    except subprocess.CalledProcessError:
        return False

    try:
        events = {
            a.split()[1]: int(a.split()[0].replace(",", ""))
            for a in filter(
                lambda x: "TOPDOWN.SLOTS" in x or "PERF_METRICS.BAD_SPECULATION" in x,
                perf_out.split("\n"),
            )
        }
    except Exception:
        return False

    # This is a perf artifact of no vPMU support
    if events["TOPDOWN.SLOTS"] == events["PERF_METRICS.BAD_SPECULATION"]:
        return False

    return True


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
        "--cid",
        help="perf-collect on up to 5 cgroups. Provide comma separated cids like e19f4fb59,6edca29db (by default, selects the 5 containers using the most CPU)",
        type=str,
        nargs="?",
        const="",
    )
    runmode.add_argument(
        "-c", "--cpu", help="Collect for cpu metrics", action="store_true"
    )
    runmode.add_argument(
        "-s", "--socket", help="Collect for socket metrics", action="store_true"
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
    interval = 5000
    collect_psi = False

    if args.cpu:
        logging.info("Run mode: cpu")
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
    elif arch == "sierraforest":
        eventfile = "srf.txt"

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

    # check if pmu available
    if "cpu-cycles" not in perf_helpers.get_perf_list():
        crash(
            "PMU's not available. Run baremetal or in a VM which exposes PMUs (sometimes full socket)"
        )

    # get perf events to collect
    include_tma = True
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
        include_tma = tma_supported()
        if not include_tma:
            logging.warning(
                "Due to lack of vPMU support, TMA L1 events will not be collected"
            )
    if arch == "sapphirerapids" or arch == "emeraldrapids":
        include_tma = tma_supported()
        if not include_tma:
            logging.warning(
                "Due to lack of vPMU support, TMA L1 & L2 events will not be collected"
            )
    events, collection_events = prep_events.prepare_perf_events(
        eventfile,
        (args.pid is not None or args.cid is not None or not have_uncore),
        include_tma,
        not have_uncore,
    )

    if not perf_helpers.validate_outfile(args.outcsv):
        crash(
            "Output filename not accepted. Filename should be a .csv without special characters"
        )

    mux_intervals = perf_helpers.get_perf_event_mux_interval()
    if args.muxinterval > 0:
        logging.info(
            "changing default perf mux interval to " + str(args.muxinterval) + "ms"
        )
        perf_helpers.set_perf_event_mux_interval(False, args.muxinterval, mux_intervals)

    # parse cgroups
    cgroups = []
    if args.cid is not None:
        cgroups = perf_helpers.get_cgroups(args.cid)

    if args.pid is not None or args.cid is not None:
        logging.info("Not collecting uncore events in this run mode")

    # log some metadata
    logging.info("Architecture: " + arch)
    logging.info("Model: " + cpuname)
    logging.info("Kernel version: " + perf_helpers.get_version())
    logging.info("Cores per socket: " + str(perf_helpers.get_cpu_count()))
    logging.info("Socket: " + str(perf_helpers.get_socket_count()))
    logging.info("Hyperthreading on: " + str(perf_helpers.get_ht_status()))
    imc, cha, upi = perf_helpers.get_imc_cha_upi_count()
    logging.info("IMC count: " + str(imc))
    logging.info("CHA per socket: " + str(cha))
    logging.info("UPI count: " + str(upi))
    logging.info("PerfSpect version: " + perf_helpers.get_tool_version())
    if args.verbose:
        logging.info("/sys/devices/: " + str(sys_devs))

    # build perf stat command
    collection_type = "-a" if not args.cpu and not args.socket else "-a -A"
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

    if args.verbose:
        logging.info(cmd)
    perfargs = shlex.split(cmd)
    validate_perfargs(perfargs)

    psi = []
    logging.info("Collection started!")
    start = time.time()
    try:
        perf = subprocess.Popen(perfargs)  # nosec
        try:
            while perf.poll() is None:
                if collect_psi:
                    psi.append(get_psi())
                time.sleep(interval / 1000)
        except KeyboardInterrupt:
            perf.kill()
    except KeyboardInterrupt:
        logging.info("Perfspect was interrupted by the user.")
    except subprocess.SubprocessError as e:
        crash("Failed to start perf\n" + str(e))
    end = time.time()
    if end - start < 7:
        logging.warning(
            "PerfSpect was run for a short duration, some events might be zero or blank because they never got scheduled"
        )
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
        args.cpu,
        args.socket,
        list(map(list, zip(*psi))),
    )

    os.chmod(args.outcsv, 0o666)  # nosec

    # reset nmi_watchdog to what it was before running perfspect
    if nmi_watchdog != 0:
        perf_helpers.enable_nmi_watchdog()

    logging.info("changing perf mux interval back to default")
    perf_helpers.set_perf_event_mux_interval(True, 1, mux_intervals)

    logging.info("perf stat dumped to %s" % args.outcsv)
