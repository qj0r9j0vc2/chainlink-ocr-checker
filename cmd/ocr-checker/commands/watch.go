// Package commands provides CLI command implementations for the OCR checker tool.
// It contains the fetch, parse, watch, and version commands with their associated flags and handlers.
package commands

import (
	"context"
	"fmt"

	"chainlink-ocr-checker/domain/interfaces"
	"chainlink-ocr-checker/infrastructure/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

// NewWatchCommand creates the watch command.
func NewWatchCommand(container *config.Container) *cobra.Command {
	var (
		outputFormat string
		daysToIgnore int
	)
	
	cmd := &cobra.Command{
		Use:   "watch [transmitter] [rounds_to_check] [days_to_ignore]",
		Short: "Watch transmitter activity across OCR2 jobs",
		Long: `Monitors transmitter participation across all associated OCR2 jobs.
Checks recent rounds for activity and reports job status (Found, Stale, Missing, etc.).`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(_ *cobra.Command, args []string) error {
			// Check if database is configured.
			if container.WatchTransmittersUseCase == nil {
				fmt.Println("Watch command requires database configuration.")
				fmt.Println("The watch command is designed to monitor known transmitter jobs stored in a database.")
				fmt.Println("\nTo monitor a specific contract without a database, use:")
				fmt.Println("  ocr-checker fetch <contract_address> <start_round> <end_round>")
				fmt.Println("  ocr-checker info <contract_address>")
				fmt.Println("\nTo use the watch command, configure a PostgreSQL database in your config file.")
				return nil
			}
			
			// Parse arguments.
			transmitterAddr := common.HexToAddress(args[0])
			
			roundsToCheck, err := parseInt(args[1])
			if err != nil {
				return fmt.Errorf("invalid rounds to check: %w", err)
			}
			
			// Days to ignore is optional.
			if len(args) > 2 {
				daysToIgnore, err = parseInt(args[2])
				if err != nil {
					return fmt.Errorf("invalid days to ignore: %w", err)
				}
			}
			
			// Create context.
			ctx := context.Background()
			
			// Execute use case.
			params := interfaces.WatchTransmittersParams{
				TransmitterAddress: transmitterAddr,
				RoundsToCheck:      roundsToCheck,
				DaysToIgnore:       daysToIgnore,
			}
			
			container.Logger.Info("Watching transmitter",
				"transmitter", transmitterAddr.Hex(),
				"rounds", roundsToCheck,
				"daysToIgnore", daysToIgnore)
			
			result, err := container.WatchTransmittersUseCase.Execute(ctx, params)
			if err != nil {
				return fmt.Errorf("failed to watch transmitter: %w", err)
			}
			
			// Validate output format
			if err := ValidateFormat(outputFormat); err != nil {
				return err
			}

			// Display results using output formatter
			formatter := NewOutputFormatter(outputFormat)
			return formatter.Print(result)
		},
	}
	
	// Add flags.
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "json", "Output format (json, yaml)")
	cmd.Flags().IntVarP(&daysToIgnore, "days", "d", 0, "Days to ignore for stale detection")
	
	return cmd
}

// parseInt parses a string to int.
func parseInt(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}