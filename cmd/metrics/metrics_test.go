// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateFlagsRefresh(t *testing.T) {
	t.Run("refresh 0 allowed in container scope", func(t *testing.T) {
		testCmd := newTestMetricsCmd(t)
		require.NoError(t, testCmd.Flags().Set(flagScopeName, scopeContainer))
		require.NoError(t, testCmd.Flags().Set(flagRefreshName, "0"))

		err := validateFlags(testCmd, nil)
		assert.NoError(t, err)
	})

	t.Run("refresh 0 allowed in cgroup scope", func(t *testing.T) {
		testCmd := newTestMetricsCmd(t)
		require.NoError(t, testCmd.Flags().Set(flagScopeName, scopeCgroup))
		require.NoError(t, testCmd.Flags().Set(flagRefreshName, "0"))

		err := validateFlags(testCmd, nil)
		assert.NoError(t, err)
	})

	t.Run("refresh 1 with interval 5 fails", func(t *testing.T) {
		testCmd := newTestMetricsCmd(t)
		require.NoError(t, testCmd.Flags().Set(flagScopeName, scopeContainer))
		require.NoError(t, testCmd.Flags().Set(flagPerfPrintIntervalName, "5"))
		require.NoError(t, testCmd.Flags().Set(flagRefreshName, "1"))

		err := validateFlags(testCmd, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "refresh must be greater than or equal to the event collection interval (5)")
	})
}

func TestResolveRawReplayFlags(t *testing.T) {
	t.Run("restores explicit metadata scope and granularity", func(t *testing.T) {
		md := Metadata{Scope: scopeContainer, Granularity: granularitySystem}
		scope, gran := resolveRawReplayFlags(md, false, scopeSystem, false, granularityCPU)
		assert.Equal(t, scopeContainer, scope)
		assert.Equal(t, granularitySystem, gran)
	})

	t.Run("restores system cpu granularity from metadata", func(t *testing.T) {
		md := Metadata{Scope: scopeSystem, Granularity: granularityCPU}
		scope, gran := resolveRawReplayFlags(md, false, scopeSystem, false, granularitySystem)
		assert.Equal(t, scopeSystem, scope)
		assert.Equal(t, granularityCPU, gran)
	})

	t.Run("infers container scope from legacy metadata", func(t *testing.T) {
		md := Metadata{
			ContainerNames: map[string]string{"kubepods/p1/c1": "default/pod-1/app"},
		}
		scope, gran := resolveRawReplayFlags(md, false, scopeSystem, false, granularitySystem)
		assert.Equal(t, scopeContainer, scope)
		assert.Equal(t, granularitySystem, gran)
	})

	t.Run("infers cgroup scope from legacy metadata", func(t *testing.T) {
		md := Metadata{
			CgroupConfigs: []CgroupConfig{{Name: "system.slice/docker-1.scope"}},
		}
		scope, gran := resolveRawReplayFlags(md, false, scopeSystem, false, granularitySystem)
		assert.Equal(t, scopeCgroup, scope)
		assert.Equal(t, granularitySystem, gran)
	})

	t.Run("honors explicit user overrides", func(t *testing.T) {
		md := Metadata{
			Scope:          scopeContainer,
			Granularity:    granularitySystem,
			ContainerNames: map[string]string{"kubepods/p1/c1": "default/pod-1/app"},
		}
		scope, gran := resolveRawReplayFlags(md, true, scopeCgroup, true, granularitySocket)
		assert.Equal(t, scopeCgroup, scope)
		assert.Equal(t, granularitySocket, gran)
	})

	t.Run("keeps defaults when metadata has no scope or granularity", func(t *testing.T) {
		scope, gran := resolveRawReplayFlags(Metadata{}, false, scopeSystem, false, granularitySystem)
		assert.Equal(t, scopeSystem, scope)
		assert.Equal(t, granularitySystem, gran)
	})
}

func TestApplyResolvedContainersStabilityAndID(t *testing.T) {
	tc := &targetContext{}

	cids1 := applyResolvedContainers(tc, []Container{
		{
			ID:         "111111111111aaaa",
			Name:       "webapp",
			PodName:    "web-0",
			Namespace:  "default",
			Runtime:    "cri",
			CgroupPath: "kubepods/pod-a/c1",
		},
		{
			ID:         "222222222222bbbb",
			Name:       "nginx",
			Runtime:    "docker",
			CgroupPath: "docker/2222",
		},
	})
	assert.Equal(t, []string{"kubepods/pod-a/c1", "docker/2222"}, cids1)
	assert.Equal(t, "default/web-0/webapp", tc.metadata.ContainerNames["kubepods/pod-a/c1"])
	assert.Equal(t, "nginx", tc.metadata.ContainerNames["docker/2222"])
	assert.Equal(t, "111111111111aaaa", tc.metadata.ContainerInfo["default/web-0/webapp"].ID)
	assert.Equal(t, "222222222222bbbb", tc.metadata.ContainerInfo["nginx"].ID)

	// On refresh: existing cgroup keeps its label even as new namespaces appear, and a
	// recreated standalone container with a new cgroup path does not steal "nginx".
	cids2 := applyResolvedContainers(tc, []Container{
		{
			ID:         "111111111111aaaa",
			Name:       "webapp",
			PodName:    "web-0",
			Namespace:  "default",
			Runtime:    "cri",
			CgroupPath: "kubepods/pod-a/c1",
		},
		{
			ID:         "333333333333cccc",
			Name:       "nginx",
			Runtime:    "docker",
			CgroupPath: "docker/3333",
		},
		{
			ID:         "444444444444dddd",
			Name:       "api",
			PodName:    "api-0",
			Namespace:  "prod",
			Runtime:    "cri",
			CgroupPath: "kubepods/pod-b/c4",
		},
	})
	assert.Equal(t, []string{"kubepods/pod-a/c1", "docker/3333", "kubepods/pod-b/c4"}, cids2)
	assert.Equal(t, "default/web-0/webapp", tc.metadata.ContainerNames["kubepods/pod-a/c1"])
	assert.Equal(t, "nginx (333333333333)", tc.metadata.ContainerNames["docker/3333"])
	assert.Equal(t, "333333333333cccc", tc.metadata.ContainerInfo["nginx (333333333333)"].ID)
	assert.Equal(t, "prod/api-0/api", tc.metadata.ContainerNames["kubepods/pod-b/c4"])
	assert.Equal(t, "222222222222bbbb", tc.metadata.ContainerInfo["nginx"].ID)
}
