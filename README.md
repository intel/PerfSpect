# PerfSpect &middot; [![Build](https://github.com/intel/PerfSpect/actions/workflows/build.yml/badge.svg)](https://github.com/intel/PerfSpect/actions/workflows/build.yml)[![License](https://img.shields.io/badge/License-BSD--3-blue)](https://github.com/intel/PerfSpect/blob/master/LICENSE)

[Quick Start](#quick-start-requires-perf-installed) | [Requirements](#requirements) | [Build from source](#build-from-source) | [Collection](#collection) | [Post-processing](#post-processing) | [Caveats](#caveats) | [How to contribute](#how-to-contribute)

PerfSpect is a system performance characterization tool based on linux perf targeting Intel microarchitectures.  
The tool has two parts

1. perf collection to collect underlying PMU (Performance Monitoring Unit) counters
2. post processing that generates csv output of performance metrics.

### Quick start (requires perf installed)

```
wget -qO- https://github.com/intel/PerfSpect/releases/latest/download/perfspect.tgz | tar xvz
cd perfspect
sudo ./perf-collect --timeout 10
sudo ./perf-postprocess -r results/perfstat.csv --html perfstat.html
```

### Deploy in Kubernetes

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

Requires recent python

```
pip3 install -r requirements.txt
make
```

On successful build, binaries will be created in `dist` folder

## Collection:

```
(sudo) ./perf-collect (options)  -- Some options can be used only with root privileges

usage: perf-collect [-h] [-t TIMEOUT | -a APP]
                    [-p PID | -c CID | --thread | --socket] [-V] [-i INTERVAL]
                    [-m MUXINTERVAL] [-o OUTCSV] [-v]

optional arguments:
  -h, --help            show this help message and exit
  -t TIMEOUT, --timeout TIMEOUT
                        perf event collection time
  -a APP, --app APP     Application to run with perf-collect, perf collection
                        ends after workload completion
  -p PID, --pid PID     perf-collect on selected PID(s)
  -c CID, --cid CID     perf-collect on selected container ids
  --thread              Collect for thread metrics
  --socket              Collect for socket metrics
  -V, --version         display version info
  -i INTERVAL, --interval INTERVAL
                        interval in seconds for time series dump, default=1
  -m MUXINTERVAL, --muxinterval MUXINTERVAL
                        event mux interval in milli seconds, default=0 i.e.
                        will use the system default
  -o OUTCSV, --outcsv OUTCSV
                        perf stat output in csv format,
                        default=results/perfstat.csv
  -v, --verbose         Display debugging information
```

### Examples

1. sudo ./perf-collect (collect PMU counters using predefined architecture specific event file until collection is terminated)
2. sudo ./perf-collect -a "myapp.sh myparameter" (collect perf for myapp.sh)
3. sudo ./perf-collect --cid "one or more container IDs from docker or kubernetes seperated by semicolon"

## Post-processing:

```
./perf-postprocess (options)

usage: perf-postprocess [-h] [--version] [-m METRICFILE] [-o OUTFILE]
                        [--persocket] [--percore] [-v] [--epoch] [-html HTML]
                        [-r RAWFILE]

perf-postprocess: perf post process

optional arguments:
  -h, --help            show this help message and exit
  --version, -V         display version information
  -m METRICFILE, --metricfile METRICFILE
                        formula file, default metric file for the architecture
  -o OUTFILE, --outfile OUTFILE
                        perf stat outputs in csv format,
                        default=results/metric_out.csv
  --persocket           generate per socket metrics
  --percore             generate per core metrics
  -v, --verbose         include debugging information, keeps all intermediate
                        csv files
  --epoch               time series in epoch format, default is sample count
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
4. Socket/core level metrics: Additonal csv files outputfile.socket.csv/outputfile.core.csv will be generated.

## Caveats

1. The tool can collect only the counters supported by underlying linux perf version.
2. If you run into locale issues - `UnicodeDecodeError: 'ascii' codec can't decode byte 0xc2 in position 4519: ordinal not in range(128)`, more than likely the locales needs to be set appropriately. You could also try running post-process step with `LC_ALL=C.UTF-8 LANG=C.UTF-8 ./perf-postprocess -r result.csv`
3. The html report creation is not yet supported for cid collection.

## How to contribute

Create a pull request on github.com/intel/PerfSpect with your patch. Please make sure your patch is building without errors. A maintainer will contact you if there are questions or concerns.
