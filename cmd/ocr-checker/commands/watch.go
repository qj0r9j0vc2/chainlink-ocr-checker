package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/domain/interfaces"
	"chainlink-ocr-checker/infrastructure/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

// NewWatchCommand creates the watch command
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
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if database is configured
			if container.WatchTransmittersUseCase == nil {
				return fmt.Errorf("database configuration required for watch command")
			}
			
			// Parse arguments
			transmitterAddr := common.HexToAddress(args[0])
			
			roundsToCheck, err := parseInt(args[1])
			if err != nil {
				return fmt.Errorf("invalid rounds to check: %w", err)
			}
			
			// Days to ignore is optional
			if len(args) > 2 {
				daysToIgnore, err = parseInt(args[2])
				if err != nil {
					return fmt.Errorf("invalid days to ignore: %w", err)
				}
			}
			
			// Create context
			ctx := context.Background()
			
			// Execute use case
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
			
			// Display results
			if outputFormat == "json" {
				return displayWatchResultsJSON(result)
			}
			return displayWatchResultsTable(result)
		},
	}
	
	// Add flags
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format (table, json)")
	cmd.Flags().IntVarP(&daysToIgnore, "days", "d", 0, "Days to ignore for stale detection")
	
	return cmd
}

// displayWatchResultsTable displays watch results in table format
func displayWatchResultsTable(result *interfaces.WatchTransmittersResult) error {
	// Print summary
	fmt.Printf("\nTransmitter Watch Summary\n")
	fmt.Printf("========================\n")
	fmt.Printf("Total Jobs: %d\n", result.Summary.TotalJobs)
	fmt.Printf("Found: %d\n", result.Summary.FoundJobs)
	fmt.Printf("Stale: %d\n", result.Summary.StaleJobs)
	fmt.Printf("Missing: %d\n", result.Summary.MissingJobs)
	fmt.Printf("No Active: %d\n", result.Summary.NoActiveJobs)
	fmt.Printf("Error: %d\n", result.Summary.ErrorJobs)
	fmt.Printf("\n")
	
	// Print detailed status table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Status\tJob ID\tContract\tLast Round\tLast Seen")
	fmt.Fprintln(w, "------\t------\t--------\t----------\t---------")
	
	for _, status := range result.Statuses {
		lastSeen := "Never"
		if !status.LastTimestamp.IsZero() {
			lastSeen = status.LastTimestamp.Format("2006-01-02 15:04:05")
		}
		
		statusStr := string(status.Status)
		if status.Status == entities.JobStatusError && status.Error != nil {
			statusStr = fmt.Sprintf("%s (%v)", status.Status, status.Error)
		}
		
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			statusStr,
			truncate(status.JobID, 20),
			truncate(status.ContractAddress.Hex(), 20),
			status.LastRound,
			lastSeen,
		)
	}
	
	return w.Flush()
}

// displayWatchResultsJSON displays watch results in JSON format
func displayWatchResultsJSON(result *interfaces.WatchTransmittersResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// parseInt parses a string to int
func parseInt(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

// truncate truncates a string to the specified length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}