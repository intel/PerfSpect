#!/usr/bin/env bash
# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

# run this script from repo's root directory

set -ex

TAG=v1

# build tools image
docker build -f tools/build.Dockerfile --tag perfspect-tools:$TAG ./tools

# build the perfspect builder image
docker build -f builder/build.Dockerfile --build-arg TAG=$TAG --tag perfspect-builder:$TAG .

# build perfspect using the builder image
docker container run                                  \
    --volume "$(pwd)":/localrepo                      \
    -w /localrepo                                     \
    --rm                                              \
    perfspect-builder:$TAG                            \
    make dist