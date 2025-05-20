package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// functions to create summary (mean,min,max,stddev) metrics from metrics CSV

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	texttemplate "text/template" // nosemgrep
	"time"
)

func summarizeMetrics(localOutputDir string, targetName string, metadata Metadata) ([]string, error) {
	filesCreated := []string{}
	csvMetricsFile := filepath.Join(localOutputDir, targetName+"_metrics.csv")
	// csv summary
	out, err := summarize(csvMetricsFile, false, metadata)
	if err != nil {
		err = fmt.Errorf("failed to summarize output: %w", err)
		return filesCreated, err
	}
	csvSummaryFile := filepath.Join(localOutputDir, targetName+"_metrics_summary.csv")
	err = os.WriteFile(csvSummaryFile, []byte(out), 0644) // #nosec G306
	if err != nil {
		err = fmt.Errorf("failed to write summary to file: %w", err)
		return filesCreated, err
	}
	filesCreated = append(filesCreated, csvSummaryFile)
	// html summary
	htmlSummary := (flagScope == scopeSystem || flagScope == scopeProcess) && flagGranularity == granularitySystem
	if htmlSummary {
		out, err = summarize(csvMetricsFile, true, metadata)
		if err != nil {
			err = fmt.Errorf("failed to summarize output as HTML: %w", err)
			return filesCreated, err
		}
		htmlSummaryFile := filepath.Join(localOutputDir, targetName+"_metrics_summary.html")
		err = os.WriteFile(htmlSummaryFile, []byte(out), 0644) // #nosec G306
		if err != nil {
			err = fmt.Errorf("failed to write HTML summary to file: %w", err)
			return filesCreated, err
		}
		filesCreated = append(filesCreated, htmlSummaryFile)
	}
	return filesCreated, nil
}

// summarize - generates formatted output from a CSV file containing metric values.
// The output can be in CSV or HTML format. Set html to true to generate HTML output otherwise CSV is generated.
func summarize(csvInputPath string, html bool, metadata Metadata) (out string, err error) {
	var metrics []metricsFromCSV
	if metrics, err = newMetricsFromCSV(csvInputPath); err != nil {
		return
	}
	if html {
		if len(metrics) > 1 {
			err = fmt.Errorf("html format is supported only when data's scope is '%s' or '%s' and granularity is '%s'", scopeSystem, scopeProcess, granularitySystem)
			return
		}
		out, err = metrics[0].getHTML(metadata)
	} else {
		for i, m := range metrics {
			var oneOut string
			if oneOut, err = m.getCSV(i == 0); err != nil {
				return
			}
			out += oneOut
		}
	}
	return
}

type metricStats struct {
	mean   float64
	min    float64
	max    float64
	stddev float64
}

type row struct {
	timestamp float64
	socket    string
	cpu       string
	cgroup    string
	metrics   map[string]float64
}

// newRow loads a row structure with given fields and field names
func newRow(fields []string, names []string) (r row, err error) {
	r.metrics = make(map[string]float64)
	for fIdx, field := range fields {
		if fIdx == idxTimestamp {
			var ts float64
			if ts, err = strconv.ParseFloat(field, 64); err != nil {
				return
			}
			r.timestamp = ts
		} else if fIdx == idxSocket {
			r.socket = field
		} else if fIdx == idxCPU {
			r.cpu = field
		} else if fIdx == idxCgroup {
			r.cgroup = field
		} else {
			// metrics
			var v float64
			if field != "" {
				if v, err = strconv.ParseFloat(field, 64); err != nil {
					return
				}
			} else {
				v = math.NaN()
			}
			r.metrics[names[fIdx-idxFirstMetric]] = v
		}
	}
	return
}

const (
	idxTimestamp int = iota
	idxSocket
	idxCPU
	idxCgroup
	idxFirstMetric
)

type metricsFromCSV struct {
	names        []string
	rows         []row
	groupByField string
	groupByValue string
}

// newMetricsFromCSV - loads data from CSV. Returns a list of metrics, one per
// scope unit or granularity unit, e.g., one per socket, or one per PID
func newMetricsFromCSV(csvPath string) (metrics []metricsFromCSV, err error) {
	file, err := os.Open(csvPath) // #nosec G304
	if err != nil {
		return
	}
	reader := csv.NewReader(file)
	groupByField := -1
	var groupByValues []string
	var metricNames []string
	var nonMetricNames []string
	for idx := 0; true; idx++ {
		var fields []string
		if fields, err = reader.Read(); err != nil {
			if err != io.EOF {
				return
			}
			err = nil
		}
		if fields == nil {
			// no more rows
			break
		}
		if idx == 0 {
			// headers
			for fIdx, field := range fields {
				if fIdx < idxFirstMetric {
					nonMetricNames = append(nonMetricNames, field)
				} else {
					metricNames = append(metricNames, field)
				}
			}
			continue
		}
		// Determine the scope and granularity of the captured data by looking
		// at the first row of values. If none of these are set, then it's
		// system scope and system granularity
		if idx == 1 {
			if fields[idxSocket] != "" {
				groupByField = idxSocket
			} else if fields[idxCPU] != "" {
				groupByField = idxCPU
			} else if fields[idxCgroup] != "" {
				groupByField = idxCgroup
			}
		}
		// Load row into a row structure
		var r row
		if r, err = newRow(fields, metricNames); err != nil {
			return
		}
		// put the row into the associated list based on groupByField
		if groupByField == -1 { // system scope/granularity
			if len(metrics) == 0 {
				metrics = append(metrics, metricsFromCSV{})
				metrics[0].names = metricNames
			}
			metrics[0].rows = append(metrics[0].rows, r)
		} else {
			groupByValue := fields[groupByField]
			var listIdx int
			if listIdx = slices.Index(groupByValues, groupByValue); listIdx == -1 {
				groupByValues = append(groupByValues, groupByValue)
				metrics = append(metrics, metricsFromCSV{})
				listIdx = len(metrics) - 1
				metrics[listIdx].names = metricNames
				if groupByField == idxSocket {
					metrics[listIdx].groupByField = nonMetricNames[idxSocket]
				} else if groupByField == idxCPU {
					metrics[listIdx].groupByField = nonMetricNames[idxCPU]
				} else if groupByField == idxCgroup {
					metrics[listIdx].groupByField = nonMetricNames[idxCgroup]
				}
				metrics[listIdx].groupByValue = groupByValue
			}
			metrics[listIdx].rows = append(metrics[listIdx].rows, r)
		}
	}
	return
}

// getStats - calculate summary stats (min, max, mean, stddev) for each metric
func (m *metricsFromCSV) getStats() (stats map[string]metricStats, err error) {
	stats = make(map[string]metricStats)
	for _, metricName := range m.names {
		min := math.NaN()
		max := math.NaN()
		mean := math.NaN()
		stddev := math.NaN()
		count := 0
		sum := 0.0
		for _, row := range m.rows {
			val := row.metrics[metricName]
			if math.IsNaN(val) || math.IsInf(val, 0) {
				continue
			}
			if math.IsNaN(min) { // min was initialized to NaN
				// first non-NaN value, so initialize
				min = math.MaxFloat64
				max = 0
				sum = 0
			}
			if val < min {
				min = val
			}
			if val > max {
				max = val
			}
			sum += val
			count++
		}
		// must be at least one valid value for this metric to calculate mean and standard deviation
		if count > 0 {
			mean = sum / float64(count)
			distanceSquaredSum := 0.0
			for _, row := range m.rows {
				val := row.metrics[metricName]
				if math.IsNaN(val) || math.IsInf(val, 0) {
					continue
				}
				distance := mean - val
				squared := distance * distance
				distanceSquaredSum += squared
			}
			stddev = math.Sqrt(distanceSquaredSum / float64(count))
		}
		stats[metricName] = metricStats{mean: mean, min: min, max: max, stddev: stddev}
	}
	return
}

// getHTML - generate a string containing HTML representing the metrics
func (m *metricsFromCSV) getHTML(metadata Metadata) (out string, err error) {
	var htmlTemplateBytes []byte
	if htmlTemplateBytes, err = resources.ReadFile("resources/base.html"); err != nil {
		slog.Error("failed to read base.html template", slog.String("error", err.Error()))
		return
	}
	templateVals, err := m.loadHTMLTemplateValues(metadata)
	if err != nil {
		slog.Error("failed to load template values", slog.String("error", err.Error()))
		return
	}
	fg := texttemplate.Must(texttemplate.New("metricsSummaryTemplate").Delims("<<", ">>").Parse(string(htmlTemplateBytes)))
	buf := new(bytes.Buffer)
	if err = fg.Execute(buf, templateVals); err != nil {
		slog.Error("failed to render metrics template", slog.String("error", err.Error()))
		return
	}
	return buf.String(), nil
}

func (m *metricsFromCSV) loadHTMLTemplateValues(metadata Metadata) (templateVals map[string]string, err error) {
	templateVals = make(map[string]string)
	var stats map[string]metricStats
	if stats, err = m.getStats(); err != nil {
		return
	}
	//0 -> Intel, 1 -> AMD
	archIndex := 0
	if metadata.Vendor == "AuthenticAMD" {
		archIndex = 1
	}

	type tmplReplace struct {
		tmplVar     string
		metricNames []string // names per architecture, 0=Intel, 1=AMD
	}

	templateVals["TRANSACTIONS"] = "false" // no transactions for now

	// TMA Tab's pie chart
	// these are intended to be replaced with pie headers in html report
	templateNameReplace := []tmplReplace{
		{"TMA_FRONTEND", []string{"Frontend", "Frontend"}},
		{"TMA_FETCHLATENCY", []string{"Fetch Latency", "Latency"}},
		{"TMA_FETCHBANDWIDTH", []string{"Fetch Bandwidth", "Bandwidth"}},
		{"TMA_BADSPECULATION", []string{"Bad Speculation", "Bad Speculation"}},
		{"TMA_BRANCHMISPREDICTS", []string{"Branch Mispredicts", "Mispredicts"}},
		{"TMA_MACHINECLEARS", []string{"Machine Clears", "Pipeline Restarts"}},
		{"TMA_BACKEND", []string{"Backend", "Backend"}},
		{"TMA_CORE", []string{"Core", "CPU"}},
		{"TMA_MEMORY", []string{"Memory", "Memory"}},
		{"TMA_RETIRING", []string{"Retiring", "Retiring"}},
		{"TMA_LIGHTOPS", []string{"Light Operations", "Fastpath"}},
		{"TMA_HEAVYOPS", []string{"Heavy Operations", "Microcode"}},
	}
	// replace the template variables with the name header of the metric
	for _, tmpl := range templateNameReplace {
		var headerName string
		if len(tmpl.metricNames) > archIndex {
			headerName = tmpl.metricNames[archIndex]
		}
		templateVals[tmpl.tmplVar] = headerName
	}
	// TMA Tab's pie chart
	// these are intended to be replaced with the mean value of the metric
	templateReplace := []tmplReplace{
		{"FRONTEND", []string{"TMA_Frontend_Bound(%)", "Pipeline Utilization - Frontend Bound (%)"}},
		{"FETCHLATENCY", []string{"TMA_..Fetch_Latency(%)", "Pipeline Utilization - Frontend Bound - Latency (%)"}},
		{"FETCHBANDWIDTH", []string{"TMA_..Fetch_Bandwidth(%)", "Pipeline Utilization - Frontend Bound - Bandwidth (%)"}},
		{"BADSPECULATION", []string{"TMA_Bad_Speculation(%)", "Pipeline Utilization - Bad Speculation (%)"}},
		{"BRANCHMISPREDICTS", []string{"TMA_..Branch_Mispredicts(%)", "Pipeline Utilization - Bad Speculation - Mispredicts (%)"}},
		{"MACHINECLEARS", []string{"TMA_..Machine_Clears(%)", "Pipeline Utilization - Bad Speculation - Pipeline Restarts (%)"}},
		{"BACKEND", []string{"TMA_Backend_Bound(%)", "Pipeline Utilization - Backend Bound (%)"}},
		{"COREDATA", []string{"TMA_..Core_Bound(%)", "Pipeline Utilization - Backend Bound - CPU (%)"}},
		{"MEMORY", []string{"TMA_..Memory_Bound(%)", "Pipeline Utilization - Backend Bound - Memory (%)"}},
		{"RETIRING", []string{"TMA_Retiring(%)", "Pipeline Utilization - Retiring (%)"}},
		{"LIGHTOPS", []string{"TMA_..Light_Operations(%)", "Pipeline Utilization - Retiring - Fastpath (%)"}},
		{"HEAVYOPS", []string{"TMA_..Heavy_Operations(%)", "Pipeline Utilization - Retiring - Microcode (%)"}},
	}
	// replace the template variables with the mean value of the metric
	for _, tmpl := range templateReplace {
		// confirm that the metric name exists in the stats, otherwise set it to 0
		metricMean := 0.0
		if len(tmpl.metricNames) > archIndex {
			if _, ok := stats[tmpl.metricNames[archIndex]]; ok {
				metricMean = stats[tmpl.metricNames[archIndex]].mean
				if math.IsInf(metricMean, 0) || math.IsNaN(metricMean) || metricMean < 0 {
					metricMean = 0
				}
			}
		}
		templateVals[tmpl.tmplVar] = fmt.Sprintf("%f", metricMean)
	}
	// these get the series data for the graphs
	templateReplace = []tmplReplace{
		// TMAM Tab
		{"TMAFRONTEND", []string{"TMA_Frontend_Bound(%)", "Pipeline Utilization - Frontend Bound (%)"}},
		{"TMABACKEND", []string{"TMA_Backend_Bound(%)", "Pipeline Utilization - Backend Bound (%)"}},
		{"TMARETIRING", []string{"TMA_Retiring(%)", "Pipeline Utilization - Retiring (%)"}},
		{"TMABADSPECULATION", []string{"TMA_Bad_Speculation(%)", "Pipeline Utilization - Bad Speculation (%)"}},
		// CPU Tab
		{"CPUUTIL", []string{"CPU utilization %", "CPU utilization %"}},
		{"CPIDATA", []string{"CPI", "CPI"}},
		{"CPUFREQ", []string{"CPU operating frequency (in GHz)", "CPU operating frequency (in GHz)"}},
		// Memory Tab
		{"L1DATA", []string{"L1D MPI (includes data+rfo w/ prefetches)", ""}},
		{"L2DATA", []string{"L2 MPI (includes code+data+rfo w/ prefetches)", ""}},
		{"LLCDATA", []string{"LLC data read MPI (demand+prefetch)", ""}},
		{"READDATA", []string{"memory bandwidth read (MB/sec)", "Read Memory Bandwidth (MB/sec)"}},
		{"WRITEDATA", []string{"memory bandwidth write (MB/sec)", "Write Memory Bandwidth (MB/sec)"}},
		{"TOTALDATA", []string{"memory bandwidth total (MB/sec)", "Total Memory Bandwidth (MB/sec)"}},
		{"REMOTENUMA", []string{"NUMA %_Reads addressed to remote DRAM", "Remote DRAM Reads %"}},
		// Power Tab
		{"PKGPOWER", []string{"package power (watts)", "package power (watts)"}},
		{"DRAMPOWER", []string{"DRAM power (watts)", ""}},
	}
	// replace the template variables with the series data
	for tIdx, tmpl := range templateReplace {
		var timeStamps []string
		var series [][]float64
		for rIdx, row := range m.rows {
			metricRowVal := row.metrics[tmpl.metricNames[archIndex]]
			if math.IsNaN(metricRowVal) || math.IsInf(metricRowVal, 0) || metricRowVal < 0 {
				metricRowVal = 0
			}
			series = append(series, []float64{float64(rIdx), metricRowVal})
			// format the UNIX timestamp as a local tz string
			ts := time.Unix(int64(row.timestamp), 0).Format("15:04:05")
			timeStamps = append(timeStamps, ts)
		}
		var seriesBytes []byte
		if seriesBytes, err = json.Marshal(series); err != nil {
			return
		}
		templateVals[tmpl.tmplVar] = string(seriesBytes)
		if tIdx == 0 {
			var timeStampsBytes []byte
			if timeStampsBytes, err = json.Marshal(timeStamps); err != nil {
				return
			}
			templateVals["TIMESTAMPS"] = string(timeStampsBytes)
		}
	}
	// All Metrics Tab
	var metricHTMLStats [][]string
	for _, name := range m.names {
		metricHTMLStats = append(metricHTMLStats, []string{
			name,
			fmt.Sprintf("%f", stats[name].mean),
			fmt.Sprintf("%f", stats[name].min),
			fmt.Sprintf("%f", stats[name].max),
			fmt.Sprintf("%f", stats[name].stddev),
		})
	}
	var jsonMetricsBytes []byte
	if jsonMetricsBytes, err = json.Marshal(metricHTMLStats); err != nil {
		return
	}
	jsonMetrics := string(jsonMetricsBytes)
	templateVals["ALLMETRICS"] = jsonMetrics
	// Metadata tab
	jsonMetadata, err := metadata.JSON()
	if err != nil {
		return
	}
	// remove PerfSupportedEvents from json
	re := regexp.MustCompile(`"PerfSupportedEvents":".*?",`)
	jsonMetadataPurged := re.ReplaceAll(jsonMetadata, []byte(""))
	// remove SystemSummaryFields from json
	re = regexp.MustCompile(`,"SystemSummaryFields":\[\[.*?\]\]`)
	jsonMetadataPurged = re.ReplaceAll(jsonMetadataPurged, []byte(""))
	templateVals["METADATA"] = string(jsonMetadataPurged)
	// system info tab
	jsonSystemInfo, err := json.Marshal(metadata.SystemSummaryFields)
	if err != nil {
		return
	}
	templateVals["SYSTEMINFO"] = string(jsonSystemInfo)
	return
}

// getCSV - generate CSV string representing the summary statistics of the metrics
func (m *metricsFromCSV) getCSV(includeFieldNames bool) (out string, err error) {
	var stats map[string]metricStats
	if stats, err = m.getStats(); err != nil {
		return
	}
	if includeFieldNames {
		out = "metric,mean,min,max,stddev\n"
		if m.groupByField != "" {
			out = m.groupByField + "," + out
		}
	}
	for _, name := range m.names {
		if m.groupByValue == "" {
			out += fmt.Sprintf("%s,%f,%f,%f,%f\n", name, stats[name].mean, stats[name].min, stats[name].max, stats[name].stddev)
		} else {
			out += fmt.Sprintf("%s,%s,%f,%f,%f,%f\n", m.groupByValue, name, stats[name].mean, stats[name].min, stats[name].max, stats[name].stddev)
		}
	}
	return
}
