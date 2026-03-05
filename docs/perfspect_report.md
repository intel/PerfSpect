# perfspect report

```text
Collect configuration data from target(s)

Usage: perfspect report [flags]

Examples:
  Data from local host:          $ perfspect report
  Specific data from local host: $ perfspect report --bios --os --cpu --format html,json
  All data from remote target:   $ perfspect report --target 192.168.1.1 --user fred --key fred_key
  Data from multiple targets:    $ perfspect report --targets targets.yaml

Flags:
  Categories:
    --all                  report configuration for all categories (default: true)
    --system-summary       System Summary (default: false)
    --host                 Host (default: false)
    --bios                 BIOS (default: false)
    --os                   Operating System (default: false)
    --software             Software Versions (default: false)
    --cpu                  Processor Details (default: false)
    --prefetcher           Prefetchers (default: false)
    --isa                  Instruction Sets (default: false)
    --accelerator          On-board Accelerators (default: false)
    --power                Power Settings (default: false)
    --cstates              C-states (default: false)
    --frequency            Maximum Frequencies (default: false)
    --sst                  Speed Select Technology Settings (default: false)
    --uncore               Uncore Configuration (default: false)
    --elc                  Efficiency Latency Control Settings (default: false)
    --memory               Memory Configuration (default: false)
    --dimm                 DIMM Population (default: false)
    --netconfig            Network Configuration (default: false)
    --nic                  Network Cards (default: false)
    --disk                 Storage Devices (default: false)
    --filesystem           File Systems (default: false)
    --gpu                  GPUs (default: false)
    --gaudi                Gaudi Devices (default: false)
    --cxl                  CXL Devices (default: false)
    --pcie                 PCIE Slots (default: false)
    --cve                  Vulnerabilities (default: false)
    --process              Process List (default: false)
    --sensor               Sensor Status (default: false)
    --chassisstatus        Chassis Status (default: false)
    --pmu                  Performance Monitoring Unit Status (default: false)
    --sel                  System Event Log (default: false)
    --kernellog            Kernel Log (default: false)
  Other Options:
    --format               choose output format(s) from: all, html, xlsx, json, txt (default: [all])
  Remote Target Options:
    --target               host name or IP address of remote target
    --port                 port for SSH to remote target
    --user                 user name for SSH to remote target
    --key                  private key file for SSH to remote target
    --targets              file with remote target(s) connection details. See targets.yaml for format.
  Advanced Options:
    --input                ".raw" file, or directory containing ".raw" files. Will skip data collection and use raw data for reports.

Global Flags:
  --debug                enable debug logging and retain temporary directories (default: false)
  --log-stdout           write logs to stdout (default: false)
  --noupdate             skip application update check (default: false)
  --output               override the output directory
  --syslog               write logs to syslog instead of a file (default: false)
  --tempdir              override the temporary target directory, must exist and allow execution
```
