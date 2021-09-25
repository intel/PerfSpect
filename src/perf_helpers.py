###########################################################################################################
# Copyright (C) 2021 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

import os
import sys
import re
import fnmatch
import time
import struct
import math
import collections
import subprocess  # nosec
from time import strptime
from ctypes import *  # flake8: noqa
from datetime import datetime
from dateutil import tz


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


# get cpu count
def get_cpu_count():
    cpu_count = 0
    if not os.path.isfile("/sys/devices/system/cpu/online"):
        raise SystemExit("/sys/devices/system/cpu/online not found to get core count")
    with open("/sys/devices/system/cpu/online", "r") as f_online_cpu:
        content = f_online_cpu.read()
        cpu_list = content.split(",")
        for c in cpu_list:
            limit = c.split("-")
            cpu_count += int(limit[1]) - int(limit[0]) + 1
    return cpu_count / (get_socket_count() * get_ht_count())


# compute tsc frequency
def get_tsc_freq():
    script_path = os.path.dirname(os.path.realpath(__file__))
    tsclib = script_path + "/libtsc.so"
    cdll.LoadLibrary(tsclib)
    tsc = CDLL(tsclib)
    tsc_freq = str(tsc.Calibrate())
    if tsc_freq == 0:
        raise SystemExit("can't calculate TSC frequency")
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
def get_imc_cacheagent_count():
    sys_devs = get_sys_devices()
    cha_count = 0
    imc_count = 0
    upi_count = 0
    if "uncore_cha" in sys_devs:
        cha_count = int(sys_devs["uncore_cha"])
    if "uncore_cbox" in sys_devs:
        cha_count = int(sys_devs["uncore_cbox"])
    if "uncore_upi" in sys_devs:
        upi_count = int(sys_devs["uncore_upi"])
    if "uncore_qpi" in sys_devs:
        upi_count = int(sys_devs["uncore_qpi"])
    if "uncore_imc" in sys_devs:
        imc_count = int(sys_devs["uncore_imc"])
    return imc_count, cha_count, upi_count


# get imc channel ids, channel ids are not consecutive in some cases (observed on bdw)
def get_channel_ids():
    sysdevices = os.listdir("/sys/bus/event_source/devices")
    imc = "uncore_imc_*"
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


# extend uncore events to all cores
def enumerate_uncore(event, n):
    event_list = []
    for i in range(n):
        tmp = event + "_" + str(i)
        event_list.append(tmp)
    return event_list


# read the MSR register and return the value in dec format
def readmsr(msr, cpu=0):
    f = os.open("/dev/cpu/%d/msr" % (cpu,), os.O_RDONLY)
    os.lseek(f, msr, os.SEEK_SET)
    val = struct.unpack("Q", os.read(f, 8))[0]
    os.close(f)
    return val


# parse hex to int
def parse_hex(s):
    try:
        return int(s, 16)
    except ValueError:
        raise argparse.ArgumentError("Bad hex number %s" % (s))


# detect if PMU counters are in use
def pmu_contention_detect(iterations=6):

    interval = 10
    msrregs = ["0x309", "0x30a", "0x30b", "0xc1", "0xc2", "0xc3", "0xc4"]
    values = [0] * len(msrregs)
    prev_values = [0] * len(msrregs)

    for count in range(iterations):
        for i, reg in enumerate(msrregs):
            msrreg = parse_hex(reg)
            values[i] = readmsr(msrreg)

            in_use = 0
        if count > 0:
            for j, val in enumerate(values):
                if val != prev_values[j]:
                    in_use = 1
                    if msrregs[j] == "0x309":
                        print("PMU in use, hint: instructions")
                    if msrregs[j] == "0x30a":
                        print(
                            "PMU in use, hint: cpu-cycles or Check NMI watchdog. Try: echo 0 > /proc/sys/kernel/nmi_watchdog as sudo"
                        )
                    if msrregs[j] == "0x30b":
                        print("PMU in use, hint: ref-cycles")
                    if (
                        msrregs[j] == "0xc1"
                        or msrregs[j] == "0xc2"
                        or msrregs[j] == "0xc3"
                        or msrregs[j] == "0xc4"
                    ):
                        print("Some PMUs in use")
        if in_use != 0:
            print("FAIL: PMUs in use")
            return True

        print(
            "checking iteration= %d waiting for %d seconds " % ((count + 1), interval)
        )
        time.sleep(interval)
        prev_values = values[:]

    print("PASS: PMUs not in use")
    return False


# get linux kernel version
def get_version():
    version = ""
    try:
        fo = open("/proc/version", "r")
    except EnvironmentError as e:
        warnings.warn(str(e), UserWarning)
    else:
        version = fo.read()
        version = version.split("#")[0]
    return version


# populate the CPU info list after reading /proc/cpuinfo in list of dictionaries
def get_cpuinfo():
    cpuinfo = []
    temp_dict = {}
    try:
        fo = open("/proc/cpuinfo", "r")
    except EnvironmentError as e:
        warnings.warn(str(e), UserWarning)
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
        raise SystemExit(e.output + "\nFailed to get CPUInfo")
    return cpuinfo


def not_suported():
    print(
        "Current architecture not supported!\nThis version only suports Broadwell/Skylake/Cascadelake/Icelake. Exiting!"
    )
    sys.exit()


# Check if arch is broadwell/skyalke/cascadelake
def check_architecture(procinfo):
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
        else:
            arch = "unknown"
            not_suported()
    else:
        not_suported()
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
            # assuming single socket (ARM)
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
    regx = r"[@!#$%^&*()<>?/\|}{~:]"
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
    print(start_time)
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
