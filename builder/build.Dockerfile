# Copyright (C) 2021-2024 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

# image contains svr-info release package build environment
# build image:
#   $ docker build --build-arg TAG=v1 -f builder/build.Dockerfile --tag svr-info-builder:v1 .
# build svr-info:
#   $ docker run --rm -v "$PWD":/localrepo -w /localrepo svr-info-builder:v1 make dist

ARG REGISTRY=
ARG PREFIX=
ARG TAG=
# STAGE 1 - image contains pre-built tools components, rebuild the image to rebuild the tools components
FROM ${REGISTRY}${PREFIX}perfspect-tools:${TAG} AS tools

# STAGE 2 - image contains svr-info's Go components build environment
FROM ${REGISTRY}${PREFIX}perfspect-builder:${TAG} AS perfspect
RUN mkdir /prebuilt
RUN mkdir /prebuilt/tools
COPY --from=tools /bin/ /prebuilt/tools
COPY --from=tools /oss_source* /prebuilt
RUN git config --global --add safe.directory /localrepo