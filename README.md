# PerfSpect &middot; [![Build](https://github.com/intel/PerfSpect/actions/workflows/build.yml/badge.svg)](https://github.com/intel/PerfSpect/actions/workflows/build.yml)[![License](https://img.shields.io/badge/License-BSD--3-blue)](https://github.com/intel/PerfSpect/blob/master/LICENSE)

[Quick Start](#quick-start-requires-perf-installed) | [Requirements](#requirements) | [Build from source](#build-from-source) | [Collection](#collection) | [Post-processing](#post-processing) | [Caveats](#caveats) | [How to contribute](#how-to-contribute)

PerfSpect is a system performance characterization tool based on linux perf targeting Intel microarchitectures.  
The tool has two parts

1. perf collection to collect underlying PMU (Performance Monitoring Unit) counters
2. post processing that generates csv output of performance metrics.

## Quick start (requires perf installed)
```
wget -qO- https://github.com/intel/PerfSpect/releases/latest/download/perfspect.tgz | tar xvz
cd perfspect
sudo ./perf-collect --timeout 10
sudo ./perf-postprocess -r results/perfstat.csv --html perfstat.html
```

![basic_stats](https://raw.githubusercontent.com/wiki/intel/PerfSpect/basic_stats.JPG)
![perfspect-demo1](https://raw.githubusercontent.com/wiki/intel/PerfSpect/demo.gif)

## Requirements
### Packages:
- **perf** - PerfSpect uses the Linux perf tool to collect PMU counters
- **lscgroup** - Perfspect needs lscgroup from the cgroup-tools (libcgroup on RHEL/CentOS) package when collecting data for containers

### Supported kernels

| Xeon Generation | Minimum Kernel |
| - | - |
| Broadwell | kernel 4.15 |
| Skylake | kernel 4.15 |
| Cascadelake | kernel 4.15 |
| Icelake | kernel 5.9 |
| Sapphire Rapids | kernel 5.12 |

### Supported Operating Systems:
- Ubuntu 16.04 and newer
- centos 7 and newer
- Amazon Linux 2
- RHEL 9
- Debian 11

*Note: PerfSpect may work on other Linux distributions, but has not been thoroughly tested*

## Build from source

Requires recent python and golang.

```
pip3 install -r requirements.txt
make
```

On successful build, binaries will be created in "dist" folder

## Collection:

```
(sudo) ./perf-collect (options)  -- Some options can be used only with root privileges

Options:
  -h, --help            show this help message and exit
  -v, --version         display version info
  -e EVENTFILE, --eventfile EVENTFILE
                        Event file containing events to collect,
                        default=events/<architecture specific file>
  -i INTERVAL, --interval INTERVAL
                        interval in seconds for time series dump, default=1
  -m MUXINTERVAL, --muxinterval MUXINTERVAL
                        event mux interval in milli seconds, default=0 i.e. will
                        use the system default
  -o OUTCSV, --outcsv OUTCSV
                        perf stat output in csv format,
                        default=results/perfstat.csv
  -a APP, --app APP     Application to run with perf-collect, perf collection ends
                        after workload completion
  -p PID, --pid PID     perf-collect on selected PID(s)
  -c CID, --cid CID     perf-collect on selected container ids
  -t TIMEOUT, --timeout TIMEOUT
                        perf event collection time
  --percore             Enable per core event collection
  --nogroups            Disable perf event grouping, events are grouped by default
                        as in the event file
  --dryrun              Test if Performance Monitoring Counters are in-use, and
                        collect stats for 10sec to validate event file correctness
  --metadata            collect system info only, does not run perf
  -csp CLOUD, --cloud CLOUD
                        Name of the Cloud Service Provider(AWS), if collecting on
                        cloud instances. Currently supporting AWS and OCI
  -ct CLOUDTYPE, --cloudtype CLOUDTYPE
                        Instance type: Options include - VM,BM
```

### Examples

1. sudo ./perf-collect (collect PMU counters using predefined architecture specific event file until collection is terminated)
2. sudo ./perf-collect -m 10 -t 30 (sets event multiplexing interval to 10ms and collects PMU counters for 30 seconds using default architecture specific event file)
3. sudo ./perf-collect -a "myapp.sh myparameter" (collect perf for myapp.sh)
4. sudo ./perf-collect --dryrun (checks PMU usage, and collects PMU counters for 10 seconds using default architecture specific event file)
5. sudo ./perf-collect --metadata (collect system info and PMU event info without running perf, uses default outputfile if -o option is not used)
6. sudo ./perf-collect --cid "one or more container IDs from docker or kubernetes seperated by semicolon"

### Notes

1. Intel CPUs (until Cascadelake) have 3 fixed PMUs (cpu-cycles, ref-cycles, instructions) and 4 programmable PMUs. The events are grouped in event files with this assumption. However, some of the counters may not be available on some CPUs. You can check the correctness of the event file with dryrun and check the output for anamolies. Typically output will have "not counted", "unsuppported" or zero values for cpu-cycles if number of available counters are less than events in a group.
2. Globally pinned events can limit the number of counters available for perf event groups. On X86 systems NMI watchdog pins a fixed counter by default. NMI watchdog is disabled during perf collection if run as a sudo user. If NMI watchdog can't be disabled, event grouping will be forcefully disabled to let perf driver handle event multiplexing.

## Post-processing:

```
./perf-postprocess (options)

Options:
  -h, --help            show this help message and exit
  --version, -v         display version information
  -m METRICFILE, --metricfile METRICFILE
                        formula file, default metric file for the architecture
  -o OUTFILE, --outfile OUTFILE
                        perf stat outputs in csv format,
                        default=results/metric_out.csv
  --persocket           generate per socket metrics
  --percore             generate per core metrics
  --keepall             keep all intermediate csv files, use it for debug purpose
                        only
  --epoch               time series in epoch format, default is sample count
  -csp CLOUD, --cloud CLOUD
                        Name of Cloud Service Provider(AWS), if you're intending
                        to postprocess on cloud instances
  -html HTML, --html HTML
                        Static HTML report

required arguments:
  -r RAWFILE, --rawfile RAWFILE
                        Raw CSV output from perf-collect
```

### Examples

./perf-postprocess -r results/perfstat.csv (post processes perfstat.csv and creates metric_out.csv, metric_out.average.csv, metric_out.raw.csv)

./perf-postprocess -r results/perfstat.csv --html perfstat.html (creates a report for TMA analysis and system level metric charts.)

### Notes

1. metric_out.csv : Time series dump of the metrics. The metrics are defined in events/metric.json
2. metric_out.averags.csv: Average of metrics over the collection period
3. metric_out.raw.csv: csv file with raw events normalized per second
4. Socket/core level metrics: Additonal csv files outputfile.socket.csv/outputfile.core.csv will be generated. Socket/core level data will be added as new sheets if excel output is chosen

## Caveats

1. The tool can collect only the counters supported by underlying linux perf version.
2. Current version supports Intel Sapphire Rapids, Icelake, Cascadelake, Skylake and Broadwell microarchitectures only.
3. Perf collection overhead will increase with increase in number of counters and/or dump interval. Using the right perf multiplexing (check perf-collection.py Notes for more details) interval to reduce overhead
4. If you run into locale issues - `UnicodeDecodeError: 'ascii' codec can't decode byte 0xc2 in position 4519: ordinal not in range(128)`, more than likely the locales needs to be set appropriately. You could also try running post-process step with `LC_ALL=C.UTF-8 LANG=C.UTF-8 ./perf-postprocess -r result.csv`
5. The percore option is not supported while using cid.
6. The html report creation is not yet supported for cid collection.

## How to contribute

Create a pull request on github.com/intel/PerfSpect with your patch. Please make sure your patch is building without errors. A maintainer will contact you if there are questions or concerns.
