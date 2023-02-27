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


def verify_args(args):
    if not args.files:
        parser.print_help()
        logger.error("files is a required field")
        sys.exit(1)
    basepath = os.getcwd()
    outfilecsv = os.path.join(basepath, args.out + ".csv")
    if os.path.exists(outfilecsv):
        logger.warning(f"The {outfilecsv} exists already!")
        sys.exit(1)
    if args.march and args.march not in ("CLX", "ICX"):
        logger.warning(f"The current released version doesn't support {args.march}")
        parser.print_help()
        sys.exit(1)
    try:
        files = args.files.split(",")
        if "" in files:
            logger.error("File name cannot be null/empty string")
            sys.exit(1)
        component_size = len(files)
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
            logger.warning(f"The size of labels {args.label} don't match with input files {args.files}")
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
    df = pd.DataFrame(data)
    deleted_workload_profiles = []
    if not df.isnull().values.any():
        logger.debug("No NaN found in telemetry input files")
    else:
        logger.warning("NaN found in the input telemetry files, attempting to fix them")
        df_thresh_nan = df.dropna(thresh=0.8*len(df.columns))
        diff_df = pd.merge(df, df_thresh_nan, how='outer', indicator='Exist')
        diff_df = diff_df.loc[diff_df['Exist'] != 'both']
        deleted_row_indices = diff_df.index.tolist()
        if deleted_row_indices:
            if len(deleted_row_indices) in (comp_size, comp_size-1):
                #too many workload profiles have NaN greater than threshold, must quit similarity analysis
                logger.error("Attempted dropping of NaNs resulted in fewer #input profiles without NaN....quiting similarity analysis")
                sys.exit(1)
            logger.warning("The following input files contain NaN and will no longer be considered for similarity analysis")
            inp_files = args.files.split(",")
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
            logger.debug(f"A total of {df.isnull().sum().sum()} NaN found in your telemetry files and these will be replaced with large negative number")
            data = df.fillna(-99999)
    return data, df.shape[0], deleted_workload_profiles

def dopca(dataset, colnames, n_components, cols):
    # lazy loading
    from sklearn.preprocessing import StandardScaler
    from sklearn.decomposition import PCA
    logger.info("starting PCA")
    # cleaning and separating dimensions
    logger.debug(f"deleting colnames {colnames[0]}")
    del colnames[0]
    num_val = dataset.loc[:, colnames].values
    num_val, n_components, del_rows = handle_nan(num_val, n_components)
    if del_rows:
        for profiles in del_rows:
            try:
                cols.remove(profiles)
            except ValueError as e:
                logger.error(e)
                sys.exit(1)
    # normalizing the metrics
    num_val = StandardScaler().fit_transform(num_val)
    logger.debug(f"Post normalizing metrics, num_val: {num_val}")
    # PCA analysis, Create PCA model
    #pca = PCA(n_components=n_components) #Limitation: If the n_components(no of workloads) are greater than num_val(the number of features), it will throw error.

    n_components = len(num_val[0]) #Solution: To scale it for any number of workloads, generate PCAs equivalent to number of features/performance matrics (instead of number of workloads) that we have for each workload.
    pca = PCA(n_components=n_components)

    # transform
    principal_components = pca.fit_transform(num_val)
    principal_df = pd.DataFrame(
        data=principal_components,
        columns=["PC" + str(i) for i in range(1, n_components + 1)],
    )
    logger.debug(f"explained variance ratio: {pca.explained_variance_ratio_}")
    metric_df = pd.DataFrame(cols, columns=["Metric"])
    # concatenating the dataframe along axis = 1
    final_dataframe = pd.concat([principal_df, metric_df], axis=1)
    logger.debug(f"principalDF:\n\n {principal_df}")
    logger.debug(f"finalDF:\n\n {final_dataframe}")
    logger.info("PCA completed")
    return final_dataframe

# plot along PCs
def plotpca(rownames, dataframe):
    # lazy loading
    from matplotlib import pyplot as plt
    from matplotlib import cm as colmgr

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
    plot.grid()
    plt.savefig(outfile)
    logger.info(f"PCA plot saved at {outfile}")


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
        help="plot pca against reference SPECcpu2017 (int_rate) components based on architecture specified. Expected values: ICX/CLX",
    )
    parser.add_argument(
        "-l",
        "--label",
        type=str,
        help='label each workload profiles which will be used to plot for similarity analysis; This must map to corresponding input files delimited by ","',
    )
    args = parser.parse_args()
    logger = setup_custom_logger("similarity_analyzer", args.debug)
    if args.version:
        print(get_version())
        sys.exit(0)
    if args.label:
        args.label = args.label.split(",")
    logger.info(f"starting similarity analyzer {get_version()}")
    comp_size = verify_args(args)
    if args.march:
        import glob
        if args.postprocessType == "perfspect":
            spec_profiles = glob.glob("Reference/" + args.march + "/*.csv")
        else:
            logger.error("Similarity Analyzer supports perfspect telemetry data only")
            sys.exit(1)
        for spec in spec_profiles:
            args.files += "," + spec
        logger.debug(f"The files being compared are: {args.files}")
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
    data = args.out + ".csv"
    outfile = args.out + ".png"
    pd_data = pd.read_csv(data)
    pd_data = pd_data.rename(
        columns={i: i[14:] for i in pd_data.columns if i.startswith("Reference")}
    )
    if not args.label and args.postprocessType == "perfspect":
        pd_data = pd_data.rename(
            columns={i: i[:-4] for i in pd_data.columns if i != "Metric"}
        )
    elif args.label and args.postprocessType == "perfspect":
        pd_data = pd_data.rename(
            columns={i: str(j) for i,j in zip(pd_data.drop(columns="Metric").columns, args.label)}
        )
    else:
        logger.error("Similarity Analyzer supports perfspect telemetry data only")
        sys.exit(1)
    logger.debug(f"dataset before transpose:\n {pd_data}")
    columns = list(pd_data.columns)
    columns.remove("Metric")
    pd_data.set_index("Metric", inplace=True)
    pd_data = pd_data.T
    pd_data.insert(loc=0, column="metric", value=columns)
    logger.debug(f"dataset post transpose:\n {pd_data}")
    column_names = pd_data.columns.tolist()
    final_df = dopca(pd_data, column_names, comp_size, columns)
    row_names = final_df["Metric"].values
    plotpca(row_names, final_df)