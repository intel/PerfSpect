package metrics

// Copyright 2025 Google LLC.
// SPDX-License-Identifier: BSD-3-Clause

import (
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strings"
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
