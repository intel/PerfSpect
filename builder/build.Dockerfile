# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

# image contains perfspect release package build environment
# build image:
#   $ docker build --build-arg TAG=v1 -f builder/build.Dockerfile --tag perfspect-builder:v1 .
# build perfspect:
#   $ docker run --rm -v "$PWD":/localrepo -w /localrepo perfspect-builder:v1 make dist

ARG REGISTRY
ARG PREFIX
ARG TAG=v1
ARG TOOLS_IMAGE=${REGISTRY}${PREFIX}perfspect-tools:${TAG}
# STAGE 1 - image contains pre-built tools components, rebuild the image to rebuild the tools components
FROM ${TOOLS_IMAGE} AS tools

# STAGE 2 - image contains perfspect's Go components build environment
FROM golang:1.25.6@sha256:ce63a16e0f7063787ebb4eb28e72d477b00b4726f79874b3205a965ffd797ab2
# install system dependencies
RUN apt-get update && apt-get install -y jq
# allow git to operate in the mounted repository regardless of the user
RUN git config --global --add safe.directory /localrepo
# pre-install Go tools used by make check
RUN go install honnef.co/go/tools/cmd/staticcheck@latest && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && \
    go install golang.org/x/vuln/cmd/govulncheck@latest && \
    go install github.com/securego/gosec/v2/cmd/gosec@latest
# copy the tools binaries and source from the previous stage
RUN mkdir /prebuilt
RUN mkdir /prebuilt/tools
COPY --from=tools /bin/ /prebuilt/tools
COPY --from=tools /oss_source.tgz /prebuilt/
COPY --from=tools /oss_source.tgz.md5 /prebuilt/
# pre-download Go module dependencies (changes when go.mod/go.sum change)
WORKDIR /tmp/deps
COPY go.mod go.sum ./
RUN go mod download

WORKDIR /