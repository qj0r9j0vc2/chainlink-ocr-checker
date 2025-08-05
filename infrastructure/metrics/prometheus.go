// Package metrics provides Prometheus metrics for monitoring.
package metrics

import (
	"fmt"
	"strings"

	"chainlink-ocr-checker/domain/dto"
	"chainlink-ocr-checker/domain/interfaces"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics.
type Metrics struct {
	// Job status metrics
	jobsTotal       *prometheus.GaugeVec
	jobsHealthy     *prometheus.GaugeVec
	jobsStale       *prometheus.GaugeVec
	jobsMissing     *prometheus.GaugeVec
	jobsError       *prometheus.GaugeVec
	jobsNoActive    *prometheus.GaugeVec
	
	// Health score metric
	healthScore     *prometheus.GaugeVec
	
	// Monitoring metrics
	lastCheckTime   *prometheus.GaugeVec
	checkDuration   prometheus.Histogram
	checkErrors     prometheus.Counter
	
	// Transmission metrics
	lastRoundNumber *prometheus.GaugeVec
	timeSinceLastTx *prometheus.GaugeVec
	
	// Alert metrics
	alertsSent      prometheus.Counter
	alertsFailed    prometheus.Counter
}

// NewMetrics creates a new Metrics instance.
func NewMetrics() *Metrics {
	return &Metrics{
		jobsTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ocr_checker_jobs_total",
				Help: "Total number of OCR jobs",
			},
			[]string{"transmitter", "chain", "chain_id"},
		),
		jobsHealthy: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ocr_checker_jobs_healthy",
				Help: "Number of healthy OCR jobs",
			},
			[]string{"transmitter", "chain", "chain_id"},
		),
		jobsStale: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ocr_checker_jobs_stale",
				Help: "Number of stale OCR jobs",
			},
			[]string{"transmitter", "chain", "chain_id"},
		),
		jobsMissing: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ocr_checker_jobs_missing",
				Help: "Number of missing OCR jobs",
			},
			[]string{"transmitter", "chain", "chain_id"},
		),
		jobsError: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ocr_checker_jobs_error",
				Help: "Number of OCR jobs with errors",
			},
			[]string{"transmitter", "chain", "chain_id"},
		),
		jobsNoActive: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ocr_checker_jobs_no_active",
				Help: "Number of OCR jobs with no active status",
			},
			[]string{"transmitter", "chain", "chain_id"},
		),
		healthScore: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ocr_checker_health_score",
				Help: "Overall health score (0-1)",
			},
			[]string{"transmitter", "chain", "chain_id"},
		),
		lastCheckTime: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ocr_checker_last_check_timestamp",
				Help: "Timestamp of the last check",
			},
			[]string{"transmitter", "chain", "chain_id"},
		),
		checkDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "ocr_checker_check_duration_seconds",
				Help:    "Duration of monitoring checks",
				Buckets: prometheus.DefBuckets,
			},
		),
		checkErrors: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "ocr_checker_check_errors_total",
				Help: "Total number of check errors",
			},
		),
		lastRoundNumber: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ocr_checker_last_round_number",
				Help: "Last round number for each job",
			},
			[]string{"transmitter", "chain", "chain_id", "job_id", "contract"},
		),
		timeSinceLastTx: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ocr_checker_time_since_last_tx_seconds",
				Help: "Time since last transmission in seconds",
			},
			[]string{"transmitter", "chain", "chain_id", "job_id", "contract"},
		),
		alertsSent: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "ocr_checker_alerts_sent_total",
				Help: "Total number of alerts sent",
			},
		),
		alertsFailed: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "ocr_checker_alerts_failed_total",
				Help: "Total number of failed alerts",
			},
		),
	}
}

// UpdateFromResult updates metrics from a monitoring result.
func (m *Metrics) UpdateFromResult(result *dto.MonitoringResult) {
	labels := prometheus.Labels{
		"transmitter": strings.ToLower(result.Transmitter.Hex()),
		"chain":       result.Chain,
		"chain_id":    fmt.Sprintf("%d", result.ChainID),
	}

	// Update job counts
	m.jobsTotal.With(labels).Set(float64(result.Summary.TotalJobs))
	m.jobsHealthy.With(labels).Set(float64(result.Summary.FoundJobs))
	m.jobsStale.With(labels).Set(float64(result.Summary.StaleJobs))
	m.jobsMissing.With(labels).Set(float64(result.Summary.MissingJobs))
	m.jobsError.With(labels).Set(float64(result.Summary.ErrorJobs))
	m.jobsNoActive.With(labels).Set(float64(result.Summary.NoActiveJobs))
	
	// Update health score
	m.healthScore.With(labels).Set(result.Summary.HealthScore)
	
	// Update check time
	m.lastCheckTime.With(labels).Set(float64(result.Timestamp.Unix()))
	
	// Update per-job metrics
	for _, job := range result.Jobs {
		jobLabels := prometheus.Labels{
			"transmitter": strings.ToLower(result.Transmitter.Hex()),
			"chain":       result.Chain,
			"chain_id":    fmt.Sprintf("%d", result.ChainID),
			"job_id":      job.JobID,
			"contract":    strings.ToLower(job.ContractAddress.Hex()),
		}
		
		m.lastRoundNumber.With(jobLabels).Set(float64(job.LastRound))
		
		// Calculate time since last transmission
		if job.LastTimestamp != nil {
			timeSince := result.Timestamp.Sub(*job.LastTimestamp).Seconds()
			m.timeSinceLastTx.With(jobLabels).Set(timeSince)
		}
	}
}

// RecordCheckDuration records the duration of a monitoring check.
func (m *Metrics) RecordCheckDuration(seconds float64) {
	m.checkDuration.Observe(seconds)
}

// IncrementCheckErrors increments the check error counter.
func (m *Metrics) IncrementCheckErrors() {
	m.checkErrors.Inc()
}

// IncrementAlertsSent increments the alerts sent counter.
func (m *Metrics) IncrementAlertsSent() {
	m.alertsSent.Inc()
}

// IncrementAlertsFailed increments the alerts failed counter.
func (m *Metrics) IncrementAlertsFailed() {
	m.alertsFailed.Inc()
}

// Exporter provides a metrics exporter service.
type Exporter struct {
	metrics *Metrics
	logger  interfaces.Logger
}

// NewExporter creates a new metrics exporter.
func NewExporter(logger interfaces.Logger) *Exporter {
	return &Exporter{
		metrics: NewMetrics(),
		logger:  logger,
	}
}

// UpdateFromMonitoringResult updates metrics from a monitoring result.
func (e *Exporter) UpdateFromMonitoringResult(result *dto.MonitoringResult) {
	e.metrics.UpdateFromResult(result)
}

// RecordCheckDuration records the duration of a monitoring check.
func (e *Exporter) RecordCheckDuration(seconds float64) {
	e.metrics.RecordCheckDuration(seconds)
}

// IncrementCheckErrors increments the check error counter.
func (e *Exporter) IncrementCheckErrors() {
	e.metrics.IncrementCheckErrors()
}

// IncrementAlertsSent increments the alerts sent counter.
func (e *Exporter) IncrementAlertsSent() {
	e.metrics.IncrementAlertsSent()
}

// IncrementAlertsFailed increments the alerts failed counter.
func (e *Exporter) IncrementAlertsFailed() {
	e.metrics.IncrementAlertsFailed()
}