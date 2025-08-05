// Package commands provides CLI command implementations for the OCR checker tool.
// It contains the fetch, parse, watch, and version commands with their associated flags and handlers.
package commands

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// Version represents the current version of the OCR checker tool (set by build flags).
	Version   = "dev"
	// GitCommit represents the git commit hash used to build this version (set by build flags).
	GitCommit = "unknown"
	// BuildDate represents the date when this version was built (set by build flags).
	BuildDate = "unknown"
)

// NewVersionCommand creates the version command.
func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Long:  "Shows the version, git commit, build date, and runtime information.",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("OCR Checker\n")
			fmt.Printf("===========\n")
			fmt.Printf("Version:    %s\n", Version)
			fmt.Printf("Git Commit: %s\n", GitCommit)
			fmt.Printf("Build Date: %s\n", BuildDate)
			fmt.Printf("Go Version: %s\n", runtime.Version())
			fmt.Printf("OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}
	
	return cmd
}