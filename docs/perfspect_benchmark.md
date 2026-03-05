# perfspect benchmark

```text
Run performance benchmarks on target(s)

Usage: perfspect benchmark [flags]

Examples:
  Run all benchmarks:        $ perfspect benchmark
  Run specific benchmarks:   $ perfspect benchmark --speed --power
  Benchmark remote target:   $ perfspect benchmark --target 192.168.1.1 --user fred --key fred_key
  Benchmark multiple targets:$ perfspect benchmark --targets targets.yaml

Flags:
  Benchmark Options:
    --all                  run all benchmarks (default: true)
    --speed                CPU speed benchmark (default: false)
    --power                power consumption benchmark (default: false)
    --temperature          temperature benchmark (default: false)
    --frequency            turbo frequency benchmark (default: false)
    --memory               memory latency and bandwidth benchmark (default: false)
    --numa                 NUMA bandwidth matrix benchmark (default: false)
    --storage              storage performance benchmark (default: false)
  Other Options:
    --no-summary           do not include system summary in output (default: false)
    --storage-dir          existing directory where storage performance benchmark data will be temporarily stored (default: /tmp)
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
