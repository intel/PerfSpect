package script

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"perfspect/internal/target"
)

func TestRunOneLineScript(t *testing.T) {
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
				Name:           "unittest hello",
				ScriptTemplate: "echo 'Hello, World!'",
				Superuser:      superuser,
				Lkms:           []string{},
				Depends:        []string{},
			}
			localTempDir, err := os.MkdirTemp(os.TempDir(), "test")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			localTargetDir := path.Join(localTempDir, tgt.GetName())
			err = os.MkdirAll(localTargetDir, 0700)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			scriptOutput, err := RunScript(tgt, scriptDef1, localTempDir)
			os.RemoveAll(localTempDir)
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
		}
	}
}
func TestRunMultiLineScript(t *testing.T) {
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
			// test multi-line script
			scriptDef2 := ScriptDefinition{
				Name: "unittest cores",
				ScriptTemplate: `num_cores_per_socket=$( lscpu | grep 'Core(s) per socket:' | head -1 | awk '{print $4}' )
echo "Core Count: $num_cores_per_socket"`,
				Superuser: superuser,
				Lkms:      []string{},
				Depends:   []string{},
			}
			localTempDir, err := os.MkdirTemp(os.TempDir(), "test")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			localTargetDir := path.Join(localTempDir, tgt.GetName())
			err = os.MkdirAll(localTargetDir, 0700)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			scriptOutput, err := RunScript(tgt, scriptDef2, localTempDir)
			os.RemoveAll(localTempDir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			re := regexp.MustCompile("Core Count: [0-9]+")
			if !re.MatchString(scriptOutput.Stdout) {
				t.Errorf("unexpected stdout: got %q, want %q", scriptOutput.Stdout, "Core Count: [0-9]+")
			}

			expectedStderr := ""
			if scriptOutput.Stderr != expectedStderr {
				t.Errorf("unexpected stderr: got %q, want %q", scriptOutput.Stderr, expectedStderr)
			}

			expectedExitCode := 0
			if scriptOutput.Exitcode != expectedExitCode {
				t.Errorf("unexpected exit code: got %d, want %d", scriptOutput.Exitcode, expectedExitCode)
			}
		}
	}
}
func TestRunScriptsWithDependency(t *testing.T) {
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
			if false {
				// test multi-line script w/ dependency
				scriptDef3 := ScriptDefinition{
					Name: "Test Script",
					ScriptTemplate: `count=1
mpstat -u -T -I SCPU -P ALL 1 $count`,
					Superuser: superuser,
					Lkms:      []string{},
					Depends:   []string{"mpstat"},
				}
				localTempDir, err := os.MkdirTemp(os.TempDir(), "test")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				localTargetDir := path.Join(localTempDir, tgt.GetName())
				err = os.MkdirAll(localTargetDir, 0700)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				scriptOutput, err := RunScript(tgt, scriptDef3, localTempDir)
				os.RemoveAll(localTempDir)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				expectedStdout := "Linux"
				if !strings.HasPrefix(scriptOutput.Stdout, expectedStdout) {
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
			}
			// scriptDef1.Sequential := false
			// scriptDef2.Sequential := false
			// scriptOutputs, err := RunScripts(tgt, []ScriptDefinition{scriptDef1, scriptDef2}, false, os.TempDir())
			// if err != nil {
			// 	t.Fatalf("unexpected error: %v", err)
			// }
			// if len(scriptOutputs) != 2 {
			// 	t.Fatalf("unexpected number of script outputs: got %d, want %d", len(scriptOutputs), 2)
			// }
			// expectedStdout = "Hello, World!\n"
			// if scriptOutputs["unittest hello"].Stdout != expectedStdout {
			// 	t.Errorf("unexpected stdout: got %q, want %q", scriptOutputs["unittest hello"].Stdout, expectedStdout)
			// }
			// re = regexp.MustCompile("Core Count: [0-9]+")
			// if !re.MatchString(scriptOutput.Stdout) {
			// 	t.Errorf("unexpected stdout: got %q, want %q", scriptOutput.Stdout, "Core Count: [0-9]+")
			// }
		}
	}
}

func TestFormMasterScriptTemplateStructure(t *testing.T) {
	scripts := []ScriptDefinition{
		{Name: "alpha script", Superuser: false},
		{Name: "beta-script", Superuser: true},
	}
	master, elevated, err := formControllerScript("/tmp/targetdir", scripts, nil)
	if err != nil {
		t.Fatalf("error forming master script: %v", err)
	}
	if !elevated {
		t.Fatalf("expected elevated=true when at least one script is superuser")
	}
	// Shebang
	if !strings.HasPrefix(master, "#!/usr/bin/env bash") {
		t.Errorf("master script missing shebang")
	}
	// Functions present
	for _, fn := range []string{"start_concurrent_scripts()", "run_sequential_scripts()", "kill_script()", "wait_for_concurrent_scripts()", "print_summary()", "handle_sigint()"} {
		if !strings.Contains(master, fn) {
			t.Errorf("expected function %s in master script", fn)
		}
	}
	// Sanitized names appear (spaces and hyphens replaced with underscores)
	if !strings.Contains(master, "alpha_script") || !strings.Contains(master, "beta_script") {
		t.Errorf("sanitized script names not found in template output")
	}
	// Mapping of original names present (orig_names associative array entries)
	for _, mapping := range []string{"orig_names[alpha_script]=\"alpha script\"", "orig_names[beta_script]=\"beta-script\""} {
		if !strings.Contains(master, mapping) {
			t.Errorf("expected original name mapping %q in master script", mapping)
		}
	}
	// Delimiter used for parsing
	if !strings.Contains(master, "<---------------------->") {
		t.Errorf("expected delimiter for parsing in master script")
	}
}

func TestFormMasterScriptNeedsElevatedFlag(t *testing.T) {
	scripts := []ScriptDefinition{{Name: "user", Superuser: false}, {Name: "also user", Superuser: false}}
	_, elevated, err := formControllerScript("/tmp/dir", scripts, nil)
	if err != nil {
		t.Fatalf("error forming master script: %v", err)
	}
	if elevated {
		t.Fatalf("expected elevated=false when no scripts require superuser")
	}
}

func TestFormMasterScriptEmptyScripts(t *testing.T) {
	master, elevated, err := formControllerScript("/tmp/dir", nil, nil)
	if err != nil {
		t.Fatalf("error forming master script: %v", err)
	}
	if elevated {
		t.Fatalf("expected elevated=false with empty slice")
	}
	// Should still contain core function definitions even if no scripts.
	if !strings.Contains(master, "start_concurrent_scripts()") || !strings.Contains(master, "print_summary()") {
		t.Errorf("template missing expected functions for empty slice")
	}
	t.Logf("MASTER SCRIPT EMPTY:\n%s", master)
	// No orig_names assignments lines for empty slice.
	if strings.Count(master, "orig_names[") > 0 {
		for line := range strings.SplitSeq(master, "\n") {
			if strings.Contains(line, "orig_names[") && strings.Contains(line, "]=") {
				// assignment line detected
				t.Errorf("no orig_names mappings should appear for empty slice")
			}
		}
	}
}

func TestFormMasterScriptExecutionIntegration(t *testing.T) {
	// Integration test: create temp directory, stub two child scripts, run master script, parse output.
	tmp := t.TempDir()
	scripts := []ScriptDefinition{{Name: "alpha script"}, {Name: "beta-script"}}
	master, elevated, err := formControllerScript(tmp, scripts, nil)
	if err != nil {
		t.Fatalf("error forming master script: %v", err)
	}
	if elevated { // none marked superuser
		t.Fatalf("did not expect elevated=true for non-superuser scripts")
	}
	// Create child scripts.
	for _, s := range scripts {
		sanitized := sanitizeScriptName(s.Name)
		childPath := filepath.Join(tmp, sanitized+".sh")
		content := "#!/usr/bin/env bash\n" + "echo STDOUT-" + sanitized + "\n" + "echo STDERR-" + sanitized + " 1>&2\n"
		if err := os.WriteFile(childPath, []byte(content), 0o700); err != nil {
			t.Fatalf("failed writing child script %s: %v", childPath, err)
		}
	}
	// Write master script.
	masterPath := filepath.Join(tmp, "concurrent_master.sh")
	if err := os.WriteFile(masterPath, []byte(master), 0o700); err != nil {
		t.Fatalf("failed writing master script: %v", err)
	}
	// Run master script.
	out, err := runLocalBash(masterPath)
	if err != nil {
		// Read master script content for debugging
		content, _ := os.ReadFile(masterPath)
		t.Fatalf("error executing master script: %v\nstdout+stderr: %s\nMASTER SCRIPT:\n%s", err, out, string(content))
	}
	parsed := parseControllerScriptOutput(out)
	if len(parsed) != 2 {
		t.Fatalf("expected 2 parsed script outputs, got %d", len(parsed))
	}
	// Validate each output.
	for _, p := range parsed {
		if p.Exitcode != 0 { // child scripts exit 0
			t.Errorf("expected exit code 0 for %s, got %d", p.Name, p.Exitcode)
		}
		if !strings.Contains(p.Stdout, "STDOUT-"+sanitizeScriptName(p.Name)) {
			t.Errorf("stdout mismatch for %s: %q", p.Name, p.Stdout)
		}
		if !strings.Contains(p.Stderr, "STDERR-"+sanitizeScriptName(p.Name)) {
			t.Errorf("stderr mismatch for %s: %q", p.Name, p.Stderr)
		}
	}
}

// runLocalBash executes a bash script locally and returns combined stdout.
func runLocalBash(scriptPath string) (string, error) {
	outBytes, err := exec.Command("bash", scriptPath).CombinedOutput() // #nosec G204
	return string(outBytes), err
}
