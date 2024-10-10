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


# get relevant uncore device counts
# TODO:fix for memory config with some channels populated
def get_unc_device_counts():
    sys_devs = get_sys_devices()
    counts = {}
    if "uncore_cha" in sys_devs:
        counts["cha"] = int(sys_devs["uncore_cha"])
    elif "uncore_cbox" in sys_devs:  # alternate name for cha
        counts["cha"] = int(sys_devs["uncore_cbox"])
    else:
        counts["cha"] = 0

    if "uncore_upi" in sys_devs:
        counts["upi"] = int(sys_devs["uncore_upi"])
    elif "uncore_qpi" in sys_devs:  # alternate name for upi
        counts["upi"] = int(sys_devs["uncore_qpi"])
    else:
        counts["upi"] = 0

    if "uncore_imc" in sys_devs:
        counts["imc"] = int(sys_devs["uncore_imc"])
    else:
        counts["imc"] = 0

    if "uncore_b2cmi" in sys_devs:
        counts["b2cmi"] = int(sys_devs["uncore_b2cmi"])
    else:
        counts["b2cmi"] = 0
    return counts


# return a sorted list of device ids for a given device type pattern, e.g., uncore_cha_, uncore_imc_, etc.
# note: this is necessary because device ids are not always consecutive
def get_device_ids(pattern):
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


# Returns true/false depending on state of the NMI watchdog timer, or None on error.
def nmi_watchdog_enabled():
    try:
        proc_output = subprocess.check_output(["cat", "/proc/sys/kernel/nmi_watchdog"])
    except (subprocess.CalledProcessError, FileNotFoundError) as e:
        logging.warning(f"Failed to get nmi_watchdog status: {e}")
        return None
    try:
        nmi_watchdog_status = int(proc_output.decode().strip())
    except ValueError as e:
        logging.warning(f"Failed to interpret nmi_watchdog status: {e}")
        return None
    return nmi_watchdog_status == 1


# disable nmi watchdog and return its initial status
# to restore it after collection
def disable_nmi_watchdog():
    nmi_watchdog_status = nmi_watchdog_enabled()
    if nmi_watchdog_status is None:
        logging.error("Failed to get nmi_watchdog status.")
        return None
    try:
        if nmi_watchdog_status:
            proc_output = subprocess.check_output(
                ["sysctl", "kernel.nmi_watchdog=0"], stderr=subprocess.STDOUT
            )
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
        logging.warning(f"Failed to disable nmi_watchdog: {e}")


def check_perf_event_paranoid():
    try:
        return int(
            subprocess.check_output(["cat", "/proc/sys/kernel/perf_event_paranoid"])
        )
    except (ValueError, FileNotFoundError, subprocess.CalledProcessError) as e:
        logging.warning(f"Failed to check perf_event_paranoid: {e}")


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
                try:
                    with open(muxfile, "w") as f_mux:
                        val = 0
                        if reset:
                            val = int(mux_interval[f])
                        else:
                            if int(mux_interval[f]):
                                val = int(interval_ms)
                        if val:
                            f_mux.write(str(val))
                except OSError as e:
                    logging.warning(f"Failed to write mux interval: {e}")
                    break


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
        elif model == 173 and cpufamily == 6:
            arch = "graniterapids"
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


# get_cgroups
# cid: a comma-separated list of container ids
# note: works only for cgroup v2
def get_cgroups(cid):
    # check cgroup version
    if not os.path.exists("/sys/fs/cgroup/cgroup.controllers"):
        crash("cgroup v1 detected, cgroup v2 required")
    # get cgroups from /sys/fs/cgroup directory recursively. They must start with 'docker' or 'containerd' and end with '.scope'.
    # if cid is provided, only return cgroups that match the provided container ids
    cids = cid.split(",")
    cgroups = []
    # get all cgroups
    for dirpath, dirnames, filenames in os.walk("/sys/fs/cgroup"):
        for dirname in dirnames:
            if (
                ("docker" in dirname or "containerd" in dirname)
                and dirname.endswith(".scope")
                and (len(cids) == 0 or any(map(lambda y: y in dirname, cids)))
            ):
                cgroups.append(
                    os.path.relpath(os.path.join(dirpath, dirname), "/sys/fs/cgroup")
                )
    # associate cgroups with their cpu utilization found in the usage_usec field of the cgroup's cpu.stat file
    cgroup_cpu_usage = {}
    for cgroup in cgroups:
        try:
            with open(f"/sys/fs/cgroup/{cgroup}/cpu.stat", "r") as f:
                for line in f:
                    if "usage_usec" in line:
                        cgroup_cpu_usage[cgroup] = int(line.split()[1])
        except EnvironmentError as e:
            logging.warning(str(e), UserWarning)
    # sort cgroups by cpu usage, highest usage first
    cgroups = sorted(cgroup_cpu_usage, key=cgroup_cpu_usage.get, reverse=True)

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


def get_pmu_driver_version():
    command = "dmesg | grep -A 1 'Intel PMU driver' | tail -1 | awk '{print $NF}'"
    try:
        version_number = subprocess.check_output(command, shell=True).decode().strip()
        return version_number
    except subprocess.CalledProcessError as e:
        print(f"Error executing command: {e}")
        return None
