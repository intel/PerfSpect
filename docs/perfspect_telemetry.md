# perfspect telemetry

```text
Collect system telemetry from target(s)

Usage: perfspect telemetry [flags]

Examples:
  Telemetry from local host:       $ perfspect telemetry
  Telemetry from remote target:    $ perfspect telemetry --target 192.168.1.1 --user fred --key fred_key
  Memory telemetry for 60 seconds: $ perfspect telemetry --memory --duration 60
  Telemetry from multiple targets: $ perfspect telemetry --targets targets.yaml

Flags:
  Categories:
    --all                  collect telemetry for all categories (default: true)
    --cpu                  monitor cpu utilization (default: false)
    --ipc                  monitor IPC (default: false)
    --cstate               monitor C-States residency (default: false)
    --frequency            monitor cpu frequency (default: false)
    --power                monitor power (default: false)
    --temperature          monitor temperature (default: false)
    --memory               monitor memory (default: false)
    --network              monitor network (default: false)
    --storage              monitor storage (default: false)
    --irqrate              monitor IRQ rate (default: false)
    --kernel               monitor kernel (default: false)
    --instrmix             monitor instruction mix (default: false)
  Other Options:
    --format               choose output format(s) from: all, html, xlsx, json, txt (default: [html])
    --duration             number of seconds to run the collection. If 0, the collection will run indefinitely. Ctrl+c to stop. (default: 0)
    --interval             number of seconds between each sample (default: 2)
    --instrmix-pid         PID to monitor for instruction mix, no PID means all processes (default: 0)
    --instrmix-frequency   number of instructions between samples, default is 10,000,000 when collecting system wide and 100,000 when collecting for a specific PID (default: 10000000)
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
