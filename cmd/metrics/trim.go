package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"perfspect/internal/common"

	"github.com/spf13/cobra"
)

const trimCmdName = "trim"

// trim command flags
var (
	flagTrimInput       string
	flagTrimStartTime   int64
	flagTrimEndTime     int64
	flagTrimStartOffset int64
	flagTrimEndOffset   int64
	flagTrimOutputDir   string
	flagTrimSuffix      string
)

const (
	flagTrimInputName       = "input"
	flagTrimStartTimeName   = "start-time"
	flagTrimEndTimeName     = "end-time"
	flagTrimStartOffsetName = "start-offset"
	flagTrimEndOffsetName   = "end-offset"
	flagTrimOutputDirName   = "output-dir"
	flagTrimSuffixName      = "suffix"
)

var trimExamples = []string{
	"  Skip first 10 seconds and last 5 seconds:     $ perfspect metrics trim --input host_metrics.csv --start-offset 10 --end-offset 5",
	"  Use absolute timestamps:                      $ perfspect metrics trim --input host_metrics.csv --start-time 1764174327 --end-time 1764174351",
	"  Custom output suffix:                         $ perfspect metrics trim --input host_metrics.csv --start-offset 10 --suffix steady_state",
	"  Specify output directory:                     $ perfspect metrics trim --input host_metrics.csv --start-offset 5 --output-dir ./trimmed",
}

var trimCmd = &cobra.Command{
	Use:   trimCmdName,
	Short: "Refine metrics data to a specific time range",
	Long: `Generate new summary reports from existing metrics CSV data by filtering to a specific time range.

This is useful when you've collected metrics for an entire workload but want to analyze
only a specific portion, excluding setup, teardown, or other phases. The command reads an
existing metrics CSV file, filters rows to the specified time range, and generates new
summary reports (CSV and HTML).

Time range can be specified using either:
  - Absolute timestamps (--start-time and --end-time)
  - Relative offsets from beginning/end (--start-offset and --end-offset)

If a metadata JSON file exists alongside the input CSV, it will be used to generate
a complete HTML report with system summary. Otherwise, a simplified HTML report
without system summary will be generated.`,
	Example:       strings.Join(trimExamples, "\n"),
	RunE:          runTrimCmd,
	PreRunE:       validateTrimFlags,
	SilenceErrors: true,
}

func init() {
	Cmd.AddCommand(trimCmd)

	trimCmd.Flags().StringVar(&flagTrimInput, flagTrimInputName, "", "path to the metrics CSV file to trim (required)")
	trimCmd.Flags().Int64Var(&flagTrimStartTime, flagTrimStartTimeName, 0, "absolute start timestamp (seconds since epoch)")
	trimCmd.Flags().Int64Var(&flagTrimEndTime, flagTrimEndTimeName, 0, "absolute end timestamp (seconds since epoch)")
	trimCmd.Flags().Int64Var(&flagTrimStartOffset, flagTrimStartOffsetName, 0, "seconds to skip from the beginning of the data")
	trimCmd.Flags().Int64Var(&flagTrimEndOffset, flagTrimEndOffsetName, 0, "seconds to exclude from the end of the data")
	trimCmd.Flags().StringVar(&flagTrimOutputDir, flagTrimOutputDirName, "", "output directory (default: same directory as input file)")
	trimCmd.Flags().StringVar(&flagTrimSuffix, flagTrimSuffixName, "trimmed", "suffix for output filenames")

	_ = trimCmd.MarkFlagRequired(flagTrimInputName) // error only occurs if flag doesn't exist

	// Set custom usage function to avoid parent's usage function issues
	trimCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n", cmd.Long)
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
	// Check input file exists
	if _, err := os.Stat(flagTrimInput); err != nil {
		if os.IsNotExist(err) {
			return common.FlagValidationError(cmd, fmt.Sprintf("input file does not exist: %s", flagTrimInput))
		}
		return common.FlagValidationError(cmd, fmt.Sprintf("failed to access input file: %v", err))
	}

	// Check that input is a CSV file
	if !strings.HasSuffix(strings.ToLower(flagTrimInput), ".csv") {
		return common.FlagValidationError(cmd, fmt.Sprintf("input file must be a CSV file: %s", flagTrimInput))
	}

	// Check that at least one time parameter is provided
	if flagTrimStartTime == 0 && flagTrimEndTime == 0 && flagTrimStartOffset == 0 && flagTrimEndOffset == 0 {
		return common.FlagValidationError(cmd, "at least one time parameter must be specified (--start-time, --end-time, --start-offset, or --end-offset)")
	}

	// Check that both absolute time and offset are not specified for start
	if flagTrimStartTime != 0 && flagTrimStartOffset != 0 {
		return common.FlagValidationError(cmd, "cannot specify both --start-time and --start-offset")
	}

	// Check that both absolute time and offset are not specified for end
	if flagTrimEndTime != 0 && flagTrimEndOffset != 0 {
		return common.FlagValidationError(cmd, "cannot specify both --end-time and --end-offset")
	}

	// Check for negative values
	if flagTrimStartTime < 0 {
		return common.FlagValidationError(cmd, "--start-time cannot be negative")
	}
	if flagTrimEndTime < 0 {
		return common.FlagValidationError(cmd, "--end-time cannot be negative")
	}
	if flagTrimStartOffset < 0 {
		return common.FlagValidationError(cmd, "--start-offset cannot be negative")
	}
	if flagTrimEndOffset < 0 {
		return common.FlagValidationError(cmd, "--end-offset cannot be negative")
	}

	// Check that absolute times are in order if both specified
	if flagTrimStartTime != 0 && flagTrimEndTime != 0 && flagTrimStartTime >= flagTrimEndTime {
		return common.FlagValidationError(cmd, "--start-time must be less than --end-time")
	}

	// Validate output directory if specified
	if flagTrimOutputDir != "" {
		if info, err := os.Stat(flagTrimOutputDir); err != nil {
			if os.IsNotExist(err) {
				return common.FlagValidationError(cmd, fmt.Sprintf("output directory does not exist: %s", flagTrimOutputDir))
			}
			return common.FlagValidationError(cmd, fmt.Sprintf("failed to access output directory: %v", err))
		} else if !info.IsDir() {
			return common.FlagValidationError(cmd, fmt.Sprintf("output-dir must be a directory: %s", flagTrimOutputDir))
		}
	}

	// Validate suffix is not empty and doesn't contain path separators
	if flagTrimSuffix == "" {
		return common.FlagValidationError(cmd, "--suffix cannot be empty")
	}
	if strings.ContainsAny(flagTrimSuffix, "/\\") {
		return common.FlagValidationError(cmd, "--suffix cannot contain path separators")
	}

	return nil
}

// runTrimCmd executes the trim command
func runTrimCmd(cmd *cobra.Command, args []string) error {
	slog.Info("trimming metrics data",
		slog.String("input", flagTrimInput),
		slog.Int64("start-time", flagTrimStartTime),
		slog.Int64("end-time", flagTrimEndTime),
		slog.Int64("start-offset", flagTrimStartOffset),
		slog.Int64("end-offset", flagTrimEndOffset),
		slog.String("suffix", flagTrimSuffix))

	// Determine output directory
	outputDir := flagTrimOutputDir
	if outputDir == "" {
		outputDir = filepath.Dir(flagTrimInput)
	}

	// Load the original metrics CSV
	slog.Info("loading metrics from CSV", slog.String("file", flagTrimInput))
	metrics, err := newMetricCollection(flagTrimInput)
	if err != nil {
		return fmt.Errorf("failed to load metrics from CSV: %w", err)
	}

	if len(metrics) == 0 {
		return fmt.Errorf("no metrics found in CSV file")
	}

	// Calculate the time range
	startTime, endTime, err := calculateTimeRange(metrics, flagTrimStartTime, flagTrimEndTime,
		flagTrimStartOffset, flagTrimEndOffset)
	if err != nil {
		return fmt.Errorf("failed to calculate time range: %w", err)
	}

	slog.Info("calculated time range",
		slog.Int64("start", int64(startTime)),
		slog.Int64("end", int64(endTime)),
		slog.Float64("duration", endTime-startTime))

	// Filter metrics by time range
	originalRowCount := 0
	for i := range metrics {
		originalRowCount += len(metrics[i].rows)
	}

	metrics.filterByTimeRange(startTime, endTime)

	filteredRowCount := 0
	for i := range metrics {
		filteredRowCount += len(metrics[i].rows)
	}

	if filteredRowCount == 0 {
		return fmt.Errorf("no data remains after filtering to time range [%.2f, %.2f]", startTime, endTime)
	}

	slog.Info("filtered metrics",
		slog.Int("original_rows", originalRowCount),
		slog.Int("filtered_rows", filteredRowCount),
		slog.Int("removed_rows", originalRowCount-filteredRowCount))

	// Generate output filenames
	inputBase := filepath.Base(flagTrimInput)
	inputName := strings.TrimSuffix(inputBase, filepath.Ext(inputBase))

	// Determine target name from input filename
	// Input is typically "hostname_metrics.csv", target name is "hostname"
	targetName := strings.TrimSuffix(inputName, "_metrics")

	// Write trimmed metrics CSV
	trimmedCSVPath := filepath.Join(outputDir, targetName+"_metrics_"+flagTrimSuffix+".csv")
	if err := metrics.writeCSV(trimmedCSVPath); err != nil {
		return fmt.Errorf("failed to write trimmed CSV: %w", err)
	}
	slog.Info("wrote trimmed metrics CSV", slog.String("file", trimmedCSVPath))

	// Try to load metadata if it exists
	metadata, metadataFound, err := loadMetadataIfExists(flagTrimInput)
	if err != nil {
		slog.Warn("failed to load metadata, continuing without it", slog.String("error", err.Error()))
		metadataFound = false
	}

	if !metadataFound {
		slog.Warn("metadata file not found, HTML report will not include system summary")
		// Create minimal metadata for summary generation
		metadata = Metadata{}
	}

	// Load metric definitions for summary generation if we have a valid microarchitecture
	var metricDefinitions []MetricDefinition
	if metadataFound && metadata.Microarchitecture != "" {
		loader, err := NewLoader(metadata.Microarchitecture)
		if err != nil {
			return fmt.Errorf("failed to create loader: %w", err)
		}
		loaderConfig := LoaderConfig{
			Metadata: metadata,
		}
		metricDefinitions, _, err = loader.Load(loaderConfig)
		if err != nil {
			return fmt.Errorf("failed to load metric definitions: %w", err)
		}
	} else {
		// Use empty metric definitions if no metadata
		metricDefinitions = []MetricDefinition{}
	}

	// Generate summary files
	// Pass the base name (including _metrics) and suffix to generate consistent filenames
	trimmedBaseName := targetName + "_metrics_" + flagTrimSuffix
	filesCreated, err := generateTrimmedSummaries(trimmedCSVPath, outputDir, trimmedBaseName, metadata, metricDefinitions)
	if err != nil {
		return fmt.Errorf("failed to generate summary files: %w", err)
	}

	// Report success
	fmt.Println("\nTrimmed metrics successfully created:")
	fmt.Printf("  Trimmed CSV:     %s\n", trimmedCSVPath)
	for _, file := range filesCreated {
		fileType := "Summary"
		if strings.HasSuffix(file, ".html") {
			fileType = "HTML Summary"
		} else if strings.HasSuffix(file, ".csv") {
			fileType = "CSV Summary"
		}
		fmt.Printf("  %s: %s\n", fileType, file)
	}
	fmt.Printf("\nTime range: %d - %d seconds (%.0f second duration)\n", int64(startTime), int64(endTime), endTime-startTime)
	fmt.Printf("Rows: %d original, %d after trimming\n", originalRowCount, filteredRowCount)

	return nil
}

// calculateTimeRange determines the actual start and end times based on the flags and data
func calculateTimeRange(metrics MetricCollection, startTime, endTime, startOffset, endOffset int64) (float64, float64, error) {
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
		calcStartTime = float64(startTime)
	} else if startOffset != 0 {
		calcStartTime = minTimestamp + float64(startOffset)
	}

	// Calculate end time
	calcEndTime := maxTimestamp
	if endTime != 0 {
		calcEndTime = float64(endTime)
	} else if endOffset != 0 {
		calcEndTime = maxTimestamp - float64(endOffset)
	}

	// Validate the calculated range
	if calcStartTime >= calcEndTime {
		return 0, 0, fmt.Errorf("invalid time range: start (%d) >= end (%d)", int64(calcStartTime), int64(calcEndTime))
	}

	if calcStartTime > maxTimestamp {
		return 0, 0, fmt.Errorf("start time (%d) is beyond the end of available data (%d)", int64(calcStartTime), int64(maxTimestamp))
	}

	if calcEndTime < minTimestamp {
		return 0, 0, fmt.Errorf("end time (%d) is before the beginning of available data (%d)", int64(calcEndTime), int64(minTimestamp))
	}

	return calcStartTime, calcEndTime, nil
}
