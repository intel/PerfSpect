#!/usr/bin/env python3

###########################################################################################################
# Copyright (C) 2021 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

from __future__ import print_function
import os
import sys
import subprocess  # nosec
import shlex  # nosec
from src import perf_helpers
from src import prepare_perf_events as prep_events


from subprocess import PIPE, run  # nosec


# meta data gathering
def write_metadata(
    outcsv,
    collection_events,
    arch,
    cpuname,
    cpuid_info,
    interval,
    muxinterval,
    nogroups,
    percore,
    supervisor,
    metadata_only=False,
):
    tsc_freq = str(perf_helpers.get_tsc_freq())
    data = ""
    time_stamp = ""
    validate_file(outcsv)
    with open(outcsv, "r") as original:
        time_stamp = original.readline()
        if metadata_only and time_stamp.startswith("### META DATA ###"):
            print("Not prepending metadata, already present in %s " % (outcsv))
            return
        data = original.read()
    with open(outcsv, "w") as modified:
        modified.write("### META DATA ###,\n")
        modified.write("TSC Frequency(MHz)," + tsc_freq + ",\n")
        modified.write("CPU count," + str(perf_helpers.get_cpu_count()) + ",\n")
        modified.write("SOCKET count," + str(perf_helpers.get_socket_count()) + ",\n")
        if args.pid or args.cgroup:
            modified.write("HT count," + str(1) + ",\n")
        else:
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
        grouping = "disabled" if nogroups else "enabled"
        supervisor = "sudo" if supervisor else "non root"
        percoremode = "enabled" if percore else "disabled"
        modified.write("Event grouping," + grouping + ",\n")
        modified.write("User mode," + supervisor + ",\n")
        modified.write("Percore mode," + percoremode + ",\n")
        modified.write("PerfSpect version," + perf_helpers.get_tool_version() + ",\n")
        modified.write("### PERF EVENTS ###" + ",\n")
        for e in collection_events:
            modified.write(e + "\n")
        modified.write("\n")
        modified.write("### PERF DATA ###" + ",\n")
        if time_stamp:
            zone = subprocess.check_output(  # nosec
                ["date"], universal_newlines=True  # nosec
            ).split()  # nosec
            epoch = str(perf_helpers.get_epoch(time_stamp))
            modified.write(
                time_stamp.rstrip() + " " + zone[4] + " EPOCH " + epoch + "\n"
            )
        modified.write(data)


def resource_path(relative_path):
    """Get absolute path to resource, works for dev and for PyInstaller"""
    base_path = getattr(sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__)))
    return os.path.join(base_path, relative_path)


def validate_perfargs(perf):
    """validate perf command before executing"""
    if perf[0] != "perf":
        raise SystemExit("Not a perf command, exiting!")


def validate_file(fname):
    """validate if file is accessible"""
    if not os.access(fname, os.R_OK):
        raise SystemExit(str(fname) + " not accessible")


def is_safe_file(fname, substr):
    """verify if file name/format is accurate"""
    if not fname.endswith(substr):
        raise SystemExit(str(fname) + " isn't appropriate format")


if __name__ == "__main__":

    script_path = os.path.dirname(os.path.realpath(__file__))
    # fix the pyinstaller path
    if "_MEI" in script_path:
        script_path = script_path.rsplit("/", 1)[0]
    result_dir = script_path + "/results"
    default_output_file = result_dir + "/perfstat.csv"
    from argparse import ArgumentParser

    parser = ArgumentParser(description="perf-collect: Time series dump of PMUs")
    parser.add_argument(
        "-v", "--version", help="display version info", action="store_true"
    )
    parser.add_argument(
        "-e",
        "--eventfile",
        type=str,
        default=None,
        help="Event file containing events to collect, default=events/<architecture specific file>",
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
        "-a",
        "--app",
        type=str,
        default=None,
        help="Application to run with perf-collect, perf collection ends after workload completion",
    )
    parser.add_argument(
        "-p", "--pid", type=str, default=None, help="perf-collect on selected PID(s)"
    )
    parser.add_argument(
        "-c",
        "--cgroup",
        type=str,
        default=None,
        help="perf-collect on selected cgroup(s)",
    )
    parser.add_argument(
        "-t", "--timeout", type=int, default=None, help="perf event collection time"
    )
    parser.add_argument(
        "--percore", help="Enable per core event collection", action="store_true"
    )
    parser.add_argument(
        "--nogroups",
        help="Disable perf event grouping, events are grouped by default as in the event file",
        action="store_true",
    )
    parser.add_argument(
        "--dryrun",
        help="Test if Performance Monitoring Counters are in-use, and collect stats for 10sec to validate event file correctness",
        action="store_true",
    )
    parser.add_argument(
        "--metadata",
        help="collect system info only, does not run perf",
        action="store_true",
    )
    parser.add_argument(
        "-csp",
        "--cloud",
        type=str,
        default="none",
        help="Name of the Cloud Service Provider(AWS), if collecting on cloud instances",
    )
    parser.add_argument(
        "-ct",
        "--cloudtype",
        type=str,
        default="VM",
        help="Instance type: Options include - VM,BM",
    )

    args = parser.parse_args()

    if args.version:
        print(perf_helpers.get_tool_version())
        sys.exit(0)

    interval = int(args.interval * 1000)

    if args.app and args.timeout:
        raise SystemExit("Please provide time duration or application parameter")

    if args.muxinterval > 1000:
        raise SystemExit(
            "Input argument muxinterval is too large, max is [1s or 1000ms]"
        )
    if args.interval < 0.1 or args.interval > 300:
        raise SystemExit(
            "Input argument dump interval is too large or too small, range is [0.1 to 300s]!"
        )

    # select architecture default event file if not supplied
    procinfo = perf_helpers.get_cpuinfo()
    arch, cpuname = perf_helpers.check_architecture(procinfo)
    eventfile = args.eventfile
    eventfilename = eventfile

    if not eventfile:
        if arch == "broadwell":
            eventfile = "bdx.txt"
        elif arch == "skylake":
            eventfile = "skx.txt"
            if args.cloud in ("aws", "AWS") and args.cloudtype in ("VM", "vm"):
                eventfile = "skx_aws.txt"
        elif arch == "cascadelake":
            eventfile = "clx.txt"
            if args.cloud in ("aws", "AWS") and args.cloudtype in ("VM", "vm"):
                eventfile = "clx_aws.txt"
        elif arch == "icelake":
            eventfile = "icx.txt"
        else:
            raise SystemExit(
                "Unsupported architecture (currently supports Broadwell, Skylake, CascadeLake and Icelake Intel Xeon processors)"
            )

        # Convert path of event file to relative path if being packaged by pyInstaller into a binary
        if getattr(sys, "frozen", False):
            basepath = getattr(
                sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__))
            )
            eventfilename = eventfile
            is_safe_file(eventfile, ".txt")
            eventfile = os.path.join(basepath, eventfile)
        elif __file__:
            eventfile = script_path + "/events/" + eventfile
            eventfilename = eventfile
        else:
            raise SystemExit("Unknow application type")

    if not os.path.isfile(eventfile):
        raise SystemExit("event file not found")

    if args.outcsv == default_output_file:
        # create results dir
        if not os.path.exists(result_dir):
            os.mkdir(result_dir)
    else:
        if not perf_helpers.validate_outfile(args.outcsv):
            raise SystemExit(
                "Output filename not accepted. Filename should be a .csv without special characters"
            )

    supervisor = False
    if os.geteuid() == 0:
        supervisor = True

    mux_intervals = perf_helpers.get_perf_event_mux_interval()
    if args.muxinterval > 0:
        if supervisor:
            perf_helpers.set_perf_event_mux_interval(
                False, args.muxinterval, mux_intervals
            )
        else:
            print(
                "Warning: perf event mux interval can't be set without sudo permission"
            )

    # disable nmi watchdog before collecting perf
    f_nmi = open("/proc/sys/kernel/nmi_watchdog", "r")
    nmi_watchdog = f_nmi.read()
    f_nmi.close()

    if int(nmi_watchdog) != 0:
        if supervisor:
            f_nmi = open("/proc/sys/kernel/nmi_watchdog", "w")
            f_nmi.write("0")
            f_nmi.close()
        else:
            print("Warning: nmi_watchdog enabled, perf grouping will be disabled")
            args.nogroups = True

    # disable grouping if more than 1 cgroups are being monitored
    if args.cgroup is not None:
        num_cgroups = prep_events.get_num_cgroups(args.cgroup)
        if num_cgroups > 1:
            args.nogroups = True

    try:
        import re

        reg = r"^[0-9]*\.[0-9][0-9]*"
        kernel = perf_helpers.get_version().split("Linux version")[1].lstrip()
        significant_kernel_version = float(re.match(reg, kernel).group(0))
        full_kernel_version = kernel

    except Exception as e:
        print(e)
        raise SystemExit("Unable to get kernel version")

    # Fix events not compatible with older kernel versions only
    if significant_kernel_version == 3.10 and arch != "broadwell":
        kernel_version = full_kernel_version.split(" ")[0]
        prep_events.fix_events_for_older_kernels(eventfile, kernel_version)

    # get perf events to collect
    collection_events = []
    events, collection_events = prep_events.prepare_perf_events(
        eventfile, (args.nogroups is False), ((args.pid or args.cgroup) is not None)
    )

    if args.metadata:
        cpuid_info = perf_helpers.get_cpuid_info(procinfo)
        write_metadata(
            args.outcsv,
            collection_events,
            arch,
            cpuname,
            cpuid_info,
            args.interval,
            args.muxinterval,
            args.nogroups,
            args.percore,
            supervisor,
            True,
        )
        sys.exit("Output with metadata in  %s" % args.outcsv)

    collection_type = "-a" if args.percore is False else "-a -A"
    # start perf stat
    if args.pid and args.timeout:
        print("Info: Only CPU/core events will be enabled with pid option")
        cmd = "perf stat -I %d -x , --pid %s -e %s -o %s sleep %d" % (
            interval,
            args.pid,
            events,
            args.outcsv,
            args.timeout,
        )

    elif args.pid:
        print("Info: Only CPU/core events will be enabled with pid option")
        cmd = "perf stat -I %d -x , --pid %s -e %s -o %s" % (
            interval,
            args.pid,
            events,
            args.outcsv,
        )

    elif args.cgroup and args.timeout:
        print("Info: Only CPU/core events will be enabled with cgroup option")
        if num_cgroups == 1:
            cmd = "perf stat -I %d -x , -e %s -G %s -a -o %s sleep %d" % (
                interval,
                events,
                args.cgroup,
                args.outcsv,
                args.timeout,
            )
        else:
            perf_format = prep_events.get_cgroup_events_format(args.cgroup, events)
            cmd = "perf stat -I %d -x , %s -o %s sleep %d" % (
                interval,
                perf_format,
                args.outcsv,
                args.timeout,
            )

    elif args.cgroup:
        print("Info: Only CPU/core events will be enabled with cgroup option")
        if num_cgroups == 1:
            cmd = "perf stat -I %d -x , -e %s -G %s -o %s" % (
                interval,
                events,
                args.cgroup,
                args.outcsv,
            )
        else:
            perf_format = prep_events.get_cgroup_events_format(args.cgroup, events)
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
    elif args.dryrun:
        with open("results/pmu-checker.log", "w") as fw:
            print("Checking if PMU counters are in-use already...")
            pmuargs = resource_path("pmu-checker")
            try:
                run_result = run(  # nosec
                    shlex.split(pmuargs),
                    stdout=PIPE,
                    stderr=PIPE,
                    universal_newlines=True,
                )
                fw.write(str(run_result.stdout))

            except Exception as e:
                print(e)

        cmd = "perf stat %s -I %d -x , -e %s -o %s sleep 10" % (
            collection_type,
            interval,
            events,
            args.outcsv,
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
    try:
        print("Collecting perf stat for events in : %s" % eventfilename)
        if args.cloud != "none":
            print(
                "Consider using cloudtype flag to set instance type -> VM/BM; Default is VM"
            )
        subprocess.call(perfargs)  # nosec
        print("Collection complete! Calculating TSC frequency now")
    except KeyboardInterrupt:
        print("Collection stopped! Caculating TSC frequency now")
    except Exception:
        print("perf encountered errors")

    cpuid_info = perf_helpers.get_cpuid_info(procinfo)
    write_metadata(
        args.outcsv,
        collection_events,
        arch,
        cpuname,
        cpuid_info,
        args.interval,
        args.muxinterval,
        args.nogroups,
        args.percore,
        supervisor,
        False,
    )

    if (int(nmi_watchdog) != 0) and supervisor:
        f_nmi = open("/proc/sys/kernel/nmi_watchdog", "w")
        f_nmi.write(nmi_watchdog)

    if (args.muxinterval > 0) and supervisor:
        perf_helpers.set_perf_event_mux_interval(True, 1, mux_intervals)

    print("perf stat dumped to %s" % args.outcsv)
