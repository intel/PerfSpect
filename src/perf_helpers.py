#!/usr/bin/env python3

###########################################################################################################
# Copyright (C) 2020-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

import collections
import fnmatch
import logging
import os
import re
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
def get_uncore_count():
    unc_count = {}
    sys_devs = get_sys_devices()
    if "uncore_cha" in sys_devs:
        unc_count["cha"] = int(sys_devs["uncore_cha"])
    if "uncore_cbox" in sys_devs:
        unc_count["cha"] = int(sys_devs["uncore_cbox"])
    if "uncore_upi" in sys_devs:
        unc_count["upi"] = int(sys_devs["uncore_upi"])
    if "uncore_qpi" in sys_devs:
        unc_count["upi"] = int(sys_devs["uncore_qpi"])
    if "uncore_imc" in sys_devs:
        unc_count["imc"] = int(sys_devs["uncore_imc"])
    if "amd_l3" in sys_devs:
        unc_count["l3"] = int(sys_devs["amd_l3"])
    if "amd_df" in sys_devs:
        unc_count["df"] = int(sys_devs["amd_df"])
    if "amd_umc" in sys_devs:
        unc_count["umc"] = int(sys_devs["amd_umc"])
    return unc_count


# device ids are not consecutive in some cases
def get_channel_ids(pattern):
    sysdevices = os.listdir("/sys/bus/event_source/devices")
    devices = pattern + "[0-9]*"
    ids = []
    for entry in sysdevices:
        if fnmatch.fnmatch(entry, devices):
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
                crash("Failed to disable nmi watchdog.")
            logging.info(
                "nmi_watchdog temporarily disabled. Will re-enable after collection."
            )
        else:
            logging.info("nmi_watchdog already disabled. No change needed.")
        return nmi_watchdog_status
    except (ValueError, FileNotFoundError, subprocess.CalledProcessError) as e:
        crash(f"Failed to disable nmi_watchdog: {e}")


# enable nmi watchdog
def enable_nmi_watchdog():
    try:
        proc_output = subprocess.check_output(["sysctl", "kernel.nmi_watchdog=1"])
        new_watchdog_status = int(
            proc_output.decode().strip().replace("kernel.nmi_watchdog = ", "")
        )
        if new_watchdog_status != 1:
            logging.warning("Failed to re-enable nmi_watchdog.")
        else:
            logging.info("nmi_watchdog re-enabled.")
    except (ValueError, FileNotFoundError, subprocess.CalledProcessError) as e:
        logging.warning(f"Failed to re-enable nmi_watchdog: {e}")


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
        with open("/proc/cpuinfo", "r") as fo:
            for line in fo:
                try:
                    key, value = list(map(str.strip, line.split(":", 1)))
                except ValueError:
                    cpuinfo.append(temp_dict)
                    temp_dict = {}
                else:
                    temp_dict[key] = value
    except EnvironmentError as e:
        logging.warning(str(e), UserWarning)
    return cpuinfo


def get_lscpu():
    cpuinfo = {}
    try:
        lscpu = subprocess.check_output(["lscpu"]).decode()  # nosec
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
        perf_list = subprocess.check_output(["perf", "list"]).decode()  # nosec
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
        elif model == 207 and cpufamily == 6:
            arch = "emeraldrapids"
        elif model == 175 and cpufamily == 6:
            arch = "sierraforest"
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


# check write permissions on file, or directory if file doesn't exist
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


# get cgroups
def get_cgroups(cid):
    cids = cid.split(",")
    try:
        stat = subprocess.Popen(
            ["stat", "-fc", "%T", "/sys/fs/cgroup/"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
    except subprocess.SubprocessError as e:
        crash(
            "Cannot determine cgroup version. failed to open stat subprocess: " + str(e)
        )
    out, err = stat.communicate()
    out = out.decode("utf-8").strip()
    if out == "tmpfs":
        logging.info("cgroup v1 detected")
    elif out == "cgroup2fs":
        logging.info("cgroup v2 detected")
    else:
        logging.info("unknown cgroup version " + out)

    try:
        p = subprocess.Popen(
            ["ps", "-a", "-x", "-o", "cgroup", "--sort=-%cpu"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
    except subprocess.SubprocessError as e:
        crash("failed to open ps subprocess: " + str(e))
    out, err = p.communicate()
    if err:
        crash(f"error reading cgroups: {err}")

    cgroups = [
        *dict.fromkeys(
            filter(
                lambda x: (  # must be container runtime
                    "docker" in x or "containerd" in x
                )
                and x.endswith(".scope")  # don't include services
                and (  # select all or provided cids
                    len(cids) == 0 or any(map(lambda y: y in x, cids))
                ),
                map(
                    lambda x: x.split(":")[-1],  # get trailing cgroup name
                    filter(  # remove extraneous lines
                        lambda x: x != "" and x != "CGROUP" and x != "-",
                        out.decode("utf-8").split("\n"),
                    ),
                ),
            )
        )
    ]
    if len(cgroups) == 0:
        crash("no matching cgroups found")
    elif len(cgroups) > 5:
        logging.warning(
            "more than 5 matching cgroups found removing: " + str(cgroups[5:])
        )
        cgroups = cgroups[:5]
    for c in cgroups:
        logging.info("attaching to cgroup: " + c)
    return cgroups
