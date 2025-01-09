package metrics

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// nmi_watchdog provides helper functions for enabling and disabling the NMI (non-maskable interrupt) watchdog

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"perfspect/internal/script"
	"perfspect/internal/target"
)

// EnableNMIWatchdog - sets the kernel.nmi_watchdog value to "1"
func EnableNMIWatchdog(myTarget target.Target, localTempDir string) (err error) {
	slog.Info("enabling NMI watchdog")
	err = setNMIWatchdog(myTarget, "1", localTempDir)
	return
}

// DisableNMIWatchdog - sets the kernel.nmi_watchdog value to "0"
func DisableNMIWatchdog(myTarget target.Target, localTempDir string) (err error) {
	slog.Info("disabling NMI watchdog")
	err = setNMIWatchdog(myTarget, "0", localTempDir)
	return
}

// NMIWatchdogEnabled - reads the kernel.nmi_watchdog value. If it is "1", returns true
func NMIWatchdogEnabled(myTarget target.Target) (enabled bool, err error) {
	var setting string
	if setting, err = getNMIWatchdog(myTarget); err != nil {
		return
	}
	enabled = setting == "1"
	return
}

// getNMIWatchdog - gets the kernel.nmi_watchdog configuration value (0 or 1)
func getNMIWatchdog(myTarget target.Target) (setting string, err error) {
	// sysctl kernel.nmi_watchdog
	// kernel.nmi_watchdog = [0|1]
	var sysctl string
	if sysctl, err = findSysctl(myTarget); err != nil {
		return
	}
	cmd := exec.Command(sysctl, "kernel.nmi_watchdog")
	stdout, _, _, err := myTarget.RunCommand(cmd, 0, true)
	if err != nil {
		return
	}
	out := stdout
	setting = out[len(out)-2 : len(out)-1]
	return
}

// setNMIWatchdog -sets the kernel.nmi_watchdog configuration value
func setNMIWatchdog(myTarget target.Target, setting string, localTempDir string) (err error) {
	// sysctl kernel.nmi_watchdog=[0|1]
	var sysctl string
	if sysctl, err = findSysctl(myTarget); err != nil {
		return
	}
	_, err = script.RunScript(myTarget, script.ScriptDefinition{
		Name:      "set NMI watchdog",
		Script:    fmt.Sprintf("%s kernel.nmi_watchdog=%s", sysctl, setting),
		Superuser: true},
		localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to set NMI watchdog to %s, %v", setting, err)
		return
	}
	var outSetting string
	if outSetting, err = getNMIWatchdog(myTarget); err != nil {
		return
	}
	if outSetting != setting {
		err = fmt.Errorf("failed to set NMI watchdog to %s", setting)
	}
	return
}

// findSysctl - gets a useable path to sysctl or error
func findSysctl(myTarget target.Target) (path string, err error) {
	cmd := exec.Command("which", "sysctl")
	stdout, _, _, err := myTarget.RunCommand(cmd, 0, true)
	if err == nil {
		//found it
		path = strings.TrimSpace(stdout)
		return
	}
	// didn't find it on the path, try being specific
	sbinPath := "/usr/sbin/sysctl"
	cmd = exec.Command("which", sbinPath)
	_, _, _, err = myTarget.RunCommand(cmd, 0, true)
	if err == nil {
		// found it
		path = sbinPath
		return
	}
	err = fmt.Errorf("sysctl not found on path or at %s", sbinPath)
	return
}
