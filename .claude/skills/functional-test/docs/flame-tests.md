# Flame Tests (TEST_FLAME)

All flame tests require root (`t_requires_root=true`).

## Test catalog

| Test name | Runner | Args exercised | Validates |
|---|---|---|---|
| `flame duration java` | `run_test` | `flame --duration 10 --format all` + java workload | JSON output contains `primes.java` in `Flamegraph[0]["Java Stacks"]` |
| `flame duration native` | `run_test` | `flame --duration 10 --format all` + stress-ng | JSON output contains `stress-ng` in `Flamegraph[0]["Native Stacks"]` |
| `flame dual native stacks` | `run_test` | `flame --duration 10 --format all --dual-native-stacks` + stress-ng | Dual stack mode, JSON validates `stress-ng` in Native Stacks |
| `flame all options` | `run_test` | `flame --duration 10 --frequency 10 --format html,json --no-summary --max-depth 20 --perf-event instructions` + java + `--pids` | All flags combined, JSON validates `primes.java` in Java Stacks |
| `flame with input` | `run_test` | `flame --input <prev_output>` | Reprocessing from raw data produced by `flame all options` |
| `flame invalid format` | `run_test` | `flame --format html,invalid` | Exit 1, stderr: `format options are: all, html, txt, json` |
| `flame invalid duration` | `run_test` | `flame --duration -1` | Exit 1, stderr: `duration must be 0 or greater` |
| `flame invalid frequency` | `run_test` | `flame --frequency 0` | Exit 1, stderr: `frequency must be 1 or greater` |
| `flame sigint native` | `run_sigint_test` | `flame --format all --no-summary` + stress-ng, SIGINT after 15s | Graceful shutdown: log ends with `Shutting down`, `perf` and `processwatch` no longer running, JSON validates `stress-ng` |
| `flame sigint java` | `run_sigint_test` | `flame --format all --no-summary` + java, SIGINT after 15s | Graceful shutdown: log ends with `Shutting down`, JSON validates `primes.java` |

## Flags exercised

`--duration`, `--format`, `--frequency`, `--no-summary`, `--max-depth`, `--perf-event`, `--dual-native-stacks`, `--pids`, `--input`

## Custom validation functions

Tests `flame duration java`, `flame all options`, `flame sigint java` use:
```bash
jq -r ".["Flamegraph"][0]["Java Stacks"]" "$1"/*_flame.json | grep -q "primes.java"
```

Tests `flame duration native`, `flame dual native stacks`, `flame sigint native` use:
```bash
jq -r ".["Flamegraph"][0]["Native Stacks"]" "$1"/*_flame.json | grep -q "stress-ng"
```

## Output verification guidance

- **Collection tests** (`flame duration java`, `flame duration native`, `flame dual native stacks`, `flame all options`): Verify `*_flame.json` exists in the output directory. Parse it with `jq` to confirm the expected stack type contains the workload name.
- **Input reprocessing** (`flame with input`): Verify it regenerates output from previously-collected raw data without re-collecting.
- **Negative tests**: Verify `stderr.txt` contains the exact error message string. Verify exit code is 1.
- **SIGINT tests**: Verify `perfspect.log` last line contains `Shutting down`. Verify no `perf` or `processwatch` processes remain on target. Verify the `t_expect_func` JSON validation still passes (data was collected before shutdown).
- **If `--format` options change**: The `flame invalid format` test expects the error `format options are: all, html, txt, json`. Update the expected string if format options are added or removed.
- **If JSON output structure changes**: The custom validation functions parse `*_flame.json` with specific jq paths. If the JSON schema changes, these tests will fail — flag to user that both code and test `t_expect_func` must be updated.
