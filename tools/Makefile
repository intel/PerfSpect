#!make
#
# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
#

default: tools
.PHONY: default tools async-profiler avx-turbo cpuid dmidecode ethtool fio ipmitool lshw lspci msr-tools pcm perf perf-archive processwatch spectre-meltdown-checker sshpass stackcollapse-perf stress-ng sysstat tsc turbostat

tools: async-profiler avx-turbo cpuid dmidecode ethtool fio ipmitool lshw lspci msr-tools pcm perf-archive spectre-meltdown-checker sshpass stackcollapse-perf stress-ng sysstat tsc turbostat
	mkdir -p bin
	cp -R async-profiler bin/
	cp avx-turbo/avx-turbo bin/
	cp cpuid/cpuid bin/
	cp dmidecode/dmidecode bin/
	cp ethtool/ethtool bin/
	cp fio/fio bin/
	cp stackcollapse-perf/stackcollapse-perf bin/
	cp ipmitool/src/ipmitool.static bin/ipmitool
	cp lshw/src/lshw-static bin/lshw
	cp lspci/lspci bin/
	cp lspci/pci.ids.gz bin/
	cp msr-tools/rdmsr bin/
	cp msr-tools/wrmsr bin/
	cp pcm/build/bin/pcm-tpmi bin/
	cp pcm/scripts/bhs-power-mode.sh bin/
	cp spectre-meltdown-checker/spectre-meltdown-checker.sh bin/
	cp sshpass/sshpass bin/
	cp stress-ng/stress-ng bin/
	cp sysstat/mpstat bin/
	cp sysstat/iostat bin/
	cp sysstat/sar bin/
	cp sysstat/sadc bin/
	cp tsc/tsc bin/
	cp linux_turbostat/tools/power/x86/turbostat/turbostat bin/
	-cd bin && strip --strip-unneeded *

ASYNC_PROFILER_VERSION := "4.0"
async-profiler:
ifeq ("$(wildcard async-profiler)","")
ifeq ("$(wildcard async-profiler-$(ASYNC_PROFILER_VERSION)-linux-x64.tar.gz)","")
	wget https://github.com/jvm-profiling-tools/async-profiler/releases/download/v$(ASYNC_PROFILER_VERSION)/async-profiler-$(ASYNC_PROFILER_VERSION)-linux-x64.tar.gz
endif
	tar -xf async-profiler-$(ASYNC_PROFILER_VERSION)-linux-x64.tar.gz && mv async-profiler-$(ASYNC_PROFILER_VERSION)-linux-x64 async-profiler
endif

AVX_TURBO_VERSION := "threadcpuid"
avx-turbo:
ifeq ("$(wildcard avx-turbo)","")
	git clone https://github.com/harp-intel/avx-turbo.git
else
	cd avx-turbo && git checkout master && git pull
endif
	cd avx-turbo && git checkout $(AVX_TURBO_VERSION) && git pull
	cd avx-turbo && make

# if you change the version, check the sed hacks below
CPUID_VERSION := "20240409"
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
	cd dmidecode && make

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
	cd ethtool && make

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
	cd fio && make

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
	cd ipmitool && make
	cd ipmitool/src && ../libtool --silent --tag=CC --mode=link gcc -fno-strict-aliasing -Wreturn-type -all-static -o ipmitool.static ipmitool.o ipmishell.o ../lib/libipmitool.la plugins/libintf.la

LSHW_VERSION := "B.02.19"
lshw:
ifeq ("$(wildcard lshw)","")
	git clone https://github.com/lyonel/lshw.git
else
	cd lshw && git checkout master && git pull
endif
	cd lshw && git checkout $(LSHW_VERSION)
	cd lshw/src && make static

LSPCI_VERSION := "v3.13.0"
lspci:
ifeq ("$(wildcard lspci)","")
	git clone https://github.com/pciutils/pciutils.git lspci
else
	cd lspci && git checkout master && git pull
endif
	cd lspci && git checkout $(LSPCI_VERSION)
	cd lspci && make
	cd lspci && ./update-pciids.sh
	cd lspci && gzip -c pci.ids > pci.ids.gz

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

PERF_VERSION := "6.8.12"
perf:
	wget https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-$(PERF_VERSION).tar.xz
	tar -xf linux-$(PERF_VERSION).tar.xz && mv linux-$(PERF_VERSION)/ linux_perf/
	cd linux_perf/tools/perf && make LDFLAGS="-static --static" BUILD_BPF_SKEL=1 NO_JVMTI=1
	mkdir -p bin
	cp linux_perf/tools/perf/perf bin/
	strip --strip-unneeded bin/perf
#	cp linux_perf/tools/perf/perf-archive.sh bin/perf-archive
#	chmod +x bin/perf-archive

perf-archive:
	mkdir -p bin
	cp perf-archive/perf-archive.sh bin/perf-archive
	chmod +x bin/perf-archive

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
	cp processwatch/processwatch bin/
	strip --strip-unneeded bin/processwatch

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
	cd sshpass && make

stackcollapse-perf:
	cd stackcollapse-perf && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build

STRESS_NG_VERSION := "V0.13.08"
stress-ng:
ifeq ("$(wildcard stress-ng)","")
	git clone https://github.com/ColinIanKing/stress-ng.git
else
	cd stress-ng && git checkout master && git pull
endif
	cd stress-ng && git checkout $(STRESS_NG_VERSION)
	cd stress-ng && STATIC=1 make

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
	cd sysstat && make

tsc:
	cd tsc && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build

TURBOSTAT_VERSION := "6.9.12"
turbostat:
	wget https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-$(TURBOSTAT_VERSION).tar.xz
	tar -xf linux-$(TURBOSTAT_VERSION).tar.xz && mv linux-$(TURBOSTAT_VERSION)/ linux_turbostat/
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

# not used in build but required in oss archive file because some of the tools are statically linked
glibc-2.19.tar.bz2:
	wget http://ftp.gnu.org/gnu/glibc/glibc-2.19.tar.bz2
zlib.tar.gz:
	wget https://github.com/madler/zlib/archive/refs/heads/master.tar.gz -O zlib.tar.gz
libcrypt.tar.gz:
	wget https://github.com/gpg/libgcrypt/archive/refs/heads/master.tar.gz -O libcrypt.tar.gz
libs: glibc-2.19.tar.bz2 zlib.tar.gz libcrypt.tar.gz

oss-source: reset libs
	tar --exclude-vcs -czf oss_source.tgz async-profiler/ cpuid/ dmidecode/ ethtool/ fio/ ipmitool/ lshw/ lspci/ msr-tools/ pcm/ spectre-meltdown-checker/ sshpass/ stress-ng/ sysstat/ linux_turbostat/tools/power/x86/turbostat glibc-2.19.tar.bz2 zlib.tar.gz libcrypt.tar.gz
	md5sum oss_source.tgz > oss_source.tgz.md5
