// Package config is a subcommand of the root command. It sets system configuration items on target platform(s).
package config

// Copyright (C) 2021-2024 Intel Corporation
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
	"perfspect/internal/util"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	flagCores               int
	flagLlcSize             float64
	flagAllCoreMaxFrequency float64
	flagUncoreMaxFrequency  float64
	flagUncoreMinFrequency  float64
	flagPower               int
	flagEpb                 int
	flagEpp                 int
	flagGovernor            string
	flagElc                 string
)

const (
	flagCoresName               = "cores"
	flagLlcSizeName             = "llc"
	flagAllCoreMaxFrequencyName = "coremax"
	flagUncoreMaxFrequencyName  = "uncoremax"
	flagUncoreMinFrequencyName  = "uncoremin"
	flagPowerName               = "power"
	flagEpbName                 = "epb"
	flagEppName                 = "epp"
	flagGovernorName            = "governor"
	flagElcName                 = "elc"
)

// governorOptions - list of valid governor options
var governorOptions = []string{"performance", "powersave"}

// elcOptions - list of valid elc options
var elcOptions = []string{"latency-optimized", "default"}

const cmdName = "config"

var examples = []string{
	fmt.Sprintf("  Set core count on local host:            $ %s %s --cores 32", common.AppName, cmdName),
	fmt.Sprintf("  Set multiple config items on local host: $ %s %s --coremaxfreq 3.0 --uncoremaxfreq 2.1 --power 120", common.AppName, cmdName),
	fmt.Sprintf("  Set core count on remote target:         $ %s %s --cores 32 --target 192.168.1.1 --user fred --key fred_key", common.AppName, cmdName),
	fmt.Sprintf("  View current config on remote target:    $ %s %s --target 192.168.1.1 --user fred --key fred_key", common.AppName, cmdName),
	fmt.Sprintf("  Set governor on remote targets:          $ %s %s --governor performance --targets targets.yaml", common.AppName, cmdName),
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
	Cmd.Flags().IntVar(&flagCores, flagCoresName, 0, "")
	Cmd.Flags().Float64Var(&flagLlcSize, flagLlcSizeName, 0, "")
	Cmd.Flags().Float64Var(&flagAllCoreMaxFrequency, flagAllCoreMaxFrequencyName, 0, "")
	Cmd.Flags().Float64Var(&flagUncoreMaxFrequency, flagUncoreMaxFrequencyName, 0, "")
	Cmd.Flags().Float64Var(&flagUncoreMinFrequency, flagUncoreMinFrequencyName, 0, "")
	Cmd.Flags().IntVar(&flagPower, flagPowerName, 0, "")
	Cmd.Flags().IntVar(&flagEpb, flagEpbName, 0, "")
	Cmd.Flags().IntVar(&flagEpp, flagEppName, 0, "")
	Cmd.Flags().StringVar(&flagGovernor, flagGovernorName, "", "")
	Cmd.Flags().StringVar(&flagElc, flagElcName, "", "")

	common.AddTargetFlags(Cmd)

	Cmd.SetUsageFunc(usageFunc)
}

func usageFunc(cmd *cobra.Command) error {
	cmd.Printf("Usage: %s [flags]\n\n", cmd.CommandPath())
	cmd.Printf("Examples:\n%s\n\n", cmd.Example)
	cmd.Println("Flags:")
	for _, group := range getFlagGroups() {
		cmd.Printf("  %s:\n", group.GroupName)
		for _, flag := range group.Flags {
			cmd.Printf("    --%-20s %s\n", flag.Name, flag.Help)
		}
	}
	cmd.Println("\nGlobal Flags:")
	cmd.Parent().PersistentFlags().VisitAll(func(pf *pflag.Flag) {
		flagDefault := ""
		if cmd.Parent().PersistentFlags().Lookup(pf.Name).DefValue != "" {
			flagDefault = fmt.Sprintf(" (default: %s)", cmd.Flags().Lookup(pf.Name).DefValue)
		}
		cmd.Printf("  --%-20s %s%s\n", pf.Name, pf.Usage, flagDefault)
	})
	return nil
}

func getFlagGroups() []common.FlagGroup {
	flags := []common.Flag{
		{
			Name: flagCoresName,
			Help: "set number of physical cores per processor",
		},
		{
			Name: flagLlcSizeName,
			Help: "set LLC size (MB)",
		},
		{
			Name: flagAllCoreMaxFrequencyName,
			Help: "set all-core max frequency (GHz)",
		},
		{
			Name: flagUncoreMaxFrequencyName,
			Help: "set uncore max frequency (GHz)",
		},
		{
			Name: flagUncoreMinFrequencyName,
			Help: "set uncore min frequency (GHz)",
		},
		{
			Name: flagPowerName,
			Help: "set TDP per processor (W)",
		},
		{
			Name: flagEpbName,
			Help: "set energy perf bias (EPB) from best performance (0) to most power savings (9)",
		},
		{
			Name: flagEppName,
			Help: "set energy perf profile (EPP) from best performance (0) to most power savings (255)",
		},
		{
			Name: flagGovernorName,
			Help: "set CPU scaling governor (" + strings.Join(governorOptions, ", ") + ")",
		},
		{
			Name: flagElcName,
			Help: "set Efficiency Latency Control (SRF and GNR) (" + strings.Join(elcOptions, ", ") + ")",
		},
	}
	groups := []common.FlagGroup{}
	groups = append(groups, common.FlagGroup{
		GroupName: "Configuration Options",
		Flags:     flags,
	})
	groups = append(groups, common.GetTargetFlagGroup())
	return groups
}

func validateFlags(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Lookup(flagCoresName).Changed && flagCores < 1 {
		return fmt.Errorf("invalid core count: %d", flagCores)
	}
	if cmd.Flags().Lookup(flagLlcSizeName).Changed && flagLlcSize < 1 {
		return fmt.Errorf("invalid LLC size: %.2f MB", flagLlcSize)
	}
	if cmd.Flags().Lookup(flagAllCoreMaxFrequencyName).Changed && flagAllCoreMaxFrequency < 0.1 {
		return fmt.Errorf("invalid core frequency: %.1f GHz", flagAllCoreMaxFrequency)
	}
	if cmd.Flags().Lookup(flagUncoreMaxFrequencyName).Changed && flagUncoreMaxFrequency < 0.1 {
		return fmt.Errorf("invalid uncore max frequency: %.1f GHz", flagUncoreMaxFrequency)
	}
	if cmd.Flags().Lookup(flagUncoreMinFrequencyName).Changed && flagUncoreMinFrequency < 0.1 {
		return fmt.Errorf("invalid uncore min frequency: %.1f GHz", flagUncoreMinFrequency)
	}
	if cmd.Flags().Lookup(flagPowerName).Changed && flagPower < 1 {
		return fmt.Errorf("invalid power: %d", flagPower)
	}
	if cmd.Flags().Lookup(flagEpbName).Changed && (flagEpb < 0 || flagEpb > 9) {
		return fmt.Errorf("invalid epb: %d", flagEpb)
	}
	if cmd.Flags().Lookup(flagEppName).Changed && (flagEpp < 0 || flagEpp > 255) {
		return fmt.Errorf("invalid epp: %d", flagEpp)
	}
	if cmd.Flags().Lookup(flagGovernorName).Changed && !util.StringInList(flagGovernor, governorOptions) {
		return fmt.Errorf("invalid governor: %s", flagGovernor)
	}
	if cmd.Flags().Lookup(flagElcName).Changed && !util.StringInList(flagElc, elcOptions) {
		return fmt.Errorf("invalid elc mode: %s", flagElc)
	}
	return nil
}

func runCmd(cmd *cobra.Command, args []string) error {
	// appContext is the application context that holds common data and resources.
	appContext := cmd.Context().Value(common.AppContext{}).(common.AppContext)
	localTempDir := appContext.TempDir
	// get the targets
	myTargets, targetErrs, err := common.GetTargets(cmd, true, true, localTempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	// check for errors in target connections
	for i := range targetErrs {
		if targetErrs[i] != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", targetErrs[i])
			slog.Error(targetErrs[i].Error())
			cmd.SilenceUsage = true
			return err
		}
	}
	if len(myTargets) == 0 {
		err := fmt.Errorf("no targets specified")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	// create a temporary directory on each target
	for _, myTarget := range myTargets {
		targetTempRoot, _ := cmd.Flags().GetString(common.FlagTargetTempDirName)
		targetTempDir, err := myTarget.CreateTempDirectory(targetTempRoot)
		if err != nil {
			err = fmt.Errorf("failed to create temporary directory: %w", err)
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
		defer func() {
			err = myTarget.RemoveDirectory(targetTempDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to remove target directory: %+v\n", err)
				slog.Error(err.Error())
			}
		}()
	}
	// print config prior to changes
	if err := printConfig(myTargets, localTempDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	// were any changes requested?
	changeRequested := false
	flagGroups := getFlagGroups()
	for _, flag := range flagGroups[0].Flags {
		if cmd.Flags().Lookup(flag.Name).Changed {
			changeRequested = true
			break
		}
	}
	if !changeRequested {
		fmt.Println("No changes requested.")
		return nil
	}
	// make requested changes, one target at a time
	for _, myTarget := range myTargets {
		if cmd.Flags().Lookup(flagCoresName).Changed {
			out, err := setCoreCount(flagCores, myTarget, localTempDir)
			if err != nil {
				fmt.Printf("Error: %v, %s\n", err, out)
				cmd.SilenceUsage = true
				return err
			}
		}
		if cmd.Flags().Lookup(flagLlcSizeName).Changed {
			setLlcSize(flagLlcSize, myTarget, localTempDir)
		}
		if cmd.Flags().Lookup(flagAllCoreMaxFrequencyName).Changed {
			setCoreFrequency(flagAllCoreMaxFrequency, myTarget, localTempDir)
		}
		if cmd.Flags().Lookup(flagUncoreMaxFrequencyName).Changed {
			setUncoreFrequency(true, flagUncoreMaxFrequency, myTarget, localTempDir)
		}
		if cmd.Flags().Lookup(flagUncoreMinFrequencyName).Changed {
			setUncoreFrequency(false, flagUncoreMinFrequency, myTarget, localTempDir)
		}
		if cmd.Flags().Lookup(flagPowerName).Changed {
			setPower(flagPower, myTarget, localTempDir)
		}
		if cmd.Flags().Lookup(flagEpbName).Changed {
			setEpb(flagEpb, myTarget, localTempDir)
		}
		if cmd.Flags().Lookup(flagEppName).Changed {
			setEpp(flagEpp, myTarget, localTempDir)
		}
		if cmd.Flags().Lookup(flagGovernorName).Changed {
			setGovernor(flagGovernor, myTarget, localTempDir)
		}
		if cmd.Flags().Lookup(flagElcName).Changed {
			setElc(flagElc, myTarget, localTempDir)
		}
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
		if tableValues, err = report.Process(tableNames, scriptOutputs); err != nil {
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

func setCoreCount(cores int, myTarget target.Target, localTempDir string) (string, error) {
	fmt.Printf("set core count per processor to %d on %s\n", cores, myTarget.GetName())
	setScript := script.ScriptDefinition{
		Name: "set core count",
		Script: fmt.Sprintf(`
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
	return runScript(myTarget, setScript, localTempDir)
}

func setLlcSize(llcSize float64, myTarget target.Target, localTempDir string) {
	fmt.Printf("set LLC size to %.2f MB on %s\n", llcSize, myTarget.GetName())
	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.GetScriptByName(script.LscpuScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciBitsScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciDevicesScriptName))
	scripts = append(scripts, script.GetScriptByName(script.L3WaySizeName))

	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		slog.Error("failed to run scripts on target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
		return
	}
	maximumLlcSize, err := report.GetL3LscpuMB(outputs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		slog.Error("failed to get maximum LLC size", slog.String("error", err.Error()))
		return
	}
	// microarchitecture
	uarch := report.UarchFromOutput(outputs)
	cacheWays := report.GetCacheWays(uarch)
	if len(cacheWays) == 0 {
		fmt.Fprintln(os.Stderr, "failed to get cache ways")
		slog.Error("failed to get cache ways")
		return
	}
	// current LLC size
	currentLlcSize, err := report.GetL3MSRMB(outputs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		slog.Error("failed to get LLC size", slog.String("error", err.Error()))
		return
	}
	if currentLlcSize == llcSize {
		fmt.Printf("LLC size is already set to %.2f MB\n", llcSize)
		return
	}
	// calculate the number of ways to set
	cachePerWay := maximumLlcSize / float64(len(cacheWays))
	waysToSet := int(math.Ceil((llcSize / cachePerWay)) - 1)
	if waysToSet >= len(cacheWays) {
		fmt.Fprintf(os.Stderr, "LLC size is too large, maximum is %.2f MB\n", maximumLlcSize)
		slog.Error("LLC size is too large", slog.Float64("llc size", llcSize), slog.Float64("current llc size", currentLlcSize))
		return
	}
	// set the LLC size
	setScript := script.ScriptDefinition{
		Name:          "set LLC size",
		Script:        fmt.Sprintf("wrmsr -a 0xC90 %d", cacheWays[waysToSet]),
		Superuser:     true,
		Architectures: []string{"x86_64"},
		Families:      []string{"6"}, // Intel only
		Depends:       []string{"wrmsr"},
		Lkms:          []string{"msr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to set LLC size: %v\n", err)
	}
}

func setCoreFrequency(coreFrequency float64, myTarget target.Target, localTempDir string) {
	fmt.Printf("set core frequency to %.1f GHz on %s\n", coreFrequency, myTarget.GetName())
	freqInt := uint64(coreFrequency * 10)
	var msr uint64
	for i := 0; i < 8; i++ {
		msr = msr | freqInt<<uint(i*8)
	}
	setScript := script.ScriptDefinition{
		Name:          "set frequency bins",
		Script:        fmt.Sprintf("wrmsr -a 0x1AD %d", msr),
		Superuser:     true,
		Architectures: []string{"x86_64"},
		Families:      []string{"6"}, // Intel only
		Depends:       []string{"wrmsr"},
	}
	_, err := runScript(myTarget, setScript, localTempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to set core frequency: %v\n", err)
	}
}

func setUncoreFrequency(maxFreq bool, uncoreFrequency float64, myTarget target.Target, localTempDir string) {
	var minmax string
	if maxFreq {
		minmax = "max"
	} else {
		minmax = "min"
	}
	fmt.Printf("set uncore %s frequency to %.1f GHz on %s\n", minmax, uncoreFrequency, myTarget.GetName())
	scripts := []script.ScriptDefinition{}
	scripts = append(scripts, script.GetScriptByName(script.LscpuScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciBitsScriptName))
	scripts = append(scripts, script.GetScriptByName(script.LspciDevicesScriptName))
	scripts = append(scripts, script.GetScriptByName(script.UncoreMaxFromMSRScriptName))
	scripts = append(scripts, script.GetScriptByName(script.UncoreMinFromMSRScriptName))
	scripts = append(scripts, script.GetScriptByName(script.UncoreMaxFromTPMIScriptName))
	scripts = append(scripts, script.GetScriptByName(script.UncoreMinFromTPMIScriptName))
	scripts = append(scripts, script.ScriptDefinition{
		Name:          "get uncore frequency MSR",
		Script:        "rdmsr 0x620",
		Lkms:          []string{"msr"},
		Architectures: []string{"x86_64"},
		Families:      []string{"6"}, // Intel only
		Depends:       []string{"rdmsr"},
		Superuser:     true,
	})
	outputs, err := script.RunScripts(myTarget, scripts, true, localTempDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		slog.Error("failed to run scripts on target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
		return
	}
	targetFamily, err := myTarget.GetFamily()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting target family: %v\n", err)
		slog.Error("failed to get target family", slog.String("error", err.Error()))
		return
	}
	targetModel, err := myTarget.GetModel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting target model: %v\n", err)
		slog.Error("failed to get target model", slog.String("error", err.Error()))
		return
	}
	if targetFamily == "6" && (targetModel == "173" || targetModel == "175") { //Intel, GNR and SRF only
		value := uint64(uncoreFrequency * 10)
		var bits string
		if maxFreq {
			bits = "8:14" // bits 8:14 are the max frequency
		} else {
			bits = "15:21" // bits 15:21 are the min frequency
		}
		setScript := script.ScriptDefinition{
			Name:          "write max and min uncore frequency TPMI",
			Script:        fmt.Sprintf("pcm-tpmi 2 0x18 -d -b %s -w %d", bits, value),
			Architectures: []string{"x86_64"},
			Families:      []string{"6"}, // Intel only
			Depends:       []string{"pcm-tpmi"},
			Superuser:     true,
		}
		_, err = runScript(myTarget, setScript, localTempDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to set uncore frequency: %v\n", err)
		}
	} else if targetFamily == "6" { // Intel only
		msrHex := strings.TrimSpace(outputs["get uncore frequency MSR"].Stdout)
		msrInt, err := strconv.ParseInt(msrHex, 16, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			slog.Error("failed to read or parse msr value", slog.String("msr", msrHex), slog.String("error", err.Error()))
			return
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
			Name:          "set uncore frequency MSR",
			Script:        fmt.Sprintf("wrmsr -a 0x620 %d", newVal),
			Superuser:     true,
			Architectures: []string{"x86_64"},
			Families:      []string{"6"}, // Intel only
			Lkms:          []string{"msr"},
			Depends:       []string{"wrmsr"},
		}
		_, err = runScript(myTarget, setScript, localTempDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to set uncore frequency: %v\n", err)
		}
	}
}

func setPower(power int, myTarget target.Target, localTempDir string) {
	fmt.Printf("set power to %d Watts on %s\n", power, myTarget.GetName())
	readScript := script.ScriptDefinition{
		Name:          "get power MSR",
		Script:        "rdmsr 0x610",
		Superuser:     true,
		Architectures: []string{"x86_64"},
		Families:      []string{"6"}, // Intel only
		Lkms:          []string{"msr"},
		Depends:       []string{"rdmsr"},
	}
	readOutput, err := script.RunScript(myTarget, readScript, localTempDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		slog.Error("failed to run script on target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
	} else {
		msrHex := strings.TrimSpace(readOutput.Stdout)
		msrInt, err := strconv.ParseInt(msrHex, 16, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			slog.Error("failed to parse msr value", slog.String("msr", msrHex), slog.String("error", err.Error()))
		} else {
			// mask out lower 14 bits
			newVal := uint64(msrInt) & 0xFFFFFFFFFFFFC000
			// add in the new power value
			newVal = newVal | uint64(power*8)
			setScript := script.ScriptDefinition{
				Name:          "set tdp",
				Script:        fmt.Sprintf("wrmsr -a 0x610 %d", newVal),
				Superuser:     true,
				Architectures: []string{"x86_64"},
				Families:      []string{"6"}, // Intel only
				Lkms:          []string{"msr"},
				Depends:       []string{"wrmsr"},
			}
			_, err := runScript(myTarget, setScript, localTempDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to set power: %v\n", err)
			}
		}
	}
}

func setEpb(epb int, myTarget target.Target, localTempDir string) {
	fmt.Printf("set energy performance bias (EPB) to %d on %s\n", epb, myTarget.GetName())
	setScript := script.ScriptDefinition{
		Name:          "set epb",
		Script:        fmt.Sprintf("wrmsr -a 0x1B0 %d", epb),
		Superuser:     true,
		Architectures: []string{"x86_64"},
		Families:      []string{"6"}, // Intel only
		Lkms:          []string{"msr"},
		Depends:       []string{"wrmsr"},
	}
	_, err := runScript(myTarget, setScript, localTempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to set EPB: %v\n", err)
	}
}

func setEpp(epp int, myTarget target.Target, localTempDir string) {
	fmt.Printf("set energy performance profile (EPP) to %d on %s\n", epp, myTarget.GetName())
	// Mark the per-processor EPP values as invalid, so that the
	// package EPP value is used. Then set the package EPP value.
	// Reference: 15.4.4 Managing HWP in the Intel SDM

	// get the current value of the IAEW_HWP_REQUEST MSR that includes the current EPP valid value in bit 60
	getScript := script.ScriptDefinition{
		Name:          "get epp msr",
		Script:        "rdmsr 0x774", // IA32_HWP_REQUEST
		Architectures: []string{"x86_64"},
		Families:      []string{"6"}, // Intel only
		Lkms:          []string{"msr"},
		Depends:       []string{"rdmsr"},
		Superuser:     true,
	}
	stdout, err := runScript(myTarget, getScript, localTempDir)
	if err != nil {
		return
	}
	msrValue, err := strconv.ParseUint(strings.TrimSpace(stdout), 16, 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		slog.Error("failed to parse msr value", slog.String("msr", stdout), slog.String("error", err.Error()))
		return
	}
	// clear bit 60 in the IA32_HWP_REQUEST MSR value
	maskedValue := msrValue & 0xEFFFFFFFFFFFFFFF
	// write it back to the MSR
	setScript := script.ScriptDefinition{
		Name:          "set epp valid",
		Script:        fmt.Sprintf("wrmsr -a 0x774 %d", maskedValue),
		Superuser:     true,
		Architectures: []string{"x86_64"},
		Families:      []string{"6"}, // Intel only
		Lkms:          []string{"msr"},
		Depends:       []string{"wrmsr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to set EPP valid: %v\n", err)
		return
	}

	// get the current value of the IA32_HWP_REQUEST_PKG MSR that includes the current package EPP value
	getScript = script.ScriptDefinition{
		Name:          "get epp pkg msr",
		Script:        "rdmsr 0x772", // IA32_HWP_REQUEST_PKG
		Architectures: []string{"x86_64"},
		Families:      []string{"6"}, // Intel only
		Lkms:          []string{"msr"},
		Depends:       []string{"rdmsr"},
		Superuser:     true,
	}
	stdout, err = runScript(myTarget, getScript, localTempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get EPP: %v\n", err)
		return
	}
	msrValue, err = strconv.ParseUint(strings.TrimSpace(stdout), 16, 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		slog.Error("failed to parse msr value", slog.String("msr", stdout), slog.String("error", err.Error()))
		return
	}
	// mask out bits 24-31 IA32_HWP_REQUEST_PKG MSR value
	maskedValue = msrValue & 0xFFFFFFFF00FFFFFF
	// put the EPP value in bits 24-31
	eppValue := maskedValue | uint64(epp)<<24
	// write it back to the MSR
	setScript = script.ScriptDefinition{
		Name:          "set epp",
		Script:        fmt.Sprintf("wrmsr -a 0x772 %d", eppValue),
		Superuser:     true,
		Architectures: []string{"x86_64"},
		Families:      []string{"6"}, // Intel only
		Lkms:          []string{"msr"},
		Depends:       []string{"wrmsr"},
	}
	_, err = runScript(myTarget, setScript, localTempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to set EPP: %v\n", err)
	}
}

func setGovernor(governor string, myTarget target.Target, localTempDir string) {
	fmt.Printf("set governor to %s on %s\n", governor, myTarget.GetName())
	setScript := script.ScriptDefinition{
		Name:      "set governor",
		Script:    fmt.Sprintf("echo %s | tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor", governor),
		Superuser: true,
	}
	_, err := runScript(myTarget, setScript, localTempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to set governor: %v\n", err)
	}
}

func setElc(elc string, myTarget target.Target, localTempDir string) {
	fmt.Printf("set efficiency latency control (ELC) mode to %s on %s\n", elc, myTarget.GetName())
	var mode string
	if elc == elcOptions[0] {
		mode = "latency-optimized-mode"
	} else if elc == elcOptions[1] {
		mode = "default"
	} else {
		fmt.Fprintf(os.Stderr, "invalid elc mode: %s\n", elc)
		slog.Error("invalid elc mode", slog.String("elc", elc))
		return
	}
	setScript := script.ScriptDefinition{
		Name:          "set elc",
		Script:        fmt.Sprintf("bhs-power-mode.sh --%s", mode),
		Superuser:     true,
		Architectures: []string{"x86_64"},
		Families:      []string{"6"},          // Intel only
		Models:        []string{"173", "175"}, // GNR and SRF only
		Depends:       []string{"bhs-power-mode.sh"},
	}
	_, err := runScript(myTarget, setScript, localTempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to set ELC mode: %v\n", err)
	}
}

func runScript(myTarget target.Target, myScript script.ScriptDefinition, localTempDir string) (string, error) {
	output, err := script.RunScript(myTarget, myScript, localTempDir)
	if err != nil {
		slog.Error("failed to run script on target", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()), slog.String("stdout", output.Stdout), slog.String("stderr", output.Stderr))
	} else {
		slog.Debug("ran script on target", slog.String("target", myTarget.GetName()), slog.String("script", myScript.Name), slog.String("stdout", output.Stdout), slog.String("stderr", output.Stderr))
	}
	return output.Stdout, err
}
