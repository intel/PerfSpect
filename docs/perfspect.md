# perfspect

```text
PerfSpect (perfspect) is a multi-function utility for performance engineers analyzing software running on Intel Xeon platforms.

Usage:
  perfspect [command] [flags]

Examples:
  Generate a configuration report:                             $ perfspect report
  Collect micro-architectural metrics:                         $ perfspect metrics
  Generate a configuration report on a remote target:          $ perfspect report --target 192.168.1.2 --user elaine --key ~/.ssh/id_rsa
  Generate configuration reports for multiple remote targets:  $ perfspect report --targets ./targets.yaml

Use "perfspect [command] --help" for more information about a command.

Commands:
  report      Collect configuration data from target(s)
  benchmark   Run performance benchmarks on target(s)
  metrics     Collect performance metrics from target(s)
  telemetry   Collect system telemetry from target(s)
  flamegraph  Collect flamegraph data from target(s)
  lock        Collect kernel lock data from target(s)
  config      Modify system configuration on target(s)

Other Commands:
  update      Update the application (Intel network only)
  extract     Extract the embedded resources (for developers)

Flags:
      --debug            enable debug logging and retain temporary directories
  -h, --help             help for perfspect
      --log-stdout       write logs to stdout
      --noupdate         skip application update check
      --output string    override the output directory
      --syslog           write logs to syslog instead of a file
      --tempdir string   override the temporary target directory, must exist and allow execution
  -v, --version          version for perfspect

Additional help topics:
  perfspect            
```
