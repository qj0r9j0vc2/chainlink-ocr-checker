// Package config provides configuration constants and utilities for the OCR checker application.
// It contains flag constants, database configuration, and shared configuration types.
package config

// Flag constants for CLI commands.
const (
	// LogLevelFlag is the flag for setting log level.
	LogLevelFlag      = "log-level"
	ShortLogLevelFlag = "l"

	// ConfigFileFlag is the flag for specifying config file.
	ConfigFileFlag      = "config"
	ShortConfigFileFlag = "c"

	// OutputTypeFlag is the flag for output format.
	OutputTypeFlag      = "output"
	ShortOutputTypeFlag = "o"
)
