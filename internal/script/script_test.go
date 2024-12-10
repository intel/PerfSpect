package script

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"perfspect/internal/target"
)

func TestRunScript(t *testing.T) {
	var targets []target.Target
	// targets = append(targets, target.NewRemoteTarget("", "emr", "", "", "", "", "../../tools/bin/sshpass", ""))
	targets = append(targets, target.NewLocalTarget())
	for _, tgt := range targets {
		targetTempDir, err := tgt.CreateTempDirectory("/tmp")
		defer func() {
			err := tgt.RemoveDirectory(targetTempDir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var superuserVals []bool
		//superuserVals = append(superuserVals, true)
		superuserVals = append(superuserVals, false)
		for _, superuser := range superuserVals {
			// test one line script
			scriptDef1 := ScriptDefinition{
				Name:      "unittest hello",
				Script:    "echo 'Hello, World!'",
				Superuser: superuser,
				Lkms:      []string{},
				Depends:   []string{},
			}
			tempDir, err := os.MkdirTemp(os.TempDir(), "test")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			scriptOutput, err := RunScript(tgt, scriptDef1, tempDir)
			os.RemoveAll(tempDir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expectedStdout := "Hello, World!\n"
			if scriptOutput.Stdout != expectedStdout {
				t.Errorf("unexpected stdout: got %q, want %q", scriptOutput.Stdout, expectedStdout)
			}

			expectedStderr := ""
			if scriptOutput.Stderr != expectedStderr {
				t.Errorf("unexpected stderr: got %q, want %q", scriptOutput.Stderr, expectedStderr)
			}

			expectedExitCode := 0
			if scriptOutput.Exitcode != expectedExitCode {
				t.Errorf("unexpected exit code: got %d, want %d", scriptOutput.Exitcode, expectedExitCode)
			}

			// test multi-line script
			scriptDef2 := ScriptDefinition{
				Name: "unittest cores",
				Script: `num_cores_per_socket=$( lscpu | grep 'Core(s) per socket:' | head -1 | awk '{print $4}' )
echo "Core Count: $num_cores_per_socket"`,
				Superuser: superuser,
				Lkms:      []string{},
				Depends:   []string{},
			}
			tempDir, err = os.MkdirTemp(os.TempDir(), "test")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			scriptOutput, err = RunScript(tgt, scriptDef2, tempDir)
			os.RemoveAll(tempDir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			re := regexp.MustCompile("Core Count: [0-9]+")
			if !re.MatchString(scriptOutput.Stdout) {
				t.Errorf("unexpected stdout: got %q, want %q", scriptOutput.Stdout, "Core Count: [0-9]+")
			}

			expectedStderr = ""
			if scriptOutput.Stderr != expectedStderr {
				t.Errorf("unexpected stderr: got %q, want %q", scriptOutput.Stderr, expectedStderr)
			}

			expectedExitCode = 0
			if scriptOutput.Exitcode != expectedExitCode {
				t.Errorf("unexpected exit code: got %d, want %d", scriptOutput.Exitcode, expectedExitCode)
			}

			if false {
				// test multi-line script w/ dependency
				scriptDef3 := ScriptDefinition{
					Name: "Test Script",
					Script: `count=1
mpstat -u -T -I SCPU -P ALL 1 $count`,
					Superuser: superuser,
					Lkms:      []string{},
					Depends:   []string{"mpstat"},
				}
				tempDir, err := os.MkdirTemp(os.TempDir(), "test")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				scriptOutput, err = RunScript(tgt, scriptDef3, tempDir)
				os.RemoveAll(tempDir)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				expectedStdout = "Linux"
				if !strings.HasPrefix(scriptOutput.Stdout, expectedStdout) {
					t.Errorf("unexpected stdout: got %q, want %q", scriptOutput.Stdout, expectedStdout)
				}

				expectedStderr = ""
				if scriptOutput.Stderr != expectedStderr {
					t.Errorf("unexpected stderr: got %q, want %q", scriptOutput.Stderr, expectedStderr)
				}

				expectedExitCode = 0
				if scriptOutput.Exitcode != expectedExitCode {
					t.Errorf("unexpected exit code: got %d, want %d", scriptOutput.Exitcode, expectedExitCode)
				}
			}
			scriptDef1.Sequential = false
			scriptDef2.Sequential = false
			scriptOutputs, err := RunScripts(tgt, []ScriptDefinition{scriptDef1, scriptDef2}, false, os.TempDir())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(scriptOutputs) != 2 {
				t.Fatalf("unexpected number of script outputs: got %d, want %d", len(scriptOutputs), 2)
			}
			expectedStdout = "Hello, World!\n"
			if scriptOutputs["unittest hello"].Stdout != expectedStdout {
				t.Errorf("unexpected stdout: got %q, want %q", scriptOutputs["unittest hello"].Stdout, expectedStdout)
			}
			re = regexp.MustCompile("Core Count: [0-9]+")
			if !re.MatchString(scriptOutput.Stdout) {
				t.Errorf("unexpected stdout: got %q, want %q", scriptOutput.Stdout, "Core Count: [0-9]+")
			}
		}
	}
}
