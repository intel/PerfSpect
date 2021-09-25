
#! /usr/bin/python

###########################################################################################################
# Copyright (C) 2021 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################


import os
import re
import tarfile
import subprocess #nosec


def _run_command(command, cwd=""):
    proc = subprocess.Popen(command, stdout=subprocess.PIPE, cwd=cwd)   #nosec
    try:
        stdout, stderr = proc.communicate(timeout=10)
    except:
        proc.kill()
        stdout, stderr = proc.communicate()
    retcode = proc.returncode
    return stdout, stderr, retcode

def _test_run_regex(
    command, expected_retcode, capsys, stdout_regex=None, stderr_regex=None, regex_flags=0, cwd="perfspect" 
):

    stdout, stderr, retcode = _run_command(command, cwd)
    assert retcode == expected_retcode
    if stderr_regex:
        captured = capsys.readouterr()
        assert re.search(stderr_regex, captured.err, regex_flags)
    if stdout_regex:
        if stderr_regex == "These events are not supported":
            print("cwd",cwd)
        stdout = stdout.decode("UTF-8")
        assert re.search(stdout_regex, stdout, regex_flags)


def _test_run_output(command, expected_output_extensions):
    proc = subprocess.Popen(command, stdout=subprocess.PIPE)    #nosec
    try:
      stdout, stderr = proc.communicate(timeout=10)
    except:
        proc.kill()
        stdout, stderr = proc.communicate()
    retcode = proc.returncode    
    assert retcode == 0
    # get output dir from stdout
    matches = re.findall(r"Output archive: (.*)", stdout)
    assert matches
    relative_output_tar = matches[0]
    assert os.path.exists(relative_output_tar)
    # make sure all expected files are present
    tar = tarfile.open(relative_output_tar, "r")
    extension_present = {k: False for k in expected_output_extensions}
    for filename in tar.getnames():
        ext = os.path.splitext(filename)[1]
        if ext in extension_present.keys():
            extension_present[ext] = True
    for ext in extension_present.keys():
        assert extension_present[ext]


def test_version(capsys):
    _test_run_regex(
        ["./perf-collect", "--version"],
        0, 
        capsys,
        stdout_regex=r"^[0-9]*.[0-9]*.[0-9]*$",
    )
    _test_run_regex(
        ["./perf-postprocess", "--version"],
        0, 
        capsys,
        stdout_regex=r"^[0-9]*.[0-9]*.[0-9]*$",
    )


def test_help_collect(capsys):
    _test_run_regex(["./perf-collect", "--help"], 0, capsys, stdout_regex=r"optional arguments:")


def test_help2_collect(capsys):
    _test_run_regex(["./perf-collect", "-h"], 0, capsys, stdout_regex=r"optional arguments:")

def test_help_postprocess(capsys):
    _test_run_regex(["./perf-postprocess", "--help"], 0, capsys, stdout_regex=r"optional arguments:")

def test_help2_postprocess(capsys):
    _test_run_regex(["./perf-postprocess", "-h"], 0, capsys, stdout_regex=r"optional arguments:")

