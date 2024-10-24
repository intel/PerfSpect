#!/usr/bin/env bash
# Copyright (C) 2021-2024 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

# run this script from repo's root directory

set -ex

TAG=v1

# build tools image
docker build -f tools/build.Dockerfile --tag perfspect-tools:$TAG ./tools
# Create a temporary container
id=$(docker create perfspect-tools:$TAG foo)

# Copy the files from the container to your local disk
# Note: not used in build process, but useful to have around
docker cp "$id":/bin ./tools

# Remove the temporary container
docker rm "$id"

# build go app builder image
docker build -f build.Dockerfile --tag perfspect-builder:$TAG .

# build perfspect release package builder image
docker build -f builder/build.Dockerfile --build-arg TAG=$TAG --tag perfspect-package-builder:$TAG .

# build perfspect release package
docker container run                                  \
    --volume "$(pwd)":/localrepo                      \
    -w /localrepo                                     \
    --rm                                              \
    perfspect-package-builder:$TAG                           \
    make dist
