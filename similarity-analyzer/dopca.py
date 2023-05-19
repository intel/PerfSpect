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
from pca import pca
import matplotlib.pyplot as plt
from matplotlib import cm as colmgr
from sklearn.preprocessing import StandardScaler
from sklearn.decomposition import PCA
from scipy.cluster import hierarchy


def verify_args(parser, args):
    if not args.files:
        parser.print_help()
        logger.error("files is a required field")
        sys.exit(1)
    if args.march and args.march not in ("CLX", "ICX"):
        logger.warning(f"The current released version doesn't support {args.march}")
        parser.print_help()
        sys.exit(1)
    try:
        print(args.files)
        args.files = args.files.split(",")
        print(args.files)
        if "" in args.files:
            logger.error("File name cannot be null/empty string")
            sys.exit(1)
        component_size = len(args.files)
        if component_size in (0, 1) and not args.march:
            logger.error(
                f"The number of components requested is {component_size}, a minimum of 2 is required..."
            )
            raise Exception
    except Exception as invalid_comp_size:
        raise SystemExit(
            'Minimum of 2 input files required and must contain "," delimiter between them'
        ) from invalid_comp_size
    if args.label:
        if "" in args.label:
            logger.error("label cannot be null/empty string")
            parser.print_help()
            sys.exit(1)
        if component_size != len(args.label):
            logger.warning(
                f"The size of labels {args.label} don't match with input files {args.files}"
            )
            parser.print_help()
            sys.exit(1)
    return component_size


def get_version():
    basepath = os.getcwd()
    version_file = os.path.join(basepath, "_version.txt")
    if os.access(version_file, os.R_OK):
        with open(version_file) as vfile:
            version = vfile.readline()
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
    custom_logger = logging.getLogger(name)
    if debug:
        custom_logger.setLevel(logging.DEBUG)
    else:
        custom_logger.setLevel(logging.INFO)
    custom_logger.addHandler(handler)
    custom_logger.addHandler(screen_handler)
    return custom_logger


def handle_nan(data, comp_size):
    logger.debug("Checking for NaN in telemetry input files")
    df = pd.DataFrame(data).fillna(0)
    deleted_workload_profiles = []
    if not df.isnull().values.any():
        logger.debug("No NaN found in telemetry input files")
    else:
        logger.warning("NaN found in the input telemetry files, attempting to fix them")
        df_thresh_nan = df.dropna(thresh=0.8 * len(df.columns))
        diff_df = pd.merge(df, df_thresh_nan, how="outer", indicator="Exist")
        diff_df = diff_df.loc[diff_df["Exist"] != "both"]
        deleted_row_indices = diff_df.index.tolist()
        if deleted_row_indices:
            if len(deleted_row_indices) in (comp_size, comp_size - 1):
                # too many workload profiles have NaN greater than threshold, must quit similarity analysis
                logger.error(
                    "Attempted dropping of NaNs resulted in fewer #input profiles without NaN....quiting similarity analysis"
                )
                sys.exit(1)
            logger.warning(
                "The following input files contain NaN and will no longer be considered for similarity analysis"
            )
            inp_files = args.files
            for row in deleted_row_indices:
                for index, filename in enumerate(inp_files):
                    if row == index:
                        comp_size = comp_size - 1
                        logger.warning(f"{filename}")
                        if args.label:
                            deleted_workload_profiles.append(args.label[index])
                        else:
                            deleted_workload_profiles.append(filename)
            df = data = df_thresh_nan
        if df.isnull().values.any():
            logger.debug(
                f"A total of {df.isnull().sum().sum()} NaN found in your telemetry files and these will be replaced with large negative number"
            )
            data = df.fillna(-99999)
    return data, df.shape[0], deleted_workload_profiles


def add_dimension_to_data(dataset, metric_name, metric_names):
    new_vec = [0] * len(metric_names)
    new_vec[metric_names.index(metric_name)] = 100
    dataset.loc[len(dataset.index)] = new_vec


def dopca(org_dataset, metric_names, org_workload_names, dimensions):
    workload_names = org_workload_names.copy()
    # Make a coupy of dataset
    dataset = org_dataset.copy()
    dataset.columns = metric_names
    print(dataset)
    print(workload_names)
    vec = [0] * len(metric_names)
    print(vec)
    print(len(vec))
    dataset.loc[len(dataset.index)] = vec
    workload_names.append("Origin")

    for d in dimensions:
        add_dimension_to_data(dataset, d[1], metric_names)
        workload_names.append(d[0])
    dataset.index = workload_names
    print("after adding dimensions")
    print(dataset)
    logger.info("starting PCA")
    # Cleaning and separating dimensions
    num_val = dataset.loc[:, metric_names].values
    num_val, n_components, del_rows = handle_nan(num_val, 2)
    if del_rows:
        for profiles in del_rows:
            try:
                workload_names.remove(profiles)
            except ValueError as e:
                logger.error(e)
                sys.exit(1)
    # Normalizing the metrics
    num_val = StandardScaler().fit_transform(num_val)
    logger.debug(f"Post normalizing metrics, num_val: {num_val}")
    # To scale to any number of workloads, generate PCAs equivalent to minimum between workloads and features
    n_components = min(len(num_val), len(metric_names))
    pca = PCA(n_components=n_components)
    # Transform
    principal_components = pca.fit_transform(num_val)
    principal_df = pd.DataFrame(
        data=principal_components,
        columns=["PC" + str(i) for i in range(1, n_components + 1)],
    )
    logger.debug(f"explained variance ratio: {pca.explained_variance_ratio_}")
    metric_df = pd.DataFrame(workload_names, columns=["Metric"])
    # concatenating the dataframe along axis = 1
    final_dataframe = pd.concat([principal_df, metric_df], axis=1)
    logger.debug(f"principalDF:\n\n {principal_df}")
    logger.debug(f"finalDF:\n\n {final_dataframe}")
    logger.info("PCA completed")
    return final_dataframe


def do_hierarchy(dataset, features_names, workload_names, outfile_hierarchy):
    Y = hierarchy.linkage(dataset)
    print("hierarchy")
    print(dataset)
    print(workload_names)
    _ = hierarchy.dendrogram(
        Y, labels=workload_names, show_leaf_counts=True, leaf_rotation=90
    )
    plt.ylabel("distance")
    plt.savefig(outfile_hierarchy)
    plt.clf()
    logger.info(f"Hierarchy plot saved at {outfile_hierarchy}")


def dopca_density(dataset, features_names, workload_names, outfile_pca2):
    logger.info("starting PCA-2")

    # Initialize
    model = pca(normalize=True)
    # Fit transform and include the column labels and row labels
    dataset = dataset.reset_index(drop=True)
    dataset = dataset.apply(pd.to_numeric, errors="ignore")
    dataset.columns = features_names
    # generate scatter plot with density and workload labels
    results = model.fit_transform(
        dataset, col_labels=features_names, row_labels=["0" for x in workload_names]
    )
    pc_data_frame = results["PC"]
    c = 0
    model.scatter(HT2=True, density=True)
    for index, row in pc_data_frame.iterrows():
        plt.text(row["PC1"], row["PC2"], workload_names[c], fontsize=16)
        c += 1
    plt.savefig(outfile_pca2)
    plt.clf()
    logger.info("PCA-2 completed")
    logger.info(f"PCA_2 plot saved at {outfile_pca2}")
    return results


# plot along PCs
def plotpca(rownames, dataframe, outfile_pca, dimensions):
    logger.info("PCA plot initiated")
    fig = plt.figure(figsize=(8, 8))
    plot = fig.add_subplot(1, 1, 1)
    plot.set_xlabel("Principal Component 1", fontsize=15)
    plot.set_ylabel("Principal Component 2", fontsize=15)
    plot.set_title("Similarity Analyzer", fontsize=20)
    xs = np.arange(len(rownames))
    ys = [i + xs + (i * xs) ** 2 for i in range(len(rownames))]
    colors = colmgr.rainbow(np.linspace(0, 1, len(ys)))
    for target, color in zip(rownames, colors):
        indices_to_keep = dataframe["Metric"] == target
        pc1 = dataframe.loc[indices_to_keep, "PC1"]
        pc2 = dataframe.loc[indices_to_keep, "PC2"]
        plot.scatter(pc1, pc2, c=color.reshape(1, -1), s=50)
        plot.annotate(target, (pc1, pc2))
    plt.xlabel("PC1", fontsize=8)
    plt.ylabel("PC2", fontsize=8)
    plt.grid()
    # add arrows
    origin_vector = dataframe[dataframe["Metric"] == "Origin"]
    for d in dimensions:
        end_vector = dataframe[dataframe["Metric"] == d[0]]
        plt.arrow(
            origin_vector["PC1"].values[0],
            origin_vector["PC2"].values[0],
            3 * (end_vector["PC1"].values[0] - origin_vector["PC1"].values[0]),
            3 * (end_vector["PC2"].values[0] - origin_vector["PC2"].values[0]),
            length_includes_head=True,
            width=0.1,
        )

    plt.savefig(outfile_pca)
    plt.clf()

    logger.info(f"PCA plot saved at {outfile_pca}")


def get_args():
    parser = ArgumentParser(description="Similarity Analyzer")
    required_arg = parser.add_argument_group("required arguments")
    required_arg.add_argument(
        "-f", "--files", type=str, default=None, help='excel files delimited by ","'
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
        help="plot pca against reference SPECcpu2017 (int_rate) components based on architecture specified. Expected values: ICX/CLX",
    )
    parser.add_argument(
        "-l",
        "--label",
        type=str,
        help='label each workload profiles which will be used to plot for similarity analysis; This must map to corresponding input files delimited by ","',
    )
    args = parser.parse_args()
    if args.version:
        print(get_version())
        sys.exit(0)
    if args.label:
        args.label = args.label.split(",")
    print("verifying args")
    verify_args(parser, args)
    print("verifying args done")

    return parser.parse_args()


def format_data_for_PCA(args):
    cmd = []
    cmd.append("python3")
    cmd.append("data_formatter/main.py")
    cmd.append("-f")
    cmd.append(args.files)
    cmd.append("-m")
    cmd.append("d")
    cmd.append("-o")
    cmd.append(args.out + ".csv")
    cmd.append("-p")
    logger.debug(f"The command used by data formatter: {cmd}")
    print(cmd)
    with subprocess.Popen(  # nosec
        cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE
    ) as process:
        out, err = process.communicate()
        if err:
            logger.error(err.decode())
            sys.exit(1)
        if "Data compared and stored" in str(out):
            logger.info(f"data formatter collated pmu metrics at {args.out}.csv file")
        else:
            logger.error(
                "data formatter wasn't able to collate all the pmu metrics from input files"
            )


def get_formatted_data_for_PCA(data_file_path):
    pd_data = pd.read_csv(data_file_path)
    pd_data = pd_data.rename(
        columns={i: i[14:] for i in pd_data.columns if i.startswith("Reference")}
    )
    if not args.label:
        pd_data = pd_data.rename(
            columns={i: i[:-4] for i in pd_data.columns if i != "Metric"}
        )
    elif args.label:
        pd_data = pd_data.rename(
            columns={
                i: str(j)
                for i, j in zip(pd_data.drop(columns="Metric").columns, args.label)
            }
        )
    return pd_data


if __name__ == "__main__":
    args = get_args()
    logger = setup_custom_logger("similarity_analyzer", args.debug)
    logger.info(f"starting similarity analyzer {get_version()}")

    format_data_for_PCA(args)

    data_file_path = args.out + ".csv"
    outfile = args.out + ".png"

    pd_data = get_formatted_data_for_PCA(data_file_path)
    features_names = pd_data["Metric"].tolist()

    pd_data = pd_data.iloc[:, 1:]
    logger.debug(f"dataset before transpose:\n {pd_data}")

    workload_names = list(pd_data.columns)

    pd_data = pd_data.T
    pd_data = pd_data.reset_index(drop=True)

    pd_data.insert(loc=0, column="metric", value=workload_names)
    pd_data.set_index("metric", inplace=True)

    logger.debug(f"dataset post transpose:\n {pd_data}")
    features_index = pd_data.columns.tolist()

    pd_data = pd_data.fillna(0)

    # PCA
    dimensions = [
        ("Front-end", "metric_TMA_Frontend_Bound(%)"),
        ("Back-end", "metric_TMA_Backend_Bound(%)"),
    ]
    final_df = dopca(pd_data, features_names, workload_names, dimensions)
    row_names = final_df["Metric"].values
    outfile_pca = args.out + "_pca.png"
    plotpca(row_names, final_df, outfile_pca, dimensions)

    # PCA with density
    outfile_pca_density = args.out + "_pca_density.png"
    final_df = dopca_density(
        pd_data, features_names, workload_names, outfile_pca_density
    )

    # Hierarchy
    outfile_hierarchy = args.out + "_hierarchy.png"
    do_hierarchy(pd_data, features_index, workload_names, outfile_hierarchy)
