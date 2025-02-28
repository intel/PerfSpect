# Intel&reg; PerfSpect

[![Build](https://github.com/intel/PerfSpect/actions/workflows/build-test.yml/badge.svg)](https://github.com/intel/PerfSpect/actions/workflows/build-test.yml)[![CodeQL](https://github.com/intel/PerfSpect/actions/workflows/codeql.yml/badge.svg)](https://github.com/intel/PerfSpect/actions/workflows/codeql.yml)[![License](https://img.shields.io/badge/License-BSD--3-blue)](https://github.com/intel/PerfSpect/blob/master/LICENSE)

[Getting PerfSpect](#getting-perfspect) | [Running PerfSpect](#running-perfspect) | [Building PerfSpect](#building-perfspect-from-source)

Intel&reg; PerfSpect is a command-line tool designed to help you analyze and optimize Linux servers and the software running on them. Whether you’re a system administrator, a developer, or a performance engineer, PerfSpect provides comprehensive insights and actionable recommendations to enhance performance and efficiency.

We welcome bug reports and enhancement requests, which can be submitted via the [Issues](https://github.com/intel/PerfSpect/issues) section on GitHub. For those interested in contributing to the code, please refer to the guidelines outlined in the [CONTRIBUTING.md](CONTRIBUTING.md) file.

## Getting PerfSpect
```
wget -qO- https://github.com/intel/PerfSpect/releases/latest/download/perfspect.tgz | tar xvz
cd perfspect
```
## Running PerfSpect
PerfSpect includes a suite of commands designed to analyze and optimize both system and software performance.
<pre>
Usage:
  perfspect [command] [flags]
</pre>

### Commands
| Command | Description |
| ------- | ----------- |
| [`metrics`](#metrics-command) | Report core and uncore metrics |
| [`report`](#report-command) | Report configuration and health |
| [`telemetry`](#telemetry-command) | Report system telemetry |
| [`flame`](#flame-command) | Generate software call-stack flamegraphs |
| [`config`](#config-command) | Modify system configuration |
| [`lock`](#lock-command) | Collect system wide hot spot, c2c and lock contention information |

> [!TIP]
> Run `perfspect [command] -h` to view command-specific help text.

#### Metrics Command
The `metrics` command generates reports containing CPU architectural performance characterization metrics in HTML and CSV formats. Run `perfspect metrics`.

![metrics html TMA](docs/metrics_html_tma.png)

##### Live Metrics
The `metrics` command supports two modes -- default and "live". Default mode behaves as above -- metrics are collected and saved into report files for review.  The "live" mode prints the metrics to stdout where they can be viewed in the console and/or redirected into a file or observability pipeline. Run `perfspect metrics --live`.

![live metrics](docs/metrics_live.png)

##### Metrics Without Root Permissions
If sudo is not possible and running as the root user is not possible, the following configuration needs to be applied to the target system(s) by an administrator:
- sysctl -w kernel.perf_event_paranoid=0
- sysctl -w kernel.nmi_watchdog=0
- write '125' to all perf_event_mux_interval_ms files found under /sys/devices/*, e.g., `for i in $(find /sys/devices -name perf_event_mux_interval_ms); do echo 125 > $i; done`

Once the configuration changes are applied, use the `--noroot` flag on the command line, e.g., `perfspect metrics --noroot`.

See `perfspect metrics -h` for the extensive set of options and examples.

#### Report Command
The `report` command generates system configuration reports in a variety of formats. By default, all categories of information are collected. See `perfspect report -h` for all options.

![report html](docs/report_html.png)

It's possible to collect a subset of information by providing command line options. Note that by specifying only the `txt` format, it is printed to stdout, as well as written to a report file.
<pre>
$ ./perfspect report --bios --format txt
BIOS
====
Vendor:       Intel Corporation
Version:      EGSDCRB1.SYS.1752.P05.2401050248
Release Date: 01/05/2024
</pre>
##### Report Benchmarks
To assist in evaluating the health of target systems, the `report` command can run a series of micro-benchmarks by applying the `--benchmark` flag, e.g., `perfspect report --benchmark all` The benchmark results will be reported along with the target's configuration details. 

> [!IMPORTANT]
> Benchmarks should be run on idle systems to measure accurately and to avoid interfering with active workloads.

| benchmark | Description |
| --------- | ----------- |
| all | runs all benchmarks |
| speed | runs each [stress-ng](https://github.com/ColinIanKing/stress-ng) cpu-method for 1s each, reports the geo-metric mean of all results. |
| power | runs stress-ng in two stages: 1) load 1 cpu to 100% for 20s to measure maximum frequency, 2) load all cpus to 100% for 60s. Uses [turbostat](https://github.com/torvalds/linux/tree/master/tools/power/x86/turbostat) to measure power. |
| temperature | runs the same micro benchmark as 'power', but extracts maximum temperature from turbostat output. |
| frequency | runs [avx-turbo](https://github.com/travisdowns/avx-turbo) to measure scalar and AVX frequencies across processor's cores. **Note:** Runtime increases with core count.  |
| memory | runs [Intel(r) Memory Latency Checker](https://www.intel.com/content/www/us/en/download/736633/intel-memory-latency-checker-intel-mlc.html) (MLC) to measure memory bandwidth and latency across a load range. **Note: MLC is not included with PerfSpect.** It can be downloaded from [here](https://www.intel.com/content/www/us/en/download/736633/intel-memory-latency-checker-intel-mlc.html). Once downloaded, extract the Linux executable and place it in the perfspect/tools/x86_64 directory. |
| numa | runs Intel(r) Memory Latency Checker(MLC) to measure bandwidth between NUMA nodes. See Note above about downloading MLC. |
| storage | runs [fio](https://github.com/axboe/fio) for 2 minutes in read/write mode with a single worker to measure single-thread read and write bandwidth. Use the --storage-dir flag to override the default location. Minimum 5GB disk space required to run test. |

#### Telemetry Command
The `telemetry` command reports CPU utilization, instruction mix, disk stats, network stats, and more on the specified target(s). By default, all telemetry types are collected. To choose telemetry types, see the additional command line options (`perfspect telemetry -h`).

![telemetry html](docs/telemetry_html.png)

#### Flame Command
Software flamegraphs are useful in diagnosing software performance bottlenecks. Run `perfspect flame` to capture a system-wide software flamegraph.

> [!NOTE]
> Perl must be installed on the target system to process the data required for flamegraphs.

![flame graph](docs/flamegraph.png)

#### Config Command
The `config` command provides a method to view and change various system configuration parameters. Run `perfspect config -h` to view the parameters that can be modified. 

> [!WARNING]
> It is possible to configure the system in a way that it will no longer operate. In some cases, a reboot will be required to return to default settings. 

Example:
<pre>
$ ./perfspect config --cores 24 --llc 2.0 --uncoremaxfreq 1.8
...
</pre>

#### Lock Command
As systems contain more and more cores, it can be useful to analyze the Linux kernel lock overhead and potential false-sharing that impacts system scalability. Run `perfspect lock` to collect system wide hot spot, cache-to-cache and lock contention information. Experienced performance engineers can analyze the collected information to identify bottlenecks.

### Common Command Options

#### Local vs. Remote Targets
By default, PerfSpect targets the local host, i.e., the host where PerfSpect is running. Remote system(s) can also be targetted when the remote systems are reachable through SSH from the local host.

> [!NOTE]
> Ensure the remote user has password-less sudo access (or root privileges) to fully utilize PerfSpect's capabilities.

To target a single remote system using a pre-configured private key:
<pre>
$ ./perfspect report --target 192.168.1.42 --user fred --key ~/.ssh/fredkey
...
</pre>
To target a single remote system using a password:
<pre>
$ ./perfspect report --target 192.168.1.42 --user fred
fred@192.168.1.42's password: ******
...
</pre>
To target more than one remote system, a YAML file with the necessary connection parameters is provided to PerfSpect. See the example YAML file [targets.yaml](targets.yaml).
<pre>
$ ./perfspect report --targets mytargets.yaml
...
</pre>
## Building PerfSpect from Source
> [!TIP]
> Skip the build. Pre-built PerfSpect releases are available in the repository's [Releases](https://github.com/intel/PerfSpect/releases). Download and extract perfspect.tgz.
### 1st Build
`builder/build.sh` builds the dependencies and the app in Docker containers that provide the required build environments. Assumes you have Docker installed on your development system.

### Subsequent Builds
`make` builds the app. Assumes the dependencies have been built previously and that you have Go installed on your development system.
