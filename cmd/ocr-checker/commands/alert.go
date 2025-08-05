// Package commands provides CLI command implementations for the OCR checker tool.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"chainlink-ocr-checker/domain/dto"
	"chainlink-ocr-checker/infrastructure/config"
	"chainlink-ocr-checker/infrastructure/notifier"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

// NewAlertCommand creates the alert command.
func NewAlertCommand(container *config.Container) *cobra.Command {
	var (
		outputFormat   string
		staleThreshold string
		webhookURL     string
		channel        string
		mentionUsers   []string
		dryRun         bool
		saveResult     string
	)
	
	cmd := &cobra.Command{
		Use:   "alert [transmitter] [rounds_to_check] [blocks_to_check]",
		Short: "Monitor transmitter and send alerts",
		Long: `Monitors transmitter activity across OCR2 jobs and sends alerts to Slack
when issues are detected. Compatible with existing shell script workflow.`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse arguments
			transmitterAddr := common.HexToAddress(args[0])
			
			roundsToCheck, err := parsePositiveInt(args[1])
			if err != nil {
				return fmt.Errorf("invalid rounds_to_check: %w", err)
			}
			
			blocksToCheck, err := parsePositiveInt(args[2])
			if err != nil {
				return fmt.Errorf("invalid blocks_to_check: %w", err)
			}
			
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
			
			// Create context
			ctx := context.Background()
			
			// Execute monitoring
			params := WatchParams{
				Transmitter:    transmitterAddr,
				RoundsToCheck:  roundsToCheck,
				BlocksToCheck:  blocksToCheck,
				StaleThreshold: staleThresholdDuration,
			}
			
			result, err := executeWatch(ctx, container, params)
			if err != nil {
				return fmt.Errorf("monitoring failed: %w", err)
			}
			
			// Convert to DTO
			monitoringResult := convertToMonitoringResult(result, transmitterAddr, container.Config.ChainID)
			
			// Save result if requested
			if saveResult != "" {
				if err := saveMonitoringResult(monitoringResult, saveResult); err != nil {
					container.Logger.Error("Failed to save result", "error", err)
				}
			}
			
			// Output result
			if err := outputMonitoringResult(monitoringResult, outputFormat); err != nil {
				return err
			}
			
			// Send alert if needed
			if monitoringResult.AlertRequired && !dryRun {
				if webhookURL == "" {
					container.Logger.Warn("Alert required but no webhook URL configured")
					return nil
				}
				
				// Create notifier
				slackNotifier := notifier.NewSlackNotifier(
					webhookURL,
					channel,
					mentionUsers,
					container.Logger,
				)
				
				// Send alert
				if err := slackNotifier.SendAlert(ctx, monitoringResult); err != nil {
					container.Logger.Error("Failed to send alert", "error", err)
					return fmt.Errorf("failed to send alert: %w", err)
				}
				
				container.Logger.Info("Alert sent successfully")
			}
			
			return nil
		},
	}
	
	// Add flags
	cmd.Flags().StringVarP(&outputFormat, "output", "o", OutputFormatJSON, "Output format (json, yaml, text)")
	cmd.Flags().StringVar(&staleThreshold, "stale-threshold", "24h", "Duration to consider job stale")
	cmd.Flags().StringVar(&webhookURL, "webhook", "", "Slack webhook URL (overrides SLACK_WEB_HOOK env)")
	cmd.Flags().StringVar(&channel, "channel", "", "Slack channel")
	cmd.Flags().StringSliceVar(&mentionUsers, "mention", nil, "Users to mention in alerts")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Check without sending alerts")
	cmd.Flags().StringVar(&saveResult, "save", "", "Save result to file")
	
	return cmd
}

// convertToMonitoringResult converts watch result to monitoring DTO.
func convertToMonitoringResult(result *WatchResult, transmitter common.Address, chainID int64) *dto.MonitoringResult {
	// Count jobs by status
	summary := dto.MonitoringSummary{
		TotalJobs:    len(result.Statuses),
		JobsByStatus: make(map[string]int),
	}
	
	// Convert job statuses
	jobs := make([]dto.JobMonitoringResult, 0, len(result.Statuses))
	for _, status := range result.Statuses {
		// Convert status
		var dtoStatus dto.JobStatus
		switch status.Status {
		case "Found":
			dtoStatus = dto.JobStatusFound
			summary.FoundJobs++
		case "Stale":
			dtoStatus = dto.JobStatusStale
			summary.StaleJobs++
		case "Missing":
			dtoStatus = dto.JobStatusMissing
			summary.MissingJobs++
		case "No Active":
			dtoStatus = dto.JobStatusNoActive
			summary.NoActiveJobs++
		case "Error":
			dtoStatus = dto.JobStatusError
			summary.ErrorJobs++
		}
		
		summary.JobsByStatus[string(dtoStatus)]++
		
		// Calculate time since last transmission
		var lastTimestamp *time.Time
		var timeSinceLastTx string
		if !status.LastTimestamp.IsZero() {
			lastTimestamp = &status.LastTimestamp
			duration := time.Since(status.LastTimestamp)
			timeSinceLastTx = formatDuration(duration)
		}
		
		// Build error message
		errorMsg := ""
		if status.Error != nil {
			errorMsg = status.Error.Error()
		}
		
		jobs = append(jobs, dto.JobMonitoringResult{
			JobID:           status.JobID,
			ContractAddress: status.ContractAddress,
			Status:          dtoStatus,
			LastRound:       status.LastRound,
			LastTimestamp:   lastTimestamp,
			TimeSinceLastTx: timeSinceLastTx,
			Error:           errorMsg,
		})
	}
	
	// Calculate health score
	healthScore := float64(summary.FoundJobs) / float64(summary.TotalJobs)
	if summary.TotalJobs == 0 {
		healthScore = 0
	}
	summary.HealthScore = healthScore
	
	// Determine overall status
	overallStatus := dto.StatusHealthy
	alertRequired := false
	alertMessage := ""
	
	if summary.ErrorJobs > 0 || summary.MissingJobs > 0 {
		overallStatus = dto.StatusCritical
		alertRequired = true
		alertMessage = fmt.Sprintf("Critical: %d missing, %d errors", summary.MissingJobs, summary.ErrorJobs)
	} else if summary.StaleJobs > 0 {
		overallStatus = dto.StatusWarning
		alertRequired = true
		alertMessage = fmt.Sprintf("Warning: %d stale jobs", summary.StaleJobs)
	}
	
	// Get chain name
	chainName := getChainName(chainID)
	
	return &dto.MonitoringResult{
		Timestamp:     time.Now(),
		Status:        overallStatus,
		Transmitter:   transmitter,
		Chain:         chainName,
		ChainID:       chainID,
		Jobs:          jobs,
		Summary:       summary,
		AlertRequired: alertRequired,
		AlertMessage:  alertMessage,
	}
}

// saveMonitoringResult saves the monitoring result to a file.
func saveMonitoringResult(result *dto.MonitoringResult, filename string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}
	
	return os.WriteFile(filename, data, 0644)
}

// outputMonitoringResult outputs the monitoring result in the specified format.
func outputMonitoringResult(result *dto.MonitoringResult, format string) error {
	switch format {
	case OutputFormatJSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
		
	case OutputFormatYAML:
		// TODO: Implement YAML output
		return fmt.Errorf("YAML output not yet implemented")
		
	case OutputFormatText:
		return outputMonitoringResultText(result)
		
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// outputMonitoringResultText outputs the result in human-readable text format.
func outputMonitoringResultText(result *dto.MonitoringResult) error {
	fmt.Printf("=== OCR Monitor Report ===\n")
	fmt.Printf("Time: %s\n", result.Timestamp.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Chain: %s (ID: %d)\n", result.Chain, result.ChainID)
	fmt.Printf("Transmitter: %s\n", result.Transmitter.Hex())
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Health Score: %.1f%%\n\n", result.Summary.HealthScore*100)
	
	fmt.Printf("Summary:\n")
	fmt.Printf("  Total Jobs: %d\n", result.Summary.TotalJobs)
	fmt.Printf("  🟢 Found: %d\n", result.Summary.FoundJobs)
	fmt.Printf("  🟡 Stale: %d\n", result.Summary.StaleJobs)
	fmt.Printf("  🔴 Missing: %d\n", result.Summary.MissingJobs)
	fmt.Printf("  🔒 No Active: %d\n", result.Summary.NoActiveJobs)
	fmt.Printf("  🚨 Error: %d\n\n", result.Summary.ErrorJobs)
	
	if len(result.Jobs) > 0 {
		fmt.Printf("Job Details:\n")
		for _, job := range result.Jobs {
			statusEmoji := getStatusEmoji(job.Status)
			fmt.Printf("  %s %s (%s)\n", statusEmoji, job.JobID, job.ContractAddress.Hex())
			fmt.Printf("     Status: %s\n", job.Status)
			if job.LastTimestamp != nil {
				fmt.Printf("     Last Seen: %s (%s ago)\n", 
					job.LastTimestamp.Format("15:04:05"),
					job.TimeSinceLastTx)
			}
			if job.Error != "" {
				fmt.Printf("     Error: %s\n", job.Error)
			}
		}
	}
	
	if result.AlertRequired {
		fmt.Printf("\n⚠️  Alert: %s\n", result.AlertMessage)
	}
	
	return nil
}

// getStatusEmoji returns an emoji for the status.
func getStatusEmoji(status dto.JobStatus) string {
	switch status {
	case dto.JobStatusFound:
		return "🟢"
	case dto.JobStatusStale:
		return "🟡"
	case dto.JobStatusMissing:
		return "🔴"
	case dto.JobStatusNoActive:
		return "🔒"
	case dto.JobStatusError:
		return "🚨"
	default:
		return "❓"
	}
}

// getChainName returns a friendly name for the chain.
func getChainName(chainID int64) string {
	switch chainID {
	case 1:
		return "Ethereum"
	case 137:
		return "Polygon"
	case 56:
		return "BSC"
	case 43114:
		return "Avalanche"
	case 42161:
		return "Arbitrum"
	case 10:
		return "Optimism"
	default:
		return fmt.Sprintf("Chain %d", chainID)
	}
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	} else {
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}