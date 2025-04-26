package target

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import "os/exec"

// GetName returns the name of the RawTarget.
// It retrieves the value of the private field 'name'.
func (t *RawTarget) GetName() (name string) {
	return t.name
}

// RawTarget is used only when processing data from a ".raw" file.
// No other methods are implemented for this type.

func (t *RawTarget) CanConnect() bool {
	panic("not implemented")
}

func (t *RawTarget) CanElevatePrivileges() bool {
	panic("not implemented")
}

func (t *RawTarget) IsSuperUser() bool {
	panic("not implemented")
}

func (t *RawTarget) GetArchitecture() (arch string, err error) {
	panic("not implemented")
}

func (t *RawTarget) GetFamily() (family string, err error) {
	panic("not implemented")
}

func (t *RawTarget) GetModel() (model string, err error) {
	panic("not implemented")
}

func (t *RawTarget) GetStepping() (stepping string, err error) {
	panic("not implemented")
}

func (t *RawTarget) GetVendor() (vendor string, err error) {
	panic("not implemented")
}

func (t *RawTarget) GetUserPath() (path string, err error) {
	panic("not implemented")
}

func (t *RawTarget) RunCommand(cmd *exec.Cmd, timeout int, reuseSSHConnection bool) (stdout string, stderr string, exitCode int, err error) {
	panic("not implemented")
}

func (t *RawTarget) RunCommandAsync(cmd *exec.Cmd, timeout int, reuseSSHConnection bool, stdoutChannel chan string, stderrChannel chan string, exitcodeChannel chan int, cmdChannel chan *exec.Cmd) error {
	panic("not implemented")
}

func (t *RawTarget) PushFile(srcPath string, dstPath string) error {
	panic("not implemented")
}

func (t *RawTarget) PullFile(srcPath string, dstDir string) error {
	panic("not implemented")
}

func (t *RawTarget) CreateDirectory(baseDir string, targetDir string) (dir string, err error) {
	panic("not implemented")
}

func (t *RawTarget) CreateTempDirectory(rootDir string) (tempDir string, err error) {
	panic("not implemented")
}

func (t *RawTarget) GetTempDirectory() string {
	panic("not implemented")
}

func (t *RawTarget) RemoveTempDirectory() error {
	panic("not implemented")
}

func (t *RawTarget) RemoveDirectory(targetDir string) error {
	panic("not implemented")
}

func (t *RawTarget) InstallLkms(lkms []string) (installedLkms []string, err error) {
	panic("not implemented")
}

func (t *RawTarget) UninstallLkms(lkms []string) error {
	panic("not implemented")
}
