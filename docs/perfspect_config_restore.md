# perfspect config restore

```text
Restores system configuration from a file that was previously recorded using the --record flag.

The restore command will parse the configuration file, validate the settings against the target system,
and apply the configuration changes. By default, you will be prompted to confirm before applying changes.

Usage: perfspect config restore <file> [flags]

Examples:
  Restore config from file on local host:     $ perfspect config restore gnr_config.txt
  Restore config on remote target:            $ perfspect config restore gnr_config.txt --target 192.168.1.1 --user fred --key fred_key
  Restore config without confirmation:        $ perfspect config restore gnr_config.txt --yes

Arguments:
  file: path to the configuration file to restore

Flags:
  General Options:
    --yes                  skip confirmation prompt
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
