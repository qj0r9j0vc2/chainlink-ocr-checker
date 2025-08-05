// Package commands provides CLI command implementations for the OCR checker tool.
package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// OutputFormatter handles output formatting for commands.
type OutputFormatter struct {
	format string
}

// NewOutputFormatter creates a new output formatter.
func NewOutputFormatter(format string) *OutputFormatter {
	return &OutputFormatter{
		format: format,
	}
}

// Print formats and prints the data according to the specified format.
func (f *OutputFormatter) Print(data interface{}) error {
	// First convert to JSON
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	switch f.format {
	case OutputFormatJSON:
		// Pretty print JSON
		var prettyJSON interface{}
		if err := json.Unmarshal(jsonBytes, &prettyJSON); err != nil {
			return fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(prettyJSON)

	case OutputFormatYAML:
		// Convert JSON to YAML
		var data interface{}
		if err := json.Unmarshal(jsonBytes, &data); err != nil {
			return fmt.Errorf("failed to unmarshal JSON for YAML conversion: %w", err)
		}
		yamlBytes, err := yaml.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal to YAML: %w", err)
		}
		_, err = os.Stdout.Write(yamlBytes)
		return err

	default:
		return fmt.Errorf("unsupported output format: %s", f.format)
	}
}

// ValidateFormat checks if the output format is valid.
func ValidateFormat(format string) error {
	switch format {
	case OutputFormatJSON, OutputFormatYAML:
		return nil
	default:
		return fmt.Errorf("invalid output format: %s (supported: json, yaml)", format)
	}
}