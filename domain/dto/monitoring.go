// Package dto contains data transfer objects for API responses and monitoring.
package dto

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// MonitoringStatus represents the overall monitoring status.
type MonitoringStatus string

const (
	// StatusHealthy indicates all jobs are functioning normally.
	StatusHealthy MonitoringStatus = "healthy"
	// StatusWarning indicates some jobs have issues.
	StatusWarning MonitoringStatus = "warning"
	// StatusCritical indicates critical issues detected.
	StatusCritical MonitoringStatus = "critical"
)

// JobStatus represents the status of a single job.
type JobStatus string

const (
	// JobStatusFound indicates the job is active and transmitting.
	JobStatusFound JobStatus = "found"
	// JobStatusStale indicates the job hasn't transmitted recently.
	JobStatusStale JobStatus = "stale"
	// JobStatusMissing indicates no transmissions found.
	JobStatusMissing JobStatus = "missing"
	// JobStatusNoActive indicates no active job found.
	JobStatusNoActive JobStatus = "no_active"
	// JobStatusError indicates an error occurred.
	JobStatusError JobStatus = "error"
)

// MonitoringResult represents the complete monitoring result.
type MonitoringResult struct {
	Timestamp       time.Time              `json:"timestamp"`
	Status          MonitoringStatus       `json:"status"`
	Transmitter     common.Address         `json:"transmitter"`
	Chain           string                 `json:"chain"`
	ChainID         int64                  `json:"chain_id"`
	Jobs            []JobMonitoringResult  `json:"jobs"`
	Summary         MonitoringSummary      `json:"summary"`
	AlertRequired   bool                   `json:"alert_required"`
	AlertMessage    string                 `json:"alert_message,omitempty"`
}

// JobMonitoringResult represents monitoring result for a single job.
type JobMonitoringResult struct {
	JobID           string         `json:"job_id"`
	ContractAddress common.Address `json:"contract_address"`
	Status          JobStatus      `json:"status"`
	LastRound       uint32         `json:"last_round"`
	LastTimestamp   *time.Time     `json:"last_timestamp,omitempty"`
	TimeSinceLastTx string         `json:"time_since_last_tx,omitempty"`
	Error           string         `json:"error,omitempty"`
}

// MonitoringSummary provides summary statistics.
type MonitoringSummary struct {
	TotalJobs     int            `json:"total_jobs"`
	FoundJobs     int            `json:"found_jobs"`
	StaleJobs     int            `json:"stale_jobs"`
	MissingJobs   int            `json:"missing_jobs"`
	NoActiveJobs  int            `json:"no_active_jobs"`
	ErrorJobs     int            `json:"error_jobs"`
	HealthScore   float64        `json:"health_score"`
	JobsByStatus  map[string]int `json:"jobs_by_status"`
}

// AlertConfig defines alert configuration.
type AlertConfig struct {
	Enabled          bool          `json:"enabled"`
	WebhookURL       string        `json:"webhook_url,omitempty"`
	Channel          string        `json:"channel,omitempty"`
	MentionUsers     []string      `json:"mention_users,omitempty"`
	StaleThreshold   time.Duration `json:"stale_threshold"`
	AlertOnStale     bool          `json:"alert_on_stale"`
	AlertOnMissing   bool          `json:"alert_on_missing"`
	AlertOnError     bool          `json:"alert_on_error"`
}

// PrometheusMetrics represents metrics for Prometheus export.
type PrometheusMetrics struct {
	JobsTotal         int                       `json:"jobs_total"`
	JobsHealthy       int                       `json:"jobs_healthy"`
	JobsStale         int                       `json:"jobs_stale"`
	JobsMissing       int                       `json:"jobs_missing"`
	JobsError         int                       `json:"jobs_error"`
	LastCheckTime     time.Time                 `json:"last_check_time"`
	TransmitterLabels map[string]string         `json:"transmitter_labels"`
}

// SlackMessage represents a Slack notification message.
type SlackMessage struct {
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
}

// SlackAttachment represents a Slack message attachment.
type SlackAttachment struct {
	Color      string       `json:"color"`
	Title      string       `json:"title"`
	Text       string       `json:"text,omitempty"`
	Fields     []SlackField `json:"fields,omitempty"`
	Footer     string       `json:"footer,omitempty"`
	Timestamp  int64        `json:"ts,omitempty"`
}

// SlackField represents a field in Slack attachment.
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// TransmissionReport represents a detailed transmission report.
type TransmissionReport struct {
	ContractAddress  common.Address    `json:"contract_address"`
	StartRound       uint32            `json:"start_round"`
	EndRound         uint32            `json:"end_round"`
	TotalRounds      int               `json:"total_rounds"`
	Transmissions    []Transmission    `json:"transmissions"`
	ObserverActivity []ObserverReport `json:"observer_activity"`
}

// Transmission represents a single transmission event.
type Transmission struct {
	Round         uint32         `json:"round"`
	Epoch         uint32         `json:"epoch"`
	Timestamp     time.Time      `json:"timestamp"`
	Transmitter   common.Address `json:"transmitter"`
	ObserverCount int            `json:"observer_count"`
	Observers     []int          `json:"observers"`
}

// ObserverReport represents observer activity statistics.
type ObserverReport struct {
	ObserverIndex int            `json:"observer_index"`
	Address       common.Address `json:"address"`
	TotalCount    int            `json:"total_count"`
	Percentage    float64        `json:"percentage"`
}