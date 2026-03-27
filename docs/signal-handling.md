# Signal Handling

This document describes how PerfSpect handles Linux signals across all commands and scenarios. It is intended for developers working on the codebase.

## Overview

PerfSpect registers signal handlers for `SIGINT` and `SIGTERM` at multiple levels. The handling strategy differs depending on which command is running and whether the target is local or remote. There are three independent signal-handling layers:

1. **Root-level handler** -- active during the pre-command update check (`cmd/root.go`)
2. **Reporting-command handler** -- used by `report`, `benchmark`, `telemetry`, `flamegraph`, and `lock` (`internal/workflow/signals.go`)
3. **Metrics-command handler** -- used by `metrics` (`cmd/metrics/metrics.go`)

Each layer replaces the previous one via `signal.Notify`, so only one Go-level handler is active at a time for a given signal.

## Background: Process Groups and Terminal Signals

A key design decision throughout PerfSpect is the use of **new process groups** (`Setpgid: true`) for child processes. This is critical for understanding signal propagation:

- When the user presses Ctrl-C in a terminal, the kernel sends `SIGINT` to **every process in the foreground process group**.
- PerfSpect runs the controller script (and `perf stat` for metrics) in a **separate process group** so that the terminal's `SIGINT` is **not** automatically delivered to them.
- This gives PerfSpect's signal handler the opportunity to orchestrate a graceful shutdown rather than having children killed out from under it.

For remote targets, the "child" from PerfSpect's perspective is the local SSH process. The controller script runs on the remote host in its own process group as well.

## 1. Root-Level Handler (`cmd/root.go`)

### When Active

During the `PersistentPreRunE` phase, while PerfSpect checks for available updates (Intel network only). This handler is short-lived and is deregistered via `defer signal.Stop(sigChannel)` before the subcommand begins.

### What It Does

```
SIGINT or SIGTERM received
  -> Log the signal
  -> Call terminateApplication() to clean up (close log file, remove temp directory)
  -> os.Exit(1)
```

### Scenarios

This handler is identical for all scenarios (local/remote, Ctrl-C/external signal) because it runs before any target connection or data collection begins. It simply ensures a clean exit if the user interrupts during the update check.

**Source:** `cmd/root.go:226-239`

## 2. Reporting-Command Handler (`internal/workflow/`)

Used by: `report`, `benchmark`, `telemetry`, `flamegraph`, `lock`

### Architecture

```text
                     perfspect (Go)
                     ┌─────────────────────────────────────────────┐
                     │  signal handler goroutine                   │
                     │  (configureSignalHandler)                   │
                     │                                             │
                     │  Listens for SIGINT / SIGTERM               │
                     └──────────┬──────────────────────────────────┘
                                │
            ┌───────────────────┼───────────────────┐
            ▼                   ▼                   ▼
     Target A              Target B              Target N
     ┌──────────┐          ┌──────────┐          ┌──────────┐
     │controller│          │controller│          │controller│
     │  .sh     │          │  .sh     │          │  .sh     │
     │(own PGID)│          │(own PGID)│          │(own PGID)│
     └────┬─────┘          └────┬─────┘          └────┬─────┘
          │                     │                     │
     ┌────┴────┐           ┌────┴────┐           ┌────┴────┐
     │ scripts │           │ scripts │           │ scripts │
     │(setsid) │           │(setsid) │           │(setsid) │
     └─────────┘           └─────────┘           └─────────┘
```

Each target's controller script runs in its own process group. Each individual script within the controller also runs in its own session (`setsid`).

### Signal Flow

The handler is installed in `configureSignalHandler()` (`internal/workflow/signals.go:64-174`). When a signal arrives:

1. **Read each target's controller PID** from `controller.pid` on the target (via `cat` over SSH for remote, locally for local).
2. **Send SIGINT to each controller** using `kill -SIGINT <pid>` on the target.
3. **Spawn per-target goroutines** that poll (`ps -p <pid>`) until the controller exits, with a **20-second timeout**.
4. If the timeout expires, **send SIGKILL** to the controller.
5. After all controllers have exited, **sleep 500ms** to allow local SSH processes to finish cleanly.
6. **Send SIGINT to perfspect's remaining children** (e.g., SSH processes that haven't exited yet) via `util.SignalChildren()`.

### Controller Script Signal Handling

The controller script (`internal/script/script.go:258-402`) has its own `trap handle_sigint SIGINT` handler:

1. For each running concurrent script, call `kill_script()`:
   - Send `SIGTERM` to the script's **process group** (`kill -SIGTERM -$pid`). Bash background jobs ignore SIGINT but respond to SIGTERM.
   - Poll for up to **60 seconds** for graceful exit.
   - If still alive, send `SIGKILL` to the process group.
   - Set exit code to 143 (SIGTERM) for killed scripts.
2. Kill the current sequential script if one is running.
3. Call `print_summary` to emit whatever output was collected (partial results).
4. Remove `controller.pid` and exit 0.

### Individual Script Signal Handling

Some long-running scripts have their own signal traps:

| Script | Trap | Behavior |
|--------|------|----------|
| Flamegraph (`flamegraph`) | `trap on_signal INT TERM EXIT` | Stops perf, collapses stacks, prints partial results, restores kernel settings |
| Lock contention (`lock`) | `trap on_signal INT TERM EXIT` | Similar to flamegraph -- stops perf, processes data, prints results |
| Instruction telemetry | `trap finalize INT TERM EXIT` | Kills processwatch, cleans up |
| Kernel telemetry | `trap cleanup SIGINT SIGTERM` | Exits cleanly |
| Syscall telemetry | `trap cleanup INT TERM` | Kills perf, exits cleanly |

### Scenario: Local Target, Ctrl-C in Shell

```text
User presses Ctrl-C
  │
  ├─→ Kernel sends SIGINT to foreground process group
  │     ├─→ perfspect receives SIGINT
  │     └─→ controller.sh does NOT receive it (different PGID)
  │
  └─→ perfspect's signal handler activates
        ├─→ Reads controller.pid from local filesystem
        ├─→ Runs: kill -SIGINT <controller_pid>
        ├─→ Controller's handle_sigint trap fires
        │     ├─→ Sends SIGTERM to each script's process group
        │     ├─→ Waits for scripts to exit (up to 60s each)
        │     ├─→ Prints collected output
        │     └─→ Exits 0
        ├─→ Signal handler detects controller has exited
        ├─→ Sends SIGINT to any remaining perfspect children
        └─→ Perfspect processes partial results and exits
```

### Scenario: Local Target, SIGINT from Another Process

The flow is identical to Ctrl-C except:
- Only perfspect receives the signal (the kernel does not broadcast to the foreground process group since the signal comes from `kill`, not the terminal).
- The controller and its scripts are unaffected until perfspect's handler explicitly signals them.
- This is the scenario the separate-process-group design specifically addresses -- it ensures the same graceful shutdown path regardless of signal source.

### Scenario: Remote Target, Ctrl-C in Shell

```text
User presses Ctrl-C
  │
  ├─→ Kernel sends SIGINT to foreground process group
  │     ├─→ perfspect receives SIGINT
  │     └─→ local SSH process does NOT receive it (different PGID)
  │
  └─→ perfspect's signal handler activates
        ├─→ Reads controller.pid from remote target (via SSH, reusing connection)
        ├─→ Runs: kill -SIGINT <controller_pid> on remote target (via SSH)
        ├─→ Remote controller's handle_sigint trap fires
        │     ├─→ Sends SIGTERM to each remote script's process group
        │     ├─→ Waits for scripts to exit (up to 60s each)
        │     ├─→ Prints collected output
        │     └─→ Exits 0
        ├─→ SSH process carrying controller output exits naturally
        │     (500ms grace period to avoid interrupting output transfer)
        ├─→ Signal handler detects controller has exited (via SSH ps -p)
        ├─→ Sends SIGINT to any remaining local children (SSH processes)
        └─→ Perfspect processes partial results and exits
```

**Key detail:** The `signalProcessOnTarget()` function (`internal/workflow/signals.go:26-48`) uses `t.RunCommandEx()` to run `kill` on the target. For remote targets, this opens a new SSH session. The `waitTime` of 15 seconds accounts for the controller's 5-second grace period plus network latency.

### Scenario: Remote Target, SIGINT from Another Process

Same as Ctrl-C except:
- Only perfspect receives the signal; neither the local SSH process nor the remote controller are affected by the initial signal.
- The handler follows the same path: signal the remote controller via SSH, wait for exit, then clean up local SSH children.

### Edge Cases and Known Issues

- **Race condition on PID file removal:** The controller deletes `controller.pid` before fully exiting. The signal handler adds a 500ms sleep after detecting PID file removal to let SSH finish transferring output. See comment at `internal/workflow/signals.go:150-155`.
- **SSH exit code 255:** When SSH is interrupted, it returns exit code 255 even if the remote controller exited cleanly. The output parser checks for the `<---------------------->` delimiter to determine if usable output exists and processes it despite the non-zero exit code (`internal/script/script.go:140-149`).

## 3. Metrics-Command Handler (`cmd/metrics/`)

The `metrics` command has its own signal handling because it does not use the controller script. Instead, it streams `perf stat` output directly.

### Architecture

```text
                     perfspect (Go)
                     ┌─────────────────────────────────────────────┐
                     │  signalManager                              │
                     │  - ctx/cancel (context.Context)             │
                     │  - sigChannel (SIGINT, SIGTERM)             │
                     │  - handleSignals() goroutine                │
                     └──────────┬──────────────────────────────────┘
                                │
            ┌───────────────────┼───────────────────┐
            ▼                   ▼                   ▼
     Target A              Target B              Target N
     ┌──────────┐          ┌──────────┐          ┌──────────┐
     │ perf stat│          │ perf stat│          │ perf stat│
     │(streamed)│          │(streamed)│          │(streamed)│
     └──────────┘          └──────────┘          └──────────┘
```

### Signal Flow

The `signalManager` (`cmd/metrics/metrics.go:70-132`) uses a `context.Context` for coordinated shutdown:

1. **On signal receipt** (or internal error triggering `triggerShutdown()`):
   - Cancel the context (`sm.cancel()`).
   - Send `SIGKILL` to all child processes (`util.SignalChildren(syscall.SIGKILL)`).

2. **Collection loop** (`collectOnTarget`, `cmd/metrics/metrics.go:1332`):
   - Checks `signalMgr.shouldStop()` on each iteration. When true, exits the loop.

3. **Processing pipeline** (`processPerfOutput`, `cmd/metrics/metrics.go:1539-1606`):
   - Also monitors the context for cancellation.
   - Can itself trigger shutdown via `signalMgr.triggerShutdown()` on repeated processing errors or cgroup refresh timeout.

**Important difference from reporting commands:** The metrics handler sends `SIGKILL` immediately rather than attempting graceful shutdown with `SIGINT` then escalating to `SIGKILL`. This is because `perf stat` is a streaming process and partial output is already captured in real-time -- there is no risk of losing already-collected data.

### Scenario: Local Target, Ctrl-C in Shell

```text
User presses Ctrl-C
  │
  ├─→ Kernel sends SIGINT to foreground process group
  │     ├─→ perfspect receives SIGINT
  │     └─→ perf stat does NOT receive it (running via RunScriptStream,
  │          but the script command is not in perfspect's process group
  │          because metrics uses the target layer which isolates children)
  │
  └─→ signalManager.handleSignals() activates
        ├─→ Cancels context
        ├─→ Sends SIGKILL to all child processes
        ├─→ collectOnTarget loop sees shouldStop() == true, exits
        ├─→ processPerfOutput sees context cancellation, drains remaining data
        └─→ Perfspect outputs whatever metrics were collected and exits
```

### Scenario: Local Target, SIGINT from Another Process

Identical to Ctrl-C. Only perfspect receives the signal; child processes are killed via `SIGKILL` by the handler.

### Scenario: Remote Target, Ctrl-C in Shell

```text
User presses Ctrl-C
  │
  ├─→ Kernel sends SIGINT to foreground process group
  │     ├─→ perfspect receives SIGINT
  │     └─→ local SSH process does NOT receive it (different PGID)
  │
  └─→ signalManager.handleSignals() activates
        ├─→ Cancels context
        ├─→ Sends SIGKILL to all child processes (including local SSH)
        ├─→ SSH connection drops, remote perf stat is orphaned
        │     (remote perf stat will eventually be cleaned up by the OS
        │      or by the target's temp directory cleanup on next run)
        ├─→ collectOnTarget loop exits
        └─→ Perfspect outputs whatever metrics were streamed so far
```

**Note:** Unlike the reporting-command handler, the metrics handler does not attempt to gracefully stop the remote `perf stat` process. The remote process becomes orphaned when SSH is killed. This is acceptable because metrics data is streamed and processed incrementally.

### Scenario: Remote Target, SIGINT from Another Process

Same as Ctrl-C. Only perfspect receives the signal; the local SSH process and remote `perf stat` are killed/orphaned as described above.

## Summary Table

| Aspect | Root | Reporting Commands | Metrics Command |
|--------|------|--------------------|-----------------|
| **Source file** | `cmd/root.go` | `internal/workflow/signals.go` | `cmd/metrics/metrics.go` |
| **Signals caught** | SIGINT, SIGTERM | SIGINT, SIGTERM | SIGINT, SIGTERM |
| **Active during** | Update check | Data collection | Data collection |
| **Child isolation** | N/A | `Setpgid: true` | `Setpgid: true` (via target layer) |
| **Shutdown strategy** | Immediate exit | Graceful: SIGINT -> wait -> SIGKILL | Immediate: SIGKILL |
| **Partial results** | No | Yes (controller prints summary) | Yes (already streamed) |
| **Remote cleanup** | N/A | Explicit signal via SSH | None (orphaned) |
| **Timeout** | None | 20s per target | None |

## Key Source Files

| File | Role |
|------|------|
| `cmd/root.go:226-239` | Root-level signal handler during update check |
| `internal/workflow/signals.go` | Reporting-command signal handler and target signaling |
| `internal/workflow/workflow.go:149-150` | Where the reporting handler is installed |
| `cmd/metrics/metrics.go:70-132` | Metrics signalManager definition |
| `cmd/metrics/metrics.go:850-851` | Where the metrics handler is installed |
| `internal/script/script.go:128-134` | Controller process group isolation |
| `internal/script/script.go:258-402` | Controller script template (including `handle_sigint` trap) |
| `internal/target/helpers.go:128-130` | `Setpgid` process group isolation |
| `internal/util/util.go:561-640` | Signal utility functions (`SignalProcess`, `SignalChildren`, `SignalSelf`) |
