# PerfSpect &middot; [![Build](https://github.com/intel/PerfSpect/actions/workflows/build.yml/badge.svg)](https://github.com/intel/PerfSpect/actions/workflows/build.yml)[![License](https://img.shields.io/badge/License-BSD--3-blue)](https://github.com/intel/PerfSpect/blob/master/LICENSE)

[Quick Start](#quick-start-requires-perf-installed) | [Requirements](#requirements) | [Build from source](#build-from-source) | [Caveats](#caveats) | [How to contribute](#how-to-contribute)

PerfSpect is a system performance characterization tool built on top of linux perf. It contains two parts

perf-collect: Collects harware events

- Collection mode:
  - `sudo ./perf-collect` _default system wide_
  - `sudo ./perf-collect --socket`
  - `sudo ./perf-collect --thread`
  - `sudo ./perf-collect --pid <process-id>`
  - `sudo ./perf-collect --cid <container-id1>;<container-id2>`
- Duration:
  - `sudo ./perf-collect` _default run until terminated_
  - `sudo ./perf-collect --timeout 10` _run for 10 seconds_
  - `sudo ./perf-collect --app "myapp.sh myparameter"` _runs for duration of another process_

perf-postprocess: Calculates high level metrics from hardware events

- `perf-postprocess -r results/perfstat.csv`

## Quick start (requires perf installed)

```
wget -qO- https://github.com/intel/PerfSpect/releases/latest/download/perfspect.tgz | tar xvz
cd perfspect
sudo ./perf-collect --timeout 10
sudo ./perf-postprocess -r results/perfstat.csv --html perfstat.html
```

## Deploy in Kubernetes

Modify the template [deamonset.yml](docs/daemonset.yml) to deploy in kubernetes

![basic_stats](https://raw.githubusercontent.com/wiki/intel/PerfSpect/basic_stats.JPG)
![perfspect-demo1](https://raw.githubusercontent.com/wiki/intel/PerfSpect/demo.gif)

## Requirements

### Packages:

- **perf** - PerfSpect uses the Linux perf tool to collect PMU counters

### Minimum supported kernels

| Xeon Generation | centos 7+ | ubuntu 16.04+ |
| --------------- | --------- | ------------- |
| Broadwell       | 3.10      | 4.15          |
| Skylake         | 3.10      | 4.15          |
| Cascadelake     | 3.10      | 4.15          |
| Icelake         | 3.10      | 4.15          |
| Sapphire Rapids | 5.12      | 5.12          |

### Supported Operating Systems:

- Ubuntu 16.04 and newer
- centos 7 and newer
- Amazon Linux 2
- RHEL 9
- Debian 11

_Note: PerfSpect may work on other Linux distributions, but has not been thoroughly tested_

## Build from source

Requires recent python. On successful build, binaries will be created in `dist` folder

```
pip3 install -r requirements.txt
make
```

## Caveats

1. The tool can collect only the counters supported by underlying linux perf version.
2. If you run into locale issues - `UnicodeDecodeError: 'ascii' codec can't decode byte 0xc2 in position 4519: ordinal not in range(128)`, more than likely the locales needs to be set appropriately. You could also try running post-process step with `LC_ALL=C.UTF-8 LANG=C.UTF-8 ./perf-postprocess -r result.csv`
3. The html report creation is not yet supported for cid collection.

## How to contribute

Create a pull request on github.com/intel/PerfSpect with your patch. Please make sure your patch is building without errors. A maintainer will contact you if there are questions or concerns.
