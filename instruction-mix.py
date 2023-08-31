#!/usr/bin/env python3

###########################################################################################################
# Copyright (C) 2021-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################

import logging
import os
import platform
import shlex
import subprocess
import sys

from iced_x86 import Decoder, Formatter, FormatterSyntax  # type: ignore
from src.common import configure_logging, crash
from src.perf_helpers import get_perf_list

formatter = Formatter(FormatterSyntax.NASM)


def get_insn(insn):
    global formatter
    insn = bytes.fromhex(insn.replace(" ", ""))
    insnraw = formatter.format(Decoder(64, insn).decode()).split(" ", 1)
    insn = insnraw[0]
    reg = []
    if len(insnraw) > 1:
        if "xmm" in insnraw[1]:
            reg.append("AVX128")
        if "ymm" in insnraw[1]:
            reg.append("AVX256")
        if "zmm" in insnraw[1]:
            reg.append("AVX512")
        if len(reg) > 0:
            insn += ";" + ",".join(reg)
    return insn


if __name__ == "__main__":
    configure_logging(".")

    if len(sys.argv) != 2 or not sys.argv[1].isdigit():
        print('usage: "sudo ./instruction-mix 3  # run for 3 seconds"')
        sys.exit()
    if os.geteuid() != 0:
        crash("Must run as root, please re-run")
    if platform.system() != "Linux":
        crash("PerfSpect currently supports Linux only")
    get_perf_list()

    logging.info("collecting...")

    subprocess.run(shlex.split("perf record -a -F 99 sleep " + sys.argv[1]))
    rawdata = (
        subprocess.Popen(
            shlex.split("perf script -F comm,pid,insn"),
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        .communicate()[0]
        .decode()
        .split("\n")[:-1]
    )

    logging.info("postprocessing...")

    processmap = {}

    for row in rawdata:
        sides = row.split("insn:")
        if len(sides) > 1:
            insn = get_insn(sides[1])
            id = sides[0].split()[0] + ";" + insn
            if id not in processmap:
                processmap[id] = 0
            processmap[id] += 1

    # generate freqs
    col = ""
    for p in processmap:
        col += p + " " + str(processmap[p]) + "\n"

    with open("instruction-mix.svg", "w") as f:
        subprocess.run(
            shlex.split(
                os.path.join(
                    getattr(
                        sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__))
                    ),
                    "flamegraph.pl",
                )
            ),
            input=col.encode(),
            stdout=f,
        )

    os.chmod("instruction-mix.svg", 0o666)
    logging.info("generated instruction-mix.svg")
