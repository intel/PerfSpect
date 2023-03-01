#!/usr/bin/env python3
###########################################################################################################
# Copyright (C) 2020-2023 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
###########################################################################################################


import os
import re
import sys
import glob
import tarfile
import pytest
import subprocess #nosec


def _run_command(command, cwd=""):
    proc = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, cwd=cwd)   #nosec
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


def _test_run_output(command, cwd, expected_output_extensions,collect=True):
    proc = subprocess.Popen(command, cwd=cwd,stdout=subprocess.PIPE)    #nosec
    try:
      stdout, stderr = proc.communicate(timeout=20)
    except:
        proc.kill()
        stdout, stderr = proc.communicate()
    retcode = proc.returncode    
    assert retcode == 0
    # get output dir from stdout
    if collect:
        matches = re.findall(r"perf stat dumped to (.*)", stdout.decode())
    else:
        matches = re.findall(r"Post processing done, result file:(.*)", stdout.decode())
    assert matches
    relative_output_path = matches[0].split("/")[:-1]
    relative_output_path = '/'.join(relative_output_path)
    assert os.path.exists(relative_output_path)
    # make sure all expected files are present
    # tar = tarfile.open(relative_output_tar, "r")
    extension_present = {k: False for k in expected_output_extensions}
    for filename in glob.iglob(f'{relative_output_path}/*'):
        ext = os.path.splitext(filename)[1]
        if ext in extension_present.keys():
            extension_present[ext] = True
    for ext in extension_present.keys():
        assert extension_present[ext]

# def test_version(capsys):
#     _test_run_regex(
#         ["./perf-collect", "--version"],
#         0,
#         capsys,
#         stdout_regex=r"^[0-9]*.[0-9]*.[0-9]*$",
#     )
#     _test_run_regex(
#         ["./perf-postprocess", "--version"],
#         0,
#         capsys,
#         stdout_regex=r"^[0-9]*.[0-9]*.[0-9]*$",
#     )


def test_help_collect(capsys):
    _test_run_regex(["./perf-collect", "--help"], 0, capsys, stdout_regex=r"optional arguments:")


def test_help2_collect(capsys):
    _test_run_regex(["./perf-collect", "-h"], 0, capsys, stdout_regex=r"optional arguments:")

def test_help_postprocess(capsys):
    _test_run_regex(["./perf-postprocess", "--help"], 0, capsys, stdout_regex=r"optional arguments:")

def test_help2_postprocess(capsys):
    _test_run_regex(["./perf-postprocess", "-h"], 0, capsys, stdout_regex=r"optional arguments:")

def test_run_no_options():
    # no arguments passed
    stdout,_,_ = _run_command(["sudo", "./perf-postprocess"],"perfspect")
    matches = re.findall(r"usage: (.*)", stdout.decode())
    assert matches

    # no HTML filename provided
    _,_,retcode = _run_command(["sudo", "./perf-postprocess" ,"--html"],"perfspect")
    assert retcode == 2

    #no options to pid
    _,_,retcode = _run_command(["sudo", "./perf-collect" ,"--pid"],"perfspect")
    assert retcode == 2

    #no options to cid
    _,_,retcode = _run_command(["sudo", "./perf-collect" ,"--cid"],"perfspect")
    assert retcode == 2

    #no options to app
    _,_,retcode = _run_command(["sudo", "./perf-collect" ,"--app"],"perfspect")
    assert retcode == 2

    #no options to timeout
    _,_,retcode = _run_command(["sudo", "./perf-collect" ,"-t"],"perfspect")
    assert retcode == 2

    #no options to csp
    _,_,retcode = _run_command(["sudo", "./perf-collect" ,"-csp"],"perfspect")
    assert retcode == 2

    #no options to eventfile
    _,_,retcode = _run_command(["sudo", "./perf-collect" ,"-e"],"perfspect")
    assert retcode == 2

    #no options to outcsv
    _,_,retcode = _run_command(["sudo", "./perf-collect" ,"-o"],"perfspect")
    assert retcode == 2




def test_invalid_arguments():
    #perf-collect
    #invalid csp name
    stdout,_,retcode = _run_command(["sudo", "./perf-collect","-csp", "bad_cspname"],"perfspect")
    hit = re.search(r"Invalid csp/cloud", stdout.decode())
    assert hit
    assert retcode == 1

    #invalid eventfile
    stdout,_,retcode = _run_command(["sudo", "./perf-collect","-e", "bad_eventfile"],"perfspect")
    hit = re.search(r"event file not found", stdout.decode())
    assert hit
    assert retcode == 1

    #invalid interval
    stdout,_,retcode = _run_command(["sudo", "./perf-collect","-i", "invalid_interval"],"perfspect")
    hit = re.search(r"invalid float value", stdout.decode())
    assert hit
    assert retcode == 2

    #invalid interval range
    stdout,_,retcode = _run_command(["sudo", "./perf-collect","-i", "-.007"],"perfspect")
    hit = re.search(r"dump interval is too large or too small", stdout.decode())
    assert hit
    assert retcode == 1

    #invalid interval range
    stdout,_,retcode = _run_command(["sudo", "./perf-collect","-i", "1001"],"perfspect")
    hit = re.search(r"dump interval is too large or too small", stdout.decode())
    assert hit
    assert retcode == 1

    #invalid outcsv
    stdout,_,retcode = _run_command(["sudo", "./perf-collect","-o", "bad_outcsv"],"perfspect")
    hit = re.search(r"Output filename not accepted", stdout.decode())
    assert hit
    assert retcode == 1

    #uncomment if testing outside of containers
    #invalid app argument
    # stdout,_,retcode = _run_command(["sudo", "./perf-collect","-a", "bad_app_argument", "-m", "0"],"perfspect")
    # hit = re.search(r"Workload failed", stdout.decode())
    # print("--app",stdout)
    # assert hit

    #invalid pid argument
    # stdout,_,retcode = _run_command(["sudo", "./perf-collect","-p", "bad_pid"],"perfspect")
    # hit = re.search(r"Problems finding threads of monitor", stdout.decode())
    # assert hit

    #invalid cid argument
    # stdout,_,retcode = _run_command(["sudo", "./perf-collect","--cid", "bad_cid"],"perfspect")
    # hit = re.search(r"invalid container ID", stdout.decode())
    # # print("cid",stdout)
    # assert hit
    # assert retcode == 1

    #invalid timeout
    stdout,_,retcode = _run_command(["sudo", "./perf-collect","--timeout", "bad_timeout"],"perfspect")
    # print(stdout)
    hit = re.search(r"invalid int value", stdout.decode())
    assert hit
    assert retcode == 2

    #perf-postprocess
    #invalid HTML filename provided
    stdout,_,retcode = _run_command(["sudo", "./perf-postprocess","-r","../data/perfstat.csv","--html", "bad.filename"],"perfspect")
    matches = re.findall(r"isn't a valid html file (.*)", stdout.decode())
    # print("html_matches",stdout)
    assert matches
    assert retcode == 1

    #invalid metricfile
    stdout,_,retcode = _run_command(["sudo", "./perf-postprocess","-r","../data/perfstat.csv", "-m", "bad_metricfile.name"],"perfspect")
    matches = re.findall(r"metric file not found (.*)", stdout.decode())
    assert matches
    assert retcode == 1


    #invalid raw file
    stdout,_,retcode = _run_command(["sudo", "./perf-postprocess","-r", "bad_raw_file"],"perfspect")
    hit = re.search(r"perf raw data file not found", stdout.decode())
    assert hit
    assert retcode == 1


def test_no_raw_perf_data():
    #invalid perf raw file provided
    stdout,_,_ = _run_command(["sudo", "./perf-postprocess" ,"-r", "perfstat.badfilename"],"perfspect")
    matches = re.findall(r"usage: (.*)", stdout.decode())
    assert matches

    #empty perf raw file provided
    stdout,_,retcode = _run_command(["sudo", "./perf-postprocess" ,"-r", "../data/perfstat_empty.csv"],"perfspect")
    hit = re.search(r"The perf raw file doesn't contain metadata", stdout.decode())
    assert hit
    assert retcode == 1

def test_perf_data_without_metadata():
    #empty perf raw file provided
    stdout,_,retcode = _run_command(["sudo", "./perf-postprocess" ,"-r", "../data/perfstat_without_metadata.csv"],"perfspect")
    hit = re.search(r"The perf raw file doesn't contain metadata", stdout.decode())
    assert hit
    assert retcode == 1

def test_valid_perf_postprocess():
    #empty perf raw file provided
    _test_run_output(["sudo", "./perf-postprocess" ,"-r", "../data/perfstat.csv"],"perfspect",[".csv"], False)
    _test_run_output(["sudo", "./perf-postprocess" ,"-r", "../data/perfstat.csv", "--html", "abc.html"],"perfspect",[".csv", ".html"], False)

# uncomment if testing outside of container
# def test_valid_perf_collect():
#     #empty perf raw file provided
#     _test_run_output(["sudo", "./perf-collect", "-m", "0", "-t", "5"],"perfspect",[".csv"])
