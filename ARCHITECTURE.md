# Architecture

This document describes the high-level architecture of PerfSpect to help new contributors understand the codebase.

## Overview

PerfSpect is a performance analysis tool for Linux systems. It collects system configuration data, hardware performance metrics, and generates reports. The tool supports both local execution and remote targets via SSH.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         CLI (cmd/root.go)                               │
│   report │ benchmark │ telemetry │ flamegraph │ lock │ metrics | config │
└─────────────────────────────────────────────────────────────────────────┘
          │                                                    │
          │                                                    │
          ▼                                                    ▼
┌───────────────────────────────────────┐    ┌────────────────────────────┐
│       ReportingCommand Framework      │    │    Custom Command Logic    │
│       (internal/workflow/)            │    │                            │
│                                       │    │  metrics: Loader pattern,  │
│  Used by: report, benchmark,          │    │    perf event collection,  │
│    telemetry, flamegraph, lock        │    │    real-time processing    │
│                                       │    │                            │
│  - Target setup and validation        │    │  config: System tuning,    │
│  - Parallel data collection           │    │    set/restore operations  │
│  - Signal handling and cleanup        │    │                            │
│  - Report generation orchestration    │    │                            │
└───────────────────────────────────────┘    └────────────────────────────┘
          │                                               │
          └──────────────────┬────────────────────────────┘
                             │
            ┌────────────────┼────────────────┐
            ▼                ▼                ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  Target Layer   │  │  Script Engine  │  │  Report Engine  │
│ (internal/      │  │ (internal/      │  │ (internal/      │
│  target/)       │  │  script/)       │  │  report/)       │
│                 │  │                 │  │                 │
│ - LocalTarget   │  │ - Embedded      │  │ - txt, json,    │
│ - RemoteTarget  │  │   scripts       │  │   html, xlsx    │
│ - SSH handling  │  │ - Controller    │  │ - Multi-target  │
│                 │  │   orchestration │  │   reports       │
└─────────────────┘  └─────────────────┘  └─────────────────┘
```

## Directory Structure

```
perfspect/
├── main.go              # Entry point
├── cmd/                 # Command implementations
│   ├── root.go          # CLI setup, global flags, app lifecycle
│   ├── report/          # System configuration reports
│   ├── metrics/         # CPU performance counter collection
│   ├── benchmark/       # Performance micro-benchmarks
│   ├── telemetry/       # System telemetry collection
│   ├── flamegraph/      # CPU flamegraph generation
│   ├── lock/            # Lock contention analysis
│   └── config/          # System configuration commands
├── internal/            # Internal packages
│   ├── app/             # Application context and shared types
│   ├── extract/         # Data extraction functions from script outputs
│   ├── workflow/        # Workflow orchestration for reporting commands
│   ├── target/          # Target abstraction (local/remote)
│   ├── script/          # Script execution framework
│   ├── report/          # Report generation (txt, json, html, xlsx)
│   ├── table/           # Table definitions and processing
│   ├── cpus/            # CPU architecture detection
│   ├── progress/        # Progress indicator (multi-spinner)
│   └── util/            # General utilities
└── tools/               # Binaries used by scripts (embedded at build time)
```

## Key Abstractions

### 1. Target Interface (`internal/target/target.go`)

The `Target` interface abstracts away the difference between local and remote systems. All data collection code works identically whether running locally or via SSH.

```go
type Target interface {
    CanConnect() bool
    RunCommand(cmd *exec.Cmd) (stdout, stderr string, exitCode int, err error)
    RunCommandEx(cmd *exec.Cmd, timeout int, newProcessGroup bool, reuseSSH bool) (...)
    PushFile(srcPath, dstPath string) error
    PullFile(srcPath, dstDir string) error
    CreateTempDirectory(rootDir string) (string, error)
    // ... additional methods for privilege elevation, architecture detection, etc.
}
```

**Implementations:**
- `LocalTarget`: Executes commands directly on the local machine
- `RemoteTarget`: Executes commands via SSH on remote machines

### 2. ReportingCommand (`internal/workflow/workflow.go`)

Most commands (`report`, `telemetry`, `flamegraph`, `lock`) follow the same workflow. The `ReportingCommand` struct encapsulates this common flow:

```go
type ReportingCommand struct {
    Cmd          *cobra.Command
    Tables       []table.TableDefinition  // What data to collect
    ScriptParams map[string]string        // Parameters for scripts
    SummaryFunc  SummaryFunc              // Optional: build summary from collected data
    InsightsFunc InsightsFunc             // Optional: generate insights/recommendations
    AdhocFunc    AdhocFunc                // Optional: post-collection actions
}
```

**Workflow (`ReportingCommand.Run()`):**
1. Parse flags and validate inputs
2. Initialize targets (local or from `--target`/`--targets` flags)
3. For each target in parallel:
   - Copy scripts and dependencies to target
   - Run data collection scripts via controller
   - Retrieve outputs
4. Generate reports in requested formats (txt, json, html, xlsx)
5. Run optional adhoc actions

### 3. Script Engine (`internal/script/`)

Collection scripts are defined in `internal/script/scripts.go`. Script dependencies, i.e., tools used by the scripts to collect data, are in `internal/script/resources/` and embedded in the binary using `//go:embed`. The scripts are executed on targets via a controller script that manages concurrent/sequential execution and signal handling.

**Key concepts:**
- `ScriptDefinition`: Defines a script (template, dependencies, required privileges)
- `ScriptOutput`: Captures stdout, stderr, and exit code
- `controller.sh`: Generated script that orchestrates all scripts on a target

**Flow:**
```
1. Scripts defined in code with templates and dependencies
2. Controller script generated from concurrent + sequential scripts
3. Scripts and dependencies copied to target temp directory
4. Controller runs all scripts, captures outputs
5. Outputs parsed and returned to caller
```

### 4. Table Definitions (`internal/table/`)

Tables define what data to collect and how to extract values from script outputs.

```go
type TableDefinition struct {
    Name              string
    ScriptNames       []string           // Scripts that provide data for this table
    Fields            []FieldDefinition  // Fields to extract from script outputs
    Architectures     []string           // Optional: limit to specific architectures
    Vendors           []string           // Optional: limit to specific vendors
    MicroArchitectures []string          // Optional: limit to specific microarchitectures
}
```

**Field value retrieval:**
- `ValuesFunc`: Function that parses script output and returns field values
- Supports regex extraction, JSON parsing, and custom logic

### 5. Loader Pattern (`cmd/metrics/loader.go`)

The metrics command uses a loader pattern to support different metric definition formats across CPU architectures.

```go
type Loader interface {
    Load(config LoaderConfig) (metrics []MetricDefinition, groups []GroupDefinition, err error)
}
```

**Implementations:**
- `LegacyLoader`: Original format (CLX, SKX, BDX, AMD processors)
- `PerfmonLoader`: Intel perfmon JSON format (GNR, EMR, SPR, ICX)
- `ComponentLoader`: ARM processors (Graviton, Axion, Ampere)

The `NewLoader()` factory function returns the appropriate loader based on CPU microarchitecture.

## Data Flow Example: `perfspect report`

```
1. User runs: perfspect report --target 192.168.1.2 --user admin --key ~/.ssh/id_rsa

2. cmd/report/report.go:
   - Creates ReportingCommand with table definitions
   - Calls rc.Run()

3. internal/workflow/workflow.go (ReportingCommand.Run):
   - Creates RemoteTarget from flags
   - Validates connectivity and privileges
   - Calls outputsFromTargets()

4. outputsFromTargets():
   - Determines which tables apply to target (architecture filtering)
   - Builds list of scripts to run
   - Calls collectOnTarget() in goroutine

5. collectOnTarget() → internal/script/script.go:
   - Prepares target (creates temp dir, copies scripts/dependencies)
   - Generates and runs controller.sh
   - Parses outputs, returns ScriptOutputs

6. Back in Run():
   - Calls createReports() with collected data
   - internal/table/ processes tables using internal/extract/ helper functions
   - internal/report/ generates reports in requested formats

7. Output:
   - Reports written to output directory
   - File paths printed to console
```

## Concurrency Model

PerfSpect uses goroutines for parallel operations:

1. **Multi-target collection**: Each target runs in its own goroutine
2. **Script execution**: On each target, concurrent scripts run in parallel (via controller.sh), sequential scripts run one-by-one
3. **Signal handling**: A goroutine listens for SIGINT/SIGTERM and coordinates graceful shutdown across all targets

**Signal handling flow:**
```
SIGINT received
    → Signal handler goroutine activated
    → For each target: send SIGINT to controller.sh PID
    → Wait for controllers to exit (with timeout)
    → If timeout: send SIGKILL
    → Print partial results
```

## Unit Testing

```bash
make test       # Run unit tests
make check      # Run all code quality checks (format, vet, lint)
```

Test files are colocated with source files (e.g., `extract_test.go` alongside `extract.go`).

## Functional Testing
Functional tests are located in an Intel internal GitHub repository. The tests run against various Linux distributions and CPU architectures on internal servers and public cloud systems to validate end-to-end functionality.