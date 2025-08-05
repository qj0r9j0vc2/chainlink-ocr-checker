// Package interfaces defines the contracts for domain services.
package interfaces

import (
	"context"

	"chainlink-ocr-checker/domain/dto"
)

// Notifier sends notifications to external services.
type Notifier interface {
	// SendAlert sends an alert notification.
	SendAlert(ctx context.Context, result *dto.MonitoringResult) error
	
	// SendSlackMessage sends a custom Slack message.
	SendSlackMessage(ctx context.Context, message *dto.SlackMessage) error
	
	// IsConfigured checks if the notifier is properly configured.
	IsConfigured() bool
}