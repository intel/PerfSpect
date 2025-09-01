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

	"github.com/Knetic/govaluate"
)

func summarizeMetrics(localOutputDir string, targetName string, metadata Metadata, metricDefinitions []MetricDefinition) ([]string, error) {
	filesCreated := []string{}
	// read the metrics from CSV
	csvMetricsFile := filepath.Join(localOutputDir, targetName+"_metrics.csv")
	metrics, err := newMetricCollection(csvMetricsFile)
	if err != nil {
		return filesCreated, fmt.Errorf("failed to read metrics from %s: %w", csvMetricsFile, err)
	}
	// csv summary
	out, err := metrics.getCSV()
	if err != nil {
		err = fmt.Errorf("failed to summarize output as CSV: %w", err)
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
	out, err = metrics.getHTML(metadata, metricDefinitions)
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
	return filesCreated, nil
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
		switch fIdx {
		case idxTimestamp:
			var ts float64
			if ts, err = strconv.ParseFloat(field, 64); err != nil {
				return
			}
			r.timestamp = ts
		case idxSocket:
			r.socket = field
		case idxCPU:
			r.cpu = field
		case idxCgroup:
			r.cgroup = field
		default:
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

// indexes of known fields in CSV
const (
	idxTimestamp int = iota
	idxSocket
	idxCPU
	idxCgroup
	idxFirstMetric
)

// MetricGroup - holds a group of metrics, e.g., one per socket, cpu, or cgroup
type MetricGroup struct {
	names        []string
	rows         []row
	groupByField string
	groupByValue string
}

// MetricCollection - a collection of MetricGroup, one per scope unit or granularity unit
type MetricCollection []MetricGroup

// newMetricCollection - loads data from CSV. Returns a list of metrics, one per
// scope unit or granularity unit, i.e., one per socket, one per CPU, one per cgroup,
// or one for the entire system if no disaggregation is present.
func newMetricCollection(csvPath string) (metrics MetricCollection, err error) {
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
				metrics = append(metrics, MetricGroup{})
				metrics[0].names = metricNames
			}
			metrics[0].rows = append(metrics[0].rows, r)
		} else {
			groupByValue := fields[groupByField]
			var listIdx int
			if listIdx = slices.Index(groupByValues, groupByValue); listIdx == -1 {
				groupByValues = append(groupByValues, groupByValue)
				metrics = append(metrics, MetricGroup{})
				listIdx = len(metrics) - 1
				metrics[listIdx].names = metricNames
				switch groupByField {
				case idxSocket:
					metrics[listIdx].groupByField = nonMetricNames[idxSocket]
				case idxCPU:
					metrics[listIdx].groupByField = nonMetricNames[idxCPU]
				case idxCgroup:
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
func (mg *MetricGroup) getStats() (stats map[string]metricStats, err error) {
	stats = make(map[string]metricStats)
	for _, metricName := range mg.names {
		min := math.NaN()
		max := math.NaN()
		mean := math.NaN()
		stddev := math.NaN()
		count := 0
		sum := 0.0
		for _, row := range mg.rows {
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
			for _, row := range mg.rows {
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

// aggregate - combine multiple metricsFromCSV into a single one by averaging the metrics
// This is used when there are multiple metricsFromCSV objects, e.g., one per socket, cpu, or cgroup.
// The output metricsFromCSV will have groupByField and groupByValue set to empty strings.
func (mc MetricCollection) aggregate() (m *MetricGroup, err error) {
	if len(mc) == 0 {
		err = fmt.Errorf("no metrics to aggregate")
		return
	}
	if len(mc) == 1 {
		// if there is only one metricsFromCSV, then it is system scope and granularity
		// so we can just use the first one
		return &mc[0], nil
	}
	// Validate groupByField for all metrics
	validGroupByFields := []string{"SKT", "CPU", "CID"}
	for i, m := range mc {
		if !slices.Contains(validGroupByFields, m.groupByField) {
			return nil, fmt.Errorf("invalid groupByField in metrics[%d]: %s", i, m.groupByField)
		}
	}
	// first, get the names of the metrics
	metricNames := mc[0].names
	for idx, m := range mc[1:] {
		if !slices.Equal(m.names, metricNames) {
			return nil, fmt.Errorf("metricsFromCSV objects have different metric names or order at index %d: %v vs %v", idx+1, m.names, metricNames)
		}
	}
	// create the output metricsFromCSV
	m = &MetricGroup{
		names:        metricNames,
		groupByField: "",
		groupByValue: "",
	}
	// aggregate the rows by timestamp
	timestampMap := make(map[float64][]map[string]float64) // map of timestamp to list of metric maps
	var timestamps []float64                               // list of timestamps in order
	for _, metrics := range mc {
		for _, row := range metrics.rows {
			if _, ok := timestampMap[row.timestamp]; !ok {
				timestamps = append(timestamps, row.timestamp)
			}
			timestampMap[row.timestamp] = append(timestampMap[row.timestamp], row.metrics)
		}
	}
	// for each timestamp, average the metrics
	for _, ts := range timestamps {
		metricList := timestampMap[ts]
		avgMetrics := make(map[string]float64)
		for _, metricName := range metricNames {
			sum := 0.0
			count := 0
			for _, metrics := range metricList {
				val := metrics[metricName]
				if math.IsNaN(val) || math.IsInf(val, 0) {
					continue
				}
				sum += val
				count++
			}
			if count > 0 {
				avgMetrics[metricName] = sum / float64(count)
			} else {
				avgMetrics[metricName] = math.NaN()
			}
		}
		m.rows = append(m.rows, row{timestamp: ts, metrics: avgMetrics})
	}
	return
}

// getHTML - generate a string containing HTML representing the metrics
func (mg *MetricGroup) getHTML(metadata Metadata, metricDefinitions []MetricDefinition) (out string, err error) {
	var htmlTemplateBytes []byte
	if htmlTemplateBytes, err = resources.ReadFile("resources/base.html"); err != nil {
		slog.Error("failed to read base.html template", slog.String("error", err.Error()))
		return
	}
	templateVals, err := mg.loadHTMLTemplateValues(metadata, metricDefinitions)
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

func (mc MetricCollection) getHTML(metadata Metadata, metricDefinitions []MetricDefinition) (out string, err error) {
	if len(mc) == 0 {
		err = fmt.Errorf("no metrics to summarize")
		return
	}
	if len(mc) == 1 {
		return mc[0].getHTML(metadata, metricDefinitions)
	}
	metrics, err := mc.aggregate()
	if err != nil {
		return
	}
	out, err = metrics.getHTML(metadata, metricDefinitions)
	return
}

type tmaTip struct {
	Issue string `json:"issue"`
	Tip   string `json:"tip"`
}

func (mg *MetricGroup) loadHTMLTemplateValues(metadata Metadata, metricDefinitions []MetricDefinition) (templateVals map[string]string, err error) {
	templateVals = make(map[string]string)
	var stats map[string]metricStats
	if stats, err = mg.getStats(); err != nil {
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

	// TMA Tab's pie chart (labels)
	// templateLabelReplace is a list of template variables that are used as labels for the TMA pie chart
	// The template variable is replaced with the label appropriate for the architecture
	templateLabelReplace := []tmplReplace{
		{"FRONTEND_LABEL", []string{"Frontend", "Frontend"}},                     // level 1
		{"FETCHLATENCY_LABEL", []string{"Fetch Latency", "Latency"}},             // level 2
		{"FETCHBANDWIDTH_LABEL", []string{"Fetch BW", "Bandwidth"}},              // level 2
		{"BADSPECULATION_LABEL", []string{"Bad Speculation", "Bad Speculation"}}, // level 1
		{"BRANCHMISPREDICTS_LABEL", []string{"Mispredicts", "Mispredicts"}},      // level 2
		{"MACHINECLEARS_LABEL", []string{"Machine Clears", "Pipeline Restarts"}}, // level 2
		{"BACKEND_LABEL", []string{"Backend", "Backend"}},                        // level 1
		{"MEMORY_LABEL", []string{"Memory", "Memory"}},                           // level 2
		{"CORE_LABEL", []string{"Core", "CPU"}},                                  // level 2
		{"RETIRING_LABEL", []string{"Retiring", "Retiring"}},                     // level 1
		{"LIGHTOPS_LABEL", []string{"Light Ops", "Fastpath"}},                    // level 2
		{"HEAVYOPS_LABEL", []string{"Heavy Ops", "Microcode"}},                   // level 2
	}
	// replace the template variables with the label of the metric for the pie chart
	for _, tmpl := range templateLabelReplace {
		var label string
		if len(tmpl.metricNames) > archIndex {
			label = tmpl.metricNames[archIndex]
		}
		templateVals[tmpl.tmplVar] = label
	}
	// TMA Tab's pie chart (values)
	// templateReplace is a list of template variables to replace with the mean value of
	// the metric named in the metricNames field for the architecture
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
		for rIdx, row := range mg.rows {
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
	// load the TMA metrics tuning tips from resources
	var tmaTipsBytes []byte
	if tmaTipsBytes, err = resources.ReadFile("resources/perfmon/tma_tuning_tips.json"); err != nil {
		slog.Error("failed to read tma_tips.json", slog.String("error", err.Error()))
		return
	}
	tmaTips := make(map[string]tmaTip)
	if err = json.Unmarshal(tmaTipsBytes, &tmaTips); err != nil {
		slog.Error("failed to unmarshal tma_tips.json", slog.String("error", err.Error()))
		return
	}
	var metricHTMLStats [][]string
	for _, name := range mg.names {
		metricVals := []string{
			name,                                  // column 0
			fmt.Sprintf("%f", stats[name].mean),   // column 1
			fmt.Sprintf("%f", stats[name].min),    // column 2
			fmt.Sprintf("%f", stats[name].max),    // column 3
			fmt.Sprintf("%f", stats[name].stddev), // column 4
		}
		metricDef := findMetricDefinitionByName(name, metricDefinitions)
		if metricDef != nil {
			exceeded, thresholdDescription := getThresholdInfo(*metricDef, stats, metricDefinitions, tmaTips)
			metricVals = append(metricVals, exceeded)                                   // column 5 - "Yes" if threshold exceeded, else "No"
			metricVals = append(metricVals, thresholdDescription)                       // column 6 - issue/tip or threshold itself
			metricVals = append(metricVals, fmt.Sprintf("%d", max(metricDef.Level, 1))) // column 7 - metric level (for TMA metrics)
		} else {
			// this shouldn't happen, but just in case
			metricVals = append(metricVals, "No") // 5
			metricVals = append(metricVals, "")   // 6
			metricVals = append(metricVals, "")   // 7
			slog.Error("metric definition not found for metric", slog.String("metric", name))
		}
		metricHTMLStats = append(metricHTMLStats, metricVals)
	}
	var jsonMetricsBytes []byte
	if jsonMetricsBytes, err = json.Marshal(metricHTMLStats); err != nil {
		return
	}
	jsonMetrics := string(jsonMetricsBytes)
	templateVals["ALLMETRICS"] = jsonMetrics
	// Add metric descriptions for tooltip info
	metricDescriptionMap := make(map[string]string, len(metricDefinitions))
	for _, def := range metricDefinitions {
		if def.Description != "" {
			metricDescriptionMap[getMetricDisplayName(def)] = def.Description
		}
	}
	var jsonMetricDescBytes []byte
	if jsonMetricDescBytes, err = json.Marshal(metricDescriptionMap); err != nil {
		return
	}
	templateVals["DESCRIPTION"] = string(jsonMetricDescBytes)

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

func findMetricDefinitionByName(name string, metricDefinitions []MetricDefinition) *MetricDefinition {
	for i, d := range metricDefinitions {
		if getMetricDisplayName(d) == name {
			return &metricDefinitions[i]
		}
	}
	return nil
}

func findMetricDefinitionByLegacyName(legacyName string, metricDefinitions []MetricDefinition) *MetricDefinition {
	for i, d := range metricDefinitions {
		if d.LegacyName == legacyName {
			return &metricDefinitions[i]
		}
	}
	return nil
}

func getThresholdInfo(metricDef MetricDefinition, stats map[string]metricStats, metricDefinitions []MetricDefinition, tmaTips map[string]tmaTip) (string, string) {
	if metricDef.ThresholdEvaluable == nil {
		// no threshold defined
		return "No", ""
	}
	variables := make(map[string]any) // map of variable names to values
	// threshold variable names are legacy metric names, so find the corresponding metric definitions
	for _, v := range metricDef.ThresholdVariables {
		vDef := findMetricDefinitionByLegacyName(v, metricDefinitions)
		if vDef == nil {
			slog.Warn("threshold variable not found in metric definitions", slog.String("metric", metricDef.Name), slog.String("variable", v))
			return "No", ""
		}
		if stat, ok := stats[getMetricDisplayName(*vDef)]; ok {
			variables[v] = stat.mean
		} else {
			slog.Warn("threshold variable not found in stats", slog.String("metric", metricDef.Name), slog.String("variable", v))
			return "No", ""
		}
	}
	// evaluate the threshold expression
	result, err := evaluateThresholdExpression(metricDef.ThresholdEvaluable, variables)
	if err != nil {
		slog.Warn("failed to evaluate threshold expression", slog.String("metric", metricDef.Name), slog.String("expression", metricDef.ThresholdExpression), slog.String("error", err.Error()))
		return "No", ""
	}
	boolResult, ok := result.(bool)
	if !ok {
		slog.Warn("threshold expression did not evaluate to a boolean", slog.String("metric", metricDef.Name), slog.String("expression", metricDef.ThresholdExpression))
		return "No", ""
	}
	var exceeded string
	if boolResult {
		exceeded = "Yes"
	} else {
		exceeded = "No"
	}
	var resultTip string
	if exceeded == "Yes" {
		issueTip, ok := tmaTips[metricDef.Name]
		if ok {
			if issueTip.Issue != "" {
				resultTip = fmt.Sprintf("Issue: %s ", issueTip.Issue)
			}
			if issueTip.Tip != "" {
				resultTip += fmt.Sprintf("Tip: %s", issueTip.Tip)
			}
		}
		if resultTip == "" {
			// fallback if no tip found
			resultTip = "Value exceeds metric threshold: " + metricDef.ThresholdExpression + "."
		}
	}
	return exceeded, resultTip
}

// function to call evaluator so that we can catch panics that come from the evaluator
func evaluateThresholdExpression(evaluable *govaluate.EvaluableExpression, variables map[string]any) (any, error) {
	var err error
	defer func() {
		if errx := recover(); errx != nil {
			err = errx.(error)
		}
	}()
	result, err := evaluable.Evaluate(variables)
	return result, err
}

// getCSV - generate CSV string representing the summary statistics of the metrics
func (mg *MetricGroup) getCSV() (out string, err error) {
	var stats map[string]metricStats
	if stats, err = mg.getStats(); err != nil {
		return
	}
	out = "metric,mean,min,max,stddev\n"
	if mg.groupByField != "" {
		out = mg.groupByField + "," + out
	}
	for _, name := range mg.names {
		if mg.groupByValue == "" {
			out += fmt.Sprintf("%s,%f,%f,%f,%f\n", name, stats[name].mean, stats[name].min, stats[name].max, stats[name].stddev)
		} else {
			out += fmt.Sprintf("%s,%s,%f,%f,%f,%f\n", mg.groupByValue, name, stats[name].mean, stats[name].min, stats[name].max, stats[name].stddev)
		}
	}
	return
}

// getCSV - generate CSV string representing the summary statistics of multiple metricsFromCSV
// This is used when there are multiple metricsFromCSV objects, e.g., one per socket, cpu, or cgroup.
// Output format is:
//
//	metric,cpu0,cpu1,cpu2,cpu3
//	metric,val0,val1,val2,val3
//
// where metric is the name of the metric, and val0, val1, etc. are the values for each CPU/socket/cgroup.
func (mc MetricCollection) getCSV() (out string, err error) {
	if len(mc) == 0 {
		return "", fmt.Errorf("no metrics to summarize")
	}
	if len(mc) == 1 {
		// if there is only one metricsFromCSV, then it is system scope and granularity
		// so we can just use the first one
		return mc[0].getCSV()
	}
	// Validate groupByField for all metrics
	validGroupByFields := []string{"SKT", "CPU", "CID"}
	for i, m := range mc {
		if !slices.Contains(validGroupByFields, m.groupByField) {
			return "", fmt.Errorf("invalid groupByField in metrics[%d]: %s", i, m.groupByField)
		}
	}
	// first, get the names of the metrics
	metricNames := mc[0].names
	for idx, m := range mc[1:] {
		if !slices.Equal(m.names, metricNames) {
			return "", fmt.Errorf("metricsFromCSV objects have different metric names or order at index %d: %v vs %v", idx+1, m.names, metricNames)
		}
	}
	// write the header
	out = "metric"
	var fieldPrefix string
	switch mc[0].groupByField {
	case validGroupByFields[0]: // SKT
		fieldPrefix = validGroupByFields[0] // "SKT"
	case validGroupByFields[1]: // CPU
		fieldPrefix = validGroupByFields[1] // "CPU"
	case validGroupByFields[2]: // CID
		fieldPrefix = "" // leave empty for CID
	default:
		// shouldn't happen due to earlier validation
		return "", fmt.Errorf("invalid groupByField: %s", mc[0].groupByField)
	}
	for _, m := range mc {
		if m.groupByValue == "" {
			return "", fmt.Errorf("groupByValue is empty for metricsFromCSV with groupByField %s", m.groupByField)
		}
		// add the groupByValue to the header
		// e.g., SKT0, SKT1, CPU0, CPU1, etc.
		// if groupByField is CID, it will be empty
		out += "," + fieldPrefix + m.groupByValue
	}
	out += "\n"
	// get the stats for each metricsFromCSV
	allStats := make([]map[string]float64, len(mc))
	for i, m := range mc {
		allStats[i] = make(map[string]float64)
		stats, err := m.getStats()
		if err != nil {
			return "", fmt.Errorf("failed to get stats for metricsFromCSV %d: %w", i, err)
		}
		for name, stat := range stats {
			allStats[i][name] = stat.mean // use mean for the summary
		}
	}
	// write the metric names and values
	for _, name := range metricNames {
		out += name
		for j := range mc {
			out += fmt.Sprintf(",%f", allStats[j][name])
		}
		out += "\n"
	}
	return
}
