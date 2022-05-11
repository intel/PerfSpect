#!/usr/bin/env python3

###########################################################################################################
# Copyright (C) 2021 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

from __future__ import print_function
import os
import re
import sys
import csv
import json
import collections
from src import perf_helpers
from simpleeval import simple_eval

script_path = os.path.dirname(os.path.realpath(__file__))

# fix the pyinstaller path
if "_MEI" in script_path:
    script_path = script_path.rsplit("/", 1)[0]

# temporary output :time series dump of raw events
output_file = script_path + "/_tmp_perf_/tmp_perf_out.csv"

# temporary output :time series dump of raw events at socket level
tmp_socket_file = script_path + "/_tmp_perf_/tmp_socket_out.csv"

# temporary output:trasposed view of perf-collect output
time_dump_file = script_path + "/_tmp_perf_/time_dump.csv"

# final output of post-process
out_metric_file = script_path + "/results/metric_out.csv"

# formula file
metric_file = ""


# globals
class workbook:
    def __init__(self):
        self.book = None
        self.sys_sheet = None
        self.sys_avg_sheet = None
        self.sys_raw_sheet = None
        self.socket_sheet = None
        self.socket_avg_sheet = None
        self.socket_raw_sheet = None
        self.core_sheet = None
        self.core_avg_sheet = None

    def initialize(self, name, persocket, percore):
        self.book = xlsxwriter.Workbook(name)
        filename = os.path.basename(name)
        filename = filename[:5]
        self.sys_avg_sheet = self.book.add_worksheet(
            get_extra_out_file(filename, "a", True)
        )
        self.sys_sheet = self.book.add_worksheet(
            get_extra_out_file(filename, "m", True)
        )
        self.sys_raw_sheet = self.book.add_worksheet(
            get_extra_out_file(filename, "r", True)
        )
        if percore or persocket:
            self.socket_avg_sheet = self.book.add_worksheet(
                get_extra_out_file(filename, "sa", True)
            )
            self.socket_sheet = self.book.add_worksheet(
                get_extra_out_file(filename, "s", True)
            )
            self.socket_raw_sheet = self.book.add_worksheet(
                get_extra_out_file(name, "sr", True)
            )
            if percore:
                self.core_avg_sheet = self.book.add_worksheet(
                    get_extra_out_file(filename, "ca", True)
                )
                self.core_sheet = self.book.add_worksheet(
                    get_extra_out_file(filename, "c", True)
                )
                self.core_raw_sheet = self.book.add_worksheet(
                    get_extra_out_file(filename, "cr", True)
                )

    def writerow(self, row, vals, sheet):
        for col, val in enumerate(vals):
            if (row != 0) and (col != 0):
                val = float(val)
            if (
                (row != 0)
                and (col == 0)
                and (sheet == "m" or sheet == "s" or sheet == "c")
            ):
                val = float(val)
            if sheet == "m":
                self.sys_sheet.write(row, col, val)
            elif sheet == "a":
                self.sys_avg_sheet.write(row, col, val)
            elif sheet == "r":
                self.sys_raw_sheet.write(row, col, val)
            elif sheet == "s":
                self.socket_sheet.write(row, col, val)
            elif sheet == "sa":
                self.socket_avg_sheet.write(row, col, val)
            elif sheet == "sr":
                self.socket_raw_sheet.write(row, col, val)
            elif sheet == "c":
                self.core_sheet.write(row, col, val)
            elif sheet == "ca":
                self.core_avg_sheet.write(row, col, val)
            elif sheet == "cr":
                self.core_raw_sheet.write(row, col, val)

    def close(self):
        self.book.close()


# global class object for excel writing
OUT_WORKBOOK = workbook()
EXCEL_OUT = False

# assumes sampling interval or dump interval is 1s
CONST_INTERVAL = 1.0
CONST_TSC_FREQ = 0.0
CONST_CORE_COUNT = 0.0
CONST_HT_COUNT = 0.0
CONST_SOCKET_COUNT = 0.0
CONST_IMC_COUNT = 0.0
CONST_CHA_COUNT = 0.0
CONST_ARCH = ""
EVENT_GROUPING = False
PERCORE_MODE = False
TIME_ZONE = "UTC"
PERF_EVENTS = []
SOCKET_CORES = []


# get the PMU names from metric expression
def get_metric_events(formula):
    f_len = len(formula)
    start = 0
    metric_events = []
    while start < f_len:
        s_idx = formula.find("[", start)
        e_idx = formula.find("]", start)
        if s_idx != -1 and e_idx != -1:
            metric_events.append(formula[s_idx + 1 : e_idx])
        else:
            break
        start = e_idx + 1
    return metric_events


# get event index based on the groupid
def get_event_index(group_id, event, event_dict):
    offset = 0
    for i in range(group_id):
        offset += len(event_dict[i])
    idx = offset + event_dict[group_id].index(event)
    return idx


# evaluate metric expression
def evaluate_expression(
    formula, const_dict, value_list, event_dict, level=0, lvl_idx=-1
):
    temp_formula = formula
    metric_events = get_metric_events(formula)
    formula = formula.replace("[", "")
    formula = formula.replace("]", "")

    # use socket count as one when evaluating per socket
    # TSC accumulation at socket level and core
    if level == 1:
        const_dict["const_socket_count"] = 1
        const_dict["const_TSC"] = CONST_TSC_FREQ * CONST_CORE_COUNT * CONST_HT_COUNT
    elif level == 2:
        const_dict["const_TSC"] = CONST_TSC_FREQ

    # assign consts in the expression and create a list for collected events
    collected_events = []
    for event in metric_events:
        if event in const_dict:
            formula = formula.replace(event, str(const_dict[event]))
        else:
            collected_events.append(event)

    grouped = False
    for group, events in event_dict.items():
        # check if all events needed for the metric are in the same group
        if all(event in events for event in collected_events):
            grouped = True
            for event in collected_events:
                if level == 0:
                    idx = (
                        get_event_index(group, event, event_dict) + 1
                    )  # add 1 to account for the time column
                elif level == 1:
                    idx = (
                        get_event_index(group, event, event_dict)
                        * int(CONST_SOCKET_COUNT)
                        + lvl_idx
                        + 1
                    )
                elif level == 2:
                    idx = (
                        get_event_index(group, event, event_dict)
                        * get_online_corecount()
                        + lvl_idx
                        + 1
                    )
                try:
                    # TODO: clean it up. quick fix for strings with /
                    if event == "power/energy-pkg/" or event == "power/energy-ram/":
                        formula = formula.replace(event, str(value_list[idx]))
                    else:
                        formula = re.sub(
                            r"\b" + event + r"\b", str(value_list[idx]), formula
                        )
                except IndexError:
                    print("Index Error while evaluating expression")
                    print(formula, event, idx, len(value_list))
                    exit()

            break

    # pick first matching event from the event list if not grouped
    if not grouped:
        for event in collected_events:
            for group, events in event_dict.items():
                if event in events:
                    if level == 0:
                        idx = (
                            get_event_index(group, event, event_dict) + 1
                        )  # add 1 to account for the time column
                    elif level == 1:
                        idx = (
                            get_event_index(group, event, event_dict)
                            * int(CONST_SOCKET_COUNT)
                            + lvl_idx
                            + 1
                        )
                    elif level == 2:
                        idx = (
                            get_event_index(group, event, event_dict)
                            * get_online_corecount()
                            + lvl_idx
                            + 1
                        )
                    # TODO: clean it up. quick fix for strings with /
                    if event == "power/energy-pkg/" or event == "power/energy-ram/":
                        formula = formula.replace(event, str(value_list[idx]))
                    else:
                        formula = re.sub(
                            r"\b" + event + r"\b", str(value_list[idx]), formula
                        )
                    break
    result = ""
    try:
        result = str(
            "{:.8f}".format(simple_eval(formula, functions={"min": min, "max": max}))
        )
    except ZeroDivisionError:
        if "UNC_M_PMM" not in temp_formula and "UNC_M_TAGC" not in temp_formula:
            print("Divide by Zero evaluating", temp_formula)
        result = "0"
        pass
    except SyntaxError:
        print("Syntax error evaluating ", formula)
        print(temp_formula)
        sys.exit()
    except Exception as e:
        print(e)
        print(temp_formula)
        print("Unknown error evaluating ", formula)
        sys.exit()

    return result


# disable invalid events
def disable_event(index):
    global PERF_EVENTS
    try:
        PERF_EVENTS[index] = "#" + PERF_EVENTS[index]
    except IndexError:
        exit("Index out of range for disabling perf event")


def validate_file(fname):
    if not os.access(fname, os.R_OK):
        raise SystemExit(str(fname) + " not accessible")


def is_safe_file(fname, substr):
    if not fname.endswith(substr):
        raise SystemExit(str(fname) + " not a valid file")
    return 1


# get events from perf event file
def get_perf_events(level):
    event_list = []
    event_dict = collections.OrderedDict()
    group_id = 0
    for line in PERF_EVENTS:
        if (line != "\n") and (line.startswith("#") is False):
            if level == 2 and line.strip().endswith(
                ":u"
            ):  # ignore uncore events for percore processing
                continue
            # remove the core/uncore identifier
            line = line.strip()[:-2]
            new_group = False
            if line.strip().endswith(";"):
                new_group = True

            line = line.strip()[:-1]
            event = line
            if "name=" in line:
                event = (line.split("'"))[1]
            event_list.append(event)
            if event_dict.get(group_id) is None:
                event_dict.setdefault(group_id, [event])
            else:
                event_dict[group_id].append(event)
            if new_group:
                group_id += 1
    return event_list, event_dict


# get the filenames for miscellaneous outputs
def get_extra_out_file(out_file, t, excelsheet=False):
    dirname = os.path.dirname(out_file)
    filename = os.path.basename(out_file)
    t_file = ""
    if t == "a":
        text = "sys.average" if excelsheet else "average"
    elif t == "r":
        text = "sys.raw" if excelsheet else "raw"
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
    if excelsheet:
        return text
    parts = os.path.splitext(filename)
    if len(parts) == 1:
        t_file = text + "." + filename
    else:
        t_file = parts[-2] + "." + text + ".csv"
    if is_safe_file(t_file, ".csv"):
        pass
    return os.path.join(dirname, t_file)


# load metrics from json file and evaluate
# level: 0-> system, 1->socket, 2->thread
def load_metrics(level=0):
    event_list, event_dict = get_perf_events(level)
    metrics = {}
    validate_file(metric_file)
    with open(metric_file, "r") as f_metric:
        try:
            metrics = json.load(f_metric)
        except json.decoder.JSONDecodeError:
            raise SystemExit(
                "Invalid JSON, please provide a valid JSON as metrics file"
            )

        for i, metric in enumerate(metrics):
            metric_events = get_metric_events(metric["expression"].strip())
            metrics[i]["add"] = True
            # check if metric can be computed from the current events
            for e in metric_events:
                if e.startswith("const"):
                    continue
                if e not in event_list:
                    metrics[i]["add"] = False
        f_metric.close()

    metric_row = ["time"]
    add_metrics = False
    if is_safe_file(out_metric_file, ".csv"):
        pass
    for m in metrics:
        if m["add"] is True:
            add_metrics = True
            if level == 0:
                metric_row.append(m["name"])
                f_out = open(out_metric_file, "w")
                input_file = output_file
                sheet_type = "m"
            elif level == 1:
                for s in range(int(CONST_SOCKET_COUNT)):
                    metric_row.append(m["name"] + ".S" + str(s))
                socket_file = get_extra_out_file(out_metric_file, "s")
                f_out = open(socket_file, "w")
                input_file = tmp_socket_file
                sheet_type = "s"
            elif level == 2:
                for c in range(
                    int(CONST_CORE_COUNT * CONST_HT_COUNT * CONST_SOCKET_COUNT)
                ):
                    metric_row.append(m["name"] + ".C" + str(c))
                core_file = get_extra_out_file(out_metric_file, "c")
                f_out = open(core_file, "w")
                input_file = time_dump_file
                sheet_type = "c"

    # nothing to do, return
    if not add_metrics:
        return 0

    metriccsv = csv.writer(f_out, dialect="excel")
    metriccsv.writerow(metric_row)
    if EXCEL_OUT:
        OUT_WORKBOOK.writerow(0, metric_row, sheet_type)
    f_pmu = open(input_file, "r")
    pmucsv = csv.reader(f_pmu, delimiter=",")

    const_TSC = CONST_TSC_FREQ * CONST_CORE_COUNT * CONST_HT_COUNT * CONST_SOCKET_COUNT
    const_dict = {
        "const_tsc_freq": CONST_TSC_FREQ,
        "const_core_count": CONST_CORE_COUNT,
        "const_socket_count": CONST_SOCKET_COUNT,
        "const_thread_count": CONST_HT_COUNT,
        "const_cha_count": CONST_CHA_COUNT,
        "const_TSC": const_TSC,
    }
    pmu_row_count = 0
    metric_value = [""] * len(metric_row)
    for row in pmucsv:
        if not row:
            continue
        if pmu_row_count > 0:
            metric_value[0] = row[0]
            for metric in metrics:
                if metric["add"]:
                    if level == 0:
                        idx = metric_row.index(metric["name"])
                        result = evaluate_expression(
                            metric["expression"], const_dict, row, event_dict
                        )
                        metric_value[idx] = result
                    elif level == 1:
                        for s in range(int(CONST_SOCKET_COUNT)):
                            metric_name = metric["name"] + ".S" + str(s)
                            idx = metric_row.index(metric_name)
                            result = evaluate_expression(
                                metric["expression"],
                                const_dict,
                                row,
                                event_dict,
                                level,
                                s,
                            )
                            metric_value[idx] = result
                    elif level == 2:
                        for c in range(
                            int(CONST_CORE_COUNT * CONST_HT_COUNT * CONST_SOCKET_COUNT)
                        ):
                            metric_name = metric["name"] + ".C" + str(c)
                            idx = metric_row.index(metric_name)
                            result = (
                                evaluate_expression(
                                    metric["expression"],
                                    const_dict,
                                    row,
                                    event_dict,
                                    level,
                                    c,
                                )
                                if is_online_core(c)
                                else 0.0
                            )
                            metric_value[idx] = result
            metriccsv.writerow(metric_value)
            if EXCEL_OUT:
                OUT_WORKBOOK.writerow(pmu_row_count, metric_value, sheet_type)
        pmu_row_count += 1

    f_out.close()
    f_pmu.close()
    return 1


# generate summary output with averages, min, max, p95
def write_summary(level=0):
    if level == 0:
        metric_file = out_metric_file
    elif level == 1:
        metric_file = get_extra_out_file(out_metric_file, "s")
    elif level == 2:
        metric_file = get_extra_out_file(out_metric_file, "c")
    validate_file(metric_file)
    f_metrics = open(metric_file, "r")
    columns = collections.defaultdict(list)
    reader = csv.DictReader(f_metrics, delimiter=",")

    first_row = True
    metrics = []
    for row in reader:
        if first_row:
            for h in reader.fieldnames:
                metrics.append(h)
            first_row = False
        for (k, v) in row.items():
            columns[k].append(float(v))

    sheet_type = ""
    if level == 0:
        sum_file = get_extra_out_file(out_metric_file, "a")
        first_row = ["metrics", "avg", "p95", "min", "max"]
        sheet_type = "a"
    elif level == 1:
        sum_file = get_extra_out_file(out_metric_file, "sa")
        first_row = ["metrics"]
        out_row = [""] * (int(CONST_SOCKET_COUNT) * 2 + 1)
        for t in range(2):
            for i in range(int(CONST_SOCKET_COUNT)):
                first_row.append("S" + str(i) + (".avg" if t == 0 else ".p95"))
        sheet_type = "sa"
    elif level == 2:
        sum_file = get_extra_out_file(out_metric_file, "ca")
        first_row = ["metrics"]
        corecount = get_online_corecount()
        out_row = [""] * (corecount + 1)
        for i in range(corecount):
            first_row.append("C" + str(i) + ".avg")
        sheet_type = "ca"

    f_sum = open(sum_file, "w")
    sumcsv = csv.writer(f_sum, dialect="excel")
    sumcsv.writerow(first_row)
    if EXCEL_OUT:
        OUT_WORKBOOK.writerow(0, first_row, sheet_type)
    out_idx = 1

    for i, h in enumerate(metrics):
        if i == 0:
            continue
        avg = sum(columns[h]) / len(columns[h])
        minval = min(columns[h])
        maxval = max(columns[h])
        p95 = perf_helpers.percentile(columns[h], 0.95)
        if level == 0:
            sumcsv.writerow([h, avg, p95, minval, maxval])
            if EXCEL_OUT:
                OUT_WORKBOOK.writerow(i, [h, avg, p95, minval, maxval], sheet_type)
        elif level == 1:
            socket_id = (i - 1) % int(
                CONST_SOCKET_COUNT
            )  # -1 for first column in metrics - time
            out_row[socket_id + 1] = avg
            out_row[socket_id + 1 + int(CONST_SOCKET_COUNT)] = p95
            if socket_id == (int(CONST_SOCKET_COUNT) - 1):
                out_row[0] = h[:-3]  # to remove .S0/.S1 etc
                sumcsv.writerow(out_row)
                if EXCEL_OUT:
                    OUT_WORKBOOK.writerow(out_idx, out_row, sheet_type)
                out_idx += 1
        elif level == 2:
            # [metric, C0.avg, C1.avg, .. CN-1.avg]
            core_id = (i - 1) % corecount
            out_row[core_id + 1] = avg
            if core_id == (corecount - 1):
                name_len = len(h) - len(h.split(".")[-1]) - 1
                out_row[0] = h[:name_len]
                sumcsv.writerow(out_row)
                if EXCEL_OUT:
                    OUT_WORKBOOK.writerow(out_idx, out_row, sheet_type)
                out_idx += 1


def get_online_corecount():
    return int(CONST_CORE_COUNT * CONST_HT_COUNT * CONST_SOCKET_COUNT)


def is_online_core(c):
    return True


# get metadata from perf stat dump
def get_metadata():
    global CONST_TSC_FREQ
    global CONST_CORE_COUNT
    global CONST_HT_COUNT
    global CONST_SOCKET_COUNT
    global CONST_IMC_COUNT
    global CONST_CHA_COUNT
    global PERF_EVENTS
    global CONST_INTERVAL
    global CONST_ARCH
    global EVENT_GROUPING
    global PERCORE_MODE
    global SOCKET_CORES
    global TIME_ZONE

    start_events = False
    validate_file(dat_file)
    f_dat = open(dat_file, "r")
    for line in f_dat:
        if start_events:
            if "PERF DATA" in line:
                break
            PERF_EVENTS.append(line)
            continue

        if line.startswith("TSC"):
            CONST_TSC_FREQ = float(line.split(",")[1]) * 1000000
        elif line.startswith("CPU"):
            CONST_CORE_COUNT = float(line.split(",")[1])
        elif line.startswith("HT"):
            CONST_HT_COUNT = float(line.split(",")[1])
        elif line.startswith("SOCKET"):
            CONST_SOCKET_COUNT = float(line.split(",")[1])
        elif line.startswith("IMC"):
            CONST_IMC_COUNT = float(line.split(",")[1])
        elif line.startswith("CHA") or line.startswith("CBOX"):
            CONST_CHA_COUNT = float(line.split(",")[1])
        elif line.startswith("Sampling"):
            CONST_INTERVAL = float(line.split(",")[1])
        elif line.startswith("Architecture"):
            CONST_ARCH = str(line.split(",")[1])
        elif line.startswith("Event grouping"):
            EVENT_GROUPING = True if (str(line.split(",")[1]) == "enabled") else False
        elif line.startswith("Percore mode"):
            PERCORE_MODE = True if (str(line.split(",")[1]) == "enabled") else False
        elif line.startswith("# started on"):
            TIME_ZONE = str(line.split(",")[2])
        elif line.startswith("Socket"):
            cores = ((line.split("\n")[0]).split(",")[1]).split(";")[:-1]
            SOCKET_CORES.append(cores)
        elif "### PERF EVENTS" in line:
            start_events = True
    f_dat.close()


# write perf output from perf stat dump
def write_perf_tmp_output(use_epoch):
    global CONST_TSC_FREQ
    global CONST_CORE_COUNT
    global CONST_HT_COUNT
    global CONST_SOCKET_COUNT
    global CONST_IMC_COUNT
    global CONST_CHA_COUNT
    global CONST_INTERVAL
    global PERCORE_MODE
    global TIME_ZONE

    f_out = open(time_dump_file, "w")
    outcsv = csv.writer(f_out, dialect="excel")

    row0_event_name = []
    row_data = []
    percent_data = []
    first_out_row = True
    prev_sample_time = 0.0
    prev_sample_time_row = 0.0
    start_perf = False
    start_time = True
    samples = 0
    epoch = 0
    validate_file(dat_file)
    with open(dat_file, "r") as f_dat:
        incsv = csv.reader(f_dat, delimiter=",")
        row0_event_name.append("time")

        for it, row in enumerate(incsv):
            if not start_perf:
                if row and "PERF DATA" in row[0]:
                    start_perf = True
                continue
            if use_epoch and start_time:
                start_time = False
                words = "".join(row).split()
                try:
                    epoch = int(words[-1])
                except ValueError:
                    exit("Conversion error parsing timestamp")
                except:
                    exit("Unkown error parsing timestamp")
            if row and start_perf and (len(row) > 3):
                time = float(row[0])
                # extract data , Note: relies on the perf output format
                if use_epoch:
                    time = int(time) + epoch
                # perf reports PMU parameters at slightly longer interval than the specific 1s.
                # "accurate_time" gets more precise time interval to calculate and normalize per sec metrics
                # Note: calculation is based on the fact that timing interval does not change for all PMU events in the groups
                if prev_sample_time != time:
                    accurate_time = float(row[0]) - prev_sample_time_row
                    prev_sample_time_row = float(row[0])

                if PERCORE_MODE:
                    cpuid = row[1].strip()
                    cpuid = cpuid[3:]
                    name = row[4].strip() + "." + cpuid
                    value_idx = 2
                    percent_idx = 6
                else:
                    name = row[3].strip()
                    value_idx = 1
                    percent_idx = 5

                try:
                    # Replaced the constanct_interval (1s) with the more precious per time interval value = float(row[value_idx]) / CONST_INTERVAL
                    value = float(row[value_idx]) / accurate_time
                    percent = row[percent_idx]
                except ValueError:
                    if first_out_row:
                        print("Conversion Error parsing ", name)
                    value = -1.0
                    percent = 0
                    pass
                except:
                    exit("Unkown error parsing ", name)

                # finished parsing one timestamp - write to output
                if prev_sample_time != time:
                    if (len(row_data) > 0) and first_out_row:
                        tmp_list = row0_event_name[1:]
                        # extend label with pecent sample
                        if not PERCORE_MODE:
                            for e in tmp_list:
                                row0_event_name.append(e + " %sample")
                        outcsv.writerow(row0_event_name)
                        first_out_row = False

                    if not PERCORE_MODE:
                        row_data.extend(percent_data)
                    if not first_out_row:
                        outcsv.writerow(row_data)
                    # prep for new row
                    row_data = []
                    percent_data = []
                    row_data.append(time if use_epoch else (samples + 1))
                    samples += 1

                if first_out_row:
                    row0_event_name.append(name)

                row_data.append(value)
                if not PERCORE_MODE:
                    percent_data.append(percent)
                prev_sample_time = time
        if len(row_data) > 0:
            outcsv.writerow(row_data)
    f_dat.close()
    f_out.close()
    return samples


# core level accumulation
def write_core_view():
    core_file = get_extra_out_file(out_metric_file, "cr")
    f_out = open(core_file, "w")
    outcsv = csv.writer(f_out, dialect="excel")
    f_in = open(time_dump_file, "r")
    incsv = csv.reader(f_in, delimiter=",")
    rowcount = 0
    names = []
    idxs = []
    events, _ = get_perf_events(2)
    sumrow = []
    for row in incsv:
        if not row:
            continue
        if not rowcount:
            for i, event in enumerate(row):
                id_len = len(event.split(".")[-1])
                name = event[: len(event) - id_len - 1]

                if name in events:
                    names.append(event)
                    idxs.append(i)  # store indexes of input file
            rowcount = rowcount + 1
            sumrow = [0.0] * len(names)
            continue
        for i, idx in enumerate(idxs):
            sumrow[i] += float(row[idx])
        rowcount += 1

    # summary/raw file. format:
    # metrics, c0, c1, c2 ..
    # name_of_metric, val0, val1, val2 ..
    first_row = ["metrics"]
    core_count = get_online_corecount()
    for i in range(core_count):
        first_row.append("C" + str(i))
    outcsv.writerow(first_row)
    if EXCEL_OUT:
        OUT_WORKBOOK.writerow(0, first_row, "cr")
    out_idx = 1
    tempsum = [0.0] * (core_count)
    for i in range(len(sumrow)):
        core_id = i % core_count
        tempsum[core_id] = int(sumrow[i] / rowcount)
        if core_id == core_count - 1:
            temprow = []
            name_len = len(names[i]) - len(names[i].split(".")[-1]) - 1
            temprow.append((names[i])[:name_len])
            for s in tempsum:
                temprow.append(str(s))
            outcsv.writerow(temprow)
            if EXCEL_OUT:
                OUT_WORKBOOK.writerow(out_idx, temprow, "cr")
            out_idx += 1
    f_out.close()
    f_in.close()


# for storing column indicies for socket view
class persocket_idx:
    def __init__(self, name, idx):
        self.name = name
        self.idx = idx

    def display(self):
        print(self.name)
        print(self.idx)

    def getidx(self):
        return self.idx

    def getname(self):
        return self.name

    def append(self, level):
        for i, val in enumerate(level):
            if len(val):
                self.idx[i].extend(val)


# create socketlevel accumulation
def write_socket_view(level, samples):
    global SOCKET_CORES
    global EVENT_GROUPING
    global EXCEL_OUT
    socket_count = len(SOCKET_CORES)

    f_out = open(tmp_socket_file, "w")
    outcsv = csv.writer(f_out, dialect="excel")
    f_in = open(time_dump_file, "r")
    incsv = csv.reader(f_in, delimiter=",")

    row_count = 0
    prev_event_name = ""
    outrow0 = []
    mappings = []
    sumrow = []
    writeoutput = True

    for inrow in incsv:
        if not inrow:
            continue
        rowlen = len(inrow)
        if row_count == 0:
            core_to_idx = []
            for i, name in enumerate(inrow):
                if i == 0:
                    # first column is time
                    outrow0.append(name)
                    continue
                tmp = name.split(".")
                coreid = (name.split("."))[-1]
                namelen = len(name) - len(coreid) - 1
                name = name[:namelen]
                if name.startswith("UNC") and EVENT_GROUPING:
                    namelen = len(name) - len(tmp[-2]) - 1
                    name = name[:namelen]

                # flushout the indicies to mapping
                # new event starting, push it to output list
                if name != prev_event_name or i == (rowlen - 1):
                    if len(core_to_idx):
                        if i == (rowlen - 1):
                            for s, cores in enumerate(SOCKET_CORES):
                                if coreid in cores:
                                    core_to_idx[s].append(i)
                                    break
                        present = False
                        if name.startswith("UNC") and EVENT_GROUPING:
                            for m in mappings:
                                if m.getname() == prev_event_name:
                                    m.append(core_to_idx)
                                    present = True
                                    break
                        if not present:
                            mapping = persocket_idx(prev_event_name, core_to_idx)
                            mappings.append(mapping)
                            ename = mapping.getname()
                            for s in range(socket_count):
                                outrow0.append(ename + "." + str(s))
                    core_to_idx = []
                    for s in range(socket_count):
                        core_to_idx.append([])
                    if i == (rowlen - 1):
                        outcsv.writerow(outrow0)
                        break

                prev_event_name = name

                for s, cores in enumerate(SOCKET_CORES):
                    if coreid in cores:
                        core_to_idx[s].append(i)
                        break

            row_count = row_count + 1
            if len(outrow0) != (len(mappings) * socket_count + 1):
                print(
                    "something wrong in socket view processing %d %d"
                    % (len(outrow0), len(mappings))
                )
                sys.exit()
            continue

        outrow = [0.0] * len(outrow0)
        sumrow = [0.0] * len(outrow0)
        prev_inrow = [0.0] * rowlen
        for i, name in enumerate(outrow0):
            if not i:
                outrow[i] = inrow[i]
                continue

            socket_id = int((name.split("."))[-1])
            mapping_idx = int((i - 1) / socket_count)
            mapping = mappings[mapping_idx]

            indices = mapping.getidx()
            for idx in indices[socket_id]:
                if float(inrow[idx]) >= 0.0:
                    outrow[i] = outrow[i] + float(inrow[idx])
                else:  # invalid perf stat, drop the values if last sample, else use the previous values
                    if row_count == samples:
                        writeoutput = False
                    outrow[i] = outrow[i] + float(prev_inrow[idx])
            sumrow[i] += outrow[i]

        if writeoutput:
            outcsv.writerow(outrow)
            row_count = row_count + 1
        prev_inrow = inrow

    # summary/raw file
    if not level:
        return
    sum_file = get_extra_out_file(out_metric_file, "sr")
    f_sum = open(sum_file, "w")
    sumcsv = csv.writer(f_sum, dialect="excel")
    first_row = ["metrics"]
    for s in range(int(CONST_SOCKET_COUNT)):
        first_row.append("S" + str(s))
    sumcsv.writerow(first_row)
    if EXCEL_OUT:
        OUT_WORKBOOK.writerow(0, first_row, "sr")
    tempsum = [0.0] * (int(CONST_SOCKET_COUNT))
    out_idx = 1
    for i in range(len(sumrow)):
        if not i:
            continue
        socket_id = (i - 1) % int(CONST_SOCKET_COUNT)
        tempsum[socket_id] = int(sumrow[i] / row_count)
        if socket_id == int(CONST_SOCKET_COUNT) - 1:
            temprow = []
            temprow.append((outrow0[i])[:-2])
            for s in tempsum:
                temprow.append(str(s))
            sumcsv.writerow(temprow)
            if EXCEL_OUT:
                OUT_WORKBOOK.writerow(out_idx, temprow, "sr")
            out_idx += 1

    f_sum.close()


# write system view from socket level data
def write_socket2system():
    f_in = open(tmp_socket_file, "r")
    incsv = csv.reader(f_in, delimiter=",")
    f_out = open(output_file, "w")
    outcsv = csv.writer(f_out, dialect="excel")

    firstrow = True
    outrow0 = []
    outrow = []
    rowlen = 0
    sumrow = []
    entries = 0
    for row in incsv:
        if not row:
            continue
        idx = 0
        if firstrow:
            rowlen = int((len(row) - 1) / int(CONST_SOCKET_COUNT)) + 1
            outrow0 = [""] * rowlen
            for i, name in enumerate(row):
                if i == 0:
                    outrow0[idx] = name
                    idx += 1
                elif ((i - 1) % int(CONST_SOCKET_COUNT)) == (
                    int(CONST_SOCKET_COUNT) - 1
                ):
                    outrow0[idx] = name[:-2]
                    idx += 1
            outcsv.writerow(outrow0)
            sumrow = [0.0] * rowlen
            firstrow = False
            continue

        outrow = [0.0] * rowlen
        for i, val in enumerate(row):
            if i == 0:
                outrow[idx] = val
                totalval = 0.0
                idx += 1
            elif ((i - 1) % int(CONST_SOCKET_COUNT)) == (int(CONST_SOCKET_COUNT) - 1):
                totalval += float(val)
                outrow[idx] = str(totalval)
                sumrow[idx] += totalval
                totalval = 0.0
                idx += 1
            else:
                totalval += float(val)
        outcsv.writerow(outrow)
        entries += 1

    f_sum = open(get_extra_out_file(out_metric_file, "r"), "w")
    sumcsv = csv.writer(f_sum, dialect="excel")
    sumcsv.writerow(["metrics", "avg"])
    if EXCEL_OUT:
        OUT_WORKBOOK.writerow(0, ["metrics", "avg"], "r")
    for i in range(rowlen - 1):
        sumrow[i + 1] = sumrow[i + 1] / entries
        sumcsv.writerow([outrow0[i + 1], str(sumrow[i + 1])])
        if EXCEL_OUT:
            OUT_WORKBOOK.writerow(i + 1, [outrow0[i + 1], str(sumrow[i + 1])], "r")
    f_sum.close()
    f_out.close()
    f_in.close()


# combine per cha/imc counters from tmp output to systemview
def write_system_view():
    f_out = open(output_file, "w")
    outcsv = csv.writer(f_out, dialect="excel")
    f_tmp = open(time_dump_file, "r")
    tmpcsv = csv.reader(f_tmp, delimiter=",")
    row_count = 0
    out_row0 = []
    out_row = []
    sum_row = []
    final_out_row = []
    final_out_row0 = []
    prev_out_row = []
    disabled_events = []
    for in_row in tmpcsv:
        if not in_row:
            continue
        if row_count == 0:
            in_row0 = in_row[:]

        for i, event in enumerate(in_row0):
            if event.endswith("%sample"):
                break
            # cumulative sum for uncore event counters
            if event.startswith("UNC"):
                id_idx_start = event.rfind(".")
                # save row0 event name from the first uncore event
                if row_count == 0:
                    if event[id_idx_start + 1 :].isdigit():
                        if event.endswith(".0") and event[:-2] not in out_row0:
                            out_row0.append(event[:-2])
                    else:  # grouping disabled case: disaggregated uncore events will have the same name
                        if event not in out_row0:
                            out_row0.append(event)
                else:
                    # FIX ME: assumes each uncore event occur only once in the event file
                    if event[id_idx_start + 1 :].isdigit():
                        unc_event = event[:id_idx_start]
                        idx = out_row0.index(unc_event)
                        out_row[idx] += float(in_row[in_row0.index(event)])
                    else:  # grouping disabled case
                        idx = out_row0.index(event)
                        out_row[idx] += float(in_row[i])
            else:
                if row_count == 0:
                    out_row0.append(event)
                else:
                    if out_row0.count(event) > 1:
                        for j, e in enumerate(out_row0):
                            if e == event and out_row[j] == 0:
                                out_row[j] = in_row[i]
                                break
                    else:
                        out_row[out_row0.index(event)] = in_row[i]

        if row_count > 0:
            for i, val in enumerate(out_row):
                # remove columns with invalid values and remove the headers
                if float(val) >= 0.0:
                    final_out_row.append(val)
                    if row_count == 1:
                        final_out_row0.append(out_row0[i])
                else:
                    if row_count == 1:
                        disable_event(i - 1)
                        disabled_events.append(out_row0[i])
                    # too late to disable events
                    else:
                        if len(disabled_events) and (out_row0[i] in disabled_events):
                            val = 0
                        else:
                            print(
                                "Warning: Invalid value found for %s counter at interval %d (defaults to previous count)"
                                % (out_row0[i], row_count + 1)
                            )
                            val = prev_out_row[i]
                        final_out_row.append(val)
            if row_count == 1:
                outcsv.writerow(final_out_row0)
                sum_row = [0.0] * len(final_out_row0)
            outcsv.writerow(final_out_row)
            for j in range(len(final_out_row0) - 1):
                try:
                    sum_row[j + 1] += float(final_out_row[j + 1])
                except IndexError:
                    print(
                        "event=%s, j=%d, len=%d "
                        % (final_out_row0[j], j, len(final_out_row))
                    )
            prev_out_row = final_out_row
            final_out_row = []

        out_row = [0] * len(out_row0)
        row_count += 1

    f_out.close()
    f_tmp.close()

    sum_file = get_extra_out_file(out_metric_file, "r")
    f_sum = open(sum_file, "w")
    sumcsv = csv.writer(f_sum, dialect="excel")
    sumcsv.writerow(["metrics", "avg"])
    if EXCEL_OUT:
        OUT_WORKBOOK.writerow(0, ["metrics", "avg"], "r")

    for i in range(len(sum_row) - 1):
        sumcsv.writerow([final_out_row0[i + 1], int(sum_row[i + 1] / row_count)])
        if EXCEL_OUT:
            OUT_WORKBOOK.writerow(
                i + 1, [final_out_row0[i + 1], int(sum_row[i + 1] / row_count)], "r"
            )
    f_sum.close()


# delete given file
def deletefile(tempfile):
    if os.path.isfile(tempfile):
        os.remove(tempfile)


# cleanup temp files
def cleanup():
    deletefile(time_dump_file)
    deletefile(output_file)
    deletefile(tmp_socket_file)
    if EXCEL_OUT:
        tempfile = get_extra_out_file(out_metric_file, "r")
        deletefile(tempfile)
        tempfile = get_extra_out_file(out_metric_file, "a")
        deletefile(tempfile)
        tempfile = get_extra_out_file(out_metric_file, "s")
        deletefile(tempfile)
        tempfile = get_extra_out_file(out_metric_file, "sr")
        deletefile(tempfile)
        tempfile = get_extra_out_file(out_metric_file, "sa")
        deletefile(tempfile)
        tempfile = get_extra_out_file(out_metric_file, "c")
        deletefile(tempfile)
        tempfile = get_extra_out_file(out_metric_file, "cr")
        deletefile(tempfile)
        tempfile = get_extra_out_file(out_metric_file, "ca")
        deletefile(tempfile)
        tempfile = out_metric_file[:-4] + "csv"
        deletefile(tempfile)
    tmpdir = script_path + "/_tmp_perf_"
    os.rmdir(tmpdir)


# restrict joining path to same directories
def is_safe_path(base_dir, path, follow_symlinks=True):
    if follow_symlinks:
        match = os.path.realpath(path).startswith(base_dir)
    else:
        match = os.path.abspath(path).startswith(base_dir)
    return base_dir == os.path.commonpath((base_dir, match))


if __name__ == "__main__":

    from argparse import ArgumentParser

    parser = ArgumentParser(description="perf-postprocess: perf post process")
    parser.add_argument(
        "--version", "-v", help="display version information", action="store_true"
    )
    parser.add_argument(
        "-m",
        "--metricfile",
        type=str,
        default=None,
        help="formula file, default metric file for the architecture",
    )
    parser.add_argument(
        "-o",
        "--outfile",
        type=str,
        default=out_metric_file,
        help="perf stat outputs in csv format, default=results/metric_out.csv",
    )
    parser.add_argument(
        "--persocket", help="generate per socket metrics", action="store_true"
    )
    parser.add_argument(
        "--percore", help="generate per core metrics", action="store_true"
    )
    parser.add_argument(
        "--keepall",
        help="keep all intermediate csv files, use it for debug purpose only",
        action="store_true",
    )
    parser.add_argument(
        "--epoch",
        help="time series in epoch format, default is sample count",
        action="store_true",
    )
    required_arg = parser.add_argument_group("required arguments")
    required_arg.add_argument(
        "-r",
        "--rawfile",
        type=str,
        default=None,
        help="Raw CSV output from perf-collect",
    )

    args = parser.parse_args()

    if args.version:
        print(perf_helpers.get_tool_version())
        sys.exit(0)

    if not len(sys.argv) > 2:
        parser.print_help()
        sys.exit()

    script_path = os.path.dirname(os.path.realpath(__file__))
    # fix the pyinstaller path
    if "_MEI" in script_path:
        script_path = script_path.rsplit("/", 1)[0]

    temp_dir = script_path + "/_tmp_perf_"
    # create tmp dir
    if not os.path.exists(temp_dir):
        os.mkdir(temp_dir)

    dat_file = args.rawfile
    # default output file
    if args.outfile == out_metric_file:
        res_dir = script_path + "/results"
        if not os.path.exists(res_dir):
            os.mkdir(res_dir)
            perf_helpers.fix_path_ownership(res_dir)
    if args.outfile:
        out_metric_file = args.outfile
    if args.metricfile:
        metric_file = args.metricfile
    if not os.path.isfile(dat_file):
        raise SystemExit("perf dat file not found")

    if not perf_helpers.validate_outfile(args.outfile, True):
        raise SystemExit(
            "Output filename not accepted. Filename should be a .csv without special characters"
        )
    if not perf_helpers.check_file_writeable(args.outfile):
        raise SystemExit("Output file %s not writeable " % args.outfile)
    if (args.outfile).endswith("xlsx"):
        try:
            import xlsxwriter
        except:
            raise SystemExit(
                "xlsxwriter not found to generate excel output. Install xlsxwriter or use .csv"
            )
        EXCEL_OUT = True

    # parse header
    get_metadata()

    if not metric_file:
        if CONST_ARCH == "broadwell":
            metric_file = "metric_bdx.json"
        elif CONST_ARCH == "skylake" or CONST_ARCH == "cascadelake":
            metric_file = "metric_skx_clx.json"
        elif CONST_ARCH == "icelake":
            metric_file = "metric_icx.json"
        else:
            raise SystemExit("Suitable metric file not found")

        # Convert path of json file to relative path if being packaged by pyInstaller into a binary
        if getattr(sys, "frozen", False):
            basepath = getattr(
                sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__))
            )
            if is_safe_file(metric_file, ".json"):
                metric_file = os.path.join(basepath, metric_file)
        elif __file__:
            metric_file = script_path + "/events/" + metric_file
        else:
            raise SystemExit("Unknown application type")

    if not os.path.isfile(metric_file):
        raise SystemExit("metric file not found %s" % metric_file)

    percore_output = False
    persocket_output = False
    # check if detailed socket and core level data can be generated
    if args.percore or args.persocket:
        if PERCORE_MODE:
            persocket_output = True
            if args.percore:
                percore_output = True
        else:
            print(
                "Warning: Generating system level data only. Run perf-collect with --percore to generate socket/core level data."
            )

    if EXCEL_OUT:
        OUT_WORKBOOK.initialize(args.outfile, persocket_output, percore_output)

    samples = write_perf_tmp_output(args.epoch)

    # levels: 0->system 1->socket 2->core
    if percore_output or persocket_output:
        write_socket_view(1, samples)
        if load_metrics(1):
            write_summary(1)
        if percore_output:
            write_core_view()
            if load_metrics(2):
                write_summary(2)
        write_socket2system()
    else:
        if PERCORE_MODE:
            write_socket_view(0, samples)
            write_socket2system()
        else:
            write_system_view()
    if load_metrics():
        write_summary()
    if not args.keepall:
        cleanup()
    if EXCEL_OUT:
        OUT_WORKBOOK.close()
    print("Post processing done, result file:%s" % args.outfile)
    if "res_dir" in locals():
        perf_helpers.fix_path_ownership(res_dir, True)
