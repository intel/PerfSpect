#!/usr/bin/env python3

###########################################################################################################
# Copyright (C) 2021-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

import json
import numpy as np
import logging
import os
import pandas as pd
import re
import sys
from argparse import ArgumentParser
from enum import Enum
from simpleeval import simple_eval
from src.common import crash
from src import common
from src import perf_helpers


class Mode(Enum):
    System = 1
    Socket = 2
    CPU = 3


# get the filenames for miscellaneous outputs
def get_extra_out_file(out_file, t):
    dirname = os.path.dirname(out_file)
    filename = os.path.basename(out_file)
    t_file = ""
    if t == "a":
        text = "sys.average"
    elif t == "r":
        text = "sys.raw"
    elif t == "s":
        text = "socket"
    elif t == "sa":
        text = "socket.average"
    elif t == "sr":
        text = "socket.raw"
    elif t == "c":
        text = "cpu"
    elif t == "ca":
        text = "cpu.average"
    elif t == "cr":
        text = "cpu.raw"
    elif t == "m":
        text = "sys"

    parts = os.path.splitext(filename)
    if len(parts) == 1:
        t_file = text + "." + filename
    else:
        t_file = parts[-2] + "." + text + ".csv"
    return os.path.join(dirname, t_file)


def get_args(script_path):
    parser = ArgumentParser(description="perf-postprocess: perf post process")
    required_arg = parser.add_argument_group("required arguments")
    required_arg.add_argument(
        "-r",
        "--rawfile",
        type=str,
        default="perfstat.csv",
        help="Raw CSV output from perf-collect, default=perfstat.csv",
    )
    parser.add_argument(
        "--version", "-V", help="display version information", action="store_true"
    )
    parser.add_argument(
        "-o",
        "--outfile",
        type=str,
        default=script_path + "/metric_out.csv",
        help="perf stat outputs in csv format, default=metric_out.csv",
    )
    parser.add_argument(
        "-v",
        "--verbose",
        help="include debugging information, keeps all intermediate csv files",
        action="store_true",
    )
    parser.add_argument(
        "-f",
        "--fail-postprocessing",
        help="gives exit code 1 when postprocessing detects missing event or zero division errors",
        action="store_true",
    )
    parser.add_argument(
        "--rawevents", help="save raw events in .csv format", action="store_true"
    )
    parser.add_argument(
        "--pertxn",
        type=int,
        help="Generate per-transaction metrics using the provided transactions/sec.",
    )

    args = parser.parse_args()

    # if args.version, print version then exit
    if args.version:
        print(perf_helpers.get_tool_version())
        sys.exit()

    # check number of transactions > 1
    if args.pertxn is not None:
        if args.pertxn < 1:
            crash("Number of transactions cannot be < 1" % args.outfile)
        else:
            args.outfile = args.outfile.replace(".csv", "_txn.csv")

    # check rawfile argument is given
    if args.rawfile is None:
        crash("Missing raw file, please provide raw csv generated using perf-collect")

    # check rawfile argument exists
    if args.rawfile and not os.path.isfile(args.rawfile):
        crash("perf raw data file not found, please provide valid raw file")

    # check output file is valid
    if not perf_helpers.validate_outfile(args.outfile, True):
        crash(
            "Output filename: "
            + args.outfile
            + " not accepted. Filename should be a .csv without special characters"
        )

    # check output file is writable
    if not perf_helpers.check_file_writeable(args.outfile):
        crash("Output file %s not writeable " % args.outfile)

    return args


# fix c6-residency data lines
# for system: multiply value by number of HyperThreads
# for socket or cpu: add rows for each 2nd HyperThread with same values as 1st CPU
def get_fixed_c6_residency_fields(perf_data_lines, perf_mode):
    # handle special case events: c6-residency
    # if hyperthreading is disabled, no fixing is required
    if meta_data["constants"]["HYPERTHREADING_ON"] == 0:
        return perf_data_lines

    new_perf_data_lines = []
    if meta_data["constants"]["CONST_THREAD_COUNT"] == 2:
        for fields in perf_data_lines:
            if perf_mode == Mode.System and fields[3] == "cstate_core/c6-residency/":
                # since "cstate_core/c6-residency/" is collected for only one cpu per core
                # we double the value for the system wide collection (assign same value to the 2nd cpu)
                try:
                    fields[1] = int(fields[1]) * 2  # fields[1] -> event value
                except ValueError:
                    # value can be <not supported> or <not counted>
                    logging.warning(
                        "Failed to convert cstate_core/c6-residency/ metric value: "
                        + str(fields[1])
                        + " to integer. Skipping"
                    )
                    pass
                new_perf_data_lines.append(fields)
            elif fields[4] == "cstate_core/c6-residency/":
                new_fields = fields.copy()
                cpuID = int(fields[1].replace("CPU", ""))
                HT_cpuID = cpuID + int(
                    meta_data["constants"]["CONST_THREAD_COUNT"]
                    * meta_data["constants"]["CORES_PER_SOCKET"]
                )
                new_fields[1] = "CPU" + str(HT_cpuID)
                new_perf_data_lines.append(fields)
                new_perf_data_lines.append(new_fields)
            else:
                new_perf_data_lines.append(fields)
    return new_perf_data_lines


# get metadata lines and perf events' lines in three separate lists
def get_all_data_lines(input_file_path):
    with open(input_file_path, "r") as infile:
        lines = infile.readlines()

        # input file has three headers:
        # 1- ### META DATA ###,
        # 2- ### PERF EVENTS ###,
        # 3- ### PERF DATA ###,

        meta_data_lines = []
        perf_events_lines = []
        perf_data_lines = []

        meta_data_started = False
        perf_events_started = False
        perf_data_started = False
        for idx, line in enumerate(lines):
            if line.strip() == "":  # skip empty lines
                continue

            # check first line is META Data header
            elif (idx == 0) and ("### META DATA ###" not in line):
                crash(
                    "The perf raw file doesn't contain metadata, please re-collect perf raw data"
                )
            elif "### META DATA ###" in line:
                meta_data_started = True
                perf_events_started = False
                perf_data_started = False

            elif "### PERF EVENTS ###" in line:
                meta_data_started = False
                perf_events_started = True
                perf_data_started = False

            elif "### PERF DATA ###" in line:
                meta_data_started = False
                perf_events_started = False
                perf_data_started = True

            elif meta_data_started:
                meta_data_lines.append(line.strip())

            elif perf_events_started:
                perf_events_lines.append(line.strip())

            elif perf_data_started:
                if line.startswith("# started on"):
                    # this line is special, it is under "PERF DATA" (printed by perf), but it is treatesd as metadata
                    meta_data_lines.append(line.strip())
                else:
                    fields = line.split(",")
                    perf_data_lines.append(fields)

    if len(perf_data_lines) == 0:
        crash(
            "perfstat.csv contains no perf event data, try collecting for a longer time"
        )
    return meta_data_lines, perf_events_lines, perf_data_lines


# get_metadata
def get_metadata_as_dict(meta_data_lines, txns=None):
    meta_data = {}
    meta_data["constants"] = {}
    meta_data["metadata"] = {}
    if txns is not None:
        meta_data["constants"]["TXN"] = txns

    for line in meta_data_lines:
        if line.startswith("SYSTEM_TSC_FREQ"):
            meta_data["constants"]["SYSTEM_TSC_FREQ"] = (
                float(line.split(",")[1]) * 1000000
            )
        elif line.startswith("CORES_PER_SOCKET"):
            meta_data["constants"]["CORES_PER_SOCKET"] = int(line.split(",")[1])
        elif line.startswith("HYPERTHREADING_ON"):
            meta_data["constants"]["HYPERTHREADING_ON"] = int(
                line.split(",")[1] == "True"
            )
            meta_data["constants"]["CONST_THREAD_COUNT"] = (
                int(line.split(",")[1] == "True") + 1
            )
        elif line.startswith("SOCKET_COUNT"):
            meta_data["constants"]["SOCKET_COUNT"] = int(line.split(",")[1])
        elif line.startswith("CHAS_PER_SOCKET") or line.startswith("CBOX"):
            meta_data["constants"]["CHAS_PER_SOCKET"] = int(line.split(",")[1])
        elif line.startswith("Architecture"):
            meta_data["constants"]["CONST_ARCH"] = str(line.split(",")[1])

        elif line.startswith("Event grouping"):
            meta_data["EVENT_GROUPING"] = (
                True if (str(line.split(",")[1]) == "enabled") else False
            )
        elif line.startswith("cgroups"):
            if line.startswith("cgroups=disabled"):
                meta_data["CGROUPS"] = "disabled"
                continue
            # Get cgroup status and cgroup_id to container_name mapping
            meta_data["CGROUPS"] = "enabled"
            meta_data["CGROUP_HASH"] = dict(
                item.split("=")
                for item in line.split("cgroups=enabled,")[1].rstrip(",\n").split(",")
            )
            docker_HASH = []
            docker_HASH = list(meta_data["CGROUP_HASH"].values())
        elif (
            line.startswith("cpusets")
            and "CGROUPS" in meta_data
            and meta_data["CGROUPS"] == "enabled"
        ):
            line = line.replace("cpusets,", "")
            docker_SETS = []
            docker_SETS = line.split(",")
            docker_SETS = docker_SETS[:-1]
            # here length of docker_HASH should be exactly len(docker_SETS)
            assert len(docker_HASH) == len(docker_SETS)
            meta_data["CPUSETS"] = {}
            for i, docker_SET in enumerate(docker_SETS):
                if "-" in docker_SET:  # range of cpus
                    num_of_cpus = (
                        int(docker_SET.split("-")[1])
                        - int(docker_SET.split("-")[0])
                        + 1
                    )
                else:  # either one cpu, or a list of cpus separated by + sign
                    num_of_cpus = len(docker_SET.split("+"))
                meta_data["CPUSETS"][docker_HASH[i]] = num_of_cpus

        elif line.startswith("Percpu mode"):
            meta_data["PERCPU_MODE"] = (
                True if (str(line.split(",")[1]) == "enabled") else False
            )

        elif line.startswith("Persocket mode"):
            meta_data["PERSOCKET_MODE"] = (
                True if (str(line.split(",")[1]) == "enabled") else False
            )

        elif line.startswith("# started on"):
            meta_data["TIME_ZONE"] = str(line.split("# started on")[1])

        elif line.startswith("Socket"):
            if "SOCKET_CORES" not in meta_data:
                meta_data["SOCKET_CORES"] = []
            CPUs = ((line.split("\n")[0]).split(",")[1]).split(";")[:-1]
            meta_data["SOCKET_CORES"].append(CPUs)
        elif line.startswith("PSI"):
            meta_data["PSI"] = json.loads(line.split("PSI,")[1])

    for line in meta_data_lines:
        for info in [
            "SYSTEM_TSC_FREQ (MHz)",
            "CORES_PER_SOCKET",
            "SOCKET_COUNT",
            "HYPERTHREADING_ON",
            "IMC count",
            "CHAS_PER_SOCKET",
            "UPI count",
            "Architecture",
            "Model",
            "kernel version",
            "PerfSpect version",
        ]:
            if info in line:
                meta_data["metadata"][info] = line.split(",", 1)[1]
                if meta_data["metadata"][info][-1] == ",":
                    meta_data["metadata"][info] = meta_data["metadata"][info][:-1]

    return meta_data


def set_CONST_TSC(meta_data, perf_mode, num_cpus=0):
    if perf_mode == Mode.System:
        if meta_data["CGROUPS"] == "enabled" and num_cpus > 0:
            meta_data["constants"]["TSC"] = (
                meta_data["constants"]["SYSTEM_TSC_FREQ"] * num_cpus
            )
        else:
            meta_data["constants"]["TSC"] = (
                meta_data["constants"]["SYSTEM_TSC_FREQ"]
                * meta_data["constants"]["CORES_PER_SOCKET"]
                * meta_data["constants"]["CONST_THREAD_COUNT"]
                * meta_data["constants"]["SOCKET_COUNT"]
            )
    elif perf_mode == Mode.Socket:
        meta_data["constants"]["TSC"] = (
            meta_data["constants"]["SYSTEM_TSC_FREQ"]
            * meta_data["constants"]["CORES_PER_SOCKET"]
            * meta_data["constants"]["CONST_THREAD_COUNT"]
        )
    elif perf_mode == Mode.CPU:
        meta_data["constants"]["TSC"] = meta_data["constants"]["SYSTEM_TSC_FREQ"]
    return


def get_event_name(event_line):
    event_name = event_line
    if "name=" in event_name:
        matches = re.findall(r"\.*name=\'(.*?)\'.*", event_name)
        assert len(matches) > 0
        event_name = matches[0]
    if event_name.endswith(":c"):  # core event
        event_name = event_name.split(":c")[0]
    if event_name.endswith(":u"):  # uncore event
        event_name = event_name.split(":u")[0]
    # clean up , or ;
    event_name = event_name.replace(",", "").replace(";", "")

    return event_name


def get_event_groups(event_lines):
    groups = {}
    group_indx = 0

    current_group = []
    for event in event_lines:
        if ";" in event:  # end of group
            current_group.append(get_event_name(event))
            groups["group_" + str(group_indx)] = current_group
            group_indx += 1
            current_group = []
        else:
            current_group.append(get_event_name(event))
    return groups


def get_metric_file_name(microarchitecture):
    metric_file = ""
    if microarchitecture == "broadwell":
        metric_file = "metric_bdx.json"
    elif microarchitecture == "skylake" or microarchitecture == "cascadelake":
        metric_file = "metric_skx_clx.json"
    elif microarchitecture == "icelake":
        metric_file = "metric_icx.json"
    elif microarchitecture == "sapphirerapids" or microarchitecture == "emeraldrapids":
        metric_file = "metric_spr.json"
    elif microarchitecture == "sierraforest":
        metric_file = "metric_srf.json"
    else:
        crash("Suitable metric file not found")

    # Convert path of json file to relative path if being packaged by pyInstaller into a binary
    if getattr(sys, "frozen", False):
        basepath = getattr(sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__)))
        metric_file = os.path.join(basepath, metric_file)
    elif __file__:
        metric_file = script_path + "/events/" + metric_file
    else:
        crash("Unknown application type")
    return metric_file


def validate_file(fname):
    if not os.access(fname, os.R_OK):
        crash(str(fname) + " not accessible")


def get_metrics_formula(architecture, txns=None):
    # get the metric file name based on architecture
    metric_file = get_metric_file_name(architecture)
    validate_file(metric_file)

    with open(metric_file, "r") as f_metric:
        try:
            metrics = json.load(f_metric)
            for metric in metrics:
                if txns is not None:
                    if "name-txn" in metric:
                        metric["name"] = metric["name-txn"]
                    if "expression-txn" in metric:
                        metric["expression"] = metric["expression-txn"]
                metric["events"] = re.findall(r"\[(.*?)\]", metric["expression"])
            return metrics
        except json.decoder.JSONDecodeError:
            crash("Invalid JSON, please provide a valid JSON as metrics file")
    return


def get_socket_number(sockets_dict, CPU):
    CPU_index = CPU.replace("CPU", "")
    for s in range(len(sockets_dict)):
        if CPU_index in sockets_dict[s]:
            return s
    return


def extract_dataframe(perf_data_lines, meta_data, perf_mode):
    logging.info("Formatting event data")
    # parse event data into dataframe and set header names
    perf_data_df = pd.DataFrame(perf_data_lines)
    if "CGROUPS" in meta_data and meta_data["CGROUPS"] == "enabled":
        # 1.001044566,6261968509,,L1D.REPLACEMENT,/system.slice/docker-826c1c9de0bde13b0c3de7c4d96b38710cfb67c2911f30622508905ece7e0a16.scope,6789274819,5.39,,
        assert len(perf_data_df.columns) >= 7
        columns = [
            "ts",
            "value",
            "col0",
            "metric",
            "cgroup",
            "perf_group_id",
            "percentage",
        ]
        # add dummy col names for remaining columns
        for col in range(7, len(perf_data_df.columns)):
            columns.append("col" + str(col))
        perf_data_df.columns = columns
    elif perf_mode == Mode.System:
        # Ubuntu 16.04 returns 6 columns, later Ubuntu's and other OS's return 8 columns
        assert len(perf_data_df.columns) >= 6
        columns = ["ts", "value", "col0", "metric", "perf_group_id", "percentage"]
        # add dummy col names for remaining columns
        for col in range(6, len(perf_data_df.columns)):
            columns.append("col" + str(col))
        perf_data_df.columns = columns
    elif perf_mode == Mode.CPU or perf_mode == Mode.Socket:
        assert len(perf_data_df.columns) >= 7
        columns = [
            "ts",
            "cpu",
            "value",
            "col0",
            "metric",
            "perf_group_id",
            "percentage",
        ]
        # add dummy col names for remaining columns
        for col in range(7, len(perf_data_df.columns)):
            columns.append("col" + str(col))
        perf_data_df.columns = columns
        # Add socket column
        perf_data_df["socket"] = perf_data_df.apply(
            lambda x: "S" + str(get_socket_number(meta_data["SOCKET_CORES"], x["cpu"])),
            axis=1,
        )

    # fix metric name X.1, X.2, etc -> just X
    perf_data_df["metric"] = perf_data_df.apply(
        lambda x: ".".join(x["metric"].split(".")[:-1])
        if len(re.findall(r"^[0-9]*$", x["metric"].split(".")[-1])) > 0
        else x["metric"],
        axis=1,
    )

    # set data frame types
    perf_data_df["value"] = pd.to_numeric(
        perf_data_df["value"], errors="coerce"
    ).fillna(0)

    return perf_data_df


# get group data frame after grouping
def get_group_df_from_full_frame(
    time_slice_df, start_index, end_of_group_index, perf_mode
):
    g_df = time_slice_df[start_index:end_of_group_index]
    if perf_mode == Mode.System:
        g_df = g_df[["metric", "value"]].groupby("metric")["value"].sum().to_frame()
    elif perf_mode == Mode.Socket:
        if "socket" in g_df:
            g_df = (
                g_df[["metric", "socket", "value"]]
                .groupby(["metric", "socket"])["value"]
                .sum()
                .to_frame()
            )
        else:
            crash("No socket information found, exiting...")
    elif perf_mode == Mode.CPU:  # check dataframe has cpu column, otherwise raise error
        if "cpu" in g_df:
            g_df = (
                g_df[["metric", "cpu", "value"]]
                .groupby(["metric", "cpu"])["value"]
                .sum()
                .to_frame()
            )
        else:
            crash("No CPU information found, exiting...")

    return g_df


def generate_metrics_time_series(time_series_df, perf_mode, out_file_path):
    time_series_df_T = time_series_df.T
    time_series_df_T.index.name = "time"
    metric_file_name = ""
    if perf_mode == Mode.System:
        metric_file_name = get_extra_out_file(out_file_path, "m")
    if perf_mode == Mode.Socket:
        metric_file_name = get_extra_out_file(out_file_path, "s")

    if perf_mode == Mode.CPU:
        metric_file_name = get_extra_out_file(out_file_path, "c")
    # generate metrics with time indexes
    time_series_df_T.to_csv(metric_file_name)
    return


def generate_metrics_averages(
    time_series_df: pd.DataFrame, perf_mode: Mode, out_file_path: str
) -> None:
    average_metric_file_name = ""
    if perf_mode == Mode.System:
        average_metric_file_name = get_extra_out_file(out_file_path, "a")
    if perf_mode == Mode.Socket:
        average_metric_file_name = get_extra_out_file(out_file_path, "sa")
    if perf_mode == Mode.CPU:
        average_metric_file_name = get_extra_out_file(out_file_path, "ca")

    time_series_df.index.name = "metrics"
    avgcol = time_series_df.mean(numeric_only=True, axis=1).to_frame().reset_index()
    p95col = time_series_df.quantile(q=0.95, axis=1).to_frame().reset_index()
    mincol = time_series_df.min(axis=1).to_frame().reset_index()
    maxcol = time_series_df.max(axis=1).to_frame().reset_index()
    # define columns headers
    avgcol.columns = ["metrics", "avg"]
    p95col.columns = ["metrics", "p95"]
    mincol.columns = ["metrics", "min"]
    maxcol.columns = ["metrics", "max"]
    # merge columns
    time_series_df = time_series_df.merge(avgcol, on="metrics", how="outer")
    time_series_df = time_series_df.merge(p95col, on="metrics", how="outer")
    time_series_df = time_series_df.merge(mincol, on="metrics", how="outer")
    time_series_df = time_series_df.merge(maxcol, on="metrics", how="outer")

    time_series_df[["metrics", "avg", "p95", "min", "max"]].to_csv(
        average_metric_file_name, index=False
    )
    return


def row(df, name):
    if name in df.index:
        timeseries = df.loc[[name]].to_dict("split")
        timeseries["columns"] = map(lambda x: round(float(x), 1), timeseries["columns"])
        return json.dumps(list(zip(timeseries["columns"], timeseries["data"][0])))
    else:
        return "[]"


def write_html(time_series_df, perf_mode, out_file_path, meta_data, pertxn=None):
    html_file = "base.html"
    if getattr(sys, "frozen", False):
        basepath = getattr(sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__)))
        html_file = os.path.join(basepath, html_file)
    elif __file__:
        html_file = script_path + "/src/" + html_file
    else:
        crash("Unknown application type")

    html = ""
    with open(html_file, "r", encoding="utf-8") as f_html:
        html = f_html.read()

    # only show TMA if system-wide mode
    if perf_mode == Mode.System:
        html = html.replace("TRANSACTIONS", str(pertxn is not None).lower())
        time_series_df.index.name = "metrics"
        for metric in [
            ["CPUUTIL", "metric_CPU utilization %"],
            ["CPIDATA", "metric_CPI"],
            ["CPUFREQ", "metric_CPU operating frequency (in GHz)"],
            ["CPIDATA", "metric_CPI"],
            ["PKGPOWER", "metric_package power (watts)"],
            ["DRAMPOWER", "metric_DRAM power (watts)"],
            ["L1DATA", "metric_L1D MPI (includes data+rfo w/ prefetches)"],
            ["L2DATA", "metric_L2 MPI (includes code+data+rfo w/ prefetches)"],
            ["LLCDATA", "metric_LLC data read MPI (demand+prefetch)"],
            ["READDATA", "metric_memory bandwidth read (MB/sec)"],
            ["WRITEDATA", "metric_memory bandwidth write (MB/sec)"],
            ["TOTALDATA", "metric_memory bandwidth total (MB/sec)"],
            ["REMOTENUMA", "metric_NUMA %_Reads addressed to remote DRAM"],
        ]:
            new_metric = metric[1]
            if pertxn is not None:
                if "_CPI" in new_metric:
                    new_metric = new_metric.replace("_CPI", "_cycles per txn")
                if " MPI" in new_metric:
                    new_metric = new_metric.replace(" MPI", " misses per txn")
            html = html.replace(metric[0], row(time_series_df, new_metric))

        avg = time_series_df.mean(numeric_only=True, axis=1).to_frame()
        html = html.replace(
            "ALLMETRICS", json.dumps(avg.reset_index().to_dict("records"))
        )
        html = html.replace("METADATA", json.dumps(list(meta_data["metadata"].items())))
        for number in [
            ["FRONTEND", "metric_TMA_Frontend_Bound(%)"],
            ["BACKEND", "metric_TMA_Backend_Bound(%)"],
            ["COREDATA", "metric_TMA_..Core_Bound(%)"],
            ["MEMORY", "metric_TMA_..Memory_Bound(%)"],
            ["BADSPECULATION", "metric_TMA_Bad_Speculation(%)"],
            ["RETIRING", "metric_TMA_Retiring(%)"],
            ["PSI_CPU", "cpu stall %"],
            ["PSI_MEM", "memory stall %"],
            ["PSI_IO", "io stall %"],
        ]:
            try:
                html = html.replace(number[0], str(avg.loc[number[1], 0]))
            except Exception:
                html = html.replace(number[0], "0")

    with open(
        os.path.splitext(out_file_path)[0] + ".html", "w", encoding="utf-8"
    ) as file:
        file.write(html)


def log_skip_metric(metric, instance, msg):
    logging.warning(
        msg
        + ': metric "'
        + metric["name"]
        + '" expression "'
        + metric["expression"]
        + '" values "'
        + instance
        + '"'
    )


# group_start_end_index_dict is both an input and output argument
# if empty, the start and end indexes for each geroup will be added
# if not, the start and end indexes for each group will be read from it
def get_groups_to_dataframes(
    time_slice_df, group_to_event, group_start_end_index_dict, perf_mode
):
    group_to_df = {}
    if len(group_start_end_index_dict) == 0:
        current_group_indx = 0
        group_name = "group_" + str(current_group_indx)
        event_list = group_to_event[group_name]
        start_index = 0
        end_index = 0
        for i in time_slice_df.index:
            row = time_slice_df.loc[i]
            if row["metric"] in event_list:
                end_index += 1
            else:
                group_to_df[group_name] = get_group_df_from_full_frame(
                    time_slice_df, start_index, end_index, perf_mode
                )
                group_start_end_index_dict[group_name] = (start_index, end_index)
                start_index = end_index
                current_group_indx += 1
                group_name = "group_" + str(current_group_indx)
                event_list = group_to_event[group_name]
                end_index += 1
        group_to_df[group_name] = get_group_df_from_full_frame(
            time_slice_df, start_index, time_slice_df.shape[0], perf_mode
        )
        group_start_end_index_dict[group_name] = (start_index, time_slice_df.shape[0])
    else:
        for group_name in group_start_end_index_dict:
            start_index = group_start_end_index_dict[group_name][0]
            end_index = group_start_end_index_dict[group_name][1]
            group_to_df[group_name] = get_group_df_from_full_frame(
                time_slice_df, start_index, end_index, perf_mode
            )
    return group_to_df


def substitute_constants(expression, constants):
    returned_expression = expression
    for constant in constants:
        returned_expression = returned_expression.replace(
            "[" + constant + "]", str(constants[constant])
        )
    return returned_expression


# Find the best group to use to evalaute a set of events
# The best group is the one that has the majority of the events
# For example, to evaluate events [ev1, ev2, ev3, ev4]
# If group 1 has [ev1,ev2] and group 2 has [ev1, ev2, ev3]
# Then group 2 is better than group 1
def find_best_group(remaining_events, group_to_event):
    diff_size = sys.maxsize
    best_group = None
    for group, events in group_to_event.items():
        ds = len(set(remaining_events) - set(events))
        if ds < diff_size and ds < len(set(remaining_events)):
            diff_size = ds
            best_group = group
            if diff_size == 0:
                break
    return best_group


# substitute the value of an event in the given expression
# "exp_to_evaluate" is modified by the function and added to "evaluated_expressions"
# detected errors are added to "errors"
# in arguments: verbose, best_group, group_to_df, event,exp_to_evaluate,
# out arguments: errors, evaluated_expressions,
def substitute_event_in_expression(
    verbose,
    best_group,
    group_to_df,
    event,
    exp_to_evaluate,
    errors,
    evaluated_expressions,
):
    if best_group in group_to_df:
        g_df = group_to_df[best_group]
        event_df = g_df.loc[event]
        if event_df.shape == (1,):  # system wide
            if "sys" not in evaluated_expressions:
                evaluated_expressions["sys"] = exp_to_evaluate.replace(
                    "[" + event + "]", str(event_df[0])
                )
            else:
                evaluated_expressions["sys"] = evaluated_expressions["sys"].replace(
                    "[" + event + "]", str(event_df[0])
                )
        else:
            for index in event_df.index:
                value = event_df["value"][index]
                if index not in evaluated_expressions:
                    evaluated_expressions[index] = exp_to_evaluate
                evaluated_expressions[index] = evaluated_expressions[index].replace(
                    "[" + event + "]",
                    str(value),
                )
    else:  # group was not counted
        if verbose and best_group not in errors["NOT COUNTED GROUPS"]:
            errors["NOT COUNTED GROUPS"].add(best_group)
            logging.warning("Event group:" + best_group + "Not counted")
        return


# evaluate the expression of a given metric
# returns the metric name (and subname) and the evaluation result
# detected errors will be appended to "errors"
def evaluate_metric_expression(
    expressions_to_evaluate, verbose, metric, instance, errors
):
    if (
        "[" in expressions_to_evaluate[instance]
        or "]" in expressions_to_evaluate[instance]
    ):
        if verbose and metric["name"] not in errors["MISSING DATA"]:
            errors["MISSING DATA"].add(metric["name"])
            log_skip_metric(metric, expressions_to_evaluate[instance], "MISSING DATA")
        return None
    try:
        result = "{:.8f}".format(
            simple_eval(
                expressions_to_evaluate[instance],
                functions={"min": min, "max": max},
            )
        )
    except ZeroDivisionError:
        if verbose and metric["name"] not in errors["ZERO DIVISION"]:
            errors["ZERO DIVISION"].add(metric["name"])
            log_skip_metric(metric, expressions_to_evaluate[instance], "ZERO DIVISION")
        result = 0
    sub_txt = "" if instance == "sys" else "." + str(instance)
    return metric["name"] + sub_txt, float(result)


# evaluate all metrics from dataframes in group_to_df
# for each metric, we find the best group to use to evaluate the metric's expression from
def evaluate_metrics(verbose, metrics, metadata, group_to_event, group_to_df, errors):
    metrics_results = {}
    best_groups_for_events = {}
    for metric in metrics:
        non_constant_events = []
        exp_to_evaluate = substitute_constants(
            metric["expression"], metadata["constants"]
        )
        for event in metric["events"]:
            if event.upper() in metadata["constants"]:
                exp_to_evaluate = substitute_constants(
                    exp_to_evaluate,
                    {event.upper(): metadata["constants"][event.upper()]},
                )
            else:
                non_constant_events.append(event)

        remaining_events_to_find = list(non_constant_events)
        evaluated_expressions = {}
        passes = 0

        while len(remaining_events_to_find) > 0:
            if (
                passes == 1
                and verbose
                and metric["name"] not in errors["MULTIPLE GROUPS"]
            ):
                errors["MULTIPLE GROUPS"].add(metric["name"])
                logging.warning(
                    f'MULTIPLE GROUPS: metric "{metric["name"]}", events "{set(non_constant_events)}"'
                )
            passes += 1
            remaining_events_txt = str(remaining_events_to_find)
            if remaining_events_txt in best_groups_for_events:
                best_group = best_groups_for_events[remaining_events_txt]
            else:
                best_group = find_best_group(remaining_events_to_find, group_to_event)
                best_groups_for_events[remaining_events_txt] = best_group

            if best_group is None:
                break
            for event in remaining_events_to_find[:]:
                if event in group_to_event[best_group]:
                    remaining_events_to_find.remove(event)
                    substitute_event_in_expression(
                        verbose,
                        best_group,
                        group_to_df,
                        event,
                        exp_to_evaluate,
                        errors,
                        evaluated_expressions,
                    )

        if len(remaining_events_to_find) == 0:
            for instance in evaluated_expressions:
                metric_result = evaluate_metric_expression(
                    evaluated_expressions, verbose, metric, instance, errors
                )
                if metric_result is not None:
                    metrics_results[metric_result[0]] = metric_result[1]
        else:
            if verbose and metric["name"] not in errors["MISSING EVENTS"]:
                logging.warning(
                    'MISSING EVENTS: metric "'
                    + metric["name"]
                    + '" events "'
                    + str(remaining_events_to_find)
                    + '"'
                )
                errors["MISSING EVENTS"].add(metric["name"])
            continue
    return metrics_results


def generate_metrics(
    perf_data_df,
    out_file_path,
    group_to_event,
    metadata,
    metrics,
    perf_mode,
    pertxn=None,
    verbose=False,
    fail_postprocessing=False,
):
    # filter out uncore metrics if in cpu or socket mode
    filtered_metrics = []
    for m in metrics:
        if perf_mode == Mode.CPU or perf_mode == Mode.Socket:
            if any(
                [
                    e.startswith("power/")
                    or e.startswith("cstate_")
                    or e.startswith("UNC_")
                    for e in m["events"]
                ]
            ):
                continue
        filtered_metrics.append(m)

    time_slice_groups = perf_data_df.groupby("ts", sort=False)
    time_metrics_result = {}
    errors = {
        "MISSING DATA": set(),
        "ZERO DIVISION": set(),
        "MISSING EVENTS": set(),
        "MULTIPLE GROUPS": set(),
        "NOT COUNTED GROUPS": set(),
    }
    prev_time_slice = 0
    logging.info(
        "processing "
        + str(time_slice_groups.ngroups)
        + " samples in "
        + (
            "System"
            if perf_mode == Mode.System
            else "CPU"
            if perf_mode == Mode.CPU
            else "Socket"
        )
        + " mode"
    )
    group_start_end_index_dict = {}
    for time_slice, item in time_slice_groups:
        time_slice_float = float(time_slice)
        if time_slice_float - prev_time_slice < 4.5:
            logging.warning("throwing out last sample because it was too short")
            if time_slice_groups.ngroups == 1:
                crash("no remaining samples")
            continue
        time_slice_df = time_slice_groups.get_group(time_slice).copy()
        # normalize by difference between current time slice and previous time slice
        # this ensures that all our events are per-second, even if perf is collecting
        # over a longer time slice
        time_slice_df["value"] = time_slice_df["value"] / (
            time_slice_float - prev_time_slice
        )
        prev_time_slice = time_slice_float
        # get dictionary with group_ids as keys and group dataframes as values
        # We save the start and end indexes for each group in the first iteration and use it in the following iterations
        # group_start_end_index_dict is an out argument in the first iteration, and an input argument for following iterations
        group_to_df = get_groups_to_dataframes(
            time_slice_df, group_to_event, group_start_end_index_dict, perf_mode
        )

        time_metrics_result[time_slice] = evaluate_metrics(
            verbose, filtered_metrics, metadata, group_to_event, group_to_df, errors
        )

    time_series_df = pd.DataFrame(time_metrics_result).reindex(
        index=list(time_metrics_result[list(time_metrics_result.keys())[0]].keys())
    )

    if verbose:
        for error in errors:
            logging.warning(
                str(len(errors[error])) + " " + error + ": " + str(errors[error])
            )
    if fail_postprocessing and (
        len(errors["MISSING EVENTS"]) > 0 or len(errors["ZERO DIVISION"]) > 0
    ):
        crash("Failing due to postprocessing errors")

    # add psi
    if len(meta_data["PSI"]) > 0 and perf_mode == Mode.System:
        psi_len = range(len(time_series_df.columns))
        time_series_df.loc["cpu stall %"] = [
            (int(meta_data["PSI"][0][x + 1]) - int(meta_data["PSI"][0][x])) / 50000
            for x in psi_len
        ]
        time_series_df.loc["memory stall %"] = [
            (int(meta_data["PSI"][1][x + 1]) - int(meta_data["PSI"][1][x])) / 50000
            for x in psi_len
        ]
        time_series_df.loc["io stall %"] = [
            (int(meta_data["PSI"][2][x + 1]) - int(meta_data["PSI"][2][x])) / 50000
            for x in psi_len
        ]

    generate_metrics_time_series(time_series_df, perf_mode, out_file_path)
    generate_metrics_averages(time_series_df, perf_mode, out_file_path)
    if perf_mode == Mode.System:
        write_html(time_series_df, perf_mode, out_file_path, meta_data, pertxn)
    return


def generate_raw_events_system(perf_data_df, out_file_path):
    perf_data_df_system_raw = (
        perf_data_df[["metric", "value"]].groupby("metric")["value"].sum().to_frame()
    )
    last_time_stamp = float(perf_data_df["ts"].tail(1).values[0])
    # average per second. Last time stamp = total collection duration in seconds
    perf_data_df_system_raw["avg"] = np.where(
        perf_data_df_system_raw["value"] > 0,
        perf_data_df_system_raw["value"] / last_time_stamp,
        0,
    )

    sys_raw_file_name = get_extra_out_file(out_file_path, "r")
    perf_data_df_system_raw["avg"].to_csv(sys_raw_file_name)

    return


def generate_raw_events_socket(perf_data_df, out_file_path):
    # print raw values persocket
    perf_data_df_scoket_raw = (
        perf_data_df[["metric", "socket", "value"]]
        .groupby(["metric", "socket"])["value"]
        .sum()
        .to_frame()
    )
    last_time_stamp = float(perf_data_df["ts"].tail(1).values[0])
    perf_data_df_scoket_raw["avg"] = np.where(
        perf_data_df_scoket_raw["value"] > 0,
        perf_data_df_scoket_raw["value"] / last_time_stamp,
        0,
    )

    metric_per_socket_frame = pd.pivot_table(
        perf_data_df_scoket_raw,
        index="metric",
        columns="socket",
        values="avg",
        fill_value=0,
    )

    socket_raw_file_name = get_extra_out_file(out_file_path, "sr")
    metric_per_socket_frame.to_csv(socket_raw_file_name)

    return


def generate_raw_events_cpu(perf_data_df, out_file_path):
    # print raw values per CPU
    perf_data_df_CPU_raw = (
        perf_data_df[["metric", "cpu", "value"]]
        .groupby(["metric", "cpu"])["value"]
        .sum()
        .to_frame()
    )
    last_time_stamp = float(perf_data_df["ts"].tail(1).values[0])
    perf_data_df_CPU_raw["avg"] = np.where(
        perf_data_df_CPU_raw["value"] > 0,
        perf_data_df_CPU_raw["value"] / last_time_stamp,
        0,
    )

    metric_per_CPU_frame = pd.pivot_table(
        perf_data_df_CPU_raw,
        index="metric",
        columns="cpu",
        values="avg",
        fill_value=0,
    )
    # drop uncore and power metrics
    to_drop = []
    for metric in metric_per_CPU_frame.index:
        if metric.startswith("UNC_") or metric.startswith("power/"):
            to_drop.append(metric)
    metric_per_CPU_frame.drop(to_drop, inplace=True)

    CPU_raw_file_name = get_extra_out_file(out_file_path, "cr")
    metric_per_CPU_frame.to_csv(CPU_raw_file_name)

    return


def generate_raw_events(perf_data_df, out_file_path, perf_mode):
    if perf_mode.System:
        generate_raw_events_system(perf_data_df, out_file_path)
    elif perf_mode.Socket:
        generate_raw_events_socket(perf_data_df, out_file_path)
    elif perf_mode.CPU:
        generate_raw_events_cpu(perf_data_df, out_file_path)


if __name__ == "__main__":
    common.configure_logging(".")
    script_path = os.path.dirname(os.path.realpath(__file__))
    if "_MEI" in script_path:
        script_path = script_path.rsplit("/", 1)[0]
    # Parse arguments and check validity
    args = get_args(script_path)
    input_file_path = args.rawfile
    out_file_path = args.outfile
    # read all metadata, perf evernts, and perf data lines
    # Note: this might not be feasible for very large files
    meta_data_lines, perf_event_lines, perf_data_lines = get_all_data_lines(
        input_file_path
    )

    # parse metadata and get mode (system, socket, or CPU)
    meta_data = get_metadata_as_dict(meta_data_lines, args.pertxn)
    perf_mode = Mode.System
    if "PERSOCKET_MODE" in meta_data and meta_data["PERSOCKET_MODE"]:
        perf_mode = Mode.Socket
    elif "PERCPU_MODE" in meta_data and meta_data["PERCPU_MODE"]:
        perf_mode = Mode.CPU

    # fix c6 residency values
    perf_data_lines = get_fixed_c6_residency_fields(perf_data_lines, perf_mode)

    # set const TSC according to perf_mode
    set_CONST_TSC(meta_data, perf_mode)

    # parse event groups
    event_groups = get_event_groups(perf_event_lines)
    # extract data frame
    perf_data_df = extract_dataframe(perf_data_lines, meta_data, perf_mode)

    # parse metrics expressions
    metrics = get_metrics_formula(meta_data["constants"]["CONST_ARCH"], args.pertxn)

    if args.rawevents:  # generate raw events for system, socket and CPU
        generate_raw_events(perf_data_df, out_file_path, perf_mode)

    # generate metrics for each cgroup
    if "CGROUPS" in meta_data and meta_data["CGROUPS"] == "enabled":
        for cgroup_id in meta_data["CGROUP_HASH"]:
            container_id = meta_data["CGROUP_HASH"][cgroup_id]
            set_CONST_TSC(meta_data, perf_mode, meta_data["CPUSETS"][container_id])
            cgroup_id_perf_data_df = perf_data_df[perf_data_df["cgroup"] == cgroup_id]
            cgroup_id_out_file_path = (
                out_file_path.rsplit(".csv", 1)[0]
                + "_"
                + meta_data["CGROUP_HASH"][cgroup_id]
                + ".csv"
            )
            generate_metrics(
                cgroup_id_perf_data_df,
                cgroup_id_out_file_path,
                event_groups,
                meta_data,
                metrics,
                perf_mode,
                args.pertxn,
                args.verbose,
                args.fail_postprocessing,
            )
    else:
        generate_metrics(
            perf_data_df,
            out_file_path,
            event_groups,
            meta_data,
            metrics,
            perf_mode,
            args.pertxn,
            args.verbose,
            args.fail_postprocessing,
        )
        if perf_mode != Mode.System:  # always generate metrics on system level
            set_CONST_TSC(meta_data, Mode.System)
            generate_metrics(
                perf_data_df,
                out_file_path,
                event_groups,
                meta_data,
                metrics,
                Mode.System,
                args.pertxn,
                args.verbose,
                args.fail_postprocessing,
            )

    logging.info("Generated results file(s) in: " + out_file_path.rsplit("/", 1)[0])
    logging.info("Done!")
