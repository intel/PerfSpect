#!/usr/bin/env python3
# extract_dependencies.py - extracts dependencies from the tools Makefile
#
# Usage: scripts/extract_dependencies.py tools/Makefile

# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

import re
import json
import argparse

def parse_makefile(makefile_path):
    dependencies = []
    version_pattern = re.compile(r"(\w+_VERSION)\s*:?=\s*\"([^\"]+)\"")
    url_pattern = re.compile(r"(https?://[^\s]+)")

    with open(makefile_path, 'r') as file:
        lines = file.readlines()

    # Look for *_VERSION variables and the next URL in the Makefile
    current_var = None
    current_version = None
    for line in lines:
        version_match = version_pattern.search(line)
        if version_match: # found a version variable
            current_var = version_match.group(1)
            current_version = version_match.group(2)
            continue
        url_match = url_pattern.search(line)
        if current_var and current_version and url_match:
            url = url_match.group(1)
            if current_var in url:
                url = url.replace("$("+current_var+")", current_version)
            dependencies.append({
                "name": current_var[:-8].lower().replace("_", "-"),  # Remove _VERSION suffix, lower case, replace _ with -
                "version": current_version,
                "url": url
            })
            current_var = None  # Reset current_var to avoid duplicates
            current_version = None  # Reset current_version to avoid duplicates

    return dependencies

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Parse the tools/Makefile to extract a list of dependencies.")
    parser.add_argument("makefile_path", help="Path to the Makefile")
    parser.add_argument("--json", action="store_true", help="Output in JSON format")
    args = parser.parse_args()

    dependencies = parse_makefile(args.makefile_path)
    if args.json:
        print(json.dumps(dependencies, indent=4))
    else:
        # Determine column widths for alignment
        name_width = max(len(dep['name']) for dep in dependencies) + 2
        version_width = max(len(dep['version']) for dep in dependencies) + 2
        url_width = max(len(dep['url']) for dep in dependencies) + 2

        # Print header
        print(f"{'Name'.ljust(name_width)}{'Version'.ljust(version_width)}{'URL'}")
        print("-" * (name_width + version_width + url_width))

        # Print dependencies in columns
        for dep in dependencies:
            print(f"{dep['name'].ljust(name_width)}{dep['version'].ljust(version_width)}{dep['url']}")    