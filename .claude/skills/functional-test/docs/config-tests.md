# Config Tests (TEST_CONFIG)

All config tests require root (`t_requires_root=true`).

## Test catalog

| Test name | Args exercised | Validates |
|---|---|---|
| `config help` | `config --help` | Help text prints `Usage:` |
| `config default` | `config` | No-op prints `No changes requested` and `Configuration` |
| `config gov epb epp` | `config --gov performance --epb 0 --epp 0` | Applies governor/epb/epp, stderr confirms each setting |
| `config disable l2hw prefetcher` | `config --pref-l2hw disable` | Prefetcher disable, stderr confirms |
| `config enable l2hw prefetcher no-summary` | `config --pref-l2hw enable --no-summary` | Prefetcher enable with `--no-summary` suppresses stdout table |
| `config invalid core count` | `config --cores 0` | Exit 1, stderr: `invalid flag value, --cores 0, valid values are` |
| `config invalid llc size` | `config --llc 0` | Exit 1, stderr: `invalid flag value, --llc 0, valid values are` |
| `config invalid core frequency` | `config --core-max .05` | Exit 1, stderr: `invalid flag value, --core-max 0.05, valid values are` |
| `config invalid tdp` | `config --tdp 0` | Exit 1, stderr: `invalid flag value, --tdp 0, valid values are` |
| `config invalid epb` | `config --epb 16` | Exit 1, stderr: `invalid flag value, --epb 16, valid values are` |
| `config invalid epp` | `config --epp 256` | Exit 1, stderr: `invalid flag value, --epp 256, valid values are` |
| `config invalid governor` | `config --gov invalid` | Exit 1, stderr: `invalid flag value, --gov invalid, valid values are` |
| `config invalid elc` | `config --elc invalid` | Exit 1, stderr: `invalid flag value, --elc invalid, valid values are` |
| `config invalid uncore max frequency` | `config --uncore-max .05` | Exit 1, stderr: `invalid flag value, --uncore-max 0.05, valid values are` |
| `config invalid uncore min frequency` | `config --uncore-min .05` | Exit 1, stderr: `invalid flag value, --uncore-min 0.05, valid values are` |
| `config invalid uncore max compute frequency` | `config --uncore-max-compute .05` | Exit 1, stderr: `invalid flag value, --uncore-max-compute 0.05, valid values are` |
| `config invalid uncore min compute frequency` | `config --uncore-min-compute .05` | Exit 1, stderr: `invalid flag value, --uncore-min-compute 0.05, valid values are` |
| `config invalid uncore max io frequency` | `config --uncore-max-io .05` | Exit 1, stderr: `invalid flag value, --uncore-max-io 0.05, valid values are` |
| `config invalid uncore min io frequency` | `config --uncore-min-io .05` | Exit 1, stderr: `invalid flag value, --uncore-min-io 0.05, valid values are` |
| `config invalid l2hw prefetcher` | `config --pref-l2hw invalid` | Exit 1, stderr: `invalid flag value, --pref-l2hw invalid, valid values are` |
| `config invalid c6` | `config --c6 invalid` | Exit 1, stderr: `invalid flag value, --c6 invalid, valid values are` |
| `config invalid c1 demotion` | `config --c1-demotion invalid` | Exit 1, stderr: `invalid flag value, --c1-demotion invalid, valid values are` |

## Flags exercised

`--gov`, `--epb`, `--epp`, `--pref-l2hw`, `--no-summary`, `--cores`, `--llc`, `--core-max`, `--tdp`, `--elc`, `--uncore-max`, `--uncore-min`, `--uncore-max-compute`, `--uncore-min-compute`, `--uncore-max-io`, `--uncore-min-io`, `--c6`, `--c1-demotion`, `--help`

## Output verification guidance

- **Positive tests** (`config gov epb epp`, `config disable l2hw prefetcher`, etc.): Verify `stderr.txt` contains the `set <flag> to <value>` confirmation messages. Verify `stdout.txt` contains the `Configuration` table when `--no-summary` is not set, and does not contain it when `--no-summary` is set.
- **Negative tests** (all `config invalid *`): Verify `stderr.txt` contains the exact `Error: invalid flag value, --<flag> <value>, valid values are` message. Verify exit code is 1.
- **If a validation range changes** (e.g., `--epb` now accepts 0-20 instead of 0-15): The `config invalid epb` test passes `--epb 16` and expects exit 1. If 16 is now valid, this test must be updated — flag to user with the new boundary value.
