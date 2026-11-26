// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"perfspect/internal/script"
	"perfspect/internal/target"
	"perfspect/internal/util"
	"perfspect/internal/workflow"
)

const (
	labelK8sPodName       = "io.kubernetes.pod.name"
	labelK8sPodNamespace  = "io.kubernetes.pod.namespace"
	labelK8sContainerName = "io.kubernetes.container.name"
	labelCRIKind          = "io.cri-containerd.kind"
	labelNerdctlName      = "nerdctl/name"
	labelContainerdName   = "io.containerd.container.name"
)

func isCRISandbox(labels map[string]string) bool {
	return strings.TrimSpace(labels[labelCRIKind]) == "sandbox"
}

func buildDiscoverContainersScript(runtimes []DetectedRuntime) string {
	byKind := make(map[string]DetectedRuntime, len(runtimes))
	for _, rt := range runtimes {
		if _, exists := byKind[rt.Kind]; !exists {
			byKind[rt.Kind] = rt
		}
	}

	var b strings.Builder
	b.WriteString(`docker_ok=0
crictl_ok=0
ctr_ok=0
pids=""
no_pid_cids=""
cid_pid_pairs=""
resolved_cgroups=""

record_pid() {
    cid="$1"
    pid="$2"
    if [ -n "$pid" ] && [ "$pid" -gt 0 ] 2>/dev/null; then
        pids="$pids $pid"
        if [ -n "$cid" ]; then
            cid_pid_pairs="$cid_pid_pairs
$cid|$pid"
        fi
    elif [ -n "$cid" ]; then
        no_pid_cids="$no_pid_cids $cid"
    fi
}

add_cgroup() {
    cg_rel="$1"
    cg_rel="${cg_rel#/}"
    cg_rel="${cg_rel%/}"
    [ -z "$cg_rel" ] && return 0
    case "/$cg_rel/" in
        */../*) return 1 ;;
    esac
    resolved_cgroups="$resolved_cgroups
$cg_rel"
    return 0
}
`)

	if rt, ok := byKind[runtimeKindDocker]; ok {
		b.WriteString(strings.ReplaceAll(`docker_bin=__DOCKER_BIN__
if docker_ids=$("$docker_bin" ps -q --no-trunc 2>/dev/null); then
    docker_ok=1
    if [ -n "$docker_ids" ]; then
        docker_out=$("$docker_bin" inspect --format '{{json .}}' $docker_ids 2>/dev/null || true)
        if [ -n "$docker_out" ]; then
            echo "===DOCKER_BEGIN==="
            printf '%s\n' "$docker_out"
            echo "===DOCKER_END==="
            docker_pids=$("$docker_bin" inspect --format '{{.Id}}|{{.State.Pid}}' $docker_ids 2>/dev/null || true)
            while IFS='|' read -r did dpid; do
                [ -z "$did" ] && continue
                record_pid "$did" "$dpid"
            done <<< "$docker_pids"
        fi
    fi
fi
`, "__DOCKER_BIN__", util.ShellSingleQuote(rt.BinaryPath)))
	}

	if rt, ok := byKind[runtimeKindCrictl]; ok {
		crictlCmd := util.ShellSingleQuote(rt.BinaryPath)
		if rt.Endpoint != "" {
			crictlCmd += " --runtime-endpoint " + util.ShellSingleQuote(rt.Endpoint)
		}
		b.WriteString(strings.ReplaceAll(`if crictl_ps=$(__CRICTL_CMD__ ps -o json 2>/dev/null) && [ -n "$crictl_ps" ]; then
    crictl_ok=1
    echo "===CRICTL_PS_BEGIN==="
    printf '%s\n' "$crictl_ps"
    echo "===CRICTL_PS_END==="
fi
cri_ids=$(__CRICTL_CMD__ ps -q 2>/dev/null || true)
if [ -n "$cri_ids" ]; then
    crictl_insp=$(__CRICTL_CMD__ inspect -o json $cri_ids 2>/dev/null || true)
    for cid in $cri_ids; do
        if ! printf '%s\n' "$crictl_insp" | grep -Fq "\"$cid\""; then
            one_insp=$(__CRICTL_CMD__ inspect -o json "$cid" 2>/dev/null || true)
            if [ -n "$one_insp" ]; then
                if [ -n "$crictl_insp" ]; then
                    crictl_insp="$crictl_insp
$one_insp"
                else
                    crictl_insp="$one_insp"
                fi
            fi
        fi
    done
    if [ -n "$crictl_insp" ]; then
        echo "===CRICTL_INSPECT_BEGIN==="
        printf '%s\n' "$crictl_insp"
        echo "===CRICTL_INSPECT_END==="
    fi
    cri_pairs=$(__CRICTL_CMD__ inspect -o go-template --template '{{.status.id}}|{{.info.pid}}' $cri_ids 2>/dev/null || true)
    case "$cri_pairs" in
        [{\[]*) cri_pairs="" ;;
        *"|"*) ;;
        *) cri_pairs="" ;;
    esac
    for cid in $cri_ids; do
        if ! printf '%s\n' "$cri_pairs" | grep -Fq "${cid}|"; then
            one_pair=$(__CRICTL_CMD__ inspect -o go-template --template '{{.status.id}}|{{.info.pid}}' "$cid" 2>/dev/null || true)
            case "$one_pair" in
                "${cid}|"*)
                    if [ -n "$cri_pairs" ]; then
                        cri_pairs="$cri_pairs
$one_pair"
                    else
                        cri_pairs="$one_pair"
                    fi
                    ;;
            esac
        fi
    done
    if [ -n "$cri_pairs" ]; then
        seen_cri_ids=""
        while IFS='|' read -r cid cpid; do
            [ -z "$cid" ] && continue
            seen_cri_ids="$seen_cri_ids $cid"
            record_pid "$cid" "$cpid"
        done <<< "$cri_pairs"
        for cid in $cri_ids; do
            case " $seen_cri_ids " in
                *" $cid "*) ;;
                *) record_pid "$cid" "" ;;
            esac
        done
    else
        if [ -n "$crictl_insp" ]; then
            for cpid in $(printf '%s\n' "$crictl_insp" | grep -oE '(^|[{, \t])"pid"[ \t]*:[ \t]*[0-9]+' | grep -oE '[0-9]+' || true); do
                record_pid "" "$cpid"
            done
        fi
        for cid in $cri_ids; do
            record_pid "$cid" ""
        done
    fi
fi
`, "__CRICTL_CMD__", crictlCmd))
	}

	if rt, ok := byKind[runtimeKindCtr]; ok {
		ctrCmd := util.ShellSingleQuote(rt.BinaryPath)
		if rt.Endpoint != "" {
			ctrCmd += " --address " + util.ShellSingleQuote(rt.Endpoint)
		}
		b.WriteString(strings.ReplaceAll(`ctr_namespaces=$(__CTR_CMD__ namespaces list -q 2>/dev/null || true)
if [ -n "$ctr_namespaces" ]; then
    ctr_ok=1
    echo "===CTR_BEGIN==="
    for ns in $ctr_namespaces; do
        if [ "$docker_ok" -eq 1 ] && [ "$ns" = "moby" ]; then
            continue
        fi
        if [ "$crictl_ok" -eq 1 ] && [ "$ns" = "k8s.io" ]; then
            continue
        fi
        tasks=$(__CTR_CMD__ -n "$ns" tasks ls 2>/dev/null | awk 'NR>1 && $3=="RUNNING" {print $1 "|" $2}' || true)
        if [ -n "$tasks" ]; then
            while IFS='|' read -r tid tpid; do
                [ -z "$tid" ] && continue
                echo "===CTR_TASK $ns|$tid|$tpid==="
                __CTR_CMD__ -n "$ns" containers info "$tid" 2>/dev/null || true
                echo ""
                echo "===CTR_TASK_END==="
                record_pid "$tid" "$tpid"
            done <<< "$tasks"
        fi
    done
    echo "===CTR_END==="
fi
`, "__CTR_CMD__", ctrCmd))
	}

	if rt, ok := byKind[runtimeKindNerdctl]; ok {
		b.WriteString(strings.ReplaceAll(`nerdctl_bin=__NERDCTL_BIN__
if [ "$ctr_ok" -eq 0 ]; then
    nerdctl_ns=$("$nerdctl_bin" namespace list -q 2>/dev/null || true)
    if [ -z "$nerdctl_ns" ]; then
        nerdctl_ns="default"
    fi
    echo "===NERDCTL_BEGIN==="
    for ns in $nerdctl_ns; do
        if [ "$docker_ok" -eq 1 ] && [ "$ns" = "moby" ]; then
            continue
        fi
        if [ "$crictl_ok" -eq 1 ] && [ "$ns" = "k8s.io" ]; then
            continue
        fi
        nids=$("$nerdctl_bin" -n "$ns" ps -q --no-trunc 2>/dev/null || true)
        if [ -n "$nids" ]; then
            nout=$("$nerdctl_bin" -n "$ns" inspect --format '{{json .}}' $nids 2>/dev/null || true)
            if [ -n "$nout" ]; then
                printf '%s\n' "$nout"
                npids=$("$nerdctl_bin" -n "$ns" inspect --format '{{.Id}}|{{.State.Pid}}' $nids 2>/dev/null || true)
                while IFS='|' read -r nid npid; do
                    [ -z "$nid" ] && continue
                    record_pid "$nid" "$npid"
                done <<< "$npids"
            fi
        fi
    done
    echo "===NERDCTL_END==="
fi
`, "__NERDCTL_BIN__", util.ShellSingleQuote(rt.BinaryPath)))
	}

	b.WriteString(`unique_pids=$(printf '%s\n' $pids | awk 'NF && !seen[$0]++')
for p in $unique_pids; do
    if [ -r "/proc/$p/cgroup" ]; then
        echo "===PID_CGROUP $p==="
        cat "/proc/$p/cgroup" 2>/dev/null || true
        pid_had_valid_cg=0
        pid_has_parent_ref=0
        while IFS= read -r cg_line; do
            cg_id="${cg_line%%:*}"
            cg_rest="${cg_line#*:}"
            cg_ctrls="${cg_rest%%:*}"
            cg_path_part="${cg_rest#*:}"
            case "$cg_path_part" in
                /*) ;;
                *) continue ;;
            esac
            cg_rel=""
            case ",$cg_ctrls," in
                ",,")
                    [ "$cg_id" = "0" ] && cg_rel="${cg_path_part#/}"
                    ;;
                *,perf_event,*|*,cpu,*|*,cpuacct,*)
                    cg_rel="${cg_path_part#/}"
                    ;;
            esac
            [ -z "$cg_rel" ] && continue
            case "/$cg_rel/" in
                */../*)
                    case ",$cg_ctrls," in
                        ",,") [ "$cg_id" = "0" ] && pid_has_parent_ref=1 ;;
                        *,perf_event,*) pid_has_parent_ref=1 ;;
                    esac
                    continue
                    ;;
            esac
            if [ -d "/sys/fs/cgroup/$cg_rel" ] || [ -d "/sys/fs/cgroup/perf_event/$cg_rel" ] || [ -d "/sys/fs/cgroup/cpu,cpuacct/$cg_rel" ] || [ -d "/sys/fs/cgroup/cpuacct,cpu/$cg_rel" ] || [ -d "/sys/fs/cgroup/cpuacct/$cg_rel" ] || [ -d "/sys/fs/cgroup/cpu/$cg_rel" ]; then
                if add_cgroup "$cg_rel"; then
                    pid_had_valid_cg=1
                fi
            fi
        done < "/proc/$p/cgroup"
        if [ "$pid_has_parent_ref" -eq 1 ] || [ "$pid_had_valid_cg" -eq 0 ]; then
            for mapped_cid in $(printf '%s\n' "$cid_pid_pairs" | awk -F'|' -v target_pid="$p" '$2 == target_pid && $1 != "" {print $1}'); do
                no_pid_cids="$no_pid_cids $mapped_cid"
            done
        fi
    else
        for mapped_cid in $(printf '%s\n' "$cid_pid_pairs" | awk -F'|' -v target_pid="$p" '$2 == target_pid && $1 != "" {print $1}'); do
            no_pid_cids="$no_pid_cids $mapped_cid"
        done
    fi
done

unique_cids=$(printf '%s\n' $no_pid_cids | awk 'NF && !seen[$0]++')
if [ -n "$unique_cids" ]; then
    all_cg_dirs=$(find /sys/fs/cgroup -type d 2>/dev/null || true)
    if [ -n "$all_cg_dirs" ]; then
        set -f
        for cid in $unique_cids; do
            cid_len=${#cid}
            found_dirs=""
            while IFS= read -r d; do
                [ -z "$d" ] && continue
                matched=0
                if [ "$cid_len" -ge 12 ]; then
                    case "$d" in
                        *"$cid"*) matched=1 ;;
                    esac
                else
                    case "/$d/" in
                        *"/$cid/"*|*"/$cid.scope/"*|*"-$cid.scope/"*|*":$cid/"*) matched=1 ;;
                    esac
                fi
                if [ "$matched" -eq 1 ]; then
                    if [ -n "$found_dirs" ]; then
                        found_dirs="$found_dirs
$d"
                    else
                        found_dirs="$d"
                    fi
                fi
            done <<< "$all_cg_dirs"
            if [ -n "$found_dirs" ]; then
                echo "===CID_CGROUP $cid==="
                printf '%s\n' "$found_dirs"
                while IFS= read -r d; do
                    [ -z "$d" ] && continue
                    cg_rel="${d#/sys/fs/cgroup/}"
                    for ctrl_prefix in "perf_event/" "cpu,cpuacct/" "cpuacct,cpu/" "cpuacct/" "cpu/"; do
                        case "$cg_rel" in
                            "$ctrl_prefix"*)
                                cg_rel="${cg_rel#"$ctrl_prefix"}"
                                break
                                ;;
                        esac
                    done
                    add_cgroup "$cg_rel" || true
                done <<< "$found_dirs"
            fi
        done
        set +f
    fi
fi

echo "===CG_CPU==="
all_cg=""
while IFS= read -r rel; do
    [ -z "$rel" ] && continue
    cur="$rel"
    while [ -n "$cur" ]; do
        all_cg="$all_cg
$cur"
        case "$cur" in
            */*) cur="${cur%/*}" ;;
            *) cur="" ;;
        esac
    done
done <<< "$resolved_cgroups"
unique_cgs=$(printf '%s\n' "$all_cg" | awk 'NF && !seen[$0]++')
while IFS= read -r rel; do
    [ -z "$rel" ] && continue
    stat_file="/sys/fs/cgroup/$rel/cpu.stat"
    if [ -f "$stat_file" ]; then
        usage=$(awk '$1 == "usage_usec" {print $2; exit}' "$stat_file" 2>/dev/null || true)
        if [ -n "$usage" ]; then
            printf '%s|%s\n' "$rel" "$usage"
        fi
    else
        for v1_dir in "/sys/fs/cgroup/cpu,cpuacct/$rel" "/sys/fs/cgroup/cpuacct,cpu/$rel" "/sys/fs/cgroup/cpuacct/$rel"; do
            if [ -f "$v1_dir/cpuacct.usage" ]; then
                usage_ns=$(cat "$v1_dir/cpuacct.usage" 2>/dev/null || true)
                if [ -n "$usage_ns" ] && [ "$usage_ns" -ge 0 ] 2>/dev/null; then
                    printf '%s|%s\n' "$rel" "$((usage_ns / 1000))"
                    break
                fi
            fi
        done
    fi
done <<< "$unique_cgs"
echo "===CG_CPU_END==="
`)
	return b.String()
}

// GetHotContainers discovers running containers using detected runtimes on myTarget,
// optionally filters them by regex against container/pod/namespace names, ranks them by
// cumulative CPU usage, and returns up to maxContainers with unique display names.
func GetHotContainers(myTarget target.Target, runtimes []DetectedRuntime, maxContainers int, filter string, localTempDir string, noRoot bool) ([]Container, error) {
	if len(runtimes) == 0 {
		return nil, fmt.Errorf("no active container runtime (docker, crictl, ctr, nerdctl) detected on target")
	}
	if maxContainers <= 0 {
		return nil, fmt.Errorf("maxContainers must be greater than 0")
	}
	var reFilter *regexp.Regexp
	if filter != "" {
		var err error
		reFilter, err = regexp.Compile(filter)
		if err != nil {
			return nil, fmt.Errorf("invalid filter regular expression: %w", err)
		}
	}
	discoverScript := script.ScriptDefinition{
		Name:           "discover_containers",
		ScriptTemplate: buildDiscoverContainersScript(runtimes),
		Superuser:      !noRoot,
	}
	output, err := workflow.RunScript(myTarget, discoverScript, localTempDir, noRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to discover containers: %w", err)
	}
	containers := parseContainerDiscoveryOutput(output.Stdout)
	containers = filterContainersByRegex(containers, reFilter)
	containers = selectTopContainers(containers, maxContainers)
	if len(containers) == 0 {
		return nil, errNoContainersFound
	}
	containers = assignContainerDisplayNames(containers, nil)
	var names []string
	for _, c := range containers {
		names = append(names, fmt.Sprintf("%s->%s", c.DisplayName, c.CgroupPath))
	}
	slog.Debug("Hot Containers", slog.String("containers", strings.Join(names, ", ")))
	return containers, nil
}

type discoveredContainer struct {
	Container
	fallbackCgroupPath string
	isSandbox          bool
}

type ociInspectEntry struct {
	ID    string `json:"Id"`
	Name  string `json:"Name"`
	State struct {
		PID int `json:"Pid"`
	} `json:"State"`
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

type criContainerStatus struct {
	ID       string `json:"id"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Labels map[string]string `json:"labels"`
	State  string            `json:"state"`
}

type crictlPSOutput struct {
	Containers []criContainerStatus `json:"containers"`
}

type crictlInspectEntry struct {
	Status criContainerStatus `json:"status"`
	Info   struct {
		PID         int    `json:"pid"`
		CgroupsPath string `json:"cgroupsPath"`
		RuntimeSpec struct {
			Linux struct {
				CgroupsPath string `json:"cgroupsPath"`
			} `json:"linux"`
		} `json:"runtimeSpec"`
	} `json:"info"`
}

type ctrContainerInfo struct {
	ID     string            `json:"ID"`
	Labels map[string]string `json:"Labels"`
}

type discoverySections struct {
	docker        string
	crictlPS      string
	crictlInspect string
	ctr           string
	nerdctl       string
	cgCPU         string
	pidCgroups    map[string]string
	cidCgroups    map[string]string
}

type sectionSpec struct {
	endMarker string
	target    *[]string
}

func parsePrefixedMarker(trimmed string, prefix string) (string, bool) {
	if !strings.HasSuffix(trimmed, "===") {
		return "", false
	}
	after, ok := strings.CutPrefix(trimmed, prefix)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(strings.TrimSuffix(after, "===")), true
}

func parseDiscoverySections(stdout string) discoverySections {
	sections := discoverySections{
		pidCgroups: make(map[string]string),
		cidCgroups: make(map[string]string),
	}
	var dockerLines, crictlPSLines, crictlInspLines, ctrLines, nerdctlLines, cgCPULines []string
	blocks := map[string]*sectionSpec{
		"===DOCKER_BEGIN===":         {endMarker: "===DOCKER_END===", target: &dockerLines},
		"===CRICTL_PS_BEGIN===":      {endMarker: "===CRICTL_PS_END===", target: &crictlPSLines},
		"===CRICTL_INSPECT_BEGIN===": {endMarker: "===CRICTL_INSPECT_END===", target: &crictlInspLines},
		"===CTR_BEGIN===":            {endMarker: "===CTR_END===", target: &ctrLines},
		"===NERDCTL_BEGIN===":        {endMarker: "===NERDCTL_END===", target: &nerdctlLines},
		"===CG_CPU===":               {endMarker: "===CG_CPU_END===", target: &cgCPULines},
	}

	var active *sectionSpec
	var prefixMap map[string]string
	var prefixKey string
	var prefixBuf []string
	flushPrefix := func() {
		if prefixKey != "" && prefixMap != nil {
			prefixMap[prefixKey] = strings.Join(prefixBuf, "\n")
		}
		prefixMap, prefixKey, prefixBuf = nil, "", nil
	}

	for line := range strings.SplitSeq(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if active != nil {
			if trimmed == active.endMarker {
				active = nil
				continue
			}
			// Preserve nested markers such as ===CTR_TASK ...=== inside ===CTR_BEGIN===..===CTR_END===.
			*active.target = append(*active.target, line)
			continue
		}
		if spec := blocks[trimmed]; spec != nil {
			flushPrefix()
			active = spec
			continue
		}
		if key, ok := parsePrefixedMarker(trimmed, "===PID_CGROUP "); ok {
			flushPrefix()
			prefixMap, prefixKey = sections.pidCgroups, key
			continue
		}
		if key, ok := parsePrefixedMarker(trimmed, "===CID_CGROUP "); ok {
			flushPrefix()
			prefixMap, prefixKey = sections.cidCgroups, key
			continue
		}
		if prefixKey != "" {
			prefixBuf = append(prefixBuf, line)
		}
	}
	flushPrefix()

	sections.docker = strings.Join(dockerLines, "\n")
	sections.crictlPS = strings.Join(crictlPSLines, "\n")
	sections.crictlInspect = strings.Join(crictlInspLines, "\n")
	sections.ctr = strings.Join(ctrLines, "\n")
	sections.nerdctl = strings.Join(nerdctlLines, "\n")
	sections.cgCPU = strings.Join(cgCPULines, "\n")
	return sections
}

// parseContainerDiscoveryOutput parses all delimited sections emitted by the discovery
// script, resolves cgroups and CPU usage, skips sandboxes, and deduplicates across runtimes.
func parseContainerDiscoveryOutput(stdout string) []Container {
	sections := parseDiscoverySections(stdout)
	cgCPU := parseCgroupCPUUsage(sections.cgCPU)

	var candidates []discoveredContainer
	candidates = append(candidates, parseOCIInspectJSON(sections.docker, "docker")...)
	candidates = append(candidates, parseCrictlSections(sections.crictlPS, sections.crictlInspect)...)
	candidates = append(candidates, parseCtrSection(sections.ctr)...)
	candidates = append(candidates, parseOCIInspectJSON(sections.nerdctl, "containerd")...)

	var resolved []Container
	sandboxIDs := make(map[string]bool)
	for _, cand := range candidates {
		if cand.isSandbox {
			if cand.ID != "" {
				sandboxIDs[cand.ID] = true
			}
			continue
		}
		c := cand.Container
		if c.PID > 0 {
			if procContent, ok := sections.pidCgroups[strconv.Itoa(c.PID)]; ok {
				c.CgroupPath = parseContainerCgroup(procContent, c.ID)
			}
		}
		if c.CgroupPath == "" && c.ID != "" {
			if cidOut, ok := sections.cidCgroups[c.ID]; ok {
				c.CgroupPath = selectCgroupFromFindOutput(cidOut, c.ID)
			}
		}
		if c.CgroupPath == "" && strings.HasPrefix(cand.fallbackCgroupPath, "/") {
			c.CgroupPath = normalizeCgroupPath(cand.fallbackCgroupPath, c.ID)
		}
		if c.CgroupPath != "" {
			c.CPUUsage = cgCPU[c.CgroupPath]
		}
		resolved = append(resolved, c)
	}

	// Filter out any sandbox IDs that another runtime (e.g. without sandbox labels) also picked up,
	// and drop containers whose cgroup could not be resolved.
	var valid []Container
	for _, c := range resolved {
		if c.ID != "" && sandboxIDs[c.ID] {
			continue
		}
		if c.CgroupPath == "" {
			slog.Debug("skipping container without resolved cgroup", slog.String("id", c.ID), slog.String("name", c.Name), slog.String("runtime", c.Runtime))
			continue
		}
		valid = append(valid, c)
	}
	return deduplicateContainers(valid)
}

func parseCgroupCPUUsage(section string) map[string]uint64 {
	usageByCgroup := make(map[string]uint64)
	for line := range strings.SplitSeq(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		rel, usageStr, ok := strings.Cut(line, "|")
		if !ok {
			continue
		}
		rel = strings.Trim(strings.TrimSpace(rel), "/")
		usage, err := strconv.ParseUint(strings.TrimSpace(usageStr), 10, 64)
		if err != nil {
			continue
		}
		usageByCgroup[rel] = usage
	}
	return usageByCgroup
}

func decodeJSONStream[T any](section string, label string) []T {
	trimmed := strings.TrimSpace(section)
	if trimmed == "" {
		return nil
	}
	var out []T
	dec := json.NewDecoder(strings.NewReader(trimmed))
	for {
		var rawMsg json.RawMessage
		if err := dec.Decode(&rawMsg); err != nil {
			if err != io.EOF {
				slog.Debug("failed to decode json stream", slog.String("source", label), slog.String("error", err.Error()))
			}
			break
		}
		msgTrimmed := bytes.TrimSpace(rawMsg)
		if len(msgTrimmed) == 0 {
			continue
		}
		if msgTrimmed[0] == '[' {
			var batch []T
			if err := json.Unmarshal(msgTrimmed, &batch); err != nil {
				slog.Debug("failed to unmarshal json array", slog.String("source", label), slog.String("error", err.Error()))
				continue
			}
			out = append(out, batch...)
			continue
		}
		var entry T
		if err := json.Unmarshal(msgTrimmed, &entry); err != nil {
			slog.Debug("failed to unmarshal json entry", slog.String("source", label), slog.String("error", err.Error()))
			continue
		}
		out = append(out, entry)
	}
	return out
}

func parseOCIInspectJSON(section string, runtime string) []discoveredContainer {
	entries := decodeJSONStream[ociInspectEntry](section, runtime)
	if len(entries) == 0 {
		return nil
	}

	out := make([]discoveredContainer, 0, len(entries))
	for _, entry := range entries {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			continue
		}
		labels := entry.Config.Labels
		rawName := strings.TrimPrefix(strings.TrimSpace(entry.Name), "/")
		podName := strings.TrimSpace(labels[labelK8sPodName])
		namespace := strings.TrimSpace(labels[labelK8sPodNamespace])
		k8sName := strings.TrimSpace(labels[labelK8sContainerName])

		if isCRISandbox(labels) || (k8sName == "POD" && podName != "") {
			out = append(out, discoveredContainer{Container: Container{ID: id}, isSandbox: true})
			continue
		}
		name := rawName
		if k8sName != "" && podName != "" && (name == "" || strings.HasPrefix(name, "k8s_")) {
			name = k8sName
		}
		if name == "" {
			name = strings.TrimSpace(labels[labelNerdctlName])
		}
		if name == "" {
			name = strings.TrimSpace(labels[labelContainerdName])
		}
		if name == "" {
			name = shortContainerID(id)
		}
		out = append(out, discoveredContainer{
			Container: Container{
				ID:        id,
				Name:      name,
				PodName:   podName,
				Namespace: namespace,
				Runtime:   runtime,
				PID:       entry.State.PID,
			},
		})
	}
	return out
}

func containerFromCRIStatus(status criContainerStatus, pid int, fallbackCgroup string) discoveredContainer {
	name := strings.TrimSpace(status.Metadata.Name)
	if name == "" {
		name = strings.TrimSpace(status.Labels[labelK8sContainerName])
	}
	if name == "" {
		name = shortContainerID(status.ID)
	}
	return discoveredContainer{
		Container: Container{
			ID:        status.ID,
			Name:      name,
			PodName:   strings.TrimSpace(status.Labels[labelK8sPodName]),
			Namespace: strings.TrimSpace(status.Labels[labelK8sPodNamespace]),
			Runtime:   "cri",
			PID:       pid,
		},
		fallbackCgroupPath: fallbackCgroup,
		isSandbox:          isCRISandbox(status.Labels),
	}
}

func parseCrictlSections(psSection string, inspectSection string) []discoveredContainer {
	byID := make(map[string]*discoveredContainer)
	var order []string

	if strings.TrimSpace(psSection) != "" {
		var ps crictlPSOutput
		if err := json.Unmarshal([]byte(psSection), &ps); err != nil {
			slog.Debug("failed to parse crictl ps json", slog.String("error", err.Error()))
		} else {
			for _, c := range ps.Containers {
				if c.State != "" && c.State != "CONTAINER_RUNNING" {
					continue
				}
				entry := containerFromCRIStatus(c, 0, "")
				byID[c.ID] = &entry
				order = append(order, c.ID)
			}
		}
	}

	inspectEntries := parseCrictlInspectEntries(inspectSection)
	for _, insp := range inspectEntries {
		id := insp.Status.ID
		if id == "" {
			continue
		}
		fallbackCgroup := insp.Info.CgroupsPath
		if fallbackCgroup == "" {
			fallbackCgroup = insp.Info.RuntimeSpec.Linux.CgroupsPath
		}
		entry, exists := byID[id]
		if !exists {
			c := containerFromCRIStatus(insp.Status, insp.Info.PID, fallbackCgroup)
			byID[id] = &c
			order = append(order, id)
			continue
		}
		if isCRISandbox(insp.Status.Labels) {
			entry.isSandbox = true
		}
		if insp.Info.PID > 0 {
			entry.PID = insp.Info.PID
		}
		if fallbackCgroup != "" {
			entry.fallbackCgroupPath = fallbackCgroup
		}
	}

	out := make([]discoveredContainer, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out
}

func parseCrictlInspectEntries(raw string) []crictlInspectEntry {
	return decodeJSONStream[crictlInspectEntry](raw, "crictl inspect")
}

func parseCtrSection(section string) []discoveredContainer {
	var out []discoveredContainer
	var inTask bool
	var id string
	var pid int
	var jsonLines []string

	flush := func() {
		if !inTask {
			return
		}
		rawJSON := strings.TrimSpace(strings.Join(jsonLines, "\n"))
		var info ctrContainerInfo
		if rawJSON != "" {
			if err := json.Unmarshal([]byte(rawJSON), &info); err != nil {
				slog.Debug("failed to parse ctr container info json", slog.String("id", id), slog.String("error", err.Error()))
			}
		}
		isSandbox := isCRISandbox(info.Labels)
		name := strings.TrimSpace(info.Labels[labelK8sContainerName])
		if name == "" {
			name = strings.TrimSpace(info.Labels[labelNerdctlName])
		}
		if name == "" {
			name = strings.TrimSpace(info.Labels[labelContainerdName])
		}
		if name == "" {
			name = id
		}
		podName := strings.TrimSpace(info.Labels[labelK8sPodName])
		namespace := strings.TrimSpace(info.Labels[labelK8sPodNamespace])
		out = append(out, discoveredContainer{
			Container: Container{
				ID:        id,
				Name:      name,
				PodName:   podName,
				Namespace: namespace,
				Runtime:   "containerd",
				PID:       pid,
			},
			isSandbox: isSandbox,
		})
		inTask = false
		id, pid = "", 0
		jsonLines = nil
	}

	for line := range strings.SplitSeq(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "===CTR_TASK ") && strings.HasSuffix(trimmed, "===") {
			flush()
			header := strings.TrimPrefix(trimmed, "===CTR_TASK ")
			header = strings.TrimSuffix(header, "===")
			parts := strings.SplitN(header, "|", 3)
			if len(parts) == 3 {
				inTask = true
				id = strings.TrimSpace(parts[1])
				pid, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
			}
			continue
		}
		if trimmed == "===CTR_TASK_END===" {
			flush()
			continue
		}
		if inTask {
			jsonLines = append(jsonLines, line)
		}
	}
	flush()
	return out
}
