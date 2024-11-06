// Package script provides functions to run scripts on a target and get the output.
package script

// Copyright (C) 2021-2024 Intel Corporation
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

	"perfspect/internal/target"
	"perfspect/internal/util"
)

//go:embed resources
var Resources embed.FS

type ScriptDefinition struct {
	Name          string   // just a name
	Script        string   // the bash script that will be run
	Architectures []string // architectures, i.e., x86_64, arm64. If empty, it will run on all architectures.
	Families      []string // families, e.g., 6, 7. If empty, it will run on all families.
	Models        []string // models, e.g., 62, 63. If empty, it will run on all models.
	Lkms          []string // loadable kernel modules
	Depends       []string // binary dependencies that must be available for the script to run
	Superuser     bool     // requires sudo or root
	Sequential    bool     // run script sequentially (not at the same time as others)
	Timeout       int      // seconds
}

type ScriptOutput struct {
	ScriptDefinition
	Stdout   string
	Stderr   string
	Exitcode int
}

// RunScript runs a script on the specified target and returns the output.
func RunScript(myTarget target.Target, script ScriptDefinition, localTempDir string) (scriptOutput ScriptOutput, err error) {
	targetArchitecture, err := myTarget.GetArchitecture()
	if err != nil {
		err = fmt.Errorf("error getting target architecture: %v", err)
		return
	}
	targetFamily, err := myTarget.GetFamily()
	if err != nil {
		err = fmt.Errorf("error getting target family: %v", err)
		return
	}
	targetModel, err := myTarget.GetModel()
	if err != nil {
		err = fmt.Errorf("error getting target model: %v", err)
		return
	}
	if len(script.Architectures) > 0 && !util.StringInList(targetArchitecture, script.Architectures) ||
		len(script.Families) > 0 && !util.StringInList(targetFamily, script.Families) ||
		len(script.Models) > 0 && !util.StringInList(targetModel, script.Models) {
		err = fmt.Errorf("\"%s\" script is not intended for the target processor", script.Name)
		return
	}
	scriptOutputs, err := RunScripts(myTarget, []ScriptDefinition{script}, false, localTempDir)
	scriptOutput = scriptOutputs[script.Name]
	return
}

// RunScripts runs a list of scripts on a target and returns the outputs of each script as a map with the script name as the key.
func RunScripts(myTarget target.Target, scripts []ScriptDefinition, ignoreScriptErrors bool, localTempDir string) (map[string]ScriptOutput, error) {
	// need a unique temp directory for each target to avoid race conditions
	localTempDirForTarget := path.Join(localTempDir, myTarget.GetName())
	// if the directory doesn't exist, create it
	if _, err := os.Stat(localTempDirForTarget); os.IsNotExist(err) {
		if err := os.Mkdir(localTempDirForTarget, 0755); err != nil {
			err = fmt.Errorf("error creating directory for target: %v", err)
			return nil, err
		}
	}
	targetArchitecture, err := myTarget.GetArchitecture()
	if err != nil {
		err = fmt.Errorf("error getting target architecture: %v", err)
		return nil, err
	}
	targetFamily, err := myTarget.GetFamily()
	if err != nil {
		err = fmt.Errorf("error getting target family: %v", err)
		return nil, err
	}
	targetModel, err := myTarget.GetModel()
	if err != nil {
		err = fmt.Errorf("error getting target model: %v", err)
		return nil, err
	}
	// drop scripts that should not be run and separate scripts that must run sequentially from those that can be run in parallel
	canElevate := myTarget.CanElevatePrivileges()
	var sequentialScripts []ScriptDefinition
	var parallelScripts []ScriptDefinition
	for _, script := range scripts {
		if len(script.Architectures) > 0 && !util.StringInList(targetArchitecture, script.Architectures) ||
			len(script.Families) > 0 && !util.StringInList(targetFamily, script.Families) ||
			len(script.Models) > 0 && !util.StringInList(targetModel, script.Models) {
			slog.Info("skipping script because it is not intended to run on the target processor", slog.String("target", myTarget.GetName()), slog.String("script", script.Name), slog.String("targetArchitecture", targetArchitecture), slog.String("targetFamily", targetFamily), slog.String("targetModel", targetModel))
			continue
		}
		if script.Superuser && !canElevate {
			slog.Info("skipping script because it requires superuser privileges and the target cannot elevate privileges", slog.String("script", script.Name))
			continue
		}
		if script.Sequential {
			sequentialScripts = append(sequentialScripts, script)
		} else {
			parallelScripts = append(parallelScripts, script)
		}
	}

	// prepare target to run scripts by copying scripts and dependencies to target and installing LKMs
	installedLkms, err := prepareTargetToRunScripts(myTarget, append(sequentialScripts, parallelScripts...), localTempDirForTarget, false)
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
		masterScript, needsElevatedPrivileges := formMasterScript(myTarget, parallelScripts)
		// write master script to local file
		masterScriptPath := path.Join(localTempDirForTarget, masterScriptName)
		err = os.WriteFile(masterScriptPath, []byte(masterScript), 0644)
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
		if needsElevatedPrivileges {
			// run master script with sudo, "-S" to read password from stdin
			cmd = exec.Command("sudo", "-S", "bash", path.Join(myTarget.GetTempDirectory(), masterScriptName))
		} else {
			cmd = exec.Command("bash", path.Join(myTarget.GetTempDirectory(), masterScriptName))
		}
		stdout, stderr, exitcode, err := myTarget.RunCommand(cmd, 0)
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
			scriptOutput.Script = parallelScripts[scriptIdx].Script
			scriptOutputs[scriptOutput.Name] = scriptOutput
		}
	}
	// run sequential scripts
	for _, script := range sequentialScripts {
		cmd := prepareCommand(script, myTarget.GetTempDirectory())
		stdout, stderr, exitcode, err := myTarget.RunCommand(cmd, 0)
		if err != nil {
			slog.Error("error running script on target", slog.String("script", script.Script), slog.String("stdout", stdout), slog.String("stderr", stderr), slog.Int("exitcode", exitcode), slog.String("error", err.Error()))
		}
		scriptOutputs[script.Name] = ScriptOutput{ScriptDefinition: script, Stdout: stdout, Stderr: stderr, Exitcode: exitcode}
		if !ignoreScriptErrors {
			return scriptOutputs, err
		}
		err = nil
	}
	return scriptOutputs, nil
}

// RunScriptAsync runs a script on the specified target and returns the output. It is meant to be called
// in a go routine.
func RunScriptAsync(myTarget target.Target, script ScriptDefinition, localTempDir string, stdoutChannel chan string, stderrChannel chan string, exitcodeChannel chan int, errorChannel chan error, cmdChannel chan *exec.Cmd) {
	// need a unique temp directory for each target to avoid race conditions when there are multiple targets
	localTempDirForTarget := path.Join(localTempDir, myTarget.GetName())
	// if the directory doesn't exist, create it
	// we try a few times because there could multiple go routines trying to create the same directory
	maxTries := 3
	for i := 0; i < maxTries; i++ {
		if _, err := os.Stat(localTempDirForTarget); os.IsNotExist(err) {
			if err := os.Mkdir(localTempDirForTarget, 0755); err != nil {
				if i == maxTries-1 {
					err = fmt.Errorf("error creating local temp directory for target: %v", err)
					errorChannel <- err
					return
				}
			}
		}
	}
	targetArchitecture, err := myTarget.GetArchitecture()
	if err != nil {
		err = fmt.Errorf("error getting target architecture: %v", err)
		errorChannel <- err
		return
	}
	if len(script.Architectures) > 0 && !util.StringInList(targetArchitecture, script.Architectures) {
		err = fmt.Errorf("skipping script because it is not meant for this architecture: %s", targetArchitecture)
		errorChannel <- err
		return
	}
	installedLkms, err := prepareTargetToRunScripts(myTarget, []ScriptDefinition{script}, localTempDirForTarget, true)
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
	err = myTarget.RunCommandAsync(cmd, stdoutChannel, stderrChannel, exitcodeChannel, script.Timeout, cmdChannel)
	errorChannel <- err
}

func prepareCommand(script ScriptDefinition, targetTempDirectory string) (cmd *exec.Cmd) {
	scriptPath := path.Join(targetTempDirectory, scriptNameToFilename(script.Name))
	if script.Superuser {
		cmd = exec.Command("sudo", "bash", scriptPath)
	} else {
		cmd = exec.Command("bash", scriptPath)
	}
	return
}

func sanitizeScriptName(name string) string {
	sanitized := strings.ReplaceAll(name, " ", "_")
	sanitized = strings.ReplaceAll(sanitized, "-", "_")
	return sanitized
}

func scriptNameToFilename(name string) string {
	return sanitizeScriptName(name) + ".sh"
}

// formMasterScript forms a master script that runs all parallel scripts in the background, waits for them to finish, then prints the output of each script.
// Return values are the master script and a boolean indicating whether the master script requires elevated privileges.
func formMasterScript(myTarget target.Target, parallelScripts []ScriptDefinition) (string, bool) {
	// we write the stdout and stderr from each command to temporary files and save the PID of each command
	// in a variable named after the script
	var masterScript strings.Builder
	targetTempDirectory := myTarget.GetTempDirectory()
	masterScript.WriteString(fmt.Sprintf("script_dir=%s\n", targetTempDirectory))
	// change working directory to target temporary directory in case any of the scripts write out temporary files
	masterScript.WriteString(fmt.Sprintf("cd %s\n", targetTempDirectory))
	// the master script will run all parallel scripts in the background
	masterScript.WriteString("\n# run all scripts in the background\n")
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
	// the master script will wait for all parallel scripts to finish
	masterScript.WriteString("\n# wait for all scripts to finish\n")
	for _, script := range parallelScripts {
		masterScript.WriteString(fmt.Sprintf("wait \"$%s_pid\"\n", sanitizeScriptName(script.Name)))
		masterScript.WriteString(fmt.Sprintf("%s_exitcode=$?\n", sanitizeScriptName(script.Name)))
	}
	// the master script will print the output of each script
	masterScript.WriteString("\n# print output of each script\n")
	for _, script := range parallelScripts {
		masterScript.WriteString("echo \"<---------------------->\"\n")
		masterScript.WriteString(fmt.Sprintf("echo SCRIPT NAME: %s\n", script.Name))
		masterScript.WriteString(fmt.Sprintf("echo STDOUT:\ncat %s\n", path.Join("$script_dir", sanitizeScriptName(script.Name)+".stdout")))
		masterScript.WriteString(fmt.Sprintf("echo STDERR:\ncat %s\n", path.Join("$script_dir", sanitizeScriptName(script.Name)+".stderr")))
		masterScript.WriteString(fmt.Sprintf("echo EXIT CODE: $%s_exitcode\n", sanitizeScriptName(script.Name)))
	}
	return masterScript.String(), needsElevatedPrivileges
}

// parseMasterScriptOutput parses the output of the master script that runs all parallel scripts in the background.
// It returns a list of ScriptOutput objects, one for each script that was run.
func parseMasterScriptOutput(masterScriptOutput string) (scriptOutputs []ScriptOutput) {
	// split output of master script into individual script outputs
	outputs := strings.Split(masterScriptOutput, "<---------------------->\n")
	for _, output := range outputs {
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
			if strings.HasPrefix(line, "EXIT CODE:") {
				exitcode = strings.TrimSpace(strings.TrimPrefix(line, "EXIT CODE:"))
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
		exitCodeInt, err := strconv.Atoi(exitcode)
		if err != nil {
			slog.Error("error converting exit code to integer, setting to -100", slog.String("exitcode", exitcode), slog.String("error", err.Error()))
			exitCodeInt = -100
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
	lkmsToInstall := make(map[string]int)
	dependenciesToCopy := make(map[string]int)
	userPath, err := myTarget.GetUserPath()
	if err != nil {
		err = fmt.Errorf("error while retrieving user's path: %v", err)
		return
	}
	userPath = fmt.Sprintf("%s:%s", targetTempDirectory, userPath)

	targetArchitecture, err := myTarget.GetArchitecture()
	if err != nil {
		err = fmt.Errorf("error getting target architecture: %v", err)
		return
	}
	// for each script we will run
	for _, script := range scripts {
		// add lkms to list of lkms to install
		for _, lkm := range script.Lkms {
			lkmsToInstall[lkm] = 1
		}
		// add dependencies to list of dependencies to copy to target
		for _, dependency := range script.Depends {
			if len(script.Architectures) == 0 || util.StringInList(targetArchitecture, script.Architectures) {
				dependenciesToCopy[path.Join(targetArchitecture, dependency)] = 1
			}
		}
		// add user's path to script
		scriptWithPath := fmt.Sprintf("export PATH=\"%s\"\n%s", userPath, script.Script)
		if script.Name == "" {
			panic("script name cannot be empty")
		}
		scriptPath := path.Join(localTempDir, scriptNameToFilename(script.Name))
		// write script to local file
		err = os.WriteFile(scriptPath, []byte(scriptWithPath), 0644)
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
	// copy dependencies to target
	for dependency := range dependenciesToCopy {
		var localDependencyPath string
		// first look for the dependency in the "tools" directory
		appDir := util.GetAppDir()
		if util.Exists(path.Join(appDir, "tools", dependency)) {
			localDependencyPath = path.Join(appDir, "tools", dependency)
		} else { // not found in the tools directory, so extract it from resources
			localDependencyPath, err = util.ExtractResource(Resources, path.Join("resources", dependency), localTempDir)
			if err != nil {
				if failIfDependencyNotFound {
					err = fmt.Errorf("error extracting dependency: %v", err)
					return
				}
				slog.Warn("failed to extract dependency", slog.String("dependency", dependency), slog.String("error", err.Error()))
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
