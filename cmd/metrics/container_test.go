// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseContainerCgroup(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		id       string
		expected string
	}{
		{
			name:     "cgroup v2 docker systemd",
			content:  "0::/system.slice/docker-1111111111111111.scope\n",
			id:       "1111111111111111",
			expected: "system.slice/docker-1111111111111111.scope",
		},
		{
			name:     "cgroup v2 docker cgroupfs",
			content:  "0::/docker/2222222222222222\n",
			id:       "2222222222222222",
			expected: "docker/2222222222222222",
		},
		{
			name:     "cgroup v2 k8s cri-containerd with init.scope trimmed",
			content:  "0::/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podabc.slice/cri-containerd-3333333333333333.scope/init.scope\n",
			id:       "3333333333333333",
			expected: "kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podabc.slice/cri-containerd-3333333333333333.scope",
		},
		{
			name: "cgroup v1 perf_event controller preferred",
			content: `12:pids:/kubepods/burstable/pod123/4444444444444444
11:perf_event:/kubepods/burstable/pod123/4444444444444444
2:cpu,cpuacct:/kubepods/burstable/pod123/4444444444444444
`,
			id:       "4444444444444444",
			expected: "kubepods/burstable/pod123/4444444444444444",
		},
		{
			name:     "root cgroup returns empty",
			content:  "0::/\n",
			id:       "5555",
			expected: "",
		},
		{
			name: "hybrid v1/v2 prefers perf_event even when 0:: appears first",
			content: `0::/system.slice/docker.service
11:perf_event:/docker/6666666666666666
4:cpu,cpuacct:/docker/6666666666666666
`,
			id:       "6666666666666666",
			expected: "docker/6666666666666666",
		},
		{
			name: "hybrid v1/v2 without perf_event prefers path containing container ID",
			content: `0::/system.slice/docker.service
4:cpu,cpuacct:/docker/7777777777777777
`,
			id:       "7777777777777777",
			expected: "docker/7777777777777777",
		},
		{
			name:     "private cgroupns parent traversal rejected",
			content:  "0::/../../kubepods.slice/kubepods-burstable.slice/pod1/8888888888888888\n",
			id:       "8888888888888888",
			expected: "",
		},
		{
			name: "private cgroupns 0:: rejected while valid perf_event kept",
			content: `0::/../../kubepods.slice/pod1/9999999999999999
11:perf_event:/kubepods/pod1/9999999999999999
`,
			id:       "9999999999999999",
			expected: "kubepods/pod1/9999999999999999",
		},
		{
			name:     "short ctr ID does not truncate parent segment substring",
			content:  "0::/kubepods.slice/pod123/cri-containerd-xyz.scope/init.scope\n",
			id:       "pod",
			expected: "kubepods.slice/pod123/cri-containerd-xyz.scope",
		},
		{
			name:     "short ctr ID matches exact segment",
			content:  "0::/default/app/init.scope\n",
			id:       "app",
			expected: "default/app",
		},
		{
			name:     "short ctr ID matches systemd scope suffix",
			content:  "0::/system.slice/ctr-app.scope/init.scope\n",
			id:       "app",
			expected: "system.slice/ctr-app.scope",
		},
		{
			name:     "short ctr ID matches colon suffix",
			content:  "0::/kubepods.slice/pod1/default:app/sub\n",
			id:       "app",
			expected: "kubepods.slice/pod1/default:app",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, parseContainerCgroup(tc.content, tc.id))
		})
	}
}

func TestNormalizeCgroupPathSanitization(t *testing.T) {
	assert.Equal(t, "", normalizeCgroupPath("/foo/../bar", "123456789012"))
	assert.Equal(t, "", normalizeCgroupPath("/../../kubepods/123456789012", "123456789012"))
	assert.Equal(t, "", normalizeCgroupPath("/foo/..", ""))
	assert.Equal(t, "", normalizeCgroupPath("/docker/123\n456", ""))
	assert.Equal(t, "", normalizeCgroupPath("/docker/123\r456", ""))
	assert.Equal(t, "docker/foo..bar", normalizeCgroupPath("/docker/foo..bar", ""))
}

func TestSelectCgroupFromFindOutput(t *testing.T) {
	t.Run("prefers perf_event over v2 and skips non-perf v1 controllers", func(t *testing.T) {
		findOut := `
/sys/fs/cgroup/pids/docker/aaaa1111aaaa1111
/sys/fs/cgroup/memory/docker/aaaa1111aaaa1111
/sys/fs/cgroup/systemd/docker/aaaa1111aaaa1111
/sys/fs/cgroup/cpu,cpuacct/docker/aaaa1111aaaa1111
/sys/fs/cgroup/perf_event/docker/aaaa1111aaaa1111
`
		assert.Equal(t, "docker/aaaa1111aaaa1111", selectCgroupFromFindOutput(findOut, "aaaa1111aaaa1111"))
	})

	t.Run("skips non-perf v1 controllers and falls back to stripped cpuacct", func(t *testing.T) {
		findOut := `
/sys/fs/cgroup/pids/kubepods/pod1/bbbb2222bbbb2222
/sys/fs/cgroup/blkio/kubepods/pod1/bbbb2222bbbb2222
/sys/fs/cgroup/systemd/kubepods/pod1/bbbb2222bbbb2222
/sys/fs/cgroup/cpu,cpuacct/kubepods/pod1/bbbb2222bbbb2222
`
		assert.Equal(t, "kubepods/pod1/bbbb2222bbbb2222", selectCgroupFromFindOutput(findOut, "bbbb2222bbbb2222"))
	})

	t.Run("ignores non-perf v1 controllers completely when no perf_event, v2, or cpu controller exists", func(t *testing.T) {
		findOut := `
/sys/fs/cgroup/pids/docker/cccc3333cccc3333
/sys/fs/cgroup/memory/docker/cccc3333cccc3333
/sys/fs/cgroup/devices/docker/cccc3333cccc3333
/sys/fs/cgroup/freezer/docker/cccc3333cccc3333
/sys/fs/cgroup/net_cls,net_prio/docker/cccc3333cccc3333
/sys/fs/cgroup/hugetlb/docker/cccc3333cccc3333
/sys/fs/cgroup/rdma/docker/cccc3333cccc3333
/sys/fs/cgroup/cpuset/docker/cccc3333cccc3333
/sys/fs/cgroup/misc/docker/cccc3333cccc3333
`
		assert.Equal(t, "", selectCgroupFromFindOutput(findOut, "cccc3333cccc3333"))
	})
}

func TestParseContainerDiscoveryOutputAllRuntimes(t *testing.T) {
	stdout := `===DOCKER_BEGIN===
{"Id":"aaaa1111aaaa1111aaaa1111aaaa1111","Name":"/nginx","State":{"Pid":101},"Config":{"Labels":{}}}
{"Id":"bbbb2222bbbb2222bbbb2222bbbb2222","Name":"/postgres","State":{"Pid":102},"Config":{"Labels":{}}}
===DOCKER_END===
===CRICTL_PS_BEGIN===
{
  "containers": [
    {
      "id": "cccc3333cccc3333cccc3333cccc3333",
      "metadata": {"name": "webapp"},
      "labels": {
        "io.kubernetes.pod.name": "web-pod-0",
        "io.kubernetes.pod.namespace": "prod",
        "io.kubernetes.container.name": "webapp",
        "io.cri-containerd.kind": "container"
      },
      "state": "CONTAINER_RUNNING"
    },
    {
      "id": "pppp0000pppp0000pppp0000pppp0000",
      "metadata": {"name": "pause"},
      "labels": {
        "io.kubernetes.pod.name": "web-pod-0",
        "io.kubernetes.pod.namespace": "prod",
        "io.cri-containerd.kind": "sandbox"
      },
      "state": "CONTAINER_RUNNING"
    }
  ]
}
===CRICTL_PS_END===
===CRICTL_INSPECT_BEGIN===
{
  "status": {"id": "cccc3333cccc3333cccc3333cccc3333"},
  "info": {"pid": 201}
}
===CRICTL_INSPECT_END===
===CTR_BEGIN===
===CTR_TASK k8s.io|cccc3333cccc3333cccc3333cccc3333|201===
{
  "ID": "cccc3333cccc3333cccc3333cccc3333",
  "Labels": {
    "io.cri-containerd.kind": "container",
    "io.kubernetes.container.name": "webapp",
    "io.kubernetes.pod.name": "web-pod-0",
    "io.kubernetes.pod.namespace": "prod"
  }
}
===CTR_TASK_END===
===CTR_TASK k8s.io|pppp0000pppp0000pppp0000pppp0000|200===
{
  "ID": "pppp0000pppp0000pppp0000pppp0000",
  "Labels": {
    "io.cri-containerd.kind": "sandbox",
    "io.kubernetes.pod.name": "web-pod-0",
    "io.kubernetes.pod.namespace": "prod"
  }
}
===CTR_TASK_END===
===CTR_TASK default|dddd4444dddd4444dddd4444dddd4444|301===
{
  "ID": "dddd4444dddd4444dddd4444dddd4444",
  "Labels": {
    "nerdctl/name": "cache"
  }
}
===CTR_TASK_END===
===CTR_TASK moby|aaaa1111aaaa1111aaaa1111aaaa1111|101===
{
  "ID": "aaaa1111aaaa1111aaaa1111aaaa1111",
  "Labels": {}
}
===CTR_TASK_END===
===CTR_END===
===PID_CGROUP 101===
0::/system.slice/docker-aaaa1111aaaa1111aaaa1111aaaa1111.scope
===PID_CGROUP 102===
0::/docker/bbbb2222bbbb2222bbbb2222bbbb2222
===PID_CGROUP 201===
0::/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podabc.slice/cri-containerd-cccc3333cccc3333cccc3333cccc3333.scope/init.scope
===PID_CGROUP 301===
0::/default/dddd4444dddd4444dddd4444dddd4444
===CG_CPU===
system.slice/docker-aaaa1111aaaa1111aaaa1111aaaa1111.scope|1200000
docker/bbbb2222bbbb2222bbbb2222bbbb2222|3400000
kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podabc.slice/cri-containerd-cccc3333cccc3333cccc3333cccc3333.scope/init.scope|5000
kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podabc.slice/cri-containerd-cccc3333cccc3333cccc3333cccc3333.scope|9900000
default/dddd4444dddd4444dddd4444dddd4444|450000
===CG_CPU_END===
`

	containers := parseContainerDiscoveryOutput(stdout)
	// pause sandbox filtered, crictl+ctr deduplicated, moby+docker deduplicated
	assert.Len(t, containers, 4)

	byID := make(map[string]Container)
	for _, c := range containers {
		byID[c.ID] = c
	}

	assert.Equal(t, "nginx", byID["aaaa1111aaaa1111aaaa1111aaaa1111"].Name)
	assert.Equal(t, "docker", byID["aaaa1111aaaa1111aaaa1111aaaa1111"].Runtime)
	assert.Equal(t, "system.slice/docker-aaaa1111aaaa1111aaaa1111aaaa1111.scope", byID["aaaa1111aaaa1111aaaa1111aaaa1111"].CgroupPath)
	assert.Equal(t, uint64(1200000), byID["aaaa1111aaaa1111aaaa1111aaaa1111"].CPUUsage)

	assert.Equal(t, "postgres", byID["bbbb2222bbbb2222bbbb2222bbbb2222"].Name)
	assert.Equal(t, "docker/bbbb2222bbbb2222bbbb2222bbbb2222", byID["bbbb2222bbbb2222bbbb2222bbbb2222"].CgroupPath)
	assert.Equal(t, uint64(3400000), byID["bbbb2222bbbb2222bbbb2222bbbb2222"].CPUUsage)

	// crictl + ctr merged; init.scope trimmed and parent container cpu.stat used
	criC := byID["cccc3333cccc3333cccc3333cccc3333"]
	assert.Equal(t, "webapp", criC.Name)
	assert.Equal(t, "web-pod-0", criC.PodName)
	assert.Equal(t, "prod", criC.Namespace)
	assert.Equal(t, "cri", criC.Runtime)
	assert.Equal(t, "kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podabc.slice/cri-containerd-cccc3333cccc3333cccc3333cccc3333.scope", criC.CgroupPath)
	assert.Equal(t, uint64(9900000), criC.CPUUsage)

	// standalone ctr with nerdctl/name label (does not leak containerd "default" namespace)
	ctrC := byID["dddd4444dddd4444dddd4444dddd4444"]
	assert.Equal(t, "cache", ctrC.Name)
	assert.Equal(t, "", ctrC.Namespace)
	assert.Equal(t, "containerd", ctrC.Runtime)
	assert.Equal(t, "default/dddd4444dddd4444dddd4444dddd4444", ctrC.CgroupPath)
	assert.Equal(t, uint64(450000), ctrC.CPUUsage)

	// Regex matching "(nginx|webapp|postgres)"
	re := regexp.MustCompile("(nginx|webapp|postgres)")
	filtered := filterContainersByRegex(containers, re)
	ranked := selectTopContainers(filtered, 5)
	ranked = assignContainerDisplayNames(ranked, nil)
	assert.Len(t, ranked, 3)
	// Ranked descending by CPUUsage: webapp (9900000), postgres (3400000), nginx (1200000)
	assert.Equal(t, "prod/web-pod-0/webapp", ranked[0].DisplayName)
	assert.Equal(t, "postgres", ranked[1].DisplayName)
	assert.Equal(t, "nginx", ranked[2].DisplayName)
}

func TestParseContainerDiscoveryNerdctlAndCIDFallback(t *testing.T) {
	stdout := `===NERDCTL_BEGIN===
{"Id":"eeee5555eeee5555eeee5555eeee5555","Name":"/api-gw","State":{"Pid":0},"Config":{"Labels":{"nerdctl/name":"api-gw"}}}
{"Id":"ffff6666ffff6666ffff6666ffff6666","Name":"","State":{"Pid":401},"Config":{"Labels":{"nerdctl/name":"worker"}}}
===NERDCTL_END===
===PID_CGROUP 401===
0::/user.slice/user-1000.slice/nerdctl-ffff6666ffff6666ffff6666ffff6666.scope
===CID_CGROUP eeee5555eeee5555eeee5555eeee5555===
/sys/fs/cgroup/system.slice/containerd-eeee5555eeee5555eeee5555eeee5555.scope
/sys/fs/cgroup/system.slice/containerd-eeee5555eeee5555eeee5555eeee5555.scope/init.scope
===CG_CPU===
system.slice/containerd-eeee5555eeee5555eeee5555eeee5555.scope|800000
user.slice/user-1000.slice/nerdctl-ffff6666ffff6666ffff6666ffff6666.scope|250000
===CG_CPU_END===
`
	containers := parseContainerDiscoveryOutput(stdout)
	assert.Len(t, containers, 2)
	ranked := assignContainerDisplayNames(selectTopContainers(containers, 5), nil)
	assert.Equal(t, "api-gw", ranked[0].DisplayName)
	assert.Equal(t, "", ranked[0].Namespace)
	assert.Equal(t, "system.slice/containerd-eeee5555eeee5555eeee5555eeee5555.scope", ranked[0].CgroupPath)
	assert.Equal(t, "worker", ranked[1].DisplayName)
	assert.Equal(t, "user.slice/user-1000.slice/nerdctl-ffff6666ffff6666ffff6666ffff6666.scope", ranked[1].CgroupPath)
}

func TestParseOCIInspectAndCtrEdgeCases(t *testing.T) {
	t.Run("docker and nerdctl json inspect handle special chars and sandboxes", func(t *testing.T) {
		section := `{"Id":"1111aaaa1111aaaa","Name":"/web|\"app\"\nline2","State":{"Pid":123},"Config":{"Labels":{"com.example.note":"a|b\nc\"d"}}}
{"Id":"2222bbbb2222bbbb","Name":"/k8s_POD_web-0_prod_uid_0","State":{"Pid":124},"Config":{"Labels":{"io.kubernetes.pod.name":"web-0","io.kubernetes.pod.namespace":"prod","io.kubernetes.container.name":"POD"}}}
{"Id":"3333cccc3333cccc","Name":"/sandbox-cri","State":{"Pid":125},"Config":{"Labels":{"io.cri-containerd.kind":"sandbox"}}}
`
		parsed := parseOCIInspectJSON(section, "docker")
		require.Len(t, parsed, 3)
		assert.False(t, parsed[0].isSandbox)
		assert.Equal(t, "web|\"app\"\nline2", parsed[0].Name)
		assert.Equal(t, 123, parsed[0].PID)
		assert.Equal(t, "docker", parsed[0].Runtime)
		assert.True(t, parsed[1].isSandbox)
		assert.True(t, parsed[2].isSandbox)

		assigned := assignContainerDisplayNames([]Container{parsed[0].Container}, nil)
		assert.Equal(t, "web__app__line2", assigned[0].DisplayName)
	})

	t.Run("ctr does not leak containerd namespace into k8s namespace", func(t *testing.T) {
		section := `===CTR_TASK k8s.io|c1|501===
{"ID":"c1","Labels":{"io.kubernetes.container.name":"app"}}
===CTR_TASK_END===
===CTR_TASK moby|c2|502===
{"ID":"c2","Labels":{"nerdctl/name":"db"}}
===CTR_TASK_END===
===CTR_TASK k8s.io|c3|503===
{"ID":"c3","Labels":{"io.kubernetes.pod.name":"web-0","io.kubernetes.pod.namespace":"payments","io.kubernetes.container.name":"api"}}
===CTR_TASK_END===
`
		parsed := parseCtrSection(section)
		require.Len(t, parsed, 3)
		assert.Equal(t, "", parsed[0].Namespace)
		assert.Equal(t, "", parsed[1].Namespace)
		assert.Equal(t, "payments", parsed[2].Namespace)
	})

	t.Run("sanitizeContainerDisplayName strips quotes and pipes", func(t *testing.T) {
		assert.Equal(t, "a_b_c_d_e_f", sanitizeContainerDisplayName(`a"b|c,d	e`+"\nf"))
	})
}

func TestFilterContainersByRegex(t *testing.T) {
	containers := []Container{
		{ID: "1", Name: "nginx", CgroupPath: "docker/1"},
		{ID: "2", Name: "nginx-exporter", CgroupPath: "docker/2"},
		{ID: "3", Name: "webapp", PodName: "checkout-0", Namespace: "prod", CgroupPath: "cri/3"},
		{ID: "4", Name: "redis", PodName: "cache-0", Namespace: "staging", CgroupPath: "cri/4"},
	}

	t.Run("alternation", func(t *testing.T) {
		re := regexp.MustCompile("(nginx|webapp|postgres)")
		matched := filterContainersByRegex(containers, re)
		assert.Len(t, matched, 3)
	})

	t.Run("anchored container name", func(t *testing.T) {
		re := regexp.MustCompile("^nginx$")
		matched := filterContainersByRegex(containers, re)
		assert.Len(t, matched, 1)
		assert.Equal(t, "nginx", matched[0].Name)
	})

	t.Run("pod slash container", func(t *testing.T) {
		re := regexp.MustCompile(`^checkout-0/webapp$`)
		matched := filterContainersByRegex(containers, re)
		assert.Len(t, matched, 1)
		assert.Equal(t, "3", matched[0].ID)
	})

	t.Run("namespace slash pod slash container", func(t *testing.T) {
		re := regexp.MustCompile(`^staging/cache-0/redis$`)
		matched := filterContainersByRegex(containers, re)
		assert.Len(t, matched, 1)
		assert.Equal(t, "4", matched[0].ID)
	})
}

func TestAssignContainerDisplayNamesDisambiguation(t *testing.T) {
	t.Run("mixed namespaces and standalone", func(t *testing.T) {
		containers := []Container{
			{ID: "111111111111aaaa", Name: "nginx", CgroupPath: "docker/1"},
			{ID: "222222222222bbbb", Name: "webapp", PodName: "pod-a", Namespace: "default", CgroupPath: "cri/2"},
			{ID: "333333333333cccc", Name: "webapp", PodName: "pod-b", Namespace: "default", CgroupPath: "cri/3"},
			{ID: "444444444444dddd", Name: "sidecar", PodName: "app-0", Namespace: "ns1", CgroupPath: "cri/4"},
			{ID: "555555555555eeee", Name: "sidecar", PodName: "app-0", Namespace: "ns2", CgroupPath: "cri/5"},
			{ID: "666666666666ffff", Name: "worker", Namespace: "ns1", CgroupPath: "ctr/6"},
			{ID: "7777777777770000", Name: "worker", Namespace: "ns2", CgroupPath: "ctr/7"},
		}

		assigned := assignContainerDisplayNames(containers, nil)
		assert.Equal(t, "nginx", assigned[0].DisplayName)
		assert.Equal(t, "default/pod-a/webapp", assigned[1].DisplayName)
		assert.Equal(t, "default/pod-b/webapp", assigned[2].DisplayName)
		assert.Equal(t, "ns1/app-0/sidecar", assigned[3].DisplayName)
		assert.Equal(t, "ns2/app-0/sidecar", assigned[4].DisplayName)
		assert.Equal(t, "worker (666666666666)", assigned[5].DisplayName)
		assert.Equal(t, "worker (777777777777)", assigned[6].DisplayName)
	})

	t.Run("all kubernetes pods in default namespace preserve namespace prefix", func(t *testing.T) {
		containers := []Container{
			{ID: "1111", Name: "nginx", PodName: "nginx-pod", Namespace: "default", CgroupPath: "kubepods/1"},
			{ID: "2222", Name: "webapp", PodName: "webapp-788bd774ff-5spvq", Namespace: "default", CgroupPath: "kubepods/2"},
			{ID: "3333", Name: "webapp", PodName: "webapp-788bd774ff-qpc66", Namespace: "default", CgroupPath: "kubepods/3"},
			{ID: "4444", Name: "postgres", PodName: "postgres-pod", Namespace: "default", CgroupPath: "kubepods/4"},
			{ID: "5555", Name: "docker-sidecar", CgroupPath: "docker/5"},
		}

		assigned := assignContainerDisplayNames(containers, nil)
		assert.Equal(t, "default/nginx-pod/nginx", assigned[0].DisplayName)
		assert.Equal(t, "default/webapp-788bd774ff-5spvq/webapp", assigned[1].DisplayName)
		assert.Equal(t, "default/webapp-788bd774ff-qpc66/webapp", assigned[2].DisplayName)
		assert.Equal(t, "default/postgres-pod/postgres", assigned[3].DisplayName)
		assert.Equal(t, "docker-sidecar", assigned[4].DisplayName)
	})

	t.Run("cross-refresh stability prevents label stealing and preserves existing mappings", func(t *testing.T) {
		existing := map[string]string{
			"kubepods/1": "default/nginx-pod/nginx",
			"docker/2":   "cache",
		}
		refreshed := []Container{
			{ID: "1111", Name: "nginx", PodName: "nginx-pod", Namespace: "default", CgroupPath: "kubepods/1"},
			// Recreated pod container with same ns/pod/name at a new cgroup path
			{ID: "9999aaaaaaaa1111", Name: "nginx", PodName: "nginx-pod", Namespace: "default", CgroupPath: "kubepods/1-recreated"},
			// Recreated standalone container even if old docker/2 is no longer in the hot list
			{ID: "8888bbbbbbbb2222", Name: "cache", CgroupPath: "docker/2-new"},
			// New pod in another namespace
			{ID: "7777cccccccc3333", Name: "api", PodName: "api-0", Namespace: "prod", CgroupPath: "kubepods/3"},
		}
		assigned := assignContainerDisplayNames(refreshed, existing)
		assert.Equal(t, "default/nginx-pod/nginx", assigned[0].DisplayName)
		assert.Equal(t, "default/nginx-pod/nginx (9999aaaaaaaa)", assigned[1].DisplayName)
		assert.Equal(t, "cache (8888bbbbbbbb)", assigned[2].DisplayName)
		assert.Equal(t, "prod/api-0/api", assigned[3].DisplayName)
	})
}

func TestSelectTopContainersDeterministicAndCapped(t *testing.T) {
	containers := []Container{
		{ID: "c", Name: "charlie", CPUUsage: 100},
		{ID: "a", Name: "alpha", CPUUsage: 100},
		{ID: "b", Name: "bravo", CPUUsage: 500},
		{ID: "d", Name: "delta", CPUUsage: 50},
	}
	top := selectTopContainers(containers, 3)
	assert.Len(t, top, 3)
	assert.Equal(t, "bravo", top[0].Name)
	assert.Equal(t, "alpha", top[1].Name)
	assert.Equal(t, "charlie", top[2].Name)
}

func TestParseContainerRuntimeSpec(t *testing.T) {
	validTests := []struct {
		name     string
		raw      string
		expected ContainerRuntimeSpec
	}{
		{name: "auto", raw: "auto", expected: ContainerRuntimeSpec{Kind: "auto"}},
		{name: "docker", raw: "docker", expected: ContainerRuntimeSpec{Kind: "docker"}},
		{name: "crictl", raw: "crictl", expected: ContainerRuntimeSpec{Kind: "crictl"}},
		{name: "ctr", raw: "ctr", expected: ContainerRuntimeSpec{Kind: "ctr"}},
		{name: "nerdctl", raw: "nerdctl", expected: ContainerRuntimeSpec{Kind: "nerdctl"}},
		{name: "absolute crictl path", raw: "/home/kubernetes/bin/crictl", expected: ContainerRuntimeSpec{Kind: "crictl", BinaryPath: "/home/kubernetes/bin/crictl"}},
		{name: "absolute docker path", raw: "/usr/local/bin/docker", expected: ContainerRuntimeSpec{Kind: "docker", BinaryPath: "/usr/local/bin/docker"}},
		{name: "absolute ctr path", raw: "/usr/bin/ctr", expected: ContainerRuntimeSpec{Kind: "ctr", BinaryPath: "/usr/bin/ctr"}},
		{name: "absolute nerdctl path", raw: "/opt/bin/nerdctl", expected: ContainerRuntimeSpec{Kind: "nerdctl", BinaryPath: "/opt/bin/nerdctl"}},
	}
	for _, tc := range validTests {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			spec, err := parseContainerRuntimeSpec(tc.raw)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, spec)
		})
	}

	invalidTests := []struct {
		name string
		raw  string
	}{
		{name: "empty", raw: ""},
		{name: "unknown runtime", raw: "podman"},
		{name: "relative path", raw: "bin/crictl"},
		{name: "dot-slash relative path", raw: "./docker"},
		{name: "dotdot traversal", raw: "/usr/bin/../bin/docker"},
		{name: "trailing slash", raw: "/usr/bin/docker/"},
		{name: "shell semicolon", raw: "/usr/bin/docker;id"},
		{name: "spaces", raw: "/usr/bin/docker --help"},
		{name: "command substitution", raw: "/usr/bin/$(id)/docker"},
		{name: "unsupported basename", raw: "/usr/bin/podman"},
		{name: "root slash", raw: "/"},
	}
	for _, tc := range invalidTests {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			_, err := parseContainerRuntimeSpec(tc.raw)
			assert.Error(t, err)
		})
	}
}

func TestParseDetectedRuntimesAndValidation(t *testing.T) {
	stdout := `
RUNTIME|docker|/usr/bin/docker|
RUNTIME|crictl|/home/kubernetes/bin/crictl|unix:///run/containerd/containerd.sock
RUNTIME|ctr|/usr/bin/ctr|/run/containerd/containerd.sock
RUNTIME|ctr|/usr/local/bin/ctr|/var/run/containerd/containerd.sock
RUNTIME|unknown|/usr/bin/foo|
RUNTIME|malformed
RUNTIME|nerdctl||
`
	detected := parseDetectedRuntimes(stdout)
	assert.Equal(t, []DetectedRuntime{
		{Kind: "docker", BinaryPath: "/usr/bin/docker", Endpoint: ""},
		{Kind: "crictl", BinaryPath: "/home/kubernetes/bin/crictl", Endpoint: "unix:///run/containerd/containerd.sock"},
		{Kind: "ctr", BinaryPath: "/usr/bin/ctr", Endpoint: "/run/containerd/containerd.sock"},
	}, detected)

	err := validateDetectedRuntimes(nil, ContainerRuntimeSpec{Kind: "auto"}, "auto")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active container runtime (docker, crictl, ctr, nerdctl) detected on target")

	err = validateDetectedRuntimes(nil, ContainerRuntimeSpec{Kind: "crictl"}, "crictl")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `configured container runtime "crictl" is not available or not running on target`)

	err = validateDetectedRuntimes(nil, ContainerRuntimeSpec{Kind: "crictl", BinaryPath: "/home/kubernetes/bin/crictl"}, "/home/kubernetes/bin/crictl")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `configured container runtime "/home/kubernetes/bin/crictl" is not available or not running on target`)
}

func TestBuildDiscoverContainersScript(t *testing.T) {
	t.Run("docker only", func(t *testing.T) {
		script := buildDiscoverContainersScript([]DetectedRuntime{
			{Kind: "docker", BinaryPath: "/usr/bin/docker"},
		})
		assert.Contains(t, script, "docker_bin='/usr/bin/docker'")
		assert.Contains(t, script, "===DOCKER_BEGIN===")
		assert.NotContains(t, script, "===CRICTL_PS_BEGIN===")
		assert.NotContains(t, script, "===CTR_BEGIN===")
		assert.NotContains(t, script, "===NERDCTL_BEGIN===")
	})

	t.Run("crictl only with runtime endpoint", func(t *testing.T) {
		script := buildDiscoverContainersScript([]DetectedRuntime{
			{Kind: "crictl", BinaryPath: "/home/kubernetes/bin/crictl", Endpoint: "unix:///run/containerd/containerd.sock"},
		})
		assert.Contains(t, script, "'/home/kubernetes/bin/crictl' --runtime-endpoint 'unix:///run/containerd/containerd.sock' ps -o json")
		assert.Contains(t, script, "===CRICTL_PS_BEGIN===")
		assert.NotContains(t, script, "===DOCKER_BEGIN===")
		assert.NotContains(t, script, "===CTR_BEGIN===")
		assert.NotContains(t, script, "===NERDCTL_BEGIN===")
	})

	t.Run("auto emits all runtimes with ctr and crictl guards on nerdctl", func(t *testing.T) {
		script := buildDiscoverContainersScript([]DetectedRuntime{
			{Kind: "docker", BinaryPath: "/usr/bin/docker"},
			{Kind: "crictl", BinaryPath: "/home/kubernetes/bin/crictl", Endpoint: "unix:///run/containerd/containerd.sock"},
			{Kind: "ctr", BinaryPath: "/usr/bin/ctr", Endpoint: "/run/containerd/containerd.sock"},
			{Kind: "nerdctl", BinaryPath: "/usr/local/bin/nerdctl"},
		})
		assert.Contains(t, script, "crictl_ok=1")
		assert.Contains(t, script, `'/usr/bin/ctr' --address '/run/containerd/containerd.sock' namespaces list -q`)
		assert.Contains(t, script, `if [ "$docker_ok" -eq 1 ] && [ "$ns" = "moby" ]; then`)
		assert.Contains(t, script, `if [ "$crictl_ok" -eq 1 ] && [ "$ns" = "k8s.io" ]; then`)
		assert.Contains(t, script, `if [ "$ctr_ok" -eq 0 ]; then`)
		assert.Contains(t, script, "===NERDCTL_BEGIN===")
		assert.Contains(t, script, "inspect --format '{{json .}}'")
		assert.Contains(t, script, "inspect --format '{{.Id}}|{{.State.Pid}}'")
	})

	t.Run("nerdctl only", func(t *testing.T) {
		script := buildDiscoverContainersScript([]DetectedRuntime{
			{Kind: "nerdctl", BinaryPath: "/usr/local/bin/nerdctl"},
		})
		assert.Contains(t, script, "nerdctl_bin='/usr/local/bin/nerdctl'")
		assert.Contains(t, script, "===NERDCTL_BEGIN===")
		assert.NotContains(t, script, "===CTR_BEGIN===")
	})
}

func newTestMetricsCmd(t *testing.T) *cobra.Command {
	t.Helper()
	origDuration := flagDuration
	origScope := flagScope
	origPidList := flagPidList
	origCidList := flagCidList
	origContainerRuntime := flagContainerRuntime
	origFilter := flagFilter
	origCount := flagCount
	origRefresh := flagRefresh
	origCpuRange := flagCpuRange
	origGranularity := flagGranularity
	origOutputFormat := flagOutputFormat
	origLive := flagLive
	origTxnRate := flagTransactionRate
	origList := flagShowMetricNames
	origMetrics := flagMetricsList
	origEventFile := flagEventFilePath
	origMetricFile := flagMetricFilePath
	origInterval := flagPerfPrintInterval
	origMuxInterval := flagPerfMuxInterval
	origNoRoot := flagNoRoot
	origWriteEvents := flagWriteEventsToFile
	origInput := flagInput
	origNoSummary := flagNoSystemSummary
	origPromServer := flagPrometheusServer
	origPromAddr := flagPrometheusServerAddr
	origWorkload := argsWorkload

	t.Cleanup(func() {
		flagDuration = origDuration
		flagScope = origScope
		flagPidList = origPidList
		flagCidList = origCidList
		flagContainerRuntime = origContainerRuntime
		flagFilter = origFilter
		flagCount = origCount
		flagRefresh = origRefresh
		flagCpuRange = origCpuRange
		flagGranularity = origGranularity
		flagOutputFormat = origOutputFormat
		flagLive = origLive
		flagTransactionRate = origTxnRate
		flagShowMetricNames = origList
		flagMetricsList = origMetrics
		flagEventFilePath = origEventFile
		flagMetricFilePath = origMetricFile
		flagPerfPrintInterval = origInterval
		flagPerfMuxInterval = origMuxInterval
		flagNoRoot = origNoRoot
		flagWriteEventsToFile = origWriteEvents
		flagInput = origInput
		flagNoSystemSummary = origNoSummary
		flagPrometheusServer = origPromServer
		flagPrometheusServerAddr = origPromAddr
		argsWorkload = origWorkload
	})

	testCmd := &cobra.Command{Use: "metrics"}
	var stderr bytes.Buffer
	testCmd.SetErr(&stderr)
	registerMetricsFlags(testCmd)
	argsWorkload = nil
	return testCmd
}

func TestValidateFlagsContainerRuntime(t *testing.T) {
	t.Run("container-runtime implies scope container", func(t *testing.T) {
		testCmd := newTestMetricsCmd(t)
		require.NoError(t, testCmd.Flags().Set(flagContainerRuntimeName, "crictl"))
		require.NoError(t, testCmd.Flags().Set(flagFilterName, "(nginx|webapp)"))

		err := validateFlags(testCmd, nil)
		require.NoError(t, err)
		assert.Equal(t, scopeContainer, flagScope)
	})

	t.Run("explicit scope container with custom binary path", func(t *testing.T) {
		testCmd := newTestMetricsCmd(t)
		require.NoError(t, testCmd.Flags().Set(flagScopeName, scopeContainer))
		require.NoError(t, testCmd.Flags().Set(flagContainerRuntimeName, "/home/kubernetes/bin/crictl"))

		err := validateFlags(testCmd, nil)
		require.NoError(t, err)
		assert.Equal(t, scopeContainer, flagScope)
	})

	t.Run("conflict with explicit non-container scope", func(t *testing.T) {
		testCmd := newTestMetricsCmd(t)
		require.NoError(t, testCmd.Flags().Set(flagScopeName, scopeSystem))
		require.NoError(t, testCmd.Flags().Set(flagContainerRuntimeName, "crictl"))

		err := validateFlags(testCmd, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify container-runtime when scope is not container")
	})

	t.Run("conflict with pids", func(t *testing.T) {
		testCmd := newTestMetricsCmd(t)
		require.NoError(t, testCmd.Flags().Set(flagPidListName, "1234"))
		require.NoError(t, testCmd.Flags().Set(flagContainerRuntimeName, "docker"))

		err := validateFlags(testCmd, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify container-runtime when pids or cids are specified")
	})

	t.Run("conflict with cids", func(t *testing.T) {
		testCmd := newTestMetricsCmd(t)
		require.NoError(t, testCmd.Flags().Set(flagCidListName, "my-cgroup"))
		require.NoError(t, testCmd.Flags().Set(flagContainerRuntimeName, "docker"))

		err := validateFlags(testCmd, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify container-runtime when pids or cids are specified")
	})

	t.Run("invalid container-runtime value", func(t *testing.T) {
		testCmd := newTestMetricsCmd(t)
		require.NoError(t, testCmd.Flags().Set(flagContainerRuntimeName, "podman"))

		err := validateFlags(testCmd, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid container runtime")
	})

	t.Run("disallowed with workload", func(t *testing.T) {
		testCmd := newTestMetricsCmd(t)
		require.NoError(t, testCmd.Flags().Set(flagContainerRuntimeName, "docker"))

		err := validateFlags(testCmd, []string{"/bin/true"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "container-runtime is not supported with a workload")
	})
}

func TestGeneratedContainerScriptsBashSyntaxAndExecution(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	specs := []ContainerRuntimeSpec{
		{Kind: containerRuntimeAuto},
		{Kind: runtimeKindDocker},
		{Kind: runtimeKindCrictl, BinaryPath: "/home/kubernetes/bin/crictl"},
		{Kind: runtimeKindCtr, BinaryPath: "/usr/bin/ctr"},
		{Kind: runtimeKindNerdctl, BinaryPath: "/usr/local/bin/nerdctl"},
	}
	for _, spec := range specs {
		cmd := exec.Command("bash", "-n")
		cmd.Stdin = strings.NewReader(buildDetectContainerRuntimesScript(spec))
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "bash -n failed for detect script (%+v): %s", spec, string(out))
	}

	discoverScript := buildDiscoverContainersScript([]DetectedRuntime{
		{Kind: runtimeKindDocker, BinaryPath: "/usr/bin/docker"},
		{Kind: runtimeKindCrictl, BinaryPath: "/home/kubernetes/bin/crictl", Endpoint: "unix:///run/containerd/containerd.sock"},
		{Kind: runtimeKindCtr, BinaryPath: "/usr/bin/ctr", Endpoint: "/run/containerd/containerd.sock"},
		{Kind: runtimeKindNerdctl, BinaryPath: "/usr/local/bin/nerdctl"},
	})
	cmd := exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(discoverScript)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "bash -n failed for discover script (all 4 runtimes): %s", string(out))

	nerdctlOnlyDiscover := buildDiscoverContainersScript([]DetectedRuntime{
		{Kind: runtimeKindNerdctl, BinaryPath: "/usr/local/bin/nerdctl"},
	})
	cmdNerd := exec.Command("bash", "-n")
	cmdNerd.Stdin = strings.NewReader(nerdctlOnlyDiscover)
	out, err = cmdNerd.CombinedOutput()
	require.NoError(t, err, "bash -n failed for nerdctl-only discover script: %s", string(out))

	// Execute detect script against mock docker (inactive daemon) and mock crictl (active) binaries.
	tmpDir := t.TempDir()
	mockDocker := filepath.Join(tmpDir, "docker")
	mockCrictl := filepath.Join(tmpDir, "crictl")
	require.NoError(t, os.WriteFile(mockDocker, []byte("#!/bin/sh\nexit 1\n"), 0755))
	require.NoError(t, os.WriteFile(mockCrictl, []byte("#!/bin/sh\nif [ \"$1\" = \"ps\" ]; then exit 0; fi\nexit 1\n"), 0755))

	runDockerCmd := exec.Command("bash", "-c", buildDetectContainerRuntimesScript(ContainerRuntimeSpec{
		Kind:       runtimeKindDocker,
		BinaryPath: mockDocker,
	}))
	dockerOut, err := runDockerCmd.CombinedOutput()
	require.NoError(t, err)
	assert.Empty(t, parseDetectedRuntimes(string(dockerOut)), "inactive docker daemon should not be detected")

	runCrictlCmd := exec.Command("bash", "-c", buildDetectContainerRuntimesScript(ContainerRuntimeSpec{
		Kind:       runtimeKindCrictl,
		BinaryPath: mockCrictl,
	}))
	crictlOut, err := runCrictlCmd.CombinedOutput()
	require.NoError(t, err)
	assert.Equal(t, []DetectedRuntime{{Kind: runtimeKindCrictl, BinaryPath: mockCrictl}}, parseDetectedRuntimes(string(crictlOut)))

	// Execute discovery script with a mock crictl binary where batch inspect only returns 1 of 2 IDs,
	// testing both -o go-template PID recovery and -o json per-ID inspect fallback.
	livePID := os.Getpid()
	mockPartialCrictl := filepath.Join(tmpDir, "crictl-partial")
	partialScript := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "ps" ] && [ "$2" = "-o" ]; then
  echo '{"containers":[{"id":"cid_aaa","metadata":{"name":"app-a"},"labels":{"io.kubernetes.pod.name":"pod-a","io.kubernetes.pod.namespace":"prod"},"state":"CONTAINER_RUNNING"},{"id":"cid_bbb","metadata":{"name":"app-b"},"labels":{"io.kubernetes.pod.name":"pod-b","io.kubernetes.pod.namespace":"prod"},"state":"CONTAINER_RUNNING"}]}'
  exit 0
fi
if [ "$1" = "ps" ] && [ "$2" = "-q" ]; then
  echo "cid_aaa"
  echo "cid_bbb"
  exit 0
fi
if [ "$1" = "inspect" ] && [ "$2" = "-o" ] && [ "$3" = "json" ]; then
  if [ "$#" -gt 4 ]; then
    echo '[{"status":{"id":"cid_aaa","annotations":{"note":"\"pid\": 99999"}},"info":{"pid":0}}]'
    exit 0
  fi
  if [ "$4" = "cid_bbb" ]; then
    echo '{"status":{"id":"cid_bbb"},"info":{"pid":0}}'
    exit 0
  fi
fi
if [ "$1" = "inspect" ] && [ "$2" = "-o" ] && [ "$3" = "go-template" ]; then
  if [ "$#" -gt 6 ]; then
    echo "cid_aaa|0"
    exit 0
  fi
  if [ "$6" = "cid_bbb" ]; then
    echo "cid_bbb|%d"
    exit 0
  fi
fi
exit 1
`, livePID)
	require.NoError(t, os.WriteFile(mockPartialCrictl, []byte(partialScript), 0755))
	discoverPartial := buildDiscoverContainersScript([]DetectedRuntime{
		{Kind: runtimeKindCrictl, BinaryPath: mockPartialCrictl},
	})
	partialCmd := exec.Command("bash", "-c", discoverPartial)
	partialOut, err := partialCmd.CombinedOutput()
	require.NoError(t, err, "partial crictl discover script failed: %s", string(partialOut))
	assert.NotContains(t, string(partialOut), "===PID_CGROUP 99999===", "escaped pid inside JSON string must not be treated as host PID")
	sections := parseDiscoverySections(string(partialOut))
	entries := parseCrictlInspectEntries(sections.crictlInspect)
	require.Len(t, entries, 2)
	assert.Equal(t, "cid_aaa", entries[0].Status.ID)
	assert.Equal(t, "cid_bbb", entries[1].Status.ID)
	if _, err := os.Stat("/proc/self/cgroup"); err == nil {
		assert.Contains(t, sections.pidCgroups, strconv.Itoa(livePID), "per-ID go-template inspect should resolve live PID")
	}

	// Verify the fallback path when crictl does not support -o go-template.
	mockLegacyCrictl := filepath.Join(tmpDir, "crictl-legacy")
	legacyScript := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "ps" ] && [ "$2" = "-o" ]; then
  echo '{"containers":[{"id":"cid_aaa","metadata":{"name":"app-a"},"labels":{"io.kubernetes.pod.name":"pod-a","io.kubernetes.pod.namespace":"prod"},"state":"CONTAINER_RUNNING"},{"id":"cid_bbb","metadata":{"name":"app-b"},"labels":{"io.kubernetes.pod.name":"pod-b","io.kubernetes.pod.namespace":"prod"},"state":"CONTAINER_RUNNING"}]}'
  exit 0
fi
if [ "$1" = "ps" ] && [ "$2" = "-q" ]; then
  echo "cid_aaa"
  echo "cid_bbb"
  exit 0
fi
if [ "$1" = "inspect" ] && [ "$2" = "-o" ] && [ "$3" = "json" ]; then
  echo '[{"status":{"id":"cid_aaa","annotations":{"note":"\"pid\": 99999"}},"info":{"pid":0}},{"status":{"id":"cid_bbb"},"info":{"pid":%d}}]'
  exit 0
fi
exit 1
`, livePID)
	require.NoError(t, os.WriteFile(mockLegacyCrictl, []byte(legacyScript), 0755))
	discoverLegacy := buildDiscoverContainersScript([]DetectedRuntime{
		{Kind: runtimeKindCrictl, BinaryPath: mockLegacyCrictl},
	})
	legacyCmd := exec.Command("bash", "-c", discoverLegacy)
	legacyOut, err := legacyCmd.CombinedOutput()
	require.NoError(t, err, "legacy crictl discover script failed: %s", string(legacyOut))
	assert.NotContains(t, string(legacyOut), "===PID_CGROUP 99999===", "escaped pid inside JSON string must not be treated as host PID in grep fallback")
	legacySections := parseDiscoverySections(string(legacyOut))
	legacyEntries := parseCrictlInspectEntries(legacySections.crictlInspect)
	require.Len(t, legacyEntries, 2)
	assert.Equal(t, "cid_aaa", legacyEntries[0].Status.ID)
	assert.Equal(t, "cid_bbb", legacyEntries[1].Status.ID)
	if _, err := os.Stat("/proc/self/cgroup"); err == nil {
		assert.Contains(t, legacySections.pidCgroups, strconv.Itoa(livePID), "compact grep fallback should extract PID from crictl inspect JSON")
	}
}
