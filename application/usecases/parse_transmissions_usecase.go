// Package usecases contains application use cases that orchestrate business logic.
// It implements the primary operations for fetching, parsing, and watching OCR transmissions.
package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/domain/errors"
	"chainlink-ocr-checker/domain/interfaces"
	"gopkg.in/yaml.v3"
)

// parseTransmissionsUseCase implements the ParseTransmissionsUseCase interface.
type parseTransmissionsUseCase struct {
	analyzer interfaces.TransmissionAnalyzer
	logger   interfaces.Logger
}

// NewParseTransmissionsUseCase creates a new parse transmissions use case.
func NewParseTransmissionsUseCase(
	analyzer interfaces.TransmissionAnalyzer,
	logger interfaces.Logger,
) interfaces.ParseTransmissionsUseCase {
	return &parseTransmissionsUseCase{
		analyzer: analyzer,
		logger:   logger,
	}
}

// Execute parses transmission data and generates reports.
func (uc *parseTransmissionsUseCase) Execute(_ context.Context, params interfaces.ParseTransmissionsParams) error {
	// Validate parameters
	if err := uc.validateParams(params); err != nil {
		return err
	}
	
	uc.logger.Info("Parsing transmissions",
		"input", params.InputPath,
		"groupBy", params.GroupBy,
		"format", params.OutputFormat)
	
	// Read input file
	transmissions, err := uc.readTransmissions(params.InputPath)
	if err != nil {
		uc.logger.Error("Failed to read transmissions", "error", err)
		return err
	}
	
	if len(transmissions) == 0 {
		uc.logger.Warn("No transmissions found in input file")
		return nil
	}
	
	uc.logger.Info("Loaded transmissions", "count", len(transmissions))
	
	// Analyze transmissions
	observerActivities, err := uc.analyzer.AnalyzeObserverActivity(transmissions)
	if err != nil {
		uc.logger.Error("Failed to analyze observer activity", "error", err)
		return err
	}
	
	// Generate output based on format
	switch params.OutputFormat {
	case interfaces.OutputFormatJSON:
		return uc.outputJSON(params.OutputWriter, observerActivities, params.GroupBy)
	case interfaces.OutputFormatYAML:
		return uc.outputYAML(params.OutputWriter, observerActivities, params.GroupBy)
	default:
		return fmt.Errorf("unsupported output format: %s", params.OutputFormat)
	}
}

// validateParams validates the parse parameters.
func (uc *parseTransmissionsUseCase) validateParams(params interfaces.ParseTransmissionsParams) error {
	validationErr := &errors.ValidationError{}
	
	if params.InputPath == "" {
		validationErr.AddFieldError("input_path", "input path is required")
	}
	
	if params.OutputWriter == nil {
		validationErr.AddFieldError("output_writer", "output writer is required")
	}
	
	validGroupBy := map[interfaces.GroupByUnit]bool{
		interfaces.GroupByDay:   true,
		interfaces.GroupByMonth: true,
		interfaces.GroupByRound: true,
	}
	
	if !validGroupBy[params.GroupBy] {
		validationErr.AddFieldError(
			"group_by",
			fmt.Sprintf("invalid group by unit: %s", params.GroupBy),
		)
	}
	
	validFormats := map[interfaces.OutputFormat]bool{
		interfaces.OutputFormatJSON: true,
		interfaces.OutputFormatYAML: true,
	}
	
	if !validFormats[params.OutputFormat] {
		validationErr.AddFieldError(
			"output_format",
			fmt.Sprintf("invalid output format: %s", params.OutputFormat),
		)
	}
	
	if validationErr.HasErrors() {
		return validationErr
	}
	
	return nil
}

// readTransmissions reads transmissions from a YAML file.
func (uc *parseTransmissionsUseCase) readTransmissions(path string) ([]entities.Transmission, error) {
	// Clean and validate the path
	cleanPath := filepath.Clean(path)
	file, err := os.Open(cleanPath) // #nosec G304 -- path is cleaned
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			uc.logger.Error("Failed to close file", "error", cerr)
		}
	}()
	
	var result entities.TransmissionResult
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}
	
	return result.Transmissions, nil
}

// outputJSON outputs observer activities as JSON.
func (uc *parseTransmissionsUseCase) outputJSON(
	w io.Writer,
	activities []entities.ObserverActivity,
	groupBy interfaces.GroupByUnit,
) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	
	output := map[string]interface{}{
		"groupBy":    groupBy,
		"activities": activities,
	}
	
	return encoder.Encode(output)
}

// outputYAML outputs observer activities as YAML via JSON conversion.
func (uc *parseTransmissionsUseCase) outputYAML(
	w io.Writer,
	activities []entities.ObserverActivity,
	groupBy interfaces.GroupByUnit,
) error {
	// First convert to JSON
	output := map[string]interface{}{
		"groupBy":    groupBy,
		"activities": activities,
	}
	
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal to JSON: %w", err)
	}
	
	// Then convert JSON to YAML
	var data interface{}
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal JSON for YAML conversion: %w", err)
	}
	
	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal to YAML: %w", err)
	}
	
	_, err = w.Write(yamlBytes)
	return err
}
