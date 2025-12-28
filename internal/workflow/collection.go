// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package workflow

import (
	"fmt"
	"log/slog"
	"strings"

	"slices"

	"perfspect/internal/app"
	"perfspect/internal/progress"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/table"
	"perfspect/internal/target"
	"perfspect/internal/util"

	"github.com/spf13/cobra"
)

// outputsFromInput reads the raw file(s) and returns the data in the order of the raw files
func outputsFromInput(tables []table.TableDefinition, summaryTableName string) ([]TargetScriptOutputs, error) {
	orderedTargetScriptOutputs := []TargetScriptOutputs{}
	includedTables := []table.TableDefinition{}
	// read the raw file(s) as JSON
	rawReports, err := report.ReadRawReports(app.FlagInput)
	if err != nil {
		err = fmt.Errorf("failed to read raw file(s): %w", err)
		return nil, err
	}
	for _, rawReport := range rawReports {
		for _, tableName := range rawReport.TableNames { // just in case someone tries to use the raw files that were collected with a different set of categories
			// filter out tables that we add after processing
			if tableName == app.TableNameInsights || tableName == app.TableNamePerfspect || tableName == summaryTableName {
				continue
			}
			includedTable, err := findTableByName(tables, tableName)
			if err != nil {
				slog.Warn("table from raw report not found in current tables", slog.String("table", tableName), slog.String("target", rawReport.TargetName))
				continue
			}
			includedTables = append(includedTables, *includedTable)
		}
		orderedTargetScriptOutputs = append(orderedTargetScriptOutputs, TargetScriptOutputs{TargetName: rawReport.TargetName, ScriptOutputs: rawReport.ScriptOutputs, Tables: includedTables})
	}
	return orderedTargetScriptOutputs, nil
}

// outputsFromTargets runs the scripts on the targets and returns the data in the order of the targets
func outputsFromTargets(cmd *cobra.Command, myTargets []target.Target, tables []table.TableDefinition, scriptParams map[string]string, statusUpdate progress.MultiSpinnerUpdateFunc, localTempDir string) ([]TargetScriptOutputs, error) {
	orderedTargetScriptOutputs := []TargetScriptOutputs{}
	channelTargetScriptOutputs := make(chan TargetScriptOutputs)
	channelError := make(chan error)
	// create the list of tables and associated scripts for each target
	targetTables := [][]table.TableDefinition{}
	targetScriptNames := [][]string{}
	for targetIdx, target := range myTargets {
		targetTables = append(targetTables, []table.TableDefinition{})
		targetScriptNames = append(targetScriptNames, []string{})
		for _, tbl := range tables {
			if isTableForTarget(tbl, target, localTempDir) {
				// add table to list of tables to collect
				targetTables[targetIdx] = append(targetTables[targetIdx], tbl)
				// add scripts to list of scripts to run
				for _, scriptName := range tbl.ScriptNames {
					targetScriptNames[targetIdx] = util.UniqueAppend(targetScriptNames[targetIdx], scriptName)
				}
			} else {
				slog.Debug("table not supported for target", slog.String("table", tbl.Name), slog.String("target", target.GetName()))
			}
		}
	}
	// run the scripts on the targets
	for targetIdx, target := range myTargets {
		scriptsToRunOnTarget := []script.ScriptDefinition{}
		for _, scriptName := range targetScriptNames[targetIdx] {
			script := script.GetParameterizedScriptByName(scriptName, scriptParams)
			scriptsToRunOnTarget = append(scriptsToRunOnTarget, script)
		}
		// run the selected scripts on the target
		ctrlCToStop := cmd.Name() == "telemetry" || cmd.Name() == "flamegraph"
		go collectOnTarget(target, scriptsToRunOnTarget, localTempDir, scriptParams["Duration"], ctrlCToStop, channelTargetScriptOutputs, channelError, statusUpdate)
	}
	// wait for scripts to run on all targets
	var allTargetScriptOutputs []TargetScriptOutputs
	for range myTargets {
		select {
		case scriptOutputs := <-channelTargetScriptOutputs:
			allTargetScriptOutputs = append(allTargetScriptOutputs, scriptOutputs)
		case err := <-channelError:
			slog.Error(err.Error())
		}
	}
	// allTargetScriptOutputs is in the order of data collection completion
	// reorder to match order of myTargets
	for targetIdx, target := range myTargets {
		for _, targetScriptOutputs := range allTargetScriptOutputs {
			if targetScriptOutputs.TargetName == target.GetName() {
				targetScriptOutputs.Tables = targetTables[targetIdx]
				orderedTargetScriptOutputs = append(orderedTargetScriptOutputs, targetScriptOutputs)
				break
			}
		}
	}
	return orderedTargetScriptOutputs, nil
}

// isTableForTarget checks if the given table is applicable for the specified target
func isTableForTarget(tbl table.TableDefinition, t target.Target, localTempDir string) bool {
	if len(tbl.Architectures) > 0 {
		architecture, err := t.GetArchitecture()
		if err != nil {
			slog.Error("failed to get architecture for target", slog.String("target", t.GetName()), slog.String("error", err.Error()))
			return false
		}
		if !slices.Contains(tbl.Architectures, architecture) {
			return false
		}
	}
	if len(tbl.Vendors) > 0 {
		vendor, err := GetTargetVendor(t)
		if err != nil {
			slog.Error("failed to get vendor for target", slog.String("target", t.GetName()), slog.String("error", err.Error()))
			return false
		}
		if !slices.Contains(tbl.Vendors, vendor) {
			return false
		}
	}
	if len(tbl.MicroArchitectures) > 0 {
		uarch, err := GetTargetMicroArchitecture(t, localTempDir, false)
		if err != nil {
			slog.Error("failed to get microarchitecture for target", slog.String("target", t.GetName()), slog.String("error", err.Error()))
		}
		shortUarch := strings.Split(uarch, "_")[0]     // handle EMR_XCC, etc.
		shortUarch = strings.Split(shortUarch, "-")[0] // handle GNR-D
		shortUarch = strings.Split(shortUarch, " ")[0] // handle Turin (Zen 5)
		if !slices.Contains(tbl.MicroArchitectures, uarch) && !slices.Contains(tbl.MicroArchitectures, shortUarch) {
			return false
		}
	}
	return true
}

// elevatedPrivilegesRequired returns true if any of the scripts needed for the tables require elevated privileges
func elevatedPrivilegesRequired(tables []table.TableDefinition) bool {
	for _, tbl := range tables {
		for _, scriptName := range tbl.ScriptNames {
			script := script.GetScriptByName(scriptName)
			if script.Superuser {
				return true
			}
		}
	}
	return false
}

// collectOnTarget runs the scripts on the target and sends the results to the appropriate channels
func collectOnTarget(myTarget target.Target, scriptsToRun []script.ScriptDefinition, localTempDir string, duration string, ctrlCToStop bool, channelTargetScriptOutputs chan TargetScriptOutputs, channelError chan error, statusUpdate progress.MultiSpinnerUpdateFunc) {
	// run the scripts on the target
	status := "collecting data"
	if ctrlCToStop && duration == "0" {
		status += ", press Ctrl+c to stop"
	} else if duration != "0" && duration != "" {
		status += fmt.Sprintf(" for %s seconds", duration)
	}
	scriptOutputs, err := RunScripts(myTarget, scriptsToRun, true, localTempDir, statusUpdate, status, false)
	if err != nil {
		if statusUpdate != nil {
			_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("error collecting data: %v", err))
		}
		err = fmt.Errorf("error running data collection scripts on %s: %v", myTarget.GetName(), err)
		channelError <- err
		return
	}
	if statusUpdate != nil {
		_ = statusUpdate(myTarget.GetName(), "collection complete")
	}
	channelTargetScriptOutputs <- TargetScriptOutputs{TargetName: myTarget.GetName(), ScriptOutputs: scriptOutputs}
}
