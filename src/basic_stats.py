#!/usr/bin/env python3

###########################################################################################################
# Copyright (C) 2020-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

import os
import pandas as pd
import plotly
import plotly.graph_objects as go
import tempfile
from yattag import Doc
from src.common import crash
from collections import OrderedDict


os.environ["MPLCONFIGDIR"] = tempfile.mkdtemp()
doc, tag, text = Doc().tagtext()


def get_fig(df, y, name, title, title_text):
    # Plot row 1 col 1
    fig = go.Figure()
    for i in range(len(y)):
        if y[i] not in df.columns:
            continue
        fig.add_trace(
            go.Scatter(
                x=df["time"],
                y=df[y[i]],
                name=name[i],
                showlegend=True,
            )
        )
    fig.update_layout(title=title)
    fig.update_yaxes(title_text=title_text)
    if y[0] == "metric_CPU utilization %":
        fig.update_layout(yaxis_range=[0, 100])
    return fig


def get_row_header():
    fig = '<div class="row">'
    return fig


def get_row_footer():
    fig = "</div>"
    return fig


def get_col(html_list):
    col_start = '<div class="col" style="float: left; width: 33%; height: 45%;">'
    col_end = "</div>"
    fig = col_start + html_list.pop(0) + col_end
    return fig


def row_of_3(html_list):
    return (
        get_row_header()
        + get_col(html_list)
        + get_col(html_list)
        + get_col(html_list)
        + get_row_footer()
    )


def row_of_2(html_list):
    return get_row_header() + get_col(html_list) + get_col(html_list) + get_row_footer()


def row_of_1(html_list):
    return get_row_header() + get_col(html_list) + get_row_footer()


def get_stats_plot(input_file, arch):
    try:
        df = pd.read_csv(input_file, keep_default_na=False)

    except FileNotFoundError:
        crash(f"{input} file not found")
    figure_to_column_dict = OrderedDict()
    figure_to_column_dict["CPU Operating Frequency"] = {
        "metrics_prefixes": ["metric_CPU operating frequency (in GHz)"],
        "Y_axis_text": "Freq (GHz)",
        "name_prefix": ["Frequency"],
    }
    figure_to_column_dict["CPU Utilization"] = {
        "metrics_prefixes": [
            "metric_CPU utilization %",
            "metric_CPU utilization% in kernel mode",
        ],
        "Y_axis_text": "Percentage",
        "name_prefix": ["User", "Kernel"],
    }
    figure_to_column_dict["CPI"] = {
        "metrics_prefixes": ["metric_CPI", "metric_kernel_CPI"],
        "Y_axis_text": "CPI",
        "name_prefix": ["CPI", "Kernel CPI"],
    }
    figure_to_column_dict["Power"] = {
        "metrics_prefixes": [
            "metric_package power (watts)",
            "metric_DRAM power (watts)",
        ],
        "Y_axis_text": "Watts",
        "name_prefix": ["Package", "DRAM"],
    }
    figure_to_column_dict["Memory Bandwidth"] = {
        "metrics_prefixes": [
            "metric_memory bandwidth read (MB/sec)",
            "metric_memory bandwidth write (MB/sec)",
            "metric_memory bandwidth total (MB/sec)",
        ],
        "Y_axis_text": "MB/sec",
        "name_prefix": ["Read", "Write", "Total"],
    }
    figure_to_column_dict["AVX Percentage"] = {
        "metrics_prefixes": [
            "metric_core % cycles in non AVX license",
            "metric_core % cycles in AVX2 license",
            "metric_core % cycles in AVX-512 license",
        ],
        "Y_axis_text": "Percentage",
        "name_prefix": ["AVX", "AVX2", "AVX512"],
    }
    figure_to_column_dict["NUMA Locality DRAM Reads %"] = {
        "metrics_prefixes": [
            "metric_NUMA %_Reads addressed to local DRAM",
            "metric_NUMA %_Reads addressed to remote DRAM",
        ],
        "Y_axis_text": "Percentage",
        "name_prefix": ["Local", "Remote"],
    }
    figure_to_column_dict["TMA"] = {
        "metrics_prefixes": [
            "metric_TMA_Frontend_Bound(%)",
            "metric_TMA_Backend_Bound(%)",
        ],
        "Y_axis_text": "Percentage",
        "name_prefix": ["TMA_Frontend", "TMA_Backend"],
    }
    figure_to_column_dict["Cache MPI"] = {
        "metrics_prefixes": [
            "metric_L1D MPI (includes data+rfo w/ prefetches)",
            "metric_L2 MPI (includes code+data+rfo w/ prefetches)",
            "metric_LLC data read MPI (demand+prefetch)",
        ],
        "Y_axis_text": "MPI",
        "name_prefix": ["L1D MPI", "L2 MPI", "LLC MPI"],
    }

    figure_list = []
    for figure_title in figure_to_column_dict:
        figure_data = figure_to_column_dict[figure_title]
        for metric_index, metric_prefix in enumerate(figure_data["metrics_prefixes"]):
            for column in df.columns:
                if metric_prefix in column:
                    if "cols" not in figure_data:
                        figure_data["cols"] = []
                    if "names" not in figure_data:
                        figure_data["names"] = []
                    figure_data["cols"].append(column)
                    series_name = (
                        figure_data["name_prefix"][metric_index]
                        + "_"
                        + column.replace(metric_prefix, "")
                    )
                    figure_data["names"].append(series_name)
        if "cols" in figure_data:
            fig = get_fig(
                df,
                y=figure_data["cols"],
                title=figure_title,
                title_text=figure_data["Y_axis_text"],
                name=figure_data["names"],
            )
            figure_list.append(fig)

    for fig in figure_list:
        # update layout
        fig.update_layout(
            font=dict(family="Courier New, monospace", size=14, color="Black")
        )

        fig.update_layout(paper_bgcolor="#f0f0f5", plot_bgcolor="white")

        fig.update_layout(
            title_font_family="Open Sans",
            title_font_color="Black",
        )

        fig.update_layout(autosize=True, margin=dict(l=20, r=30, b=20, t=70))

        fig.update_layout(
            legend=dict(
                orientation="h", yanchor="bottom", y=1.01, xanchor="right", x=1.0
            ),
            legend_groupclick="toggleitem",
        )

        # update axes
        # fig.update_yaxes(showline=True, linewidth=2, linecolor="black", mirror=True)
        fig.update_yaxes(rangemode="tozero")
        fig.update_xaxes(showline=True, linewidth=1.5, linecolor="black")
        fig.update_yaxes(showline=True, linewidth=1.5, linecolor="black")
        fig.update_xaxes(showgrid=True, gridwidth=1, gridcolor="LightPink")
        fig.update_yaxes(showgrid=True, gridwidth=1, gridcolor="LightPink")
        fig.update_xaxes(ticks="inside", tickwidth=2, tickcolor="black", ticklen=6)
        fig.update_yaxes(ticks="inside", tickwidth=2, tickcolor="black", ticklen=6)

    html_fig_list = []
    for fig in figure_list:
        html_fig = plotly.offline.plot(fig, include_plotlyjs=False, output_type="div")
        html_fig_list.append(html_fig)

    fig = ""
    while len(html_fig_list) > 0:
        if len(html_fig_list) >= 3:
            fig = fig + row_of_3(html_fig_list)
        if len(html_fig_list) == 2:
            fig = fig + row_of_2(html_fig_list)
        if len(html_fig_list) == 1:
            fig = fig + row_of_1(html_fig_list)

    return fig
