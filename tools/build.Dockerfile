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
ARG GO_VERSION=1.24.3
RUN apt-get update && apt-get install -y apt-utils locales wget curl git netcat-openbsd software-properties-common jq zip unzip
RUN locale-gen en_US.UTF-8 &&  echo "LANG=en_US.UTF-8" > /etc/default/locale
RUN for i in {1..5}; do \
        add-apt-repository ppa:git-core/ppa -y && break; \
        echo "Retrying in 5 seconds... ($i/5)" && sleep 5; \
    done
RUN for i in {1..5}; do \
        apt-get update && apt-get install -y git build-essential autotools-dev automake \
        gawk zlib1g-dev libtool libaio-dev libaio1 pandoc pkgconf libcap-dev docbook-utils \
        libreadline-dev default-jre default-jdk cmake flex bison libssl-dev && break; \
        echo "Retrying in 5 seconds... ($i/5)" && sleep 5; \
    done


RUN mkdir workdir
ADD . /workdir

RUN ARCH=$(uname -m); \
if [ "$ARCH" = "x86_64" ]; then \
    PKG_ARCH="amd64"; \
elif [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then \
    PKG_ARCH="arm64"; \
fi; \
echo PKG_ARCH=$PKG_ARCH; \
JAVA_HOME="/usr/lib/jvm/java-1.11.0-openjdk-${PKG_ARCH}" ; export JAVA_HOME; \
rm -rf /usr/local/go && wget -qO- https://go.dev/dl/go${GO_VERSION}.linux-${PKG_ARCH}.tar.gz | tar -C /usr/local -xz && \
PATH="${PATH}:/usr/local/go/bin" && \
git clone https://github.com/madler/zlib.git && cd zlib && ./configure && make install && \
cp /usr/local/lib/libz.a /usr/lib/${ARCH}-linux-gnu/libz.a && \
cd /workdir && \
make -j$(nproc) tools

RUN ARCH=$(uname -m); \
if [ "$ARCH" = "x86_64" ]; then \
    PKG_ARCH="amd64"; \
elif [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then \
    PKG_ARCH="arm64"; \
fi; \
echo PKG_ARCH=$PKG_ARCH; \
JAVA_HOME="/usr/lib/jvm/java-1.11.0-openjdk-${PKG_ARCH}" ; export JAVA_HOME; \
PATH="${PATH}:/usr/local/go/bin" && \
cd /workdir && \
make oss-source
ENV PATH="${PATH}:/usr/local/go/bin"

FROM ubuntu:22.04 AS perf-builder
# Define default values for proxy environment variables
ARG http_proxy=""
ARG https_proxy=""
ENV http_proxy=${http_proxy}
ENV https_proxy=${https_proxy}
ENV LANG=en_US.UTF-8
ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y apt-utils locales wget curl git netcat-openbsd software-properties-common jq zip unzip
RUN locale-gen en_US.UTF-8 &&  echo "LANG=en_US.UTF-8" > /etc/default/locale
RUN for i in {1..5}; do \
        add-apt-repository ppa:git-core/ppa -y && break; \
        echo "Retrying in 5 seconds... ($i/5)" && sleep 5; \
    done
RUN for i in {1..5}; do \
        apt-get update && apt-get install -y \
        automake autotools-dev binutils-dev bison build-essential clang cmake debuginfod \
        default-jdk default-jre docbook-utils flex gawk git libaio-dev libaio1 \
        libbabeltrace-dev libbpf-dev libc6 libcap-dev libdw-dev libdwarf-dev libelf-dev \
        libiberty-dev liblzma-dev libnuma-dev libperl-dev libpfm4-dev libreadline-dev \
        libslang2-dev libssl-dev libtool libtraceevent-dev libunwind-dev libzstd-dev \
        libzstd1 llvm-13 pandoc pkgconf python-setuptools python2-dev python3 python3-dev \
        python3-pip systemtap-sdt-dev zlib1g-dev && break; \
        echo "Retrying in 5 seconds... ($i/5)" && sleep 5; \
    done
ENV PATH="${PATH}:/usr/lib/llvm-13/bin"
RUN mkdir workdir
ADD . /workdir
WORKDIR /workdir
RUN make perf
RUN make processwatch

FROM scratch AS output
COPY --from=builder workdir/bin /bin
COPY --from=builder workdir/oss_source* /
COPY --from=perf-builder workdir/bin/ /bin
