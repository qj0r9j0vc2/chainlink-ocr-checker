// Package main is the entry point for the OCR checker CLI application.
// It provides commands for fetching, parsing, and watching OCR transmission data.
package main

import (
	"fmt"
	"os"

	"chainlink-ocr-checker/cmd/ocr-checker/commands"
	"chainlink-ocr-checker/infrastructure/config"
	"github.com/spf13/cobra"
)

func main() {
	exitCode := run()
	os.Exit(exitCode)
}

func run() int {
	// Create root command.
	rootCmd := &cobra.Command{
		Use:   "ocr-checker",
		Short: "Chainlink OCR2 monitoring tool",
		Long: `A tool for monitoring and analyzing Chainlink OCR2 transmitter activity
and protocol performance across different blockchain networks.`,
	}
	
	// Global flags.
	var configPath string
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path")
	
	// Load configuration.
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		// For some commands, config might not be required.
		cfg = &config.Config{
			LogLevel: "info",
		}
	}
	
	// Create dependency container.
	container, err := config.NewContainer(cfg)
	if err != nil {
		rootCmd.PrintErrf("Failed to initialize: %v\n", err)
		return 1
	}
	defer func() {
		if err := container.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to close container: %v\n", err)
		}
	}()
	
	// Add commands.
	rootCmd.AddCommand(
		commands.NewFetchCommand(container),
		commands.NewWatchCommand(container),
		commands.NewParseCommand(container),
		commands.NewVersionCommand(),
	)
	
	// Execute.
	if err := rootCmd.Execute(); err != nil {
		return 1
	}
	
	return 0
}
