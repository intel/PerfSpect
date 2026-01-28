// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"perfspect/internal/app"
	"perfspect/internal/workflow"

	"perfspect/internal/util"

	"github.com/spf13/cobra"
)

const trimCmdName = "trim"

// trim command flags
var (
	flagTrimInput       string
	flagTrimStartTime   int
	flagTrimEndTime     int
	flagTrimStartOffset int
	flagTrimEndOffset   int
)

const (
	flagTrimInputName       = "input"
	flagTrimStartTimeName   = "start-time"
	flagTrimEndTimeName     = "end-time"
	flagTrimStartOffsetName = "start-offset"
	flagTrimEndOffsetName   = "end-offset"
)

var trimExamples = []string{
	"  Skip first 30 seconds:                        $ perfspect metrics trim --input perfspect_2025-11-28_09-21-56 --start-offset 30",
	"  Skip first 10 seconds and last 5 seconds:     $ perfspect metrics trim --input perfspect_2025-11-28_09-21-56 --start-offset 10 --end-offset 5",
	"  Use absolute timestamps and specific CSV:     $ perfspect metrics trim --input perfspect_2025-11-28_09-21-56/myhost_metrics.csv --start-time 1764174327 --end-time 1764174351",
}

var trimCmd = &cobra.Command{
	Use:           trimCmdName,
	Short:         "Filter existing metrics to a time range",
	Long:          "",
	Example:       strings.Join(trimExamples, "\n"),
	RunE:          runTrimCmd,
	PreRunE:       validateTrimFlags,
	SilenceErrors: true,
}

func init() {
	Cmd.AddCommand(trimCmd)

	trimCmd.Flags().StringVar(&flagTrimInput, flagTrimInputName, "", "path to the directory or specific metrics CSV file to trim (required)")
	trimCmd.Flags().IntVar(&flagTrimStartTime, flagTrimStartTimeName, 0, "absolute start timestamp (seconds since epoch)")
	trimCmd.Flags().IntVar(&flagTrimEndTime, flagTrimEndTimeName, 0, "absolute end timestamp (seconds since epoch)")
	trimCmd.Flags().IntVar(&flagTrimStartOffset, flagTrimStartOffsetName, 0, "seconds to skip from the beginning of the data")
	trimCmd.Flags().IntVar(&flagTrimEndOffset, flagTrimEndOffsetName, 0, "seconds to exclude from the end of the data")

	_ = trimCmd.MarkFlagRequired(flagTrimInputName) // error only occurs if flag doesn't exist

	// Set custom usage function to avoid parent's usage function issues
	trimCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStdout(), "Usage:\n  %s\n\n", cmd.UseLine())
		if cmd.HasExample() {
			fmt.Fprintf(cmd.OutOrStdout(), "Examples:\n%s\n\n", cmd.Example)
		}
		if cmd.HasAvailableLocalFlags() {
			fmt.Fprintf(cmd.OutOrStdout(), "Flags:\n%s\n", cmd.LocalFlags().FlagUsages())
		}
		if cmd.HasAvailableInheritedFlags() {
			fmt.Fprintf(cmd.OutOrStdout(), "Global Flags:\n%s\n", cmd.InheritedFlags().FlagUsages())
		}
		return nil
	})
}

// validateTrimFlags checks that the trim command flags are valid and consistent
func validateTrimFlags(cmd *cobra.Command, args []string) error {
	// Check input file or directory exists
	if _, err := os.Stat(flagTrimInput); err != nil {
		if os.IsNotExist(err) {
			return workflow.FlagValidationError(cmd, fmt.Sprintf("input file or directory does not exist: %s", flagTrimInput))
		}
		return workflow.FlagValidationError(cmd, fmt.Sprintf("failed to access input file or directory: %v", err))
	}

	// Check that at least one time parameter is provided
	if flagTrimStartTime == 0 && flagTrimEndTime == 0 && flagTrimStartOffset == 0 && flagTrimEndOffset == 0 {
		return workflow.FlagValidationError(cmd, "at least one time parameter must be specified (--start-time, --end-time, --start-offset, or --end-offset)")
	}

	// Check that both absolute time and offset are not specified for start
	if flagTrimStartTime != 0 && flagTrimStartOffset != 0 {
		return workflow.FlagValidationError(cmd, "cannot specify both --start-time and --start-offset")
	}

	// Check that both absolute time and offset are not specified for end
	if flagTrimEndTime != 0 && flagTrimEndOffset != 0 {
		return workflow.FlagValidationError(cmd, "cannot specify both --end-time and --end-offset")
	}

	// Check for negative values
	if flagTrimStartTime < 0 {
		return workflow.FlagValidationError(cmd, "--start-time cannot be negative")
	}
	if flagTrimEndTime < 0 {
		return workflow.FlagValidationError(cmd, "--end-time cannot be negative")
	}
	if flagTrimStartOffset < 0 {
		return workflow.FlagValidationError(cmd, "--start-offset cannot be negative")
	}
	if flagTrimEndOffset < 0 {
		return workflow.FlagValidationError(cmd, "--end-offset cannot be negative")
	}

	// Check that absolute times are in order if both specified
	if flagTrimStartTime != 0 && flagTrimEndTime != 0 && flagTrimStartTime >= flagTrimEndTime {
		return workflow.FlagValidationError(cmd, "--start-time must be less than --end-time")
	}

	return nil
}

// runTrimCmd executes the trim command
func runTrimCmd(cmd *cobra.Command, args []string) error {
	// appContext is the application context that holds common data and resources.
	appContext := cmd.Parent().Context().Value(app.Context{}).(app.Context)
	outputDir := appContext.OutputDir

	// flagTrimInput can be a file or directory
	var sourceDir string
	fileInfo, err := os.Stat(flagTrimInput)
	if err != nil {
		err = fmt.Errorf("failed to access input path: %w", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	if fileInfo.IsDir() {
		sourceDir = flagTrimInput
	} else {
		sourceDir = filepath.Dir(flagTrimInput)
	}

	// Determine source files to process
	sourceInfos, err := getTrimmedSourceInfos(flagTrimInput)
	if err != nil {
		err = fmt.Errorf("failed to determine source files: %w", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}
	if len(sourceInfos) == 0 {
		err = fmt.Errorf("no valid metrics CSV files found to trim")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}

	// create output directory if it doesn't exist
	err = util.CreateDirectoryIfNotExists(outputDir, 0755) // #nosec G301
	if err != nil {
		err = fmt.Errorf("failed to create output directory: %w", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		slog.Error(err.Error())
		cmd.SilenceUsage = true
		return err
	}

	// Process each source file
	var filesCreated []string
	for _, sourceInfo := range sourceInfos {
		filesCreated, err = summarizeMetricsWithTrim(sourceDir, outputDir, sourceInfo.targetName, sourceInfo.metadata, sourceInfo.metricDefinitions, sourceInfo.startTime, sourceInfo.endTime)
		if err != nil {
			err = fmt.Errorf("failed to generate trimmed summaries for %s: %w", sourceInfo.allCSVPath, err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			slog.Error(err.Error())
			cmd.SilenceUsage = true
			return err
		}
	}

	// Report success
	fmt.Println("\nTrimmed metrics successfully created:")
	for _, filePath := range filesCreated {
		fmt.Printf("  %s\n", filePath)
	}

	return nil
}

type trimSourceInfo struct {
	allCSVPath        string
	summaryCSVPath    string
	summaryHTMLPath   string
	targetName        string
	metadata          Metadata
	metricDefinitions []MetricDefinition
	startTime         int
	endTime           int
}

func getTrimmedSourceInfos(sourceDirOrFilename string) ([]trimSourceInfo, error) {
	var sourceInfos []trimSourceInfo

	// If a specific file is provided, use that
	if sourceDirOrFilename != "" && strings.HasSuffix(strings.ToLower(sourceDirOrFilename), ".csv") {
		baseName := strings.TrimSuffix(filepath.Base(sourceDirOrFilename), filepath.Ext(sourceDirOrFilename))
		summaryCSV := filepath.Join(filepath.Dir(sourceDirOrFilename), baseName+"_summary.csv")
		summaryHTML := filepath.Join(filepath.Dir(sourceDirOrFilename), baseName+"_summary.html")
		sourceInfos = append(sourceInfos, trimSourceInfo{
			allCSVPath:      sourceDirOrFilename,
			summaryCSVPath:  summaryCSV,
			summaryHTMLPath: summaryHTML,
		})
	} else {

		// Otherwise, scan the directory for all *_metrics.csv files
		files, err := os.ReadDir(sourceDirOrFilename)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory: %w", err)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			if strings.HasSuffix(strings.ToLower(file.Name()), "_metrics.csv") {
				baseName := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
				allCSVPath := filepath.Join(sourceDirOrFilename, file.Name())
				summaryCSV := filepath.Join(sourceDirOrFilename, baseName+"_summary.csv")
				summaryHTML := filepath.Join(sourceDirOrFilename, baseName+"_summary.html")
				sourceInfos = append(sourceInfos, trimSourceInfo{
					allCSVPath:      allCSVPath,
					summaryCSVPath:  summaryCSV,
					summaryHTMLPath: summaryHTML,
				})
			}
		}
	}

	for i, sourceInfo := range sourceInfos {
		// Determine target name from filename
		inputBase := filepath.Base(sourceInfo.allCSVPath)
		inputName := strings.TrimSuffix(inputBase, filepath.Ext(inputBase))
		targetName := strings.TrimSuffix(inputName, "_metrics")
		sourceInfos[i].targetName = targetName
		// Load all metrics to determine time range
		metrics, err := newMetricCollection(sourceInfo.allCSVPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load metrics from CSV: %w", err)
		}
		if len(metrics) == 0 {
			return nil, fmt.Errorf("no metrics found in CSV file")
		}
		// Calculate the time range
		startTime, endTime, err := calculateTimeRange(metrics, flagTrimStartTime, flagTrimEndTime, flagTrimStartOffset, flagTrimEndOffset)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate time range: %w", err)
		}
		sourceInfos[i].startTime = startTime
		sourceInfos[i].endTime = endTime
		// Retrieve the metadata from the HTML summary
		metadata, err := loadMetadataFromHTMLSummary(sourceInfo.summaryHTMLPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load metadata from HTML summary: %w", err)
		}
		sourceInfos[i].metadata = metadata
		// Load metric definitions using the metadata
		metricDefinitions, err := loadMetricDefinitions(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to get metric definitions: %w", err)
		}
		sourceInfos[i].metricDefinitions = metricDefinitions
	}

	return sourceInfos, nil
}

func loadMetricDefinitions(metadata Metadata) ([]MetricDefinition, error) {
	loader, err := NewLoader(metadata.Microarchitecture, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric definition loader: %w", err)
	}
	metricDefinitions, _, err := loader.Load(getLoaderConfig(loader, []string{}, metadata, "", ""))
	if err != nil {
		return nil, fmt.Errorf("failed to load metric definitions: %w", err)
	}
	return metricDefinitions, nil
}

func loadMetadataFromHTMLSummary(summaryHTMLPath string) (Metadata, error) {
	var metadata Metadata
	// Check if the summary HTML file exists
	_, err := os.Stat(summaryHTMLPath)
	if err != nil {
		return metadata, fmt.Errorf("summary HTML file does not exist: %s", summaryHTMLPath)
	}

	// find "const metadata = " and "const system_info = " in HTML file.
	// The JSON string follows the equals sign.
	// e.g., const metadata = {"NumGeneralPurposeCounters":8,"SocketCount":2, ... }
	content, err := os.ReadFile(summaryHTMLPath)
	if err != nil {
		return metadata, fmt.Errorf("failed to read summary HTML file: %w", err)
	}

	// assumes system_summary comes after metadata in the file
	const metadataPrefix = "const metadata = "
	const systemSummaryPrefix = "const system_summary = "
	for line := range strings.SplitSeq(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, metadataPrefix) {
			jsonStart := len(metadataPrefix)
			// to end of line
			jsonString := strings.TrimSpace(line[jsonStart:])
			// parse JSON string into Metadata struct
			err = json.Unmarshal([]byte(jsonString), &metadata)
			if err != nil {
				return metadata, fmt.Errorf("failed to parse metadata JSON: %w", err)
			}
		} else if strings.HasPrefix(line, systemSummaryPrefix) {
			// system summary
			var systemSummary [][]string
			jsonStart := len(systemSummaryPrefix)
			jsonString := strings.TrimSpace(line[jsonStart:])
			err = json.Unmarshal([]byte(jsonString), &systemSummary)
			if err != nil {
				return metadata, fmt.Errorf("failed to parse system summary JSON: %w", err)
			}
			metadata.SystemSummaryFields = systemSummary
			return metadata, nil
		}
	}

	return metadata, fmt.Errorf("metadata not found in summary HTML file: %s", summaryHTMLPath)
}

// calculateTimeRange determines the actual start and end times based on the flags and data
// Returns startTime, endTime, error
func calculateTimeRange(metrics MetricCollection, startTime, endTime, startOffset, endOffset int) (int, int, error) {
	if len(metrics) == 0 || len(metrics[0].rows) == 0 {
		return 0, 0, fmt.Errorf("no data available to calculate time range")
	}

	// Find min and max timestamps in the data
	minTimestamp := metrics[0].rows[0].timestamp
	maxTimestamp := metrics[0].rows[0].timestamp

	for _, mg := range metrics {
		for _, row := range mg.rows {
			if row.timestamp < minTimestamp {
				minTimestamp = row.timestamp
			}
			if row.timestamp > maxTimestamp {
				maxTimestamp = row.timestamp
			}
		}
	}

	// Calculate start time
	calcStartTime := minTimestamp
	if startTime != 0 {
		calcStartTime = startTime
	} else if startOffset != 0 {
		calcStartTime = minTimestamp + startOffset
	}

	// Calculate end time
	calcEndTime := maxTimestamp
	if endTime != 0 {
		calcEndTime = endTime
	} else if endOffset != 0 {
		calcEndTime = maxTimestamp - endOffset
	}

	// Validate the calculated range
	if calcStartTime >= calcEndTime {
		return 0, 0, fmt.Errorf("invalid time range: start (%d) >= end (%d)", calcStartTime, calcEndTime)
	}

	if calcStartTime > maxTimestamp {
		return 0, 0, fmt.Errorf("start time (%d) is beyond the end of available data (%d)", calcStartTime, maxTimestamp)
	}

	if calcEndTime < minTimestamp {
		return 0, 0, fmt.Errorf("end time (%d) is before the beginning of available data (%d)", calcEndTime, minTimestamp)
	}

	return calcStartTime, calcEndTime, nil
}
