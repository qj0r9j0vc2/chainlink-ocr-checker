// Package commands provides CLI command implementations for the OCR checker tool.
// It contains the fetch, parse, watch, and version commands with their associated flags and handlers.
package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"chainlink-ocr-checker/domain/interfaces"
	"chainlink-ocr-checker/infrastructure/config"
	"github.com/spf13/cobra"
)

// NewParseCommand creates the parse command.
func NewParseCommand(container *config.Container) *cobra.Command {
	var (
		outputFormat string
		outputPath   string
	)
	
	cmd := &cobra.Command{
		Use:   "parse [input_file] [group_by]",
		Short: "Parse and analyze transmission data",
		Long: `Parses transmission data from a YAML/JSON file and generates
observer activity reports grouped by day, month, or round.`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			// Parse arguments.
			inputPath := args[0]
			groupByStr := args[1]
			
			// Map group by string to enum.
			var groupBy interfaces.GroupByUnit
			switch groupByStr {
			case "day":
				groupBy = interfaces.GroupByDay
			case "month":
				groupBy = interfaces.GroupByMonth
			case "round":
				groupBy = interfaces.GroupByRound
			default:
				return fmt.Errorf("invalid group by unit: %s (use day, month, or round)", groupByStr)
			}
			
			// Map output format string to enum.
			var format interfaces.OutputFormat
			switch outputFormat {
			case OutputFormatJSON:
				format = interfaces.OutputFormatJSON
			case "csv":
				format = interfaces.OutputFormatCSV
			case "text":
				format = interfaces.OutputFormatText
			case "yaml":
				format = interfaces.OutputFormatYAML
			default:
				format = interfaces.OutputFormatText
			}
			
			// Create context.
			ctx := context.Background()
			
			// Determine output writer.
			var outputWriter *os.File
			if outputPath != "" {
				cleanPath := filepath.Clean(outputPath)
				file, err := os.Create(cleanPath) // #nosec G304 -- path is cleaned
				if err != nil {
					return fmt.Errorf("failed to create output file: %w", err)
				}
				defer func() {
					if err := file.Close(); err != nil {
						container.Logger.Error("Failed to close output file", "error", err)
					}
				}()
				outputWriter = file
			} else {
				outputWriter = os.Stdout
			}
			
			// Execute use case.
			params := interfaces.ParseTransmissionsParams{
				InputPath:    inputPath,
				OutputWriter: outputWriter,
				GroupBy:      groupBy,
				OutputFormat: format,
			}
			
			container.Logger.Info("Parsing transmissions",
				"input", inputPath,
				"groupBy", groupBy,
				"format", format)
			
			if err := container.ParseTransmissionsUseCase.Execute(ctx, params); err != nil {
				return fmt.Errorf("failed to parse transmissions: %w", err)
			}
			
			container.Logger.Info("Parsing completed")
			
			if outputPath != "" {
				fmt.Printf("Results saved to: %s\n", outputPath)
			}
			
			return nil
		},
	}
	
	// Add flags.
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "text", "Output format (text, json, csv, yaml)")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (default: stdout)")
	
	return cmd
}