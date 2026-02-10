// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	// waitTime must exceed the controller script's kill_script graceful shutdown period (5s)
	// plus buffer for network latency when working with remote targets
	waitTime := 15
	_, _, exitCode, err := t.RunCommandEx(cmd, waitTime, false, true) // #nosec G204
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("kill command returned exit code %d", exitCode)
	}
	return nil
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
		// wait for a signal
		sig := <-sigChannel
		slog.Debug("received signal", slog.String("signal", sig.String()))
		// The controller script is run in its own process group, so we need to send the signal
		// directly to the PID of the controller. For every target, look for the controller
		// PID file and send SIGINT to it, then wait for it to exit concurrently.
		var wg sync.WaitGroup
		for _, t := range myTargets {
			if statusFunc != nil {
				_ = statusFunc(t.GetName(), "Signal received, cleaning up...")
			}
			pidFilePath := filepath.Join(t.GetTempDirectory(), script.ControllerPIDFileName)
			stdout, _, exitCode, err := t.RunCommandEx(exec.Command("cat", pidFilePath), 5, false, true) // #nosec G204
			// if there's an execution error, log and skip
			if err != nil {
				slog.Error("failed to retrieve target controller PID", slog.String("target", t.GetName()), slog.String("error", err.Error()))
				continue
			}
			// if exit code is non-zero, the file likely doesn't exist
			// so we can skip sending the signal to this target
			if exitCode != 0 {
				slog.Debug("target controller PID file not found, assuming script has already exited", slog.String("target", t.GetName()))
				continue
			}
			pid := strings.TrimSpace(stdout)
			// confirm pid is a valid integer
			if _, err := strconv.Atoi(pid); err != nil {
				slog.Error("invalid PID retrieved from target controller PID file", slog.String("target", t.GetName()), slog.String("pid", pid), slog.String("error", err.Error()))
				continue
			}
			// send SIGINT to the controller process on the target
			slog.Debug("signaling target controller process with SIGINT", slog.String("target", t.GetName()), slog.String("pid", pid))
			err = signalProcessOnTarget(t, pid, "SIGINT")
			if err != nil {
				slog.Error("failed to send SIGINT signal to target controller", slog.String("target", t.GetName()), slog.String("error", err.Error()))
				continue
			}
			// spawn a goroutine to wait for this target's controller to exit
			wg.Add(1)
			go func(tgt target.Target, pid string) {
				defer wg.Done()
				// create a per-target timeout context
				targetTimeout := 20 * time.Second
				ctx, cancel := context.WithTimeout(context.Background(), targetTimeout)
				defer cancel()
				timedOut := false
				for {
					// determine if the process still exists
					_, _, exitCode, err := tgt.RunCommandEx(exec.Command("ps", "-p", pid), 5, false, true) // #nosec G204
					if err != nil {
						slog.Error("failed to check target controller process", slog.String("target", tgt.GetName()), slog.String("error", err.Error()))
						break
					}
					// ps -p returns non-zero exit code if the process doesn't exist
					if exitCode != 0 {
						slog.Debug("target controller process no longer exists", slog.String("target", tgt.GetName()))
						break
					}
					// check for timeout
					select {
					case <-ctx.Done():
						timedOut = true
					default:
					}
					if timedOut {
						if statusFunc != nil {
							_ = statusFunc(tgt.GetName(), "cleanup timeout exceeded, sending kill signal")
						}
						slog.Warn("signal handler cleanup timeout exceeded for target, sending SIGKILL", slog.String("target", tgt.GetName()))
						err := signalProcessOnTarget(tgt, pid, "SIGKILL")
						if err != nil {
							slog.Error("failed to send SIGKILL signal to target controller", slog.String("target", tgt.GetName()), slog.String("error", err.Error()))
						}
						break
					}
					// sleep for a short time before checking again
					time.Sleep(500 * time.Millisecond)
				}
			}(t, pid)
		}
		wg.Wait()

		// Race condition between the controller script deleting its PID file and it truly exiting.
		// Future work: reconsider decision to have the controller script delete its own PID file.
		// When working with a remote target we want to give our local SSH command time to exit cleanly
		// before we send SIGINT to it. If we interrupt the SSH command unnecessarily, the controller output
		// will be lost.
		time.Sleep(500 * time.Millisecond)

		// send SIGINT to perfspect's remaining children, if any
		perfspectPid := os.Getpid()
		children, err := util.GetChildren(perfspectPid)
		if err != nil {
			slog.Error("failed to retrieve perfspect's child processes", slog.String("error", err.Error()))
			return
		}
		if len(children) == 0 {
			slog.Debug("perfspect has no child processes to signal")
			return
		}
		slog.Debug("signaling child processes", slog.String("child PIDs", fmt.Sprintf("%v", children)))
		err = util.SignalChildren(syscall.SIGINT)
		if err != nil {
			slog.Error("failed to send SIGINT signal to perfspect's child processes", slog.String("error", err.Error()))
		}
	}()
}
