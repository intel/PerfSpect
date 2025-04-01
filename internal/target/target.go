/*
Package target provides a way to interact with local and remote systems.
*/
package target

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"perfspect/internal/util"
)

// Target represents a machine or system where commands can be run.
// Implementations of this interface should provide methods to run
// commands, check connectivity, elevate privileges, and other operations
// that depend on the specific type of target (e.g., local or remote).
type Target interface {
	// CanConnect checks if a connection can be established with the target.
	// It returns true if a connection can be established, false otherwise.
	CanConnect() bool

	// CanElevatePrivileges checks if the current user can elevate privileges.
	// It returns true if the user can elevate privileges, false otherwise.
	CanElevatePrivileges() bool

	// IsSuperUser checks if the current user is a superuser.
	// It returns true if the user is a superuser, false otherwise.
	IsSuperUser() bool

	// GetArchitecture returns the architecture of the target system.
	// It returns a string representing the architecture and any error that occurred.
	GetArchitecture() (arch string, err error)

	// GetFamily returns the family of the target system's CPU.
	// It returns a string representing the family and any error that occurred.
	GetFamily() (family string, err error)

	// GetModel returns the model of the target system's CPU.
	// It returns a string representing the model and any error that occurred.
	GetModel() (model string, err error)

	// GetStepping returns the stepping of the target system's CPU.
	// It returns a string representing the stepping and any error that occurred.
	GetStepping() (stepping string, err error)

	// GetVendor returns the vendor of the target system.
	// It returns a string representing the vendor and any error that occurred.
	GetVendor() (vendor string, err error)

	// GetName returns the name of the target system.
	// It returns a string representing the host.
	GetName() (name string)

	// GetUserPath returns the path of the current user on the target system.
	// It returns a string representing the path and any error that occurred.
	GetUserPath() (path string, err error)

	// RunCommand runs the specified command on the target.
	// Arguments:
	// - cmd: the command to run
	// - timeout: the maximum time allowed for the command to run (zero means no timeout)
	// - reuseSSHConnection: whether to reuse the SSH connection for the command (only relevant for RemoteTarget)
	// It returns the standard output, standard error, exit code, and any error that occurred.
	RunCommand(cmd *exec.Cmd, timeout int, reuseSSHConnection bool) (stdout string, stderr string, exitCode int, err error)

	// RunCommandAsync runs the specified command on the target in an asynchronous manner.
	// Arguments:
	// - cmd: the command to run
	// - timeout: the maximum time allowed for the command to run (zero means no timeout)
	// - reuseSSHConnection: whether to reuse the SSH connection for the command (only relevant for RemoteTarget)
	// - stdoutChannel: a channel to send the standard output of the command
	// - stderrChannel: a channel to send the standard error of the command
	// - exitcodeChannel: a channel to send the exit code of the command
	// - cmdChannel: a channel to send the command that was run
	// It returns any error that occurred.
	RunCommandAsync(cmd *exec.Cmd, timeout int, reuseSSHConnection bool, stdoutChannel chan string, stderrChannel chan string, exitcodeChannel chan int, cmdChannel chan *exec.Cmd) error

	// PushFile transfers a file from the local system to the target.
	// It returns any error that occurred.
	PushFile(srcPath string, dstPath string) error

	// PullFile transfers a file from the target to the local system.
	// It returns any error that occurred.
	PullFile(srcPath string, dstDir string) error

	// CreateDirectory creates a directory on the target at the specified path with the specified permissions.
	// It returns the path of the created directory and any error that occurred.
	CreateDirectory(baseDir string, targetDir string) (dir string, err error)

	// CreateTempDirectory creates a temporary directory on the target with the specified prefix.
	// It returns the path of the created directory and any error that occurred.
	CreateTempDirectory(rootDir string) (tempDir string, err error)

	// GetTempDirectory returns the path of the temporary directory on the target. It will be
	// empty if the temporary directory has not been created yet.
	GetTempDirectory() string

	// RemoveTempDirectory removes the temporary directory on the target.
	// It returns any error that occurred.
	RemoveTempDirectory() error

	// RemoveDirectory removes a directory from the target at the specified path.
	// It returns any error that occurred.
	RemoveDirectory(targetDir string) error

	// InstallLkms installs the specified Linux Kernel Modules (LKMs) on the target.
	// It returns a list of installed LKMs and any error that occurred.
	InstallLkms(lkms []string) (installedLkms []string, err error)

	// UninstallLkms uninstalls the specified Linux Kernel Modules (LKMs) from the target.
	// It returns any error that occurred.
	UninstallLkms(lkms []string) error
}

type LocalTarget struct {
	host       string
	sudo       string
	tempDir    string
	arch       string
	family     string
	model      string
	stepping   string
	userPath   string
	canElevate int // zero indicates unknown, 1 indicates yes, -1 indicates no
	vendor     string
}

type RemoteTarget struct {
	name        string
	host        string
	port        string
	user        string
	key         string
	sshPass     string
	sshpassPath string
	tempDir     string
	arch        string
	family      string
	model       string
	stepping    string
	userPath    string
	canElevate  int
	vendor      string
}

// NewLocalTarget creates a new LocalTarget
func NewLocalTarget() *LocalTarget {
	hostName, err := os.Hostname()
	if err != nil {
		hostName = "localhost"
	}
	t := &LocalTarget{
		host: hostName,
	}
	return t
}

// NewRemoteTarget creates a new RemoteTarget instance with the provided parameters.
// It initializes the RemoteTarget struct and returns a pointer to it.
func NewRemoteTarget(name string, host string, port string, user string, key string) *RemoteTarget {
	t := &RemoteTarget{
		name: name,
		host: host,
		port: port,
		user: user,
		key:  key,
	}
	return t
}

// SetSudo sets the sudo password for the target (LocalTarget only).
// Also sets the canElevate field to 0 to indicate that the sudo password has not been verified.
func (t *LocalTarget) SetSudo(sudo string) {
	t.sudo = sudo
	t.canElevate = 0
}

// SetSshPassPath sets the path to the sshpass binary (RemoteTarget only).
func (t *RemoteTarget) SetSshPassPath(sshpassPath string) {
	t.sshpassPath = sshpassPath
}

// SetSshPass sets the ssh password for the target (RemoteTarget only).
func (t *RemoteTarget) SetSshPass(sshPass string) {
	t.sshPass = sshPass
}

// RunCommand executes the given command with a timeout and returns the standard output,
// standard error, exit code, and any error that occurred.
func (t *LocalTarget) RunCommand(cmd *exec.Cmd, timeout int, argNotUsed bool) (stdout string, stderr string, exitCode int, err error) {
	input := ""
	if t.sudo != "" && len(cmd.Args) > 2 && cmd.Args[0] == "sudo" && strings.HasPrefix(cmd.Args[1], "-") && strings.Contains(cmd.Args[1], "S") { // 'sudo -S' gets password from stdin
		input = t.sudo + "\n"
	}
	return runLocalCommandWithInputWithTimeout(cmd, input, timeout)
}

func (t *RemoteTarget) RunCommand(cmd *exec.Cmd, timeout int, reuseSSHConnection bool) (stdout string, stderr string, exitCode int, err error) {
	localCommand := t.prepareLocalCommand(cmd, reuseSSHConnection)
	return runLocalCommandWithInputWithTimeout(localCommand, "", timeout)
}

// RunCommandAsync runs the given command asynchronously on the target.
// It sends the command to the cmdChannel and executes it with a timeout.
// The output from the command is sent to the stdoutChannel and stderrChannel,
// and the exit code is sent to the exitcodeChannel.
// The timeout parameter specifies the maximum time allowed for the command to run.
// Returns an error if there was a problem running the command.
func (t *LocalTarget) RunCommandAsync(cmd *exec.Cmd, timeout int, argNotUsed bool, stdoutChannel chan string, stderrChannel chan string, exitcodeChannel chan int, cmdChannel chan *exec.Cmd) (err error) {
	localCommand := cmd
	cmdChannel <- localCommand
	err = runLocalCommandWithInputWithTimeoutAsync(localCommand, stdoutChannel, stderrChannel, exitcodeChannel, "", timeout)
	return
}

func (t *RemoteTarget) RunCommandAsync(cmd *exec.Cmd, timeout int, reuseSSHConnection bool, stdoutChannel chan string, stderrChannel chan string, exitcodeChannel chan int, cmdChannel chan *exec.Cmd) (err error) {
	localCommand := t.prepareLocalCommand(cmd, reuseSSHConnection)
	cmdChannel <- localCommand
	err = runLocalCommandWithInputWithTimeoutAsync(localCommand, stdoutChannel, stderrChannel, exitcodeChannel, "", timeout)
	return
}

// GetArchitecture returns the architecture of the target.
// It retrieves the architecture by calling the getArchitecture function.
func (t *LocalTarget) GetArchitecture() (arch string, err error) {
	if t.arch == "" {
		t.arch, err = getArchitecture(t)
	}
	return t.arch, err
}

func (t *RemoteTarget) GetArchitecture() (arch string, err error) {
	if t.arch == "" {
		t.arch, err = getArchitecture(t)
	}
	return t.arch, err
}

func (t *LocalTarget) GetFamily() (family string, err error) {
	if t.family == "" {
		t.family, err = getFamily(t)
	}
	return t.family, err
}

func (t *RemoteTarget) GetFamily() (family string, err error) {
	if t.family == "" {
		t.family, err = getFamily(t)
	}
	return t.family, err
}

func (t *LocalTarget) GetModel() (family string, err error) {
	if t.model == "" {
		t.model, err = getModel(t)
	}
	return t.model, err
}

func (t *RemoteTarget) GetModel() (family string, err error) {
	if t.model == "" {
		t.model, err = getModel(t)
	}
	return t.model, err
}

func (t *LocalTarget) GetStepping() (stepping string, err error) {
	if t.stepping == "" {
		t.stepping, err = getStepping(t)
	}
	return t.stepping, err
}

func (t *RemoteTarget) GetStepping() (stepping string, err error) {
	if t.stepping == "" {
		t.stepping, err = getStepping(t)
	}
	return t.stepping, err
}

// GetVendor returns the vendor of the target.
// It retrieves the vendor by calling the GetVendor function.
func (t *LocalTarget) GetVendor() (arch string, err error) {
	if t.vendor == "" {
		t.vendor, err = GetVendor(t)
	}
	return t.vendor, err
}

func (t *RemoteTarget) GetVendor() (arch string, err error) {
	if t.vendor == "" {
		t.vendor, err = GetVendor(t)
	}
	return t.vendor, err
}

// CreateTempDirectory creates a temporary directory under the specified root directory.
// If the root directory is not specified, the temporary directory will be created in the current directory.
// It returns the path of the created temporary directory and any error encountered.
func (t *LocalTarget) CreateTempDirectory(rootDir string) (tempDir string, err error) {
	if t.tempDir != "" {
		return t.tempDir, nil
	}
	temp, err := os.MkdirTemp(rootDir, "perfspect.tmp.")
	if err != nil {
		return
	}
	tempDir, err = util.AbsPath(temp)
	if err != nil {
		return
	}
	t.tempDir = tempDir
	return
}

func (t *RemoteTarget) CreateTempDirectory(rootDir string) (tempDir string, err error) {
	if t.tempDir != "" {
		return t.tempDir, nil
	}
	var root string
	if rootDir != "" {
		root = fmt.Sprintf("--tmpdir=%s", rootDir)
	}
	cmd := exec.Command("mktemp", "-d", "-t", root, "perfspect.tmp.XXXXXXXXXX", "|", "xargs", "realpath")
	tempDir, _, _, err = t.RunCommand(cmd, 0, true)
	if err != nil {
		return
	}
	tempDir = strings.TrimSpace(tempDir)
	t.tempDir = tempDir
	return
}

// RemoveTempDirectory removes the temporary directory created by CreateTempDirectory.
func (t *LocalTarget) RemoveTempDirectory() (err error) {
	if t.tempDir != "" {
		err = t.RemoveDirectory(t.tempDir)
		if err == nil {
			t.tempDir = ""
		}
	}
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

func (t *LocalTarget) GetTempDirectory() string {
	return t.tempDir
}

func (t *RemoteTarget) GetTempDirectory() string {
	return t.tempDir
}

// PushFile copies a file or directory from the source path to the destination path on the target.
// If the destination path is a directory, the file will be copied with the same name to that directory.
// If the destination path is a file, the file will be copied and overwritten.
// The file permissions of the source file will be preserved in the destination file.
func (t *LocalTarget) PushFile(srcPath string, dstPath string) (err error) {
	srcFileStat, err := os.Stat(srcPath)
	if err != nil {
		return
	}
	if srcFileStat.IsDir() {
		newDstDir := filepath.Join(dstPath, filepath.Base(srcPath))
		err = util.CreateDirectoryIfNotExists(newDstDir, 0755)
		if err != nil {
			return
		}
		err = util.CopyDirectory(srcPath, newDstDir)
		return
	}
	err = util.CopyFile(srcPath, dstPath)
	return
}

func (t *RemoteTarget) PushFile(srcPath string, dstDir string) error {
	stdout, stderr, exitCode, err := t.prepareAndRunSCPCommand(srcPath, dstDir, true)
	slog.Debug("push file", slog.String("srcPath", srcPath), slog.String("dstDir", dstDir), slog.String("stdout", stdout), slog.String("stderr", stderr), slog.Int("exitCode", exitCode))
	return err
}

// PullFile pulls a file from the target's source path to the destination directory.
// It is a convenience method that internally calls the PushFile method.
func (t *LocalTarget) PullFile(srcPath string, dstDir string) error {
	return t.PushFile(srcPath, dstDir)
}

func (t *RemoteTarget) PullFile(srcPath string, dstDir string) error {
	stdout, stderr, exitCode, err := t.prepareAndRunSCPCommand(srcPath, dstDir, false)
	slog.Debug("pull file", slog.String("srcPath", srcPath), slog.String("dstDir", dstDir), slog.String("stdout", stdout), slog.String("stderr", stderr), slog.Int("exitCode", exitCode))
	return err
}

// CreateDirectory creates a new directory under the specified base directory.
// It returns the full path of the created directory and any error encountered.
func (t *LocalTarget) CreateDirectory(baseDir string, targetDir string) (dir string, err error) {
	dir = filepath.Join(baseDir, targetDir)
	err = os.Mkdir(dir, 0764)
	return
}

func (t *RemoteTarget) CreateDirectory(baseDir string, targetDir string) (dir string, err error) {
	dir = filepath.Join(baseDir, targetDir)
	cmd := exec.Command("mkdir", dir)
	_, _, _, err = t.RunCommand(cmd, 0, true)
	return
}

// RemoveDirectory removes the specified target directory.
// If the target directory is not empty, it will be deleted along with all its contents.
// The method returns an error if any error occurs during the removal process.
func (t *LocalTarget) RemoveDirectory(targetDir string) (err error) {
	if targetDir != "" {
		err = os.RemoveAll(targetDir)
	}
	return
}

func (t *RemoteTarget) RemoveDirectory(targetDir string) (err error) {
	if targetDir != "" {
		cmd := exec.Command("rm", "-rf", targetDir)
		_, _, _, err = t.RunCommand(cmd, 0, true)
	}
	return
}

// CanConnect checks if the local target can establish a connection.
func (t *LocalTarget) CanConnect() bool {
	return true
}

func (t *RemoteTarget) CanConnect() bool {
	cmd := exec.Command("exit", "0")
	_, _, _, err := t.RunCommand(cmd, 5, true)
	return err == nil
}

// CanElevatePrivileges (on LocalTarget) checks if the user is root or sudo can be used to elevate privileges.
// It returns true if the user is root or if the sudo password works.
// If the `sudo` command is configured, it will attempt to run a command with sudo
// and check if the password works. If the passwordless sudo is configured,
// it will also check if passwordless sudo works.
// Returns true if the user can elevate privileges, false otherwise.
func (t *LocalTarget) CanElevatePrivileges() bool {
	if t.canElevate != 0 {
		return t.canElevate == 1
	}
	if t.IsSuperUser() {
		t.canElevate = 1
		return true // user is root
	}
	if t.sudo != "" {
		cmd := exec.Command("sudo", "-kS", "ls")
		stdin, _ := cmd.StdinPipe()
		go func() {
			defer stdin.Close()
			_, err := io.WriteString(stdin, t.sudo+"\n")
			if err != nil {
				slog.Error("error writing sudo password", slog.String("error", err.Error()))
			}
		}()
		_, _, _, err := t.RunCommand(cmd, 0, true)
		if err == nil {
			t.canElevate = 1
			return true // sudo password works
		}
	}
	cmd := exec.Command("sudo", "-kS", "ls")
	_, _, _, err := t.RunCommand(cmd, 0, true)
	if err == nil { // true - passwordless sudo works
		t.canElevate = 1
		return true
	}
	t.canElevate = -1
	return false
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
	_, _, _, err := t.RunCommand(cmd, 0, true)
	if err == nil { // true - passwordless sudo works
		t.canElevate = 1
		return true
	}
	t.canElevate = -1
	return false
}

// IsSuperUser checks if the current user is a superuser.
// It returns true if the user is a superuser, false otherwise.
func (t *LocalTarget) IsSuperUser() bool {
	return os.Geteuid() == 0
}

func (t *RemoteTarget) IsSuperUser() bool {
	return t.user == "root"
}

// InstallLkms installs the specified LKMs (Loadable Kernel Modules) on the target.
// It returns the list of installed LKMs and any error encountered during the installation process.
func (t *LocalTarget) InstallLkms(lkms []string) (installedLkms []string, err error) {
	return installLkms(t, lkms)
}

func (t *RemoteTarget) InstallLkms(lkms []string) (installedLkms []string, err error) {
	return installLkms(t, lkms)
}

// UninstallLkms uninstalls the specified LKMs (Loadable Kernel Modules) from the target.
// It takes a slice of strings representing the names of the LKMs to be uninstalled.
// It returns an error if any error occurs during the uninstallation process.
func (t *LocalTarget) UninstallLkms(lkms []string) (err error) {
	return uninstallLkms(t, lkms)
}

func (t *RemoteTarget) UninstallLkms(lkms []string) (err error) {
	return uninstallLkms(t, lkms)
}

// GetName returns the name of the Target.
func (t *LocalTarget) GetName() (host string) {
	return t.host
}

func (t *RemoteTarget) GetName() (host string) {
	if t.name == "" {
		return t.host
	}
	return t.name
}

// GetUserPath returns the user's PATH environment variable after verifying that it only contains valid paths.
// It checks each path in the PATH environment variable and filters out any non-path strings.
// The function returns the verified paths joined by ":" as a string.
func (t *LocalTarget) GetUserPath() (string, error) {
	if t.userPath == "" {
		// get user's PATH environment variable, verify that it only contains paths (mitigate risk raised by Checkmarx)
		var verifiedPaths []string
		pathEnv := os.Getenv("PATH")
		pathEnvPaths := strings.SplitSeq(pathEnv, ":")
		for p := range pathEnvPaths {
			files, err := filepath.Glob(p)
			// Goal is to filter out any non path strings
			// Glob will throw an error on pattern mismatch and return no files if no files
			if err == nil && len(files) > 0 {
				verifiedPaths = append(verifiedPaths, p)
			}
		}
		t.userPath = strings.Join(verifiedPaths, ":")
	}
	return t.userPath, nil
}

func (t *RemoteTarget) GetUserPath() (string, error) {
	if t.userPath == "" {
		cmd := exec.Command("echo", "$PATH")
		stdout, _, _, err := t.RunCommand(cmd, 0, true)
		if err != nil {
			return "", err
		}
		t.userPath = strings.TrimSpace(stdout)
	}
	return t.userPath, nil
}

// helpers below

func runLocalCommandWithInputWithTimeout(cmd *exec.Cmd, input string, timeout int) (stdout string, stderr string, exitCode int, err error) {
	logInput := ""
	if input != "" {
		logInput = "******"
	}
	slog.Debug("running local command", slog.String("cmd", cmd.String()), slog.String("input", logInput), slog.Int("timeout", timeout))
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()
		commandWithContext := exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
		commandWithContext.Env = cmd.Env
		cmd = commandWithContext
	}
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	var outbuf, errbuf strings.Builder
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf
	err = cmd.Run()
	stdout = outbuf.String()
	stderr = errbuf.String()
	if err != nil {
		exitError := &exec.ExitError{}
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		}
	}
	return
}

// TODO: does timeout make sense with async functions?
func runLocalCommandWithInputWithTimeoutAsync(cmd *exec.Cmd, stdoutChannel chan string, stderrChannel chan string, exitcodeChannel chan int, input string, timeout int) (err error) {
	logInput := ""
	if input != "" {
		logInput = "******"
	}
	slog.Debug("running local command (async)", slog.String("cmd", cmd.String()), slog.String("input", logInput), slog.Int("timeout", timeout))
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()
		commandWithContext := exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
		commandWithContext.Env = cmd.Env
		cmd = commandWithContext
	}
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	stdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		err = fmt.Errorf("failed to get stdout pipe: %v", err)
		return
	}
	stdoutScanner := bufio.NewScanner(stdoutReader)
	stderrReader, err := cmd.StderrPipe()
	if err != nil {
		err = fmt.Errorf("failed to get stderr pipe: %v", err)
		return
	}
	stderrScanner := bufio.NewScanner(stderrReader)
	if err = cmd.Start(); err != nil {
		err = fmt.Errorf("failed to run command (%s): %v", cmd, err)
		return
	}
	go func() {
		for stdoutScanner.Scan() {
			text := stdoutScanner.Text()
			stdoutChannel <- text
		}
	}()
	go func() {
		for stderrScanner.Scan() {
			text := stderrScanner.Text()
			stderrChannel <- text
		}
	}()
	err = cmd.Wait()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitcodeChannel <- exitError.ExitCode()
		} else {
			panic(fmt.Sprintf("err from cmd.Wait is not type exec.ExitError: %v", err))
		}
	} else {
		exitcodeChannel <- 0
	}
	return nil
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
	localCommand := exec.Command(name, args...)
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
	localCommand := exec.Command(name, args...)
	if usePass {
		localCommand.Env = append(localCommand.Env, "SSHPASS="+t.sshPass)
	}
	stdout, stderr, exitCode, err = runLocalCommandWithInputWithTimeout(localCommand, "", 0)
	return
}

func getArchitecture(t Target) (arch string, err error) {
	cmd := exec.Command("uname", "-m")
	arch, _, _, err = t.RunCommand(cmd, 0, true)
	if err != nil {
		return
	}
	arch = strings.TrimSpace(arch)
	return
}

func getFamily(t Target) (family string, err error) {
	cmd := exec.Command("bash", "-c", "lscpu | grep -i \"^CPU family:\" | awk '{print $NF}'")
	family, _, _, err = t.RunCommand(cmd, 0, true)
	if err != nil {
		return
	}
	family = strings.TrimSpace(family)
	return
}

func getModel(t Target) (model string, err error) {
	cmd := exec.Command("bash", "-c", "lscpu | grep -i model: | awk '{print $NF}'")
	model, _, _, err = t.RunCommand(cmd, 0, true)
	if err != nil {
		return
	}
	model = strings.TrimSpace(model)
	return
}

func getStepping(t Target) (stepping string, err error) {
	cmd := exec.Command("bash", "-c", "lscpu | grep -i stepping: | awk '{print $NF}'")
	stepping, _, _, err = t.RunCommand(cmd, 0, true)
	if err != nil {
		return
	}
	stepping = strings.TrimSpace(stepping)
	return
}

func GetVendor(t Target) (vendor string, err error) {
	cmd := exec.Command("bash", "-c", "lscpu | grep -i \"^Vendor ID:\" | awk '{print $NF}'")
	vendor, _, _, err = t.RunCommand(cmd, 0, true)
	if err != nil {
		return
	}
	vendor = strings.TrimSpace(vendor)
	return
}

func installLkms(t Target, lkms []string) (installedLkms []string, err error) {
	if !t.CanElevatePrivileges() {
		err = fmt.Errorf("can't elevate privileges; elevated privileges required to install lkms")
		return
	}
	for _, lkm := range lkms {
		slog.Debug("attempting to install kernel module", slog.String("lkm", lkm))
		_, _, _, err := t.RunCommand(exec.Command("modprobe", "--first-time", lkm), 10, true)
		if err != nil {
			slog.Debug("kernel module already installed or problem installing", slog.String("lkm", lkm), slog.String("error", err.Error()))
			continue
		}
		slog.Debug("kernel module installed", slog.String("lkm", lkm))
		installedLkms = append(installedLkms, lkm)
	}
	return
}

func uninstallLkms(t Target, lkms []string) (err error) {
	if !t.CanElevatePrivileges() {
		err = fmt.Errorf("can't elevate privileges; elevated privileges required to uninstall lkms")
		return
	}
	for _, lkm := range lkms {
		slog.Debug("attempting to uninstall kernel module", slog.String("lkm", lkm))
		_, _, _, err := t.RunCommand(exec.Command("modprobe", "-r", lkm), 10, true)
		if err != nil {
			slog.Error("error uninstalling kernel module", slog.String("lkm", lkm), slog.String("error", err.Error()))
			continue
		}
		slog.Debug("kernel module uninstalled", slog.String("lkm", lkm))
	}
	return
}
