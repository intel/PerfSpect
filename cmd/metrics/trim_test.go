package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestMetricsCSV creates a test CSV file with sample metrics data
func createTestMetricsCSV(t *testing.T, dir string, filename string) string {
	path := filepath.Join(dir, filename)
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	err = writer.Write([]string{"timestamp", "socket", "cpu", "cgroup", "metric_cpu_utilization", "metric_instructions"})
	require.NoError(t, err)

	// Write sample data with timestamps from 0 to 100 seconds
	for i := 0; i <= 20; i++ {
		timestamp := float64(i * 5) // 0, 5, 10, ..., 100
		err = writer.Write([]string{
			fmt.Sprintf("%.6f", timestamp),
			"",
			"",
			"",
			fmt.Sprintf("%.2f", 50.0+float64(i)),
			fmt.Sprintf("%.0f", 1000000.0*float64(i+1)),
		})
		require.NoError(t, err)
	}

	return path
}

func TestFilterByTimeRange(t *testing.T) {
	// Create test data
	metrics := MetricCollection{
		{
			names: []string{"metric1", "metric2"},
			rows: []row{
				{timestamp: 10.0, metrics: map[string]float64{"metric1": 1.0, "metric2": 2.0}},
				{timestamp: 20.0, metrics: map[string]float64{"metric1": 3.0, "metric2": 4.0}},
				{timestamp: 30.0, metrics: map[string]float64{"metric1": 5.0, "metric2": 6.0}},
				{timestamp: 40.0, metrics: map[string]float64{"metric1": 7.0, "metric2": 8.0}},
				{timestamp: 50.0, metrics: map[string]float64{"metric1": 9.0, "metric2": 10.0}},
			},
		},
	}

	tests := []struct {
		name          string
		startTime     float64
		endTime       float64
		expectedCount int
	}{
		{
			name:          "filter middle range",
			startTime:     20.0,
			endTime:       40.0,
			expectedCount: 3, // timestamps 20, 30, 40
		},
		{
			name:          "filter all",
			startTime:     10.0,
			endTime:       50.0,
			expectedCount: 5,
		},
		{
			name:          "filter to single point",
			startTime:     30.0,
			endTime:       30.0,
			expectedCount: 1,
		},
		{
			name:          "filter to none (range before data)",
			startTime:     1.0,
			endTime:       5.0,
			expectedCount: 0,
		},
		{
			name:          "filter to none (range after data)",
			startTime:     60.0,
			endTime:       70.0,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying the original
			testMetrics := make(MetricCollection, len(metrics))
			for i := range metrics {
				testMetrics[i] = MetricGroup{
					names: metrics[i].names,
					rows:  make([]row, len(metrics[i].rows)),
				}
				copy(testMetrics[i].rows, metrics[i].rows)
			}

			// Apply filter
			testMetrics.filterByTimeRange(tt.startTime, tt.endTime)

			// Check result
			assert.Equal(t, tt.expectedCount, len(testMetrics[0].rows))

			// Verify all remaining rows are in range
			for _, row := range testMetrics[0].rows {
				assert.GreaterOrEqual(t, row.timestamp, tt.startTime)
				assert.LessOrEqual(t, row.timestamp, tt.endTime)
			}
		})
	}
}

func TestCalculateTimeRange(t *testing.T) {
	// Create test data spanning from 10.0 to 100.0
	metrics := MetricCollection{
		{
			rows: []row{
				{timestamp: 10.0},
				{timestamp: 30.0},
				{timestamp: 50.0},
				{timestamp: 70.0},
				{timestamp: 100.0},
			},
		},
	}

	tests := []struct {
		name        string
		startTime   int64
		endTime     int64
		startOffset int64
		endOffset   int64
		wantStart   float64
		wantEnd     float64
		wantErr     bool
	}{
		{
			name:      "use absolute times",
			startTime: 20,
			endTime:   80,
			wantStart: 20.0,
			wantEnd:   80.0,
			wantErr:   false,
		},
		{
			name:        "use offsets from beginning and end",
			startOffset: 10,
			endOffset:   5,
			wantStart:   20.0, // 10.0 + 10.0
			wantEnd:     95.0, // 100.0 - 5.0
			wantErr:     false,
		},
		{
			name:      "use defaults (entire range)",
			wantStart: 10.0,
			wantEnd:   100.0,
			wantErr:   false,
		},
		{
			name:        "use start offset only",
			startOffset: 15,
			wantStart:   25.0,
			wantEnd:     100.0,
			wantErr:     false,
		},
		{
			name:      "use end time only",
			endTime:   60,
			wantStart: 10.0,
			wantEnd:   60.0,
			wantErr:   false,
		},
		{
			name:      "invalid range (start >= end)",
			startTime: 80,
			endTime:   20,
			wantErr:   true,
		},
		{
			name:        "invalid range (offset results in start >= end)",
			startOffset: 50,
			endOffset:   50,
			wantErr:     true,
		},
		{
			name:      "start time beyond data",
			startTime: 150,
			endTime:   200,
			wantErr:   true,
		},
		{
			name:      "end time before data",
			startTime: 1,
			endTime:   5,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd, err := calculateTimeRange(metrics, tt.startTime, tt.endTime, tt.startOffset, tt.endOffset)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantStart, gotStart)
				assert.Equal(t, tt.wantEnd, gotEnd)
			}
		})
	}
}

func TestWriteCSV(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name    string
		metrics MetricCollection
		wantErr bool
	}{
		{
			name: "write simple metrics",
			metrics: MetricCollection{
				{
					names: []string{"metric1", "metric2"},
					rows: []row{
						{
							timestamp: 10.5,
							socket:    "0",
							cpu:       "",
							cgroup:    "",
							metrics:   map[string]float64{"metric1": 1.5, "metric2": 2.5},
						},
						{
							timestamp: 20.5,
							socket:    "0",
							cpu:       "",
							cgroup:    "",
							metrics:   map[string]float64{"metric1": 3.5, "metric2": 4.5},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "write empty collection",
			metrics: MetricCollection{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tempDir, tt.name+".csv")
			err := tt.metrics.writeCSV(path)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify file was created and has content
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Greater(t, info.Size(), int64(0))

			// Read back and verify basic structure
			file, err := os.Open(path)
			require.NoError(t, err)
			defer file.Close()

			reader := csv.NewReader(file)
			records, err := reader.ReadAll()
			require.NoError(t, err)

			// Should have header + data rows
			expectedRows := 1 + len(tt.metrics[0].rows)
			assert.Equal(t, expectedRows, len(records))

			// Verify header
			assert.Equal(t, "timestamp", records[0][0])
			assert.Equal(t, "socket", records[0][1])
			assert.Equal(t, "cpu", records[0][2])
			assert.Equal(t, "cgroup", records[0][3])
		})
	}
}

func TestLoadMetadataIfExists(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("metadata exists", func(t *testing.T) {
		// Create a metrics CSV file
		metricsPath := filepath.Join(tempDir, "test_metrics.csv")
		_, err := os.Create(metricsPath)
		require.NoError(t, err)

		// Create a corresponding metadata JSON file
		metadataPath := filepath.Join(tempDir, "test_metadata.json")
		metadataContent := `{"Hostname":"testhost","Microarchitecture":"SPR"}`
		err = os.WriteFile(metadataPath, []byte(metadataContent), 0644)
		require.NoError(t, err)

		// Load metadata
		metadata, found, err := loadMetadataIfExists(metricsPath)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "testhost", metadata.Hostname)
		assert.Equal(t, "SPR", metadata.Microarchitecture)
	})

	t.Run("metadata does not exist", func(t *testing.T) {
		// Create a metrics CSV file without metadata
		metricsPath := filepath.Join(tempDir, "nometa_metrics.csv")
		_, err := os.Create(metricsPath)
		require.NoError(t, err)

		// Try to load metadata
		_, found, err := loadMetadataIfExists(metricsPath)
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("metadata file is malformed", func(t *testing.T) {
		// Create a metrics CSV file
		metricsPath := filepath.Join(tempDir, "badmeta_metrics.csv")
		_, err := os.Create(metricsPath)
		require.NoError(t, err)

		// Create a malformed metadata JSON file
		metadataPath := filepath.Join(tempDir, "badmeta_metadata.json")
		err = os.WriteFile(metadataPath, []byte("not valid json{"), 0644)
		require.NoError(t, err)

		// Try to load metadata
		_, found, err := loadMetadataIfExists(metricsPath)
		assert.Error(t, err)
		assert.False(t, found)
	})
}

func TestTrimValidateFlags(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test CSV file
	testCSV := createTestMetricsCSV(t, tempDir, "test_metrics.csv")

	tests := []struct {
		name    string
		setup   func()
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid input file",
			setup: func() {
				flagTrimInput = testCSV
				flagTrimStartOffset = 10
				flagTrimSuffix = "trimmed"
			},
			wantErr: false,
		},
		{
			name: "no time parameters specified",
			setup: func() {
				flagTrimInput = testCSV
				flagTrimSuffix = "trimmed"
			},
			wantErr: true,
			errMsg:  "at least one time parameter must be specified",
		},
		{
			name: "input file does not exist",
			setup: func() {
				flagTrimInput = filepath.Join(tempDir, "nonexistent.csv")
				flagTrimStartOffset = 10
				flagTrimSuffix = "trimmed"
			},
			wantErr: true,
			errMsg:  "does not exist",
		},
		{
			name: "input is not a CSV file",
			setup: func() {
				txtFile := filepath.Join(tempDir, "test.txt")
				_ = os.WriteFile(txtFile, []byte("test"), 0644) // #nosec G306
				flagTrimInput = txtFile
				flagTrimStartOffset = 10
				flagTrimSuffix = "trimmed"
			},
			wantErr: true,
			errMsg:  "must be a CSV file",
		},
		{
			name: "both start-time and start-offset specified",
			setup: func() {
				flagTrimInput = testCSV
				flagTrimStartTime = 10
				flagTrimStartOffset = 5
				flagTrimSuffix = "trimmed"
			},
			wantErr: true,
			errMsg:  "cannot specify both",
		},
		{
			name: "both end-time and end-offset specified",
			setup: func() {
				flagTrimInput = testCSV
				flagTrimStartTime = 0
				flagTrimStartOffset = 0
				flagTrimEndTime = 50
				flagTrimEndOffset = 10
				flagTrimSuffix = "trimmed"
			},
			wantErr: true,
			errMsg:  "cannot specify both",
		},
		{
			name: "negative start-time",
			setup: func() {
				flagTrimInput = testCSV
				flagTrimStartTime = -10.0
				flagTrimEndTime = 0
				flagTrimStartOffset = 0
				flagTrimEndOffset = 0
				flagTrimSuffix = "trimmed"
			},
			wantErr: true,
			errMsg:  "cannot be negative",
		},
		{
			name: "start-time >= end-time",
			setup: func() {
				flagTrimInput = testCSV
				flagTrimStartTime = 50
				flagTrimEndTime = 40
				flagTrimStartOffset = 0
				flagTrimEndOffset = 0
				flagTrimSuffix = "trimmed"
			},
			wantErr: true,
			errMsg:  "must be less than",
		},
		{
			name: "empty suffix",
			setup: func() {
				flagTrimInput = testCSV
				flagTrimStartTime = 0
				flagTrimEndTime = 0
				flagTrimStartOffset = 10
				flagTrimEndOffset = 0
				flagTrimSuffix = ""
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name: "suffix with path separator",
			setup: func() {
				flagTrimInput = testCSV
				flagTrimSuffix = "trim/med"
				flagTrimStartTime = 0
				flagTrimEndTime = 0
				flagTrimStartOffset = 10
				flagTrimEndOffset = 0
			},
			wantErr: true,
			errMsg:  "cannot contain path separators",
		},
		{
			name: "output directory does not exist",
			setup: func() {
				flagTrimInput = testCSV
				flagTrimOutputDir = filepath.Join(tempDir, "nonexistent")
				flagTrimSuffix = "trimmed"
				flagTrimStartTime = 0
				flagTrimEndTime = 0
				flagTrimStartOffset = 10
				flagTrimEndOffset = 0
			},
			wantErr: true,
			errMsg:  "does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags to defaults
			flagTrimInput = ""
			flagTrimStartTime = 0
			flagTrimEndTime = 0
			flagTrimStartOffset = 0
			flagTrimEndOffset = 0
			flagTrimOutputDir = ""
			flagTrimSuffix = "trimmed"

			// Setup test-specific flags
			tt.setup()

			// Validate
			err := validateTrimFlags(trimCmd, nil)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
