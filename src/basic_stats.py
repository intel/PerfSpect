###########################################################################################################
# Copyright (C) 2020-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

import plotly.graph_objects as go
import plotly
import pandas as pd
import os
from yattag import Doc
import tempfile


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


def get_stats_plot(input, arch):
    try:
        df = pd.read_csv(input, keep_default_na=False)
    except FileNotFoundError:
        raise SystemExit(f"{input} file not found")
    fig_list = []
    if "metric_CPU operating frequency (in GHz)" in df.columns:
        fig1 = get_fig(
            df,
            y=["metric_CPU operating frequency (in GHz)"],
            title="CPU Operating Frequency",
            title_text="Freq (GHz)",
            name=["Frequency"],
        )
        fig_list.append(fig1)
    if "metric_CPU utilization %" in df.columns:
        fig2 = get_fig(
            df,
            y=["metric_CPU utilization %", "metric_CPU utilization% in kernel mode"],
            title="CPU Utilization",
            title_text="Percentage",
            name=["User", "Kernel"],
        )
        fig_list.append(fig2)
    if "metric_CPI" in df.columns:
        fig3 = get_fig(
            df,
            y=["metric_CPI", "metric_kernel_CPI"],
            title="CPI",
            title_text="CPI",
            name=["CPI", "Kernel CPI"],
        )
        fig_list.append(fig3)
    if "metric_package power (watts)" in df.columns:
        fig4 = get_fig(
            df,
            y=["metric_package power (watts)", "metric_DRAM power (watts)"],
            title="Power",
            title_text="Watts",
            name=["Package", "DRAM"],
        )
        fig_list.append(fig4)
    if "metric_memory bandwidth read (MB/sec)" in df.columns:
        fig5 = get_fig(
            df,
            y=[
                "metric_memory bandwidth read (MB/sec)",
                "metric_memory bandwidth write (MB/sec)",
                "metric_memory bandwidth total (MB/sec)",
            ],
            title="Memory Bandwidth",
            title_text="MB/sec",
            name=["Read", "Write", "Total"],
        )
        fig_list.append(fig5)
    if "metric_core % cycles in non AVX license" in df.columns and arch != "broadwell":
        fig6 = get_fig(
            df,
            y=[
                "metric_core % cycles in non AVX license",
                "metric_core % cycles in AVX2 license",
                "metric_core % cycles in AVX-512 license",
            ],
            title="AVX Percentage",
            title_text="Percentage",
            name=["AVX", "AVX2", "AVX512"],
        )
        fig_list.append(fig6)
    if "metric_NUMA %_Reads addressed to local DRAM" in df.columns:
        fig7 = get_fig(
            df,
            y=[
                "metric_NUMA %_Reads addressed to local DRAM",
                "metric_NUMA %_Reads addressed to remote DRAM",
            ],
            title="NUMA Locality DRAM Reads %",
            title_text="Percentage",
            name=["Local", "Remote"],
        )
        fig_list.append(fig7)
    if "metric_TMAM_Frontend_Bound(%)" in df.columns:
        fig8 = get_fig(
            df,
            y=["metric_TMAM_Frontend_Bound(%)", "metric_TMAM_Backend_bound(%)"],
            title="TMA",
            title_text="Percentage",
            name=["TMA_Frontend", "TMA_Backend"],
        )
        fig_list.append(fig8)

    cache_mpi = [
        "metric_L1D MPI (includes data+rfo w/ prefetches)",
        "metric_L2 MPI (includes code+data+rfo w/ prefetches)",
        "metric_LLC data read MPI (demand+prefetch)",
    ]

    if "metric_L1D MPI (includes data+rfo w/ prefetches)" in df.columns:
        fig9 = get_fig(
            df,
            y=cache_mpi,
            title="Cache MPI",
            title_text="MPI",
            name=["L1D MPI", "L2 MPI", "LLC MPI"],
        )
        fig_list.append(fig9)

    figure_list = fig_list
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
