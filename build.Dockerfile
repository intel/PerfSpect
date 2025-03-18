# Copyright (C) 2021-2024 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

# image contains build environment for the application
# build the image (from repo root directory): 
#    $ docker image build -f build.Dockerfile --tag perfspect-builder:v1 .
# build the perfspect Go components using this image
#    $ docker run --rm -v "$PWD":/workdir -w /workdir perfspect-builder:v1 make dist

FROM golang:1.24.1@sha256:af0bb3052d6700e1bc70a37bca483dc8d76994fd16ae441ad72390eea6016d03
WORKDIR /workdir
# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
COPY internal/ ./internal/
RUN go mod download && go mod verify
# Radamsa is used for fuzz testing
RUN curl -s https://gitlab.com/akihe/radamsa/uploads/a2228910d0d3c68d19c09cee3943d7e5/radamsa-0.6.c.gz | gzip -d | cc -O2 -x c -o /usr/local/bin/radamsa -
# jq is needed in the functional test to inspect the svr-info json reports
# zip is needed by CI/CD GHA
RUN apt update && apt install -y jq zip