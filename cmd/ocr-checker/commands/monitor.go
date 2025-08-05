// Package commands provides CLI command implementations for the OCR checker tool.
package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chainlink-ocr-checker/domain/interfaces"
	"chainlink-ocr-checker/infrastructure/config"
	"chainlink-ocr-checker/infrastructure/metrics"
	"chainlink-ocr-checker/infrastructure/notifier"
	"github.com/ethereum/go-ethereum/common"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

// NewMonitorCommand creates the monitor command.
func NewMonitorCommand(container *config.Container) *cobra.Command {
	var (
		port           int
		interval       string
		transmitters   []string
		staleThreshold string
		webhookURL     string
		channel        string
		mentionUsers   []string
		metricsPath    string
	)
	
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Run continuous monitoring with Prometheus metrics",
		Long: `Runs a monitoring server that continuously checks transmitter activity
and exposes Prometheus metrics. Can send alerts to Slack based on conditions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse stale threshold
			staleThresholdDuration := 24 * time.Hour
			if staleThreshold != "" {
				duration, err := time.ParseDuration(staleThreshold)
				if err != nil {
					return fmt.Errorf("invalid stale threshold: %w", err)
				}
				staleThresholdDuration = duration
			}
			
			// Override webhook URL from environment if not provided
			if webhookURL == "" {
				webhookURL = os.Getenv("SLACK_WEB_HOOK")
			}
			
			// Parse transmitter addresses
			var transmitterAddrs []common.Address
			for _, addr := range transmitters {
				transmitterAddrs = append(transmitterAddrs, common.HexToAddress(addr))
			}
			
			// Create notifier
			var slackNotifier interfaces.Notifier
			if webhookURL != "" {
				slackNotifier = notifier.NewSlackNotifier(
					webhookURL,
					channel,
					mentionUsers,
					container.Logger,
				)
			}
			
			// Create metrics
			promMetrics := metrics.NewMetrics()
			
			// Create monitor
			monitor := &continuousMonitor{
				container:       container,
				transmitters:    transmitterAddrs,
				staleThreshold:  staleThresholdDuration,
				notifier:        slackNotifier,
				metrics:         promMetrics,
				logger:          container.Logger,
			}
			
			// Setup HTTP server for metrics
			mux := http.NewServeMux()
			mux.Handle(metricsPath, promhttp.Handler())
			mux.HandleFunc("/health", healthHandler)
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`<html>
<head><title>OCR Checker</title></head>
<body>
<h1>OCR Checker</h1>
<p><a href='` + metricsPath + `'>Metrics</a></p>
</body>
</html>`))
			})
			
			server := &http.Server{
				Addr:    fmt.Sprintf(":%d", port),
				Handler: mux,
			}
			
			// Start server
			go func() {
				container.Logger.Info("Starting metrics server", "port", port, "path", metricsPath)
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					container.Logger.Error("Metrics server error", "error", err)
				}
			}()
			
			// Setup cron scheduler
			c := cron.New()
			_, err := c.AddFunc(interval, func() {
				monitor.runCheck(context.Background())
			})
			if err != nil {
				return fmt.Errorf("invalid interval: %w", err)
			}
			
			// Start scheduler
			c.Start()
			container.Logger.Info("Monitor started", "interval", interval, "transmitters", len(transmitterAddrs))
			
			// Run initial check
			monitor.runCheck(context.Background())
			
			// Wait for interrupt
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			<-sigChan
			
			// Shutdown
			container.Logger.Info("Shutting down monitor...")
			c.Stop()
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			if err := server.Shutdown(ctx); err != nil {
				container.Logger.Error("Server shutdown error", "error", err)
			}
			
			return nil
		},
	}
	
	// Add flags
	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Metrics server port")
	cmd.Flags().StringVar(&interval, "interval", "@every 5m", "Check interval (cron format)")
	cmd.Flags().StringSliceVar(&transmitters, "transmitters", nil, "Transmitter addresses to monitor")
	cmd.Flags().StringVar(&staleThreshold, "stale-threshold", "24h", "Duration to consider job stale")
	cmd.Flags().StringVar(&webhookURL, "webhook", "", "Slack webhook URL")
	cmd.Flags().StringVar(&channel, "channel", "", "Slack channel")
	cmd.Flags().StringSliceVar(&mentionUsers, "mention", nil, "Users to mention in alerts")
	cmd.Flags().StringVar(&metricsPath, "metrics-path", "/metrics", "Path to expose metrics")
	
	return cmd
}

// continuousMonitor runs continuous monitoring checks.
type continuousMonitor struct {
	container      *config.Container
	transmitters   []common.Address
	staleThreshold time.Duration
	notifier       interfaces.Notifier
	metrics        *metrics.Metrics
	logger         interfaces.Logger
}

// runCheck performs a monitoring check for all transmitters.
func (m *continuousMonitor) runCheck(ctx context.Context) {
	m.logger.Info("Running monitoring check", "transmitters", len(m.transmitters))
	startTime := time.Now()
	
	for _, transmitter := range m.transmitters {
		if err := m.checkTransmitter(ctx, transmitter); err != nil {
			m.logger.Error("Check failed for transmitter", 
				"transmitter", transmitter.Hex(),
				"error", err)
			m.metrics.IncrementCheckErrors()
		}
	}
	
	duration := time.Since(startTime).Seconds()
	m.metrics.RecordCheckDuration(duration)
	m.logger.Info("Monitoring check completed", "duration", duration)
}

// checkTransmitter checks a single transmitter.
func (m *continuousMonitor) checkTransmitter(ctx context.Context, transmitter common.Address) error {
	// Execute watch
	params := WatchParams{
		Transmitter:    transmitter,
		RoundsToCheck:  100,  // Default
		BlocksToCheck:  10000, // Default
		StaleThreshold: m.staleThreshold,
	}
	
	result, err := executeWatch(ctx, m.container, params)
	if err != nil {
		return fmt.Errorf("watch failed: %w", err)
	}
	
	// Convert to monitoring result
	monitoringResult := convertToMonitoringResult(result, transmitter, m.container.Config.ChainID)
	
	// Update metrics
	m.metrics.UpdateFromResult(monitoringResult)
	
	// Send alert if needed
	if monitoringResult.AlertRequired && m.notifier != nil && m.notifier.IsConfigured() {
		if err := m.notifier.SendAlert(ctx, monitoringResult); err != nil {
			m.logger.Error("Failed to send alert", "error", err)
			m.metrics.IncrementAlertsFailed()
		} else {
			m.logger.Info("Alert sent", "transmitter", transmitter.Hex())
			m.metrics.IncrementAlertsSent()
		}
	}
	
	return nil
}

// healthHandler handles health check requests.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// WatchParams holds parameters for watch execution.
type WatchParams struct {
	Transmitter    common.Address
	RoundsToCheck  int
	BlocksToCheck  int
	StaleThreshold time.Duration
}

// WatchResult represents the result of a watch operation.
type WatchResult struct {
	Transmitter common.Address
	Statuses    []JobStatus
	Summary     WatchSummary
}

// JobStatus represents the status of a job.
type JobStatus struct {
	JobID           string
	ContractAddress common.Address
	Status          string
	LastRound       uint32
	LastTimestamp   time.Time
	Error           error
}

// WatchSummary provides summary of watch results.
type WatchSummary struct {
	TotalJobs    int
	FoundJobs    int
	StaleJobs    int
	MissingJobs  int
	NoActiveJobs int
	ErrorJobs    int
}

// executeWatch executes the watch operation.
func executeWatch(ctx context.Context, container *config.Container, params WatchParams) (*WatchResult, error) {
	// Check if database is configured
	if container.WatchTransmittersUseCase == nil {
		return nil, fmt.Errorf("database configuration required")
	}
	
	// Convert to use case params
	ucParams := interfaces.WatchTransmittersParams{
		TransmitterAddress: params.Transmitter,
		RoundsToCheck:      params.RoundsToCheck,
		DaysToIgnore:       int(params.StaleThreshold.Hours() / 24),
	}
	
	// Execute use case
	ucResult, err := container.WatchTransmittersUseCase.Execute(ctx, ucParams)
	if err != nil {
		return nil, fmt.Errorf("watch use case failed: %w", err)
	}
	
	// Convert result
	statuses := make([]JobStatus, 0, len(ucResult.Statuses))
	for _, s := range ucResult.Statuses {
		statuses = append(statuses, JobStatus{
			JobID:           s.JobID,
			ContractAddress: s.ContractAddress,
			Status:          string(s.Status),
			LastRound:       s.LastRound,
			LastTimestamp:   s.LastTimestamp,
			Error:           s.Error,
		})
	}
	
	return &WatchResult{
		Transmitter: params.Transmitter,
		Statuses:    statuses,
		Summary: WatchSummary{
			TotalJobs:    ucResult.Summary.TotalJobs,
			FoundJobs:    ucResult.Summary.FoundJobs,
			StaleJobs:    ucResult.Summary.StaleJobs,
			MissingJobs:  ucResult.Summary.MissingJobs,
			NoActiveJobs: ucResult.Summary.NoActiveJobs,
			ErrorJobs:    ucResult.Summary.ErrorJobs,
		},
	}, nil
}