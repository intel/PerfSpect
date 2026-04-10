# Metrics Tests (TEST_METRICS)

## Test catalog

| Test name | Runner | Args exercised | Validates | Constraints |
|---|---|---|---|---|
| `metrics scope cgroup count` | `run_test` | `metrics --duration 10 --scope cgroup --count 3` + docker stress-ng | stdout: `Metric files`, `metrics.csv`, `summary.csv`; stderr: `collection complete` | Local only (`t_requires_local=true`) |
| `metrics sigint` | `run_sigint_test` | `metrics` + stress-ng, SIGINT after 15s | Graceful shutdown, log: `Shutting down`, no orphan `perf`/`processwatch` | x86_64 only (`t_requires_arch="x86_64"`) |
| `metrics duration` | `run_test` | `metrics --duration 10` + stress-ng | stdout: `Metric files`; stderr: `collection complete` | |
| `metrics granularity cpu` | `run_test` | `metrics --duration 10 --granularity cpu` + stress-ng | stdout: `Metric files`; stderr: `collection complete` | |
| `metrics granularity socket` | `run_test` | `metrics --duration 10 --granularity socket` + stress-ng | stdout: `Metric files`; stderr: `collection complete` | |
| `metrics scope process` | `run_test` | `metrics --duration 10 --scope process` + stress-ng | stdout: `Metric files`; stderr: `collection complete` | |
| `metrics scope process pids` | `run_test` | `metrics --duration 10 --scope process` + stress-ng + `--pids` | Pass with explicit PID targeting | |
| `metrics txnrate` | `run_test` | `metrics --duration 10 --txnrate 1000` + stress-ng | stdout: `Metric files`; stderr: `collection complete` | |
| `metrics all raw debug` | `run_test` | `metrics --format csv,json,txt,wide --duration 10 --raw --debug` + stress-ng | stdout: `Metric files`; stderr: `collection complete`; all 4 formats generated; raw events written | |
| `metrics input txnrate` | `run_test` | `metrics --input <prev> --txnrate 33` | stdout: `Metric files`; reprocesses raw data with new txnrate | Depends on `metrics all raw debug` output |
| `metrics trim source` | `run_test` | `metrics --duration 30` + stress-ng | stdout: `Metric files`; stderr: `collection complete`; 30s collection for trim input | |
| `metrics trim` | `run_test` | `metrics trim --input <prev> --start-offset 10 --end-offset 5` | stdout: `Trimmed metrics successfully created:` | `t_skip_target_args=true` (local-only subcommand) |
| `metrics live` | `run_test` | `metrics --live --duration 10` + stress-ng | stdout: `TS,SKT` (CSV header); stderr: `collecting metrics` | |
| `metrics workload` | `run_test` | `metrics` + `-- stress-ng --cpu 0 --cpu-load 60 --timeout 10` | stdout: `Metric files`; stderr: `collection complete`; workload-driven duration | |
| `metrics cpu range` | `run_test` | `metrics --cpus 0-1 --duration 10` + stress-ng | stdout: `Metric files`; stderr: `collection complete` | |
| `metrics cpu range not zero` | `run_test` | `metrics --cpus 4-7 --duration 10` + stress-ng | stdout: `Metric files`; stderr: `collection complete` | |
| `metrics list` | `run_test` | `metrics --list` | stdout: `Metrics available` | |
| `metrics metrics filter` | `run_test` | `metrics --duration 10 --metrics IPC` + stress-ng | stdout: `Metric files`; stderr: `collection complete` | |

## Flags exercised

`--duration`, `--scope`, `--count`, `--granularity`, `--pids`, `--txnrate`, `--format`, `--raw`, `--debug`, `--input`, `--live`, `--cpus`, `--list`, `--metrics`, `--noroot`

Note: `--noroot` is appended automatically when `NO_ROOT=true`.

## Test dependencies

- `metrics input txnrate` depends on the output of `metrics all raw debug` (uses its output directory as `--input`).
- `metrics trim` depends on the output of `metrics trim source` (uses its output directory as `--input`).
- These tests must run in order within the category. The test script handles this via sequential `test_num` numbering.

## Output verification guidance

- **Collection tests** (`metrics duration`, `metrics granularity *`, `metrics scope *`, etc.): Verify `stdout.txt` contains `Metric files`. Verify `stderr.txt` contains `collection complete`. Check output directory for expected format files (`.csv`, `.json`, `.txt` depending on `--format`).
- **Granularity tests**: If granularity logic changes, verify the output CSV/JSON contains data at the expected level (per-CPU rows for `cpu`, per-socket for `socket`, system-wide for `system`).
- **Scope tests**: If scope logic changes, verify process-level or cgroup-level data appears in output. For `metrics scope process pids`, verify the specific PID's data is present.
- **Live mode** (`metrics live`): Verify `stdout.txt` starts with `TS,SKT` CSV header. This test validates that live output goes to stdout rather than files.
- **Workload-driven** (`metrics workload`): Verify collection duration matches the workload's `--timeout 10` rather than an explicit `--duration`.
- **CPU range** (`metrics cpu range`, `metrics cpu range not zero`): If CPU range parsing changes, verify output contains data only for the specified CPUs.
- **Trim** (`metrics trim`): Verify `stdout.txt` contains `Trimmed metrics successfully created:`. Verify trimmed output has fewer data points than the source.
- **Input reprocessing** (`metrics input txnrate`): Verify reprocessed output reflects the new txnrate value (33 vs original 1000).
- **SIGINT** (`metrics sigint`): Verify `perfspect.log` ends with `Shutting down`. Verify no orphan `perf`/`processwatch` on target.
- **If `--format` options change**: Check `metrics all raw debug` which exercises `csv,json,txt,wide`. If a format is renamed or removed, this test must be updated.
- **If `--scope` options change**: Check `metrics scope cgroup count` and `metrics scope process` tests. Update expected behavior accordingly.
- **If `--list` output format changes**: Check `metrics list` which expects `Metrics available` in stdout. Update pattern if the header text changes.
