# GitHub Copilot Instructions for PerfSpect

## Project Overview

PerfSpect is a performance analysis tool for Linux systems written in Go. It provides commands for collecting CPU metrics, system reports, telemetry data, flamegraphs, and lock contention analysis. The tool can target both local and remote systems via SSH.

## Project Structure

- `cmd/` - Command implementations (metrics, report, telemetry, flame, lock, config)
- `internal/` - Internal packages (common, progress, report, script, target, util)
- `tools/` - External tools and utilities
- `builder/` - Docker-based build system
- `scripts/` - Helper scripts
- `main.go` - Application entry point

## Development Guidelines

### Language and Version

- **Language**: Go (see `go.mod` for minimum version - currently Go 1.25)
- Use standard library packages when possible
- Prefer established third-party libraries (see `go.mod` for dependencies)

### Code Style and Formatting

- **Always run `make format`** before committing to format code with `gofmt`
- Follow standard Go conventions and idioms
- Use meaningful variable and function names
- Add comments for exported functions and types
- Use `package-level` comments at the top of each package

### License Headers

**Every source file must include the BSD-3-Clause license header:**

```go
// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause
```

For shell scripts and Makefiles:
```bash
# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
```

### Architecture-Specific Code

- Architecture-specific code belongs in dedicated directories (e.g., `x86_64/`, `aarch64/`)
- CPU architecture support files should be placed in appropriate subdirectories
- Keep platform-specific logic isolated and well-documented

### Building and Testing

#### Build Commands

- `make` or `make perfspect` - Build the x86_64 binary
- `make perfspect-aarch64` - Build the ARM64 binary
- `make dist` - Build distribution packages for both architectures
- `builder/build.sh` - Full build in Docker containers (first build)

#### Testing

- `make test` - Run all unit tests
- Tests are written using the standard `testing` package and `github.com/stretchr/testify`
- Test files follow the `*_test.go` naming convention
- Place test files alongside the code they test
- Write table-driven tests when testing multiple scenarios
- Mock external dependencies appropriately

#### Code Quality Checks

Run these before submitting PRs:
- `make check_format` - Check code formatting
- `make check_vet` - Run go vet
- `make check_static` - Run staticcheck
- `make check_lint` - Run golangci-lint
- `make check_vuln` - Check for vulnerabilities
- `make check_license` - Verify license headers
- `make check` - Run all checks plus tests

### Error Handling

- Return errors explicitly; don't panic except for unrecoverable errors
- Use `github.com/pkg/errors` for wrapping errors with context
- Provide meaningful error messages that help users understand what went wrong
- Log errors appropriately using the `log/slog` package

### Logging

- Use structured logging with `log/slog`
- Log levels: Debug, Info, Warn, Error
- Support both file-based logging and syslog
- Include relevant context in log messages

### Command Line Interface

- Uses `github.com/spf13/cobra` for CLI
- Each command is in its own subdirectory under `cmd/`
- Common flags are defined in `cmd/root.go`
- Use clear, descriptive flag names and help text
- Provide examples in help text

### Target Systems

- Code should support both local and remote target systems
- Remote targets accessed via SSH
- Use the `internal/target` package for target abstractions
- Handle both single targets and multiple targets (via YAML)

### Performance Considerations

- The tool collects performance metrics, so be mindful of overhead
- Use efficient data structures
- Consider memory usage when processing large datasets
- Profile code when performance is critical (use `PERFSPECT_PROFILE=1`)

### Dependencies

- Keep dependencies minimal and well-maintained
- Update dependencies carefully and test thoroughly
- Run `make update-deps` to update Go dependencies
- Document any new dependencies and their purpose

### Documentation

- Update README.md for user-facing changes
- Use comments for complex logic
- Document public APIs
- Add examples for new features
- Keep command help text up to date

### Security

- Run security checks: `make check_sec` (gosec) and `make check_semgrep`
- No hardcoded credentials or secrets
- Validate user input
- Use secure defaults
- Handle sensitive data appropriately (passwords, keys)

## Contributing

- Sign all commits with `git commit -s` (Developer Certificate of Origin)
- Follow guidelines in CONTRIBUTING.md
- Open GitHub Issues for significant changes before implementation
- Provide validation plans for architecture-specific features
- Commit maintainers required for new features

## Common Patterns

### Adding a New Command

1. Create a new directory under `cmd/` (e.g., `cmd/newcommand/`)
2. Implement the command using cobra
3. Register the command in `cmd/root.go`
4. Add tests in `cmd/newcommand/newcommand_test.go`
5. Update documentation and help text

### Working with Targets

```go
import "perfspect/internal/target"

// Use the target abstraction
t, err := target.NewTarget(...)
output, err := t.RunCommand(ctx, "command")
```

### Structured Logging

```go
import "log/slog"

slog.Info("message", "key", value)
slog.Error("error occurred", "error", err, "context", details)
```

## Output Formats

The tool supports multiple output formats:
- HTML (default for reports)
- CSV (for metrics)
- JSON
- Text
- Excel (for some reports)

When adding features, consider supporting multiple formats where appropriate.

## Continuous Integration

- GitHub Actions workflows in `.github/workflows/`
- Build and test run on PRs and main branch pushes
- CodeQL security scanning enabled
- Artifacts uploaded for testing

## Resources

- README.md - User documentation
- CONTRIBUTING.md - Contribution guidelines
- Makefile - Build and development commands
- go.mod - Go dependencies
