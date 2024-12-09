# Copyright (C) 2021-2024 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

# builds tools used by the project
# build output binaries will be in workdir/bin
# build output oss_source* will be in workdir
# build image (from project root directory):
#   $ docker build -f tools/build.Dockerfile --tag perfspect-tools:$TAG ./tools
FROM ubuntu:22.04 AS builder
ENV http_proxy=${http_proxy}
ENV https_proxy=${https_proxy}
ENV LANG=en_US.UTF-8
ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y apt-utils locales wget curl git netcat-openbsd software-properties-common jq zip unzip
RUN locale-gen en_US.UTF-8 &&  echo "LANG=en_US.UTF-8" > /etc/default/locale
RUN add-apt-repository ppa:git-core/ppa -y
RUN apt-get update && apt-get install -y \
    automake autotools-dev binutils-dev bison build-essential clang cmake debuginfod \
    default-jdk default-jre docbook-utils flex gawk git libaio-dev libaio1 \
    libbabeltrace-dev libbpf-dev libc6 libcap-dev libdw-dev libdwarf-dev libelf-dev \
    libiberty-dev liblzma-dev libnuma-dev libperl-dev libpfm4-dev libreadline-dev \
    libslang2-dev libssl-dev libtool libtraceevent-dev libunwind-dev libzstd-dev \
    libzstd1 llvm-13 pandoc pkgconf python-setuptools python2-dev python3 python3-dev \
    python3-pip systemtap-sdt-dev zlib1g-dev

RUN bash -c "$(wget -O - https://apt.llvm.org/llvm.sh)"
ENV PATH="${PATH}:/usr/lib/llvm-18/bin"

ENV JAVA_HOME=/usr/lib/jvm/java-1.11.0-openjdk-amd64

# need golang to build go tools
RUN rm -rf /usr/local/go && wget -qO- https://go.dev/dl/go1.23.0.linux-amd64.tar.gz | tar -C /usr/local -xz
ENV PATH="${PATH}:/usr/local/go/bin"

# need up-to-date zlib (used by stress-ng static build) to fix security vulnerabilities
RUN git clone https://github.com/madler/zlib.git && cd zlib && ./configure && make install
RUN cp /usr/local/lib/libz.a /usr/lib/x86_64-linux-gnu/libz.a

# Build third-party components
RUN mkdir workdir
ADD . /workdir
WORKDIR /workdir
RUN make tools && make oss-source

FROM scratch AS output
COPY --from=builder workdir/bin /bin
COPY --from=builder workdir/oss_source* /
