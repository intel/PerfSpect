# perfspect flamegraph

```text
Collect flamegraph data from target(s)

Usage: perfspect flamegraph [flags]

Examples:
  Flamegraph from local host:       $ perfspect flamegraph
  Flamegraph from remote target:    $ perfspect flamegraph --target 192.168.1.1 --user fred --key fred_key
  Flamegraph from multiple targets: $ perfspect flamegraph --targets targets.yaml
  Flamegraph for cache misses:      $ perfspect flamegraph --perf-event cache-misses

Flags:
  Options:
    --duration             number of seconds to run the collection. If 0, the collection will run indefinitely. Ctrl+c to stop. (default: 0)
    --frequency            number of samples taken per second (default: 11)
    --pids                 comma separated list of PIDs. If not specified, all PIDs will be collected (default: [])
    --perf-event           perf event to use for native sampling (e.g., cpu-cycles, instructions, cache-misses, branches, context-switches, mem-loads, mem-stores, etc.) (default: cycles:P)
    --asprof-args          arguments to pass to async-profiler, e.g., $ asprof start <these arguments> -i <interval> <pid>. (default: -t -F probesp+vtable)
    --max-depth            maximum render depth of call stack in flamegraph (0 = no limit) (default: 0)
    --format               choose output format(s) from: all, html, txt, json (default: [html])
    --no-summary           do not include system summary table in report (default: false)
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
