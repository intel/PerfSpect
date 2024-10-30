#!/usr/bin/env python3
# targets2yaml.py - converts a list of targets in the svr-info 2.x format to the YAML file format used by PerfSpect 3.0+.
#
# Usage: targets2yaml.py < targets > targets.yaml

# Copyright (C) 2021-2024 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

import sys
import yaml

# Read the input file
input = sys.stdin.readlines()
# parse the input file into a list of dictionaries
targets = []
for line in input:
    # Remove leading and trailing whitespace
    line = line.strip()
    # Skip empty lines
    if not line:
        continue
    # Skip comment lines
    if line.startswith('#'):
        continue
    # Split the line into fields
    fields = line.split(':')
    target = {}
    if len(fields) == 7:
        target['name'] = fields[0]
        target['host'] = fields[1]
        target['port'] = fields[2]
        target['user'] = fields[3]
        target['key'] = fields[4]
        target['pwd'] = fields[5]
        #target['sudo'] = fields[6] # not used in PerfSpect 3.0+
    elif len(fields) == 6:
        target['name'] = ''
        target['host'] = fields[0]
        target['port'] = fields[1]
        target['user'] = fields[2]
        target['key'] = fields[3]
        target['pwd'] = fields[4]
        #target['sudo'] = fields[5] # not used in PerfSpect 3.0+
    else:
        continue
    targets.append(target)

# Write the list of dictionaries to the output file in YAML format
header = '''# This YAML file contains a list of remote targets with their corresponding properties.
# Each target has the following properties:
#   name: The name of the target (optional)
#   host: The IP address or host name of the target (required)
#   port: The port number used to connect to the target via SSH (optional)
#   user: The user name used to connect to the target via SSH (optional)
#   key: The path to the private key file used to connect to the target via SSH (optional)
#   pwd: The password used to connect to the target via SSH (optional)
#
# Note: If key and pwd are both provided, the key will be used for authentication.
#
# Security Notes: 
#   It is recommended to use a private key for authentication instead of a password.
#   Keep this file in a secure location and do not expose it to unauthorized users.
#'''
print(header)
output = {}
output['targets'] = targets
print(yaml.dump(output))
