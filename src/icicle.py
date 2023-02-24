###########################################################################################################
# Copyright (C) 2020-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

from yattag import Doc
import plotly.graph_objects as go
import pandas as pd
import numpy as np

doc, tag, text = Doc().tagtext()
metric_parent = {}

""" returns icicle figure with L1, L2, L3 and L4 TMA """


def get_icicle(input_csv):
    L1 = "pipeline"
    L2 = ""
    L3 = ""
    try:
        df = pd.read_csv(input_csv, keep_default_na=False)
    except FileNotFoundError:
        raise SystemExit(f"{input_csv} File not found")
    unwanted = ["%", "metric_TMA_", ".", "(", ")", "metric_TMAM_"]
    df = df.replace("N/A", np.nan)

    TMA = df[df["metrics"].str.startswith("metric_TMA")]

    """ assign parent to each metric """
    for metric in TMA["metrics"]:
        if metric == "pipeline":
            metric_parent[metric] = ""
        if any(
            x in metric.lower()
            for x in ["frontend_bound", "bad_speculation", "backend_bound", "retiring"]
        ):
            metric_parent[metric] = "pipeline"
            L1 = metric
        if metric.count(".") == 2:
            metric_parent[metric] = L1
            L2 = metric
        if metric.count(".") == 4:
            metric_parent[metric] = L2
            L3 = metric
        if metric.count(".") == 6:
            metric_parent[metric] = L3

    """ get parents """
    parent, ignore = get_parents(TMA["metrics"].tolist())
    TMA = TMA[~TMA["metrics"].isin(ignore)]

    """ prepare data """
    tma = TMA.copy()
    for item in unwanted:
        tma["metrics"] = tma["metrics"].str.replace(item, "", regex=False)
    characters = ["pipeline"] + tma["metrics"].tolist()
    parent = [""] + parent
    new_data = []
    # pipeline_avg = TMA[TMA["metrics"] =="metric_TMA_Frontend_Bound(%)"].avg.iloc[0] + TMA[TMA["metrics"] =="metric_TMA_Bad_Speculation(%)"].avg.iloc[0] + TMA[TMA["metrics"] =="metric_TMA_Backend_Bound(%)"].avg.iloc[0] + TMA[TMA["metrics"] =="metric_TMA_Retiring(%)"].avg.iloc[0]
    new_data.insert(
        0, {"metrics": "pipeline", "avg": 100, "p95": 100, "min": 100, "max": 100}
    )
    TMA = pd.concat([pd.DataFrame(new_data), TMA], ignore_index=True)
    TMA["parent"] = parent
    TMA["id"] = characters

    """ plot icicle """
    fig = go.Figure()
    fig.add_trace(
        go.Icicle(
            ids=TMA.id,
            labels=TMA.id,
            parents=TMA.parent,
            root_color="lightgrey",
            tiling=dict(orientation="v"),
        )
    )
    fig.update_traces(
        text=TMA.avg.round(decimals=2),
        textinfo="label+text",
        textposition="top center",
    )

    fig.update_layout(
        # autosize=False,
        # height=300,
        # width=200,
        margin=dict(t=50, l=25, r=25, b=25)
    )
    return fig


def strip_unwanted(metric_name):
    unwanted = ["%", "metric_TMA_", ".", "(", ")", "metric_TMAM_"]
    for char in unwanted:
        metric_name = metric_name.replace(char, "")
    return metric_name


def get_parents(metrics):
    parent = []
    no_parent = []
    for metric in metrics:
        try:
            parent.append(strip_unwanted(metric_parent[metric]))
        except KeyError:
            no_parent.append(metric)
    return parent, no_parent
