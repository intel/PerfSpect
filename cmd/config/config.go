// Package config is a subcommand of the root command. It sets system configuration items on target platform(s).
package config

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"perfspect/internal/common"
	"perfspect/internal/progress"
	"perfspect/internal/report"
	"perfspect/internal/script"
	"perfspect/internal/target"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const cmdName = "config"

var examples = []string{
	fmt.Sprintf("  Set core count on local host:            $ %s %s --cores 32", common.AppName, cmdName),
	fmt.Sprintf("  Set multiple config items on local host: $ %s %s --core-max 3.0 --uncore-max 2.1 --tdp 120", common.AppName, cmdName),
	fmt.Sprintf("  Set core count on remote target:         $ %s %s --cores 32 --target 192.168.1.1 --user fred --key fred_key", common.AppName, cmdName),
	fmt.Sprintf("  View current config on remote target:    $ %s %s --target 192.168.1.1 --user fred --key fred_key", common.AppName, cmdName),
	fmt.Sprintf("  Set governor on remote targets:          $ %s %s --gov performance --targets targets.yaml", common.AppName, cmdName),
}

var Cmd = &cobra.Command{
	Use:   cmdName,
	Short: "Modify target(s) system configuration",
	Long: `Sets system configuration items on target platform(s).

USE CAUTION! Target may become unstable. It is up to the user to ensure that the requested configuration is valid for the target. There is not an automated way to revert the configuration changes. If all else fails, reboot the target.`,
	Example:       strings.Join(examples, "\n"),
	RunE:          runCmd,
	PreRunE:       validateFlags,
	GroupID:       "primary",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
}

func init() {
	initializeFlags(Cmd)
}

func runCmd(cmd *cobra.Command, args []string) error {
	// appContext is the application context that holds common data and resources.
	appContext := cmd.Parent().Context().Value(common.AppContext{}).(common.AppContext)
	localTempDir := appContext.LocalTempDir
	// get the targets
	myTargets, targetErrs, err := common.GetTargets(cmd, true, true, localTempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	// schedule the removal of the temp directory on each target (if the debug flag is not set)
	if cmd.Parent().PersistentFlags().Lookup("debug").Value.String() != "true" {
		for _, myTarget := range myTargets {
			if myTarget.GetTempDirectory() != "" {
				deferTarget := myTarget // create a new variable to capture the current value
				defer func(deferTarget target.Target) {
					err = myTarget.RemoveTempDirectory()
					if err != nil {
						fmt.Fprintf(os.Stderr, "Failed to remove target temp directory: %+v\n", err)
						slog.Error(err.Error())
					}
				}(deferTarget)
			}
		}
	}
	// check for errors in target creation
	for i := range targetErrs {
		if targetErrs[i] != nil {
			fmt.Fprintf(os.Stderr, "Error: target: %s, %v\n", myTargets[i].GetName(), targetErrs[i])
			slog.Error(targetErrs[i].Error())
			// remove target from targets list
			myTargets = slices.Delete(myTargets, i, i+1)
		}
	}
	if len(myTargets) == 0 {
		err := fmt.Errorf("no targets remain")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	// print config prior to changes
	if err := printConfig(myTargets, localTempDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	// make requested changes, one target at a time
	changeRequested := false
	for _, myTarget := range myTargets {
		for _, group := range flagGroups {
			for _, flag := range group.flags {
				if cmd.Flags().Lookup(flag.GetName()).Changed {
					changeRequested = true
					var err error
					switch flag.GetType() {
					case "int":
						if flag.intSetFunc != nil {
							value, _ := cmd.Flags().GetInt(flag.GetName())
							err = flag.intSetFunc(value, myTarget, localTempDir)
						}
					case "float64":
						if flag.floatSetFunc != nil {
							value, _ := cmd.Flags().GetFloat64(flag.GetName())
							err = flag.floatSetFunc(value, myTarget, localTempDir)
						}
					case "string":
						if flag.stringSetFunc != nil {
							value, _ := cmd.Flags().GetString(flag.GetName())
							err = flag.stringSetFunc(value, myTarget, localTempDir)
						}
					}
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error on %s: %v\n", myTarget.GetName(), err)
						slog.Error(err.Error(), slog.String("target", myTarget.GetName()))
					}
				}
			}
		}
	}
	if !changeRequested {
		fmt.Println("No changes requested.")
		return nil
	}
	// print config after making changes
	fmt.Println("") // blank line
	if err := printConfig(myTargets, localTempDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		return err
	}
	return nil
}

func printConfig(myTargets []target.Target, localTempDir string) (err error) {
	scriptNames := report.GetScriptNamesForTable(report.ConfigurationTableName)
	var scriptsToRun []script.ScriptDefinition
	for _, scriptName := range scriptNames {
		scriptsToRun = append(scriptsToRun, script.GetScriptByName(scriptName))
	}
	for _, myTarget := range myTargets {
		multiSpinner := progress.NewMultiSpinner()
		err = multiSpinner.AddSpinner(myTarget.GetName())
		if err != nil {
			err = fmt.Errorf("failed to add spinner: %v", err)
			return
		}
		multiSpinner.Start()
		_ = multiSpinner.Status(myTarget.GetName(), "collecting data")
		// run the scripts
		var scriptOutputs map[string]script.ScriptOutput
		if scriptOutputs, err = script.RunScripts(myTarget, scriptsToRun, true, localTempDir); err != nil {
			err = fmt.Errorf("failed to run collection scripts: %v", err)
			_ = multiSpinner.Status(myTarget.GetName(), "error collecting data")
			multiSpinner.Finish()
			return
		}
		_ = multiSpinner.Status(myTarget.GetName(), "collection complete")
		multiSpinner.Finish()
		// process the tables, i.e., get field values from raw script output
		tableNames := []string{report.ConfigurationTableName}
		var tableValues []report.TableValues
		if tableValues, err = report.ProcessTables(tableNames, scriptOutputs); err != nil {
			err = fmt.Errorf("failed to process collected data: %v", err)
			return
		}
		// create the report for this single table
		var reportBytes []byte
		if reportBytes, err = report.Create("txt", tableValues, scriptOutputs, myTarget.GetName()); err != nil {
			err = fmt.Errorf("failed to create report: %v", err)
			return
		}
		// print the report
		fmt.Print(string(reportBytes))
	}
	return
}

func setCoreCount(cores int, myTarget target.Target, localTempDir string) error {
	fmt.Printf("set core count per processor to %d on %s\n", cores, myTarget.GetName())
	setScript := script.ScriptDefinition{
		Name: "set core count",
		ScriptTemplate: fmt.Sprintf(`
desired_core_count_per_socket=%d
num_cpus=$(ls /sys/devices/system/cpu/ | grep -E "^cpu[0-9]+$" | wc -l)
num_threads=$(lscpu | grep 'Thread(s) per core' | awk '{print $NF}')
num_sockets=$(lscpu | grep 'Socket(s)' | awk '{print $NF}')
num_cores_per_socket=$((num_cpus / num_sockets / num_threads))

# if desired core count is greater than current core count, exit
if [[ $desired_core_count_per_socket -gt $num_cores_per_socket ]]; then
	echo "requested core count ($desired_core_count_per_socket) is greater than physical cores ($num_cores_per_socket)"
	exit 1
fi

# enable all logical CPUs
echo 1 | tee /sys/devices/system/cpu/cpu*/online > /dev/null

# if no cores to disable, exit
num_cores_to_disable_per_socket=$((num_cores_per_socket - desired_core_count_per_socket))
if [[ $num_cores_to_disable_per_socket -eq 0 ]]; then
    echo "no cpus to off-line"
    exit 0
fi

# get lines from cpuinfo that match the fields we need
proc_cpuinfo_filtered=$(grep -E '(processor|core id|physical id)' /proc/cpuinfo)

# loop through each line of text in proc_cpuinfo_filtered, creating a new record for each logical CPU
while IFS= read -r line; do
    # if line contains 'processor', start a new record
    if [[ $line =~ "processor" ]]; then
        # if record isn't empty (is empty first time through loop), put the record in the list of cpuinfo records
        if [[ -n "$record" ]]; then
            cpuinfo+=("$record")
        fi
        record="$line"$'\n'
    else
        record+="$line"$'\n'
    fi
done <<< "$proc_cpuinfo_filtered"
# add the last record
if [[ -n "$record" ]]; then
    cpuinfo+=("$record")
fi

# build a unique list of core ids from the records
core_ids=()
for record in "${cpuinfo[@]}"; do
    core_id=$(echo "$record" | grep 'core id' | awk '{print $NF}')
    found=0
    for id in "${core_ids[@]}"; do
        if [[ "$id" == "$core_id" ]]; then
            found=1
            break
        fi
    done
    if [[ $found -eq 0 ]]; then
        core_ids+=("$core_id")
    fi
done

# disable logical CPUs to reach the desired core count per socket
for ((socket=0; socket<num_sockets; socket++)); do
    offlined_cores=0
    # loop through core_ids in reverse order to off-line the highest numbered cores first
    for ((i=${#core_ids[@]}-1; i>=0; i--)); do
        core=${core_ids[i]}
        if [[ $offlined_cores -eq $num_cores_to_disable_per_socket ]]; then
            break
        fi
        offlined_cores=$((offlined_cores+1))
        # find record that matches socket and core and off-line the logical CPU
        for record in "${cpuinfo[@]}"; do
            processor=$(echo "$record" | grep 'processor' | awk '{print $NF}')
            core_id=$(echo "$record" | grep 'core id' | awk '{print $NF}')
            physical_id=$(echo "$record" | grep 'physical id' | awk '{print $NF}')
            if [[ $physical_id -eq $socket && $core_id -eq $core ]]; then
                echo "Off-lining processor $processor (socket $physical_id, core $core_id)"
                echo 0 | tee /sys/devices/system/cpu/cpu"$processor"/online > /dev/null
                num_disabled_cores=$((num_disabled_cores+1))
            fi
        done
    done
done
`, cores),
		Superuser: true,
	}
	_, err := runScript(myTarget, setScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to set core count: %w", err)
	}
	return nil
}

func setLlcSize(llcSize float64, myTarget target.Target, localTempDir string) error {
	fmt.Printf("set LLC size to %.2f MB on %s\n", llcSize, myTarget.GetName())
	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.GetScriptByName(script.LscpuScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciBitsScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciDevicesScriptName))
	scripts = append(scripts, script.GetScriptByName(script.L3WaySizeName))

	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to run scripts on target: %w", err)
	}
	maximumLlcSize, _, err := report.GetL3LscpuMB(outputs)
	if err != nil {
		return fmt.Errorf("failed to get maximum LLC size: %w", err)
	}
	// microarchitecture
	uarch := report.UarchFromOutput(outputs)
	cacheWays := report.GetCacheWays(uarch)
	if len(cacheWays) == 0 {
		return fmt.Errorf("failed to get cache ways")
	}
	// current LLC size
	currentLlcSize, err := report.GetL3MSRMB(outputs)
	if err != nil {
		return fmt.Errorf("failed to get current LLC size: %w", err)
	}
	if currentLlcSize == llcSize {
		return fmt.Errorf("LLC size is already set to %.2f MB", llcSize)
	}
	// calculate the number of ways to set
	cachePerWay := maximumLlcSize / float64(len(cacheWays))
	waysToSet := int(math.Ceil((llcSize / cachePerWay)) - 1)
	if waysToSet >= len(cacheWays) {
		return fmt.Errorf("LLC size is too large, maximum is %.2f MB", maximumLlcSize)
	}
	// set the LLC size
	setScript := script.ScriptDefinition{
		Name:           "set LLC size",
		ScriptTemplate: fmt.Sprintf("wrmsr -a 0xC90 %d", cacheWays[waysToSet]),
		Superuser:      true,
		Families:       []string{"6"},                                                // Intel only
		Models:         []string{"63", "79", "86", "85", "106", "108", "143", "207"}, // not SRF, GNR
		Depends:        []string{"wrmsr"},
		Lkms:           []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to set LLC size: %w", err)
	}
	return nil
}

func setCoreFrequency(coreFrequency float64, myTarget target.Target, localTempDir string) error {
	fmt.Printf("set core frequency to %.1f GHz on %s\n", coreFrequency, myTarget.GetName())
	targetFamily, err := myTarget.GetFamily()
	if err != nil {
		return fmt.Errorf("failed to get target family: %w", err)
	}
	targetModel, err := myTarget.GetModel()
	if err != nil {
		return fmt.Errorf("failed to get target model: %w", err)
	}
	targetVendor, err := myTarget.GetVendor()
	if err != nil {
		return fmt.Errorf("failed to get target vendor: %w", err)
	}
	if targetVendor != "GenuineIntel" {
		return fmt.Errorf("core frequency setting not supported on %s due to vendor mismatch", myTarget.GetName())
	}
	var setScript script.ScriptDefinition
	freqInt := uint64(coreFrequency * 10)
	if targetFamily == "6" && targetModel == "175" { // SRF
		// get the pstate driver
		getScript := script.ScriptDefinition{
			Name:           "get pstate driver",
			ScriptTemplate: "cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_driver",
			Vendors:        []string{"GenuineIntel"},
		}
		output, err := runScript(myTarget, getScript, localTempDir)
		if err != nil {
			return fmt.Errorf("failed to get pstate driver: %w", err)
		}
		if strings.Contains(output, "intel_pstate") {
			var value uint64
			for i := range 2 {
				value = value | freqInt<<uint(i*8)
			}
			setScript = script.ScriptDefinition{
				Name:           "set frequency bins",
				ScriptTemplate: fmt.Sprintf("wrmsr 0x774 %d", value),
				Superuser:      true,
				Vendors:        []string{"GenuineIntel"},
				Depends:        []string{"wrmsr"},
			}
		} else {
			value := freqInt << uint(2*8)
			setScript = script.ScriptDefinition{
				Name:           "set frequency bins",
				ScriptTemplate: fmt.Sprintf("wrmsr 0x199 %d", value),
				Superuser:      true,
				Vendors:        []string{"GenuineIntel"},
				Depends:        []string{"wrmsr"},
			}
		}
	} else {
		var value uint64
		for i := range 8 {
			value = value | freqInt<<uint(i*8)
		}
		setScript = script.ScriptDefinition{
			Name:           "set frequency bins",
			ScriptTemplate: fmt.Sprintf("wrmsr -a 0x1AD %d", value),
			Superuser:      true,
			Vendors:        []string{"GenuineIntel"},
			Depends:        []string{"wrmsr"},
		}
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to set core frequency: %w", err)
	}
	return nil
}

func setUncoreDieFrequency(maxFreq bool, computeDie bool, uncoreFrequency float64, myTarget target.Target, localTempDir string) error {
	var minmax, dietype string
	if maxFreq {
		minmax = "max"
	} else {
		minmax = "min"
	}
	if computeDie {
		dietype = "compute"
	} else {
		dietype = "I/O"
	}
	fmt.Printf("set uncore %s %s die frequency to %.1f GHz on %s\n", minmax, dietype, uncoreFrequency, myTarget.GetName())
	targetFamily, err := myTarget.GetFamily()
	if err != nil {
		return fmt.Errorf("failed to get target family: %w", err)
	}
	targetModel, err := myTarget.GetModel()
	if err != nil {
		return fmt.Errorf("failed to get target model: %w", err)
	}
	if targetFamily != "6" || (targetFamily == "6" && targetModel != "173" && targetModel != "175" && targetModel != "221") {
		return fmt.Errorf("uncore frequency setting not supported on %s due to family/model mismatch", myTarget.GetName())
	}
	type dieId struct {
		instance string
		entry    string
	}
	var dies []dieId
	// build list of compute or IO dies
	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.GetScriptByName(script.UncoreDieTypesFromTPMIScriptName))
	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to get uncore die types: %w", err)
	}
	re := regexp.MustCompile(`Read bits \d+:\d+ value (\d+) from TPMI ID .* for entry (\d+) in instance (\d+)`)
	for line := range strings.SplitSeq(outputs[script.UncoreDieTypesFromTPMIScriptName].Stdout, "\n") {
		match := re.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if computeDie && match[1] == "0" {
			dies = append(dies, dieId{instance: match[3], entry: match[2]})
		}
		if !computeDie && match[1] == "1" {
			dies = append(dies, dieId{instance: match[3], entry: match[2]})
		}
	}

	value := uint64(uncoreFrequency * 10)
	var bits string
	if maxFreq {
		bits = "8:14" // bits 8:14 are the max frequency
	} else {
		bits = "15:21" // bits 15:21 are the min frequency
	}
	// run script for each die of specified type
	for _, die := range dies {
		setScript := script.ScriptDefinition{
			Name:           "write max and min uncore frequency TPMI",
			ScriptTemplate: fmt.Sprintf("pcm-tpmi 2 0x18 -d -b %s -w %d -i %s -e %s", bits, value, die.instance, die.entry),
			Vendors:        []string{"GenuineIntel"},
			Depends:        []string{"pcm-tpmi"},
			Superuser:      true,
		}
		_, err = runScript(myTarget, setScript, localTempDir)
		if err != nil {
			return fmt.Errorf("failed to set uncore frequency: %w", err)
		}
	}
	return nil
}

func setUncoreFrequency(maxFreq bool, uncoreFrequency float64, myTarget target.Target, localTempDir string) error {
	var minmax string
	if maxFreq {
		minmax = "max"
	} else {
		minmax = "min"
	}
	fmt.Printf("set uncore %s frequency to %.1f GHz on %s\n", minmax, uncoreFrequency, myTarget.GetName())
	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.ScriptDefinition{
		Name:           "get uncore frequency MSR",
		ScriptTemplate: "rdmsr 0x620",
		Vendors:        []string{"GenuineIntel"},
		Depends:        []string{"rdmsr"},
		Lkms:           []string{"msr"},
		Superuser:      true,
	})
	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to read uncore frequency MSR: %w", err)
	}
	targetFamily, err := myTarget.GetFamily()
	if err != nil {
		return fmt.Errorf("failed to get target family: %w", err)
	}
	targetModel, err := myTarget.GetModel()
	if err != nil {
		return fmt.Errorf("failed to get target model: %w", err)
	}
	if targetFamily != "6" || (targetFamily == "6" && (targetModel == "173" || targetModel == "175" || targetModel == "221")) { // not Intel || not GNR, SRF, CWF
		return fmt.Errorf("uncore frequency setting not supported on %s due to family/model mismatch", myTarget.GetName())
	}
	msrHex := strings.TrimSpace(outputs["get uncore frequency MSR"].Stdout)
	msrInt, err := strconv.ParseInt(msrHex, 16, 0)
	if err != nil {
		return fmt.Errorf("failed to read uncore frequency MSR: %w", err)
	}
	newFreq := uint64((uncoreFrequency * 1000) / 100)
	var newVal uint64
	if maxFreq {
		// mask out lower 6 bits to write the max frequency
		newVal = uint64(msrInt) & 0xFFFFFFFFFFFFFFC0
		// add in the new frequency value
		newVal = newVal | newFreq
	} else {
		// mask bits 8:14 to write the min frequency
		newVal = uint64(msrInt) & 0xFFFFFFFFFFFF80FF
		// add in the new frequency value
		newVal = newVal | newFreq<<8
	}
	setScript := script.ScriptDefinition{
		Name:           "set uncore frequency MSR",
		ScriptTemplate: fmt.Sprintf("wrmsr -a 0x620 %d", newVal),
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"wrmsr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to set uncore frequency: %w", err)
	}
	return nil
}

func setTDP(power int, myTarget target.Target, localTempDir string) error {
	fmt.Printf("set power to %d Watts on %s\n", power, myTarget.GetName())
	readScript := script.ScriptDefinition{
		Name:           "get power MSR",
		ScriptTemplate: "rdmsr 0x610",
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
	}
	readOutput, err := script.RunScript(myTarget, readScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to read power MSR: %w", err)
	} else {
		msrHex := strings.TrimSpace(readOutput.Stdout)
		msrInt, err := strconv.ParseInt(msrHex, 16, 0)
		if err != nil {
			return fmt.Errorf("failed to parse power MSR: %w", err)
		} else {
			// mask out lower 14 bits
			newVal := uint64(msrInt) & 0xFFFFFFFFFFFFC000
			// add in the new power value
			newVal = newVal | uint64(power*8)
			setScript := script.ScriptDefinition{
				Name:           "set tdp",
				ScriptTemplate: fmt.Sprintf("wrmsr -a 0x610 %d", newVal),
				Superuser:      true,
				Vendors:        []string{"GenuineIntel"},
				Lkms:           []string{"msr"},
				Depends:        []string{"wrmsr"},
			}
			_, err := runScript(myTarget, setScript, localTempDir)
			if err != nil {
				return fmt.Errorf("failed to set power: %w", err)
			}
		}
	}
	return nil
}

func setEPB(epb int, myTarget target.Target, localTempDir string) error {
	fmt.Printf("set energy performance bias (EPB) to %d on %s\n", epb, myTarget.GetName())
	epbSourceScript := script.GetScriptByName(script.EpbSourceScriptName)
	epbSourceOutput, err := runScript(myTarget, epbSourceScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to get EPB source: %w", err)
	}
	epbSource := strings.TrimSpace(epbSourceOutput)
	source, err := strconv.ParseInt(epbSource, 16, 0)
	if err != nil {
		return fmt.Errorf("failed to parse EPB source: %w", err)
	}
	var msr string
	var bitOffset uint
	if source == 0 { // 0 means the EPB is controlled by the OS
		msr = "0x1B0"
		bitOffset = 0
	} else { // 1 means the EPB is controlled by the BIOS
		msr = "0xA01"
		bitOffset = 3
	}
	readScript := script.ScriptDefinition{
		Name:           "read " + msr,
		ScriptTemplate: "rdmsr " + msr,
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	}
	readOutput, err := runScript(myTarget, readScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to read EPB MSR %s: %w", msr, err)
	}
	msrValue, err := strconv.ParseUint(strings.TrimSpace(readOutput), 16, 64)
	if err != nil {
		return fmt.Errorf("failed to parse EPB MSR %s: %w", msr, err)
	}
	// mask out 4 bits starting at bitOffset
	maskedValue := msrValue &^ (0xF << bitOffset)
	// put the EPB value in the masked bits
	msrValue = maskedValue | uint64(epb)<<bitOffset
	// write the new value to the MSR
	setScript := script.ScriptDefinition{
		Name:           "set epb",
		ScriptTemplate: fmt.Sprintf("wrmsr -a %s %d", msr, msrValue),
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"wrmsr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to set EPB: %w", err)
	}
	return nil
}

func setEPP(epp int, myTarget target.Target, localTempDir string) error {
	fmt.Printf("set energy performance profile (EPP) to %d on %s\n", epp, myTarget.GetName())
	// Set both the per-core EPP value and the package EPP value
	// Reference: 15.4.4 Managing HWP in the Intel SDM

	// get the current value of the IAEW_HWP_REQUEST MSR that includes the current EPP valid value in bit 60
	getScript := script.ScriptDefinition{
		Name:           "get epp msr",
		ScriptTemplate: "rdmsr 0x774", // IA32_HWP_REQUEST
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	}
	stdout, err := runScript(myTarget, getScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to read EPP MSR %s: %w", "0x774", err)
	}
	msrValue, err := strconv.ParseUint(strings.TrimSpace(stdout), 16, 64)
	if err != nil {
		return fmt.Errorf("failed to parse EPP MSR %s: %w", "0x774", err)
	}
	// mask out bits 24-31 IA32_HWP_REQUEST MSR value
	maskedValue := msrValue & 0xFFFFFFFF00FFFFFF
	// put the EPP value in bits 24-31
	eppValue := maskedValue | uint64(epp)<<24
	// write it back to the MSR
	setScript := script.ScriptDefinition{
		Name:           "set epp",
		ScriptTemplate: fmt.Sprintf("wrmsr -a 0x774 %d", eppValue),
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"wrmsr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to set EPP: %w", err)
	}

	// get the current value of the IA32_HWP_REQUEST_PKG MSR that includes the current package EPP value
	getScript = script.ScriptDefinition{
		Name:           "get epp pkg msr",
		ScriptTemplate: "rdmsr 0x772", // IA32_HWP_REQUEST_PKG
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	}
	stdout, err = runScript(myTarget, getScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to read EPP pkg MSR %s: %w", "0x772", err)
	}
	msrValue, err = strconv.ParseUint(strings.TrimSpace(stdout), 16, 64)
	if err != nil {
		return fmt.Errorf("failed to parse EPP pkg MSR %s: %w", "0x772", err)
	}
	// mask out bits 24-31 IA32_HWP_REQUEST_PKG MSR value
	maskedValue = msrValue & 0xFFFFFFFF00FFFFFF
	// put the EPP value in bits 24-31
	eppValue = maskedValue | uint64(epp)<<24
	// write it back to the MSR
	setScript = script.ScriptDefinition{
		Name:           "set epp",
		ScriptTemplate: fmt.Sprintf("wrmsr -a 0x772 %d", eppValue),
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"wrmsr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to set EPP pkg: %w", err)
	}
	return nil
}

func setGovernor(governor string, myTarget target.Target, localTempDir string) error {
	fmt.Printf("set governor to %s on %s\n", governor, myTarget.GetName())
	setScript := script.ScriptDefinition{
		Name:           "set governor",
		ScriptTemplate: fmt.Sprintf("echo %s | tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor", governor),
		Superuser:      true,
	}
	_, err := runScript(myTarget, setScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to set governor: %w", err)
	}
	return nil
}

func setELC(elc string, myTarget target.Target, localTempDir string) error {
	fmt.Printf("set efficiency latency control (ELC) mode to %s on %s\n", elc, myTarget.GetName())
	var mode string
	if elc == elcOptions[0] {
		mode = "latency-optimized-mode"
	} else if elc == elcOptions[1] {
		mode = "default"
	} else {
		return fmt.Errorf("invalid ELC mode: %s", elc)
	}
	setScript := script.ScriptDefinition{
		Name:           "set elc",
		ScriptTemplate: fmt.Sprintf("bhs-power-mode.sh --%s", mode),
		Superuser:      true,
		Vendors:        []string{"GenuineIntel"},
		Models:         []string{"173", "175", "221"}, // GNR, SRF, CWF
		Depends:        []string{"bhs-power-mode.sh"},
	}
	_, err := runScript(myTarget, setScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to set ELC mode: %w", err)
	}
	return nil
}

func setPrefetcher(enableDisable string, myTarget target.Target, localTempDir string, prefetcherType string) error {
	fmt.Printf("set %s prefetcher to %s on %s\n", prefetcherType, enableDisable, myTarget.GetName())
	pf, err := report.GetPrefetcherDefByName(prefetcherType)
	if err != nil {
		return fmt.Errorf("failed to get prefetcher definition: %w", err)
	}
	// check if the prefetcher is supported on this target's architecture
	// get the uarch
	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.GetScriptByName(script.LscpuScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciBitsScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciDevicesScriptName))
	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to run target identification scripts on target: %w", err)
	}
	uarch := report.UarchFromOutput(outputs)
	if uarch == "" {
		return fmt.Errorf("failed to get microarchitecture")
	}
	// is the prefetcher supported on this uarch?
	if !slices.Contains(pf.Uarchs, "all") && !slices.Contains(pf.Uarchs, uarch[:3]) {
		return fmt.Errorf("prefetcher %s is not supported on %s", prefetcherType, uarch)
	}
	// get the current value of the prefetcher MSR
	getScript := script.ScriptDefinition{
		Name:           "get prefetcher msr",
		ScriptTemplate: fmt.Sprintf("rdmsr %d", pf.Msr),
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"rdmsr"},
		Superuser:      true,
	}
	stdout, err := runScript(myTarget, getScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to read prefetcher MSR: %w", err)
	}
	msrValue, err := strconv.ParseUint(strings.TrimSpace(stdout), 16, 64)
	if err != nil {
		return fmt.Errorf("failed to parse prefetcher MSR: %w", err)
	}
	// set the prefetcher bit to bitValue determined by the onOff value, note: 0 is enable, 1 is disable
	var bitVal int
	if enableDisable == prefetcherOptions[0] {
		bitVal = 0
	} else if enableDisable == prefetcherOptions[1] {
		bitVal = 1
	} else {
		return fmt.Errorf("invalid prefetcher setting: %s", enableDisable)
	}
	// mask out the prefetcher bit
	maskedValue := msrValue &^ (1 << pf.Bit)
	// set the prefetcher bit
	newVal := maskedValue | uint64(bitVal<<pf.Bit)
	// write the new value to the MSR
	setScript := script.ScriptDefinition{
		Name:           "set prefetcher",
		ScriptTemplate: fmt.Sprintf("wrmsr -a %d %d", pf.Msr, newVal),
		Vendors:        []string{"GenuineIntel"},
		Lkms:           []string{"msr"},
		Depends:        []string{"wrmsr"},
		Superuser:      true,
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		return fmt.Errorf("failed to set %s prefetcher: %w", prefetcherType, err)
	}
	return nil
}

func runScript(myTarget target.Target, myScript script.ScriptDefinition, localTempDir string) (string, error) {
	output, err := script.RunScript(myTarget, myScript, localTempDir) // nosemgrep
	if err != nil {
		slog.Error("failed to run script on target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()), slog.String("stdout", output.Stdout), slog.String("stderr", output.Stderr))
	} else {
		slog.Debug("ran script on target", slog.String("target", myTarget.GetName()), slog.String("script", myScript.Name), slog.String("stdout", output.Stdout), slog.String("stderr", output.Stderr))
	}
	return output.Stdout, err
}
