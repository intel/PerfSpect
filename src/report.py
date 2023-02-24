###########################################################################################################
# Copyright (C) 2020-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

from src import basic_stats
from src import icicle
import os


def write_html(res_dir, base_input_file, arch, html_report_out, type="both"):
    if type not in ("tma", "basic", "both"):
        type = "both"
    tma_inp = base_input_file.split(".")[0] + ".average.csv"
    basic_inp = os.path.join(res_dir, base_input_file)
    tma_inp = os.path.join(res_dir, tma_inp)
    from yattag import Doc, indent

    doc, tag, text = Doc().tagtext()
    with tag("html"):
        # ToDO: add navigation later
        # with doc.tag('div'):
        #     doc.attr(klass='navbar')
        #     with tag('a', href="#tma", klass="active"):
        #             text("TMA")
        #     with tag('a', href="#basic_stats"):
        #             text("Basic Stats")

        with tag("style"):
            text("h1{text-align: center;background-color: #00ccff;}")
            text("h2{text-align: center;background-color: #e6faff;}")
        #     text('.navbar {background-color: #333;overflow: hidden;position: fixed;bottom: 0;width: 100%;}')
        #     text('.navbar a {float: left;display: block;color: #f2f2f2;text-align: center;padding: 14px 16px;text-decoration: none;font-size: 17px;}')
        #     text('.navbar a:hover {background-color: #ddd;color: black;}')
        #     text('.navbar a.active {background-color: #04AA6D;color: white;}')
        # text('input{position: fixed;}')

        with tag("head"):
            doc.asis('<script src="https://cdn.plot.ly/plotly-latest.min.js"></script>')
            with tag("h1"):
                text("Intel® PerfSpect Report")
        with tag("body"):
            if type in ("both", "tma"):
                fig1 = icicle.get_icicle(tma_inp)
                with tag("h2", align="center"):
                    text("TopDown Microarchitecture Analysis (TMA)")
                with doc.tag("div"):
                    doc.attr(id="tma")
                    doc.asis(fig1.to_html(full_html=False, include_plotlyjs="cdn"))
            if type in ("both", "basic"):
                fig2 = basic_stats.get_stats_plot(basic_inp, arch)
                with tag("h2", align="center"):
                    text("Basic Statistics")
                with doc.tag("div"):
                    doc.attr(id="basic_stats")
                    doc.stag("br")
                    doc.asis(fig2)
    result = indent(doc.getvalue())
    out_html = os.path.join(res_dir, html_report_out)
    with open(out_html, "w") as file:
        file.write(result)
    print(f"static HTML file written at {out_html}")
