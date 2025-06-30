// Package metrics is a subcommand of the root command. It provides functionality to monitor core and uncore metrics on one target.
package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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

	"github.com/prometheus/client_golang/prometheus"

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
	flagShowMetricNames      bool
	flagMetricsList          []string
	flagEventFilePath        string
	flagMetricFilePath       string
	flagPerfPrintInterval    int
	flagPerfMuxInterval      int
	flagNoRoot               bool
	flagWriteEventsToFile    bool
	flagInput                string
	flagNoSystemSummary      bool
	flagPrometheusServer     bool
	flagPrometheusServerAddr string

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

	flagShowMetricNamesName      = "list"
	flagMetricsListName          = "metrics"
	flagEventFilePathName        = "eventfile"
	flagMetricFilePathName       = "metricfile"
	flagPerfPrintIntervalName    = "interval"
	flagPerfMuxIntervalName      = "muxinterval"
	flagNoRootName               = "noroot"
	flagWriteEventsToFileName    = "raw"
	flagInputName                = "input"
	flagNoSystemSummaryName      = "no-summary"
	flagPrometheusServerName     = "prometheus-server"
	flagPrometheusServerAddrName = "prometheus-server-addr"
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
	Cmd.Flags().BoolVar(&flagNoSystemSummary, flagNoSystemSummaryName, false, "")
	Cmd.Flags().BoolVar(&flagPrometheusServer, flagPrometheusServerName, false, "")
	Cmd.Flags().StringVar(&flagPrometheusServerAddr, flagPrometheusServerAddrName, ":9090", "")

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
			Help: "number of seconds to run before refreshing the \"hot\" or \"filtered\" process or cgroup list. If 0, the list will not be refreshed.",
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
		{
			Name: flagPrometheusServerName,
			Help: "enable promtheus metrics server",
		},
		{
			Name: flagPrometheusServerAddrName,
			Help: "address (e.g., host:port) to start Prometheus metrics server on (implies --promtheus-server true)",
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
		{
			Name: flagNoSystemSummaryName,
			Help: "do not include system summary table in report",
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
		if cmd.Flags().Lookup(flagDurationName).Changed {
			return common.FlagValidationError(cmd, "duration is not supported with an application argument")
		}
		if cmd.Flags().Lookup(flagPidListName).Changed {
			return common.FlagValidationError(cmd, "pids are not supported with an application argument")
		}
		if cmd.Flags().Lookup(flagCidListName).Changed {
			return common.FlagValidationError(cmd, "cids are not supported with an application argument")
		}
		if cmd.Flags().Lookup(flagFilterName).Changed {
			return common.FlagValidationError(cmd, "filter is not supported with an application argument")
		}
		if cmd.Flags().Lookup(flagRefreshName).Changed {
			return common.FlagValidationError(cmd, "refresh is not supported with an application argument")
		}
		if cmd.Flags().Lookup(flagCountName).Changed {
			return common.FlagValidationError(cmd, "count is not supported with an application argument")
		}
	}
	// confirm valid duration
	if cmd.Flags().Lookup(flagDurationName).Changed && flagDuration != 0 && flagDuration < flagPerfPrintInterval {
		return common.FlagValidationError(cmd, fmt.Sprintf("duration must be greater than or equal to the event collection interval (%d)", flagPerfPrintInterval))
	}
	// confirm valid scope
	if cmd.Flags().Lookup(flagScopeName).Changed && !slices.Contains(scopeOptions, flagScope) {
		return common.FlagValidationError(cmd, fmt.Sprintf("invalid scope: %s, valid options are: %s", flagScope, strings.Join(scopeOptions, ", ")))
	}
	// pids and cids are mutually exclusive
	if len(flagPidList) > 0 && len(flagCidList) > 0 {
		return common.FlagValidationError(cmd, "cannot specify both pids and cids")
	}
	// pid list changed
	if len(flagPidList) > 0 {
		// if scope was set and it wasn't set to process, error
		if cmd.Flags().Changed(flagScopeName) && flagScope != scopeProcess {
			return common.FlagValidationError(cmd, fmt.Sprintf("cannot specify pids when scope is not %s", scopeProcess))
		}
		// if scope wasn't set, set it to process
		flagScope = scopeProcess
		// verify PIDs are integers
		for _, pid := range flagPidList {
			if _, err := strconv.Atoi(pid); err != nil {
				return common.FlagValidationError(cmd, "pids must be integers")
			}
		}
	}
	// cid list changed
	if len(flagCidList) > 0 {
		// if scope was set and it wasn't set to cgroup, error
		if cmd.Flags().Changed(flagScopeName) && flagScope != scopeCgroup {
			return common.FlagValidationError(cmd, fmt.Sprintf("cannot specify cids when scope is not %s", scopeCgroup))
		}
		// if scope wasn't set, set it to cgroup
		flagScope = scopeCgroup
	}
	// filter changed
	if flagFilter != "" {
		// if scope isn't process or cgroup, error
		if flagScope != scopeProcess && flagScope != scopeCgroup {
			return common.FlagValidationError(cmd, fmt.Sprintf("cannot specify filter when scope is not %s or %s", scopeProcess, scopeCgroup))
		}
		// if pids or cids are specified, error
		if len(flagPidList) > 0 || len(flagCidList) > 0 {
			return common.FlagValidationError(cmd, "cannot specify filter when pids or cids are specified")
		}
	}
	// count changed
	if cmd.Flags().Lookup(flagCountName).Changed {
		// if scope isn't process or cgroup, error
		if flagScope != scopeProcess && flagScope != scopeCgroup {
			return common.FlagValidationError(cmd, fmt.Sprintf("cannot specify count when scope is not %s or %s", scopeProcess, scopeCgroup))
		}
		// if count is less than 1, error
		if flagCount < 1 {
			return common.FlagValidationError(cmd, "count must be greater than 0")
		}
		// if pids or cids are specified, error
		if len(flagPidList) > 0 || len(flagCidList) > 0 {
			return common.FlagValidationError(cmd, "cannot specify count when pids or cids are specified")
		}
	}
	// refresh changed
	if cmd.Flags().Lookup(flagRefreshName).Changed {
		// if scope isn't process or cgroup, error
		if flagScope != scopeProcess && flagScope != scopeCgroup {
			return common.FlagValidationError(cmd, fmt.Sprintf("cannot specify refresh when scope is not %s or %s", scopeProcess, scopeCgroup))
		}
		// if pidlist or cidlist is set, error
		if len(flagPidList) > 0 || len(flagCidList) > 0 {
			return common.FlagValidationError(cmd, "cannot specify refresh when pids or cids are specified")
		}
		// if duration is set, error
		if flagDuration > 0 {
			return common.FlagValidationError(cmd, "cannot specify refresh when duration is set")
		}
		// if refresh is less than 1, error
		if flagRefresh < 0 {
			return common.FlagValidationError(cmd, "refresh must be greater than or equal to 0")
		}
		// if refresh is less than perf print interval, error
		if flagRefresh < flagPerfPrintInterval {
			return common.FlagValidationError(cmd, fmt.Sprintf("refresh must be greater than or equal to the event collection interval (%d)", flagPerfPrintInterval))
		}
	}
	// output options
	// confirm valid granularity
	if cmd.Flags().Lookup(flagGranularityName).Changed && !slices.Contains(granularityOptions, flagGranularity) {
		return common.FlagValidationError(cmd, fmt.Sprintf("invalid granularity: %s, valid options are: %s", flagGranularity, strings.Join(granularityOptions, ", ")))
	}
	// if scope is not system, granularity must be system
	if flagGranularity != granularitySystem && flagScope != scopeSystem {
		return common.FlagValidationError(cmd, fmt.Sprintf("granularity option must be %s when collecting at a scope other than %s", granularitySystem, scopeSystem))
	}
	// confirm valid output format
	for _, format := range flagOutputFormat {
		if !slices.Contains(formatOptions, format) {
			return common.FlagValidationError(cmd, fmt.Sprintf("invalid output format: %s, valid options are: %s", format, strings.Join(formatOptions, ", ")))
		}
	}
	// advanced options
	// confirm valid perf print interval
	if cmd.Flags().Lookup(flagPerfPrintIntervalName).Changed {
		if flagPerfPrintInterval < 1 {
			return common.FlagValidationError(cmd, "event collection interval must be at least 1 second")
		}
		// if perf print interval is greater than duration, error
		if flagDuration > 0 && flagPerfPrintInterval > flagDuration {
			return common.FlagValidationError(cmd, fmt.Sprintf("event collection interval must be less than or equal to the duration (%d)", flagDuration))
		}
		// if refresh is relevant, perf print interval must be less than refresh
		relevant := flagRefresh > 0 && flagScope != scopeSystem && len(flagPidList) == 0 && len(flagCidList) == 0
		if relevant && flagPerfPrintInterval > flagRefresh {
			return common.FlagValidationError(cmd, fmt.Sprintf("event collection interval must be less than or equal to the refresh interval (%d)", flagRefresh))
		}
	}
	// confirm valid perf mux interval
	if cmd.Flags().Lookup(flagPerfMuxIntervalName).Changed && flagPerfMuxInterval < 10 {
		return common.FlagValidationError(cmd, "mux interval must be at least 10 milliseconds")
	}
	// print events to file
	if flagWriteEventsToFile && flagLive {
		return common.FlagValidationError(cmd, fmt.Sprintf("cannot write raw perf events to file when --%s is set", flagLiveName))
	}
	// only one output format if live
	if flagLive && len(flagOutputFormat) > 1 {
		return common.FlagValidationError(cmd, fmt.Sprintf("specify one output format with --%s <format> when --%s is set", flagOutputFormatName, flagLiveName))
	}
	// event file path
	if flagEventFilePath != "" {
		if _, err := os.Stat(flagEventFilePath); err != nil {
			if os.IsNotExist(err) {
				return common.FlagValidationError(cmd, fmt.Sprintf("event file path does not exist: %s", flagEventFilePath))
			}
			return common.FlagValidationError(cmd, fmt.Sprintf("failed to access event file path: %s, error: %v", flagEventFilePath, err))
		}
	}
	// metric file path
	if flagMetricFilePath != "" {
		if _, err := os.Stat(flagMetricFilePath); err != nil {
			if os.IsNotExist(err) {
				return common.FlagValidationError(cmd, fmt.Sprintf("metric file path does not exist: %s", flagMetricFilePath))
			}
			return common.FlagValidationError(cmd, fmt.Sprintf("failed to access metric file path: %s, error: %v", flagMetricFilePath, err))
		}
	}
	// input file path
	if flagInput != "" {
		if _, err := os.Stat(flagInput); err != nil {
			if os.IsNotExist(err) {
				return common.FlagValidationError(cmd, fmt.Sprintf("input file path does not exist: %s", flagInput))
			}
			return common.FlagValidationError(cmd, fmt.Sprintf("failed to access input file path: %s, error: %v", flagInput, err))
		}
	}
	// common target flags
	if err := common.ValidateTargetFlags(cmd); err != nil {
		return common.FlagValidationError(cmd, err.Error())
	}
	// prometheus server address
	if cmd.Flags().Changed(flagPrometheusServerAddrName) {
		flagPrometheusServer = true
		_, port, err := net.SplitHostPort(flagPrometheusServerAddr)
		if err != nil {
			slog.Error(err.Error())
			err = fmt.Errorf("invalid prometheus server address format: %s, expected host:port", flagPrometheusServerAddr)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		if _, err := strconv.Atoi(port); err != nil {
			slog.Error(err.Error())
			err = fmt.Errorf("invalid port in prometheus server address: %s, port must be an integer", flagPrometheusServerAddr)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
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
		} else if strings.HasSuffix(file.Name(), "_events.jsonl") {
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
	eventFile, err = os.Open(eventPath) // #nosec G304
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
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	go func() {
		for sig := range sigChannel {
			slog.Debug("received signal", slog.String("signal", sig.String()))
			if sig == syscall.SIGINT || sig == syscall.SIGTERM {
				setSignalReceived()
			}
			// send kill signal to children
			err := util.SignalChildren(syscall.SIGKILL)
			if err != nil {
				slog.Error("failed to send kill signal to children", slog.String("error", err.Error()))
			}
		}
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
				deferTarget := myTarget // create a new variable to capture the current value
				defer func(deferTarget target.Target) {
					err := myTarget.RemoveTempDirectory()
					if err != nil {
						slog.Error("error removing target temporary directory", slog.String("error", err.Error()))
					}
				}(deferTarget)
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
	for range targetContexts {
		targetError := <-channelTargetError
		if targetError.err != nil {
			slog.Error("failed to prepare metrics", slog.String("target", targetError.target.GetName()), slog.String("error", targetError.err.Error()))
			_ = multiSpinner.Status(targetError.target.GetName(), fmt.Sprintf("Error: %v", targetError.err))
		} else {
			numTargetsWithPreparedMetrics++
		}
	}
	if numTargetsWithPreparedMetrics == 0 {
		err := fmt.Errorf("no targets had metrics successfully prepared")
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
	if !flagLive && !flagPrometheusServer {
		err = common.CreateOutputDir(localOutputDir)
		if err != nil {
			err = fmt.Errorf("failed to create output directory: %w", err)
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
			cmd.SilenceUsage = true
			return err
		}
	}
	// start the metric production for each target
	collectOnTargetWG := sync.WaitGroup{}
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
		collectOnTargetWG.Add(1)
		go collectOnTarget(&targetContexts[i], localTempDir, localOutputDir, &collectOnTargetWG, multiSpinner.Status)
	}
	if flagLive {
		multiSpinner.Finish()
	}
	// Start Prometheus server if requested
	if flagPrometheusServer && flagPrometheusServerAddr != "" {
		multiSpinner.Finish()
		fmt.Printf("starting metrics server on %s\n", flagPrometheusServerAddr)
		startPrometheusServer(flagPrometheusServerAddr)
		cmd.SilenceUsage = true
	}
	// wait for all collectOnTarget goroutines to finish
	collectOnTargetWG.Wait()
	// finalize the spinner status, capture any errors, and create output files
	var exitErrs []error
	allPrintedFileNames := make([][]string, 0)
	for i, targetContext := range targetContexts {
		if targetContext.err == nil {
			if !flagLive {
				_ = multiSpinner.Status(targetContext.target.GetName(), "collection complete")
				csvMetricsFile := filepath.Join(localOutputDir, targetContext.target.GetName()+"_metrics.csv")
				exists, _ := util.FileExists(csvMetricsFile)
				if !exists {
					_ = multiSpinner.Status(targetContext.target.GetName(), "no metrics collected")
				} else {
					targetContext.metadata.PerfSpectVersion = appContext.Version
					summaryFiles, err := summarizeMetrics(localOutputDir, targetContext.target.GetName(), targetContext.metadata)
					if err != nil {
						err = fmt.Errorf("failed to summarize metrics: %w", err)
						exitErrs = append(exitErrs, err)
					}
					targetContexts[i].printedFiles = append(targetContexts[i].printedFiles, summaryFiles...)
				}
			}
		} else {
			err := fmt.Errorf("failed to collect on target %s: %w", targetContext.target.GetName(), targetContext.err)
			exitErrs = append(exitErrs, err)
		}
		allPrintedFileNames = append(allPrintedFileNames, targetContexts[i].printedFiles)
		// write metadata to file
		if flagWriteEventsToFile {
			if err = targetContext.metadata.WriteJSONToFile(localOutputDir + "/" + targetContext.target.GetName() + "_" + "metadata.json"); err != nil {
				err = fmt.Errorf("failed to write metadata to file: %w", err)
				exitErrs = append(exitErrs, err)
			}
		}
	}
	if !flagLive && !flagPrometheusServer {
		multiSpinner.Finish()
		printOutputFileNames(allPrintedFileNames)
	}
	// join the errors and print them
	err = errors.Join(exitErrs...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
	}
	return err
}

func prepareTarget(targetContext *targetContext, localTempDir string, localPerfPath string, channelError chan targetError, statusUpdate progress.MultiSpinnerUpdateFunc, useDefaultMuxInterval bool) {
	myTarget := targetContext.target
	var err error
	_ = statusUpdate(myTarget.GetName(), "configuring target")
	// are PMUs being used on target?
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
			// if one of the PMU MSR registers is active, then the PMU is in use (ignore cpu_cycles)
			if strings.Contains(line, "Active") && !strings.Contains(line, "0x30a") {
				slog.Warn("PMU is in use on target", slog.String("target", myTarget.GetName()), slog.String("line", line))
				_ = statusUpdate(myTarget.GetName(), "Warning: PMU in use, see log for details")
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
	// set perf mux interval to desired value
	if !flagNoRoot {
		if targetContext.perfMuxIntervals, err = GetMuxIntervals(myTarget, localTempDir); err != nil {
			err = fmt.Errorf("failed to get perf mux intervals: %w", err)
			_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
			targetContext.err = err
			channelError <- targetError{target: myTarget, err: err}
			return
		}
		perfMuxInterval := flagPerfMuxInterval
		if useDefaultMuxInterval {
			// set the default mux interval to 16ms for AMD architecture
			vendor, err := myTarget.GetVendor()
			if err == nil && vendor == "AuthenticAMD" {
				perfMuxInterval = 16
			}
		}
		if err = SetAllMuxIntervals(myTarget, perfMuxInterval, localTempDir); err != nil {
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
	slog.Debug("Using Linux perf", slog.String("target", targetContext.target.GetName()), slog.String("path", targetContext.perfPath))
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
	skipSystemSummary := flagNoSystemSummary
	if flagLive {
		skipSystemSummary = true // no system summary when live, it doesn't get used/printed
	}
	if targetContext.metadata, err = LoadMetadata(myTarget, flagNoRoot, skipSystemSummary, targetContext.perfPath, localTempDir); err != nil {
		_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
		targetContext.err = err
		channelError <- targetError{target: myTarget, err: err}
		return
	}
	slog.Debug("metadata: " + targetContext.metadata.String())
	if !targetContext.metadata.SupportsInstructions {
		slog.Error("Target does not support instructions event collection", slog.String("target", myTarget.GetName()))
		targetContext.err = fmt.Errorf("target not supported, does not support instructions event collection")
		channelError <- targetError{target: myTarget, err: targetContext.err}
		return
	}
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
	if flagPrometheusServer {
		for _, def := range targetContext.metricDefinitions {
			desc := fmt.Sprintf("%s (expr: %s)", def.Name, def.Expression)
			name := promMetricPrefix + sanitizeMetricName(def.Name)
			gauge := prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: name,
					Help: desc,
				},
				[]string{"socket", "cpu", "cgroup", "pid", "cmd"},
			)
			promMetrics[name] = gauge
		}
		for _, m := range promMetrics {
			prometheus.MustRegister(m)
		}
	}
	channelError <- targetError{target: myTarget, err: nil}
}

func getProcessesForPerf(myTarget target.Target, pidList []string, count int, filter string) ([]Process, error) {
	var processes []Process
	if len(pidList) > 0 {
		var err error
		processes, err = GetProcesses(myTarget, pidList)
		if err != nil {
			return nil, fmt.Errorf("failed to get processes: %w", err)
		}
	} else {
		var err error
		processes, err = GetHotProcesses(myTarget, count, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to get hot processes: %w", err)
		}
		if len(processes) == 0 {
			return nil, fmt.Errorf("no processes found")
		}
	}
	return processes, nil
}

func getCidsForPerf(myTarget target.Target, cidList []string, count int, filter string, localTempDir string) ([]string, error) {
	var cids []string
	if len(cidList) > 0 {
		var err error
		cids, err = GetCgroups(myTarget, cidList, localTempDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get cgroups: %w", err)
		}
	} else {
		var err error
		cids, err = GetHotCgroups(myTarget, count, filter, localTempDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get hot cgroups: %w", err)
		}
		if len(cids) == 0 {
			return nil, fmt.Errorf("no cgroups found")
		}
	}
	return cids, nil
}

func collectOnTarget(targetContext *targetContext, localTempDir string, localOutputDir string, wg *sync.WaitGroup, statusUpdate progress.MultiSpinnerUpdateFunc) {
	defer wg.Done()
	myTarget := targetContext.target
	if targetContext.err != nil {
		return
	}
	// only refresh if duration is 0, i.e., no timeout and pids/cids are not specified
	var needsRefresh bool
	if flagDuration == 0 {
		switch flagScope {
		case scopeProcess:
			if len(flagPidList) == 0 {
				needsRefresh = true
			}
		case scopeCgroup:
			if len(flagCidList) == 0 {
				needsRefresh = true
			}
		}
	}
	errorChannel := make(chan error)
	frameChannel := make(chan []MetricFrame)
	printCompleteChannel := make(chan []string)
	// get current time for use in setting timestamps on output
	targetContext.metadata.CollectionStartTime = time.Now() // save the start time in the metadata for use when using the --input option to process raw data
	go printMetricsAsync(targetContext, localOutputDir, frameChannel, printCompleteChannel)
	var err error
	for !getSignalReceived() {
		var processes []Process
		var pids []string
		var cids []string
		if flagScope == scopeProcess {
			// get the list of pids to collect
			processes, err = getProcessesForPerf(myTarget, flagPidList, flagCount, flagFilter)
			if err != nil {
				err = fmt.Errorf("failed to get processes: %w", err)
				slog.Error("failed to get processes", slog.String("error", err.Error()))
				_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
				break
			}
			// get pids from processes
			for _, process := range processes {
				pids = append(pids, process.pid)
			}
		} else if flagScope == scopeCgroup {
			// get the list of cids to collect
			cids, err = getCidsForPerf(myTarget, flagCidList, flagCount, flagFilter, localTempDir)
			if err != nil {
				if targetContext.perfStartTime.Equal((time.Time{})) {
					targetContext.perfStartTime = time.Now()
				}
				exceededDuration := flagDuration != 0 && time.Since(targetContext.perfStartTime) > time.Duration(flagDuration)*time.Second
				if !exceededDuration && len(flagCidList) == 0 && strings.Contains(err.Error(), "no cgroups found") {
					err = nil // ignore this error, we'll try again
					slog.Debug("no cgroups found, will try again in 5 seconds")
					time.Sleep(5 * time.Second) // wait for 5 seconds before trying again
					continue
				}
				err = fmt.Errorf("failed to get cgroups: %w", err)
				_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
				break
			}
		}
		var perfCommand *exec.Cmd
		perfCommand, err = getPerfCommand(targetContext.perfPath, targetContext.groupDefinitions, pids, cids)
		if err != nil {
			err = fmt.Errorf("failed to get perf command: %w", err)
			_ = statusUpdate(myTarget.GetName(), fmt.Sprintf("Error: %s", err.Error()))
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
			slog.Debug("perf error", slog.String("error", perfErr.Error()))
			break
		}
		// perf exited with no errors
		if !needsRefresh {
			slog.Debug("we're done, no refresh needed")
			break
		}
	}
	close(frameChannel) // we're done writing frames so shut it down
	// wait for printing to complete
	targetContext.printedFiles = <-printCompleteChannel
	close(printCompleteChannel)
	// keep track of the error
	targetContext.err = err
}

// runPerf starts Linux perf using the provided command, then reads perf's output
// until perf stops. When collecting for cgroups, perf will be manually terminated if/when the
// run duration exceeds the collection time or the time when the cgroup list needs
// to be refreshed.
func runPerf(myTarget target.Target, noRoot bool, processes []Process, cmd *exec.Cmd, eventGroupDefinitions []GroupDefinition, metricDefinitions []MetricDefinition, metadata Metadata, localTempDir string, outputDir string, frameChannel chan []MetricFrame, errorChannel chan error) {
	// start perf
	perfCommand := strings.Join(cmd.Args, " ")
	stdoutChannel := make(chan string)
	stderrChannel := make(chan string)
	exitcodeChannel := make(chan int)
	scriptErrorChannel := make(chan error)
	cmdChannel := make(chan *exec.Cmd)
	slog.Debug("running perf stat", slog.String("command", perfCommand))
	perfStatScript := script.ScriptDefinition{
		Name:           "perf stat",
		ScriptTemplate: perfCommand,
		Superuser:      !noRoot,
	}
	// start goroutine to run perf, output will be streamed back in provided channels
	go script.RunScriptStream(myTarget, perfStatScript, localTempDir, stdoutChannel, stderrChannel, exitcodeChannel, scriptErrorChannel, cmdChannel)
	select {
	case <-cmdChannel:
	case err := <-scriptErrorChannel:
		if err != nil {
			errorChannel <- err // error running the script
			return
		}
	}
	// must manually terminate perf in cgroup scope when a timeout is specified and/or need to refresh cgroups
	startPerfTimestamp := time.Now()
	cgroupTimeout := 0 // default to 0, which means no timeout
	if flagScope == scopeCgroup {
		// if cids are specified, we don't need to refresh, but we do need to set a timeout
		if len(flagCidList) > 0 {
			cgroupTimeout = flagDuration
		} else { // no cids are specified
			// if duration is specified, use that as the timeout
			if flagDuration != 0 {
				cgroupTimeout = flagDuration
			} else {
				cgroupTimeout = flagRefresh
			}
		}
	}
	// Start a goroutine to wait for and then process perf output
	// Use a timer to determine when we received an entire frame of events from perf
	// The timer will expire when no lines (events) have been received from perf for more than 100ms. This
	// works because perf writes the events to stderr in a burst every collection interval, e.g., 5 seconds.
	// When the timer expires, this code assumes that perf is done writing events to stderr.
	const perfEventWaitTime = time.Duration(100 * time.Millisecond)                   // 100ms is somewhat arbitrary, but is long enough for perf to print a frame of events
	perfOutputTimer := time.NewTimer(time.Duration(2 * flagPerfPrintInterval * 1000)) // #nosec G115
	perfProcessingContext, cancelPerfProcessing := context.WithCancel(context.Background())
	outputLines := make([][]byte, 0)
	donePerfProcessingChannel := make(chan struct{}) // channel to wait for processPerfOutput to finish
	go processPerfOutput(
		perfProcessingContext,
		myTarget,
		metadata,
		eventGroupDefinitions,
		metricDefinitions,
		outputDir,
		processes,
		cgroupTimeout,
		startPerfTimestamp,
		perfOutputTimer,
		&outputLines,
		frameChannel,
		donePerfProcessingChannel,
	)
	// receive perf output
	done := false
	for !done {
		select {
		case line := <-stderrChannel: // perf output comes in on this channel, one line at a time
			perfOutputTimer.Stop()
			perfOutputTimer.Reset(perfEventWaitTime)
			// accumulate the lines, they will be processed in the goroutine when the timer expires
			outputLines = append(outputLines, []byte(line))
		case exitCode := <-exitcodeChannel: // when perf exits, the exit code comes to this channel
			slog.Debug("perf exited", slog.Int("exit code", exitCode))
			time.Sleep(perfEventWaitTime) // wait for timer to expire so that last events can be processed
			done = true                   // exit the loop
		case err := <-scriptErrorChannel: // if there is an error running perf, it comes here
			if err != nil {
				slog.Error("error from perf", slog.String("error", err.Error()))
			}
			done = true // exit the loop
		}
	}
	perfOutputTimer.Stop()
	// cancel the context to stop processPerfOutput
	cancelPerfProcessing()
	// wait for processPerfOutput to finish
	<-donePerfProcessingChannel
	errorChannel <- nil
}

// processPerfOutput processes perf output in a goroutine and supports cancellation via context.
// This function must not return until the context is cancelled.
// When context is cancelled, this function will close the done channel to signal that processing is complete.
// there are two scenarios where this function will trigger a context cancellation by signalling the localCommand to terminate:
//  1. when the number of consecutive errors processing events exceeds the maximum (2)
//  2. when the cgroup refresh timeout is reached (in scope==cgroup mode)
func processPerfOutput(
	ctx context.Context,
	myTarget target.Target,
	metadata Metadata,
	eventGroupDefinitions []GroupDefinition,
	metricDefinitions []MetricDefinition,
	outputDir string,
	processes []Process,
	cgroupTimeout int,
	startPerfTimestamp time.Time,
	perfOutputTimer *time.Timer,
	outputLines *[][]byte,
	frameChannel chan []MetricFrame,
	doneChannel chan struct{},
) {
	defer close(doneChannel) // close the done channel when the function returns to signal completion
	var frameTimestamp float64
	contextCancelled := false
	var numConsecutiveProcessEventErrors int
	const maxConsecutiveProcessEventErrors = 2
	for !contextCancelled {
		select {
		case <-perfOutputTimer.C: // waits for timer to expire the process the events in outputLines
		case <-ctx.Done(): // context cancellation
			contextCancelled = true // exit the loop after one more pass
		}
		if contextCancelled {
			break
		}
		if len(*outputLines) != 0 {
			// write the events to a file
			if flagWriteEventsToFile {
				if err := writeEventsToFile(outputDir+"/"+myTarget.GetName()+"_"+"events.jsonl", *outputLines); err != nil {
					slog.Error("failed to write events to file", slog.String("error", err.Error()))
				}
			}
			// process the events
			var metricFrames []MetricFrame
			var err error
			metricFrames, frameTimestamp, err = ProcessEvents(*outputLines, eventGroupDefinitions, metricDefinitions, processes, frameTimestamp, metadata)
			if err != nil {
				slog.Error(err.Error())
				numConsecutiveProcessEventErrors++
				if numConsecutiveProcessEventErrors > maxConsecutiveProcessEventErrors {
					slog.Error("too many consecutive errors processing events, killing perf", slog.Int("max errors", maxConsecutiveProcessEventErrors))
					// signaling self with SIGUSR1 will signal child processes to exit, which will cancel the context and let this function exit
					err := util.SignalSelf(syscall.SIGUSR1)
					if err != nil {
						slog.Error("failed to signal self", slog.String("error", err.Error()))
					}
				}
				*outputLines = [][]byte{} // empty it
			} else {
				// send the metrics frames out to be printed
				frameChannel <- metricFrames
				// empty the outputLines
				*outputLines = [][]byte{}
				// reset the error count
				numConsecutiveProcessEventErrors = 0
			}
		}
		// for cgroup scope, terminate perf if refresh timeout is reached
		if flagScope == scopeCgroup && cgroupTimeout != 0 {
			if int(time.Since(startPerfTimestamp).Seconds()) >= cgroupTimeout {
				slog.Debug("cgroup refresh timeout reached")
				// signaling self with SIGUSR1 will signal child processes to exit, which will cancel the context and let this function exit
				err := util.SignalSelf(syscall.SIGUSR1)
				if err != nil {
					slog.Error("failed to signal self", slog.String("error", err.Error()))
				}
			}
		}
	}
}
