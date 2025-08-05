// Package commands provides CLI command implementations for the OCR checker tool.
// It contains the fetch, parse, watch, and version commands with their associated flags and handlers.
package commands

import (
	"chainlink-ocr-checker/domain/entities"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"chainlink-ocr-checker/domain/interfaces"
	"chainlink-ocr-checker/infrastructure/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// NewFetchCommand creates the fetch command.
func NewFetchCommand(container *config.Container) *cobra.Command {
	var (
		outputFormat string
		outputPath   string
	)

	cmd := &cobra.Command{
		Use:   "fetch [contract] [start_round] [end_round]",
		Short: "Fetch OCR transmission data for a contract",
		Long: `Fetches historical OCR transmission data for a specific contract
within the given round range. The data includes transmitter participation,
observer indices, and block information.`,
		Args: cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			// Parse arguments.
			contractAddr := common.HexToAddress(args[0])
			startRound, err := parseUint32(args[1])
			if err != nil {
				return fmt.Errorf("invalid start round: %w", err)
			}
			endRound, err := parseUint32(args[2])
			if err != nil {
				return fmt.Errorf("invalid end round: %w", err)
			}

			// Create context.
			ctx := context.Background()

			// Execute use case.
			params := interfaces.FetchTransmissionsParams{
				ContractAddress: contractAddr,
				StartRound:      startRound,
				EndRound:        endRound,
			}

			container.Logger.Info("Fetching transmissions",
				"contract", contractAddr.Hex(),
				"startRound", startRound,
				"endRound", endRound)

			result, err := container.FetchTransmissionsUseCase.Execute(ctx, params)
			if err != nil {
				return fmt.Errorf("failed to fetch transmissions: %w", err)
			}

			container.Logger.Info("Fetch completed",
				"transmissions", len(result.Transmissions))

			// Save results.
			if outputPath == "" {
				outputPath = fmt.Sprintf("results/%s-%d_%d.yaml",
					contractAddr.Hex(), startRound, endRound)
			}

			if err := saveResults(result, outputPath, outputFormat); err != nil {
				return fmt.Errorf("failed to save results: %w", err)
			}

			container.Logger.Info("Results saved", "path", outputPath)

			// Print summary.
			fmt.Printf("Fetched %d transmissions for contract %s\n",
				len(result.Transmissions), contractAddr.Hex())
			fmt.Printf("Round range: %d - %d\n", startRound, endRound)
			fmt.Printf("Results saved to: %s\n", outputPath)

			return nil
		},
	}

	// Add flags.
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "yaml", "Output format (yaml, json)")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path")

	return cmd
}

// saveResults saves the transmission results to a file.
func saveResults(result *entities.TransmissionResult, path string, format string) error {
	// Create directory if needed.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Open file.
	cleanPath := filepath.Clean(path)
	file, err := os.Create(cleanPath) // #nosec G304 -- path is cleaned
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to close file: %v\n", cerr)
		}
	}()

	// Encode based on format.
	switch format {
	case OutputFormatJSON:
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	case "yaml":
		encoder := yaml.NewEncoder(file)
		return encoder.Encode(result)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// parseUint32 parses a string to uint32.
func parseUint32(s string) (uint32, error) {
	var v uint32
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}
