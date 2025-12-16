# copilot-instructions.md

## Project Overview

PerfSpect is a performance analysis tool for Linux systems written in Go. It provides several commands:
- `metrics`: Collects CPU performance metrics using hardware performance counters
- `report`: Generates system configuration and health reports from collected data
- `benchmark`: Runs performance micro-benchmarks to evaluate system health
- `telemetry`: Gathers system telemetry data
- `flamegraph`: Creates CPU flamegraphs
- `lock`: Analyzes lock contention
- `config`: Modifies system configuration for performance tuning

The tool can target both local and remote systems via SSH.

## Project Structure

- `main.go` - Application entry point
- `cmd/` - Command implementations (metrics, report, telemetry, flamegraph, lock, config)
- `internal/` - Internal packages (common, cpus, progress, report, script, table, target, util)
- `internal/common/` - Shared types, functions, and workflows for commands
- `internal/target/` - Abstraction for local and remote target systems
- `internal/table/` - Table definitions and helpers for reports
- `internal/script/` - Script execution framework and script dependencies as embedded resources
- `internal/report/` - Report generation and formatting
- `internal/progress/` - Progress tracking utilities
- `internal/cpus/` - CPU architecture detection and metadata
- `internal/util/` - General utility functions
- `tools/` - External tools and utilities, i.e., dependencies used for collecting data on target systems
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

#### Build Commands

- `make` - Build the x86_64 binary

#### Testing

- `make test` - Run all unit tests
- Follow standard Go testing practices

#### Code Quality Checks

Run `make check` to perform all code quality checks

### Logging

- Use structured logging with `log/slog`
- Log levels: Debug, Info, Warn, Error

### Command Line Interface

- Uses `github.com/spf13/cobra` for CLI
- Each command is in its own subdirectory under `cmd/`
- Some commands have subcommands (e.g., `metrics trim`, `config restore`)
- Common flags are defined in `cmd/root.go`

### Target Systems

- Code should support both local and remote target systems
- Remote targets accessed via SSH
- Use the `internal/target` package for target abstractions
- Handle both single targets and multiple targets (via YAML)

### Documentation

- Update README.md for user-facing changes
- Use comments for complex logic
- Document public APIs
- Add examples for new features
- Keep command help text up to date
