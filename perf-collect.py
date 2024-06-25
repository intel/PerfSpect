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
    pmu_driver_version,
    fixed_tma_supported,
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
        modified.write("PMUDriverVersion," + str(pmu_driver_version) + ",\n")
        modified.write("FixedTMASupported," + str(fixed_tma_supported) + ",\n")
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


# fixed_tma_supported returns true if the fixed-purpose PMU counters for TMA events are supported on the target platform
def fixed_tma_supported():
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
    except (IndexError, ValueError):
        logging.debug("Failed to parse perf output in fixed_tma_supported()")
        return False
    try:
        if events["TOPDOWN.SLOTS"] == events["PERF_METRICS.BAD_SPECULATION"]:
            return False
    except KeyError:
        logging.debug("Failed to find required events in fixed_tma_supported()")
        return False

    if events["TOPDOWN.SLOTS"] == 0 or events["PERF_METRICS.BAD_SPECULATION"] == 0:
        return False

    return True


# fixed_event_supported returns true if the fixed-purpose PMU counter for the given event (cpu-cycles or instructions) event is supported on the target platform
# it makes this determination by filling all the general purpose counters with the given events, then adding one more
def fixed_event_supported(arch, event):
    num_gp_counters = 0
    if arch == "broadwell" or arch == "skylake" or arch == "cascadelake":
        num_gp_counters = 4
    elif (
        arch == "icelake"
        or arch == "sapphirerapids"
        or arch == "emeraldrapids"
        or arch == "sierraforest"
    ):
        num_gp_counters = 8
    else:
        crash(f"Unsupported architecture: {arch}")

    perf_out = ""
    events = ",".join([event] * (num_gp_counters + 1))
    try:
        perf = subprocess.Popen(
            shlex.split("perf stat -a -e '{" + events + "}' sleep .1"),
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        perf_out = perf.communicate()[0].decode()
    except subprocess.CalledProcessError:
        return False
    # on some VMs we see "<not counted>" or "<not supported>" in the perf output
    if "<not counted>" in perf_out or "<not supported>" in perf_out:
        return False
    # on some VMs we get a count of 0
    for line in perf_out.split("\n"):
        tokens = line.split()
        if len(tokens) == 2 and tokens[0] == "0":
            return False
    return True


def fixed_cycles_supported(arch):
    return fixed_event_supported(arch, "cpu-cycles")


def fixed_instructions_supported(arch):
    return fixed_event_supported(arch, "instructions")


def ref_cycles_supported():
    perf_out = ""
    try:
        perf = subprocess.Popen(
            shlex.split("perf stat -a -e ref-cycles sleep .1"),
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        perf_out = perf.communicate()[0].decode()
    except subprocess.CalledProcessError:
        return False

    if "<not supported>" in perf_out:
        logging.warning(
            "ref-cycles not enabled in VM driver. Contact system owner to enable. Collecting reduced metrics"
        )
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


def get_eventfile_path(arch, script_path, supports_tma_fixed_events):
    eventfile = None
    if arch == "broadwell":
        eventfile = "bdx.txt"
    elif arch == "skylake" or arch == "cascadelake":
        eventfile = "clx_skx.txt"
    elif arch == "icelake":
        if supports_tma_fixed_events:
            eventfile = "icx.txt"
        else:
            eventfile = "icx_nofixedtma.txt"
    elif arch == "sapphirerapids" or arch == "emeraldrapids":
        if supports_tma_fixed_events:
            eventfile = "spr_emr.txt"
        else:
            eventfile = "spr_emr_nofixedtma.txt"
    elif arch == "sierraforest":
        eventfile = "srf.txt"

    if eventfile is None:
        return None

    # Convert path of event file to relative path if being packaged by pyInstaller into a binary
    if getattr(sys, "frozen", False):
        basepath = getattr(sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__)))
        return os.path.join(basepath, eventfile)
    elif __file__:
        return script_path + "/events/" + eventfile
    else:
        crash("Unknown application type")


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
    parser.add_argument(
        "-e", "--eventfile", default=None, help="Relative path to eventfile"
    )
    args = parser.parse_args()

    if args.version:
        print(perf_helpers.get_tool_version())
        sys.exit()

    is_root = os.geteuid() == 0
    if not is_root:
        logging.warning(
            "User is not root. See README.md for requirements and instructions on how to run as non-root user."
        )
        try:
            input("Press Enter to continue as non-root user or Ctrl-c to exit...")
        except KeyboardInterrupt:
            print("\nExiting...")
            sys.exit()

    if not is_root:
        # check kernel.perf_event_paranoid. It needs to be zero for non-root users.
        paranoid = perf_helpers.check_perf_event_paranoid()
        if paranoid is None:
            crash("kernel.perf_event_paranoid could not be determined")
        if paranoid != 0:
            crash(
                "kernel.perf_event_paranoid is set to "
                + str(paranoid)
                + ". Run as root or set it to 0"
            )

    # disable nmi watchdog before collecting perf
    nmi_watchdog_status = perf_helpers.nmi_watchdog_enabled()
    if nmi_watchdog_status is None:
        crash("NMI watchdog status could not be determined")

    if is_root and nmi_watchdog_status:
        perf_helpers.disable_nmi_watchdog()

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

    # check if pmu available
    if "cpu-cycles" not in perf_helpers.get_perf_list():
        crash(
            "PMU's not available. Run baremetal or in a VM which exposes PMUs (sometimes full socket)"
        )

    procinfo = perf_helpers.get_cpuinfo()
    arch, cpuname = perf_helpers.get_arch_and_name(procinfo)
    if not arch:
        crash(
            f"Unrecognized CPU architecture. Supported architectures: {', '.join(SUPPORTED_ARCHITECTURES)}"
        )

    # Can we use the fixed purpose PMU counters for TMA events?
    # The fixed-purpose PMU counters for TMA events are not supported on architectures older than Icelake
    # They are also not supported on some VMs, e.g., AWS ICX and SPR VMs
    supports_tma_fixed_events = False
    if arch == "icelake" or arch == "sapphirerapids" or arch == "emeraldrapids":
        supports_tma_fixed_events = fixed_tma_supported()
        if not supports_tma_fixed_events:
            logging.warning(
                "Due to lack of vPMU support, some TMA events will not be collected"
            )

    # Can we use the fixed-purpose PMU counter for the cpu-cycles event?
    supports_cycles_fixed_event = fixed_cycles_supported(arch)

    # Can we use the fixed-purpose PMU counter for the instructions event?
    supports_instructions_fixed_event = fixed_instructions_supported(arch)

    # select architecture default event file if not supplied
    if args.eventfile is not None:
        eventfile = args.eventfile
    else:
        eventfile = get_eventfile_path(arch, script_path, supports_tma_fixed_events)
    if eventfile is None:
        crash(f"failed to match architecture ({arch}) to event file name.")

    logging.info("Event file: " + eventfile)

    supports_uncore_events = True
    sys_devs = perf_helpers.get_sys_devices()
    if (
        "uncore_cha" not in sys_devs
        and "uncore_cbox" not in sys_devs
        and "uncore_upi" not in sys_devs
        and "uncore_qpi" not in sys_devs
        and "uncore_imc" not in sys_devs
    ):
        logging.info("uncore devices not found (possibly in a vm?)")
        supports_uncore_events = False

    supports_ref_cycles_event = ref_cycles_supported()

    events, collection_events = prep_events.prepare_perf_events(
        eventfile,
        (args.pid is not None or args.cid is not None or not supports_uncore_events),
        supports_tma_fixed_events,
        supports_uncore_events,
        supports_ref_cycles_event,
    )

    # check output file is writable
    if not perf_helpers.check_file_writeable(args.outcsv):
        crash("Output file %s not writeable " % args.outcsv)

    # adjust mux interval
    mux_intervals = perf_helpers.get_perf_event_mux_interval()
    if args.muxinterval > 0:
        if is_root:
            logging.info(
                "changing perf mux interval to " + str(args.muxinterval) + "ms"
            )
            perf_helpers.set_perf_event_mux_interval(
                False, args.muxinterval, mux_intervals
            )
        else:
            for device, mux in mux_intervals.items():
                mux_int = -1
                try:
                    mux_int = int(mux)
                except ValueError:
                    crash("Failed to parse mux interval on " + device)
                if mux_int != args.muxinterval:
                    crash(
                        "mux interval on "
                        + device
                        + " is set to "
                        + str(mux_int)
                        + ". Run as root or set it to "
                        + str(args.muxinterval)
                        + "."
                    )

    # parse cgroups
    cgroups = []
    if args.cid is not None:
        cgroups = perf_helpers.get_cgroups(args.cid)

    if args.pid is not None or args.cid is not None:
        logging.info("Not collecting uncore events in this run mode")

    pmu_driver_version = perf_helpers.get_pmu_driver_version()

    # log some metadata
    logging.info("Architecture: " + arch)
    logging.info("Model: " + cpuname)
    logging.info("Kernel version: " + perf_helpers.get_version())
    logging.info("PMU driver version: " + pmu_driver_version)
    logging.info("Uncore events supported: " + str(supports_uncore_events))
    logging.info(
        "Fixed counter TMA events supported: " + str(supports_tma_fixed_events)
    )
    logging.info(
        "Fixed counter cpu-cycles event supported: " + str(supports_cycles_fixed_event)
    )
    logging.info(
        "Fixed counter instructions event supported: "
        + str(supports_instructions_fixed_event)
    )
    logging.info("ref-cycles event supported: " + str(supports_ref_cycles_event))
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
        pmu_driver_version,
        supports_tma_fixed_events,
        args.muxinterval,
        args.cpu,
        args.socket,
        list(map(list, zip(*psi))),
    )

    os.chmod(args.outcsv, 0o666)  # nosec

    # reset nmi_watchdog to what it was before running perfspect
    if is_root and nmi_watchdog_status is True:
        perf_helpers.enable_nmi_watchdog()

    if is_root:
        logging.info("changing perf mux interval back to default")
        perf_helpers.set_perf_event_mux_interval(True, 1, mux_intervals)

    logging.info("perf stat dumped to %s" % args.outcsv)
