package target

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"perfspect/internal/util"
	"strings"
)

// SetSudo (LocalTarget only) sets the sudo password for the target.
// Also sets the canElevate field to 0 to indicate that the sudo password has not been verified.
func (t *LocalTarget) SetSudo(sudo string) {
	t.sudo = sudo
	t.canElevate = 0
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

// RunCommandStream runs the given command asynchronously on the target.
// It sends the command to the cmdChannel and executes it with a timeout.
// The output from the command is sent to the stdoutChannel and stderrChannel,
// and the exit code is sent to the exitcodeChannel.
// The timeout parameter specifies the maximum time allowed for the command to run.
// Returns an error if there was a problem running the command.
func (t *LocalTarget) RunCommandStream(cmd *exec.Cmd, timeout int, argNotUsed bool, stdoutChannel chan string, stderrChannel chan string, exitcodeChannel chan int, cmdChannel chan *exec.Cmd) (err error) {
	localCommand := cmd
	cmdChannel <- localCommand
	err = runLocalCommandWithInputWithTimeoutAsync(localCommand, stdoutChannel, stderrChannel, exitcodeChannel, "", timeout)
	return
}

func (t *LocalTarget) GetArchitecture() (string, error) {
	var err error
	if t.arch == "" {
		t.arch, err = getArchitecture(t)
	}
	return t.arch, err
}

func (t *LocalTarget) GetFamily() (string, error) {
	var err error
	if t.family == "" {
		t.family, err = getFamily(t)
	}
	return t.family, err
}

func (t *LocalTarget) GetModel() (string, error) {
	var err error
	if t.model == "" {
		t.model, err = getModel(t)
	}
	return t.model, err
}

func (t *LocalTarget) GetStepping() (string, error) {
	var err error
	if t.stepping == "" {
		t.stepping, err = getStepping(t)
	}
	return t.stepping, err
}

func (t *LocalTarget) GetVendor() (string, error) {
	var err error
	if t.vendor == "" {
		t.vendor, err = getVendor(t)
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

func (t *LocalTarget) GetTempDirectory() string {
	return t.tempDir
}

// PushFile copies a file or directory from the source path to the destination path.
// If the source path points to a directory, it creates the corresponding directory
// at the destination and recursively copies its contents. If the source path points
// to a file, it directly copies the file to the destination.
//
// Parameters:
//   - srcPath: The path to the source file or directory to be copied.
//   - dstPath: The destination path where the file or directory should be copied.
//
// Returns:
//   - err: An error if the operation fails, or nil if the operation succeeds.
func (t *LocalTarget) PushFile(srcPath string, dstPath string) (err error) {
	srcFileStat, err := os.Stat(srcPath)
	if err != nil {
		return
	}
	if srcFileStat.IsDir() {
		newDstDir := filepath.Join(dstPath, filepath.Base(srcPath))
		err = util.CreateDirectoryIfNotExists(newDstDir, 0700)
		if err != nil {
			return
		}
		err = util.CopyDirectory(srcPath, newDstDir)
		return
	}
	err = util.CopyFile(srcPath, dstPath)
	return
}

// PullFile copies a file from the source path on the local target to the destination directory.
// This function currently calls PushFile, which may not align with the intended behavior.
//
// Parameters:
//   - srcPath: The path to the source file to be pulled.
//   - dstDir: The destination directory where the file should be placed.
//
// Returns:
//   - An error if the operation fails.
func (t *LocalTarget) PullFile(srcPath string, dstDir string) error {
	return t.PushFile(srcPath, dstDir)
}

// CreateDirectory creates a new directory under the specified base directory.
// It returns the full path of the created directory and any error encountered.
func (t *LocalTarget) CreateDirectory(baseDir string, targetDir string) (dir string, err error) {
	dir = filepath.Join(baseDir, targetDir)
	err = os.Mkdir(dir, 0700)
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

// CanConnect checks if the local target can establish a connection (always true).
func (t *LocalTarget) CanConnect() bool {
	return true
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

// IsSuperUser checks if the current user is a superuser.
// It returns true if the user is a superuser, false otherwise.
func (t *LocalTarget) IsSuperUser() bool {
	return os.Geteuid() == 0
}

// InstallLkms installs the specified LKMs (Loadable Kernel Modules) on the target.
// It returns the list of installed LKMs and any error encountered during the installation process.
func (t *LocalTarget) InstallLkms(lkms []string) (installedLkms []string, err error) {
	return installLkms(t, lkms)
}

// UninstallLkms uninstalls the specified LKMs (Loadable Kernel Modules) from the target.
// It takes a slice of strings representing the names of the LKMs to be uninstalled.
// It returns an error if any error occurs during the uninstallation process.
func (t *LocalTarget) UninstallLkms(lkms []string) (err error) {
	return uninstallLkms(t, lkms)
}

// GetName returns the name of the Target.
func (t *LocalTarget) GetName() (host string) {
	return t.host
}

// GetUserPath returns the user's PATH environment variable after verifying that it only contains valid paths.
// It checks each path in the PATH environment variable and filters out any non-path strings.
// The function returns the verified paths joined by ":" as a string.
func (t *LocalTarget) GetUserPath() (string, error) {
	if t.userPath == "" {
		// get user's PATH environment variable, verify that it only contains paths (mitigate risk raised by Checkmarx)
		var verifiedPaths []string
		pathEnv := os.Getenv("PATH")
		for p := range strings.SplitSeq(pathEnv, ":") {
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
