#!make
#
# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
#

NPROC := $(shell nproc)

default: tools-x86_64
.PHONY: default tools tools-aarch64 tools-x86_64 async-profiler avx-turbo cpuid dmidecode ethtool fio ipmitool lshw lspci msr-tools pcm perf-x86_64 processwatch spectre-meltdown-checker sshpass stackcollapse-perf stress-ng sysstat tsc turbostat

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
	-cd bin/x86_64 && strip --strip-unneeded *

tools-aarch64: async-profiler-aarch64 dmidecode-aarch64 ethtool-aarch64 fio-aarch64 ipmitool-aarch64 lshw-aarch64 lspci-aarch64 spectre-meltdown-checker sshpass-aarch64 stackcollapse-perf-aarch64 stress-ng-aarch64 sysstat-aarch64 tsc-aarch64
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
	cp tsc/tsc-aarch64 bin/aarch64/tsc
	-cd bin/aarch64 && aarch64-linux-gnu-strip --strip-unneeded * || true

tools: tools-x86_64 tools-aarch64
	@echo "Tools built successfully in bin/x86_64 and bin/aarch64 directories."

ASYNC_PROFILER_VERSION := "4.0"
async-profiler:
ifeq ("$(wildcard async-profiler)","")
ifeq ("$(wildcard async-profiler-$(ASYNC_PROFILER_VERSION)-linux-x64.tar.gz)","")
	wget https://github.com/jvm-profiling-tools/async-profiler/releases/download/v$(ASYNC_PROFILER_VERSION)/async-profiler-$(ASYNC_PROFILER_VERSION)-linux-x64.tar.gz
endif
	tar -xf async-profiler-$(ASYNC_PROFILER_VERSION)-linux-x64.tar.gz && mv async-profiler-$(ASYNC_PROFILER_VERSION)-linux-x64 async-profiler
endif

async-profiler-aarch64:
ifeq ("$(wildcard async-profiler-aarch64)","")
ifeq ("$(wildcard async-profiler-$(ASYNC_PROFILER_VERSION)-linux-arm64.tar.gz)","")
	wget https://github.com/jvm-profiling-tools/async-profiler/releases/download/v$(ASYNC_PROFILER_VERSION)/async-profiler-$(ASYNC_PROFILER_VERSION)-linux-arm64.tar.gz
endif
	tar -xf async-profiler-$(ASYNC_PROFILER_VERSION)-linux-arm64.tar.gz && mv async-profiler-$(ASYNC_PROFILER_VERSION)-linux-arm64 async-profiler-aarch64
endif


AVX_TURBO_VERSION := "9cfe8bf"
avx-turbo:
ifeq ("$(wildcard avx-turbo)","")
	git clone https://github.com/travisdowns/avx-turbo.git
else
	cd avx-turbo && git checkout master && git pull
endif
	cd avx-turbo && git checkout $(AVX_TURBO_VERSION)
	# apply our patches to avx-turbo, aperf/mperf first because it doesn't add/remove any lines
	cd avx-turbo && git apply ../avx-turbo-patches/0001-use-fixed-CPU-number-to-determine-if-APERF-MPERF-are.patch
	cd avx-turbo && git apply ../avx-turbo-patches/0001-Add-CPU-ID-pinning-option.patch
	cd avx-turbo && make

# if you change the version, check the sed hacks below
CPUID_VERSION := "20250513"
cpuid:
ifeq ("$(wildcard cpuid)","")
ifeq ("$(wildcard cpuid-$(CPUID_VERSION).src.tar.gz)","")
	wget http://www.etallen.com/cpuid/cpuid-$(CPUID_VERSION).src.tar.gz
endif	
	tar -xf cpuid-$(CPUID_VERSION).src.tar.gz && mv cpuid-$(CPUID_VERSION)/ cpuid/
endif	
	# gcc 4.8 doesn't support -Wimplicit-fallthrough option
	cd cpuid && sed -i s/"-Wimplicit-fallthrough"/""/ Makefile
	cd cpuid && make

DMIDECODE_VERSION := "dmidecode-3-5"
dmidecode:
ifeq ("$(wildcard dmidecode)","")
	git clone https://github.com/mirror/dmidecode.git
else
	cd dmidecode && git checkout master && git pull
endif
	cd dmidecode && git checkout $(DMIDECODE_VERSION)
	cd dmidecode && make -j$(NPROC)

dmidecode-aarch64:
ifeq ("$(wildcard dmidecode-aarch64)","")
	git clone https://github.com/mirror/dmidecode.git dmidecode-aarch64
else
	cd dmidecode-aarch64 && git checkout master && git pull
endif
	cd dmidecode-aarch64 && git checkout $(DMIDECODE_VERSION)
	cd dmidecode-aarch64 && CC=aarch64-linux-gnu-gcc make -j$(NPROC)

ELFUTILS_VERSION := "0.186"
elfutils-aarch64: zlib-aarch64
ifeq ("$(wildcard elfutils-aarch64)","")
	wget https://sourceware.org/elfutils/ftp/0.186/elfutils-$(ELFUTILS_VERSION).tar.bz2 
	mkdir -p elfutils-tmp
	bzip2 -dc elfutils-$(ELFUTILS_VERSION).tar.bz2 | tar -C elfutils-tmp -xf -
	mv elfutils-tmp/elfutils-$(ELFUTILS_VERSION) elfutils-aarch64 && rm -rf elfutils-tmp
endif
	cd elfutils-aarch64 && ./configure --host=aarch64-linux-gnu --disable-debuginfod --disable-libdebuginfod
	cd elfutils-aarch64 && make -j$(NPROC)
	cd elfutils-aarch64 && cp libelf/libelf.a /usr/lib/aarch64-linux-gnu/

ETHTOOL_VERSION := "v6.5"
ethtool:
ifeq ("$(wildcard ethtool)","")
	git clone https://git.kernel.org/pub/scm/network/ethtool/ethtool.git
else
	cd ethtool && git checkout master && git pull
endif
	cd ethtool && git checkout $(ETHTOOL_VERSION)
ifeq ("$(wildcard ethtool/Makefile)","")
	cd ethtool && ./autogen.sh && ./configure enable_netlink=no
endif
	cd ethtool && make -j$(NPROC)

ethtool-aarch64:
ifeq ("$(wildcard ethtool-aarch64)","")
	git clone https://git.kernel.org/pub/scm/network/ethtool/ethtool.git ethtool-aarch64
else
	cd ethtool-aarch64 && git checkout master && git pull
endif
	cd ethtool-aarch64 && git checkout $(ETHTOOL_VERSION)
ifeq ("$(wildcard ethtool-aarch64/Makefile)","")
	cd ethtool-aarch64 && ./autogen.sh && ./configure --host aarch64-linux-gnu enable_netlink=no
endif
	cd ethtool-aarch64 && make -j$(NPROC)

FIO_VERSION := "fio-3.38"
fio:
ifeq ("$(wildcard fio)","")
	git clone https://github.com/axboe/fio.git
else
	cd fio && git checkout master && git pull
endif
	cd fio && git checkout $(FIO_VERSION)
ifeq ("$(wildcard fio/config.log)","")
	cd fio && ./configure --build-static --disable-native
endif
	cd fio && make -j$(NPROC)

fio-aarch64:
ifeq ("$(wildcard fio-aarch64)","")
	git clone https://github.com/axboe/fio.git fio-aarch64
else
	cd fio-aarch64 && git checkout master && git pull
endif
	cd fio-aarch64 && git checkout $(FIO_VERSION)
ifeq ("$(wildcard fio-aarch64/config.log)","")
	cd fio-aarch64 && ./configure --build-static --disable-native --cc=aarch64-linux-gnu-gcc
endif
	cd fio-aarch64 && make -j$(NPROC)

IPMITOOL_VERSION := "IPMITOOL_1_8_19"
ipmitool:
ifeq ("$(wildcard ipmitool)","")
	git clone https://github.com/ipmitool/ipmitool.git
endif
	cd ipmitool && git checkout $(IPMITOOL_VERSION)
ifeq ("$(wildcard ipmitool/Makefile)","")
	# hack to get around static build problem - don't check for libreadline
	sed -i "s#x\$$xenable_ipmishell#xno#" ipmitool/configure.ac
	cd ipmitool && ./bootstrap && LDFLAGS=-static ./configure
endif
	cd ipmitool && make -j$(NPROC)
	cd ipmitool/src && ../libtool --silent --tag=CC --mode=link gcc -fno-strict-aliasing -Wreturn-type -all-static -o ipmitool.static ipmitool.o ipmishell.o ../lib/libipmitool.la plugins/libintf.la

ipmitool-aarch64:
ifeq ("$(wildcard ipmitool-aarch64)","")
	git clone https://github.com/ipmitool/ipmitool.git ipmitool-aarch64
endif
	cd ipmitool-aarch64 && git checkout $(IPMITOOL_VERSION)
ifeq ("$(wildcard ipmitool-aarch64/Makefile)","")
	# hack to get around static build problem - don't check for libreadline
	sed -i "s#x\$$xenable_ipmishell#xno#" ipmitool-aarch64/configure.ac
	cd ipmitool-aarch64 && ./bootstrap && LDFLAGS=-static ./configure --host=aarch64-linux-gnu
endif
	cd ipmitool-aarch64 && CC=aarch64-linux-gnu-gcc make -j$(NPROC)
	cd ipmitool-aarch64/src && ../libtool --silent --tag=CC --mode=link aarch64-linux-gnu-gcc -fno-strict-aliasing -Wreturn-type -all-static -o ipmitool.static ipmitool.o ipmishell.o ../lib/libipmitool.la plugins/libintf.la

LIBPFM4_VERSION := "v4.11.1"
libpfm4-aarch64:
ifeq ("$(wildcard libpfm4-aarch64)","")
	git clone https://git.code.sf.net/p/perfmon2/libpfm4 libpfm4-aarch64
endif
	cd libpfm4-aarch64 && git checkout $(LIBPFM4_VERSION)
	cd libpfm4-aarch64 && sed -i 's/^ARCH :=/ARCH ?=/' config.mk
	cd libpfm4-aarch64 && ARCH=arm64 CC=aarch64-linux-gnu-gcc make -j$(NPROC)
	cd libpfm4-aarch64 && cp lib/libpfm.a /usr/lib/aarch64-linux-gnu/

LSHW_VERSION := "B.02.19"
lshw:
ifeq ("$(wildcard lshw)","")
	git clone https://github.com/lyonel/lshw.git
else
	cd lshw && git checkout master && git pull
endif
	cd lshw && git checkout $(LSHW_VERSION)
	cd lshw/src && make static

lshw-aarch64:
ifeq ("$(wildcard lshw-aarch64)","")
	git clone https://github.com/lyonel/lshw.git lshw-aarch64
else
	cd lshw-aarch64 && git checkout master && git pull
endif
	cd lshw-aarch64 && git checkout $(LSHW_VERSION)
	cd lshw-aarch64/src && STRIP=aarch64-linux-gnu-strip CXX=aarch64-linux-gnu-g++ make static

LSPCI_VERSION := "v3.13.0"
lspci:
ifeq ("$(wildcard lspci)","")
	git clone https://github.com/pciutils/pciutils.git lspci
else
	cd lspci && git checkout master && git pull
endif
	cd lspci && git checkout $(LSPCI_VERSION)
	cd lspci && make -j$(NPROC)
	cd lspci && ./update-pciids.sh
	cd lspci && gzip -c pci.ids > pci.ids.gz

lspci-aarch64: zlib-aarch64
ifeq ("$(wildcard lspc-aarch64)","")
	git clone https://github.com/pciutils/pciutils.git lspci-aarch64
else
	cd lspci-aarch64 && git checkout master && git pull
endif
	cd lspci-aarch64 && git checkout $(LSPCI_VERSION)
	cd lspci-aarch64 && sed -i '/cpu=.*/cpu=aarch64/g' lib/configure
	cd lspci-aarch64 && sed -i 's/^CROSS_COMPILE=/CROSS_COMPILE?=/g' Makefile
	cd lspci-aarch64 && sed -i 's/^SHARED=/SHARED?=/g' Makefile
	cd lspci-aarch64 && CROSS_COMPILE=aarch64-linux-gnu- SHARED=no make -j$(NPROC)
	cd lspci-aarch64 && ./update-pciids.sh
	cd lspci-aarch64 && gzip -c pci.ids > pci.ids.gz

MSR_TOOLS_VERSION := "master"
msr-tools:
ifeq ("$(wildcard msr-tools)","")
	git clone https://github.com/intel/msr-tools.git
else
	cd msr-tools && git checkout master && git pull
endif
	cd msr-tools && git checkout $(MSR_TOOLS_VERSION) && git pull
	cd msr-tools && ./autogen.sh && make

PCM_VERSION := "202502"
pcm:
ifeq ("$(wildcard pcm)","")
	git clone --recursive https://github.com/intel/pcm.git
else
	cd pcm && git checkout master && git pull
endif
	cd pcm && git checkout $(PCM_VERSION)
	mkdir -p pcm/build
	cd pcm/build &&	cmake -DNO_ASAN=1 ..
	cd pcm/build && cmake --build .

PERF_VERSION := "6.15.3"
perf-x86_64:
	wget https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-$(PERF_VERSION).tar.xz
	tar -xf linux-$(PERF_VERSION).tar.xz && mv linux-$(PERF_VERSION)/ linux_perf/
	cd linux_perf/tools/perf && make LLVM_CONFIG="llvm-config --link-static" LDFLAGS="-static --static" BUILD_BPF_SKEL=1 NO_JVMTI=1 BUILD_NONDISTRO=1 LIBUNWIND=1 -j$(NPROC)
	mkdir -p bin/x86_64
	cp linux_perf/tools/perf/perf bin/x86_64/
	strip --strip-unneeded bin/x86_64/perf

perf-aarch64: zlib-aarch64 elfutils-aarch64 libpfm4-aarch64
ifeq ("$(wildcard perf-aarch64)","")
	wget -N https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-$(PERF_VERSION).tar.xz
	tar -xf linux-$(PERF_VERSION).tar.xz && mv linux-$(PERF_VERSION)/ linux_perf-aarch64/
endif
	cd linux_perf-aarch64/tools/perf && make LDFLAGS="-static --static" BUILD_BPF_SKEL=1 NO_JVMTI=1 ARCH=arm64 NO_LIBTRACEEVENT=1 CROSS_COMPILE=aarch64-linux-gnu- -j$(NPROC)
	mkdir -p bin/aarch64
	cp linux_perf-aarch64/tools/perf/perf bin/aarch64/
	aarch64-linux-gnu-strip --strip-unneeded bin/aarch64/perf

perf: perf-x86_64 perf-aarch64
	@echo "Perf tools built successfully in bin/x86_64 and bin/aarch64 directories."

PROCESSWATCH_VERSION := "c394065"	
processwatch:
ifeq ("$(wildcard processwatch)","")
	git clone --recursive https://github.com/intel/processwatch.git
else
	cd processwatch && git checkout main && git pull
endif
	cd processwatch && git checkout $(PROCESSWATCH_VERSION)
	cd processwatch && ./build.sh
	mkdir -p bin
	cp processwatch/processwatch bin/x86_64/
	strip --strip-unneeded bin/x86_64/processwatch

SPECTRE_MELTDOWN_CHECKER_VERSION := "master"
spectre-meltdown-checker:
ifeq ("$(wildcard spectre-meltdown-checker)","")
	git clone https://github.com/speed47/spectre-meltdown-checker.git
else
	cd spectre-meltdown-checker && git checkout master && git pull
endif
	cd spectre-meltdown-checker && git checkout $(SPECTRE_MELTDOWN_CHECKER_VERSION) && git pull

SSHPASS_VERSION := "1.10"
sshpass:
ifeq ("$(wildcard sshpass)","")
	wget https://sourceforge.net/projects/sshpass/files/sshpass/$(SSHPASS_VERSION)/sshpass-$(SSHPASS_VERSION).tar.gz
	tar -xf sshpass-$(SSHPASS_VERSION).tar.gz
	mv sshpass-$(SSHPASS_VERSION) sshpass
	rm sshpass-$(SSHPASS_VERSION).tar.gz
	cd sshpass && ./configure
endif
	cd sshpass && make -j$(NPROC)

sshpass-aarch64:
ifeq ("$(wildcard sshpass-aarch64)","")
	wget https://sourceforge.net/projects/sshpass/files/sshpass/$(SSHPASS_VERSION)/sshpass-$(SSHPASS_VERSION).tar.gz
	tar -xf sshpass-$(SSHPASS_VERSION).tar.gz
	mv sshpass-$(SSHPASS_VERSION) sshpass-aarch64
	rm sshpass-$(SSHPASS_VERSION).tar.gz
	cd sshpass-aarch64 && ./configure --host=aarch64-linux-gnu
endif
	cd sshpass-aarch64 && make -j$(NPROC)

stackcollapse-perf:
	cd stackcollapse-perf && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build

stackcollapse-perf-aarch64:
	cd stackcollapse-perf && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $@

STRESS_NG_VERSION := "V0.13.08"
stress-ng:
ifeq ("$(wildcard stress-ng)","")
	git clone https://github.com/ColinIanKing/stress-ng.git
else
	cd stress-ng && git checkout master && git pull
endif
	cd stress-ng && git checkout $(STRESS_NG_VERSION)
	cd stress-ng && STATIC=1 make -j$(NPROC)

stress-ng-aarch64:
ifeq ("$(wildcard stress-ng-aarch64)","")
	git clone https://github.com/ColinIanKing/stress-ng.git stress-ng-aarch64
else
	cd stress-ng-aarch64 && git checkout master && git pull
endif
	cd stress-ng-aarch64 && git checkout $(STRESS_NG_VERSION)
	cd stress-ng-aarch64 && CC=aarch64-linux-gnu-gcc STATIC=1 make -j$(NPROC)

SYSSTAT_VERSION := "v12.7.6"
sysstat:
ifeq ("$(wildcard sysstat)","")
	git clone https://github.com/sysstat/sysstat.git
else
	cd sysstat && git checkout master && git pull
endif
	cd sysstat && git checkout $(SYSSTAT_VERSION)
ifeq ("$(wildcard sysstat/Makefile)","")
	cd sysstat && ./configure
endif
	cd sysstat && make -j$(NPROC)

sysstat-aarch64:
ifeq ("$(wildcard sysstat-aarch64)","")
	git clone https://github.com/sysstat/sysstat.git sysstat-aarch64
else
	cd sysstat-aarch64 && git checkout master && git pull
endif
	cd sysstat-aarch64 && git checkout $(SYSSTAT_VERSION)
ifeq ("$(wildcard sysstat-aarch64/Makefile)","")
	cd sysstat-aarch64 && ./configure --host=aarch64-linux-gnu
endif
	cd sysstat-aarch64 && make -j$(NPROC)

tsc:
	cd tsc && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build

tsc-aarch64:
	cd tsc && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o tsc-aarch64

TURBOSTAT_VERSION := "6.9.12"
turbostat:
	wget https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-$(TURBOSTAT_VERSION).tar.xz
	tar -xf linux-$(TURBOSTAT_VERSION).tar.xz && mv linux-$(TURBOSTAT_VERSION)/ linux_turbostat/
	sed -i '/_Static_assert/d' linux_turbostat/tools/power/x86/turbostat/turbostat.c
	cd linux_turbostat/tools/power/x86/turbostat && make

zlib-aarch64:
ifeq ("$(wildcard zlib-aarch64)","")
	git clone https://github.com/madler/zlib.git zlib-aarch64
endif
	cd zlib-aarch64 && CHOST=aarch64-linux-gnu ./configure --archs="" --static
	cd zlib-aarch64 && make -j$(NPROC)
	cd zlib-aarch64 && cp libz.a /usr/lib/aarch64-linux-gnu/

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

# not used in build but required in oss archive file because some of the tools are statically linked
glibc-2.19.tar.bz2:
	wget http://ftp.gnu.org/gnu/glibc/glibc-2.19.tar.bz2
zlib.tar.gz:
	wget https://github.com/madler/zlib/archive/refs/heads/master.tar.gz -O zlib.tar.gz
libcrypt.tar.gz:
	wget https://github.com/gpg/libgcrypt/archive/refs/heads/master.tar.gz -O libcrypt.tar.gz
libs: glibc-2.19.tar.bz2 zlib.tar.gz libcrypt.tar.gz

oss-source: reset libs
	tar --exclude-vcs -czf oss_source.tgz async-profiler/ cpuid/ dmidecode/ ethtool/ fio/ ipmitool/ lshw/ lspci/ msr-tools/ perf-archive/ pcm/ spectre-meltdown-checker/ sshpass/ stress-ng/ sysstat/ linux_turbostat/tools/power/x86/turbostat glibc-2.19.tar.bz2 zlib.tar.gz libcrypt.tar.gz
	md5sum oss_source.tgz > oss_source.tgz.md5
