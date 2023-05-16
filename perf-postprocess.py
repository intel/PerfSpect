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
from src import report


class Mode(Enum):
    System = 1
    Socket = 2
    Core = 3
    Cpu = 4


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
        text = "core"
    elif t == "ca":
        text = "core.average"
    elif t == "cr":
        text = "core.raw"
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
        "--rawevents", help="save raw events in .csv format", action="store_true"
    )
    parser.add_argument(
        "-html", "--html", type=str, default=None, help="Static HTML report"
    )

    args = parser.parse_args()

    # if args.version, print version then exit
    if args.version:
        print(perf_helpers.get_tool_version())
        sys.exit()

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
# for socket or thread: add rows for each 2nd hyper thread with same values as 1st thread
def get_fixed_c6_residency_fields(perf_data_lines, perf_mode):
    # handle special case events: c6-residency
    new_perf_data_lines = []
    if meta_data["constants"]["CONST_THREAD_COUNT"] == 2:
        for fields in perf_data_lines:
            if perf_mode == Mode.System and fields[3] == "cstate_core/c6-residency/":
                # since "cstate_core/c6-residency/" is collected for only one thread
                # we double the value for the system wide collection (assign same value to the 2nd thread)
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

        infile.close()
        return meta_data_lines, perf_events_lines, perf_data_lines


# get_metadata
def get_metadata_as_dict(meta_data_lines):
    meta_data = {}
    meta_data["constants"] = {}
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

        elif line.startswith("Percore mode"):
            meta_data["PERCORE_MODE"] = (
                True if (str(line.split(",")[1]) == "enabled") else False
            )
        elif line.startswith("Percpu mode"):
            meta_data["PERCPU_MODE"] = (
                True if (str(line.split(",")[1]) == "enabled") else False
            )
        elif line.startswith("Cpu count"):
            meta_data["CPU_COUNT"] = int(line.split(",")[1])
        elif line.startswith("Persocket mode"):
            meta_data["PERSOCKET_MODE"] = (
                True if (str(line.split(",")[1]) == "enabled") else False
            )

        elif line.startswith("# started on"):
            meta_data["TIME_ZONE"] = str(line.split("# started on")[1])

        elif line.startswith("Socket"):
            if "SOCKET_CORES" not in meta_data:
                meta_data["SOCKET_CORES"] = []
            cores = ((line.split("\n")[0]).split(",")[1]).split(";")[:-1]
            meta_data["SOCKET_CORES"].append(cores)
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
    elif perf_mode == Mode.Core:  # Core should be changed to thread
        meta_data["constants"]["TSC"] = meta_data["constants"]["SYSTEM_TSC_FREQ"]
    elif perf_mode == Mode.Cpu:
        meta_data["constants"]["TSC"] = (
            meta_data["constants"]["SYSTEM_TSC_FREQ"] * meta_data["CPU_COUNT"]
        )
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
    elif microarchitecture == "sapphirerapids":
        metric_file = "metric_spr.json"
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


def get_metrics_formula(architecture):
    # get the metric file name based on architecture
    metric_file = get_metric_file_name(architecture)
    validate_file(metric_file)

    with open(metric_file, "r") as f_metric:
        try:
            metrics = json.load(f_metric)
            for m in metrics:
                m["events"] = re.findall(r"\[(.*?)\]", m["expression"])

            return metrics
        except json.decoder.JSONDecodeError:
            crash("Invalid JSON, please provide a valid JSON as metrics file")
    return


def get_socket_number(sockets_dict, core):
    core_index = core.replace("CPU", "")
    for s in range(len(sockets_dict)):
        if core_index in sockets_dict[s]:
            return s
    return


def extract_dataframe(perf_data_lines, meta_data, perf_mode):
    # parse event data into dataframe and set header names
    perf_data_df = pd.DataFrame(perf_data_lines)
    if "CGROUPS" in meta_data and meta_data["CGROUPS"] == "enabled":
        # 1.001044566,6261968509,,L1D.REPLACEMENT,/system.slice/docker-826c1c9de0bde13b0c3de7c4d96b38710cfb67c2911f30622508905ece7e0a16.scope,6789274819,5.39,,
        assert len(perf_data_df.columns) >= 7
        columns = ["ts", "value", "col0", "metric", "cgroup", "col1", "percentage"]
        # add dummy col names for remaining columns
        for col in range(7, len(perf_data_df.columns)):
            columns.append("col" + str(col))
        perf_data_df.columns = columns
    elif perf_mode == Mode.System:
        # Ubuntu 16.04 returns 6 columns, later Ubuntu's and other OS's return 8 columns
        assert len(perf_data_df.columns) >= 6
        columns = ["ts", "value", "col0", "metric", "value2", "percentage"]
        # add dummy col names for remaining columns
        for col in range(6, len(perf_data_df.columns)):
            columns.append("col" + str(col))
        perf_data_df.columns = columns
    elif perf_mode == Mode.Core or perf_mode == Mode.Socket or perf_mode == Mode.Cpu:
        assert len(perf_data_df.columns) >= 7
        columns = ["ts", "cpu", "value", "col0", "metric", "value2", "percentage"]
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
def get_group_df(time_slice_df, start_index, end_of_group_index, perf_mode):
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
    elif perf_mode == Mode.Core:  # check dataframe has cpu colmn, otherwise raise error
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


def get_event_expression_from_group(
    expressions_to_evaluate, event_df, exp_to_evaluate, event
):
    if event_df.shape == (1,):  # system wide
        if "sys" not in expressions_to_evaluate:
            expressions_to_evaluate["sys"] = exp_to_evaluate.replace(
                "[" + event + "]", str(event_df[0])
            )
        else:
            expressions_to_evaluate["sys"] = expressions_to_evaluate["sys"].replace(
                "[" + event + "]", str(event_df[0])
            )
    else:
        for index, value in event_df.iterrows():
            if index not in expressions_to_evaluate:
                expressions_to_evaluate[index] = exp_to_evaluate
            expressions_to_evaluate[index] = expressions_to_evaluate[index].replace(
                "[" + event + "]", str(value[0])
            )
    return


def generate_metrics_time_series(time_series_df, perf_mode, out_file_path):
    time_series_df_T = time_series_df.T
    time_series_df_T.index.name = "time"
    metric_file_name = ""
    if perf_mode == Mode.System:
        metric_file_name = get_extra_out_file(out_file_path, "m")
    if perf_mode == Mode.Socket:
        metric_file_name = get_extra_out_file(out_file_path, "s")

    if perf_mode == Mode.Core:
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
    if perf_mode == Mode.Core:
        average_metric_file_name = get_extra_out_file(out_file_path, "ca")

    time_series_df.index.name = "metrics"
    # throw out 1st and last datapoints since they tend to be significantly off norm
    if len(time_series_df) > 2:
        time_series_df = time_series_df.iloc[:, 1:-1]
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


def generate_metrics(
    perf_data_df,
    out_file_path,
    group_to_event,
    metadata,
    metrics,
    perf_mode,
    verbose=False,
):
    time_slice_groups = perf_data_df.groupby("ts", sort=False)
    time_metrics_result = {}
    errors = {
        "MISSING DATA": set(),
        "ZERO DIVISION": set(),
        "MISSING EVENTS": set(),
        "MULTIPLE GROUPS": set(),
    }
    prev_time_slice = 0
    for time_slice, item in time_slice_groups:
        time_slice_df = time_slice_groups.get_group(time_slice).copy()
        # normalize by difference between current time slice and previous time slice
        # this ensures that all our events are per-second, even if perf is collecting
        # over a longer time slice
        time_slice_float = float(time_slice)
        time_slice_df["value"] = time_slice_df["value"] / (
            time_slice_float - prev_time_slice
        )
        prev_time_slice = time_slice_float
        current_group_indx = 0
        group_to_df = {}
        start_index = 0
        end_of_group_index = 0
        for index, row in time_slice_df.iterrows():
            if row["metric"] in event_groups["group_" + str(current_group_indx)]:
                end_of_group_index += 1
                continue
            else:  # move to next group
                group_to_df["group_" + str(current_group_indx)] = get_group_df(
                    time_slice_df, start_index, end_of_group_index, perf_mode
                )
                start_index = end_of_group_index
                end_of_group_index += 1
                current_group_indx += 1
        # add last group
        group_to_df["group_" + str(current_group_indx)] = get_group_df(
            time_slice_df, start_index, time_slice_df.shape[0], perf_mode
        )

        metrics_results = {}
        for m in metrics:
            non_constant_events = []
            exp_to_evaluate = m["expression"]
            # substitute constants
            for event in m["events"]:
                # replace constants
                if event.upper() in metadata["constants"]:
                    exp_to_evaluate = exp_to_evaluate.replace(
                        "[" + event + "]", str(metadata["constants"][event.upper()])
                    )
                else:
                    non_constant_events.append(event)
            # find non-constant events in groups
            remaining_events_to_find = list(non_constant_events)
            expressions_to_evaluate = {}
            passes = 0
            while len(remaining_events_to_find) > 0:
                if (
                    passes == 1
                    and verbose
                    and m["name"] not in errors["MULTIPLE GROUPS"]
                ):
                    errors["MULTIPLE GROUPS"].add(m["name"])
                    logging.warning(
                        f'MULTIPLE GROUPS: metric "{m["name"]}", events "{set(non_constant_events)}"'
                    )
                passes += 1
                # find best group for remaining events
                diff_size = sys.maxsize  # big number
                best_group = None
                for group, events in group_to_event.items():
                    ds = len(set(remaining_events_to_find) - set(events))
                    if ds < diff_size and ds < len(set(remaining_events_to_find)):
                        diff_size = ds
                        best_group = group
                        if diff_size == 0:
                            break
                if best_group is None:
                    break
                for event in remaining_events_to_find[:]:
                    if event in group_to_event[best_group]:
                        remaining_events_to_find.remove(event)
                        g_df = group_to_df[best_group]
                        event_df = g_df.loc[event]
                        get_event_expression_from_group(
                            expressions_to_evaluate,
                            event_df,
                            exp_to_evaluate,
                            event,
                        )
            if len(remaining_events_to_find) == 0:  # all events are found
                # instance is either system, specific core, or specific socket
                for instance in expressions_to_evaluate:
                    if (
                        "[" in expressions_to_evaluate[instance]
                        or "]" in expressions_to_evaluate[instance]
                    ):
                        if verbose and m["name"] not in errors["MISSING DATA"]:
                            errors["MISSING DATA"].add(m["name"])
                            log_skip_metric(
                                m, expressions_to_evaluate[instance], "MISSING DATA"
                            )
                        continue
                    try:
                        result = "{:.8f}".format(
                            simple_eval(
                                expressions_to_evaluate[instance],
                                functions={"min": min, "max": max},
                            )
                        )
                    except ZeroDivisionError:
                        if verbose and m["name"] not in errors["ZERO DIVISION"]:
                            errors["ZERO DIVISION"].add(m["name"])
                            log_skip_metric(
                                m,
                                expressions_to_evaluate[instance],
                                "ZERO DIVISION",
                            )
                        result = 0
                    sub_txt = "" if instance == "sys" else "." + instance
                    metrics_results[m["name"] + sub_txt] = float(result)
            else:  # some events are missing
                if verbose and m["name"] not in errors["MISSING EVENTS"]:
                    logging.warning(
                        'MISSING EVENTS: metric "'
                        + m["name"]
                        + '" events "'
                        + str(remaining_events_to_find)
                        + '"'
                    )
                    errors["MISSING EVENTS"].add(m["name"])
                continue  # skip metric
        time_metrics_result[time_slice] = metrics_results
    time_series_df = pd.DataFrame(time_metrics_result)
    if verbose:
        for error in errors:
            logging.warning(
                str(len(errors[error])) + " " + error + ": " + str(errors[error])
            )
    generate_metrics_time_series(time_series_df, perf_mode, out_file_path)
    generate_metrics_averages(time_series_df, perf_mode, out_file_path)
    return


def generate_raw_events_system_wide(perf_data_df, out_file_path):
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


def generate_raw_events_percore(perf_data_df, out_file_path):
    # print raw values percore
    perf_data_df_core_raw = (
        perf_data_df[["metric", "cpu", "value"]]
        .groupby(["metric", "cpu"])["value"]
        .sum()
        .to_frame()
    )
    last_time_stamp = float(perf_data_df["ts"].tail(1).values[0])
    perf_data_df_core_raw["avg"] = np.where(
        perf_data_df_core_raw["value"] > 0,
        perf_data_df_core_raw["value"] / last_time_stamp,
        0,
    )

    metric_per_cpu_frame = pd.pivot_table(
        perf_data_df_core_raw, index="metric", columns="cpu", values="avg", fill_value=0
    )
    # drop uncore and power metrics
    to_drop = []
    for metric in metric_per_cpu_frame.index:
        if metric.startswith("UNC_") or metric.startswith("power/"):
            to_drop.append(metric)
    metric_per_cpu_frame.drop(to_drop, inplace=True)

    core_raw_file_name = get_extra_out_file(out_file_path, "cr")
    metric_per_cpu_frame.to_csv(core_raw_file_name)

    return


def generate_raw_events(perf_data_df, out_file_path, perf_mode):
    if perf_mode.System:
        generate_raw_events_system_wide(perf_data_df, out_file_path)
    elif perf_mode.Socket:
        generate_raw_events_socket(perf_data_df, out_file_path)
    elif perf_mode.Core:
        generate_raw_events_percore(perf_data_df, out_file_path)


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

    # parse metadata and get mode (system, socket, or core)
    meta_data = get_metadata_as_dict(meta_data_lines)
    perf_mode = Mode.System
    if "PERSOCKET_MODE" in meta_data and meta_data["PERSOCKET_MODE"]:
        perf_mode = Mode.Socket
    elif "PERCORE_MODE" in meta_data and meta_data["PERCORE_MODE"]:
        perf_mode = Mode.Core
    elif "PERCPU_MODE" in meta_data and meta_data["CPU_COUNT"] != 0:
        perf_mode = Mode.Cpu

    # fix c6 residency values
    perf_data_lines = get_fixed_c6_residency_fields(perf_data_lines, perf_mode)
    # set const TSC accoding to perf_mode
    set_CONST_TSC(meta_data, perf_mode)

    # parse event groups
    event_groups = get_event_groups(perf_event_lines)

    # extract data frame
    perf_data_df = extract_dataframe(perf_data_lines, meta_data, perf_mode)

    # parse metrics expressions
    metrics = get_metrics_formula(meta_data["constants"]["CONST_ARCH"])

    if args.rawevents:  # generate raw events for system, persocket and percore
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
                args.verbose,
            )
            logging.info(
                "Generated results file(s) in: " + out_file_path.rsplit("/", 1)[0]
            )
            if args.html:
                report.write_html(
                    cgroup_id_out_file_path,
                    perf_mode,
                    meta_data["constants"]["CONST_ARCH"],
                    args.html.replace(
                        ".html", "_" + meta_data["CGROUP_HASH"][cgroup_id] + ".html"
                    ),
                )
    # generate metrics for system, persocket or percore
    else:
        generate_metrics(
            perf_data_df,
            out_file_path,
            event_groups,
            meta_data,
            metrics,
            perf_mode,
            args.verbose,
        )
        logging.info("Generated results file(s) in: " + out_file_path.rsplit("/", 1)[0])
        if args.html:
            report.write_html(
                out_file_path,
                perf_mode,
                meta_data["constants"]["CONST_ARCH"],
                args.html,
            )
    logging.info("Done!")
