/*
Package target provides a way to interact with local and remote systems.
*/
package target

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"os"
	"os/exec"
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

	// RunCommandStream runs the specified command on the target and streams the output to the provided channels.
	// Arguments:
	// - cmd: the command to run
	// - timeout: the maximum time allowed for the command to run (zero means no timeout)
	// - reuseSSHConnection: whether to reuse the SSH connection for the command (only relevant for RemoteTarget)
	// - stdoutChannel: a channel to send the standard output of the command
	// - stderrChannel: a channel to send the standard error of the command
	// - exitcodeChannel: a channel to send the exit code of the command
	// - cmdChannel: a channel to send the command that was run
	// It returns any error that occurred.
	RunCommandStream(cmd *exec.Cmd, timeout int, reuseSSHConnection bool, stdoutChannel chan string, stderrChannel chan string, exitcodeChannel chan int, cmdChannel chan *exec.Cmd) error

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

type BaseTarget struct {
	tempDir    string
	canElevate int // zero indicates unknown, 1 indicates yes, -1 indicates no
	arch       string
	family     string
	model      string
	stepping   string
	vendor     string
	userPath   string
}

type LocalTarget struct {
	BaseTarget
	host string
	sudo string
}

type RemoteTarget struct {
	BaseTarget
	name        string
	host        string
	port        string
	user        string
	key         string
	sshPass     string
	sshpassPath string
}

// NewLocalTarget creates a new LocalTarget.
// It initializes the host name to the local machine's hostname.
// If the hostname cannot be retrieved, it defaults to "localhost".
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
