// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMetricsCmd(t *testing.T) *cobra.Command {
	t.Helper()
	origDuration := flagDuration
	origScope := flagScope
	origPidList := flagPidList
	origCidList := flagCidList
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

func TestValidateFlagsRefresh(t *testing.T) {
	t.Run("refresh 0 allowed in cgroup scope", func(t *testing.T) {
		testCmd := newTestMetricsCmd(t)
		require.NoError(t, testCmd.Flags().Set(flagScopeName, scopeCgroup))
		require.NoError(t, testCmd.Flags().Set(flagRefreshName, "0"))

		err := validateFlags(testCmd, nil)
		assert.NoError(t, err)
	})

	t.Run("refresh 1 with interval 5 fails", func(t *testing.T) {
		testCmd := newTestMetricsCmd(t)
		require.NoError(t, testCmd.Flags().Set(flagScopeName, scopeCgroup))
		require.NoError(t, testCmd.Flags().Set(flagPerfPrintIntervalName, "5"))
		require.NoError(t, testCmd.Flags().Set(flagRefreshName, "1"))

		err := validateFlags(testCmd, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "refresh must be greater than or equal to the event collection interval (5)")
	})
}

func TestResolveRawReplayFlags(t *testing.T) {
	t.Run("restores explicit metadata scope and granularity", func(t *testing.T) {
		md := Metadata{Scope: scopeCgroup, Granularity: granularitySystem}
		scope, gran := resolveRawReplayFlags(md, false, scopeSystem, false, granularityCPU)
		assert.Equal(t, scopeCgroup, scope)
		assert.Equal(t, granularitySystem, gran)
	})

	t.Run("restores system cpu granularity from metadata", func(t *testing.T) {
		md := Metadata{Scope: scopeSystem, Granularity: granularityCPU}
		scope, gran := resolveRawReplayFlags(md, false, scopeSystem, false, granularitySystem)
		assert.Equal(t, scopeSystem, scope)
		assert.Equal(t, granularityCPU, gran)
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
			Scope:       scopeCgroup,
			Granularity: granularitySystem,
		}
		scope, gran := resolveRawReplayFlags(md, true, scopeSystem, true, granularitySocket)
		assert.Equal(t, scopeSystem, scope)
		assert.Equal(t, granularitySocket, gran)
	})

	t.Run("keeps defaults when metadata has no scope or granularity", func(t *testing.T) {
		scope, gran := resolveRawReplayFlags(Metadata{}, false, scopeSystem, false, granularitySystem)
		assert.Equal(t, scopeSystem, scope)
		assert.Equal(t, granularitySystem, gran)
	})
}
