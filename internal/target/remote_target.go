// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package target

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SetSshPassPath sets the path to the sshpass binary (RemoteTarget only).
func (t *RemoteTarget) SetSshPassPath(sshpassPath string) {
	t.sshpassPath = sshpassPath
}

// SetSshPass sets the ssh password for the target (RemoteTarget only).
func (t *RemoteTarget) SetSshPass(sshPass string) {
	t.sshPass = sshPass
}

// RunCommand executes a command on the remote target using SSH.
func (t *RemoteTarget) RunCommand(cmd *exec.Cmd) (stdout string, stderr string, exitCode int, err error) {
	localCommand := t.prepareLocalCommand(cmd, true)
	return runLocalCommandWithInputWithTimeout(localCommand, "", 0, false)
}

// RunCommandEx executes a command on the remote target using SSH with additional options.
func (t *RemoteTarget) RunCommandEx(cmd *exec.Cmd, timeout int, newProcessGroup bool, reuseSSHConnection bool) (stdout string, stderr string, exitCode int, err error) {
	localCommand := t.prepareLocalCommand(cmd, reuseSSHConnection)
	return runLocalCommandWithInputWithTimeout(localCommand, "", timeout, newProcessGroup)
}

// RunCommandStream executes a command asynchronously on a remote target.
// It prepares the local command based on the provided parameters and runs it
// with a specified timeout. The function communicates the command's output,
// error, and exit code through the provided channels.
//
// Parameters:
//   - cmd: The command to be executed, represented as an *exec.Cmd.
//   - stdoutChannel: A channel to send the standard output of the command.
//   - stderrChannel: A channel to send the standard error of the command.
//   - exitcodeChannel: A channel to send the exit code of the command.
//   - cmdChannel: A channel to send the prepared local command.
//
// Returns:
//   - err: An error object if the command fails to execute or times out.
func (t *RemoteTarget) RunCommandStream(cmd *exec.Cmd, stdoutChannel chan []byte, stderrChannel chan []byte, exitcodeChannel chan int, cmdChannel chan *exec.Cmd) (err error) {
	localCommand := t.prepareLocalCommand(cmd, false)
	cmdChannel <- localCommand
	err = runLocalCommandWithInputWithTimeoutAsync(localCommand, stdoutChannel, stderrChannel, exitcodeChannel, "", 0)
	return
}

func (t *RemoteTarget) GetArchitecture() (string, error) {
	var err error
	if t.arch == "" {
		t.arch, err = getArchitecture(t)
	}
	return t.arch, err
}

// CreateTempDirectory creates a temporary directory on the remote target.
// If a temporary directory has already been created, it returns the existing one.
// The function takes an optional rootDir parameter to specify the root directory
// for the temporary directory. If rootDir is provided, it is passed as an argument
// to the "mktemp" command to set the base directory for the temporary directory.
// The function executes the "mktemp" command to create the directory and resolves
// its absolute path using "realpath". The resulting directory path is cached in
// the RemoteTarget instance for reuse.
//
// Parameters:
//   - rootDir: An optional string specifying the root directory for the temporary directory.
//
// Returns:
//   - tempDir: The absolute path of the created temporary directory.
//   - err: An error if the temporary directory creation or command execution fails.
func (t *RemoteTarget) CreateTempDirectory(rootDir string) (tempDir string, err error) {
	if t.tempDir != "" {
		return t.tempDir, nil
	}
	var root string
	if rootDir != "" {
		root = fmt.Sprintf("--tmpdir=%s", rootDir)
	}
	cmd := exec.Command("mktemp", "-d", "-t", root, "perfspect.tmp.XXXXXXXXXX", "|", "xargs", "realpath") // #nosec G204
	tempDir, _, _, err = t.RunCommand(cmd)
	if err != nil {
		return
	}
	tempDir = strings.TrimSpace(tempDir)
	t.tempDir = tempDir
	return
}

func (t *RemoteTarget) RemoveTempDirectory() (err error) {
	if t.tempDir != "" {
		err = t.RemoveDirectory(t.tempDir)
		if err == nil {
			t.tempDir = ""
		}
	}
	return
}

// GetTempDirectory returns the path to the temporary directory associated with the RemoteTarget.
// This directory is used for storing temporary files during the target's operation.
func (t *RemoteTarget) GetTempDirectory() string {
	return t.tempDir
}

// PushFile transfers a file from the local system to a remote directory on the target.
// It uses SCP (Secure Copy Protocol) to perform the file transfer.
//
// Parameters:
//   - srcPath: The path to the source file on the local system.
//   - dstDir: The destination directory on the remote target.
//
// The function logs the operation details, including the source path, destination directory,
// standard output, standard error, and the exit code of the SCP command.
//
// Returns:
//   - An error if the file transfer fails, or nil if the operation is successful.
func (t *RemoteTarget) PushFile(srcPath string, dstDir string) error {
	stdout, stderr, exitCode, err := t.prepareAndRunSCPCommand(srcPath, dstDir, true)
	slog.Debug("push file", slog.String("srcPath", srcPath), slog.String("dstDir", dstDir), slog.String("stdout", stdout), slog.String("stderr", stderr), slog.Int("exitCode", exitCode))
	return err
}

// PullFile copies a file from a remote source path to a local destination directory
// using SCP (Secure Copy Protocol). It logs the operation details including the
// source path, destination directory, standard output, standard error, and exit code.
//
// Parameters:
//   - srcPath: The path to the file on the remote system to be copied.
//   - dstDir: The local directory where the file will be copied to.
//
// Returns:
//   - error: An error object if the operation fails, or nil if the operation succeeds.
func (t *RemoteTarget) PullFile(srcPath string, dstDir string) error {
	stdout, stderr, exitCode, err := t.prepareAndRunSCPCommand(srcPath, dstDir, false)
	slog.Debug("pull file", slog.String("srcPath", srcPath), slog.String("dstDir", dstDir), slog.String("stdout", stdout), slog.String("stderr", stderr), slog.Int("exitCode", exitCode))
	return err
}

func (t *RemoteTarget) CreateDirectory(baseDir string, targetDir string) (dir string, err error) {
	dir = filepath.Join(baseDir, targetDir)
	cmd := exec.Command("mkdir", dir)
	_, _, _, err = t.RunCommand(cmd)
	return
}

func (t *RemoteTarget) RemoveDirectory(targetDir string) (err error) {
	if targetDir != "" {
		cmd := exec.Command("rm", "-rf", targetDir)
		_, _, _, err = t.RunCommand(cmd)
	}
	return
}

// CanConnect checks if the target is reachable.
func (t *RemoteTarget) CanConnect() bool {
	cmd := exec.Command("exit", "0")
	_, _, _, err := t.RunCommand(cmd)
	return err == nil
}

// CanElevatePrivileges (on RemoteTarget) checks if the user name is root or if sudo can be used to elevate privileges.
// Note that the sudo password is not used for this check. Password-less sudo is required.
func (t *RemoteTarget) CanElevatePrivileges() bool {
	if t.canElevate != 0 {
		return t.canElevate == 1
	}
	if t.IsSuperUser() {
		t.canElevate = 1
		return true
	}
	cmd := exec.Command("sudo", "-kS", "ls")
	_, _, _, err := t.RunCommand(cmd)
	if err == nil { // true - passwordless sudo works
		t.canElevate = 1
		return true
	}
	t.canElevate = -1
	return false
}

func (t *RemoteTarget) IsSuperUser() bool {
	return t.user == "root"
}

func (t *RemoteTarget) InstallLkms(lkms []string) (installedLkms []string, err error) {
	return installLkms(t, lkms)
}

func (t *RemoteTarget) UninstallLkms(lkms []string) (err error) {
	return uninstallLkms(t, lkms)
}

func (t *RemoteTarget) GetName() (host string) {
	if t.name == "" {
		return t.host
	}
	return t.name
}

func (t *RemoteTarget) GetUserPath() (string, error) {
	if t.userPath == "" {
		cmd := exec.Command("echo", "$PATH")
		stdout, _, _, err := t.RunCommand(cmd)
		if err != nil {
			return "", err
		}
		t.userPath = strings.TrimSpace(stdout)
	}
	return t.userPath, nil
}

func (t *RemoteTarget) GetFamily() string {
	return t.family
}

func (t *RemoteTarget) SetFamily(family string) {
	t.family = family
}

func (t *RemoteTarget) GetModel() string {
	return t.model
}

func (t *RemoteTarget) SetModel(model string) {
	t.model = model
}

func (t *RemoteTarget) GetStepping() string {
	return t.stepping
}

func (t *RemoteTarget) SetStepping(stepping string) {
	t.stepping = stepping
}

func (t *RemoteTarget) GetVendor() string {
	return t.vendor
}

func (t *RemoteTarget) SetVendor(vendor string) {
	t.vendor = vendor
}

func (t *RemoteTarget) GetCapid4() string {
	return t.capid4
}

func (t *RemoteTarget) SetCapid4(capid4 string) {
	t.capid4 = capid4
}

func (t *RemoteTarget) GetDevices() string {
	return t.devices
}

func (t *RemoteTarget) SetDevices(devices string) {
	t.devices = devices
}

func (t *RemoteTarget) GetImplementer() string {
	return t.implementer
}

func (t *RemoteTarget) SetImplementer(implementer string) {
	t.implementer = implementer
}

func (t *RemoteTarget) GetPart() string {
	return t.part
}

func (t *RemoteTarget) SetPart(part string) {
	t.part = part
}

func (t *RemoteTarget) GetDmidecodePart() string {
	return t.dmidecodePart
}

func (t *RemoteTarget) SetDmidecodePart(dmidecodePart string) {
	t.dmidecodePart = dmidecodePart
}

func (t *RemoteTarget) GetMicroarchitecture() string {
	return t.microarchitecture
}

func (t *RemoteTarget) SetMicroarchitecture(microarchitecture string) {
	t.microarchitecture = microarchitecture
}

func (t *RemoteTarget) prepareSSHFlags(scp bool, useControlMaster bool, prompt bool) (flags []string) {
	flags = []string{
		"-2",
		"-o",
		"UserKnownHostsFile=/dev/null",
		"-o",
		"StrictHostKeyChecking=no",
		"-o",
		"ConnectTimeout=10",       // This one exposes a bug in Windows' SSH client. Each connection takes
		"-o",                      // 10 seconds to establish. https://github.com/PowerShell/Win32-OpenSSH/issues/1352
		"GSSAPIAuthentication=no", // This one is not supported, but is ignored on Windows.
		"-o",
		"ServerAliveInterval=30",
		"-o",
		"ServerAliveCountMax=10", // 30 * 10 = maximum 300 seconds before disconnect on no data
		"-o",
		"LogLevel=ERROR",
	}
	// turn on batch mode to avoid prompts for passwords
	if !prompt {
		promptFlags := []string{
			"-o",
			"BatchMode=yes",
		}
		flags = append(flags, promptFlags...)
	}
	// when using a control master, a long-running remote program doesn't get terminated when the local ssh client is terminated
	if useControlMaster {
		controlPathFlags := []string{
			"-o",
			"ControlPath=" + filepath.Join(os.TempDir(), fmt.Sprintf("control-%%h-%%p-%%r-%d", os.Getpid())),
			"-o",
			"ControlMaster=auto",
			"-o",
			"ControlPersist=1m",
		}
		flags = append(flags, controlPathFlags...)
	}
	if t.key != "" {
		keyFlags := []string{
			"-o",
			"PreferredAuthentications=publickey",
			"-o",
			"PasswordAuthentication=no",
			"-i",
			t.key,
		}
		flags = append(flags, keyFlags...)
	}
	if t.port != "" {
		if scp {
			flags = append(flags, "-P")
		} else {
			flags = append(flags, "-p")
		}
		flags = append(flags, t.port)
	}
	return
}

func (t *RemoteTarget) prepareSSHCommand(command []string, useControlMaster bool, prompt bool) []string {
	var cmd []string
	cmd = append(cmd, "ssh")
	cmd = append(cmd, t.prepareSSHFlags(false, useControlMaster, prompt)...)
	if t.user != "" {
		cmd = append(cmd, t.user+"@"+t.host)
	} else {
		cmd = append(cmd, t.host)
	}
	cmd = append(cmd, "--")
	cmd = append(cmd, command...)
	return cmd
}

func (t *RemoteTarget) prepareSCPCommand(src string, dstDir string, push bool) []string {
	var cmd []string
	cmd = append(cmd, "scp")
	cmd = append(cmd, t.prepareSSHFlags(true, true, false)...)
	if push {
		fileInfo, err := os.Stat(src)
		if err != nil {
			slog.Error("error getting file info", slog.String("src", src), slog.String("error", err.Error()))
			return nil
		}
		if fileInfo.IsDir() {
			cmd = append(cmd, "-r")
		}
		cmd = append(cmd, src)
		dst := t.host + ":" + dstDir
		if t.user != "" {
			dst = t.user + "@" + dst
		}
		cmd = append(cmd, dst)
	} else { // pull
		s := t.host + ":" + src
		if t.user != "" {
			s = t.user + "@" + s
		}
		cmd = append(cmd, s)
		cmd = append(cmd, dstDir)
	}
	return cmd
}

func (t *RemoteTarget) prepareLocalCommand(cmd *exec.Cmd, useControlMaster bool) *exec.Cmd {
	var name string
	var args []string
	usePass := t.key == "" && t.sshPass != ""
	sshCommand := t.prepareSSHCommand(cmd.Args, useControlMaster, usePass)
	if usePass {
		name = t.sshpassPath
		args = []string{"-e", "--"}
		args = append(args, sshCommand...)
	} else {
		name = sshCommand[0]
		args = sshCommand[1:]
	}
	localCommand := exec.Command(name, args...) // #nosec G204 // nosemgrep
	if usePass {
		localCommand.Env = append(localCommand.Env, "SSHPASS="+t.sshPass)
	}
	return localCommand
}

func (t *RemoteTarget) prepareAndRunSCPCommand(srcPath string, dstDir string, isPush bool) (stdout string, stderr string, exitCode int, err error) {
	scpCommand := t.prepareSCPCommand(srcPath, dstDir, isPush)
	var name string
	var args []string
	usePass := t.key == "" && t.sshPass != ""
	if usePass {
		name = t.sshpassPath
		args = append(args, "-e", "--")
		args = append(args, scpCommand...)
	} else {
		name = scpCommand[0]
		args = scpCommand[1:]
	}
	localCommand := exec.Command(name, args...) // #nosec G204 // nosemgrep
	if usePass {
		localCommand.Env = append(localCommand.Env, "SSHPASS="+t.sshPass)
	}
	stdout, stderr, exitCode, err = runLocalCommandWithInputWithTimeout(localCommand, "", 0, false)
	return
}
