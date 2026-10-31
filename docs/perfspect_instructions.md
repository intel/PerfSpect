# perfspect instructions

```text
Analyze the instructions in hot code on target(s).

Samples the target with perf, then disassembles the hottest symbols and classifies each
instruction. Reports register spill and reload traffic, split by general-purpose versus
vector registers.

Requires symbols in the profiled binaries. Stripped binaries cannot be annotated and are
reported as such rather than analyzed.

Usage: perfspect instructions [flags]

Examples:
  Instruction analysis of local host:    $ perfspect instructions
  Instruction analysis of remote target: $ perfspect instructions --target 192.168.1.1 --user fred --key fred_key
  Analyze specific processes:            $ perfspect instructions --pids 1234,5678
  Register pressure only:                $ perfspect instructions --register-pressure

Flags:
  Analyses:
    --all                  run all analyses (default: true)
    --register-pressure    Register spill/reload pressure in hot symbols (default: false)
  Options:
    --duration             number of seconds to run the collection (default: 10)
    --frequency            number of samples taken per second (default: 500)
    --pids                 comma separated list of PIDs. If not specified, the whole system is sampled (default: [])
    --top                  number of hottest symbols to disassemble and analyze (default: 50)
    --jobs                 number of parallel disassembly workers (default: 4)
    --kernel               include kernel symbols (userspace only by default) (default: false)
    --format               choose output format(s) from: all, html, txt, json (default: [all])
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
