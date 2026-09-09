// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Package script provides functions to run scripts on a target and get the output.
package script

import (
	"bytes"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"text/template" // nosemgrep

	"perfspect/internal/progress"
	"perfspect/internal/target"
	"perfspect/internal/util"
)

//go:embed resources
var Resources embed.FS

const ControllerPIDFileName = "controller.pid"

// descendantsOfShellFunc defines a shell function that prints a pid followed by
// all of its descendants. A process-group filter is not sufficient: 'timeout'
// puts itself and the command it runs into a new process group, so a probe run as
// 'timeout 30 perf stat ...' is invisible to a PGID-based search and survives a
// kill aimed at the script's group. It is shared by the controller script and by
// the cleanup we run after abandoning a controller.
const descendantsOfShellFunc = `descendants_of() {
  local root="$1" snapshot frontier next depth=0
  snapshot=$(ps -eo pid,ppid 2>/dev/null || true)
  frontier="$root"
  # The depth cap is a safety net only; a process tree cannot be deeper than this
  # in practice, and without it a malformed snapshot could loop forever.
  while [[ -n "$frontier" && "$depth" -lt 32 ]]; do
    echo "$frontier"
    next=$(awk -v parents="$frontier" '
      BEGIN { n = split(parents, a, " "); for (i = 1; i <= n; i++) if (a[i] != "") P[a[i]] = 1 }
      NR > 1 && ($2 in P) { printf "%s ", $1 }
    ' <<< "$snapshot")
    frontier="$next"
    depth=$((depth + 1))
  done
}`

type ScriptOutput struct {
	ScriptDefinition
	Stdout   string
	Stderr   string
	Exitcode int
}

// RunScript runs a script on the specified target and returns the output.
func RunScript(myTarget target.Target, script ScriptDefinition, localTempDir string) (ScriptOutput, error) {
	scriptOutputs, err := RunScripts(myTarget, []ScriptDefinition{script}, false, localTempDir, nil, "")
	if err != nil {
		return ScriptOutput{}, err
	}
	if scriptOutputs == nil {
		return ScriptOutput{}, err
	}
	scriptOutput, exists := scriptOutputs[script.Name]
	if !exists {
		return ScriptOutput{}, fmt.Errorf("script output not found for script: %s", script.Name)
	}
	return scriptOutput, err
}

// RunScripts runs a list of scripts on a target and returns the outputs of each script as a map with the script name as the key.
func RunScripts(myTarget target.Target, scripts []ScriptDefinition, continueOnScriptError bool, localTempDir string, statusUpdate progress.MultiSpinnerUpdateFunc, collectingStatus string) (map[string]ScriptOutput, error) {
	// drop scripts that should not be run and separate scripts that must run sequentially from those that can be run concurrently
	canElevate := myTarget.CanElevatePrivileges()
	var sequentialScripts []ScriptDefinition
	var concurrentScripts []ScriptDefinition
	for _, script := range scripts {
		if script.Superuser && !canElevate {
			slog.Warn("skipping script because it requires superuser privileges and the user cannot elevate privileges on target", slog.String("script", script.Name))
			continue
		}
		if script.Sequential {
			sequentialScripts = append(sequentialScripts, script)
		} else {
			concurrentScripts = append(concurrentScripts, script)
		}
	}
	if len(sequentialScripts) == 0 && len(concurrentScripts) == 0 {
		return nil, fmt.Errorf("no scripts to run on target")
	}
	// prepare target to run scripts by copying scripts and dependencies to target and installing LKMs
	if statusUpdate != nil {
		_ = statusUpdate(myTarget.GetName(), "preparing to collect data")
	}
	installedLkms, err := prepareTargetToRunScripts(myTarget, append(sequentialScripts, concurrentScripts...), localTempDir, false)
	if err != nil {
		return nil, fmt.Errorf("error while preparing target to run scripts: %v", err)
	}
	if len(installedLkms) > 0 {
		defer func() {
			err := myTarget.UninstallLkms(installedLkms)
			if err != nil {
				slog.Error("error uninstalling LKMs", slog.String("lkms", strings.Join(installedLkms, ", ")), slog.String("error", err.Error()))
			}
		}()
	}
	if statusUpdate != nil {
		_ = statusUpdate(myTarget.GetName(), collectingStatus)
	}
	scriptOutputs := make(map[string]ScriptOutput)
	// form a unified controller script that runs all scripts (both concurrent and sequential phases)
	controllerScriptName := "controller.sh"
	controllerScript, needsElevatedPrivileges, err := formControllerScript(myTarget.GetTempDirectory(), concurrentScripts, sequentialScripts, continueOnScriptError)
	if err != nil {
		err = fmt.Errorf("error forming controller script: %v", err)
		return nil, err
	}
	// write controller script to local file
	controllerScriptPath := path.Join(localTempDir, myTarget.GetName(), controllerScriptName)
	err = os.WriteFile(controllerScriptPath, []byte(controllerScript), 0600)
	if err != nil {
		err = fmt.Errorf("error writing controller script to local file: %v", err)
		return nil, err
	}
	// copy controller script to target
	err = myTarget.PushFile(controllerScriptPath, myTarget.GetTempDirectory())
	if err != nil {
		err = fmt.Errorf("error copying script to target: %v", err)
		return nil, err
	}
	// run controller script on target
	// if the controller script requires elevated privileges, we run it with sudo
	// Note: adding 'sudo' to the individual scripts inside the controller script
	// instigates a known bug in the terminal that corrupts the tty settings:
	// https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=1043320
	var cmd *exec.Cmd
	if needsElevatedPrivileges && !canElevate {
		// this shouldn't happen because we already filtered out the scripts that require elevated privileges if the user cannot elevate privileges on the target
		err = fmt.Errorf("controller script requires elevated privileges but the user cannot elevate privileges on target")
		return nil, err
	} else if needsElevatedPrivileges && !myTarget.IsSuperUser() {
		// run controller script with sudo, "-S" to read password from stdin. Note: password won't be asked for if password-less sudo is configured.
		cmd = exec.Command("sudo", "-S", "bash", path.Join(myTarget.GetTempDirectory(), controllerScriptName)) // #nosec G204
	} else {
		cmd = exec.Command("bash", path.Join(myTarget.GetTempDirectory(), controllerScriptName)) // #nosec G204
	}
	// Bound the controller itself when every script is bounded. The per-script
	// watchdogs normally end a hang, but they run on the target: if the target
	// wedges hard enough, or the connection carrying the controller stops
	// delivering, nothing comes back at all. A deadline here guarantees we regain
	// control and can report the partial output and diagnostics collected so far.
	// Scripts with no timeout (e.g. indefinite-duration collection) keep the
	// controller unbounded, as before.
	timeout := controllerTimeout(append(concurrentScripts, sequentialScripts...))
	slog.Debug("running controller script", slog.String("target", myTarget.GetName()), slog.Int("timeout", timeout), slog.Int("scripts", len(concurrentScripts)+len(sequentialScripts)))
	// We run controller in a new process group so that tty/terminal signals, e.g., Ctrl-C, are not sent to the command. This is
	// necessary to allow the controller script to handle signals itself and propagate them to all child scripts as needed. The
	// signal handler in perfspect will send the signal to the controller.sh script on each target so that it can clean up
	// its child processes.
	newProcessGroup := true
	reuseSSHConnection := false // don't reuse ssh connection on long-running commands, makes it difficult to kill the command
	// Stream the controller's stderr rather than only reading it at the end. Its
	// SCRIPT START/RESULT reports exist to name the script that hung, and a hung
	// controller does not return, so parsing them only after it exits means they are
	// missing from precisely the runs that need them.
	progress := &controllerProgressLogger{}
	stdout, stderr, exitcode, err := myTarget.RunCommandExLive(cmd, timeout, newProcessGroup, reuseSSHConnection, progress)
	progress.flush()
	if err != nil {
		slog.Error("failed to execute controller script on target", slog.String("stdout", stdout), slog.String("stderr", stderr), slog.Int("exitcode", exitcode), slog.String("error", err.Error()))
		return nil, err
	}
	// A negative exit code means the process was signalled rather than exiting on
	// its own, which for a bounded run means our deadline killed it. Say so
	// explicitly: the alternative is an unexplained failure that looks identical to
	// a crash. The SCRIPT START lines above name the scripts that never finished.
	if exitcode < 0 && timeout > 0 {
		slog.Error("controller script did not finish within its deadline and was terminated",
			slog.String("target", myTarget.GetName()), slog.Int("deadlineSeconds", timeout))
		// Killing our end of the connection does not stop anything on the target: the
		// controller and its probes keep running there, unattached and unbounded. On a
		// shared or repeatedly tested machine those leftovers accumulate and contend
		// for the resource the next run's probes need, so one hang turns into a run of
		// them. Reap them before returning.
		CleanupAbandonedController(myTarget)
	}
	if exitcode != 0 {
		// If the controller was interrupted (e.g., by SIGINT) but still produced output,
		// parse the output rather than discarding it. This handles the case where the
		// controller's handle_sigint exits 0 but the SSH process carrying it gets
		// interrupted and returns a non-zero exit code (e.g., 255).
		if strings.Contains(stdout, "<---------------------->") {
			slog.Warn("controller script returned non-zero exit code, but output is available and will be processed", slog.Int("exitcode", exitcode), slog.String("stderr", stderr))
		} else {
			slog.Error("controller script returned non-zero exit code", slog.String("stdout", stdout), slog.String("stderr", stderr), slog.Int("exitcode", exitcode))
			// Include stderr in the error itself. It carries the reason -- an ssh
			// transport failure (exit 255) is otherwise indistinguishable from a
			// failure in the scripts, and the distinction is not recoverable from
			// the exit code alone.
			return nil, fmt.Errorf("controller script returned exit code %d: %s", exitcode, lastLines(stderr, 5))
		}
	}
	// parse output of controller script
	allScriptOutputs := parseControllerScriptOutput(stdout)
	for _, scriptOutput := range allScriptOutputs {
		// find associated script (concurrent or sequential)
		for _, script := range append(concurrentScripts, sequentialScripts...) {
			if script.Name == scriptOutput.Name {
				scriptOutput.ScriptTemplate = script.ScriptTemplate
				scriptOutputs[scriptOutput.Name] = scriptOutput
				break
			}
		}
	}
	return scriptOutputs, nil
}

// controllerTimeoutMargin is added to the sum of the script timeouts to allow for
// the controller's own setup and for reporting results back.
const controllerTimeoutMargin = 60

// watchdogEscalationSeconds is how much longer than its own budget a hung script
// can occupy the controller: the watchdog waits WATCHDOG_KILL_AFTER seconds after
// SIGTERM, the same again after SIGKILL, plus its 1-second polling granularity.
// A deadline that omits this is guaranteed to fire during the escalation of a
// hang -- exactly when the controller is producing the diagnosis of it -- and
// killing the controller discards all of its output, because results are printed
// only once every script has finished.
const watchdogEscalationSeconds = 12

// controllerTimeout returns a deadline in seconds for the whole controller run,
// or 0 for no deadline. A deadline is only imposed when every script is itself
// bounded: a single unbounded script (indefinite-duration collection) means the
// controller legitimately has no upper bound.
//
// Sequential scripts run one after another, so their budgets add up, whereas
// concurrent scripts overlap and only the largest matters. Each phase gets the
// watchdog escalation allowance on top, since a script that hangs holds the
// controller for its budget plus the time taken to force it out.
func controllerTimeout(scripts []ScriptDefinition) int {
	sequentialTotal := 0
	maxConcurrent := 0
	for _, s := range scripts {
		if s.Timeout <= 0 {
			return 0
		}
		if s.Sequential {
			sequentialTotal += s.Timeout + watchdogEscalationSeconds
		} else if s.Timeout > maxConcurrent {
			maxConcurrent = s.Timeout
		}
	}
	if maxConcurrent > 0 {
		maxConcurrent += watchdogEscalationSeconds
	}
	return sequentialTotal + maxConcurrent + controllerTimeoutMargin
}

// cleanupTemplate kills a controller left running on a target, along with every
// process below it, and reports anything that survives. Killing the local end of
// the connection has no effect on the target, so without this a probe that
// outlived its watchdog keeps running there indefinitely.
const cleanupTemplate = `
%s
pidfile="%s"
[ -r "$pidfile" ] || exit 0
root=$(cat "$pidfile" 2>/dev/null || true)
# Refuse anything that is not a plain pid: this string comes from a file, and it
# is about to be handed to kill.
case "$root" in ''|*[!0-9]*) exit 0 ;; esac
ps -p "$root" > /dev/null 2>&1 || { rm -f "$pidfile"; exit 0; }
# Capture the tree before signalling: the first kill orphans the descendants and
# they can no longer be traced back to the controller.
tree=$(descendants_of "$root")
echo "cleaning up abandoned controller pid=$root"
for p in $tree; do
  cmd=$(tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null || true)
  [ -n "$cmd" ] && echo "  killing pid=$p cmd=$cmd"
done
# shellcheck disable=SC2086 # word splitting is intended: tree is a pid list
kill -SIGKILL $tree 2>/dev/null || true
sleep 1
for p in $tree; do
  if ps -p "$p" > /dev/null 2>&1; then
    echo "  SURVIVED SIGKILL pid=$p state=$(ps -o stat= -p "$p" 2>/dev/null | tr -d ' ')" >&2
  fi
done
rm -f "$pidfile"
`

// CleanupAbandonedController kills the controller script and its descendants on a
// target after we have stopped waiting for them. It is best-effort: it reports
// failures rather than returning them, because every caller is already on an error
// path and cleanup failing must not mask the original problem.
func CleanupAbandonedController(myTarget target.Target) {
	pidFile := path.Join(myTarget.GetTempDirectory(), ControllerPIDFileName)
	cleanupScript := fmt.Sprintf(cleanupTemplate, descendantsOfShellFunc, pidFile)
	var cmd *exec.Cmd
	if !myTarget.IsSuperUser() && myTarget.CanElevatePrivileges() {
		// The controller runs under sudo, so its children are root-owned.
		cmd = exec.Command("sudo", "bash", "-c", cleanupScript) // #nosec G204
	} else {
		cmd = exec.Command("bash", "-c", cleanupScript) // #nosec G204
	}
	stdout, stderr, exitcode, err := myTarget.RunCommandEx(cmd, 30, false, true)
	if err != nil {
		slog.Error("failed to clean up abandoned controller on target",
			slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line != "" {
			slog.Warn("abandoned controller cleanup", slog.String("target", myTarget.GetName()), slog.String("detail", strings.TrimSpace(line)))
		}
	}
	// A process that survives SIGKILL is blocked in the kernel and cannot be reaped
	// from user space at all; it needs to be reported, not retried.
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		if strings.Contains(line, "SURVIVED SIGKILL") {
			slog.Error("process on target survived SIGKILL and could not be reaped",
				slog.String("target", myTarget.GetName()), slog.String("detail", strings.TrimSpace(line)))
		}
	}
	if exitcode != 0 {
		slog.Warn("abandoned controller cleanup returned non-zero exit code",
			slog.String("target", myTarget.GetName()), slog.Int("exitcode", exitcode), slog.String("stderr", stderr))
	}
}

// logControllerDiagnosticLine surfaces one line of the controller's own reporting.
// The controller writes these to stderr, and when continuing on script error it still
// exits 0, so without this a script that hung or was abandoned would not be logged
// anywhere. It classifies a single line rather than a whole stderr buffer because the
// lines are consumed as they stream in, before the controller has exited.
func logControllerDiagnosticLine(line string) {
	switch {
	case strings.HasPrefix(line, "TIMEOUT DIAG:"):
		// Process state and kernel stack of a script that would not die.
		slog.Warn("hung script diagnostics", slog.String("detail", strings.TrimPrefix(line, "TIMEOUT DIAG: ")))
	case strings.HasPrefix(line, "TIMEOUT:"):
		slog.Warn("script exceeded its timeout", slog.String("detail", strings.TrimPrefix(line, "TIMEOUT: ")))
	case strings.Contains(line, "ABANDONED"):
		slog.Warn("script could not be stopped and was abandoned", slog.String("detail", strings.TrimPrefix(line, "SCRIPT RESULT: ")))
	case strings.HasPrefix(line, "SCRIPT RESULT:"):
		slog.Debug("script result", slog.String("detail", strings.TrimPrefix(line, "SCRIPT RESULT: ")))
	case strings.HasPrefix(line, "SCRIPT START:"):
		slog.Debug("script started", slog.String("detail", strings.TrimPrefix(line, "SCRIPT START: ")))
	}
}

// controllerProgressLogger logs the controller's progress reports as they arrive on
// its stderr. Without it those reports are parsed only after the controller exits,
// so a run that hangs -- the case they exist to explain -- produces none of them:
// the log simply stops after "running controller script" and never names the script
// that stalled. Writing them out as they stream means a stall identifies itself
// while it is still stalled, from the local side, without needing the target to
// answer anything.
type controllerProgressLogger struct {
	partial []byte
}

// maxControllerProgressLine bounds how much unterminated output is buffered, so
// stderr without newlines cannot grow this without limit.
const maxControllerProgressLine = 64 * 1024

func (l *controllerProgressLogger) Write(p []byte) (int, error) {
	l.partial = append(l.partial, p...)
	for {
		i := bytes.IndexByte(l.partial, '\n')
		if i < 0 {
			break
		}
		logControllerDiagnosticLine(string(l.partial[:i]))
		l.partial = l.partial[i+1:]
	}
	if len(l.partial) > maxControllerProgressLine {
		logControllerDiagnosticLine(string(l.partial))
		l.partial = nil
	}
	return len(p), nil
}

// flush logs a final report that arrived without a trailing newline, which is what
// a controller killed mid-write leaves behind.
func (l *controllerProgressLogger) flush() {
	if len(l.partial) > 0 {
		logControllerDiagnosticLine(string(l.partial))
		l.partial = nil
	}
}

// lastLines returns up to n trailing non-empty lines of s, joined by "; ", for
// embedding in an error message.
func lastLines(s string, n int) string {
	var lines []string
	for line := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	if len(lines) == 0 {
		return "(no stderr)"
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "; ")
}

// RunScriptStream runs a script on the specified target and streams the output to the specified channels.
func RunScriptStream(myTarget target.Target, script ScriptDefinition, localTempDir string, stdoutChannel chan []byte, stderrChannel chan []byte, exitcodeChannel chan int, errorChannel chan error, cmdChannel chan *exec.Cmd) {
	installedLkms, err := prepareTargetToRunScripts(myTarget, []ScriptDefinition{script}, localTempDir, true)
	if err != nil {
		err = fmt.Errorf("error while preparing target to run script: %v", err)
		errorChannel <- err
		return
	}
	if len(installedLkms) != 0 {
		defer func() {
			err := myTarget.UninstallLkms(installedLkms)
			if err != nil {
				slog.Error("error uninstalling LKMs", slog.String("lkms", strings.Join(installedLkms, ", ")), slog.String("error", err.Error()))
			}
		}()
	}
	cmd := prepareCommand(script, myTarget)
	err = myTarget.RunCommandStream(cmd, stdoutChannel, stderrChannel, exitcodeChannel, cmdChannel)
	errorChannel <- err
}

// prepareCommand prepares the command to run the specified script on the target.
// If the script requires superuser privileges and the target's user is not already superuser, run with sudo.
func prepareCommand(script ScriptDefinition, myTarget target.Target) (cmd *exec.Cmd) {
	scriptPath := path.Join(myTarget.GetTempDirectory(), scriptNameToFilename(script.Name))
	if script.Superuser && !myTarget.IsSuperUser() {
		cmd = exec.Command("sudo", "bash", scriptPath) // #nosec G204
	} else {
		cmd = exec.Command("bash", scriptPath) // #nosec G204
	}
	return
}

func sanitizeScriptName(name string) string {
	sanitized := strings.ReplaceAll(name, " ", "_")
	sanitized = strings.ReplaceAll(sanitized, "-", "_")
	return sanitized
}

func scriptNameToFilename(name string) string {
	if name == "" {
		panic("script name cannot be empty")
	}
	return sanitizeScriptName(name) + ".sh"
}

// formControllerScript forms a controller script that runs all scripts in two phases:
// first all concurrent scripts together in the background, then all sequential scripts one-by-one.
// It handles signals and cleanup for both phases uniformly.
// If continueOnScriptError is false, when running sequential scripts, the controller script will stop executing further
// sequential scripts upon the first script failure and return an error.
// Return values are the controller script and a boolean indicating whether the controller script requires elevated privileges.
func formControllerScript(targetTempDirectory string, concurrentScripts []ScriptDefinition, sequentialScripts []ScriptDefinition, continueOnScriptError bool) (string, bool, error) {
	// tplScript holds the minimal per-script fields passed into the
	// template that renders the shell controller script.
	// Primarily carries the sanitized script name used for filenames and
	// template keys (e.g., ${s}.sh, ${s}.stdout, pids[$s]), while the original
	// Name is kept for readable summary output. Timeout is the script's
	// watchdog budget in seconds; 0 means the script may run indefinitely.
	type tplScript struct {
		Name      string
		Sanitized string
		Timeout   int
	}
	// tplData holds all data passed into the controller script template.
	tplData := struct {
		TargetTempDir         string
		ControllerPIDFile     string
		DescendantsFunc       string
		ConcurrentScripts     []tplScript
		SequentialScripts     []tplScript
		ContinueOnScriptError bool
	}{}
	// populate tplData
	tplData.TargetTempDir = targetTempDirectory
	tplData.ControllerPIDFile = ControllerPIDFileName
	tplData.DescendantsFunc = descendantsOfShellFunc
	tplData.ContinueOnScriptError = continueOnScriptError
	needsElevated := false
	for _, s := range concurrentScripts {
		if s.Superuser {
			needsElevated = true
		}
		tplData.ConcurrentScripts = append(tplData.ConcurrentScripts, tplScript{
			Name: s.Name, Sanitized: sanitizeScriptName(s.Name), Timeout: s.Timeout,
		})
	}
	for _, s := range sequentialScripts {
		if s.Superuser {
			needsElevated = true
		}
		tplData.SequentialScripts = append(tplData.SequentialScripts, tplScript{
			Name: s.Name, Sanitized: sanitizeScriptName(s.Name), Timeout: s.Timeout,
		})
	}
	// define controller script template
	const controllerScriptTemplate = `#!/usr/bin/env bash
set -o errexit
set -o pipefail

script_dir={{.TargetTempDir}}
cd "$script_dir"

# write our pid to a file so that perfspect can send us a signal if needed
echo $$ > {{.ControllerPIDFile}}

declare -a concurrent_scripts=()
declare -a sequential_scripts=()
declare -A pids=()
declare -A exitcodes=()
declare -A orig_names=()
declare -A timeouts=()
declare -A watchdog_pids=()
declare -A start_times=()
current_seq_pid=""
current_seq_script=""
# set by wait_for_script: 1 when we stopped waiting on an unkillable script
last_wait_abandoned=0

continue_on_script_error={{if .ContinueOnScriptError}}1{{else}}0{{end}}

ensure_trailing_newline() {
    local f="$1"
    if [ ! -f "$f" ]; then return; fi
    cat "$f" || true
    if [ -s "$f" ]; then
        if [ "$(tail -c 1 "$f" 2>/dev/null | wc -l)" -eq 0 ]; then echo; fi
    fi
}

{{- range .ConcurrentScripts}}
concurrent_scripts+=({{ .Sanitized }})
orig_names[{{ .Sanitized }}]="{{ .Name }}"
timeouts[{{ .Sanitized }}]={{ .Timeout }}
{{ end }}
{{- range .SequentialScripts}}
sequential_scripts+=({{ .Sanitized }})
orig_names[{{ .Sanitized }}]="{{ .Name }}"
timeouts[{{ .Sanitized }}]={{ .Timeout }}
{{ end }}

# Grace period between the watchdog's SIGTERM and its follow-up SIGKILL.
readonly WATCHDOG_KILL_AFTER=5

# Grace period kill_script allows a script to exit after SIGTERM before it
# escalates to SIGKILL, during signal-triggered cleanup.
readonly KILL_GRACE_SECONDS=5

{{.DescendantsFunc}}

# dump_pid_states reports the kernel state of each given pid, skipping any that
# have exited. Process state D is uninterruptible sleep: the process is blocked
# inside a kernel call and will not act on any signal -- not even SIGKILL -- until
# that call returns. That is what distinguishes a probe wedged on a PMU access
# from a merely slow command, and it is why signalling alone cannot always reap
# it. The kernel stack names the exact call it is stuck in, and is readable
# because metadata scripts run with elevated privileges.
dump_pid_states() {
  local label="$1"
  shift
  local p st wch cmd
  for p in "$@"; do
    ps -p "$p" > /dev/null 2>&1 || continue
    # Read state via ps rather than parsing /proc/<pid>/stat: the comm field there
    # is parenthesized and may contain spaces, which shifts the field positions.
    st=$(ps -o stat= -p "$p" 2>/dev/null | tr -d ' ')
    wch=$(cat "/proc/$p/wchan" 2>/dev/null || true)
    cmd=$(tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null || true)
    echo "TIMEOUT DIAG: $label pid=$p state=${st:-?} wchan=${wch:-?} cmd=${cmd:-?}" >&2
    if [[ "$st" == D* ]]; then
      echo "TIMEOUT DIAG: pid=$p is in uninterruptible sleep and cannot be signalled; kernel stack:" >&2
      cat "/proc/$p/stack" 2>/dev/null >&2 || echo "TIMEOUT DIAG: (kernel stack unavailable)" >&2
    fi
  done
}

# dump_hung_process_state records why a script could not be stopped. The caller
# passes a pid list captured before any signal was sent: once the script's shell
# dies its children are reparented to init, so the tree cannot be recovered
# afterwards.
dump_hung_process_state() {
  local s="$1" pid="$2" tree="$3"
  echo "TIMEOUT DIAG: process tree for script '${orig_names[$s]}' (root pid $pid):" >&2
  ps -eo pid,ppid,pgid,stat,etime,wchan:24,args 2>/dev/null | awk -v pids="$tree" '
    BEGIN { n = split(pids, a, /[ \n]+/); for (i = 1; i <= n; i++) if (a[i] != "") P[a[i]] = 1 }
    NR == 1 || ($1 in P)
  ' >&2 || true
  # shellcheck disable=SC2086 # word splitting is intended: tree is a pid list
  dump_pid_states "in tree" $tree
}

# kill_tree signals a script's process group and then every process in the
# previously captured tree individually. The group kill alone reaps the script's
# shell but leaves a 'timeout'-wrapped probe running in its own group, orphaned
# and still holding whatever it was stuck on. Descendants that lead a group get
# the signal on their group too, so a probe forked below 'timeout' is covered.
kill_tree() {
  local pid="$1" sig="$2" tree="$3" p pg
  kill "-$sig" -"$pid" 2>/dev/null || true
  for p in $tree; do
    [[ "$p" == "$pid" ]] && continue
    pg=$(ps -o pgid= -p "$p" 2>/dev/null | tr -d ' ')
    if [[ "$pg" == "$p" ]]; then
      kill "-$sig" -"$p" 2>/dev/null || true
    else
      kill "-$sig" "$p" 2>/dev/null || true
    fi
  done
}

# start_watchdog starts a background timer for a script. Each script runs via
# setsid, so it leads its own process group; the watchdog signals the whole
# group (negative PID). This is what makes the timeout forceful: signalling only
# the script's direct child would leave a wedged grandchild (e.g. a perf stuck in
# the kernel) running, and the controller's 'wait' would block on it forever.
#
# If the group survives even SIGKILL it is abandoned: a marker file tells the
# waiter to stop waiting on it. Otherwise an unkillable probe would block the
# controller forever, and none of the diagnostics below would ever be reported,
# because the caller only reads our output once we exit.
start_watchdog() {
  local s="$1" pid="$2" budget="${timeouts[$1]:-0}"
  [[ "$budget" -le 0 ]] && return 0
  (
    # Poll rather than 'sleep $budget' so the watchdog exits promptly once the
    # script finishes, instead of lingering for the full budget.
    local waited=0
    while [[ "$waited" -lt "$budget" ]]; do
      ps -p "$pid" > /dev/null 2>&1 || exit 0
      sleep 1
      waited=$((waited + 1))
    done
    ps -p "$pid" > /dev/null 2>&1 || exit 0
    echo "TIMEOUT: script '${orig_names[$s]}' exceeded ${budget}s; sending SIGTERM to process tree of $pid" >&2
    # Capture the tree before signalling anything: the first kill orphans the
    # descendants, and an orphan cannot be traced back to this script.
    local tree
    tree=$(descendants_of "$pid")
    dump_hung_process_state "$s" "$pid" "$tree"
    kill_tree "$pid" SIGTERM "$tree"
    local killwait=0
    while ps -p "$pid" > /dev/null 2>&1 && [[ "$killwait" -lt "$WATCHDOG_KILL_AFTER" ]]; do
      sleep 1
      killwait=$((killwait + 1))
    done
    if ps -p "$pid" > /dev/null 2>&1; then
      echo "TIMEOUT: script '${orig_names[$s]}' ignored SIGTERM after ${WATCHDOG_KILL_AFTER}s; sending SIGKILL to process tree of $pid" >&2
      kill_tree "$pid" SIGKILL "$tree"
      killwait=0
      while ps -p "$pid" > /dev/null 2>&1 && [[ "$killwait" -lt "$WATCHDOG_KILL_AFTER" ]]; do
        sleep 1
        killwait=$((killwait + 1))
      done
      if ps -p "$pid" > /dev/null 2>&1; then
        echo "TIMEOUT: script '${orig_names[$s]}' survived SIGKILL; abandoning it so collection can continue" >&2
        dump_hung_process_state "$s" "$pid" "$tree"
        touch "$script_dir/${s}.abandoned"
      fi
    fi
    # Anything from the tree that is still alive here ignored SIGKILL, which only
    # a process blocked in the kernel can do. Report it even when the script's own
    # shell died: a leaked probe still holding a PMU resource is the most likely
    # reason the scripts that ran after it also hung.
    # shellcheck disable=SC2086 # word splitting is intended: tree is a pid list
    dump_pid_states "survived SIGKILL" $tree
  ) &
  watchdog_pids[$s]=$!
}

# wait_for_script waits for a script to exit, but gives up if its watchdog has
# abandoned it as unkillable. Sets last_wait_abandoned=1 in that case, and
# otherwise returns the script's real exit status. A plain 'wait' cannot be used
# here: it blocks forever on a process stuck in uninterruptible sleep.
wait_for_script() {
  local s="$1" pid="$2"
  last_wait_abandoned=0
  while ps -p "$pid" > /dev/null 2>&1; do
    if [[ -f "$script_dir/${s}.abandoned" ]]; then
      last_wait_abandoned=1
      return 0
    fi
    sleep 1
  done
  # The process has exited, so this returns immediately with its real status.
  wait "$pid"
}

# stop_watchdog cancels a script's watchdog once the script has exited.
stop_watchdog() {
  local s="$1" wpid="${watchdog_pids[$1]:-}"
  [[ -z "$wpid" ]] && return 0
  kill -SIGKILL "$wpid" 2>/dev/null || true
  wait "$wpid" 2>/dev/null || true
  unset 'watchdog_pids[$s]'
}

# report_script_result logs a script's exit code and elapsed time, and on a
# timeout kill (SIGTERM=143, SIGKILL=137) or generic failure also emits a tail of
# its stderr. This identifies exactly which probe hung, rather than leaving a
# silent stall.
report_script_result() {
  local s="$1" ec="$2"
  local elapsed=$(( $(date +%s) - ${start_times[$s]:-0} ))
  echo "SCRIPT RESULT: '${orig_names[$s]}' exit=$ec elapsed=${elapsed}s" >&2
  if [[ "$ec" -ne 0 ]]; then
    if [[ "$ec" -eq 143 || "$ec" -eq 137 ]]; then
      echo "SCRIPT RESULT: '${orig_names[$s]}' was killed by the watchdog (likely hung)" >&2
    fi
    if [[ -s "$script_dir/${s}.stderr" ]]; then
      echo "SCRIPT RESULT: '${orig_names[$s]}' stderr tail:" >&2
      tail -n 20 "$script_dir/${s}.stderr" >&2 || true
    fi
  fi
}

# report_abandoned_script records a script we gave up waiting for. The exit code
# is synthetic: the process is still alive, so there is no real status to report.
report_abandoned_script() {
  local s="$1"
  local elapsed=$(( $(date +%s) - ${start_times[$s]:-0} ))
  echo "SCRIPT RESULT: '${orig_names[$s]}' ABANDONED after ${elapsed}s (unkillable, still running)" >&2
  if [[ -s "$script_dir/${s}.stderr" ]]; then
    echo "SCRIPT RESULT: '${orig_names[$s]}' stderr tail:" >&2
    tail -n 20 "$script_dir/${s}.stderr" >&2 || true
  fi
  exitcodes[$s]=137
}

# announce_script names a script as it starts. Without this, a controller that
# dies or is killed before producing results gives no indication of which script
# it had reached.
announce_script() {
  echo "SCRIPT START: '${orig_names[$1]}' pid=$2 budget=${timeouts[$1]:-0}s" >&2
}

start_concurrent_scripts() {
  for s in "${concurrent_scripts[@]}"; do
    setsid bash "$script_dir/${s}.sh" > "$script_dir/${s}.stdout" 2> "$script_dir/${s}.stderr" &
    pids[$s]=$!
    start_times[$s]=$(date +%s)
    announce_script "$s" "${pids[$s]}"
    start_watchdog "$s" "${pids[$s]}"
  done
}

run_sequential_scripts() {
  for s in "${sequential_scripts[@]}"; do
    current_seq_pid=""
    current_seq_script="$s"
    setsid bash "$script_dir/${s}.sh" > "$script_dir/${s}.stdout" 2> "$script_dir/${s}.stderr" &
    current_seq_pid=$!
    pids[$s]=$current_seq_pid
    start_times[$s]=$(date +%s)
    announce_script "$s" "$current_seq_pid"
    start_watchdog "$s" "$current_seq_pid"
    if wait_for_script "$s" "$current_seq_pid"; then
      stop_watchdog "$s"
      if [[ "$last_wait_abandoned" -eq 1 ]]; then
        report_abandoned_script "$s"
      else
        exitcodes[$s]=0
        report_script_result "$s" 0
      fi
    else
      ec=$?
      exitcodes[$s]=$ec
      stop_watchdog "$s"
      report_script_result "$s" "$ec"
      if [ "$continue_on_script_error" -eq 0 ]; then
        echo "Script '${orig_names[$s]}' failed with exit code $ec; stopping further sequential scripts." >&2
        exit $ec
      fi
    fi
    current_seq_pid=""
    current_seq_script=""
  done
}

kill_script() {
  local s="$1"
  local pid="${pids[$s]:-}"
  stop_watchdog "$s"
  [[ -z "$pid" ]] && return 0
  if ! ps -p "$pid" > /dev/null 2>&1; then return 0; fi
  # Signal the process group and every descendant (see kill_tree: a
  # 'timeout'-wrapped probe lives in its own group and outlives a group kill).
  # Bash background jobs ignore SIGINT by default, but they do not ignore SIGTERM.
  echo "Sending SIGTERM to script '${orig_names[$s]}' with PID $pid" >&2
  local tree
  tree=$(descendants_of "$pid")
  kill_tree "$pid" SIGTERM "$tree"
  # Wait for the script to exit gracefully, in 1s intervals.
  # This budget is per-script and cleanup is serial, so it must stay small: the
  # signal handler in perfspect only allows ~20s for the whole controller to exit
  # before it escalates to SIGKILL. A long budget here (it was 60s) makes a single
  # hung script stall shutdown well past that deadline.
  local waited=0
  echo "Waiting for script '${orig_names[$s]}' with PID $pid to exit gracefully" >&2
  while ps -p "$pid" > /dev/null 2>&1 && [ "$waited" -lt "$KILL_GRACE_SECONDS" ]; do
    echo -n "." >&2
    sleep 1
    waited=$((waited + 1))
  done
  echo "Done waiting for script '${orig_names[$s]}' with PID $pid to exit gracefully" >&2
  # Force kill the process group if still alive
  if ps -p "$pid" > /dev/null 2>&1; then
    echo "Force killing script '${orig_names[$s]}' with PID $pid" >&2
    kill_tree "$pid" SIGKILL "$tree"
    # Give SIGKILL a moment to land, then report if it did not. Do not 'wait'
    # here: a process in uninterruptible sleep survives SIGKILL until its kernel
    # call returns, and waiting on it would stall shutdown indefinitely -- past
    # the ~20s the perfspect signal handler allows before it escalates.
    sleep 1
    if ps -p "$pid" > /dev/null 2>&1; then
      echo "Script '${orig_names[$s]}' with PID $pid survived SIGKILL; abandoning it" >&2
      dump_hung_process_state "$s" "$pid" "$tree"
    fi
  fi
  echo "Done killing script '${orig_names[$s]}' with PID $pid" >&2
  if [[ -z "${exitcodes[$s]:-}" ]]; then
    echo "Setting exit code for script '${orig_names[$s]}' to 143 (terminated by SIGTERM)" >&2
    exitcodes[$s]=143
  fi
}

wait_for_concurrent_scripts() {
  for s in "${concurrent_scripts[@]}"; do
    local abandoned=0
    if wait_for_script "$s" "${pids[$s]}"; then
      if [[ "$last_wait_abandoned" -eq 1 ]]; then
        abandoned=1
      else
        exitcodes[$s]=0
      fi
    else
      ec=$?
      exitcodes[$s]=$ec
    fi
    stop_watchdog "$s"
    if [[ "$abandoned" -eq 1 ]]; then
      report_abandoned_script "$s"
    else
      report_script_result "$s" "${exitcodes[$s]}"
    fi
  done
}

print_summary() {
  local all_scripts=("${concurrent_scripts[@]}" "${sequential_scripts[@]}")
  for s in "${all_scripts[@]}"; do
    echo "<---------------------->"
    echo "SCRIPT NAME: ${orig_names[$s]}"
    echo "STDOUT:"; ensure_trailing_newline "$script_dir/${s}.stdout"
    echo "STDERR:"; ensure_trailing_newline "$script_dir/${s}.stderr"
    echo "EXIT CODE: ${exitcodes[$s]:-1}"
  done
}

handle_sigint() {
  echo "Received SIGINT; attempting graceful shutdown" >&2
  # kill all running concurrent scripts
  for s in "${concurrent_scripts[@]}"; do
    kill_script "$s"
  done
  # kill current sequential script if running
  if [[ -n "$current_seq_script" ]]; then
    kill_script "$current_seq_script"
  fi
  print_summary
  rm -f {{.ControllerPIDFile}}
  exit 0
}

trap handle_sigint SIGINT

# run concurrent scripts first
start_concurrent_scripts
wait_for_concurrent_scripts
# then run sequential scripts
run_sequential_scripts
print_summary
rm -f {{.ControllerPIDFile}}
`
	// render controller script template
	tmpl, err := template.New("controller").Parse(controllerScriptTemplate)
	if err != nil {
		slog.Error("failed to parse controller script template", slog.String("error", err.Error()))
		return "", needsElevated, err
	}
	var out strings.Builder
	if err = tmpl.Execute(&out, tplData); err != nil {
		slog.Error("failed to execute controller script template", slog.String("error", err.Error()))
		return "", needsElevated, err
	}
	return out.String(), needsElevated, nil
}

// parseControllerScriptOutput parses the output of the controller script that runs all scripts (concurrent and sequential).
// It returns a list of ScriptOutput objects, one for each script that was run.
func parseControllerScriptOutput(controllerScriptOutput string) (scriptOutputs []ScriptOutput) {
	// split output of controller script into individual script outputs
	for output := range strings.SplitSeq(controllerScriptOutput, "<---------------------->\n") {
		lines := strings.Split(output, "\n")
		if len(lines) < 4 { // minimum lines for a script output
			continue
		}
		if !strings.HasPrefix(lines[0], "SCRIPT NAME: ") {
			slog.Warn("skipping output because it does not contain script name", slog.String("output", output))
			continue
		}
		scriptName := strings.TrimSpace(strings.TrimPrefix(lines[0], "SCRIPT NAME: "))
		var stdout string
		var stderr string
		var exitcode string
		var stdoutLines []string
		var stderrLines []string
		stdoutStarted := false
		stderrStarted := false
		for _, line := range lines[1:] {
			if strings.HasPrefix(line, "STDOUT:") {
				stdoutStarted = true
				stderrStarted = false
				continue
			}
			if strings.HasPrefix(line, "STDERR:") {
				stderrStarted = true
				stdoutStarted = false
				continue
			}
			if exitCodeStr, found := strings.CutPrefix(line, "EXIT CODE:"); found {
				exitcode = strings.TrimSpace(exitCodeStr)
				stdoutStarted = false
				stderrStarted = false
				break
			}
			if stdoutStarted {
				stdoutLines = append(stdoutLines, line)
			} else if stderrStarted {
				stderrLines = append(stderrLines, line)
			}
		}
		if len(stdoutLines) > 0 {
			stdoutLines = append(stdoutLines, "") // add a newline at the end to match the original output
		}
		if len(stderrLines) > 0 {
			stderrLines = append(stderrLines, "") // add a newline at the end to match the original output
		}
		stdout = strings.Join(stdoutLines, "\n")
		stderr = strings.Join(stderrLines, "\n")
		exitCodeInt := -100
		if exitcode == "" {
			slog.Warn("exit code for script not set", slog.String("script", scriptName))
		} else {
			var err error
			exitCodeInt, err = strconv.Atoi(exitcode)
			if err != nil {
				slog.Warn("error converting exit code to integer", slog.String("exitcode", exitcode), slog.String("error", err.Error()), slog.String("script", scriptName))
			}
		}
		scriptOutputs = append(scriptOutputs, ScriptOutput{
			ScriptDefinition: ScriptDefinition{Name: scriptName},
			Stdout:           stdout,
			Stderr:           stderr,
			Exitcode:         exitCodeInt,
		})
	}
	return
}

// prepareTargetToRunScripts prepares the target to run the specified scripts by copying the scripts and their dependencies to the target and installing the required LKMs on the target.
func prepareTargetToRunScripts(myTarget target.Target, scripts []ScriptDefinition, localTempDir string, failIfDependencyNotFound bool) (installedLkms []string, err error) {
	// verify temporary directory exists on target
	targetTempDirectory := myTarget.GetTempDirectory()
	if targetTempDirectory == "" {
		panic("target temporary directory cannot be empty")
	}
	// build the path that will be inserted into the script
	// to set the PATH variable
	userPath, err := myTarget.GetUserPath()
	if err != nil {
		err = fmt.Errorf("error while retrieving user's path: %v", err)
		return
	}
	userPath = fmt.Sprintf("%s:%s", targetTempDirectory, userPath)
	// get the target architecture
	// this is used to determine which dependencies to copy to the target
	targetArchitecture, err := myTarget.GetArchitecture()
	if err != nil {
		err = fmt.Errorf("error getting target architecture: %v", err)
		return
	}
	// for each script that will be run on this target
	// -- get the unique list of lkms to install on target
	// -- get the unique list of dependencies to copy to target
	// -- write the script to the target's local temp dir and then copy it to the target
	lkmsToInstall := make(map[string]int)
	dependenciesToCopy := make(map[string]int)
	for _, script := range scripts {
		// add lkms to list of lkms to install
		for _, lkm := range script.Lkms {
			lkmsToInstall[lkm] = 1
		}
		// add dependencies to list of dependencies to copy to target
		for _, dependency := range script.Depends {
			dependenciesToCopy[path.Join(targetArchitecture, dependency)] = 1
		}
		// add cd command to the script to change to the target's local temp directory
		targetScript := fmt.Sprintf("cd %s\n%s", targetTempDirectory, script.ScriptTemplate)
		// add PATH (including the target temporary directory) to the script
		targetScript = fmt.Sprintf("export PATH=\"%s\"\n%s", userPath, targetScript)
		// write script to the target's local temp directory
		scriptPath := path.Join(localTempDir, myTarget.GetName(), scriptNameToFilename(script.Name))
		err = os.WriteFile(scriptPath, []byte(targetScript), 0600)
		if err != nil {
			err = fmt.Errorf("error writing script to local file: %v", err)
			return
		}
		// copy script to target
		err = myTarget.PushFile(scriptPath, path.Join(targetTempDirectory, scriptNameToFilename(script.Name)))
		if err != nil {
			err = fmt.Errorf("error copying script to target: %v", err)
			return
		}
	}
	err = copyDependenciesToTarget(myTarget, dependenciesToCopy, localTempDir, targetTempDirectory, failIfDependencyNotFound)
	if err != nil {
		return
	}
	installedLkms, err = installLkmsOnTarget(myTarget, lkmsToInstall)
	if err != nil {
		return
	}
	return
}

// installLkmsOnTarget installs the specified LKMs on the target.
func installLkmsOnTarget(myTarget target.Target, lkmsToInstall map[string]int) (installedLkms []string, err error) {
	// install lkms on target
	var lkms []string
	for lkm := range lkmsToInstall {
		lkms = append(lkms, lkm)
	}
	if len(lkmsToInstall) > 0 {
		installedLkms, err = myTarget.InstallLkms(lkms)
		if err != nil {
			err = fmt.Errorf("error installing LKMs: %v", err)
			return
		}
	}
	return
}

// copyDependenciesToTarget copies the specified dependencies to the target.
func copyDependenciesToTarget(myTarget target.Target, dependenciesToCopy map[string]int, localTempDir string, targetTempDirectory string, failIfDependencyNotFound bool) (err error) {
	// copy dependencies to target
	for dependency := range dependenciesToCopy {
		var localDependencyPath string
		// first look for the dependency in the "tools" directory
		appDir := util.GetAppDir()
		if util.FileOrDirectoryExists(path.Join(appDir, "tools", dependency)) {
			localDependencyPath = path.Join(appDir, "tools", dependency)
		} else { // not found in the tools directory
			// extract the resource into the target's local temp directory
			targetLocalTempDir := path.Join(localTempDir, myTarget.GetName())
			localDependencyPath, err = util.ExtractResource(Resources, path.Join("resources", dependency), targetLocalTempDir)
			if err != nil {
				if failIfDependencyNotFound {
					err = fmt.Errorf("error extracting dependency. Dependency: %s, Error: %v", dependency, err)
					return
				}
				slog.Warn("dependency not found", slog.String("dependency", dependency))
				err = nil
				continue
			}
		}
		// copy dependency to target
		err = myTarget.PushFile(localDependencyPath, targetTempDirectory)
		if err != nil {
			err = fmt.Errorf("error copying dependency to target: %v", err)
			return
		}
	}
	return
}
