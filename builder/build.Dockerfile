# Copyright (C) 2021-2024 Intel Corporation
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
FROM ${REGISTRY}${PREFIX}perfspect-builder:${TAG} AS perfspect
RUN mkdir /prebuilt
RUN mkdir /prebuilt/tools
COPY --from=tools /bin/ /prebuilt/tools
COPY --from=tools /oss_source.tgz /prebuilt/
COPY --from=tools /oss_source.tgz.md5 /prebuilt/
RUN git config --global --add safe.directory /localrepo