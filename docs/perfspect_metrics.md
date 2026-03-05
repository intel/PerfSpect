# perfspect metrics

```text
Collect performance metrics from target(s)

Usage: perfspect metrics [flags] [-- application args]

Examples:
  Metrics from local host:                  $ perfspect metrics --duration 30
  Metrics from local host in CSV format:    $ perfspect metrics --format csv
  Metrics from remote host:                 $ perfspect metrics --target 192.168.1.1 --user fred --key fred_key
  Metrics for "hot" processes:              $ perfspect metrics --scope process
  Metrics for specified processes:          $ perfspect metrics --scope process --pids 1234,6789
  Start workload and collect metrics:       $ perfspect metrics -- /path/to/workload arg1 arg2
  Metrics adjusted for transaction rate:    $ perfspect metrics --txnrate 100
  "Live" metrics:                           $ perfspect metrics --live

Arguments:
  application (optional): path to an application to run and collect metrics for

Flags:
  Collection Options:
    --duration             number of seconds to run the collection. If 0, the collection will run indefinitely. (default: 0)
    --scope                scope of collection, options: system, process, cgroup (default: system)
    --pids                 comma separated list of process ids. If not provided while collecting in process scope, "hot" processes will be monitored. (default: [])
    --cids                 comma separated list of cids. If not provided while collecting at cgroup scope, "hot" cgroups will be monitored. (default: [])
    --filter               regular expression used to match process names or cgroup IDs when in process or cgroup scope and when --pids or --cids are not specified
    --count                maximum number of "hot" or "filtered" processes or cgroups to monitor (default: 5)
    --refresh              number of seconds to run before refreshing the "hot" or "filtered" process or cgroup list. If 0, the list will not be refreshed. (default: 30)
    --cpus                 range of CPUs to monitor. If not provided, all cores will be monitored.
  Output Options:
    --granularity          level of metric granularity. Only valid when collecting at system scope. Options: system, socket, cpu. (default: system)
    --format               output formats, options: txt, csv, json, wide (default: [csv])
    --live                 print metrics to stdout in one output format specified with the --format flag. No metrics files will be written. (default: false)
    --txnrate              number of transactions per second. Will divide relevant metrics by transactions/second. (default: 0)
    --prometheus-server    enable prometheus metrics server (default: false)
    --prometheus-server-addr address (e.g., host:port) to start Prometheus metrics server on (implies --prometheus-server true) (default: :9090)
  Advanced Options:
    --list                 show metric names available on this platform and exit (default: false)
    --metrics              a comma separated list of quoted metric names to include in output (default: [])
    --eventfile            perf event definition file. Will override default event definitions. Must be used with --metricfile.
    --metricfile           metric definition file. Will override default metric definitions. Must be used with --eventfile.
    --interval             event collection interval in seconds (default: 5)
    --muxinterval          multiplexing interval in milliseconds (default: 125)
    --noroot               do not elevate to root (default: false)
    --raw                  write raw perf events to file (default: false)
    --input                path to a file or directory with json file containing raw perf events. Will skip data collection and use raw data for reports.
    --no-summary           do not include system summary table in report (default: false)
  Remote Target Options:
    --target               host name or IP address of remote target
    --port                 port for SSH to remote target
    --user                 user name for SSH to remote target
    --key                  private key file for SSH to remote target
    --targets              file with remote target(s) connection details. See targets.yaml for format.

Subcommands:
  trim: Filter existing metrics to a time range

Global Flags:
  --debug                enable debug logging and retain temporary directories (default: false)
  --log-stdout           write logs to stdout (default: false)
  --noupdate             skip application update check (default: false)
  --output               override the output directory
  --syslog               write logs to syslog instead of a file (default: false)
  --tempdir              override the temporary target directory, must exist and allow execution
```
