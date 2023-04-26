#!/usr/bin/env python3

###########################################################################################################
# Copyright (C) 2020-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

import logging
from src import basic_stats
from src import icicle
from yattag import Doc, indent


def write_html(in_file, perf_mode, arch, html_report_out, data_type="both"):
    if data_type not in ("tma", "basic", "both"):
        data_type = "both"
    if str(perf_mode) == "Mode.System":
        tma_inp = in_file.replace(".csv", ".sys.csv")
        tma_inp_avg = in_file.replace(".csv", ".sys.average.csv")
    elif str(perf_mode) == "Mode.Socket":
        tma_inp = in_file.replace(".csv", ".socket.csv")
        tma_inp_avg = in_file.replace(".csv", ".socket.average.csv")
    elif str(perf_mode) == "Mode.Core":
        tma_inp = in_file.replace(".csv", ".core.csv")
        tma_inp_avg = in_file.replace(".csv", ".core.average.csv")

    doc, tag, text = Doc().tagtext()
    with tag("html"):
        with tag("style"):
            text("h1{text-align: center;background-color: #00ccff;}")
            text("h2{text-align: center;background-color: #e6faff;}")
        with tag("head"):
            doc.asis('<script src="https://cdn.plot.ly/plotly-latest.min.js"></script>')
            with tag("h1"):
                text("Intel® PerfSpect Report")
        with tag("body"):
            if data_type in ("both", "tma"):
                fig1 = icicle.get_icicle(tma_inp_avg)
                with tag("h2", align="center"):
                    text("TopDown Microarchitecture Analysis (TMA)")
                with doc.tag("div"):
                    doc.attr(id="tma")
                    doc.asis(fig1.to_html(full_html=False, include_plotlyjs="cdn"))
            if data_type in ("both", "basic"):
                fig2 = basic_stats.get_stats_plot(tma_inp, arch)
                with tag("h2", align="center"):
                    text("Basic Statistics")
                with doc.tag("div"):
                    doc.attr(id="basic_stats")
                    doc.stag("br")
                    doc.asis(fig2)
    result = indent(doc.getvalue())
    with open(html_report_out, "w") as file:
        file.write(result)
    logging.info(f"static HTML file written at {html_report_out}")
