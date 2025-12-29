// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package workflow

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"perfspect/internal/progress"
	"perfspect/internal/script"
	"perfspect/internal/target"
	"perfspect/internal/util"
)

func signalProcessOnTarget(t target.Target, pidStr string, sigStr string) error {
	var cmd *exec.Cmd
	// prepend "-" to the signal string if not already present
	if !strings.HasPrefix(sigStr, "-") {
		sigStr = "-" + sigStr
	}
	if !t.IsSuperUser() && t.CanElevatePrivileges() {
		cmd = exec.Command("sudo", "kill", sigStr, pidStr)
	} else {
		cmd = exec.Command("kill", sigStr, pidStr)
	}
	_, _, _, err := t.RunCommandEx(cmd, 5, false, true) // #nosec G204
	return err
}

// configureSignalHandler sets up a signal handler to catch SIGINT and SIGTERM
//
// When perfspect receives ctrl-c while in the shell, the shell propagates the
// signal to all our children. But when perfspect is run in the background or disowned and
// then receives SIGINT, e.g., from a script, we need to send the signal to our children
//
// When running scripts using the controller.sh script, we need to send the signal to the
// controller.sh script on each target so that it can clean up its child processes. This is
// because the controller.sh script is run in its own process group and does not receive the
// signal when perfspect receives it.
//
// Parameters:
//   - myTargets: The list of targets to send the signal to.
//   - statusFunc: A function to update the status of the progress indicator.
func configureSignalHandler(myTargets []target.Target, statusFunc progress.MultiSpinnerUpdateFunc) {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChannel
		slog.Debug("received signal", slog.String("signal", sig.String()))
		// The controller.sh script is run in its own process group, so we need to send the signal
		// directly to the PID of the controller. For every target, look for the primary_collection_script
		// PID file and send SIGINT to it.
		// The controller script is run in its own process group, so we need to send the signal
		// directly to the PID of the controller. For every target, look for the controller
		// PID file and send SIGINT to it.
		for _, t := range myTargets {
			if statusFunc != nil {
				_ = statusFunc(t.GetName(), "Signal received, cleaning up...")
			}
			pidFilePath := filepath.Join(t.GetTempDirectory(), script.ControllerPIDFileName)
			stdout, _, exitcode, err := t.RunCommandEx(exec.Command("cat", pidFilePath), 5, false, true) // #nosec G204
			if err != nil {
				slog.Error("error retrieving target controller PID", slog.String("target", t.GetName()), slog.String("error", err.Error()))
			}
			if exitcode == 0 {
				pidStr := strings.TrimSpace(stdout)
				err = signalProcessOnTarget(t, pidStr, "SIGINT")
				if err != nil {
					slog.Error("error sending SIGINT signal to target controller", slog.String("target", t.GetName()), slog.String("error", err.Error()))
				}
			}
		}
		// now wait until all controller scripts have exited
		slog.Debug("waiting for controller scripts to exit")
		for _, t := range myTargets {
			// create a per-target timeout context
			targetTimeout := 10 * time.Second
			ctx, cancel := context.WithTimeout(context.Background(), targetTimeout)
			timedOut := false
			pidFilePath := filepath.Join(t.GetTempDirectory(), script.ControllerPIDFileName)
			for {
				// read the pid file
				stdout, _, exitcode, err := t.RunCommandEx(exec.Command("cat", pidFilePath), 5, false, true) // #nosec G204
				if err != nil || exitcode != 0 {
					// pid file doesn't exist
					break
				}
				pidStr := strings.TrimSpace(stdout)
				// determine if the process still exists
				_, _, exitcode, err = t.RunCommandEx(exec.Command("ps", "-p", pidStr), 5, false, true) // #nosec G204
				if err != nil || exitcode != 0 {
					break // process no longer exists, script has exited
				}
				// check for timeout
				select {
				case <-ctx.Done():
					timedOut = true
				default:
				}
				if timedOut {
					if statusFunc != nil {
						_ = statusFunc(t.GetName(), "cleanup timeout exceeded, sending kill signal")
					}
					slog.Warn("signal handler cleanup timeout exceeded for target, sending SIGKILL", slog.String("target", t.GetName()))
					err = signalProcessOnTarget(t, pidStr, "SIGKILL")
					if err != nil {
						slog.Error("error sending SIGKILL signal to target controller", slog.String("target", t.GetName()), slog.String("error", err.Error()))
					}
					break
				}
				// sleep for a short time before checking again
				time.Sleep(500 * time.Millisecond)
			}
			cancel()
		}

		// send SIGINT to perfspect's children
		err := util.SignalChildren(syscall.SIGINT)
		if err != nil {
			slog.Error("error sending signal to children", slog.String("error", err.Error()))
		}
	}()
}
