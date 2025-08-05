// Package commands provides CLI command implementations for the OCR checker tool.
package commands

import (
	"context"
	"fmt"

	"chainlink-ocr-checker/infrastructure/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

// NewInfoCommand creates the info command to get contract information.
func NewInfoCommand(container *config.Container) *cobra.Command {
	var (
		outputFormat string
		blockRange   int
	)

	cmd := &cobra.Command{
		Use:   "info [contract]",
		Short: "Get OCR contract information including recent rounds",
		Long:  `Fetches recent transmission data to show the current state and round information for an OCR contract.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			// Parse arguments
			contractAddr := common.HexToAddress(args[0])

			// Create context
			ctx := context.Background()

			// Get current block
			currentBlock, err := container.BlockchainClient.GetBlockNumber(ctx)
			if err != nil {
				return fmt.Errorf("failed to get current block: %w", err)
			}

			container.Logger.Info("Getting contract info",
				"contract", contractAddr.Hex(),
				"currentBlock", currentBlock)

			// Calculate block range to check
			startBlock := currentBlock - uint64(blockRange)
			if startBlock > currentBlock { // Overflow check
				startBlock = 0
			}

			// Fetch recent transmissions
			transmissions, err := container.OCR2AggregatorService.GetTransmissions(
				ctx, contractAddr, startBlock, currentBlock)
			if err != nil {
				return fmt.Errorf("failed to get transmissions: %w", err)
			}

			if len(transmissions) == 0 {
				fmt.Printf("No transmissions found in the last %d blocks\n", blockRange)
				return nil
			}

			// Find min and max rounds
			minRound := uint32(0xFFFFFFFF)
			maxRound := uint32(0)
			minBlock := uint64(0xFFFFFFFFFFFFFFFF)
			maxBlock := uint64(0)

			for _, tx := range transmissions {
				roundID := tx.Epoch<<8 | uint32(tx.Round)
				if roundID < minRound {
					minRound = roundID
				}
				if roundID > maxRound {
					maxRound = roundID
				}
				if tx.BlockNumber < minBlock {
					minBlock = tx.BlockNumber
				}
				if tx.BlockNumber > maxBlock {
					maxBlock = tx.BlockNumber
				}
			}

			// Calculate average block time between rounds
			blockDiff := maxBlock - minBlock
			roundDiff := maxRound - minRound
			avgBlocksPerRound := float64(0)
			if roundDiff > 0 {
				avgBlocksPerRound = float64(blockDiff) / float64(roundDiff)
			}

			// Prepare output data
			info := map[string]interface{}{
				"contract":              contractAddr.Hex(),
				"currentBlock":          currentBlock,
				"checkedBlockRange":     blockRange,
				"transmissionsFound":    len(transmissions),
				"minRound":              minRound,
				"maxRound":              maxRound,
				"minBlock":              minBlock,
				"maxBlock":              maxBlock,
				"avgBlocksPerRound":     avgBlocksPerRound,
				"lastTransmission": map[string]interface{}{
					"round":       transmissions[len(transmissions)-1].Epoch<<8 | uint32(transmissions[len(transmissions)-1].Round),
					"epoch":       transmissions[len(transmissions)-1].Epoch,
					"roundInEpoch": transmissions[len(transmissions)-1].Round,
					"block":       transmissions[len(transmissions)-1].BlockNumber,
					"timestamp":   transmissions[len(transmissions)-1].BlockTimestamp,
				},
			}

			// Validate output format
			if err := ValidateFormat(outputFormat); err != nil {
				return err
			}

			// Display results using output formatter
			formatter := NewOutputFormatter(outputFormat)
			return formatter.Print(info)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "json", "Output format (json, yaml)")
	cmd.Flags().IntVarP(&blockRange, "blocks", "b", 10000, "Number of blocks to check")

	return cmd
}