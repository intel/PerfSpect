# Report Tests (TEST_REPORT)

## Test catalog

| Test name | Args exercised | Validates |
|---|---|---|
| `report default` | `report` | Default report generation (all categories, default format) |
| `report cpu isa` | `report --cpu --isa` | Category-specific report with CPU and ISA sections |
| `report input` | `report --input <prev>` | Reprocessing from `report cpu isa` output |
| `report format` | `report --format html,xlsx,json,txt` | All 4 output formats generated |
| `report invalid format` | `report --format invalid` | Exit 1 |
| `report invalid input` | `report --input invalid` | Exit 1 |

## Flags exercised

`--cpu`, `--isa`, `--input`, `--format`

Note: The test script does not exercise all 29+ category flags (`--system-summary`, `--host`, `--pcie`, `--bios`, `--os`, etc.). Only `--cpu` and `--isa` are explicitly tested. Changes to other category flags are covered only by the `report default` test (which runs with `--all` implicitly).

## Test dependencies

- `report input` depends on the output of `report cpu isa` (uses its output directory as `--input`).

## Output verification guidance

- **`report default`**: Verify no `level=ERROR` in `perfspect.log`. Verify output directory contains report files in the default format. If the change affects any report category or the summary/insights table, check the generated report content.
- **`report cpu isa`**: Verify output contains only CPU and ISA sections (not the full report). If `--cpu` or `--isa` flag behavior changes, verify the output reflects only those categories.
- **`report input`**: Verify reprocessing produces output without re-collecting data from the target.
- **`report format`**: Verify output directory contains files in all 4 formats: `.html`, `.xlsx`, `.json`, `.txt`. If a format is added or removed, this test's `t_args` must be updated.
- **Negative tests**: Verify exit code is 1. These tests do not set `t_expect_stderr`, so validation is exit-code-only. If error messages change, the tests still pass but you cannot verify the message — consider whether `t_expect_stderr` should be added.
- **If report table structure changes** (new columns, renamed fields, new tables): The tests do not validate report content beyond exit code. Manually inspect the output HTML/JSON/TXT for the expected changes.
- **If `--format` options change** (e.g., adding `csv`): `report invalid format` passes `--format invalid` and expects exit 1 — this still passes. But `report format` must be updated to include the new format option.
- **If category flags change** (new flag or renamed flag): Only `--cpu` and `--isa` are tested directly. Other flags are covered only by `report default`. If a new category is added, it will be included in the default report but not tested in isolation.
