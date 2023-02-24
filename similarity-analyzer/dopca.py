#!/usr/bin/env python3

###########################################################################################################
# Copyright (C) 2021-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

import os
import sys
import logging
import subprocess  # nosec
from argparse import ArgumentParser
import pandas as pd
import numpy as np


def verify_args(arg):
    if not arg.files:
        parser.print_help()
        logger.error("files is a required field")
        exit(1)
    basepath = os.getcwd()
    outfilecsv = os.path.join(basepath, arg.out + ".csv")

    if os.path.exists(outfilecsv):
        logger.warning(f"The {outfilecsv} exists already!")
        raise SystemExit()


def get_version():
    basepath = os.getcwd()
    version_file = os.path.join(basepath, "_version.txt")
    if os.access(version_file, os.R_OK):
        with open(version_file) as f:
            version = f.readline()
    else:
        raise SystemError("version file isn't accessible")

    return version


def setup_custom_logger(name, debug):
    formatter = logging.Formatter(
        fmt="%(asctime)s %(levelname)-8s %(message)s", datefmt="%Y-%m-%d %H:%M:%S"
    )
    handler = logging.FileHandler("log.txt", mode="w")
    handler.setFormatter(formatter)
    screen_handler = logging.StreamHandler(stream=sys.stdout)
    screen_handler.setFormatter(formatter)
    logger = logging.getLogger(name)
    if debug:
        logger.setLevel(logging.DEBUG)
    else:
        logger.setLevel(logging.INFO)
    logger.addHandler(handler)
    logger.addHandler(screen_handler)
    return logger


def dopca(dataset, colnames, n_components, cols):
    # lazy loading
    from sklearn.preprocessing import StandardScaler
    from sklearn.decomposition import PCA

    logger.info("starting PCA")
    # Preprocessing and separating dimensions
    logger.debug(f"deleting colnames {colnames[0]}")
    del colnames[0]
    num_val = dataset.loc[:, colnames].values

    # Normalizing the metrics
    num_val = StandardScaler().fit_transform(num_val)
    logger.debug(f"Post normalizing metrics, num_val: {num_val}")

    # PCA analysis, Create PCA model
    pca = PCA(n_components=n_components)

    # Fit transform function
    principal_components = pca.fit_transform(num_val)
    principal_df = pd.DataFrame(
        data=principal_components,
        columns=["PC" + str(i) for i in range(1, n_components + 1)],
    )
    logger.debug(f"explained variance ratio: {pca.explained_variance_ratio_}")
    metric_df = pd.DataFrame(cols, columns=["Metric"])

    # Concatenating the dataframe along axis = 1
    final_dataframe = pd.concat([principal_df, metric_df], axis=1)
    logger.debug(f"principalDF:\n\n {principal_df}")
    logger.debug(f"finalDF:\n\n {final_dataframe}")

    # Get benchmarks = TMA(max) values from raw data
    df1 = dataset[["metric_TMA_Frontend_Bound(%)", "metric_TMA_Backend_Bound(%)"]]
    df1 = df1 / df1.max()
    # print(df1)
    fe_max = df1.loc[df1["metric_TMA_Frontend_Bound(%)"] == 1].index.tolist()[0]
    be_max = df1.loc[df1["metric_TMA_Backend_Bound(%)"] == 1].index.tolist()[0]
    # print(fe_max,be_max)
    # Get benchmarks = PC(max) values from PCA data
    # print(final_dataframe)
    pc1_max = final_dataframe.iloc[final_dataframe["PC1"].idxmax()]["Metric"]
    pc2_max = final_dataframe.iloc[final_dataframe["PC2"].idxmax()]["Metric"]
    # Compare benchmarks from TMA (max) and PC(max)
    if (fe_max == pc1_max) and (be_max == pc2_max):
        # print(fe_max, "is along +ve X axis")
        # print(be_max, "is along +ve Y axis")
        xlabel = "FrontEnd Bound -->"
        ylabel = "BackEnd Bound -->"
    elif (be_max == pc1_max) and (fe_max == pc2_max):
        # print(be_max, "is along +ve X axis")
        # print(fe_max, "is along +ve Y axis")
        xlabel = "BackEnd Bound -->"
        ylabel = "FrontEnd Bound -->"
    else:
        # print("Axis max's not found")
        xlabel = "PC1"
        ylabel = "PC2"
    logger.info("PCA completed")
    return final_dataframe, xlabel, ylabel


# Plotting along PCs
def plotpca(rownames, dataframe, xlabel, ylabel):
    # lazy loading
    from matplotlib import pyplot as plt
    from matplotlib import cm as cm

    logger.info("PCA plot initiated")
    fig = plt.figure(figsize=(8, 8))
    plot = fig.add_subplot(1, 1, 1)
    plot.set_xlabel("Principal Component 1", fontsize=15)
    plot.set_ylabel("Principal Component 2", fontsize=15)
    plot.set_title("Similarity Analyzer", fontsize=20)

    xs = np.arange(len(rownames))
    ys = [i + xs + (i * xs) ** 2 for i in range(len(rownames))]
    colors = cm.rainbow(np.linspace(0, 1, len(ys)))
    for target, color in zip(rownames, colors):
        indicesToKeep = dataframe["Metric"] == target
        pc1 = dataframe.loc[indicesToKeep, "PC1"]
        pc2 = dataframe.loc[indicesToKeep, "PC2"]
        plot.scatter(pc1, pc2, c=color.reshape(1, -1), s=50)
        # plot.text(pc1 - 0.03, pc2 - 0.03, target, fontsize=9)
        plot.annotate(target, (pc1, pc2))
    # plt.xlabel("PC1", fontsize=8)
    # plt.ylabel("PC2", fontsize=8)
    plt.xlabel(xlabel, fontsize=12, labelpad=10)
    plt.ylabel(ylabel, fontsize=12, labelpad=10)
    plot.grid()
    plt.savefig(outfile)
    logger.info(f"PCA plot saved at {outfile}")


# To Do: plot PC weights and variation along other PCs
if __name__ == "__main__":
    parser = ArgumentParser(description="Similarity Analyzer")
    required_arg = parser.add_argument_group("required arguments")
    required_arg.add_argument(
        "-f", "--files", type=str, default=None, help='excel files delimited by ","'
    )
    parser.add_argument(
        "-p",
        "--postprocessType",
        type=str,
        default="perfspect",
        help="pmu postprocessing tool used (perfspect)",
    )
    parser.add_argument(
        "-o", "--out", type=str, default="sim_workload", help="output file name"
    )
    parser.add_argument(
        "-d", "--debug", dest="debug", default=False, action="store_true"
    )
    parser.add_argument(
        "-v", "--version", help="prints the version of the tool", action="store_true"
    )
    parser.add_argument(
        "-m",
        "--march",
        help="plot pca against reference SPECcpu2017 (int_rate) components based on architecture specified",
    )

    args = parser.parse_args()

    logger = setup_custom_logger("similarity_analyzer", args.debug)
    if args.version:
        print(get_version())
        sys.exit(0)
    logger.info("starting similarity analyzer " + get_version())
    verify_args(args)

    if args.march:
        import glob

        if args.postprocessType == "perfspect":
            spec_profiles = glob.glob("Reference/" + args.march + "/*.csv")
        else:
            spec_profiles = glob.glob("Reference/" + args.march + "/*.xlsx")

        for spec in spec_profiles:
            args.files += "," + spec
        logger.debug("The files being compared are: " + args.files)

    try:
        component_size = len(args.files.split(","))
        if component_size == 0 or component_size == 1:
            logger.error(
                f"The number of components requested is {component_size}, a minimum of 2 is required... Exiting"
            )
            raise Exception
    except Exception as e:
        print(e)
        raise SystemExit(
            'Minimum of 2 input files required and must contain "," delimiter between them'
        )

    # Integrated data_formatter
    cmd = []
    cmd.append("python3")
    cmd.append("data_formatter/main.py")
    cmd.append("-f")
    cmd.append(args.files)
    cmd.append("-m")
    cmd.append("d")
    cmd.append("-o")
    cmd.append(args.out + ".csv")
    if args.postprocessType == "perfspect":
        cmd.append("-p")

    logger.debug(f"The command used by data formatter: {cmd}")
    logger.info(
        f"Initiating data_formatter with {args.postprocessType} pmu postprocessor"
    )
    process = subprocess.Popen(  # nosec
        cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE
    )
    out, err = process.communicate()
    if err:
        logger.error(err.decode())
        exit(1)
    if "Data compared and stored" in str(out):
        logger.info(f"data formatter collated pmu metrics at {args.out}.csv file")
    else:
        logger.error(
            "data formatter wasn't able to collate all the pmu metrics from input files"
        )

    # Ingest data_formatted output into pandas for PCA
    data = args.out + ".csv"
    outfile = args.out + ".png"
    pd_data = pd.read_csv(data)
    pd_data = pd_data.rename(
        columns={i: i[14:] for i in pd_data.columns if i.startswith("Reference")}
    )
    pd_data = pd_data.rename(
        columns={i: i[:-4] for i in pd_data.columns if i != "Metric"}
    )
    logger.debug(f"dataset before transpose:\n {pd_data}")
    columns = list(pd_data.columns)
    columns.remove("Metric")
    pd_data.set_index("Metric", inplace=True)
    pd_data = pd_data.T
    pd_data.insert(loc=0, column="metric", value=columns)
    logger.debug(f"dataset post transpose:\n {pd_data}")
    column_names = pd_data.columns.tolist()
    row_names = pd_data.iloc[:, 0].values.tolist()
    final_df, xlabel, ylabel = dopca(pd_data, column_names, component_size, columns)
    plotpca(row_names, final_df, xlabel, ylabel)
