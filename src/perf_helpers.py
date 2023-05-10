#!/usr/bin/env python3

###########################################################################################################
# Copyright (C) 2020-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

import collections
import fnmatch
import logging
import math
import os
import re
import struct
import subprocess  # nosec
import time
from ctypes import cdll, CDLL
from datetime import datetime
from dateutil import tz
from src.common import crash
from time import strptime


version = "PerfSpect_DEV_VERSION"


# get tool version info
def get_tool_version():
    return str(version)


# extract number of sockets
def get_socket_count():
    cpuinfo = get_cpuinfo()
    return int(cpuinfo[-1]["physical id"]) + 1


# extract number of hyperthreads
def get_ht_count():
    cpuinfo = get_cpuinfo()
    return int(any([core["siblings"] != core["cpu cores"] for core in cpuinfo])) + 1


# get hyperthreading status
def get_ht_status():
    cpuinfo = get_cpuinfo()
    return any([core["siblings"] != core["cpu cores"] for core in cpuinfo])


# get cpu count
def get_cpu_count():
    cpu_count = 0
    if not os.path.isfile("/sys/devices/system/cpu/online"):
        crash("/sys/devices/system/cpu/online not found to get core count")
    with open("/sys/devices/system/cpu/online", "r") as f_online_cpu:
        content = f_online_cpu.read()
        cpu_list = content.split(",")
        for c in cpu_list:
            limit = c.split("-")
            cpu_count += int(limit[1]) - int(limit[0]) + 1
    return int(cpu_count / (get_socket_count() * get_ht_count()))


# compute tsc frequency
def get_tsc_freq():
    script_path = os.path.dirname(os.path.realpath(__file__))
    tsclib = script_path + "/libtsc.so"
    cdll.LoadLibrary(tsclib)
    tsc = CDLL(tsclib)
    tsc_freq = str(tsc.Calibrate())
    if tsc_freq == 0:
        crash("can't calculate TSC frequency")
    return tsc_freq


def get_dev_name(name):
    name = name.strip()
    parts = name.split("_")
    if parts[-1].isdigit():
        newlen = len(name) - len(parts[-1]) - 1
        name = name[:newlen]
    return name


# get sys/devices mapped files
def get_sys_devices():
    devs = {}
    for f in os.listdir("/sys/devices"):
        name = get_dev_name(f.strip())
        if name not in devs:
            devs[name] = 1
        else:
            devs[name] = devs[name] + 1
    return devs


# get imc and uncore counts
# TODO:fix for memory config with some channels populated
def get_imc_upi_count():
    sys_devs = get_sys_devices()
    imc_count = 0
    upi_count = 0
    if "uncore_upi" in sys_devs:
        upi_count = int(sys_devs["uncore_upi"])
    if "uncore_qpi" in sys_devs:
        upi_count = int(sys_devs["uncore_qpi"])
    if "uncore_imc" in sys_devs:
        imc_count = int(sys_devs["uncore_imc"])
    return imc_count, upi_count


# get CHA count
def get_cha_count():
    cha_msrs = {
        "0x396": "uncore client cha count",
        "0x702": "uncore cha count",
        "0x2FFE": "uncore cha count spr",
    }
    for msr in cha_msrs.keys():
        result = read_msr(int(msr, 16))
        if result is not None and result != 0:
            return result
    return 0


# get imc channel ids, channel ids are not consecutive in some cases (observed on bdw)
def get_channel_ids():
    sysdevices = os.listdir("/sys/bus/event_source/devices")
    imc = "uncore_imc_[0-9]*"
    ids = []
    for entry in sysdevices:
        if fnmatch.fnmatch(entry, imc):
            words = entry.split("_")
            ids.append(int(words[-1]))
    ids = sorted(ids)
    return ids


# get perf event mux interval for pmu events
def get_perf_event_mux_interval():
    mux_interval = {}
    for f in os.listdir("/sys/devices"):
        dirpath = os.path.join("/sys/devices/", f)
        if os.path.isdir(dirpath):
            muxfile = os.path.join(dirpath, "perf_event_mux_interval_ms")
            if os.path.isfile(muxfile):
                with open(muxfile, "r") as f_mux:
                    mux_interval[f] = f_mux.read()
    return mux_interval


# disable nmi watchdog and return its initial status
# to restore it after collection
def disable_nmi_watchdog():
    try:
        proc_output = subprocess.check_output(["cat", "/proc/sys/kernel/nmi_watchdog"])
        nmi_watchdog_status = int(proc_output.decode().strip())
        if nmi_watchdog_status == 1:
            proc_output = subprocess.check_output(["sysctl", "kernel.nmi_watchdog=0"])
            new_watchdog_status = int(
                proc_output.decode().strip().replace("kernel.nmi_watchdog = ", "")
            )
            if new_watchdog_status != 0:
                crash("Failed to disable nmi watchdog!")
            logging.info(
                "nmi_watchdog is temporary disabled. Will enable after collection."
            )
        else:
            logging.info("nmi_watchdog disabled!")
        return nmi_watchdog_status
    except (subprocess.CalledProcessError, ValueError) as e:
        crash(e.output + "\nFailed to disable nmi_watchdog.")


# enable nmi watchdog
def enable_nmi_watchdog():
    try:
        proc_output = subprocess.check_output(["sysctl", "kernel.nmi_watchdog=1"])
        new_watchdog_status = int(
            proc_output.decode().strip().replace("kernel.nmi_watchdog = ", "")
        )
        if new_watchdog_status != 1:
            logging.warning("Failed to re-enable nmi_watchdog!")
        else:
            logging.info("nmi_watchdog enabled!")
    except (subprocess.CalledProcessError, ValueError) as e:
        logging.warning(e.output + "\nFailed to re-enable nmi_watchdog!")


# set/reset perf event mux interval for pmu events
def set_perf_event_mux_interval(reset, interval_ms, mux_interval):
    for f in os.listdir("/sys/devices"):
        dirpath = os.path.join("/sys/devices/", f)
        if os.path.isdir(dirpath):
            muxfile = os.path.join(dirpath, "perf_event_mux_interval_ms")
            if os.path.isfile(muxfile):
                with open(muxfile, "w") as f_mux:
                    val = 0
                    if reset:
                        val = int(mux_interval[f])
                    else:
                        if int(mux_interval[f]):
                            val = int(interval_ms)
                    if val:
                        f_mux.write(str(val))


# read the MSR register and return the value in dec format
def read_msr(msr, cpu=0):
    fName = f"/dev/cpu/{cpu}/msr"
    try:
        with open(fName, "rb") as f:
            f.seek(msr)
            result = struct.unpack("Q", f.read(8))[0]
    except OSError:
        result = None
    return result


# detect if PMU counters are in use
def pmu_contention_detect(
    msrs={
        "0x309": {"name": "instructions", "value": None},
        "0x30a": {"name": "cpu cycles", "value": None},
        "0x30b": {"name": "ref cycles", "value": None},
        "0x30c": {"name": "topdown slots", "value": None},
        "0xc1": {"name": "general purpose PMU 1", "value": None},
        "0xc2": {"name": "general purpose PMU 2", "value": None},
        "0xc3": {"name": "general purpose PMU 3", "value": None},
        "0xc4": {"name": "general purpose PMU 4", "value": None},
        "0xc5": {"name": "general purpose PMU 5", "value": None},
        "0xc6": {"name": "general purpose PMU 6", "value": None},
        "0xc7": {"name": "general purpose PMU 7", "value": None},
        "0xc8": {"name": "general purpose PMU 8", "value": None},
    },
    detect=False,
):
    warn = False
    for r in msrs:
        try:
            value = read_msr(int(r, 16))
            if msrs[r]["value"] is not None and value != msrs[r]["value"]:
                logging.warning("PMU in use: " + msrs[r]["name"])
                warn = True
            msrs[r]["value"] = value
        except IOError:
            pass
    if detect:
        if warn:
            logging.warning("output could be inaccurate")
        else:
            logging.info("PMUs not in use")
    return msrs


# get linux kernel version
def get_version():
    version = ""
    try:
        with open("/proc/version", "r") as f:
            version = f.read()
            version = version.split("#")[0]
    except EnvironmentError as e:
        logging.warning(str(e), UserWarning)
    return version


# populate the CPU info list after reading /proc/cpuinfo in list of dictionaries
def get_cpuinfo():
    cpuinfo = []
    temp_dict = {}
    try:
        fo = open("/proc/cpuinfo", "r")
    except EnvironmentError as e:
        logging.warning(str(e), UserWarning)
    else:
        for line in fo:
            try:
                key, value = list(map(str.strip, line.split(":", 1)))
            except ValueError:
                cpuinfo.append(temp_dict)
                temp_dict = {}
            else:
                temp_dict[key] = value
        fo.close()
    return cpuinfo


def get_lscpu():
    cpuinfo = {}
    try:
        lscpu = subprocess.check_output(["lscpu"], universal_newlines=True)  # nosec
        # print(lscpu.split("\n"))
        lscpu = [i for i in lscpu.split("\n") if i]
        for prop in lscpu:
            key, value = prop.split(":")
            value = value.lstrip()
            cpuinfo[key] = value
    except subprocess.CalledProcessError as e:
        crash(e.output + "\nFailed to get CPUInfo")
    return cpuinfo


# get supported perf events
def get_perf_list():
    try:
        perf_list = subprocess.check_output(  # nosec
            ["perf", "list"], universal_newlines=True
        )
    except FileNotFoundError:
        crash("Please install Linux perf and re-run")
    except subprocess.CalledProcessError as e:
        crash(f"Error calling Linux perf, error code: {e.returncode}")
    return perf_list


def get_arch_and_name(procinfo):
    arch = modelname = ""
    try:
        model = int(procinfo[0]["model"].strip())
        cpufamily = int(procinfo[0]["cpu family"].strip())
        stepping = int(procinfo[0]["stepping"].strip())
        vendor = str(procinfo[0]["vendor_id"].strip())
        modelname = procinfo[0]["model name"].strip()
    except KeyError:
        # for non-Intel architectures
        cpuinfo = get_lscpu()
        modelname = str(cpuinfo["Model name"])
        stepping = str(cpuinfo["Stepping"])
        vendor = str(cpuinfo["Vendor ID"])
    if vendor == "GenuineIntel":
        if model == 85 and cpufamily == 6 and stepping == 4:
            arch = "skylake"
        elif model == 85 and cpufamily == 6 and stepping >= 5:
            arch = "cascadelake"
        elif model == 79 and cpufamily == 6 and stepping == 1:
            arch = "broadwell"
        elif model == 106 and cpufamily == 6 and stepping >= 4:
            arch = "icelake"
        elif model == 143 and cpufamily == 6 and stepping >= 3:
            arch = "sapphirerapids"
    return arch, modelname


# Get CPUs(as seen by OS) associated with each socket
def get_cpuid_info(procinfo):
    if "vendor_id" in procinfo[0].keys():
        vendor = str(procinfo[0]["vendor_id"].strip())
    else:
        vendor = "Non-Intel"
    socketinfo = collections.OrderedDict()
    for proc in procinfo:
        if vendor == "GenuineIntel":
            key = proc["physical id"]
        else:
            key = 0
        val = proc["processor"]
        if socketinfo.get(key) is None:
            socketinfo.setdefault(key, [val])
        else:
            socketinfo[key].append(val)
    return socketinfo


# check for special characters in output filename
def validate_outfile(filename, xlsx=False):
    valid = False
    resdir = os.path.dirname(filename)
    outfile = os.path.basename(filename)
    if resdir and not os.path.exists(resdir):
        return False
    regx = r"[@!#$%^&*()<>?\|}{~:]"
    # regex = re.compile("[@!#$%^&*()<>?/\|}{~:]")
    regex = re.compile(regx)
    if regex.search(outfile) is None:
        if filename.endswith(".csv"):
            return True
        if xlsx and filename.endswith(".xlsx"):
            return True
    return valid


# check write permissions
def check_file_writeable(outfile):
    if os.path.exists(outfile):
        if os.path.isfile(outfile):
            return os.access(outfile, os.W_OK)
        else:
            return False
    dirname = os.path.dirname(outfile)
    if not dirname:
        dirname = "."
    return os.access(dirname, os.W_OK)


# Find the percentile of a list of values
# parameter percent - a float value from 0.0 to 1.0
def percentile(N, percent):
    if not N:
        return None
    N.sort()
    k = (len(N) - 1) * percent
    f = math.floor(k)
    c = math.ceil(k)
    if f == c:
        return N[int(k)]
    d0 = N[int(f)] * (c - k)
    d1 = N[int(c)] * (k - f)
    return d0 + d1


# convert time to epoch
def get_epoch(start_time):
    words = "".join(start_time).split()
    month = words[4]
    date = words[5]
    year = words[7]
    month = str(strptime(month, "%b").tm_mon)
    # os.environ['TZ']='UTC'
    utc = tz.tzutc()
    utc_info = str(datetime.utcnow().replace(tzinfo=utc).astimezone(tz.tzlocal()))
    timestamp = (
        year + "-" + str(month) + "-" + date + " " + words[6] + " " + utc_info[-6:]
    )
    timestamp_utc_format = re.sub(r"([+-]\d+):(\d+)$", r"\1\2", timestamp)
    epoch = int(
        time.mktime(time.strptime(timestamp_utc_format, "%Y-%m-%d %H:%M:%S %z"))
    )
    return epoch


# get cgroup names by container ids
def get_cgroups_from_cids(cids):
    # cgroups is a set to exclude duplicate cids
    cgroups = set()
    try:
        p = subprocess.Popen(
            ["ps", "-e", "-o", "cgroup"], stdout=subprocess.PIPE, stderr=subprocess.PIPE
        )
        p2 = subprocess.Popen(
            ["grep", "docker-"],
            stdin=p.stdout,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        p.stdout.close()

    except subprocess.SubprocessError as e:
        crash("failed to open ps subprocess: " + str(e))
    out, err = p2.communicate()
    if err:
        crash(f"error reading cgroups: {err}")
    lines = out.decode("utf-8").split("\n")
    for cid in cids:
        found = False
        for line in lines:
            if ("docker-" + cid) in line:
                found = True
                cgroups.add(line.split(":")[-1])
        if not found:
            crash("invalid container ID: " + cid)
    # change cgroups back to list brefore returning
    return list(cgroups)


# Convert cids to comm/names
# Requires pstools python library
def get_comm_from_cid(cids, cgroups):
    cnamelist = ""
    for index, cid in enumerate(cids):
        cnamelist += cgroups[index] + "=" + cid + ","
    return cnamelist
