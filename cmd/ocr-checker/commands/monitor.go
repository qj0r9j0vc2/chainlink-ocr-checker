// Package commands provides CLI command implementations for the OCR checker tool.
package commands

import (
	"context"
	"fmt"
	"time"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/infrastructure/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

// NewMonitorCommand creates the monitor command to watch a specific contract and transmitter.
func NewMonitorCommand(container *config.Container) *cobra.Command {
	var (
		outputFormat string
		blockRange   int
		interval     int
	)

	cmd := &cobra.Command{
		Use:   "monitor [contract] [transmitter]",
		Short: "Monitor transmitter activity on a specific OCR contract",
		Long:  `Monitors a specific transmitter's participation in an OCR contract by checking recent transmissions.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			// Parse arguments
			contractAddr := common.HexToAddress(args[0])
			transmitterAddr := common.HexToAddress(args[1])

			// Create context
			ctx := context.Background()

			container.Logger.Info("Monitoring transmitter activity",
				"contract", contractAddr.Hex(),
				"transmitter", transmitterAddr.Hex(),
				"blockRange", blockRange)

			// Get current block
			currentBlock, err := container.BlockchainClient.GetBlockNumber(ctx)
			if err != nil {
				return fmt.Errorf("failed to get current block: %w", err)
			}

			// Calculate block range
			startBlock := currentBlock - uint64(blockRange)
			if startBlock > currentBlock { // Overflow check
				startBlock = 0
			}

			// Monitor loop
			for {
				// Fetch recent transmissions
				transmissions, err := container.OCR2AggregatorService.GetTransmissions(
					ctx, contractAddr, startBlock, currentBlock)
				if err != nil {
					container.Logger.Error("Failed to get transmissions", "error", err)
					if interval == 0 {
						return fmt.Errorf("failed to get transmissions: %w", err)
					}
					// Continue monitoring if interval is set
					time.Sleep(time.Duration(interval) * time.Second)
					continue
				}

				// Analyze transmitter activity
				transmitterStats := analyzeTransmitterActivity(transmissions, transmitterAddr)

				// Display results
				result := map[string]interface{}{
					"contract":              contractAddr.Hex(),
					"transmitter":           transmitterAddr.Hex(),
					"currentBlock":          currentBlock,
					"blockRange":            fmt.Sprintf("%d-%d", startBlock, currentBlock),
					"totalTransmissions":    len(transmissions),
					"transmitterStats":      transmitterStats,
					"lastChecked":           time.Now().Format(time.RFC3339),
				}

				// Validate output format
				if err := ValidateFormat(outputFormat); err != nil {
					return err
				}

				// Display results using output formatter
				formatter := NewOutputFormatter(outputFormat)
				if err := formatter.Print(result); err != nil {
					return err
				}

				// Exit if not continuous monitoring
				if interval == 0 {
					break
				}

				// Wait for next iteration
				container.Logger.Info("Waiting for next check",
					"interval", interval)
				time.Sleep(time.Duration(interval) * time.Second)

				// Update block numbers for next iteration
				currentBlock, err = container.BlockchainClient.GetBlockNumber(ctx)
				if err != nil {
					container.Logger.Error("Failed to get current block", "error", err)
					continue
				}
				startBlock = currentBlock - uint64(blockRange)
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "json", "Output format (json, yaml)")
	cmd.Flags().IntVarP(&blockRange, "blocks", "b", 10000, "Number of blocks to check")
	cmd.Flags().IntVarP(&interval, "interval", "i", 0, "Check interval in seconds (0 for one-time check)")

	return cmd
}

// analyzeTransmitterActivity analyzes transmitter participation in transmissions.
func analyzeTransmitterActivity(transmissions []entities.Transmission, transmitterAddr common.Address) map[string]interface{} {
	transmitterCount := 0
	totalRounds := len(transmissions)
	lastTransmissionBlock := uint64(0)
	lastTransmissionTime := time.Time{}

	// Count transmissions by this transmitter
	for _, tx := range transmissions {
		if tx.TransmitterAddress == transmitterAddr {
			transmitterCount++
			if tx.BlockNumber > lastTransmissionBlock {
				lastTransmissionBlock = tx.BlockNumber
			}
			if tx.BlockTimestamp.After(lastTransmissionTime) {
				lastTransmissionTime = tx.BlockTimestamp
			}
		}
	}

	participationRate := float64(0)
	if totalRounds > 0 {
		participationRate = float64(transmitterCount) / float64(totalRounds) * 100
	}

	stats := map[string]interface{}{
		"transmissionCount":     transmitterCount,
		"totalRounds":           totalRounds,
		"participationRate":     fmt.Sprintf("%.2f%%", participationRate),
		"lastTransmissionBlock": lastTransmissionBlock,
	}

	if !lastTransmissionTime.IsZero() {
		stats["lastTransmissionTime"] = lastTransmissionTime.Format(time.RFC3339)
		stats["timeSinceLastTransmission"] = time.Since(lastTransmissionTime).String()
	}

	return stats
}