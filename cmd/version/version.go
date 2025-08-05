// Package version provides version information and build metadata for the OCR checker application.
// It contains version constants that are set during the build process.
package version

import (
	"fmt"
	"runtime"
)

var (
	// AppVersion represents the application version, set during build.
	AppVersion = ""
	// GitCommit represents the git commit hash, set during build.
	GitCommit  = ""
	// BuildDate represents the build date, set during build.
	BuildDate  = ""

	// GoVersion represents the Go version used for building.
	GoVersion = ""
	// GoArch represents the target architecture.
	GoArch    = ""
)

func init() {
	if len(AppVersion) == 0 {
		AppVersion = "dev"
	}

	GoVersion = runtime.Version()
	GoArch = runtime.GOARCH
}

// Version returns a formatted version string with build information.
func Version() string {
	return fmt.Sprintf(
		"Version %s (%s)\nCompiled at %s using Go %s (%s)",
		AppVersion,
		GitCommit,
		BuildDate,
		GoVersion,
		GoArch,
	)
}
