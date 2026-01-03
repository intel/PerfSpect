#!make
#
# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
#

NPROC := $(shell nproc)
WGET_OPTS := --retry-connrefused --waitretry=1 --read-timeout=20 --timeout=15 --tries=3 -N
GIT_CLONE_OPTS := --depth 1 --single-branch

default: tools-x86_64
.PHONY: default tools tools-aarch64 tools-x86_64 reset libs oss-source
.PHONY: async-profiler async-profiler-aarch64 avx-turbo cpuid
.PHONY: dmidecode dmidecode-aarch64 dmidecode-repo
.PHONY: ethtool ethtool-aarch64 ethtool-repo
.PHONY: fio fio-aarch64 fio-repo libaio-aarch64
.PHONY: ipmitool ipmitool-aarch64 ipmitool-repo
.PHONY: lshw lshw-aarch64 lshw-repo
.PHONY: lspci lspci-aarch64 lspci-repo
.PHONY: msr-tools pcm perf perf-x86_64 perf-aarch64
.PHONY: processwatch spectre-meltdown-checker
.PHONY: sshpass sshpass-aarch64
.PHONY: stackcollapse-perf stackcollapse-perf-aarch64
.PHONY: stress-ng stress-ng-aarch64 stress-ng-repo
.PHONY: sysstat sysstat-aarch64 sysstat-repo
.PHONY: tsc turbostat

tools-x86_64: async-profiler avx-turbo cpuid dmidecode ethtool fio ipmitool lshw lspci msr-tools pcm spectre-meltdown-checker sshpass stackcollapse-perf stress-ng sysstat tsc turbostat
	mkdir -p bin/x86_64
	cp -R async-profiler bin/x86_64/
	cp avx-turbo/avx-turbo bin/x86_64/
	cp cpuid/cpuid bin/x86_64/
	cp dmidecode/dmidecode bin/x86_64/
	cp ethtool/ethtool bin/x86_64/
	cp fio/fio bin/x86_64/
	cp stackcollapse-perf/stackcollapse-perf bin/x86_64/
	cp ipmitool/src/ipmitool.static bin/x86_64/ipmitool
	cp lshw/src/lshw-static bin/x86_64/lshw
	cp lspci/lspci bin/x86_64/
	cp lspci/pci.ids.gz bin/x86_64/
	cp msr-tools/rdmsr bin/x86_64/
	cp msr-tools/wrmsr bin/x86_64/
	cp pcm/build/bin/pcm-tpmi bin/x86_64/
	cp pcm/scripts/bhs-power-mode.sh bin/x86_64/
	cp perf-archive/perf-archive.sh bin/x86_64/perf-archive && chmod +rx bin/x86_64/perf-archive
	cp spectre-meltdown-checker/spectre-meltdown-checker.sh bin/x86_64/
	cp sshpass/sshpass bin/x86_64/
	cp stress-ng/stress-ng bin/x86_64/
	cp sysstat/mpstat bin/x86_64/
	cp sysstat/iostat bin/x86_64/
	cp sysstat/sar bin/x86_64/
	cp sysstat/sadc bin/x86_64/
	cp tsc/tsc bin/x86_64/
	cp linux_turbostat/tools/power/x86/turbostat/turbostat bin/x86_64/
	cd bin/x86_64 && strip --strip-unneeded * || true

tools-aarch64: async-profiler-aarch64 dmidecode-aarch64 ethtool-aarch64 fio-aarch64 ipmitool-aarch64 lshw-aarch64 lspci-aarch64 spectre-meltdown-checker sshpass-aarch64 stackcollapse-perf-aarch64 stress-ng-aarch64 sysstat-aarch64
	mkdir -p bin/aarch64
	cp -R async-profiler-aarch64 bin/aarch64/async-profiler
	cp dmidecode-aarch64/dmidecode bin/aarch64/
	cp ethtool-aarch64/ethtool bin/aarch64/
	cp fio-aarch64/fio bin/aarch64/	
	cp stackcollapse-perf/stackcollapse-perf-aarch64 bin/aarch64/stackcollapse-perf
	cp ipmitool-aarch64/src/ipmitool.static bin/aarch64/ipmitool
	cp lshw-aarch64/src/lshw-static bin/aarch64/lshw
	cp lspci-aarch64/lspci bin/aarch64/
	cp lspci-aarch64/pci.ids.gz bin/aarch64/
	cp perf-archive/perf-archive.sh bin/aarch64/perf-archive && chmod +rx bin/aarch64/perf-archive
	cp spectre-meltdown-checker/spectre-meltdown-checker.sh bin/aarch64/
	cp sshpass-aarch64/sshpass bin/aarch64/
	cp stress-ng-aarch64/stress-ng bin/aarch64/
	cp sysstat-aarch64/mpstat bin/aarch64/
	cp sysstat-aarch64/iostat bin/aarch64/
	cp sysstat-aarch64/sar bin/aarch64/
	cp sysstat-aarch64/sadc bin/aarch64/
	cd bin/aarch64 && aarch64-linux-gnu-strip --strip-unneeded * || true

tools: tools-x86_64 tools-aarch64
	@echo "Tools built successfully in bin/x86_64 and bin/aarch64 directories."

ASYNC_PROFILER_VERSION := 4.0
async-profiler:
ifeq ("$(wildcard async-profiler)","")
ifeq ("$(wildcard async-profiler-$(ASYNC_PROFILER_VERSION)-linux-x64.tar.gz)","")
	wget $(WGET_OPTS) https://github.com/async-profiler/async-profiler/releases/download/v$(ASYNC_PROFILER_VERSION)/async-profiler-$(ASYNC_PROFILER_VERSION)-linux-x64.tar.gz
endif
	tar -xf async-profiler-$(ASYNC_PROFILER_VERSION)-linux-x64.tar.gz && mv async-profiler-$(ASYNC_PROFILER_VERSION)-linux-x64 async-profiler
endif

async-profiler-aarch64:
ifeq ("$(wildcard async-profiler-aarch64)","")
ifeq ("$(wildcard async-profiler-$(ASYNC_PROFILER_VERSION)-linux-arm64.tar.gz)","")
	wget $(WGET_OPTS) https://github.com/async-profiler/async-profiler/releases/download/v$(ASYNC_PROFILER_VERSION)/async-profiler-$(ASYNC_PROFILER_VERSION)-linux-arm64.tar.gz
endif
	tar -xf async-profiler-$(ASYNC_PROFILER_VERSION)-linux-arm64.tar.gz && mv async-profiler-$(ASYNC_PROFILER_VERSION)-linux-arm64 async-profiler-aarch64
endif

AVX_TURBO_VERSION := 9cfe8bf
avx-turbo:
ifeq ("$(wildcard avx-turbo)","")
	git clone $(GIT_CLONE_OPTS) https://github.com/travisdowns/avx-turbo.git
else
	cd avx-turbo && git fetch && git checkout $(AVX_TURBO_VERSION)
endif
	cd avx-turbo && git checkout $(AVX_TURBO_VERSION)
	# apply our patches to avx-turbo, aperf/mperf first because it doesn't add/remove any lines
	cd avx-turbo && git apply ../avx-turbo-patches/0001-use-fixed-CPU-number-to-determine-if-APERF-MPERF-are.patch
	cd avx-turbo && git apply ../avx-turbo-patches/0001-Add-CPU-ID-pinning-option.patch
	cd avx-turbo && make

# if you change the version, check the sed hacks below
CPUID_VERSION := 20250513
cpuid:
ifeq ("$(wildcard cpuid)","")
ifeq ("$(wildcard cpuid-$(CPUID_VERSION).src.tar.gz)","")
	wget $(WGET_OPTS) http://www.etallen.com/cpuid/cpuid-$(CPUID_VERSION).src.tar.gz
endif	
	tar -xf cpuid-$(CPUID_VERSION).src.tar.gz && mv cpuid-$(CPUID_VERSION)/ cpuid/
endif	
	# gcc 4.8 doesn't support -Wimplicit-fallthrough option
	cd cpuid && sed -i s/"-Wimplicit-fallthrough"/""/ Makefile
	cd cpuid && make

DMIDECODE_VERSION := dmidecode-3-5
dmidecode-repo:
ifeq ("$(wildcard dmidecode-repo)","")
	git clone $(GIT_CLONE_OPTS) --branch $(DMIDECODE_VERSION) https://github.com/mirror/dmidecode.git dmidecode-repo
else
	cd dmidecode-repo && git fetch --tags && git checkout $(DMIDECODE_VERSION)
endif

dmidecode: dmidecode-repo
ifeq ("$(wildcard dmidecode/dmidecode)","")
	cp -r dmidecode-repo dmidecode
	cd dmidecode && make -j$(NPROC)
else
	@echo "dmidecode already built"
endif

dmidecode-aarch64: dmidecode-repo
ifeq ("$(wildcard dmidecode-aarch64/dmidecode)","")
	cp -r dmidecode-repo dmidecode-aarch64
	cd dmidecode-aarch64 && CC=aarch64-linux-gnu-gcc make -j$(NPROC)
else
	@echo "dmidecode-aarch64 already built"
endif

ETHTOOL_VERSION := v6.5
ethtool-repo:
ifeq ("$(wildcard ethtool-repo)","")
	git clone $(GIT_CLONE_OPTS) --branch $(ETHTOOL_VERSION) https://git.kernel.org/pub/scm/network/ethtool/ethtool.git ethtool-repo || \
	git clone $(GIT_CLONE_OPTS) --branch $(ETHTOOL_VERSION) https://kernel.googlesource.com/pub/scm/network/ethtool/ethtool ethtool-repo
else
	cd ethtool-repo && git fetch --tags && git checkout $(ETHTOOL_VERSION)
endif

ethtool: ethtool-repo
ifeq ("$(wildcard ethtool/Makefile)","")
	cp -r ethtool-repo ethtool
	cd ethtool && ./autogen.sh && ./configure enable_netlink=no
	cd ethtool && make -j$(NPROC)
else
	@echo "ethtool already built"
endif

ethtool-aarch64: ethtool-repo
ifeq ("$(wildcard ethtool-aarch64/Makefile)","")
	cp -r ethtool-repo ethtool-aarch64
	cd ethtool-aarch64 && ./autogen.sh && ./configure --host aarch64-linux-gnu enable_netlink=no
	cd ethtool-aarch64 && make -j$(NPROC)
else
	@echo "ethtool-aarch64 already built"
endif

FIO_COMMIT := de3d5e68dd017a6d6099913b7831bb94f46e49cc # This commit is more recent than the latest release (3.41) and is needed for the aarch64 build's --extra-ldflags option
fio-repo:
ifeq ("$(wildcard fio-repo)","")
	git clone https://github.com/axboe/fio.git fio-repo
	cd fio-repo && git checkout $(FIO_COMMIT)
else
	cd fio-repo && git fetch && git checkout $(FIO_COMMIT)
endif

fio: fio-repo libaio-aarch64
ifeq ("$(wildcard fio/config.log)","")
	cp -r fio-repo fio
	cd fio && ./configure --build-static --disable-native
	cd fio && make -j$(NPROC)
else
	@echo "fio already built"
endif

LIBAIO_VERSION := libaio-0.3.113
libaio-aarch64:
ifeq ("$(wildcard libaio-aarch64)","")
	git clone $(GIT_CLONE_OPTS) --branch $(LIBAIO_VERSION) https://pagure.io/libaio libaio-aarch64
else
	cd libaio-aarch64 && git fetch --tags && git checkout $(LIBAIO_VERSION)
endif
	cd libaio-aarch64 && CC=aarch64-linux-gnu-gcc make -j$(NPROC)

fio-aarch64: fio-repo libaio-aarch64
ifeq ("$(wildcard fio-aarch64/config.log)","")
	@test -d libaio-aarch64/src || (echo "Error: libaio-aarch64/src directory not found" && exit 1)
	cp -r fio-repo fio-aarch64
	cd fio-aarch64 && ./configure --build-static --disable-native --cc=aarch64-linux-gnu-gcc --extra-cflags="-I../libaio-aarch64/src" --extra-ldflags="-L../libaio-aarch64/src"
	cd fio-aarch64 && make -j$(NPROC)
else
	@echo "fio-aarch64 already built"
endif

IPMITOOL_VERSION := IPMITOOL_1_8_19
ipmitool-repo:
ifeq ("$(wildcard ipmitool-repo)","")
	git clone $(GIT_CLONE_OPTS) --branch $(IPMITOOL_VERSION) https://github.com/ipmitool/ipmitool.git ipmitool-repo
else
	cd ipmitool-repo && git fetch --tags && git checkout $(IPMITOOL_VERSION)
endif

ipmitool: ipmitool-repo
ifeq ("$(wildcard ipmitool/Makefile)","")
	cp -r ipmitool-repo ipmitool
	# hack to get around static build problem - don't check for libreadline
	sed -i "s#x\$$xenable_ipmishell#xno#" ipmitool/configure.ac
	cd ipmitool && ./bootstrap && LDFLAGS=-static ./configure
	cd ipmitool && make -j$(NPROC)
	cd ipmitool/src && ../libtool --silent --tag=CC --mode=link gcc -fno-strict-aliasing -Wreturn-type -all-static -o ipmitool.static ipmitool.o ipmishell.o ../lib/libipmitool.la plugins/libintf.la
else
	@echo "ipmitool already built"
endif

ipmitool-aarch64: ipmitool-repo
ifeq ("$(wildcard ipmitool-aarch64/Makefile)","")
	cp -r ipmitool-repo ipmitool-aarch64
	# hack to get around static build problem - don't check for libreadline
	sed -i "s#x\$$xenable_ipmishell#xno#" ipmitool-aarch64/configure.ac
	cd ipmitool-aarch64 && ./bootstrap && LDFLAGS=-static ./configure --host=aarch64-linux-gnu
	cd ipmitool-aarch64 && CC=aarch64-linux-gnu-gcc make -j$(NPROC)
	cd ipmitool-aarch64/src && ../libtool --silent --tag=CC --mode=link aarch64-linux-gnu-gcc -fno-strict-aliasing -Wreturn-type -all-static -o ipmitool.static ipmitool.o ipmishell.o ../lib/libipmitool.la plugins/libintf.la
else
	@echo "ipmitool-aarch64 already built"
endif

LSHW_VERSION := B.02.19
lshw-repo:
ifeq ("$(wildcard lshw-repo)","")
	git clone $(GIT_CLONE_OPTS) --branch $(LSHW_VERSION) https://github.com/lyonel/lshw.git lshw-repo
else
	cd lshw-repo && git fetch --tags && git checkout $(LSHW_VERSION)
endif

lshw: lshw-repo
ifeq ("$(wildcard lshw/src/lshw-static)","")
	cp -r lshw-repo lshw
	cd lshw/src && make static
else
	@echo "lshw already built"
endif

lshw-aarch64: lshw-repo
ifeq ("$(wildcard lshw-aarch64/src/lshw-static)","")
	cp -r lshw-repo lshw-aarch64
	cd lshw-aarch64/src && STRIP=aarch64-linux-gnu-strip CXX=aarch64-linux-gnu-g++ make static
else
	@echo "lshw-aarch64 already built"
endif

LSPCI_VERSION := v3.13.0
lspci-repo:
ifeq ("$(wildcard lspci-repo)","")
	git clone $(GIT_CLONE_OPTS) --branch $(LSPCI_VERSION) https://github.com/pciutils/pciutils.git lspci-repo
else
	cd lspci-repo && git fetch --tags && git checkout $(LSPCI_VERSION)
endif

lspci: lspci-repo
ifeq ("$(wildcard lspci/lspci)","")
	cp -r lspci-repo lspci
	cd lspci && make -j$(NPROC)
	cd lspci && ./update-pciids.sh
	cd lspci && gzip -c pci.ids > pci.ids.gz
else
	@echo "lspci already built"
endif

lspci-aarch64: lspci-repo
ifeq ("$(wildcard lspci-aarch64/lspci)","")
	cp -r lspci-repo lspci-aarch64
	cd lspci-aarch64 && sed -i '/cpu=.*/cpu=aarch64/g' lib/configure
	cd lspci-aarch64 && sed -i 's/^CROSS_COMPILE=/CROSS_COMPILE?=/g' Makefile
	cd lspci-aarch64 && sed -i 's/^SHARED=/SHARED?=/g' Makefile
	cd lspci-aarch64 && CROSS_COMPILE=aarch64-linux-gnu- SHARED=no make -j$(NPROC)
	cd lspci-aarch64 && ./update-pciids.sh
	cd lspci-aarch64 && gzip -c pci.ids > pci.ids.gz
else
	@echo "lspci-aarch64 already built"
endif

MSR_TOOLS_VERSION := master
msr-tools:
ifeq ("$(wildcard msr-tools)","")
	git clone $(GIT_CLONE_OPTS) https://github.com/intel/msr-tools.git
else
	cd msr-tools && git fetch origin && git reset --hard origin/$(MSR_TOOLS_VERSION)
endif
	cd msr-tools && ./autogen.sh && make

PCM_VERSION := 202502
pcm:
ifeq ("$(wildcard pcm)","")
	git clone $(GIT_CLONE_OPTS) --branch $(PCM_VERSION) --recursive https://github.com/intel/pcm.git
else
	cd pcm && git fetch --tags && git checkout $(PCM_VERSION)
endif
	mkdir -p pcm/build
	cd pcm/build &&	cmake -DNO_ASAN=1 ..
	cd pcm/build && cmake --build .

PERF_VERSION := 6.15.3
linux-$(PERF_VERSION).tar.xz:
	wget $(WGET_OPTS) https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-$(PERF_VERSION).tar.xz

perf-x86_64: linux-$(PERF_VERSION).tar.xz
ifeq ("$(wildcard linux_perf)","")
	tar -xf linux-$(PERF_VERSION).tar.xz --transform='s/linux-$(PERF_VERSION)/linux_perf/'
endif
	cd linux_perf/tools/perf && make LLVM_CONFIG="llvm-config --link-static" LDFLAGS="-static --static" BUILD_BPF_SKEL=1 NO_JVMTI=1 BUILD_NONDISTRO=1 LIBUNWIND=1 -j$(NPROC)
	mkdir -p bin/x86_64
	cp linux_perf/tools/perf/perf bin/x86_64/
	strip --strip-unneeded bin/x86_64/perf

perf-aarch64: linux-$(PERF_VERSION).tar.xz
ifeq ("$(wildcard linux_perf-aarch64)","")
	tar -xf linux-$(PERF_VERSION).tar.xz --transform='s/linux-$(PERF_VERSION)/linux_perf-aarch64/'
endif
	cd linux_perf-aarch64/tools/perf && make LDFLAGS="-static --static" BUILD_BPF_SKEL=1 NO_JVMTI=1 ARCH=arm64 NO_LIBTRACEEVENT=1 CROSS_COMPILE=aarch64-linux-gnu- -j$(NPROC)
	mkdir -p bin/aarch64
	cp linux_perf-aarch64/tools/perf/perf bin/aarch64/
	aarch64-linux-gnu-strip --strip-unneeded bin/aarch64/perf

perf: perf-x86_64 perf-aarch64
	@echo "Perf tools built successfully in bin/x86_64 and bin/aarch64 directories."

PROCESSWATCH_VERSION := v1.3	
processwatch:
ifeq ("$(wildcard processwatch)","")
	git clone $(GIT_CLONE_OPTS) --branch $(PROCESSWATCH_VERSION) --recursive https://github.com/intel/processwatch.git
else
	cd processwatch && git fetch --tags && git checkout $(PROCESSWATCH_VERSION)
endif
	cd processwatch && ./build.sh
	mkdir -p bin
	cp processwatch/processwatch bin/x86_64/
	strip --strip-unneeded bin/x86_64/processwatch

SPECTRE_MELTDOWN_CHECKER_VERSION := master
spectre-meltdown-checker:
ifeq ("$(wildcard spectre-meltdown-checker)","")
	git clone $(GIT_CLONE_OPTS) https://github.com/speed47/spectre-meltdown-checker.git
else
	cd spectre-meltdown-checker && git fetch origin && git reset --hard origin/$(SPECTRE_MELTDOWN_CHECKER_VERSION)
endif

SSHPASS_VERSION := 1.10
sshpass-$(SSHPASS_VERSION).tar.gz:
	wget $(WGET_OPTS) https://sourceforge.net/projects/sshpass/files/sshpass/$(SSHPASS_VERSION)/sshpass-$(SSHPASS_VERSION).tar.gz

sshpass: sshpass-$(SSHPASS_VERSION).tar.gz
ifeq ("$(wildcard sshpass)","")
	tar -xf sshpass-$(SSHPASS_VERSION).tar.gz --transform='s/sshpass-$(SSHPASS_VERSION)/sshpass/'
	cd sshpass && ./configure
endif
	cd sshpass && make -j$(NPROC)

sshpass-aarch64: sshpass-$(SSHPASS_VERSION).tar.gz
ifeq ("$(wildcard sshpass-aarch64)","")
	tar -xf sshpass-$(SSHPASS_VERSION).tar.gz --transform='s/sshpass-$(SSHPASS_VERSION)/sshpass-aarch64/'
	cd sshpass-aarch64 && ./configure --host=aarch64-linux-gnu
endif
	cd sshpass-aarch64 && make -j$(NPROC)

stackcollapse-perf:
	cd stackcollapse-perf && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build

stackcollapse-perf-aarch64:
	cd stackcollapse-perf && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $@

STRESS_NG_VERSION := V0.13.08
stress-ng-repo:
ifeq ("$(wildcard stress-ng-repo)","")
	git clone $(GIT_CLONE_OPTS) --branch $(STRESS_NG_VERSION) https://github.com/ColinIanKing/stress-ng.git stress-ng-repo
else
	cd stress-ng-repo && git fetch --tags && git checkout $(STRESS_NG_VERSION)
endif

stress-ng: stress-ng-repo
ifeq ("$(wildcard stress-ng/stress-ng)","")
	cp -r stress-ng-repo stress-ng
	cd stress-ng && STATIC=1 make -j$(NPROC)
else
	@echo "stress-ng already built"
endif

stress-ng-aarch64: stress-ng-repo
ifeq ("$(wildcard stress-ng-aarch64/stress-ng)","")
	cp -r stress-ng-repo stress-ng-aarch64
	cd stress-ng-aarch64 && CC=aarch64-linux-gnu-gcc STATIC=1 make -j$(NPROC)
else
	@echo "stress-ng-aarch64 already built"
endif

SYSSTAT_VERSION := v12.7.6
sysstat-repo:
ifeq ("$(wildcard sysstat-repo)","")
	git clone $(GIT_CLONE_OPTS) --branch $(SYSSTAT_VERSION) https://github.com/sysstat/sysstat.git sysstat-repo
else
	cd sysstat-repo && git fetch --tags && git checkout $(SYSSTAT_VERSION)
endif

sysstat: sysstat-repo
ifeq ("$(wildcard sysstat/Makefile)","")
	cp -r sysstat-repo sysstat
	cd sysstat && ./configure
	cd sysstat && make -j$(NPROC)
else
	@echo "sysstat already built"
endif

sysstat-aarch64: sysstat-repo
ifeq ("$(wildcard sysstat-aarch64/Makefile)","")
	cp -r sysstat-repo sysstat-aarch64
	cd sysstat-aarch64 && ./configure --host=aarch64-linux-gnu
	cd sysstat-aarch64 && make -j$(NPROC)
else
	@echo "sysstat-aarch64 already built"
endif

tsc:
	cd tsc && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build

TURBOSTAT_VERSION := 6.9.12
turbostat:
ifeq ("$(wildcard linux_turbostat)","")
	wget $(WGET_OPTS) https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-$(TURBOSTAT_VERSION).tar.xz
	tar -xf linux-$(TURBOSTAT_VERSION).tar.xz && mv linux-$(TURBOSTAT_VERSION)/ linux_turbostat/
endif
	sed -i '/_Static_assert/d' linux_turbostat/tools/power/x86/turbostat/turbostat.c
	cd linux_turbostat/tools/power/x86/turbostat && make

reset:
	cd async-profiler
	cd cpuid && make clean
	cd dmidecode && git clean -fdx && git reset --hard
	cd ethtool && git clean -fdx && git reset --hard
	cd fio && git clean -fdx && git reset --hard
	cd ipmitool && git clean -fdx && git reset --hard
	cd lshw && git clean -fdx && git reset --hard
	cd lspci && git clean -fdx && git reset --hard
	cd pcm && git clean -fdx && git reset --hard
	cd msr-tools && git clean -fdx && git reset --hard
	cd spectre-meltdown-checker
	cd sshpass && make clean
	cd stress-ng && git clean -fdx && git reset --hard
	cd sysstat && git clean -fdx && git reset --hard
	cd tsc && rm -f tsc
	cd linux_turbostat/tools/power/x86/turbostat && make clean

# libs are not directly used in the build but are required in the oss archive file because some of the tools are statically linked
# glibc 2.27 was the version used in Ubuntu 18.04 which is the base docker image we used to build the tools
glibc.tar.gz:
	wget $(WGET_OPTS) https://github.com/bminor/glibc/archive/refs/tags/glibc-2.27.tar.gz -O glibc.tar.gz
zlib.tar.gz:
	wget $(WGET_OPTS) https://github.com/madler/zlib/archive/refs/heads/master.tar.gz -O zlib.tar.gz
libcrypt.tar.gz:
	wget $(WGET_OPTS) https://github.com/gpg/libgcrypt/archive/refs/heads/master.tar.gz -O libcrypt.tar.gz
libs: glibc.tar.gz zlib.tar.gz libcrypt.tar.gz

oss-source: reset libs
	tar --exclude-vcs -czf oss_source.tgz async-profiler/ cpuid/ dmidecode/ ethtool/ fio/ ipmitool/ lshw/ lspci/ msr-tools/ perf-archive/ pcm/ spectre-meltdown-checker/ sshpass/ stress-ng/ sysstat/ linux_turbostat/tools/power/x86/turbostat glibc.tar.gz zlib.tar.gz libcrypt.tar.gz
	md5sum oss_source.tgz > oss_source.tgz.md5
