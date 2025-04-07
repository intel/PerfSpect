// Package metrics is a subcommand of the root command. It provides functionality to monitor core and uncore metrics on one target.
package metrics

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"perfspect/internal/common"
	"perfspect/internal/progress"
	"perfspect/internal/script"
	"perfspect/internal/target"
	"perfspect/internal/util"

	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const cmdName = "metrics"

var examples = []string{
	fmt.Sprintf("  Metrics from local host:                  $ %s %s --duration 30", common.AppName, cmdName),
	fmt.Sprintf("  Metrics from local host in CSV format:    $ %s %s --format csv", common.AppName, cmdName),
	fmt.Sprintf("  Metrics from remote host:                 $ %s %s --target 192.168.1.1 --user fred --key fred_key", common.AppName, cmdName),
	fmt.Sprintf("  Metrics for \"hot\" processes:              $ %s %s --scope process", common.AppName, cmdName),
	fmt.Sprintf("  Metrics for specified processes:          $ %s %s --scope process --pids 1234,6789", common.AppName, cmdName),
	fmt.Sprintf("  Start application and collect metrics:    $ %s %s -- /path/to/myapp arg1 arg2", common.AppName, cmdName),
	fmt.Sprintf("  Metrics adjusted for transaction rate:    $ %s %s --txnrate 100", common.AppName, cmdName),
	fmt.Sprintf("  \"Live\" metrics:                           $ %s %s --live", common.AppName, cmdName),
}

var Cmd = &cobra.Command{
	Use:           cmdName,
	Short:         "Monitor core and uncore metrics from one target",
	Long:          "",
	Example:       strings.Join(examples, "\n"),
	RunE:          runCmd,
	PreRunE:       validateFlags,
	GroupID:       "primary",
	Args:          cobra.ArbitraryArgs,
	SilenceErrors: true,
}

//go:embed resources
var resources embed.FS

// globals
var (
	gSignalMutex    sync.Mutex
	gSignalReceived bool
)

func setSignalReceived() {
	gSignalMutex.Lock()
	defer gSignalMutex.Unlock()
	gSignalReceived = true
}

func getSignalReceived() bool {
	for range 10 {
		gSignalMutex.Lock()
		received := gSignalReceived
		gSignalMutex.Unlock()
		if received {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return gSignalReceived
}

var (
	// collection options
	flagDuration int
	flagScope    string
	flagPidList  []string
	flagCidList  []string
	flagFilter   string
	flagCount    int
	flagRefresh  int
	// output format options
	flagGranularity     string
	flagOutputFormat    []string
	flagLive            bool
	flagTransactionRate float64
	// advanced options
	flagShowMetricNames   bool
	flagMetricsList       []string
	flagEventFilePath     string
	flagMetricFilePath    string
	flagPerfPrintInterval int
	flagPerfMuxInterval   int
	flagNoRoot            bool
	flagWriteEventsToFile bool
	flagInput             string

	// positional arguments
	argsApplication []string
)

const (
	flagDurationName = "duration"
	flagScopeName    = "scope"
	flagPidListName  = "pids"
	flagCidListName  = "cids"
	flagFilterName   = "filter"
	flagCountName    = "count"
	flagRefreshName  = "refresh"

	flagGranularityName     = "granularity"
	flagOutputFormatName    = "format"
	flagLiveName            = "live"
	flagTransactionRateName = "txnrate"

	flagShowMetricNamesName   = "list"
	flagMetricsListName       = "metrics"
	flagEventFilePathName     = "eventfile"
	flagMetricFilePathName    = "metricfile"
	flagPerfPrintIntervalName = "interval"
	flagPerfMuxIntervalName   = "muxinterval"
	flagNoRootName            = "noroot"
	flagWriteEventsToFileName = "raw"
	flagInputName             = "input"
)

const (
	granularitySystem = "system"
	granularitySocket = "socket"
	granularityCPU    = "cpu"
)

var granularityOptions = []string{granularitySystem, granularitySocket, granularityCPU}

const (
	scopeSystem  = "system"
	scopeProcess = "process"
	scopeCgroup  = "cgroup"
)

var scopeOptions = []string{scopeSystem, scopeProcess, scopeCgroup}

const (
	formatTxt  = "txt"
	formatCSV  = "csv"
	formatJSON = "json"
	formatWide = "wide"
)

var formatOptions = []string{formatTxt, formatCSV, formatJSON, formatWide}

func init() {
	Cmd.Flags().IntVar(&flagDuration, flagDurationName, 0, "")
	Cmd.Flags().StringVar(&flagScope, flagScopeName, scopeSystem, "")
	Cmd.Flags().StringSliceVar(&flagPidList, flagPidListName, []string{}, "")
	Cmd.Flags().StringSliceVar(&flagCidList, flagCidListName, []string{}, "")
	Cmd.Flags().StringVar(&flagFilter, flagFilterName, "", "")
	Cmd.Flags().IntVar(&flagCount, flagCountName, 5, "")
	Cmd.Flags().IntVar(&flagRefresh, flagRefreshName, 30, "")

	Cmd.Flags().StringVar(&flagGranularity, flagGranularityName, granularitySystem, "")
	Cmd.Flags().StringSliceVar(&flagOutputFormat, flagOutputFormatName, []string{formatCSV}, "")
	Cmd.Flags().BoolVar(&flagLive, flagLiveName, false, "")
	Cmd.Flags().Float64Var(&flagTransactionRate, flagTransactionRateName, 0, "")

	Cmd.Flags().BoolVar(&flagShowMetricNames, flagShowMetricNamesName, false, "")
	Cmd.Flags().StringSliceVar(&flagMetricsList, flagMetricsListName, []string{}, "")
	Cmd.Flags().StringVar(&flagEventFilePath, flagEventFilePathName, "", "")
	Cmd.Flags().StringVar(&flagMetricFilePath, flagMetricFilePathName, "", "")
	Cmd.Flags().IntVar(&flagPerfPrintInterval, flagPerfPrintIntervalName, 5, "")
	Cmd.Flags().IntVar(&flagPerfMuxInterval, flagPerfMuxIntervalName, 125, "")
	Cmd.Flags().BoolVar(&flagNoRoot, flagNoRootName, false, "")
	Cmd.Flags().BoolVar(&flagWriteEventsToFile, flagWriteEventsToFileName, false, "")
	Cmd.Flags().StringVar(&flagInput, flagInputName, "", "")

	common.AddTargetFlags(Cmd)

	Cmd.SetUsageFunc(usageFunc)
}

func usageFunc(cmd *cobra.Command) error {
	cmd.Printf("Usage: %s [flags] [-- application args]\n\n", cmd.CommandPath())
	cmd.Printf("Examples:\n%s\n\n", cmd.Example)
	cmd.Println("Arguments:")
	cmd.Printf("  application (optional): path to an application to run and collect metrics for\n\n")
	cmd.Println("Flags:")
	for _, group := range getFlagGroups() {
		cmd.Printf("  %s:\n", group.GroupName)
		for _, flag := range group.Flags {
			flagDefault := ""
			if cmd.Flags().Lookup(flag.Name).DefValue != "" {
				flagDefault = fmt.Sprintf(" (default: %s)", cmd.Flags().Lookup(flag.Name).DefValue)
			}
			cmd.Printf("    --%-20s %s%s\n", flag.Name, flag.Help, flagDefault)
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
	var groups []common.FlagGroup
	// collection options
	flags := []common.Flag{
		{
			Name: flagDurationName,
			Help: "number of seconds to run the collection. If 0, the collection will run indefinitely.",
		},
		{
			Name: flagScopeName,
			Help: fmt.Sprintf("scope of collection, options: %s", strings.Join(scopeOptions, ", ")),
		},
		{
			Name: flagPidListName,
			Help: "comma separated list of process ids. If not provided while collecting in process scope, \"hot\" processes will be monitored.",
		},
		{
			Name: flagCidListName,
			Help: "comma separated list of cids. If not provided while collecting at cgroup scope, \"hot\" cgroups will be monitored.",
		},
		{
			Name: flagFilterName,
			Help: "regular expression used to match process names or cgroup IDs when in process or cgroup scope and when --pids or --cids are not specified",
		},
		{
			Name: flagCountName,
			Help: "maximum number of \"hot\" or \"filtered\" processes or cgroups to monitor",
		},
		{
			Name: flagRefreshName,
			Help: "number of seconds to run before refreshing the \"hot\" or \"filtered\" process or cgroup list",
		},
	}
	groups = append(groups, common.FlagGroup{
		GroupName: "Collection Options",
		Flags:     flags,
	})
	// output options
	flags = []common.Flag{
		{
			Name: flagGranularityName,
			Help: fmt.Sprintf("level of metric granularity. Only valid when collecting at system scope. Options: %s.", strings.Join(granularityOptions, ", ")),
		},
		{
			Name: flagOutputFormatName,
			Help: fmt.Sprintf("output formats, options: %s", strings.Join(formatOptions, ", ")),
		},
		{
			Name: flagLiveName,
			Help: fmt.Sprintf("print metrics to stdout in one output format specified with the --%s flag. No metrics files will be written.", flagOutputFormatName),
		},
		{
			Name: flagTransactionRateName,
			Help: "number of transactions per second. Will divide relevant metrics by transactions/second.",
		},
	}
	groups = append(groups, common.FlagGroup{
		GroupName: "Output Options",
		Flags:     flags,
	})
	// advanced options
	flags = []common.Flag{
		{
			Name: flagShowMetricNamesName,
			Help: "show metric names available on this platform and exit",
		},
		{
			Name: flagMetricsListName,
			Help: "a comma separated list of quoted metric names to include in output",
		},
		{
			Name: flagEventFilePathName,
			Help: "perf event definition file. Will override default event definitions.",
		},
		{
			Name: flagMetricFilePathName,
			Help: "metric definition file. Will override default metric definitions.",
		},
		{
			Name: flagPerfPrintIntervalName,
			Help: "event collection interval in seconds",
		},
		{
			Name: flagPerfMuxIntervalName,
			Help: "multiplexing interval in milliseconds",
		},
		{
			Name: flagNoRootName,
			Help: "do not elevate to root",
		},
		{
			Name: flagWriteEventsToFileName,
			Help: "write raw perf events to file",
		},
		{
			Name: flagInputName,
			Help: "path to a file or directory with json file containing raw perf events. Will skip data collection and use raw data for reports.",
		},
	}
	groups = append(groups, common.FlagGroup{
		GroupName: "Advanced Options",
		Flags:     flags,
	})
	groups = append(groups, common.GetTargetFlagGroup())
	return groups
}

func validateFlags(cmd *cobra.Command, args []string) error {
	// some flags will not be valid if an application argument is provided
	if len(args) > 0 {
		argsApplication = args
		if flagDuration > 0 {
			err := fmt.Errorf("duration is not supported with an application argument")
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		if len(flagPidList) > 0 || len(flagCidList) > 0 {
			err := fmt.Errorf("pids and cids are not supported with an application argument")
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		if flagFilter != "" {
			err := fmt.Errorf("filter is not supported with an application argument")
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
	}
	// confirm valid duration
	if cmd.Flags().Lookup(flagDurationName).Changed && flagDuration < 0 {
		err := fmt.Errorf("duration must be a positive integer")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	if cmd.Flags().Lookup(flagDurationName).Changed && flagDuration != 0 && flagDuration < flagPerfPrintInterval {
		err := fmt.Errorf("duration must be greater than or equal to the event collection interval (%ds)", flagPerfPrintInterval)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// confirm valid scope
	if cmd.Flags().Lookup(flagScopeName).Changed && !slices.Contains(scopeOptions, flagScope) {
		err := fmt.Errorf("invalid scope: %s, valid options are: %s", flagScope, strings.Join(scopeOptions, ", "))
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// pids and cids are mutually exclusive
	if len(flagPidList) > 0 && len(flagCidList) > 0 {
		err := fmt.Errorf("cannot specify both pids and cids")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// pids only when scope is process
	if len(flagPidList) > 0 {
		// if scope was set and it wasn't set to process, error
		if cmd.Flags().Changed(flagScopeName) && flagScope != scopeProcess {
			err := fmt.Errorf("cannot specify pids when scope is not %s", scopeProcess)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		// if scope wasn't set, set it to process
		flagScope = scopeProcess
		// verify PIDs are integers
		for _, pid := range flagPidList {
			if _, err := strconv.Atoi(pid); err != nil {
				err := fmt.Errorf("pids must be integers")
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}
		}
	}
	// cids only when scope is cgroup
	if len(flagCidList) > 0 {
		// if scope was set and it wasn't set to cgroup, error
		if cmd.Flags().Changed(flagScopeName) && flagScope != scopeCgroup {
			err := fmt.Errorf("cannot specify cids when scope is not %s", scopeCgroup)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		// if scope wasn't set, set it to cgroup
		flagScope = scopeCgroup
	}
	// filter only no cids or pids
	if flagFilter != "" && (len(flagPidList) > 0 || len(flagCidList) > 0) {
		err := fmt.Errorf("cannot specify filter when pids or cids are specified")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// filter only when scope is process or cgroup
	if flagFilter != "" && (flagScope != scopeProcess && flagScope != scopeCgroup) {
		err := fmt.Errorf("cannot specify filter when scope is not %s or %s", scopeProcess, scopeCgroup)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// count must be positive
	if flagCount < 1 {
		err := fmt.Errorf("count must be a positive integer")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// count only when scope is process or cgroup
	if cmd.Flags().Lookup(flagCountName).Changed && flagCount != 5 && (flagScope != scopeProcess && flagScope != scopeCgroup) {
		err := fmt.Errorf("cannot specify count when scope is not %s or %s", scopeProcess, scopeCgroup)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// refresh must be greater than perf print interval
	if flagRefresh < flagPerfPrintInterval {
		err := fmt.Errorf("refresh must be greater than or equal to the event collection interval (%ds)", flagPerfPrintInterval)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}

	// output options
	// confirm valid granularity
	if cmd.Flags().Lookup(flagGranularityName).Changed && !slices.Contains(granularityOptions, flagGranularity) {
		err := fmt.Errorf("invalid granularity: %s, valid options are: %s", flagGranularity, strings.Join(granularityOptions, ", "))
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// if scope is not system, granularity must be system
	if flagGranularity != granularitySystem && flagScope != scopeSystem {
		err := fmt.Errorf("granularity option must be %s when collecting at a scope other than %s", granularitySystem, scopeSystem)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// confirm valid output format
	for _, format := range flagOutputFormat {
		if !slices.Contains(formatOptions, format) {
			err := fmt.Errorf("invalid output format: %s, valid options are: %s", format, strings.Join(formatOptions, ", "))
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
	}
	// advanced options
	// confirm valid perf print interval
	if cmd.Flags().Lookup(flagPerfPrintIntervalName).Changed && flagPerfPrintInterval < 1 {
		err := fmt.Errorf("event collection interval must be at least 1 second")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// confirm valid perf mux interval
	if cmd.Flags().Lookup(flagPerfMuxIntervalName).Changed && flagPerfMuxInterval < 10 {
		err := fmt.Errorf("mux interval must be at least 10 milliseconds")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// print events to file
	if flagWriteEventsToFile && flagLive {
		err := fmt.Errorf("cannot write raw perf events to file when --%s is set", flagLiveName)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// only one output format if live
	if flagLive && len(flagOutputFormat) > 1 {
		err := fmt.Errorf("specify one output format with --%s <format> when --%s is set", flagOutputFormatName, flagLiveName)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	// common target flags
	if err := common.ValidateTargetFlags(cmd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	return nil
}

type targetContext struct {
	target              target.Target
	err                 error
	perfPath            string
	metadata            Metadata
	nmiDisabled         bool
	perfMuxIntervalsSet bool
	perfMuxIntervals    map[string]int
	groupDefinitions    []GroupDefinition
	metricDefinitions   []MetricDefinition
	printedFiles        []string
	perfStartTime       time.Time
}

type targetError struct {
	target target.Target
	err    error
}

func readRawData(directory string) (metadata Metadata, eventFile *os.File, err error) {
	var metadataPath string
	var eventPath string
	fileInfo, err := os.Stat(directory)
	if err != nil {
		err = fmt.Errorf("failed to get file info: %v", err)
		return
	}
	if !fileInfo.IsDir() {
		err = fmt.Errorf("input must be a directory")
		return
	}
	var files []os.DirEntry
	files, err = os.ReadDir(directory)
	if err != nil {
		err = fmt.Errorf("failed to read raw file directory: %v", err)
		return
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if strings.HasSuffix(file.Name(), "_metadata.json") {
			metadataPath = directory + "/" + file.Name()
		} else if strings.HasSuffix(file.Name(), "_events.json") {
			eventPath = directory + "/" + file.Name()
		}
	}
	if metadataPath == "" {
		err = fmt.Errorf("metadata file not found in %s", directory)
		return
	}
	if eventPath == "" {
		err = fmt.Errorf("events file not found in %s", directory)
		return
	}
	metadata, err = ReadJSONFromFile(metadataPath)
	if err != nil {
		err = fmt.Errorf("failed to read metadata from file: %v", err)
		return
	}
	eventFile, err = os.Open(eventPath)
	if err != nil {
		err = fmt.Errorf("failed to open events file: %v", err)
		return
	}
	return
}
func readLine(file *os.File) ([]byte, error) {
	var line []byte
	buf := make([]byte, 1)
	for {
		_, err := file.Read(buf)
		if err != nil {
			return line, err
		}
		if buf[0] == '\n' {
			break
		}
		line = append(line, buf[0])
	}
	return line, nil
}
func readNextEventFrame(file *os.File) ([][]byte, error) {
	// read one line at a time
	// line looks like this:
	// {"interval" : 5.005070723, "counter-value" ...
	// if the interval value changes, we're done until the next call so need to back up one line in the file
	re := regexp.MustCompile(`"interval" : ([0-9.]+)`)
	var section [][]byte
	var lastInterval string
	for {
		// Get the current offset
		offset, _ := file.Seek(0, io.SeekCurrent)
		line, err := readLine(file)
		if err != nil {
			if err == io.EOF {
				return section, nil
			}
			return nil, err
		}
		match := re.FindSubmatch(line)
		if len(match) < 2 {
			err = fmt.Errorf("failed to find interval in line: %s", line)
			return nil, err
		}
		// if the interval changes, we're done with this section
		if lastInterval != "" && lastInterval != string(match[1]) {
			// seek back to the beginning of the last line
			_, err := file.Seek(offset, io.SeekStart)
			if err != nil {
				return nil, err
			}
			return section, nil
		}

		// Append the line to the section
		section = append(section, line)

		// Save the interval
		lastInterval = string(match[1])
	}
}
func processRawData(localOutputDir string) error {
	metadata, eventsFile, err := readRawData(flagInput)
	if err != nil {
		return err
	}
	defer eventsFile.Close()
	// load event definitions
	var eventGroupDefinitions []GroupDefinition
	var uncollectableEvents []string
	if eventGroupDefinitions, uncollectableEvents, err = LoadEventGroups(flagEventFilePath, metadata); err != nil {
		err = fmt.Errorf("failed to load event definitions: %w", err)
		return err
	}
	// load metric definitions
	var loadedMetrics []MetricDefinition
	if loadedMetrics, err = LoadMetricDefinitions(flagMetricFilePath, flagMetricsList, metadata); err != nil {
		err = fmt.Errorf("failed to load metric definitions: %w", err)
		return err
	}
	// configure metrics
	var metricDefinitions []MetricDefinition
	if metricDefinitions, err = ConfigureMetrics(loadedMetrics, uncollectableEvents, GetEvaluatorFunctions(), metadata); err != nil {
		err = fmt.Errorf("failed to configure metrics: %w", err)
		return err
	}

	var filesWritten []string

	var frameTimestamp float64
	frameCount := 1
	for {
		bytes, err := readNextEventFrame(eventsFile)
		if err != nil {
			return err
		}
		if len(bytes) == 0 {
			break
		}
		var metricFrames []MetricFrame
		metricFrames, frameTimestamp, err = ProcessEvents(bytes, eventGroupDefinitions, metricDefinitions, []Process{}, frameTimestamp, metadata)
		if err != nil {
			return err
		}
		filesWritten = printMetrics(metricFrames, frameCount, metadata.Hostname, metadata.CollectionStartTime, localOutputDir)
		frameCount += len(metricFrames)
	}
	summaryFiles, err := summarizeMetrics(localOutputDir, metadata.Hostname, metadata)
	if err != nil {
		return err
	}
	filesWritten = append(filesWritten, summaryFiles...)
	printOutputFileNames([][]string{filesWritten})
	return nil
}
func runCmd(cmd *cobra.Command, args []string) error {
	// appContext is the application context that holds common data and resources.
	appContext := cmd.Parent().Context().Value(common.AppContext{}).(common.AppContext)
	localTempDir := appContext.LocalTempDir
	localOutputDir := appContext.OutputDir
	// handle signals
	// child processes will exit when the signals are received which will
	// allow this app to exit normally
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChannel
		setSignalReceived()
		slog.Info("received signal", slog.String("signal", sig.String()))
		// send kill signal to children
		util.SignalChildren(syscall.SIGKILL)
	}()
	if flagInput != "" {
		// create output directory
		err := common.CreateOutputDir(localOutputDir)
		if err != nil {
			err = fmt.Errorf("failed to create output directory: %w", err)
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
			cmd.SilenceUsage = true
			return err
		}
		// skip data collection and use raw data for reports
		err = processRawData(localOutputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
		return nil
	}
	// round up to next perfPrintInterval second (the collection interval used by perf stat)
	if flagDuration != 0 {
		qf := float64(flagDuration) / float64(flagPerfPrintInterval)
		qi := flagDuration / flagPerfPrintInterval
		if qf > float64(qi) {
			flagDuration = (qi + 1) * flagPerfPrintInterval
		}
	}
	// get the targets
	myTargets, targetErrs, err := common.GetTargets(cmd, !flagNoRoot, !flagNoRoot, localTempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	// schedule the cleanup of the temporary directory on each target (if not debugging)
	if cmd.Parent().PersistentFlags().Lookup("debug").Value.String() != "true" {
		for _, myTarget := range myTargets {
			if myTarget.GetTempDirectory() != "" {
				defer func() {
					err := myTarget.RemoveTempDirectory()
					if err != nil {
						slog.Error("error removing target temporary directory", slog.String("error", err.Error()))
					}
				}()
			}
		}
	}
	// check for live mode with multiple targets
	if flagLive && len(myTargets) > 1 {
		err := fmt.Errorf("live mode is only supported for a single target")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	// create progress spinner
	multiSpinner := progress.NewMultiSpinner()
	for _, myTarget := range myTargets {
		err := multiSpinner.AddSpinner(myTarget.GetName())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
	}
	multiSpinner.Start()
	defer multiSpinner.Finish()
	// check for errors in target creation
	for i := range targetErrs {
		if targetErrs[i] != nil {
			_ = multiSpinner.Status(myTargets[i].GetName(), fmt.Sprintf("Error: %v", targetErrs[i]))
			// remove target from targets list
			myTargets = slices.Delete(myTargets, i, i+1)
		}
	}
	// check if any targets remain
	if len(myTargets) == 0 {
		multiSpinner.Finish() // force print the spinner before printing the error
		err := fmt.Errorf("no targets remain")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	// check if all targets have the same architecture
	for _, target := range myTargets {
		tArch, err := target.GetArchitecture()
		if err != nil {
			err = fmt.Errorf("failed to get architecture: %w", err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
		tArch0, err := myTargets[0].GetArchitecture()
		if err != nil {
			err = fmt.Errorf("failed to get architecture: %w", err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
		if tArch != tArch0 {
			err := fmt.Errorf("all targets must have the same architecture")
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
	}
	// extract perf into local temp directory (assumes all targets have the same architecture)
	localPerfPath, err := extractPerf(myTargets[0], localTempDir)
	if err != nil {
		err = fmt.Errorf("failed to extract perf: %w", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		cmd.SilenceUsage = true
		return err
	}
	// prepare the targets
	channelTargetError := make(chan targetError)
	var targetContexts []targetContext
	for _, myTarget := range myTargets {
		targetContexts = append(targetContexts, targetContext{target: myTarget})
	}
	for i := range targetContexts {
		go prepareTarget(&targetContexts[i], localTempDir, localPerfPath, channelTargetError, multiSpinner.Status, !cmd.Flags().Lookup(flagPerfMuxIntervalName).Changed)
	}
	// wait for all targets to be prepared
	numPreparedTargets := 0
	for range targetContexts {
		targetError := <-channelTargetError
		if targetError.err != nil {
			slog.Error("failed to prepare target", slog.String("target", targetError.target.GetName()), slog.String("error", targetError.err.Error()))
		} else {
			numPreparedTargets++
		}
	}
	// schedule NMI watchdog reset
	defer func() {
		for _, targetContext := range targetContexts {
			if targetContext.nmiDisabled {
				err := EnableNMIWatchdog(targetContext.target, localTempDir)
				if err != nil {
					slog.Error("failed to re-enable NMI watchdog", slog.String("target", targetContext.target.GetName()), slog.String("error", err.Error()))
				}
			}
		}
	}()
	// schedule mux interval reset
	defer func() {
		for _, targetContext := range targetContexts {
			if targetContext.perfMuxIntervalsSet {
				err := SetMuxIntervals(targetContext.target, targetContext.perfMuxIntervals, localTempDir)
				if err != nil {
					slog.Error("failed to reset perf mux intervals", slog.String("target", targetContext.target.GetName()), slog.String("error", err.Error()))
				}
			}
		}
	}()
	// check if any targets were successfully prepared
	if numPreparedTargets == 0 {
		err := fmt.Errorf("no targets were successfully prepared")
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	// prepare the metrics for each target
	for i := range targetContexts {
		go prepareMetrics(&targetContexts[i], localTempDir, channelTargetError, multiSpinner.Status)
	}
	// wait for all metrics to be prepared
	numTargetsWithPreparedMetrics := 0
	for _, ctx := range targetContexts {
		targetError := <-channelTargetError
		if targetError.err != nil {
			slog.Error("failed to prepare metrics", slog.String("target", targetError.target.GetName()), slog.String("error", targetError.err.Error()))
			_ = multiSpinner.Status(ctx.target.GetName(), fmt.Sprintf("Error: %v", targetError.err))
		} else {
			numTargetsWithPreparedMetrics++
		}
	}
	if numTargetsWithPreparedMetrics == 0 {
		err := fmt.Errorf("no targets had metrics prepared")
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	// show metric names and exit, if requested
	if flagShowMetricNames {
		// stop the multiSpinner
		multiSpinner.Finish()
		for _, targetContext := range targetContexts {
			fmt.Printf("\nMetrics available on %s:\n", targetContext.target.GetName())
			for _, metric := range targetContext.metricDefinitions {
				fmt.Printf("\"%s\"\n", metric.Name)
			}
		}
		return nil
	}
	// create the local output directory
	if !flagLive {
		err = common.CreateOutputDir(localOutputDir)
		if err != nil {
			err = fmt.Errorf("failed to create output directory: %w", err)
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
			cmd.SilenceUsage = true
			return err
		}
	}
	// start the metric collection
	for i := range targetContexts {
		if targetContexts[i].err == nil {
			finalMessage := "collecting metrics"
			if flagDuration == 0 {
				finalMessage += ", press Ctrl+C to stop"
			} else {
				finalMessage += fmt.Sprintf(" for %d seconds", flagDuration)
			}
			_ = multiSpinner.Status(targetContexts[i].target.GetName(), finalMessage)
		}
		go collectOnTarget(&targetContexts[i], localTempDir, localOutputDir, channelTargetError, multiSpinner.Status)
	}
	if flagLive {
		multiSpinner.Finish()
	}
	for range targetContexts {
		targetError := <-channelTargetError
		if targetError.err != nil {
			slog.Error("failed to collect on target", slog.String("target", targetError.target.GetName()), slog.String("error", targetError.err.Error()))
		}
	}
	// finalize and stop the spinner
	for _, targetContext := range targetContexts {
		if targetContext.err == nil {
			_ = multiSpinner.Status(targetContext.target.GetName(), "collection complete")
		}
	}
	// write metadata to file
	if flagWriteEventsToFile {
		for _, targetContext := range targetContexts {
			if err = targetContext.metadata.WriteJSONToFile(localOutputDir + "/" + targetContext.target.GetName() + "_" + "metadata.json"); err != nil {
				err = fmt.Errorf("failed to write metadata to file: %w", err)
				fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
				cmd.SilenceUsage = true
				return err
			}
		}
	}
	// summarize outputs
	if !flagLive {
		multiSpinner.Finish()
		for i, ctx := range targetContexts {
			if targetContexts[i].err != nil {
				continue
			}
			myTarget := targetContexts[i].target
			summaryFiles, err := summarizeMetrics(localOutputDir, myTarget.GetName(), ctx.metadata)
			if err != nil {
				err = fmt.Errorf("failed to summarize metrics: %w", err)
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				slog.Error(err.Error())
				cmd.SilenceUsage = true
				return err
			}
			targetContexts[i].printedFiles = append(targetContexts[i].printedFiles, summaryFiles...)
		}
		// print the names of the files that were created
		allFileNames := make([][]string, len(targetContexts))
		for i, ctx := range targetContexts {
			allFileNames[i] = ctx.printedFiles
		}
		printOutputFileNames(allFileNames)
	}
	return nil
}

func printOutputFileNames(allFileNames [][]string) {
	fmt.Println()
	fmt.Println("Metric files:")
	for _, fileNames := range allFileNames {
		for _, fileName := range fileNames {
			fmt.Printf("  %s\n", fileName)
		}
	}
}

func summarizeMetrics(localOutputDir string, targetName string, metadata Metadata) ([]string, error) {
	filesCreated := []string{}
	csvMetricsFile := localOutputDir + "/" + targetName + "_" + "metrics.csv"
	// csv summary
	out, err := Summarize(csvMetricsFile, false, metadata)
	if err != nil {
		err = fmt.Errorf("failed to summarize output: %w", err)
		return filesCreated, err
	}
	csvSummaryFile := localOutputDir + "/" + targetName + "_" + "metrics_summary.csv"
	if err = os.WriteFile(csvSummaryFile, []byte(out), 0644); err != nil {
		err = fmt.Errorf("failed to write summary to file: %w", err)
		return filesCreated, err
	}
	filesCreated = append(filesCreated, csvSummaryFile)
	// html summary
	htmlSummary := (flagScope == scopeSystem || flagScope == scopeProcess) && flagGranularity == granularitySystem
	if htmlSummary {
		out, err = Summarize(csvMetricsFile, true, metadata)
		if err != nil {
			err = fmt.Errorf("failed to summarize output as HTML: %w", err)
			return filesCreated, err
		}
		htmlSummaryFile := localOutputDir + "/" + targetName + "_" + "metrics_summary.html"
		if err = os.WriteFile(htmlSummaryFile, []byte(out), 0644); err != nil {
			err = fmt.Errorf("failed to write HTML summary to file: %w", err)
			return filesCreated, err
		}
		filesCreated = append(filesCreated, htmlSummaryFile)
	}
	return filesCreated, nil
}

func prepareTarget(targetContext *targetContext, localTempDir string, localPerfPath string, channelError chan targetError, statusUpdate progress.MultiSpinnerUpdateFunc, useDefaultMuxInterval bool) {
	myTarget := targetContext.target
	var err error
	_ = statusUpdate(myTarget.GetName(), "configuring target")
	// make sure PMUs are not in use on target
	if family, err := myTarget.GetFamily(); err == nil && family == "6" {
		output, err := script.RunScript(myTarget, script.GetScriptByName(script.PMUBusyScriptName), localTempDir)
		if err != nil {
			err = fmt.Errorf("failed to check if PMUs are in use: %w", err)
			_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %v", err))
			targetContext.err = err
			channelError <- targetError{target: myTarget, err: err}
			return
		}
		for line := range strings.SplitSeq(output.Stdout, "\n") {
			// if one of the MSR registers is active (ignore cpu_cycles), then the PMU is in use
			if strings.Contains(line, "Active") && !strings.Contains(line, "0x30a") {
				err = fmt.Errorf("PMU in use on target: %s", line)
				_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %v", err))
				targetContext.err = err
				channelError <- targetError{target: myTarget, err: err}
				return
			}
		}
	}
	// check if NMI watchdog is enabled and disable it if necessary
	if !flagNoRoot {
		var nmiWatchdogEnabled bool
		if nmiWatchdogEnabled, err = NMIWatchdogEnabled(myTarget); err != nil {
			err = fmt.Errorf("failed to retrieve NMI watchdog status: %w", err)
			_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
			targetContext.err = err
			channelError <- targetError{target: myTarget, err: err}
			return
		}
		if nmiWatchdogEnabled {
			if err = DisableNMIWatchdog(myTarget, localTempDir); err != nil {
				err = fmt.Errorf("failed to disable NMI watchdog: %w", err)
				_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
				targetContext.err = err
				channelError <- targetError{target: myTarget, err: err}
				return
			}
			targetContext.nmiDisabled = true
		}
	}
	// update default mux interval to 16ms for AMD architecture
	if !flagNoRoot && useDefaultMuxInterval {
		vendor, err := myTarget.GetVendor()
		if err == nil && vendor == "AuthenticAMD" {
			flagPerfMuxInterval = 16
		}
	}
	// set perf mux interval to desired value
	if !flagNoRoot {
		if targetContext.perfMuxIntervals, err = GetMuxIntervals(myTarget, localTempDir); err != nil {
			err = fmt.Errorf("failed to get perf mux intervals: %w", err)
			_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
			targetContext.err = err
			channelError <- targetError{target: myTarget, err: err}
			return
		}
		if err = SetAllMuxIntervals(myTarget, flagPerfMuxInterval, localTempDir); err != nil {
			err = fmt.Errorf("failed to set all perf mux intervals: %w", err)
			_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
			targetContext.err = err
			channelError <- targetError{target: myTarget, err: err}
			return
		}
		targetContext.perfMuxIntervalsSet = true
	}
	// get the full path to the perf binary
	if targetContext.perfPath, err = getPerfPath(myTarget, localPerfPath); err != nil {
		err = fmt.Errorf("failed to find perf: %w", err)
		_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %v", err))
		targetContext.err = err
		channelError <- targetError{target: myTarget, err: err}
		return
	}
	slog.Info("Using Linux perf", slog.String("target", targetContext.target.GetName()), slog.String("path", targetContext.perfPath))
	channelError <- targetError{target: myTarget, err: nil}
}

func prepareMetrics(targetContext *targetContext, localTempDir string, channelError chan targetError, statusUpdate progress.MultiSpinnerUpdateFunc) {
	myTarget := targetContext.target
	if targetContext.err != nil {
		channelError <- targetError{target: myTarget, err: nil}
		return
	}
	// load metadata
	_ = statusUpdate(myTarget.GetName(), "collecting metadata")
	var err error
	if targetContext.metadata, err = LoadMetadata(myTarget, flagNoRoot, targetContext.perfPath, localTempDir); err != nil {
		_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
		targetContext.err = err
		channelError <- targetError{target: myTarget, err: err}
		return
	}
	if !targetContext.metadata.SupportsInstructions {
		slog.Info("Target does not support instructions event collection", slog.String("target", myTarget.GetName()))
		targetContext.err = fmt.Errorf("target not supported, does not support instructions event collection")
		channelError <- targetError{target: myTarget, err: targetContext.err}
		return
	}
	slog.Info(targetContext.metadata.String())
	// load event definitions
	var uncollectableEvents []string
	if targetContext.groupDefinitions, uncollectableEvents, err = LoadEventGroups(flagEventFilePath, targetContext.metadata); err != nil {
		err = fmt.Errorf("failed to load event definitions: %w", err)
		_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
		targetContext.err = err
		channelError <- targetError{target: myTarget, err: err}
		return
	}
	// load metric definitions
	var loadedMetrics []MetricDefinition
	if loadedMetrics, err = LoadMetricDefinitions(flagMetricFilePath, flagMetricsList, targetContext.metadata); err != nil {
		err = fmt.Errorf("failed to load metric definitions: %w", err)
		_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
		targetContext.err = err
		channelError <- targetError{target: myTarget, err: err}
		return
	}
	// configure metrics
	if targetContext.metricDefinitions, err = ConfigureMetrics(loadedMetrics, uncollectableEvents, GetEvaluatorFunctions(), targetContext.metadata); err != nil {
		err = fmt.Errorf("failed to configure metrics: %w", err)
		_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
		targetContext.err = err
		channelError <- targetError{target: myTarget, err: err}
		return
	}
	channelError <- targetError{target: myTarget, err: nil}
}

func collectOnTarget(targetContext *targetContext, localTempDir string, localOutputDir string, channelError chan targetError, statusUpdate progress.MultiSpinnerUpdateFunc) {
	myTarget := targetContext.target
	if targetContext.err != nil {
		channelError <- targetError{target: myTarget, err: nil}
		return
	}
	// refresh if collecting per-process/cgroup and list of PIDs/CIDs not specified
	refresh := (flagScope == scopeProcess && len(flagPidList) == 0) ||
		(flagScope == scopeCgroup && len(flagCidList) == 0)
	errorChannel := make(chan error)
	frameChannel := make(chan []MetricFrame)
	printCompleteChannel := make(chan []string)
	totalPerfRuntimeSeconds := 0 // only relevant in process scope
	// get current time for use in setting timestamps on output
	targetContext.metadata.CollectionStartTime = time.Now() // save the start time in the metadata for use when using the --input option to process raw data
	go printMetricsAsync(targetContext, localOutputDir, frameChannel, printCompleteChannel)
	var err error
	for {
		var perfCommand *exec.Cmd
		var processes []Process
		var tempErr error
		// get the perf command
		if processes, perfCommand, tempErr = getPerfCommand(myTarget, targetContext.perfPath, targetContext.groupDefinitions, localTempDir); tempErr != nil {
			if !getSignalReceived() {
				err = fmt.Errorf("failed to get perf command: %w", tempErr)
				_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
			}
			break
		}
		// this timestamp is used to determine if we need to exit the loop, i.e., we've run long enough
		targetContext.perfStartTime = time.Now()
		go runPerf(myTarget, flagNoRoot, processes, perfCommand, targetContext.groupDefinitions, targetContext.metricDefinitions, targetContext.metadata, localTempDir, localOutputDir, frameChannel, errorChannel)
		// wait for runPerf to finish
		perfErr := <-errorChannel // capture and return all errors
		if perfErr != nil {
			if !getSignalReceived() {
				err = perfErr
				_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
			}
			break
		}
		// no perf errors, continue
		perfEndTime := time.Now()
		totalPerfRuntimeSeconds += int(perfEndTime.Sub(targetContext.perfStartTime).Seconds())
		if !refresh || (flagDuration != 0 && totalPerfRuntimeSeconds >= flagDuration) {
			break
		}
	}
	close(frameChannel) // we're done writing frames so shut it down
	// wait for printing to complete
	targetContext.printedFiles = <-printCompleteChannel
	close(printCompleteChannel)
	if err != nil {
		targetContext.err = err
		channelError <- targetError{target: myTarget, err: err}
		return
	}
	channelError <- targetError{target: myTarget, err: nil}
}

func printMetrics(metricFrames []MetricFrame, frameCount int, targetName string, collectionStartTime time.Time, outputDir string) (printedFiles []string) {
	fileName, err := printMetricsTxt(metricFrames, targetName, collectionStartTime, flagLive && flagOutputFormat[0] == formatTxt, !flagLive && slices.Contains(flagOutputFormat, formatTxt), outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
	} else if fileName != "" {
		printedFiles = util.UniqueAppend(printedFiles, fileName)
	}
	fileName, err = printMetricsJSON(metricFrames, targetName, collectionStartTime, flagLive && flagOutputFormat[0] == formatJSON, !flagLive && slices.Contains(flagOutputFormat, formatJSON), outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
	} else if fileName != "" {
		printedFiles = util.UniqueAppend(printedFiles, fileName)
	}
	// csv is always written to file unless no files are requested -- we need it to create the summary reports
	fileName, err = printMetricsCSV(metricFrames, frameCount, targetName, collectionStartTime, flagLive && flagOutputFormat[0] == formatCSV, !flagLive, outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
	} else if fileName != "" {
		printedFiles = util.UniqueAppend(printedFiles, fileName)
	}
	fileName, err = printMetricsWide(metricFrames, frameCount, targetName, collectionStartTime, flagLive && flagOutputFormat[0] == formatWide, !flagLive && slices.Contains(flagOutputFormat, formatWide), outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
	} else if fileName != "" {
		printedFiles = util.UniqueAppend(printedFiles, fileName)
	}
	return printedFiles
}

// printMetricsAsync receives metric frames over the provided channel and prints them to file and stdout in the requested format.
// It exits when the channel is closed.
func printMetricsAsync(targetContext *targetContext, outputDir string, frameChannel chan []MetricFrame, doneChannel chan []string) {
	var allPrintedFiles []string
	frameCount := 1
	// block until next set of metric frames arrives, will exit loop when channel is closed
	for metricFrames := range frameChannel {
		printedFiles := printMetrics(metricFrames, frameCount, targetContext.target.GetName(), targetContext.perfStartTime, outputDir)
		for _, file := range printedFiles {
			allPrintedFiles = util.UniqueAppend(allPrintedFiles, file)
		}
		frameCount += len(metricFrames)
	}
	doneChannel <- allPrintedFiles
}

// extractPerf extracts the perf binary from the resources to the local temporary directory.
func extractPerf(myTarget target.Target, localTempDir string) (string, error) {
	// get the target architecture
	arch, err := myTarget.GetArchitecture()
	if err != nil {
		return "", fmt.Errorf("failed to get target architecture: %w", err)
	}
	// extract the perf binary
	return util.ExtractResource(script.Resources, path.Join("resources", arch, "perf"), localTempDir)
}

// getPerfPath determines the path to the `perf` binary for the given target.
// If the target is a local target, it uses the provided localPerfPath.
// If the target is remote, it checks if `perf` version 6.1 or later is available on the target.
// If available, it uses the `perf` binary on the target.
// If not available, it pushes the local `perf` binary to the target's temporary directory and uses that.
//
// Parameters:
// - myTarget: The target system where the `perf` binary is needed.
// - localPerfPath: The local path to the `perf` binary.
//
// Returns:
// - perfPath: The path to the `perf` binary on the target.
// - err: An error if any occurred during the process.
func getPerfPath(myTarget target.Target, localPerfPath string) (string, error) {
	var perfPath string
	if _, ok := myTarget.(*target.LocalTarget); ok {
		perfPath = localPerfPath
	} else {
		hasPerf := false
		cmd := exec.Command("perf", "--version")
		output, _, _, err := myTarget.RunCommand(cmd, 0, true)
		if err == nil && strings.Contains(output, "perf version") {
			// get the version number
			version := strings.Split(strings.TrimSpace(output), " ")[2]
			// split version into major and minor and build numbers
			versionParts := strings.Split(version, ".")
			if len(versionParts) >= 2 {
				major, _ := strconv.Atoi(versionParts[0])
				minor, _ := strconv.Atoi(versionParts[1])
				if major > 6 || (major == 6 && minor >= 1) {
					hasPerf = true
				}
			}
		}
		if hasPerf {
			perfPath = "perf"
		} else {
			targetTempDir := myTarget.GetTempDirectory()
			if targetTempDir == "" {
				panic("targetTempDir is empty")
			}
			if err = myTarget.PushFile(localPerfPath, targetTempDir); err != nil {
				slog.Error("failed to push perf binary to remote directory", slog.String("error", err.Error()))
				return "", err
			}
			perfPath = path.Join(targetTempDir, "perf")
		}
	}
	return perfPath, nil
}

// getPerfCommandArgs returns the command arguments for the 'perf stat' command
// based on the provided parameters.
//
// Parameters:
//   - pids: The process IDs for which to collect performance data. If flagScope is
//     set to "process", the data will be collected only for these processes.
//   - cgroups: The list of cgroups for which to collect performance data. If
//     flagScope is set to "cgroup", the data will be collected only for these cgroups.
//   - timeout: The timeout value in seconds. If flagScope is not set to "cgroup"
//     and timeout is not 0, the 'sleep' command will be added to the arguments
//     with the specified timeout value.
//   - eventGroups: The list of event groups to collect. Each event group is a
//     collection of events to be monitored.
//
// Returns:
// - args: The command arguments for the 'perf stat' command.
// - err: An error, if any.
func getPerfCommandArgs(pids string, cgroups []string, timeout int, eventGroups []GroupDefinition) (args []string, err error) {
	// -I: print interval in ms
	// -j: json formatted event output
	args = append(args, "stat", "-I", fmt.Sprintf("%d", flagPerfPrintInterval*1000), "-j")
	if flagScope == scopeSystem {
		args = append(args, "-a") // system-wide collection
		if flagGranularity == granularityCPU || flagGranularity == granularitySocket {
			args = append(args, "-A") // no aggregation
		}
	} else if flagScope == scopeProcess {
		args = append(args, "-p", pids) // collect only for these processes
	} else if flagScope == scopeCgroup {
		args = append(args, "--for-each-cgroup", strings.Join(cgroups, ",")) // collect only for these cgroups
	}
	// -e: event groups to collect
	args = append(args, "-e")
	var groups []string
	for _, group := range eventGroups {
		var events []string
		for _, event := range group {
			events = append(events, event.Raw)
		}
		groups = append(groups, fmt.Sprintf("{%s}", strings.Join(events, ",")))
	}
	args = append(args, fmt.Sprintf("'%s'", strings.Join(groups, ",")))
	if len(argsApplication) > 0 {
		// add application args
		args = append(args, "--")
		args = append(args, argsApplication...)
	} else if flagScope != scopeCgroup && timeout != 0 {
		// add timeout
		args = append(args, "sleep", fmt.Sprintf("%d", timeout))
	}
	return
}

// getPerfCommand is responsible for assembling the command that will be
// executed to collect event data
func getPerfCommand(myTarget target.Target, perfPath string, eventGroups []GroupDefinition, localTempDir string) (processes []Process, perfCommand *exec.Cmd, err error) {
	if flagScope == scopeSystem {
		var args []string
		if args, err = getPerfCommandArgs("", []string{}, flagDuration, eventGroups); err != nil {
			err = fmt.Errorf("failed to assemble perf args: %v", err)
			return
		}
		perfCommand = exec.Command(perfPath, args...) // nosemgrep
	} else if flagScope == scopeProcess {
		if len(flagPidList) > 0 {
			if processes, err = GetProcesses(myTarget, flagPidList); err != nil {
				return
			}
			if len(processes) == 0 {
				err = fmt.Errorf("failed to find processes associated with designated PIDs: %v", flagPidList)
				return
			}
		} else {
			if processes, err = GetHotProcesses(myTarget, flagCount, flagFilter); err != nil {
				return
			}
			if len(processes) == 0 {
				if flagFilter == "" {
					err = fmt.Errorf("failed to find \"hot\" processes")
					return
				} else {
					err = fmt.Errorf("failed to find processes matching filter: %s", flagFilter)
					return
				}
			}
		}
		var timeout int
		if flagDuration > 0 {
			timeout = flagDuration
		} else if len(flagPidList) == 0 { // don't refresh if PIDs are specified
			timeout = flagRefresh // refresh hot processes every flagRefresh seconds
		}
		pidList := make([]string, 0, len(processes))
		for _, process := range processes {
			pidList = append(pidList, process.pid)
		}
		var args []string
		if args, err = getPerfCommandArgs(strings.Join(pidList, ","), []string{}, timeout, eventGroups); err != nil {
			err = fmt.Errorf("failed to assemble perf args: %v", err)
			return
		}
		perfCommand = exec.Command(perfPath, args...) // nosemgrep
	} else if flagScope == scopeCgroup {
		var cgroups []string
		if len(flagCidList) > 0 {
			if cgroups, err = GetCgroups(myTarget, flagCidList, localTempDir); err != nil {
				return
			}
		} else {
			if cgroups, err = GetHotCgroups(myTarget, flagCount, flagFilter, localTempDir); err != nil {
				return
			}
		}
		if len(cgroups) == 0 {
			err = fmt.Errorf("no CIDs selected")
			return
		}
		var args []string
		if args, err = getPerfCommandArgs("", cgroups, -1, eventGroups); err != nil {
			err = fmt.Errorf("failed to assemble perf args: %v", err)
			return
		}
		perfCommand = exec.Command(perfPath, args...) // nosemgrep
	}
	return
}

// runPerf starts Linux perf using the provided command, then reads perf's output
// until perf stops. When collecting for cgroups, perf will be manually terminated if/when the
// run duration exceeds the collection time or the time when the cgroup list needs
// to be refreshed.
func runPerf(myTarget target.Target, noRoot bool, processes []Process, cmd *exec.Cmd, eventGroupDefinitions []GroupDefinition, metricDefinitions []MetricDefinition, metadata Metadata, localTempDir string, outputDir string, frameChannel chan []MetricFrame, errorChannel chan error) {
	var err error
	defer func() { errorChannel <- err }()
	cpuCount := metadata.SocketCount * metadata.CoresPerSocket * metadata.ThreadsPerCore
	outputLines := make([][]byte, 0, cpuCount*150) // a rough approximation of expected number of events
	// start perf
	perfCommand := strings.Join(cmd.Args, " ")
	stdoutChannel := make(chan string)
	stderrChannel := make(chan string)
	exitcodeChannel := make(chan int)
	scriptErrorChannel := make(chan error)
	cmdChannel := make(chan *exec.Cmd)
	slog.Debug("running perf stat", slog.String("command", perfCommand))
	go script.RunScriptAsync(myTarget, script.ScriptDefinition{Name: "perf stat", ScriptTemplate: perfCommand, Superuser: !noRoot}, localTempDir, stdoutChannel, stderrChannel, exitcodeChannel, scriptErrorChannel, cmdChannel)
	var localCommand *exec.Cmd
	select {
	case cmd := <-cmdChannel:
		localCommand = cmd
	case err := <-scriptErrorChannel:
		if err != nil {
			return
		}
	}
	// must manually terminate perf in cgroup scope when a timeout is specified and/or need to refresh cgroups
	startPerfTimestamp := time.Now()
	var cgroupTimeout int
	if flagScope == scopeCgroup && (flagDuration != 0 || len(flagCidList) == 0) {
		if flagDuration > 0 && flagDuration < flagRefresh {
			cgroupTimeout = flagDuration
		} else {
			cgroupTimeout = flagRefresh
		}
	}
	// Start a goroutine to wait for and then process perf output
	// Use a timer to determine when we received an entire frame of events from perf
	// The timer will expire when no lines (events) have been received from perf for more than 100ms. This
	// works because perf writes the events to stderr in a burst every collection interval, e.g., 5 seconds.
	// When the timer expires, this code assumes that perf is done writing events to stderr.
	perfEventWaitTime := time.Duration(100 * time.Millisecond) // 100ms is somewhat arbitrary, but is long enough for perf to print a frame of events
	// The first duration needs to be longer than the time it takes for perf to print its first line of output.
	t1 := time.NewTimer(time.Duration(2 * flagPerfPrintInterval * 1000))
	var frameTimestamp float64
	stopAnonymousFuncChannel := make(chan bool)
	go func() {
		stop := false
		for {
			select {
			case <-t1.C: // waits for timer to expire
			case <-stopAnonymousFuncChannel: // wait for signal to exit the goroutine
				stop = true // exit the loop
			}
			if !stop && len(outputLines) != 0 {
				// process the events
				var metricFrames []MetricFrame
				if metricFrames, frameTimestamp, err = ProcessEvents(outputLines, eventGroupDefinitions, metricDefinitions, processes, frameTimestamp, metadata); err != nil {
					slog.Warn(err.Error())
					outputLines = [][]byte{} // empty it
					continue
				}
				// send the metrics frames out to be printed
				frameChannel <- metricFrames
				// write the events to a file
				if flagWriteEventsToFile {
					if err = writeEventsToFile(outputDir+"/"+myTarget.GetName()+"_"+"events.json", outputLines); err != nil {
						err = fmt.Errorf("failed to write events to raw file: %v", err)
						slog.Error(err.Error())
						return
					}
				}
				// empty the outputLines
				outputLines = [][]byte{}
			}
			// for cgroup scope, terminate perf if timeout is reached
			if flagScope == scopeCgroup {
				if stop || (cgroupTimeout != 0 && int(time.Since(startPerfTimestamp).Seconds()) > cgroupTimeout) {
					err = localCommand.Process.Signal(os.Interrupt)
					if err != nil {
						err = fmt.Errorf("failed to terminate perf: %v", err)
						slog.Error(err.Error())
					}
				}
			}
			if stop {
				break
			}
		}
		// signal that the goroutine is done
		stopAnonymousFuncChannel <- true
	}()
	// receive perf output
	done := false
	for !done {
		select {
		case err := <-scriptErrorChannel: // if there is an error running perf, it comes here
			if err != nil {
				slog.Error("error from perf", slog.String("error", err.Error()))
			}
			done = true // exit the loop
		case exitCode := <-exitcodeChannel: // when perf exits, the exit code comes to this channel
			slog.Debug("perf exited", slog.Int("exit code", exitCode))
			time.Sleep(perfEventWaitTime) // wait for timer to expire so that last events can be processed
			done = true                   // exit the loop
		case line := <-stderrChannel: // perf output comes in on this channel, one line at a time
			t1.Stop()
			t1.Reset(perfEventWaitTime)
			// accumulate the lines, they will be processed in the goroutine when the timer expires
			outputLines = append(outputLines, []byte(line))
		}
	}
	t1.Stop()
	// send signal to exit the goroutine
	stopAnonymousFuncChannel <- true
	// wait for the goroutine to exit
	<-stopAnonymousFuncChannel
}
