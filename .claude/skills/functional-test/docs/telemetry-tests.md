# Telemetry Tests (TEST_TELEMETRY)

## Test catalog

| Test name | Runner | Args exercised | Validates |
|---|---|---|---|
| `telemetry duration` | `run_test` | `telemetry --duration 10` + stress-ng | Basic collection succeeds |
| `telemetry duration input` | `run_test` | `telemetry --input <prev>` | Reprocessing from `telemetry duration` output |
| `telemetry all options` | `run_test` | `telemetry --duration 10 --interval 1 --format txt,html --no-summary --instrmix-frequency 2000000` + stress-ng + `--instrmix-pid` | All flags combined, instruction mix with explicit PID |
| `telemetry cpu` | `run_test` | `telemetry --cpu --duration 10` + stress-ng | CPU category only |
| `telemetry with cpu input` | `run_test` | `telemetry --input <prev>` | Reprocessing CPU-only data from `telemetry cpu` output |
| `telemetry invalid duration` | `run_test` | `telemetry --duration -1` | Exit 1 (duration must be 0 or greater) |
| `telemetry invalid interval` | `run_test` | `telemetry --interval 0` | Exit 1 (interval must be 1 or greater) |
| `telemetry invalid format` | `run_test` | `telemetry --format invalid` | Exit 1 |
| `telemetry invalid input` | `run_test` | `telemetry --input invalid` | Exit 1 |
| `telemetry no output format` | `run_test` | `telemetry --format ""` | Exit 1 (empty format rejected) |
| `telemetry sigint` | `run_sigint_test` | `telemetry` + stress-ng, SIGINT after 15s | Graceful shutdown, log: `Shutting down`, no orphan `perf`/`processwatch` |

## Flags exercised

`--duration`, `--input`, `--interval`, `--format`, `--no-summary`, `--instrmix-frequency`, `--instrmix-pid`, `--cpu`

Note: The test script does not exercise all category flags (`--ipc`, `--cstate`, `--frequency`, `--power`, `--temperature`, `--memory`, `--network`, `--storage`, `--irqrate`, `--kernel`, `--instrmix`). Only `--cpu` is explicitly tested. Other categories are covered by `telemetry duration` (which runs with `--all` implicitly).

## Test dependencies

- `telemetry duration input` depends on `telemetry duration` output.
- `telemetry with cpu input` depends on `telemetry cpu` output.

## Output verification guidance

- **`telemetry duration`**: Verify no `level=ERROR` in `perfspect.log`. Verify output directory contains telemetry report files in the default format (html).
- **`telemetry all options`**: Verify output contains both `.txt` and `.html` files (matching `--format txt,html`). Verify `--no-summary` suppresses the system summary table. Verify instruction mix data is collected (check for instrmix-related content in the output). With `--interval 1`, data points should be at ~1s intervals.
- **`telemetry cpu`**: Verify output contains only CPU telemetry data (not memory, network, etc.). If `--cpu` flag behavior changes, check the output scope.
- **Input reprocessing** (`telemetry duration input`, `telemetry with cpu input`): Verify output is regenerated without re-collecting data.
- **Negative tests**: Verify exit code is 1. These tests do not set `t_expect_stderr`, so validation is exit-code-only.
- **`telemetry no output format`**: Tests that empty string format is rejected. If format validation changes, verify this edge case is still handled.
- **SIGINT** (`telemetry sigint`): Verify `perfspect.log` ends with `Shutting down`. Verify no orphan processes on target.
- **If `--interval` validation changes** (e.g., allowing sub-second intervals): `telemetry invalid interval` passes `--interval 0` and expects exit 1. If 0 becomes valid, update test.
- **If `--duration` validation changes**: `telemetry invalid duration` passes `--duration -1` and expects exit 1. Negative durations should always be invalid.
- **If category flags change** (new telemetry category added): Only `--cpu` is tested in isolation. New categories are covered by `telemetry duration` via implicit `--all`. If a category is removed, `telemetry duration` may still pass but output changes — verify.
- **If `--instrmix-frequency` validation changes**: `telemetry all options` uses `--instrmix-frequency 2000000`. If the minimum changes, verify this value is still valid. The code enforces minimum 100000.
- **If `--format` options change**: `telemetry invalid format` still passes. But `telemetry all options` uses `--format txt,html` — update if formats are renamed.
