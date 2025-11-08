# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

# image contains perfspect release package build environment
# build image:
#   $ docker build --build-arg TAG=v1 -f builder/build.Dockerfile --tag perfspect-builder:v1 .
# build perfspect:
#   $ docker run --rm -v "$PWD":/localrepo -w /localrepo perfspect-builder:v1 make dist

ARG REGISTRY=
ARG PREFIX=
ARG TAG=
# STAGE 1 - image contains pre-built tools components, rebuild the image to rebuild the tools components
FROM ${REGISTRY}${PREFIX}perfspect-tools:${TAG} AS tools

# STAGE 2 - image contains perfspect's Go components build environment
FROM golang:1.25.4@sha256:6ca9eb0b32a4bd4e8c98a4a2edf2d7c96f3ea6db6eb4fc254eef6c067cf73bb4
# copy the tools binaries and source from the previous stage
RUN mkdir /prebuilt
RUN mkdir /prebuilt/tools
COPY --from=tools /bin/ /prebuilt/tools
COPY --from=tools /oss_source.tgz /prebuilt/
COPY --from=tools /oss_source.tgz.md5 /prebuilt/
# allow git to operate in the mounted repository regardless of the user
RUN git config --global --add safe.directory /localrepo
# install jq as it is used in the Makefile to create the manifest
RUN apt-get update
RUN apt-get install -y jq
