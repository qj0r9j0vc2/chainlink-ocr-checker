// Package usecases contains application use cases that orchestrate business logic.
// It implements the primary operations for fetching, parsing, and watching OCR transmissions.
package usecases

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/domain/errors"
	"chainlink-ocr-checker/domain/interfaces"
	"gopkg.in/yaml.v2"
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
	case interfaces.OutputFormatCSV:
		return uc.outputCSV(params.OutputWriter, observerActivities, params.GroupBy)
	case interfaces.OutputFormatText:
		return uc.outputText(params.OutputWriter, observerActivities, params.GroupBy)
	default:
		return uc.outputText(params.OutputWriter, observerActivities, params.GroupBy)
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
		interfaces.OutputFormatCSV:  true,
		interfaces.OutputFormatText: true,
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

// outputCSV outputs observer activities as CSV.
func (uc *parseTransmissionsUseCase) outputCSV(
	w io.Writer,
	activities []entities.ObserverActivity,
	groupBy interfaces.GroupByUnit,
) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()
	
	// Write header
	header := []string{"Observer Index", "Address", "Total Count"}
	
	// Add group-specific headers
	if groupBy == interfaces.GroupByDay {
		// Get all unique days.
		days := make(map[string]bool)
		for _, activity := range activities {
			for day := range activity.DailyCount {
				days[day] = true
			}
		}
		
		// Sort days.
		sortedDays := make([]string, 0, len(days))
		for day := range days {
			sortedDays = append(sortedDays, day)
		}
		sort.Strings(sortedDays)
		
		header = append(header, sortedDays...)
	}
	
	if err := writer.Write(header); err != nil {
		return err
	}
	
	// Write data
	for _, activity := range activities {
		row := []string{
			fmt.Sprintf("%d", activity.ObserverIndex),
			activity.Address.Hex(),
			fmt.Sprintf("%d", activity.TotalCount),
		}
		
		if groupBy == interfaces.GroupByDay {
			// Add daily counts.
			for i := 3; i < len(header); i++ {
				day := header[i]
				count := activity.DailyCount[day]
				row = append(row, fmt.Sprintf("%d", count))
			}
		}
		
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	
	return nil
}

// outputText outputs observer activities as formatted text.
func (uc *parseTransmissionsUseCase) outputText(
	w io.Writer,
	activities []entities.ObserverActivity,
	groupBy interfaces.GroupByUnit,
) error {
	// Sort activities by observer index.
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].ObserverIndex < activities[j].ObserverIndex
	})
	
	// Print header.
	_, _ = fmt.Fprintf(w, "Observer Activity Report\n")
	_, _ = fmt.Fprintf(w, "========================\n")
	_, _ = fmt.Fprintf(w, "Group By: %s\n\n", groupBy)
	
	// Print table header.
	switch groupBy {
	case interfaces.GroupByDay:
		_, _ = fmt.Fprintf(w, "%-5s %-44s %-10s %s\n", "Index", "Address", "Total", "Daily Activity")
		_, _ = fmt.Fprintf(w, "%s\n", strings.Repeat("-", 100))
	case interfaces.GroupByMonth:
		_, _ = fmt.Fprintf(w, "%-5s %-44s %-10s %s\n", "Index", "Address", "Total", "Monthly Activity")
		_, _ = fmt.Fprintf(w, "%s\n", strings.Repeat("-", 100))
	default:
		_, _ = fmt.Fprintf(w, "%-5s %-44s %-10s\n", "Index", "Address", "Total")
		_, _ = fmt.Fprintf(w, "%s\n", strings.Repeat("-", 60))
	}
	
	// Print data.
	for _, activity := range activities {
		_, _ = fmt.Fprintf(w, "%-5d %-44s %-10d",
			activity.ObserverIndex,
			activity.Address.Hex(),
			activity.TotalCount)
		
		switch groupBy {
		case interfaces.GroupByDay:
			// Sort and print daily counts.
			days := make([]string, 0, len(activity.DailyCount))
			for day := range activity.DailyCount {
				days = append(days, day)
			}
			sort.Strings(days)
			
			dailyStr := make([]string, 0, len(days))
			for _, day := range days {
				if count := activity.DailyCount[day]; count > 0 {
					dailyStr = append(dailyStr, fmt.Sprintf("%s:%d", day, count))
				}
			}
			_, _ = fmt.Fprintf(w, " %s", strings.Join(dailyStr, ", "))
		case interfaces.GroupByMonth:
			// Sort and print monthly counts.
			months := make([]string, 0, len(activity.MonthlyCount))
			for month := range activity.MonthlyCount {
				months = append(months, month)
			}
			sort.Strings(months)
			
			monthlyStr := make([]string, 0, len(months))
			for _, month := range months {
				if count := activity.MonthlyCount[month]; count > 0 {
					monthlyStr = append(monthlyStr, fmt.Sprintf("%s:%d", month, count))
				}
			}
			_, _ = fmt.Fprintf(w, " %s", strings.Join(monthlyStr, ", "))
		}
		
		_, _ = fmt.Fprintln(w)
	}
	
	// Print summary.
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "Summary\n")
	_, _ = fmt.Fprintf(w, "-------\n")
	_, _ = fmt.Fprintf(w, "Total Observers: %d\n", len(activities))
	
	totalTransmissions := 0
	for _, activity := range activities {
		totalTransmissions += activity.TotalCount
	}
	_, _ = fmt.Fprintf(w, "Total Transmissions: %d\n", totalTransmissions)
	
	return nil
}