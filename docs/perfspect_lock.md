# perfspect lock

```text
Collect kernel lock data from target(s)

Usage: perfspect lock [flags]

Examples:
  Lock inspect from local host:       $ perfspect lock
  Lock inspect from remote target:    $ perfspect lock --target 192.168.1.1 --user fred --key fred_key
  Lock inspect from multiple targets: $ perfspect lock --targets targets.yaml

Flags:
  Options:
    --duration             number of seconds to run the collection (default: 10)
    --frequency            number of samples taken per second (default: 11)
    --package              create raw data package (default: false)
    --format               choose output format(s) from: all, html, txt (default: [all])
    --no-summary           do not include system summary table in report (default: false)
  Remote Target Options:
    --target               host name or IP address of remote target
    --port                 port for SSH to remote target
    --user                 user name for SSH to remote target
    --key                  private key file for SSH to remote target
    --targets              file with remote target(s) connection details. See targets.yaml for format.

Global Flags:
  --debug                enable debug logging and retain temporary directories (default: false)
  --log-stdout           write logs to stdout (default: false)
  --noupdate             skip application update check (default: false)
  --output               override the output directory
  --syslog               write logs to syslog instead of a file (default: false)
  --tempdir              override the temporary target directory, must exist and allow execution
```
