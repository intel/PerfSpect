# PerfSpect &middot; [![Build](https://github.com/intel/PerfSpect/actions/workflows/build.yml/badge.svg)](https://github.com/intel/PerfSpect/actions/workflows/build.yml)[![CodeQL](https://github.com/intel/PerfSpect/actions/workflows/codeql.yml/badge.svg)](https://github.com/intel/PerfSpect/actions/workflows/codeql.yml)[![License](https://img.shields.io/badge/License-BSD--3-blue)](https://github.com/intel/PerfSpect/blob/master/LICENSE)

[Quick Start](#quick-start-requires-perf-installed) | [Output](#output) | [Requirements](#requirements) | [Build from source](#build-from-source)

PerfSpect is a system performance characterization tool built on top of linux perf. Most metrics and events come from [perfmon](https://github.com/intel/perfmon) and [TMA v4.5](https://www.intel.com/content/www/us/en/docs/vtune-profiler/cookbook/2023-1/top-down-microarchitecture-analysis-method.html). It contains two parts:

perf-collect: Collects hardware events at a 5 second output interval with practically zero overhead since PMU's run in counting  mode.

- Collection mode:
  - `sudo ./perf-collect` _default system wide_
  - `sudo ./perf-collect --socket`
  - `sudo ./perf-collect --cpu`
  - `sudo ./perf-collect --pid <process-id>`
  - `sudo ./perf-collect --cid` _by default, selects the 5 containers using the most CPU at start of perf-collect. To monitor specific containers provide up to 5 comma separated cids i.e. <cid_1>,<cid_2>_
- Duration:
  - `sudo ./perf-collect` _default run until terminated_
  - `sudo ./perf-collect --timeout 10` _run for 10 seconds_
  - `sudo ./perf-collect --app "myapp.sh myparameter"` _runs for duration of another process_

perf-postprocess: Calculates high level metrics from hardware events

- `./perf-postprocess`

## Quick start (requires perf installed)

```
wget -qO- https://github.com/intel/PerfSpect/releases/latest/download/perfspect.tgz | tar xvz
cd perfspect
sudo ./perf-collect --timeout 10
./perf-postprocess
```

## Output

perf-collect outputs:
1. `perfstat.csv`: raw event counts with system metadata

perf-postprocess outputs:
1. `metric_out.sys.average.csv`: average metrics
2. `metric_out.sys.csv`: metric values at every 5 second interval
3. `metric_out.html`: html view of a few select metrics

[live example report](https://intel.github.io/PerfSpect/)

![basic_stats](https://raw.githubusercontent.com/wiki/intel/PerfSpect/newhtml.gif)

## Deploy in Kubernetes

Modify the template [deamonset.yml](docs/daemonset.yml) to deploy in kubernetes

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
