// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// Package script provides functions to run scripts on a target and get the output.
package script

import (
	"embed"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"text/template"

	"perfspect/internal/progress"
	"perfspect/internal/target"
	"perfspect/internal/util"
)

//go:embed resources
var Resources embed.FS

const ControllerPIDFileName = "controller.pid"

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
	timeout := 0 // no timeout
	// We run controller in a new process group so that tty/terminal signals, e.g., Ctrl-C, are not sent to the command. This is
	// necessary to allow the controller script to handle signals itself and propagate them to all child scripts as needed. The
	// signal handler in perfspect will send the signal to the controller.sh script on each target so that it can clean up
	// its child processes.
	newProcessGroup := true
	reuseSSHConnection := false // don't reuse ssh connection on long-running commands, makes it difficult to kill the command
	stdout, stderr, exitcode, err := myTarget.RunCommandEx(cmd, timeout, newProcessGroup, reuseSSHConnection)
	if err != nil {
		slog.Error("failed to execute controller script on target", slog.String("stdout", stdout), slog.String("stderr", stderr), slog.Int("exitcode", exitcode), slog.String("error", err.Error()))
		return nil, err
	}
	if exitcode != 0 {
		slog.Error("controller script returned non-zero exit code", slog.String("stdout", stdout), slog.String("stderr", stderr), slog.Int("exitcode", exitcode))
		return nil, fmt.Errorf("controller script returned exit code %d", exitcode)
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
	// Name is kept for readable summary output.
	type tplScript struct {
		Name      string
		Sanitized string
	}
	// tplData holds all data passed into the controller script template.
	tplData := struct {
		TargetTempDir         string
		ControllerPIDFile     string
		ConcurrentScripts     []tplScript
		SequentialScripts     []tplScript
		ContinueOnScriptError bool
	}{}
	// populate tplData
	tplData.TargetTempDir = targetTempDirectory
	tplData.ControllerPIDFile = ControllerPIDFileName
	tplData.ContinueOnScriptError = continueOnScriptError
	needsElevated := false
	for _, s := range concurrentScripts {
		if s.Superuser {
			needsElevated = true
		}
		tplData.ConcurrentScripts = append(tplData.ConcurrentScripts, tplScript{
			Name: s.Name, Sanitized: sanitizeScriptName(s.Name),
		})
	}
	for _, s := range sequentialScripts {
		if s.Superuser {
			needsElevated = true
		}
		tplData.SequentialScripts = append(tplData.SequentialScripts, tplScript{
			Name: s.Name, Sanitized: sanitizeScriptName(s.Name),
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
current_seq_pid=""
current_seq_script=""

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
{{ end }}
{{- range .SequentialScripts}}
sequential_scripts+=({{ .Sanitized }})
orig_names[{{ .Sanitized }}]="{{ .Name }}"
{{ end }}

start_concurrent_scripts() {
  for s in "${concurrent_scripts[@]}"; do
    setsid bash "$script_dir/${s}.sh" > "$script_dir/${s}.stdout" 2> "$script_dir/${s}.stderr" &
    pids[$s]=$!
  done
}

run_sequential_scripts() {
  for s in "${sequential_scripts[@]}"; do
    current_seq_pid=""
    current_seq_script="$s"
    setsid bash "$script_dir/${s}.sh" > "$script_dir/${s}.stdout" 2> "$script_dir/${s}.stderr" &
    current_seq_pid=$!
    pids[$s]=$current_seq_pid
    if wait "$current_seq_pid"; then
      exitcodes[$s]=0
    else
      ec=$?
      exitcodes[$s]=$ec
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
  [[ -z "$pid" ]] && return 0
  if ! ps -p "$pid" > /dev/null 2>&1; then return 0; fi
  # Send signal to the process group (negative PID)
  kill -SIGINT -"$pid" 2>/dev/null || true
  # Give it time to clean up so child script can finalize
  # Wait up to 5 seconds in 0.5s intervals
  local waited=0
  while ps -p "$pid" > /dev/null 2>&1 && [ "$waited" -lt 10 ]; do
    sleep 0.5
    waited=$((waited + 1))
  done
  # Force kill the process group if still alive
  if ps -p "$pid" > /dev/null 2>&1; then
    kill -SIGKILL -"$pid" 2>/dev/null || true
  fi
  wait "$pid" 2>/dev/null || true
  if [[ -z "${exitcodes[$s]:-}" ]]; then exitcodes[$s]=130; fi
}

wait_for_concurrent_scripts() {
  for s in "${concurrent_scripts[@]}"; do
    if wait "${pids[$s]}"; then
      exitcodes[$s]=0
    else
      ec=$?
      exitcodes[$s]=$ec
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
