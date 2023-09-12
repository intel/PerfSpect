<div align="center">

<div id="user-content-toc">
  <ul>
    <summary><h1 style="display: inline-block;">PerfSpect</h1></summary>
  </ul>
</div>

[![Build](https://github.com/intel/PerfSpect/actions/workflows/build.yml/badge.svg)](https://github.com/intel/PerfSpect/actions/workflows/build.yml)[![CodeQL](https://github.com/intel/PerfSpect/actions/workflows/codeql.yml/badge.svg)](https://github.com/intel/PerfSpect/actions/workflows/codeql.yml)[![License](https://img.shields.io/badge/License-BSD--3-blue)](https://github.com/intel/PerfSpect/blob/master/LICENSE)

[![Static Badge](https://img.shields.io/badge/Live_Demo-red?style=for-the-badge)](https://intel.github.io/PerfSpect/)

[Quick Start](#quick-start-requires-perf-installed) | [Output](#output) | [Deploy in Kubernetes](#deploy-in-kubernetes) | [Requirements](#requirements) | [Build from source](#build-from-source)
</div>

PerfSpect is a system performance characterization tool built on top of linux perf. It contains two parts:

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

![basic_stats](https://raw.githubusercontent.com/wiki/intel/PerfSpect/newhtml.gif)

## Deploy in Kubernetes

Modify the template [deamonset.yml](docs/daemonset.yml) to deploy in kubernetes

## Requirements

**perf** - PerfSpect uses the Linux perf tool to collect PMU counters

Different events require different minimum kernels (PerfSpect will automatically collect only supported events)
1. Base (CPU util, CPI, Cache misses, etc.)
    - 3.10
2. Uncore (NUMA traffic, DRAM traffic, etc.)
    - 4.9
3. TMA (Micro-architecture boundness breakdown)
    - ICX, SPR: 5.10
    - BDX, SKX, CLX: 3.10

## Build from source

Requires recent python. On successful build, binaries will be created in `dist` folder

```
pip3 install -r requirements.txt
make
```

_Note: Most metrics and events come from [perfmon](https://github.com/intel/perfmon) and [TMA v4.5](https://www.intel.com/content/www/us/en/docs/vtune-profiler/cookbook/2023-1/top-down-microarchitecture-analysis-method.html)_
