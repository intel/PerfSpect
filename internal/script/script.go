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
	"slices"
	"strconv"
	"strings"

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
	if !scriptForTarget(script, myTarget) {
		err := fmt.Errorf("the \"%s\" script is not intended for the target processor", script.Name)
		return ScriptOutput{}, err
	}
	scriptOutputs, err := RunScripts(myTarget, []ScriptDefinition{script}, false, localTempDir)
	scriptOutput := scriptOutputs[script.Name]
	return scriptOutput, err
}

// RunScripts runs a list of scripts on a target and returns the outputs of each script as a map with the script name as the key.
func RunScripts(myTarget target.Target, scripts []ScriptDefinition, ignoreScriptErrors bool, localTempDir string) (map[string]ScriptOutput, error) {
	// drop scripts that should not be run and separate scripts that must run sequentially from those that can be run in parallel
	canElevate := myTarget.CanElevatePrivileges()
	var sequentialScripts []ScriptDefinition
	var parallelScripts []ScriptDefinition
	for _, script := range scripts {
		if !scriptForTarget(script, myTarget) {
			slog.Debug("skipping script because it is not intended to run on the target processor", slog.String("target", myTarget.GetName()), slog.String("script", script.Name))
			continue
		}
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
		masterScript, needsElevatedPrivileges := formMasterScript(myTarget.GetTempDirectory(), parallelScripts)
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
		stdout, stderr, exitcode, err := myTarget.RunCommand(cmd, 0, false) // don't reuse ssh connection on long-running commands, makes it difficult to kill the command
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
		stdout, stderr, exitcode, err := myTarget.RunCommand(cmd, 0, false)
		if err != nil {
			slog.Error("error running script on target", slog.String("script", script.ScriptTemplate), slog.String("stdout", stdout), slog.String("stderr", stderr), slog.Int("exitcode", exitcode), slog.String("error", err.Error()))
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
func RunScriptStream(myTarget target.Target, script ScriptDefinition, localTempDir string, stdoutChannel chan string, stderrChannel chan string, exitcodeChannel chan int, errorChannel chan error, cmdChannel chan *exec.Cmd) {
	targetArchitecture, err := myTarget.GetArchitecture()
	if err != nil {
		err = fmt.Errorf("error getting target architecture: %v", err)
		errorChannel <- err
		return
	}
	if len(script.Architectures) > 0 && !slices.Contains(script.Architectures, targetArchitecture) {
		err = fmt.Errorf("skipping script because it is not meant for this architecture: %s", targetArchitecture)
		errorChannel <- err
		return
	}
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
	err = myTarget.RunCommandStream(cmd, 0, false, stdoutChannel, stderrChannel, exitcodeChannel, cmdChannel)
	errorChannel <- err
}

// scriptForTarget checks if the script is intended for the target processor.
func scriptForTarget(script ScriptDefinition, myTarget target.Target) bool {
	if len(script.Architectures) > 0 {
		architecture, err := myTarget.GetArchitecture()
		if err != nil {
			slog.Error("failed to get architecture for target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
			return false
		}
		if !slices.Contains(script.Architectures, architecture) {
			return false
		}
	}
	if len(script.Vendors) > 0 {
		vendor, err := myTarget.GetVendor()
		if err != nil {
			slog.Error("failed to get vendor for target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
			return false
		}
		if !slices.Contains(script.Vendors, vendor) {
			return false
		}
	}
	if len(script.Families) > 0 {
		family, err := myTarget.GetFamily()
		if err != nil {
			slog.Error("failed to get family for target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
			return false
		}
		if !slices.Contains(script.Families, family) {
			return false
		}
	}
	if len(script.Models) > 0 {
		model, err := myTarget.GetModel()
		if err != nil {
			slog.Error("failed to get model for target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
			return false
		}
		if !slices.Contains(script.Models, model) {
			return false
		}
	}
	return true
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
func formMasterScript(targetTempDirectory string, parallelScripts []ScriptDefinition) (string, bool) {
	// we write the stdout and stderr from each command to temporary files and save the PID of each command
	// in a variable named after the script
	var masterScript strings.Builder

	masterScript.WriteString("#!/bin/bash\n")

	// set dir var and change working directory to dir in case any of the scripts write out temporary files
	masterScript.WriteString(fmt.Sprintf("script_dir=%s\n", targetTempDirectory))
	masterScript.WriteString("cd $script_dir\n")

	// function to print the output of each script
	masterScript.WriteString("\nprint_output() {\n")
	for _, script := range parallelScripts {
		masterScript.WriteString("\techo \"<---------------------->\"\n")
		masterScript.WriteString(fmt.Sprintf("\techo SCRIPT NAME: %s\n", script.Name))
		masterScript.WriteString(fmt.Sprintf("\techo STDOUT:\n\tcat %s\n", path.Join("$script_dir", sanitizeScriptName(script.Name)+".stdout")))
		masterScript.WriteString(fmt.Sprintf("\techo STDERR:\n\tcat %s\n", path.Join("$script_dir", sanitizeScriptName(script.Name)+".stderr")))
		masterScript.WriteString(fmt.Sprintf("\techo EXIT CODE: $%s_exitcode\n", sanitizeScriptName(script.Name)))
	}
	masterScript.WriteString("}\n")

	// function to handle SIGINT
	masterScript.WriteString("\nhandle_sigint() {\n")
	for _, script := range parallelScripts {
		// send SIGINT to the child script, if it is still running
		masterScript.WriteString(fmt.Sprintf("\tif ps -p \"$%s_pid\" > /dev/null; then\n", sanitizeScriptName(script.Name)))
		masterScript.WriteString(fmt.Sprintf("\t\tkill -SIGINT $%s_pid\n", sanitizeScriptName(script.Name)))
		masterScript.WriteString("\tfi\n")
		if script.NeedsKill { // this is primarily used for scripts that start commands in the background, some of which (processwatch) doesn't respond to SIGINT as expected
			// if the *cmd.pid file exists, check if the process is still running
			masterScript.WriteString(fmt.Sprintf("\tif [ -f %s_cmd.pid ]; then\n", sanitizeScriptName(script.Name)))
			masterScript.WriteString(fmt.Sprintf("\t\tif ps -p $(cat %s_cmd.pid) > /dev/null; then\n", sanitizeScriptName(script.Name)))
			// send SIGINT to the background process first, then SIGKILL if it doesn't respond to SIGINT
			masterScript.WriteString(fmt.Sprintf("\t\t\tkill -SIGINT $(cat %s_cmd.pid)\n", sanitizeScriptName(script.Name)))
			// give the process a chance to respond to SIGINT
			masterScript.WriteString("\t\t\tsleep 0.5\n")
			// if the background process is still running, send SIGKILL
			masterScript.WriteString(fmt.Sprintf("\t\t\tif ps -p $(cat %s_cmd.pid) > /dev/null; then\n", sanitizeScriptName(script.Name)))
			masterScript.WriteString(fmt.Sprintf("\t\t\t\tkill -SIGKILL $(cat %s_cmd.pid)\n", sanitizeScriptName(script.Name)))
			masterScript.WriteString(fmt.Sprintf("\t\t\t\t%s_exitcode=137\n", sanitizeScriptName(script.Name))) // 137 is the exit code for SIGKILL
			masterScript.WriteString("\t\t\telse\n")
			// if the background process has exited, set the exit code to 0
			masterScript.WriteString(fmt.Sprintf("\t\t\t\t%s_exitcode=0\n", sanitizeScriptName(script.Name)))
			masterScript.WriteString("\t\t\tfi\n")
			masterScript.WriteString("\t\telse\n")
			// if the script itself has exited, set the exit code to 0
			masterScript.WriteString(fmt.Sprintf("\t\t\t%s_exitcode=0\n", sanitizeScriptName(script.Name)))
			masterScript.WriteString("\t\tfi\n")
			masterScript.WriteString("\telse\n")
			// if the *cmd.pid file doesn't exist, set the exit code to 1
			masterScript.WriteString(fmt.Sprintf("\t\t%s_exitcode=0\n", sanitizeScriptName(script.Name)))
			masterScript.WriteString("\tfi\n")
		} else {
			masterScript.WriteString(fmt.Sprintf("\twait \"$%s_pid\"\n", sanitizeScriptName(script.Name)))
			masterScript.WriteString(fmt.Sprintf("\t%s_exitcode=$?\n", sanitizeScriptName(script.Name)))
		}
	}
	masterScript.WriteString("\tprint_output\n")
	masterScript.WriteString("\texit 0\n")
	masterScript.WriteString("}\n")

	// call handle_sigint func when SIGINT is received
	masterScript.WriteString("\ntrap handle_sigint SIGINT\n")

	// run all parallel scripts in the background
	masterScript.WriteString("\n")
	needsElevatedPrivileges := false
	for _, script := range parallelScripts {
		if script.Superuser {
			needsElevatedPrivileges = true
		}
		masterScript.WriteString(
			fmt.Sprintf("bash %s > %s 2>%s &\n",
				path.Join("$script_dir", scriptNameToFilename(script.Name)),
				path.Join("$script_dir", sanitizeScriptName(script.Name)+".stdout"),
				path.Join("$script_dir", sanitizeScriptName(script.Name)+".stderr"),
			),
		)
		masterScript.WriteString(fmt.Sprintf("%s_pid=$!\n", sanitizeScriptName(script.Name)))
	}

	// wait for all parallel scripts to finish then print their output
	masterScript.WriteString("\n")
	for _, script := range parallelScripts {
		masterScript.WriteString(fmt.Sprintf("wait \"$%s_pid\"\n", sanitizeScriptName(script.Name)))
		masterScript.WriteString(fmt.Sprintf("%s_exitcode=$?\n", sanitizeScriptName(script.Name)))
	}
	masterScript.WriteString("\nprint_output\n")

	return masterScript.String(), needsElevatedPrivileges
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
