# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

# builds tools used by the project
# build output binaries will be in workdir/bin
# build output oss_source* will be in workdir
# build image (from project root directory):
#   $ docker build -f tools/build.Dockerfile --tag perfspect-tools:$TAG ./tools

FROM ubuntu:18.04 AS builder
# Define default values for proxy environment variables
ARG http_proxy=""
ARG https_proxy=""
ENV http_proxy=${http_proxy}
ENV https_proxy=${https_proxy}
ENV LANG=en_US.UTF-8
ARG DEBIAN_FRONTEND=noninteractive
ARG GO_VERSION=1.25.4

# install minimum packages to add repositories
RUN success=false; \
    for i in {1..5}; do \
        apt-get update && apt-get install -y \
        apt-utils locales software-properties-common \
        && success=true && break; \
        echo "Retrying in 5 seconds... ($i/5)" && sleep 5; \
    done; \
    $success || (echo "Failed to install required packages after 5 attempts" && exit 1)

# generate locale
RUN locale-gen en_US.UTF-8 &&  echo "LANG=en_US.UTF-8" > /etc/default/locale

# add git ppa for up-to-date git
RUN success=false; \
    for i in {1..5}; do \
        add-apt-repository ppa:git-core/ppa -y && apt-get update \
        && success=true && break; \
        echo "Retrying in 5 seconds... ($i/5)" && sleep 5; \
    done; \
    $success || (echo "Failed to add git PPA after 5 attempts" && exit 1)

# install packages required to build tools
RUN success=false; \
    for i in {1..5}; do \
        apt-get install -y \
        wget curl netcat-openbsd jq zip unzip \
        git build-essential autotools-dev automake \
        gawk zlib1g-dev libtool libaio-dev libaio1 pandoc pkgconf libcap-dev docbook-utils \
        libreadline-dev cmake flex bison gettext libssl-dev \
        gcc-aarch64-linux-gnu g++-aarch64-linux-gnu binutils-aarch64-linux-gnu cpp-aarch64-linux-gnu \
        && success=true && break; \
        echo "Retrying in 5 seconds... ($i/5)" && sleep 5; \
    done; \
    $success || (echo "Failed to install build tools after 5 attempts" && exit 1)

# need golang to build go tools like ethtool
RUN rm -rf /usr/local/go && wget -qO- https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz | tar -C /usr/local -xz
ENV PATH="${PATH}:/usr/local/go/bin"

# need up-to-date zlib (used by stress-ng static build) to fix security vulnerabilities
RUN git clone https://github.com/madler/zlib.git \
&& cd zlib \
&& ./configure \
&& make install \
&& cp /usr/local/lib/libz.a /usr/lib/x86_64-linux-gnu/libz.a

# build zlib for aarch64
RUN git clone https://github.com/madler/zlib.git zlib-aarch64 \
&& cd zlib-aarch64 \
&& CHOST=aarch64-linux-gnu ./configure --archs="" --static \
&& make \
&& cp libz.a /usr/lib/aarch64-linux-gnu/

# Build tools
RUN mkdir workdir
ADD . /workdir
WORKDIR /workdir
RUN make tools
RUN make oss-source

FROM ubuntu:22.04 AS perf-builder
# Define default values for proxy environment variables
ARG http_proxy=""
ARG https_proxy=""
ENV http_proxy=${http_proxy}
ENV https_proxy=${https_proxy}
ENV LANG=en_US.UTF-8
ARG DEBIAN_FRONTEND=noninteractive

# install minimum packages to add repositories
RUN success=false; \
    for i in {1..5}; do \
        apt-get update && apt-get install -y \
        apt-utils locales software-properties-common \
        && success=true && break; \
        echo "Retrying in 5 seconds... ($i/5)" && sleep 5; \
    done; \
    $success || (echo "Failed to install required packages after 5 attempts" && exit 1)

# generate locale
RUN locale-gen en_US.UTF-8 &&  echo "LANG=en_US.UTF-8" > /etc/default/locale

# add git ppa for up-to-date git
RUN success=false; \
    for i in {1..5}; do \
        add-apt-repository ppa:git-core/ppa -y && apt-get update \
        && success=true && break; \
        echo "Retrying in 5 seconds... ($i/5)" && sleep 5; \
    done; \
    $success || (echo "Failed to add git PPA after 5 attempts" && exit 1)

# install packages required to build perf and processwatch
# Use relatively small ulimit. This is due to pycompile, 
# see: https://github.com/MaastrichtUniversity/docker-dev/commit/97ab4fd04534f73c023371b07e188918b73ac9d0
# This works around python-pkg-resources taking a extremely long time to install
RUN ulimit -n 4096 && success=false; \
    for i in {1..5}; do \
        apt-get update && apt-get install -y \
        wget curl netcat-openbsd jq zip unzip \
        automake autotools-dev binutils-dev bison build-essential clang cmake debuginfod \
        default-jdk default-jre docbook-utils flex gawk git libaio-dev libaio1 \
        libbabeltrace-dev libbpf-dev libc6 libcap-dev libdw-dev libdwarf-dev libelf-dev \
        libiberty-dev liblzma-dev libnuma-dev libperl-dev libpfm4-dev libreadline-dev \
        libslang2-dev libssl-dev libtool libtraceevent-dev libunwind-dev libzstd-dev \
        libzstd1 llvm-14 pandoc pkgconf python-setuptools python2-dev python3 python3-dev \
        python3-pip systemtap-sdt-dev zlib1g-dev libbz2-dev libcapstone-dev libtracefs-dev \
        gcc-aarch64-linux-gnu g++-aarch64-linux-gnu binutils-aarch64-linux-gnu cpp-aarch64-linux-gnu \
        && success=true && break; \
        echo "Retrying in 5 seconds... ($i/5)" && sleep 5; \
    done; \
    $success || (echo "Failed to install perf build tools after 5 attempts" && exit 1)

# libdwfl will dlopen libdebuginfod at runtime, may cause segment fault in static build, disable it. ref: https://github.com/vgteam/vg/pull/3600
RUN wget https://sourceware.org/elfutils/ftp/0.190/elfutils-0.190.tar.bz2 \
&& tar -xf elfutils-0.190.tar.bz2 \
&& cd elfutils-0.190 \
&& ./configure --disable-debuginfod --disable-libdebuginfod \
&& make install -j

# build zlib for aarch64
RUN git clone https://github.com/madler/zlib.git zlib-aarch64 \
&& cd zlib-aarch64 \
&& CHOST=aarch64-linux-gnu ./configure --archs="" --static \
&& make \
&& cp libz.a /usr/lib/aarch64-linux-gnu/

# build libelf for aarch64
RUN wget https://sourceware.org/elfutils/ftp/0.186/elfutils-0.186.tar.bz2 \
&& tar -xf elfutils-0.186.tar.bz2 \
&& cd elfutils-0.186 \
&& ./configure --host=aarch64-linux-gnu --disable-debuginfod --disable-libdebuginfod \
&& make \
&& cp libelf/libelf.a /usr/lib/aarch64-linux-gnu/

# build libpfm4 for aarch64
RUN git clone https://git.code.sf.net/p/perfmon2/libpfm4 libpfm4-aarch64 \
&& cd libpfm4-aarch64 \
&& git checkout v4.11.1 \
&& sed -i 's/^ARCH :=/ARCH ?=/' config.mk \
&& ARCH=arm64 CC=aarch64-linux-gnu-gcc make \
&& cp lib/libpfm.a /usr/lib/aarch64-linux-gnu/

# build perf and processwatch
ENV PATH="${PATH}:/usr/lib/llvm-14/bin"
RUN mkdir workdir
ADD . /workdir
WORKDIR /workdir
RUN make perf
RUN make processwatch

FROM scratch AS output
COPY --from=builder workdir/bin /bin
COPY --from=builder workdir/oss_source* /
COPY --from=perf-builder workdir/bin/ /bin
