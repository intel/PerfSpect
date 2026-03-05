# perfspect config

```text
Sets system configuration items on target platform(s).

USE CAUTION! Target may become unstable. It is up to the user to ensure that the requested configuration is valid for the target. There is not an automated way to revert the configuration changes. If all else fails, reboot the target.

Usage: perfspect config [flags]

Examples:
  Set core count on local host:            $ perfspect config --cores 32
  Set multiple config items on local host: $ perfspect config --core-max 3.0 --uncore-max 2.1 --tdp 120
  Record config to file before changes:    $ perfspect config --c6 disable --epb 0 --record
  Restore config from file:                $ perfspect config restore gnr_config.txt
  Set core count on remote target:         $ perfspect config --cores 32 --target 192.168.1.1 --user fred --key fred_key
  View current config on remote target:    $ perfspect config --target 192.168.1.1 --user fred --key fred_key
  Set governor on remote targets:          $ perfspect config --gov performance --targets targets.yaml

Flags:
  General Options:
    --cores                number of physical cores per processor
    --llc                  LLC size in MB
    --tdp                  maximum power per processor in Watts
    --core-max             SSE frequency in GHz
    --core-max-buckets     SSE frequencies for all core buckets in GHz (e.g., 1-40/3.5, 41-60/3.4, 61-86/3.2)
    --epb                  energy perf bias from best performance (0) to most power savings (15)
    --epp                  energy perf profile from best performance (0) to most power savings (255)
    --gov                  CPU scaling governor (performance, powersave)
    --elc                  efficiency latency control (latency, power) [SRF+]
  Uncore Frequency Options:
    --uncore-max           maximum uncore frequency in GHz [EMR-]
    --uncore-min           minimum uncore frequency in GHz [EMR-]
    --uncore-max-compute   maximum uncore compute die frequency in GHz [SRF+]
    --uncore-min-compute   minimum uncore compute die frequency in GHz [SRF+]
    --uncore-max-io        maximum uncore IO die frequency in GHz [SRF+]
    --uncore-min-io        minimum uncore IO die frequency in GHz [SRF+]
  Prefetcher Options:
    --pref-l2hw            L2 HW [all] (enable, disable)
    --pref-l2adj           L2 Adj [all] (enable, disable)
    --pref-dcuhw           DCU HW [all] (enable, disable)
    --pref-dcuip           DCU IP [all] (enable, disable)
    --pref-dcunp           DCU NP [all] (enable, disable)
    --pref-amp             AMP [SPR,EMR,GNR,DMR] (enable, disable)
    --pref-llcpp           LLCPP [GNR,DMR] (enable, disable)
    --pref-aop             AOP [GNR] (enable, disable)
    --pref-l2p             L2P [DMR] (enable, disable)
    --pref-homeless        Homeless [SPR,EMR,GNR] (enable, disable)
    --pref-llc             LLC [SPR,EMR,GNR] (enable, disable)
    --pref-llcstream       LLC Stream [SRF,CWF] (enable, disable)
  C-State Options:
    --c6                   C6 (enable, disable)
    --c1-demotion          C1 Demotion (enable, disable)
  Other Options:
    --no-summary           do not print configuration summary
    --record               record the current configuration to a file to be restored later
  Remote Target Options:
    --target               host name or IP address of remote target
    --port                 port for SSH to remote target
    --user                 user name for SSH to remote target
    --key                  private key file for SSH to remote target
    --targets              file with remote target(s) connection details. See targets.yaml for format.

Subcommands:
  restore: Restore system configuration from file

Global Flags:
  --debug                enable debug logging and retain temporary directories (default: false)
  --log-stdout           write logs to stdout (default: false)
  --noupdate             skip application update check (default: false)
  --output               override the output directory
  --syslog               write logs to syslog instead of a file (default: false)
  --tempdir              override the temporary target directory, must exist and allow execution
```
