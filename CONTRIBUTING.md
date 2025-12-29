# Contributing

Thank you for your interest in contributing to PerfSpect! This document provides guidelines and practical information for contributors.

## Getting Started

### Prerequisites

- Go (see `go.mod` for minimum version)
- Linux environment (PerfSpect targets Linux systems)
- For remote target testing: SSH access to a Linux system

### Building

```bash
builder/build.sh       # Complete build, requires Docker
make                   # Build the x86 binary
make check             # Run all code quality checks, including unit tests
```

### Project Structure

See [ARCHITECTURE.md](./ARCHITECTURE.md) for a detailed overview of the codebase structure and key abstractions.

## Development Workflow

### Before You Start

1. **Read the architecture doc**: Understanding the `Target` interface, `ReportingCommand` framework, and script execution model will save you time.

2. **Find similar code**: Most tasks follow established patterns. Look for similar implementations:
   - Adding a command? See `cmd/report/` or `cmd/telemetry/`
   - Adding metrics for a CPU? See existing loaders in `cmd/metrics/`
   - Adding a table? See table definitions in `cmd/report/report_tables.go`

3. **Run the tool**: Before modifying code, run the commands you'll be changing to understand current behavior.

### Code Style

- Follow standard Go conventions and idioms
- Match the style of surrounding code for consistency
- Use `log/slog` for structured logging (Debug, Info, Warn, Error levels)
- Keep error messages lowercase and actionable

### Testing Your Changes

```bash
# Run all tests
make test

# Run specific test
go test -v ./internal/extract/... -run TestName

# Test locally
./perfspect report

# Test with a remote target
./perfspect report --target hostname --user username --key ~/.ssh/id_rsa
```

## Common Tasks

### Adding a New Command

1. Create `cmd/yourcommand/yourcommand.go`:

```go
package yourcommand

import (
    "perfspect/internal/workflow"
    "perfspect/internal/table"
    "github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
    GroupID: "primary",
    Use:     "yourcommand",
    Short:   "Description of your command",
    RunE:    runYourCommand,
}

func init() {
    // Add command-specific flags here
    workflow.AddTargetFlags(Cmd)
    workflow.AddFormatFlag(Cmd)
}

func runYourCommand(cmd *cobra.Command, args []string) error {
    rc := workflow.ReportingCommand{
        Cmd:    cmd,
        Tables: yourTables,  // Define tables for data collection
    }
    return rc.Run()
}

var yourTables = []table.TableDefinition{
    // Define what data to collect
}
```

2. Register in `cmd/root.go`:

```go
import "perfspect/cmd/yourcommand"

func init() {
    // ...
    rootCmd.AddCommand(yourcommand.Cmd)
}
```

### Adding a New Table

Tables define what data to collect. Add to the relevant command's table definitions:

```go
{
    YourTableName: {
        Name:        YourTableName,
        ScriptNames: []string{script.YourScriptName},
        FieldsFunc: YourTableValues},
}
func YourTableValues() []table.FieldDefinition {
    return []table.FieldDefinition{
        // Define fields here
    }
}
```

### Adding a New Script

1. Define the script in `internal/script/scripts.go`:

```go
var YourScript = ScriptDefinition{
    Name:           YourScriptName,
    ScriptTemplate: `# Your script content
echo "output"
`,
    Superuser:  false,  // true if requires root
    Sequential: false,  // true if must run alone
    Depends:    []string{},  // binary dependencies
}
```

2. Reference in your table's `ScriptNames`

3. If the script needs external binaries, add them to `tools/`. Post build they will be embedded in `internal/script/resources/`

### Adding Metrics for a New CPU

1. Add microarchitecture constant to `internal/cpus/cpus.go`:

```go
const UarchYourCPU = "YourCPU"
```

2. Add detection logic to `GetMicroArchitecture()` in `internal/cpus/cpus.go`

3. Choose appropriate loader or implement new one in `cmd/metrics/loader.go`

4. Add metric/event definitions to the associated loader directory in `cmd/metrics/resources`

5. Update `NewLoader()` switch statement

## Guidelines

### Error Handling

- Always handle errors explicitly
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`
- Use `slog.Error()` for logging errors
- Return errors to callers; let them decide how to handle

### Logging

```go
slog.Debug("detailed info for debugging", slog.String("key", value))
slog.Info("normal operation info")
slog.Warn("potential issue", slog.String("reason", reason))
slog.Error("operation failed", slog.String("error", err.Error()))
```

### Target Compatibility

- Code should work on both local and remote targets
- Test with `LocalTarget` and `RemoteTarget`
- Use the `Target` interface methods, not direct system calls
- Handle architecture differences (x86_64, aarch64)

## Contribution Types

### Bug Fixes

1. Create an issue describing the bug
2. Reference the issue in your PR
3. Include a test case if possible

### Significant Feature Additions

Plans for significant changes must be raised and discussed via GitHub Issues before work begins. This ensures:
- Alignment with project goals
- Consideration of architectural impact
- Validation planning (if needed)

### Extensions for Other CPU Architectures

Changes extending support to other CPU architectures should be contained in architecture-specific directories (e.g., `cmd/metrics/resources/events/x86_64/GenuineIntel`). If changes are required outside these directories, open a GitHub Issue first.

Support for other CPUs requires committed validation by the contributor.

## License

PerfSpect is licensed under the terms in [LICENSE](./LICENSE). By contributing to the project, you agree to the license and copyright terms therein and release your contribution under these terms.

## Sign Your Work

Please use the sign-off line at the end of the patch. Your signature certifies that you wrote the patch or otherwise have the right to pass it on as an open-source patch. The rules are pretty simple: if you can certify the below (from [developercertificate.org](http://developercertificate.org/)):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
660 York Street, Suite 102,
San Francisco, CA 94110 USA

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

Then add a line to every git commit message:

    Signed-off-by: Joe Smith <joe.smith@email.com>

Use your real name (sorry, no pseudonyms or anonymous contributions).

If you set your `user.name` and `user.email` git configs, you can sign your commit automatically with `git commit -s`.
