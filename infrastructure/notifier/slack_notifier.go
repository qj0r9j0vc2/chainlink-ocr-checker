// Package notifier provides notification service implementations.
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"chainlink-ocr-checker/domain/dto"
	"chainlink-ocr-checker/domain/interfaces"
)

// slackNotifier implements the Notifier interface for Slack.
type slackNotifier struct {
	webhookURL   string
	channel      string
	mentionUsers []string
	logger       interfaces.Logger
	httpClient   *http.Client
}

// NewSlackNotifier creates a new Slack notifier.
func NewSlackNotifier(
	webhookURL string,
	channel string,
	mentionUsers []string,
	logger interfaces.Logger,
) interfaces.Notifier {
	return &slackNotifier{
		webhookURL:   webhookURL,
		channel:      channel,
		mentionUsers: mentionUsers,
		logger:       logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendAlert sends a monitoring alert to Slack.
func (n *slackNotifier) SendAlert(ctx context.Context, result *dto.MonitoringResult) error {
	if !n.IsConfigured() {
		return fmt.Errorf("slack notifier not configured")
	}

	message := n.buildAlertMessage(result)
	return n.SendSlackMessage(ctx, message)
}

// SendSlackMessage sends a message to Slack.
func (n *slackNotifier) SendSlackMessage(ctx context.Context, message *dto.SlackMessage) error {
	if !n.IsConfigured() {
		return fmt.Errorf("slack webhook URL not configured")
	}

	// Override channel if configured
	if n.channel != "" && message.Channel == "" {
		message.Channel = n.channel
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	n.logger.Debug("Sending Slack message", "payload", string(payload))

	req, err := http.NewRequestWithContext(ctx, "POST", n.webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API returned status %d", resp.StatusCode)
	}

	n.logger.Info("Alert sent to Slack successfully")
	return nil
}

// IsConfigured checks if the notifier is properly configured.
func (n *slackNotifier) IsConfigured() bool {
	return n.webhookURL != ""
}

// buildAlertMessage constructs a Slack message from monitoring result.
func (n *slackNotifier) buildAlertMessage(result *dto.MonitoringResult) *dto.SlackMessage {
	// Build mention string
	mentions := ""
	if len(n.mentionUsers) > 0 {
		mentionList := make([]string, len(n.mentionUsers))
		for i, user := range n.mentionUsers {
			if strings.HasPrefix(user, "@") {
				mentionList[i] = user
			} else {
				mentionList[i] = "<@" + user + ">"
			}
		}
		mentions = strings.Join(mentionList, " ") + " "
	}

	// Determine color based on status
	color := "#36a64f" // green
	emoji := "🟢"
	if result.Status == dto.StatusWarning {
		color = "#ff9900" // orange
		emoji = "🟡"
	} else if result.Status == dto.StatusCritical {
		color = "#ff0000" // red
		emoji = "🔴"
	}

	// Build title
	title := fmt.Sprintf("%s Chainlink OCR Monitor Alert %s", emoji, result.Chain)
	if mentions != "" {
		title = mentions + title
	}

	// Build fields
	fields := []dto.SlackField{
		{
			Title: "Status",
			Value: string(result.Status),
			Short: true,
		},
		{
			Title: "Transmitter",
			Value: result.Transmitter.Hex(),
			Short: true,
		},
		{
			Title: "Total Jobs",
			Value: fmt.Sprintf("%d", result.Summary.TotalJobs),
			Short: true,
		},
		{
			Title: "Health Score",
			Value: fmt.Sprintf("%.1f%%", result.Summary.HealthScore*100),
			Short: true,
		},
	}

	// Add job summary
	if result.Summary.FoundJobs > 0 {
		fields = append(fields, dto.SlackField{
			Title: "🟢 Found",
			Value: fmt.Sprintf("%d", result.Summary.FoundJobs),
			Short: true,
		})
	}
	if result.Summary.StaleJobs > 0 {
		fields = append(fields, dto.SlackField{
			Title: "🟡 Stale",
			Value: fmt.Sprintf("%d", result.Summary.StaleJobs),
			Short: true,
		})
	}
	if result.Summary.MissingJobs > 0 {
		fields = append(fields, dto.SlackField{
			Title: "🔴 Missing",
			Value: fmt.Sprintf("%d", result.Summary.MissingJobs),
			Short: true,
		})
	}
	if result.Summary.ErrorJobs > 0 {
		fields = append(fields, dto.SlackField{
			Title: "🚨 Error",
			Value: fmt.Sprintf("%d", result.Summary.ErrorJobs),
			Short: true,
		})
	}

	// Build job details
	var jobDetails []string
	for _, job := range result.Jobs {
		if job.Status != dto.JobStatusFound {
			statusEmoji := n.getStatusEmoji(job.Status)
			detail := fmt.Sprintf("%s %s: %s", statusEmoji, job.JobID, job.Status)
			if job.Error != "" {
				detail += fmt.Sprintf(" (%s)", job.Error)
			}
			if job.TimeSinceLastTx != "" {
				detail += fmt.Sprintf(" - Last: %s ago", job.TimeSinceLastTx)
			}
			jobDetails = append(jobDetails, detail)
		}
	}

	text := ""
	if len(jobDetails) > 0 {
		text = "Job Details:\n```\n" + strings.Join(jobDetails, "\n") + "\n```"
	}

	// Build attachment
	attachment := dto.SlackAttachment{
		Color:     color,
		Title:     title,
		Text:      text,
		Fields:    fields,
		Footer:    "OCR Checker",
		Timestamp: result.Timestamp.Unix(),
	}

	// Build message
	message := &dto.SlackMessage{
		Text:        result.AlertMessage,
		Attachments: []dto.SlackAttachment{attachment},
		Username:    "OCR Monitor",
		IconEmoji:   ":robot_face:",
	}

	return message
}

// getStatusEmoji returns an emoji for the given status.
func (n *slackNotifier) getStatusEmoji(status dto.JobStatus) string {
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