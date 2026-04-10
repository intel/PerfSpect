---
name: functional-test
description: >
  Use this skill when running functional tests to validate PerfSpect code changes,
  when the user says "run functional tests", "test my changes", "check for regressions",
  or when verifying a code change did not break existing functionality.
---

> **Skill Loaded:** "Using functional-test skill."

# Functional Test Runner

Run targeted PerfSpect functional tests on a remote target to validate code changes. Identify the specific tests affected by a change, run them, and verify output aligns with the change.

## Test script

`../tools/perfspect/functional_test.sh` (relative to the perfspect repo root). Verify the file exists before proceeding.

## Prerequisites

1. **Built binary.** Run `make` (x86_64) or `make perfspect-aarch64` (ARM64). Binary must be at `./perfspect` (or set `PERFSPECT_DIR`).
2. **Remote target.** User must provide: hostname/IP (`TARGET`), SSH user (`USER_NAME`), private key path (`PRIVATE_KEY_PATH`). Password-less sudo must be configured on the target.
3. **Target dependencies.** `stress-ng` on the target. For flame tests: `java` and `/tmp/primes.java` (copy from `../tools/perfspect/primes.java`).

## Workflow

### Step 1 — Analyze the code change

Run `git diff main...HEAD` (or the appropriate base). Read the diff. Identify:

- **What changed**: flag names, validation logic, error messages, output formats, collection behavior, report generation, table definitions, script content.
- **Behavioral impact**: Does the change alter a CLI flag? A validation rule? An error message string? An output file format? A collection path? A report table?

### Step 2 — Identify affected test categories

Use the code-to-category mapping below to determine which `TEST_*` categories are affected.

| Changed path | Categories |
|---|---|
| `cmd/config/` | `TEST_CONFIG` |
| `cmd/flamegraph/` | `TEST_FLAME` |
| `cmd/lock/` | `TEST_LOCK` |
| `cmd/metrics/` | `TEST_METRICS` |
| `cmd/report/` | `TEST_REPORT` |
| `cmd/benchmark/` | `TEST_BENCHMARK` |
| `cmd/telemetry/` | `TEST_TELEMETRY` |
| `cmd/root.go` | All — trace the specific change to narrow |
| `internal/app/` | All — trace the specific change to narrow |
| `internal/workflow/` | All reporting commands — trace to narrow |
| `internal/extract/` | `TEST_REPORT`, `TEST_TELEMETRY`, `TEST_METRICS` |
| `internal/target/` | All — affects SSH/local execution |
| `internal/script/` | All — affects script execution |
| `internal/report/` | `TEST_REPORT`, `TEST_BENCHMARK`, `TEST_TELEMETRY`, `TEST_METRICS`, `TEST_FLAME` |
| `internal/table/` | `TEST_REPORT`, `TEST_BENCHMARK`, `TEST_TELEMETRY` |
| `internal/cpus/` | All — CPU detection used everywhere |
| `internal/progress/` | All — progress UI used everywhere |
| `internal/util/` | All — trace the specific change to narrow |
| `main.go`, `go.mod`, `go.sum` | All |
| `scripts/`, `tools/` | All — embedded resources |

### Step 3 — Identify specific affected tests

Read the test catalog for each affected category. Load **only** the doc files for affected categories:

| Category | Test catalog |
|---|---|
| `TEST_CONFIG` | [docs/config-tests.md](docs/config-tests.md) |
| `TEST_FLAME` | [docs/flame-tests.md](docs/flame-tests.md) |
| `TEST_LOCK` | [docs/lock-tests.md](docs/lock-tests.md) |
| `TEST_METRICS` | [docs/metrics-tests.md](docs/metrics-tests.md) |
| `TEST_REPORT` | [docs/report-tests.md](docs/report-tests.md) |
| `TEST_BENCHMARK` | [docs/benchmark-tests.md](docs/benchmark-tests.md) |
| `TEST_TELEMETRY` | [docs/telemetry-tests.md](docs/telemetry-tests.md) |

Within the loaded catalog, find every test whose behavior intersects with the change using these criteria:

1. **Flag changes** — Tests that pass the changed flag in `t_args`.
2. **Error message changes** — Tests whose `t_expect_stderr` matches the changed error string.
3. **Output format changes** — Tests that exercise the changed format via `--format` in `t_args`.
4. **Collection behavior changes** — Tests that exercise the changed collection path (scope, granularity, duration, live mode, workload-driven, etc.).
5. **Shared infrastructure changes** — If the change is in shared code (`internal/target/`, `internal/script/`, `internal/workflow/`, `internal/app/`, `cmd/root.go`, `main.go`), trace the change to the specific behavior and find tests that trigger it across categories. Do not blindly run all tests.
6. **stdout/stderr pattern changes** — Tests whose `t_expect_stdout` or `t_expect_stderr` contains text the change modifies.
7. **Custom validation function changes** — Tests with `t_expect_func` that validate output artifacts affected by the change.

Build a list of specific test names (`t_name` values) and their category.

### Step 4 — Predict expected test outcomes

For each identified test, determine whether the code change should:

- **Not alter the test result** (regression check) — The test must still PASS with the same output patterns.
- **Change the test's expected behavior** — The test's expectations (`t_expect_exit`, `t_expect_stdout`, `t_expect_stderr`, `t_expect_func`) no longer match the new code. Flag this to the user: the test script itself must be updated. Explain what the new expected values must be.
- **Make a previously-skipped test runnable** — If the change adds support for something that was previously guarded.

### Step 5 — Run the affected test categories

Disable all categories except those containing affected tests:

```bash
TARGET=<host> USER_NAME=<user> PRIVATE_KEY_PATH=<key> \
  PERFSPECT_DIR=. \
  TEST_CONFIG=false TEST_FLAME=false TEST_LOCK=false TEST_METRICS=false \
  TEST_REPORT=false TEST_BENCHMARK=false TEST_TELEMETRY=false \
  <enable affected categories here>=true \
  ../tools/perfspect/functional_test.sh -q -v
```

Add `NO_ROOT=true` if the remote user does not have password-less sudo.

### Step 6 — Verify output aligns with the change

Do not stop at PASS/FAIL. For each affected test:

1. **Read the test output.** Examine `test/output/<N>-<test_name>/stdout.txt`, `stderr.txt`, and `perfspect.log`.
2. **Verify the change is reflected.** Follow the output verification guidance in the category's doc file. Examples:
   - Error message changed → confirm `stderr.txt` contains the new text.
   - New output field added → confirm it appears in `stdout.txt` or generated report files.
   - Chart/report generation changed → confirm output HTML/JSON/CSV contains expected new content.
   - Bug fix that eliminated ERROR log entries → confirm `perfspect.log` no longer contains `level=ERROR` for the affected path.
   - Collection behavior changed → confirm `stderr.txt` shows expected collection messages and `stdout.txt` shows expected output files.
3. **Check for unintended side effects.** Scan output of non-target tests in the same category for unexpected ERRORs or changed output patterns.

### Step 7 — Report to user

Provide:
- The list of tests identified as affected and why.
- PASS/FAIL status of each.
- For each affected test: what was verified in the output and whether the change is reflected correctly.
- Any tests whose expectations must be updated in the test script (with the specific `t_expect_*` values that must change).
- Any tests that passed but whose output reveals a concern.

## Environment variable reference

| Variable | Default | Purpose |
|---|---|---|
| `PERFSPECT_DIR` | `.` | Path to directory containing the `perfspect` binary |
| `ROOT_OUTPUT_DIR` | `test/output` | Output directory for test artifacts |
| `TARGET` | _(empty)_ | Remote target hostname/IP (empty = local) |
| `USER_NAME` | _(empty)_ | SSH username for remote target |
| `PRIVATE_KEY_PATH` | _(empty)_ | SSH private key path for remote target |
| `NO_ROOT` | `false` | Set to `true` to run without root |
| `TEST_CONFIG` | `true` | Run config tests |
| `TEST_FLAME` | `true` | Run flame tests |
| `TEST_LOCK` | `true` | Run lock tests |
| `TEST_METRICS` | `true` | Run metrics tests |
| `TEST_REPORT` | `true` | Run report tests |
| `TEST_BENCHMARK` | `true` | Run benchmark tests |
| `TEST_TELEMETRY` | `true` | Run telemetry tests |
