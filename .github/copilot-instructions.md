# copilot-instructions.md

## Project Overview

PerfSpect is a performance analysis tool for Linux systems written in Go. It provides commands for collecting CPU performance metrics, generating system configuration reports, running micro-benchmarks, gathering telemetry, creating flamegraphs, analyzing lock contention, and modifying system configuration. It can target both local and remote systems via SSH.

See `ARCHITECTURE.md` for detailed architecture, data flow diagrams, and concurrency model.

## Project Structure

- `main.go` - Application entry point
- `cmd/` - Command implementations (metrics, report, benchmark, telemetry, flamegraph, lock, config)
  - Subcommands: `metrics trim`, `config restore`
  - `update` and `extract` commands are defined in `cmd/root.go`
- `internal/`
  - `app/` - Application context and shared types
  - `workflow/` - Shared workflow orchestration for reporting commands
  - `extract/` - Data extraction functions from script outputs
  - `target/` - Target abstraction (local and remote via SSH)
  - `script/` - Script execution framework; scripts and tool dependencies are embedded via `//go:embed`
  - `report/` - Report generation and formatting (txt, json, html, xlsx)
  - `table/` - Table definitions and helpers for reports
  - `cpus/` - CPU architecture detection and metadata
  - `progress/` - Progress indicator (multi-spinner)
  - `util/` - General utility functions
- `tools/` - Binaries used by scripts (embedded at build time)
- `builder/` - Docker-based build system
- `scripts/` - Helper scripts

## Development Guidelines

### Language and Version

- **Language**: Go (see `go.mod` for minimum version)
- Use standard library packages when possible
- Prefer established third-party libraries (see `go.mod` for dependencies)

### Code Style and Formatting

- Follow standard Go conventions and idioms
- Follow style of surrounding code for consistency

### Building and Testing

- `make` - Build the x86_64 binary
- `make perfspect-aarch64` - Build the ARM64 binary
- `make test` - Run all unit tests
- `make check` - Run all code quality checks
  - Individual checks: `make check_format`, `make check_vet`, `make check_static`, `make check_lint`, `make check_vuln`, `make check_sec`, `make check_semgrep`

### Logging

- Use structured logging with `log/slog`
- Log levels: Debug, Info, Warn, Error

### Command Line Interface

- Uses `github.com/spf13/cobra` for CLI
- Each command is in its own subdirectory under `cmd/`
- Common flags are defined in `cmd/root.go`

### Target Systems

- Code should support both local and remote target systems
- Remote targets accessed via SSH
- Use the `internal/target` package for target abstractions
- Handle both single targets and multiple targets (via YAML)

### Documentation

- Use comments for complex logic
- Add examples for new features
- Update cobra command help strings in source, then run `make docs` to regenerate `docs/perfspect_*.md` (requires a built binary)
- Update README.md for user-facing changes
- Update ARCHITECTURE.md for architectural changes

## Agent Instructions

- Ask clarifying questions if requirements are ambiguous
- Unless it is a very simple task, start with an explanation and plan instead of jumping directly to coding a solution
