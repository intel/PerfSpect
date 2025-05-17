package target

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// installLkms attempts to install a list of Linux Kernel Modules (LKMs) on the target system.
// It requires elevated privileges to perform the installation.
//
// Parameters:
//   - t: A Target interface that represents the system where the LKMs will be installed.
//     The Target must support privilege elevation.
//   - lkms: A slice of strings representing the names of the LKMs to be installed.
//
// Returns:
//   - installedLkms: A slice of strings containing the names of the LKMs that were successfully installed.
//   - err: An error if privilege elevation is not possible or if any other issue occurs.
func installLkms(t Target, lkms []string) (installedLkms []string, err error) {
	if !t.CanElevatePrivileges() {
		err = fmt.Errorf("can't elevate privileges; elevated privileges required to install lkms")
		return
	}
	for _, lkm := range lkms {
		slog.Debug("attempting to install kernel module", slog.String("lkm", lkm))
		_, _, _, err := t.RunCommand(exec.Command("modprobe", "--first-time", lkm), 10, true) // #nosec G204
		if err != nil {
			slog.Debug("kernel module already installed or problem installing", slog.String("lkm", lkm), slog.String("error", err.Error()))
			continue
		}
		slog.Debug("kernel module installed", slog.String("lkm", lkm))
		installedLkms = append(installedLkms, lkm)
	}
	return
}

// uninstallLkms attempts to uninstall a list of Linux kernel modules (LKMs) from the target system.
// It requires elevated privileges to perform the operation.
//
// Parameters:
//   - t: A Target interface that represents the system where the LKMs will be uninstalled.
//     The Target must support privilege elevation.
//   - lkms: A slice of strings representing the names of the kernel modules to be uninstalled.
//
// Returns:
//   - err: An error if privilege elevation is not possible or if any other issue occurs during the process.
func uninstallLkms(t Target, lkms []string) (err error) {
	if !t.CanElevatePrivileges() {
		err = fmt.Errorf("can't elevate privileges; elevated privileges required to uninstall lkms")
		return
	}
	for _, lkm := range lkms {
		slog.Debug("attempting to uninstall kernel module", slog.String("lkm", lkm))
		_, _, _, err := t.RunCommand(exec.Command("modprobe", "-r", lkm), 10, true) // #nosec G204
		if err != nil {
			slog.Error("error uninstalling kernel module", slog.String("lkm", lkm), slog.String("error", err.Error()))
			continue
		}
		slog.Debug("kernel module uninstalled", slog.String("lkm", lkm))
	}
	return
}

// runLocalCommandWithInputWithTimeout executes a local command with optional input and a timeout.
// It captures the command's standard output, standard error, and exit code.
//
// Parameters:
//   - cmd: The command to execute, represented as an *exec.Cmd.
//   - input: A string to be passed as input to the command's standard input.
//   - timeout: The timeout in seconds for the command execution. If set to 0, no timeout is applied.
//
// Returns:
//   - stdout: The standard output of the command as a string.
//   - stderr: The standard error of the command as a string.
//   - exitCode: The exit code of the command. If the command fails to execute, this may be undefined.
//   - err: An error object if the command fails to execute or times out.
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
		commandWithContext := exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...) // #nosec G204 // nosemgrep
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

// runLocalCommandWithInputWithTimeoutAsync executes a local command asynchronously with optional input and timeout.
// It streams the command's stdout and stderr to the provided channels and sends the exit code to the exitcodeChannel.
//
// Parameters:
//   - cmd: The command to execute, represented as an *exec.Cmd.
//   - stdoutChannel: A channel to send lines of stdout output.
//   - stderrChannel: A channel to send lines of stderr output.
//   - exitcodeChannel: A channel to send the exit code of the command.
//   - input: A string to be passed as input to the command's stdin. If empty, no input is provided.
//   - timeout: The timeout in seconds for the command execution. If 0 or less, no timeout is applied.
//
// Returns:
//   - err: An error if the command fails to start or if there are issues with pipes.
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
		commandWithContext := exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...) // #nosec G204 // nosemgrep
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
			slog.Error("unexpected error type while waiting for command to finish", slog.String("cmd", cmd.String()), slog.String("error", err.Error()))
			exitcodeChannel <- -1
		}
	} else {
		exitcodeChannel <- 0
	}
	return nil
}

// getArchitecture determines the architecture of the target system by executing
// the "uname -m" command. It returns the architecture as a string and an error
// if the command execution fails.
//
// Parameters:
//   - t: A Target instance that provides the ability to run commands on the target system.
//
// Returns:
//   - arch: A string representing the architecture of the target system (e.g., "x86_64").
//   - err: An error if the command execution fails or if there is an issue retrieving the architecture.
func getArchitecture(t Target) (arch string, err error) {
	cmd := exec.Command("uname", "-m")
	arch, _, _, err = t.RunCommand(cmd, 0, true)
	if err != nil {
		return
	}
	arch = strings.TrimSpace(arch)
	return
}

// getFamily retrieves the CPU family of the target system by executing a shell command.
// It runs the "lscpu" command to extract the "CPU family" field and returns the value as a string.
//
// Parameters:
//   - t: A Target instance that provides the method to execute the command.
//
// Returns:
//   - family: A string representing the CPU family of the target system.
//   - err: An error if the command execution or parsing fails.
func getFamily(t Target) (family string, err error) {
	cmd := exec.Command("bash", "-c", "lscpu | grep -i \"^CPU family:\" | awk '{print $NF}'")
	family, _, _, err = t.RunCommand(cmd, 0, true)
	if err != nil {
		return
	}
	family = strings.TrimSpace(family)
	return
}

// getModel retrieves the CPU model of the target system by executing a shell command.
// It runs the "lscpu" command, filters the output for the "Model" field, and extracts
// the last field of the line using "awk". The result is trimmed of any leading or trailing
// whitespace before being returned.
//
// Parameters:
//
//	t - The Target interface that provides the ability to execute commands on the target system.
//
// Returns:
//
//	model - A string representing the CPU model of the target system.
//	err - An error if the command execution fails or if there is an issue retrieving the model.
func getModel(t Target) (model string, err error) {
	cmd := exec.Command("bash", "-c", "lscpu | grep -i model: | awk '{print $NF}'")
	model, _, _, err = t.RunCommand(cmd, 0, true)
	if err != nil {
		return
	}
	model = strings.TrimSpace(model)
	return
}

// getStepping retrieves the CPU stepping information of the target system.
// It executes a shell command to parse the output of the `lscpu` command
// and extracts the stepping value using `grep` and `awk`.
//
// Parameters:
//   - t: A Target instance that provides the ability to execute commands.
//
// Returns:
//   - stepping: A string representing the CPU stepping value.
//   - err: An error if the command execution or parsing fails.
func getStepping(t Target) (stepping string, err error) {
	cmd := exec.Command("bash", "-c", "lscpu | grep -i stepping: | awk '{print $NF}'")
	stepping, _, _, err = t.RunCommand(cmd, 0, true)
	if err != nil {
		return
	}
	stepping = strings.TrimSpace(stepping)
	return
}

// getVendor retrieves the vendor ID of the CPU by executing a shell command.
// It runs the "lscpu" command, filters the output for the "Vendor ID" field,
// and extracts the last field using "awk". The result is then trimmed of any
// leading or trailing whitespace.
//
// Parameters:
//
//	t Target - The target object that provides the RunCommand method.
//
// Returns:
//
//	vendor (string) - The vendor ID of the CPU.
//	err (error) - An error if the command execution or parsing fails.
func getVendor(t Target) (vendor string, err error) {
	cmd := exec.Command("bash", "-c", "lscpu | grep -i \"^Vendor ID:\" | awk '{print $NF}'")
	vendor, _, _, err = t.RunCommand(cmd, 0, true)
	if err != nil {
		return
	}
	vendor = strings.TrimSpace(vendor)
	return
}
