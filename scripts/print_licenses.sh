#!/bin/bash

# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

# Run this script from the root of the repository
# Dependencies: jq, go-licenses
#  apt install jq
#  go install github.com/google/go-licenses@latest

# List of GitHub projects, must have LICENSE file
github_projects=(
    "async-profiler/async-profiler"
    "travisdowns/avx-turbo"
    "mirror/dmidecode"
    "axboe/fio"
    "lyonel/lshw"
    "pciutils/pciutils"
    "intel/pcm"
    "intel/processwatch"
    "ColinIanKing/stress-ng"
    "sysstat/sysstat"
)

# Loop through each URL
for github_project in "${github_projects[@]}"; do
    # Fetch the repository data from GitHub API
    response=$(curl -s "https://api.github.com/repos/$github_project")

    # Extract the license type using jq
    license=$(echo "$response" | jq -r '.license.spdx_id')

    # Print the repository URL and its license type
    echo "github.com/$github_project,https://github.com/$github_project/LICENSE,$license"
done

# non-comforming GitHub repos
echo "github.com/intel/msr-tools,https://github.com/intel/msr-tools/blob/master/cpuid.c,GPL-2.0"
echo "github.com/brendangregg/FlameGraph,https://github.com/brendangregg/FlameGraph/blob/master/flamegraph.pl,CDDL"
echo "github.com/ipmitool/ipmitool,https://github.com/ipmitool/ipmitool?tab=License-1-ov-file#readme,SUN Microsystems"
echo "github.com/speed47/spectre-meltdown-checker,https://github.com/speed47/spectre-meltdown-checker/blob/master/spectre-meltdown-checker.sh,GPL-3.0-only"

# repos not in GitHub
echo "etallen.com/cpuid,http://www.etallen.com/cpuid,GPL-2.0"
echo "git.kernel.org/pub/scm/network/ethtool,https://git.kernel.org/pub/scm/network/ethtool/ethtool.git/tree/LICENSE,GPL-2.0"
echo "sourceforge.net/projects/sshpass,https://sourceforge.net/p/sshpass/code-git/ci/main/tree/COPYING,GPL-2.0"
echo "github.com/torvalds/linux/blob/master/tools/power/x86/turbostat,https://github.com/torvalds/linux/blob/master/tools/power/x86/turbostat/turbostat.c,GPL-2.0"
echo "github.com/torvalds/linux/tree/master/tools/perf,https://github.com/torvalds/linux/tree/master/tools/perf,GPL-2.0"

# Generate a list of licenses for Go dependencies
go-licenses report . 2>/dev/null
