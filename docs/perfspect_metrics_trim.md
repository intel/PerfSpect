# perfspect metrics trim

```text
Filter existing metrics to a time range

Usage:
  perfspect metrics trim [flags]

Examples:
  Skip first 30 seconds:                        $ perfspect metrics trim --input perfspect_2025-11-28_09-21-56 --start-offset 30
  Skip first 10 seconds and last 5 seconds:     $ perfspect metrics trim --input perfspect_2025-11-28_09-21-56 --start-offset 10 --end-offset 5
  Use absolute timestamps and specific CSV:     $ perfspect metrics trim --input perfspect_2025-11-28_09-21-56/myhost_metrics.csv --start-time 1764174327 --end-time 1764174351

Flags:
      --end-offset int     seconds to exclude from the end of the data
      --end-time int       absolute end timestamp (seconds since epoch)
  -h, --help               help for trim
      --input string       path to the directory or specific metrics CSV file to trim (required)
      --start-offset int   seconds to skip from the beginning of the data
      --start-time int     absolute start timestamp (seconds since epoch)

Global Flags:
      --debug            enable debug logging and retain temporary directories
      --log-stdout       write logs to stdout
      --noupdate         skip application update check
      --output string    override the output directory
      --syslog           write logs to syslog instead of a file
      --tempdir string   override the temporary target directory, must exist and allow execution

```
