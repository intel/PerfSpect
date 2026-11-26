// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"perfspect/internal/script"
	"perfspect/internal/target"
	"perfspect/internal/util"
	"perfspect/internal/workflow"
)

const (
	containerRuntimeAuto = "auto"
	runtimeKindDocker    = "docker"
	runtimeKindCrictl    = "crictl"
	runtimeKindCtr       = "ctr"
	runtimeKindNerdctl   = "nerdctl"
)

var errNoContainersFound = errors.New("no containers found")

// ContainerRuntimeSpec describes the user-configured --container-runtime option.
type ContainerRuntimeSpec struct {
	Kind       string // "auto", "docker", "crictl", "ctr", "nerdctl"
	BinaryPath string // optional explicit path on target starting with "/"
}

// DetectedRuntime represents an active container runtime discovered on the target.
type DetectedRuntime struct {
	Kind       string // "docker", "crictl", "ctr", "nerdctl"
	BinaryPath string // resolved executable path on target
	Endpoint   string // optional --runtime-endpoint (crictl) or --address (ctr)
}

// Container represents a discovered running container and its resolved host cgroup.
type Container struct {
	ID          string
	Name        string // short container name
	DisplayName string // unique report label (disambiguated when needed)
	PodName     string
	Namespace   string
	Runtime     string // "docker", "cri", "containerd"
	PID         int
	CgroupPath  string // relative to /sys/fs/cgroup, no leading slash
	CPUUsage    uint64 // usage_usec from cpu.stat
}

type runtimeProbeSpec struct {
	kind         string
	probeArgs    []string
	endpointFlag string
	sockets      []string
}

var runtimeProbeSpecs = []runtimeProbeSpec{
	{
		kind:      runtimeKindDocker,
		probeArgs: []string{"ps", "-q"},
	},
	{
		kind:         runtimeKindCrictl,
		probeArgs:    []string{"ps", "-q"},
		endpointFlag: "--runtime-endpoint",
		sockets: []string{
			"unix:///run/containerd/containerd.sock",
			"unix:///var/run/containerd/containerd.sock",
			"unix:///run/crio/crio.sock",
			"unix:///var/run/crio/crio.sock",
			"unix:///run/k3s/containerd/containerd.sock",
			"unix:///var/run/cri-dockerd.sock",
		},
	},
	{
		kind:         runtimeKindCtr,
		probeArgs:    []string{"namespaces", "list", "-q"},
		endpointFlag: "--address",
		sockets: []string{
			"/run/containerd/containerd.sock",
			"/var/run/containerd/containerd.sock",
			"/run/k3s/containerd/containerd.sock",
		},
	},
	{
		kind:      runtimeKindNerdctl,
		probeArgs: []string{"ps", "-q"},
	},
}

func parseContainerRuntimeSpec(raw string) (ContainerRuntimeSpec, error) {
	switch raw {
	case containerRuntimeAuto:
		return ContainerRuntimeSpec{Kind: containerRuntimeAuto}, nil
	case runtimeKindDocker, runtimeKindCrictl, runtimeKindCtr, runtimeKindNerdctl:
		return ContainerRuntimeSpec{Kind: raw}, nil
	}
	if !strings.HasPrefix(raw, "/") {
		return ContainerRuntimeSpec{}, fmt.Errorf("invalid container runtime %q: valid options are auto, docker, crictl, ctr, nerdctl, or /path/to/<binary>", raw)
	}
	if strings.HasSuffix(raw, "/") || strings.Contains(raw, "..") || !util.IsValidDirectoryName(raw) {
		return ContainerRuntimeSpec{}, fmt.Errorf("invalid container runtime path %q", raw)
	}
	base := filepath.Base(raw)
	switch base {
	case runtimeKindDocker, runtimeKindCrictl, runtimeKindCtr, runtimeKindNerdctl:
		return ContainerRuntimeSpec{Kind: base, BinaryPath: raw}, nil
	default:
		return ContainerRuntimeSpec{}, fmt.Errorf("unsupported container runtime binary %q: basename must be docker, crictl, ctr, or nerdctl", raw)
	}
}

func buildDetectContainerRuntimesScript(spec ContainerRuntimeSpec) string {
	var b strings.Builder
	b.WriteString(`export PATH="$PATH:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/home/kubernetes/bin:/opt/bin:/snap/bin:/var/lib/rancher/rke2/bin"
find_runtime_bin() {
    name="$1"
    explicit="$2"
    if [ -n "$explicit" ]; then
        if [ -x "$explicit" ]; then
            printf '%s\n' "$explicit"
            return 0
        fi
        return 1
    fi
    command -v "$name" 2>/dev/null
}
probe_runtime() {
    kind="$1"
    explicit="$2"
    endpoint_flag="$3"
    endpoints="$4"
    shift 4
    if ! bin=$(find_runtime_bin "$kind" "$explicit"); then
        return 0
    fi
    if "$bin" "$@" >/dev/null 2>&1; then
        printf 'RUNTIME|%s|%s|\n' "$kind" "$bin"
        return 0
    fi
    if [ -n "$endpoint_flag" ] && [ -n "$endpoints" ]; then
        for ep in $endpoints; do
            sock="${ep#unix://}"
            if [ -S "$sock" ] && "$bin" "$endpoint_flag" "$ep" "$@" >/dev/null 2>&1; then
                printf 'RUNTIME|%s|%s|%s\n' "$kind" "$bin" "$ep"
                return 0
            fi
        done
    fi
}
`)
	for _, p := range runtimeProbeSpecs {
		if spec.Kind != containerRuntimeAuto && spec.Kind != p.kind {
			continue
		}
		explicit := "''"
		if spec.Kind == p.kind && spec.BinaryPath != "" {
			explicit = util.ShellSingleQuote(spec.BinaryPath)
		}
		quotedArgs := make([]string, 0, len(p.probeArgs))
		for _, a := range p.probeArgs {
			quotedArgs = append(quotedArgs, util.ShellSingleQuote(a))
		}
		fmt.Fprintf(&b, "probe_runtime %s %s %s %s %s\n",
			util.ShellSingleQuote(p.kind),
			explicit,
			util.ShellSingleQuote(p.endpointFlag),
			util.ShellSingleQuote(strings.Join(p.sockets, " ")),
			strings.Join(quotedArgs, " "),
		)
	}
	b.WriteString("true\n")
	return b.String()
}

func parseDetectedRuntimes(stdout string) []DetectedRuntime {
	var runtimes []DetectedRuntime
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "RUNTIME|") {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}
		kind := strings.TrimSpace(parts[1])
		binPath := strings.TrimSpace(parts[2])
		endpoint := strings.TrimSpace(parts[3])
		switch kind {
		case runtimeKindDocker, runtimeKindCrictl, runtimeKindCtr, runtimeKindNerdctl:
		default:
			continue
		}
		if binPath == "" || seen[kind] {
			continue
		}
		seen[kind] = true
		runtimes = append(runtimes, DetectedRuntime{
			Kind:       kind,
			BinaryPath: binPath,
			Endpoint:   endpoint,
		})
	}
	return runtimes
}

func validateDetectedRuntimes(detected []DetectedRuntime, spec ContainerRuntimeSpec, configured string) error {
	if len(detected) > 0 {
		return nil
	}
	if spec.Kind == containerRuntimeAuto {
		return fmt.Errorf("no active container runtime (docker, crictl, ctr, nerdctl) detected on target")
	}
	return fmt.Errorf("configured container runtime %q is not available or not running on target", configured)
}

// DetectContainerRuntimes probes the target for active container runtime CLIs and sockets.
func DetectContainerRuntimes(myTarget target.Target, configured string, localTempDir string, noRoot bool) ([]DetectedRuntime, error) {
	spec, err := parseContainerRuntimeSpec(configured)
	if err != nil {
		return nil, err
	}
	detectScript := script.ScriptDefinition{
		Name:           "detect_container_runtimes",
		ScriptTemplate: buildDetectContainerRuntimesScript(spec),
		Superuser:      !noRoot,
	}
	output, err := workflow.RunScript(myTarget, detectScript, localTempDir, noRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to detect container runtimes: %w", err)
	}
	detected := parseDetectedRuntimes(output.Stdout)
	if err := validateDetectedRuntimes(detected, spec, configured); err != nil {
		return nil, err
	}
	var desc []string
	for _, rt := range detected {
		if rt.Endpoint != "" {
			desc = append(desc, fmt.Sprintf("%s(%s@%s)", rt.Kind, rt.BinaryPath, rt.Endpoint))
		} else {
			desc = append(desc, fmt.Sprintf("%s(%s)", rt.Kind, rt.BinaryPath))
		}
	}
	slog.Info("Detected container runtimes", slog.String("target", myTarget.GetName()), slog.String("runtimes", strings.Join(desc, ", ")))
	return detected, nil
}
