// Copyright 2025 Google LLC.
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const promMetricPrefix = "perfspect_"

var prometheusMetricsGaugeVec = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "perfspect_metrics",
		Help: "PerfSpect metrics",
	},
	[]string{"metric_name", "socket", "cpu", "cgroup", "pid", "cmd"},
)
var rxTrailingChars = regexp.MustCompile(`\)$`)
var promMetrics = make(map[string]*prometheus.GaugeVec)
var promMetricsMutex sync.RWMutex

// addPrometheusMetrics safely adds metrics to the global prometheus map
func addPrometheusMetrics(newMetrics map[string]*prometheus.GaugeVec) {
	promMetricsMutex.Lock()
	defer promMetricsMutex.Unlock()

	for name, gauge := range newMetrics {
		if _, exists := promMetrics[name]; !exists {
			promMetrics[name] = gauge
			if err := prometheus.Register(gauge); err != nil {
				if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
					// Use the already registered metric
					promMetrics[name] = are.ExistingCollector.(*prometheus.GaugeVec)
				} else {
					// Log unexpected registration error
					slog.Error("Failed to register Prometheus metric", slog.String("name", name), slog.String("error", err.Error()))
				}
			}
		}
	}
}

// createPrometheusMetrics creates Prometheus gauge vectors from metric definitions
func createPrometheusMetrics(metricDefinitions []MetricDefinition) {
	localPromMetrics := make(map[string]*prometheus.GaugeVec)

	for _, def := range metricDefinitions {
		desc := fmt.Sprintf("%s (expr: %s)", def.Name, def.Expression)
		name := promMetricPrefix + sanitizeMetricName(def.Name)
		gauge := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: name,
				Help: desc,
			},
			[]string{"socket", "cpu", "cgroup", "pid", "cmd"},
		)
		localPromMetrics[name] = gauge
	}

	// Add to global map with mutex protection
	addPrometheusMetrics(localPromMetrics)
}

func sanitizeMetricName(name string) string {
	sanitized := rxTrailingChars.ReplaceAllString(name, "")
	sanitized = strings.ReplaceAll(sanitized, "%", "pct")
	return sanitized
}
func startPrometheusServer(listenAddr string) {
	prometheus.MustRegister(prometheusMetricsGaugeVec)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	slog.Info("Starting Prometheus metrics server", slog.String("address", listenAddr))
	go func() {
		server := &http.Server{
			Addr:              listenAddr,
			Handler:           mux,
			ReadHeaderTimeout: 3 * time.Second,
		}
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			slog.Error("Prometheus HTTP server ListenAndServe error", slog.String("error", err.Error()))
		}
	}()
}

func updatePrometheusMetrics(metricFrames []MetricFrame) {
	promMetricsMutex.RLock()
	defer promMetricsMutex.RUnlock()

	for _, frame := range metricFrames {
		for _, metric := range frame.Metrics {
			if !math.IsNaN(metric.Value) {
				metricKey := promMetricPrefix + sanitizeMetricName(metric.Name)
				if m, ok := promMetrics[metricKey]; ok {
					m.WithLabelValues(
						frame.Socket,
						frame.CPU,
						frame.Cgroup,
						frame.PID,
						frame.Cmd,
					).Set(metric.Value)
				} else {
					slog.Warn("Unable to find metric", slog.String("metric", metricKey))
				}
			}
		}
	}
}
