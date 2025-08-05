package commands

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// Version information (set by build flags)
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// NewVersionCommand creates the version command
func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Long:  "Shows the version, git commit, build date, and runtime information.",
		Run: func(cmd *cobra.Command, args []string) {
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