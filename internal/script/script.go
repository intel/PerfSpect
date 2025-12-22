// Package script provides functions to run scripts on a target and get the output.
package script

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

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

type ScriptOutput struct {
	ScriptDefinition
	Stdout   string
	Stderr   string
	Exitcode int
}

// RunScript runs a script on the specified target and returns the output.
func RunScript(myTarget target.Target, script ScriptDefinition, localTempDir string) (ScriptOutput, error) {
	scriptOutputs, err := RunScripts(myTarget, []ScriptDefinition{script}, false, localTempDir, nil, "")
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
func RunScripts(myTarget target.Target, scripts []ScriptDefinition, ignoreScriptErrors bool, localTempDir string, statusUpdate progress.MultiSpinnerUpdateFunc, collectingStatus string) (map[string]ScriptOutput, error) {
	// drop scripts that should not be run and separate scripts that must run sequentially from those that can be run in parallel
	canElevate := myTarget.CanElevatePrivileges()
	var sequentialScripts []ScriptDefinition
	var parallelScripts []ScriptDefinition
	for _, script := range scripts {
		if script.Superuser && !canElevate {
			slog.Debug("skipping script because it requires superuser privileges and the user cannot elevate privileges on target", slog.String("script", script.Name))
			continue
		}
		if script.Sequential {
			sequentialScripts = append(sequentialScripts, script)
		} else {
			parallelScripts = append(parallelScripts, script)
		}
	}
	// prepare target to run scripts by copying scripts and dependencies to target and installing LKMs
	if statusUpdate != nil {
		_ = statusUpdate(myTarget.GetName(), "preparing to collect data")
	}
	installedLkms, err := prepareTargetToRunScripts(myTarget, append(sequentialScripts, parallelScripts...), localTempDir, false)
	if err != nil {
		err = fmt.Errorf("error while preparing target to run scripts: %v", err)
		return nil, err
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
	// if there's only 1 parallel script, run it sequentially
	if len(parallelScripts) == 1 {
		slog.Debug("running single parallel script sequentially", slog.String("script", parallelScripts[0].Name))
		sequentialScripts = append(sequentialScripts, parallelScripts...)
		parallelScripts = nil
	}
	scriptOutputs := make(map[string]ScriptOutput)
	// run parallel scripts
	if len(parallelScripts) > 0 {
		// form one master script that calls all the parallel scripts in the background
		masterScriptName := "parallel_master.sh"
		masterScript, needsElevatedPrivileges, err := formMasterScript(myTarget.GetTempDirectory(), parallelScripts)
		if err != nil {
			err = fmt.Errorf("error forming master script: %v", err)
			return nil, err
		}
		// write master script to local file
		masterScriptPath := path.Join(localTempDir, myTarget.GetName(), masterScriptName)
		err = os.WriteFile(masterScriptPath, []byte(masterScript), 0600)
		if err != nil {
			err = fmt.Errorf("error writing master script to local file: %v", err)
			return nil, err
		}
		// copy master script to target
		err = myTarget.PushFile(masterScriptPath, myTarget.GetTempDirectory())
		if err != nil {
			err = fmt.Errorf("error copying script to target: %v", err)
			return nil, err
		}
		// run master script on target
		// if the master script requires elevated privileges, we run it with sudo
		// Note: adding 'sudo' to the individual scripts inside the master script
		// instigates a known bug in the terminal that corrupts the tty settings:
		// https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=1043320
		var cmd *exec.Cmd
		if needsElevatedPrivileges && !canElevate {
			// this shouldn't happen because we already filtered out the scripts that require elevated privileges if the user cannot elevate privileges on the target
			err = fmt.Errorf("master script requires elevated privileges but the user cannot elevate privileges on target")
			return nil, err
		} else if needsElevatedPrivileges && !myTarget.IsSuperUser() {
			// run master script with sudo, "-S" to read password from stdin. Note: password won't be asked for if password-less sudo is configured.
			cmd = exec.Command("sudo", "-S", "bash", path.Join(myTarget.GetTempDirectory(), masterScriptName)) // #nosec G204
		} else {
			cmd = exec.Command("bash", path.Join(myTarget.GetTempDirectory(), masterScriptName)) // #nosec G204
		}
		timeout := 0 // no timeout
		// We run parallel_master in a new process group so that tty/terminal signals, e.g., Ctrl-C, are not sent to the command. This is
		// necessary to allow the master script to handle signals itself and propagate them to the child scripts as needed. The
		// signal handler in perfspect will send the signal to the parallel_master.sh script on each target so that it can clean up
		// its child processes.
		newProcessGroup := true
		reuseSSHConnection := false // don't reuse ssh connection on long-running commands, makes it difficult to kill the command
		stdout, stderr, exitcode, err := myTarget.RunCommandEx(cmd, timeout, newProcessGroup, reuseSSHConnection)
		if err != nil {
			slog.Error("error running master script on target", slog.String("stdout", stdout), slog.String("stderr", stderr), slog.Int("exitcode", exitcode), slog.String("error", err.Error()))
			return nil, err
		}
		// parse output of master script
		parallelScriptOutputs := parseMasterScriptOutput(stdout)
		for _, scriptOutput := range parallelScriptOutputs {
			// find associated parallel script
			scriptIdx := -1
			for i, script := range parallelScripts {
				if script.Name == scriptOutput.Name {
					scriptIdx = i
					break
				}
			}
			scriptOutput.ScriptTemplate = parallelScripts[scriptIdx].ScriptTemplate
			scriptOutputs[scriptOutput.Name] = scriptOutput
		}
	}
	// run sequential scripts
	for _, script := range sequentialScripts {
		var cmd *exec.Cmd
		scriptPath := path.Join(myTarget.GetTempDirectory(), scriptNameToFilename(script.Name))
		if script.Superuser && !canElevate {
			// this shouldn't happen because we already filtered out the scripts that require elevated privileges if the user cannot elevate privileges on the target
			err = fmt.Errorf("script requires elevated privileges but the user cannot elevate privileges on target")
			return nil, err
		} else if script.Superuser && !myTarget.IsSuperUser() {
			// run script with sudo, "-S" to read password from stdin. Note: password won't be asked for if password-less sudo is configured.
			cmd = exec.Command("sudo", "-S", "bash", scriptPath) // #nosec G204
		} else {
			cmd = exec.Command("bash", scriptPath) // #nosec G204
		}
		// if the script is tagged with NeedsKill, we run it in a new process group so that tty/terminal signals, e.g., Ctrl-C, are not sent to the command. This is
		// necessary to allow the script to handle signals itself and clean up as needed. The
		// signal handler in perfspect will send the signal to the script on each target so that it can clean up
		// as needed.
		newProcessGroup := script.NeedsKill
		reuseSSHConnection := false // don't reuse ssh connection on long-running commands, makes it difficult to kill the command
		stdout, stderr, exitcode, err := myTarget.RunCommandEx(cmd, 0, newProcessGroup, reuseSSHConnection)
		if err != nil {
			slog.Warn("error running script on target", slog.String("name", script.Name), slog.String("stdout", stdout), slog.String("stderr", stderr), slog.Int("exitcode", exitcode), slog.String("error", err.Error()))
		}
		scriptOutputs[script.Name] = ScriptOutput{ScriptDefinition: script, Stdout: stdout, Stderr: stderr, Exitcode: exitcode}
		if !ignoreScriptErrors {
			return scriptOutputs, err
		}
		err = nil
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
	cmd := prepareCommand(script, myTarget.GetTempDirectory())
	err = myTarget.RunCommandStream(cmd, stdoutChannel, stderrChannel, exitcodeChannel, cmdChannel)
	errorChannel <- err
}

func prepareCommand(script ScriptDefinition, targetTempDirectory string) (cmd *exec.Cmd) {
	scriptPath := path.Join(targetTempDirectory, scriptNameToFilename(script.Name))
	if script.Superuser {
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

// formMasterScript forms a master script that runs all parallel scripts in the background, waits for them to finish, then prints the output of each script.
// Return values are the master script and a boolean indicating whether the master script requires elevated privileges.
func formMasterScript(targetTempDirectory string, parallelScripts []ScriptDefinition) (string, bool, error) {
	// data model for template
	type tplScript struct {
		Name      string
		Sanitized string
		NeedsKill bool
		Superuser bool
	}
	data := struct {
		TargetTempDir string
		Scripts       []tplScript
	}{}
	data.TargetTempDir = targetTempDirectory
	needsElevated := false
	for _, s := range parallelScripts {
		if s.Superuser {
			needsElevated = true
		}
		data.Scripts = append(data.Scripts, tplScript{
			Name: s.Name, Sanitized: sanitizeScriptName(s.Name), NeedsKill: s.NeedsKill, Superuser: s.Superuser,
		})
	}
	const masterScriptTemplate = `#!/usr/bin/env bash
set -o errexit
set -o pipefail

script_dir={{.TargetTempDir}}
cd "$script_dir"

# write our pid to a file so that perfspect can send us a signal if needed
echo $$ > primary_collection_script.pid

declare -a scripts=()
declare -A needs_kill=()
declare -A pids=()
declare -A exitcodes=()
declare -A orig_names=()

{{- range .Scripts}}
scripts+=({{ .Sanitized }})
needs_kill[{{ .Sanitized }}]={{ if .NeedsKill }}1{{ else }}0{{ end }}
orig_names[{{ .Sanitized }}]="{{ .Name }}"
{{ end }}

start_scripts() {
  for s in "${scripts[@]}"; do
    bash "$script_dir/${s}.sh" > "$script_dir/${s}.stdout" 2> "$script_dir/${s}.stderr" &
    pids[$s]=$!
  done
}

kill_script() {
  local s="$1"
  local pid="${pids[$s]:-}"
  [[ -z "$pid" ]] && return 0
  if ! ps -p "$pid" > /dev/null 2>&1; then return 0; fi
  if [[ "${needs_kill[$s]}" == "1" && -f "${s}_cmd.pid" ]]; then
    local bgpid
    bgpid="$(cat "${s}_cmd.pid" 2>/dev/null || true)"
    if [[ -n "$bgpid" && $(ps -p "$bgpid" -o pid= 2>/dev/null) ]]; then
      kill -SIGINT "$bgpid" 2>/dev/null || true
      sleep 0.5
      if ps -p "$bgpid" > /dev/null 2>&1; then
        kill -SIGKILL "$bgpid" 2>/dev/null || true
        exitcodes[$s]=137
      else
        exitcodes[$s]=0
      fi
    fi
  else
    kill -SIGINT "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    if [[ -z "${exitcodes[$s]:-}" ]]; then exitcodes[$s]=130; fi
  fi
}

wait_for_scripts() {
  for s in "${scripts[@]}"; do
    if wait "${pids[$s]}"; then
      exitcodes[$s]=0
    else
      ec=$?
      exitcodes[$s]=$ec
    fi
  done
}

print_summary() {
  for s in "${scripts[@]}"; do
    echo "<---------------------->"
    echo "SCRIPT NAME: ${orig_names[$s]}"
    echo "STDOUT:"; cat "$script_dir/${s}.stdout" || true
    echo "STDERR:"; cat "$script_dir/${s}.stderr" || true
    echo "EXIT CODE: ${exitcodes[$s]:-1}"
  done
}

handle_sigint() {
  echo "Received SIGINT; attempting graceful shutdown" >&2
  for s in "${scripts[@]}"; do
    kill_script "$s"
  done
  print_summary
  rm -f primary_collection_script.pid
  exit 0
}

trap handle_sigint SIGINT

start_scripts
wait_for_scripts
print_summary
rm -f primary_collection_script.pid
`
	tmpl, err := template.New("master").Parse(masterScriptTemplate)
	if err != nil {
		slog.Error("failed to parse master script template", slog.String("error", err.Error()))
		return "", needsElevated, err
	}
	var out strings.Builder
	if err = tmpl.Execute(&out, data); err != nil {
		slog.Error("failed to execute master script template", slog.String("error", err.Error()))
		return "", needsElevated, err
	}
	return out.String(), needsElevated, nil
}

// parseMasterScriptOutput parses the output of the master script that runs all parallel scripts in the background.
// It returns a list of ScriptOutput objects, one for each script that was run.
func parseMasterScriptOutput(masterScriptOutput string) (scriptOutputs []ScriptOutput) {
	// split output of master script into individual script outputs
	for output := range strings.SplitSeq(masterScriptOutput, "<---------------------->\n") {
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
