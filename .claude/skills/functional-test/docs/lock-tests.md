# Lock Tests (TEST_LOCK)

All lock tests require root (`t_requires_root=true`).

## Test catalog

| Test name | Args exercised | Validates |
|---|---|---|
| `lock all options` | `lock --duration 10 --frequency 22 --package --no-summary --format html` + stress-ng | All lock flags combined, successful collection |
| `lock invalid duration` | `lock --duration 0` | Exit 1 (duration must be > 0) |
| `lock invalid frequency` | `lock --frequency -1` | Exit 1 (frequency must be > 0) |
| `lock invalid format` | `lock --format invalid` | Exit 1 (format must be from: all, html, txt) |

## Flags exercised

`--duration`, `--frequency`, `--package`, `--no-summary`, `--format`

## Output verification guidance

- **`lock all options`**: Verify no `level=ERROR` in `perfspect.log`. Verify output directory contains HTML report file. With `--package`, verify raw data package was downloaded.
- **Negative tests**: Verify exit code is 1. These tests do not set `t_expect_stderr` patterns, so validation is exit-code-only. If a code change adds specific error messages for lock validation, the tests may need `t_expect_stderr` added.
- **If `--format` options change**: The `lock invalid format` test passes `--format invalid` and expects exit 1. If new format options are added, this test still passes (since `invalid` remains invalid). But if format validation error messages change, verify they still align.
- **If duration/frequency validation changes** (e.g., allowing 0 duration for indefinite collection): `lock invalid duration` passes `--duration 0` and expects exit 1. If 0 becomes valid, this test must be updated — flag to user.
