# Benchmark Tests (TEST_BENCHMARK)

## Test catalog

| Test name | Args exercised | Validates |
|---|---|---|
| `benchmark default` | `benchmark` | Default benchmark (all benchmarks, default format) |
| `benchmark input` | `benchmark --input <prev>` | Reprocessing from `benchmark default` output |
| `benchmark invalid benchmark` | `benchmark --foo` | Exit 1, unknown flag rejected by cobra |
| `benchmark invalid format` | `benchmark --format invalid` | Exit 1 |

## Flags exercised

`--input`, `--format`, unknown flags (cobra validation)

Note: The test script does not exercise individual benchmark selection flags (`--speed`, `--power`, `--temperature`, `--frequency`, `--memory`, `--cache`, `--storage`) or `--storage-dir`, `--no-summary`. Changes to these flags are covered only by `benchmark default` (which runs with `--all` implicitly).

## Test dependencies

- `benchmark input` depends on the output of `benchmark default` (uses its output directory as `--input`).

## Output verification guidance

- **`benchmark default`**: Verify no `level=ERROR` in `perfspect.log`. Verify output directory contains benchmark report files. If the change affects benchmark collection, summary table generation, or reference data comparisons, inspect the output report content.
- **`benchmark input`**: Verify reprocessing produces output without re-running benchmarks.
- **`benchmark invalid benchmark`**: Verifies cobra rejects unknown flags. This test is stable unless the flag name `--foo` is added as a real flag (unlikely).
- **`benchmark invalid format`**: Verify exit code is 1.
- **If benchmark selection flags change**: Only `benchmark default` (all benchmarks) is tested. Individual benchmark flags are not exercised. If a benchmark is added/removed/renamed, verify `benchmark default` still passes and its output reflects the change.
- **If `--format` options change**: Same pattern as other commands — `benchmark invalid format` still passes, but `benchmark default` output should be checked for the new format.
- **If `--storage-dir` validation changes**: No test exercises this flag directly. Manual verification required.
