// Package interfaces defines contracts and interfaces for the OCR checker domain layer.
// It contains interfaces for blockchain operations, repositories, use cases, and logging.
package interfaces

import (
	"context"
	"io"

	"chainlink-ocr-checker/domain/entities"
	"github.com/ethereum/go-ethereum/common"
)

// FetchTransmissionsUseCase handles the business logic for fetching transmissions.
type FetchTransmissionsUseCase interface {
	// Execute fetches transmissions for the given parameters.
	Execute(ctx context.Context, params FetchTransmissionsParams) (*entities.TransmissionResult, error)
}

// FetchTransmissionsParams represents parameters for fetching transmissions.
type FetchTransmissionsParams struct {
	ContractAddress common.Address
	StartRound      uint32
	EndRound        uint32
	OutputFormat    OutputFormat
}

// WatchTransmittersUseCase handles the business logic for watching transmitters.
type WatchTransmittersUseCase interface {
	// Execute watches transmitter activity.
	Execute(ctx context.Context, params WatchTransmittersParams) (*WatchTransmittersResult, error)
}

// WatchTransmittersParams represents parameters for watching transmitters.
type WatchTransmittersParams struct {
	TransmitterAddress common.Address
	RoundsToCheck      int
	DaysToIgnore       int
}

// WatchTransmittersResult represents the result of watching transmitters.
type WatchTransmittersResult struct {
	Statuses []entities.TransmitterStatus
	Summary  TransmitterSummary
}

// TransmitterSummary provides a summary of transmitter statuses.
type TransmitterSummary struct {
	TotalJobs    int
	FoundJobs    int
	StaleJobs    int
	MissingJobs  int
	NoActiveJobs int
	ErrorJobs    int
}

// ParseTransmissionsUseCase handles parsing transmission data.
type ParseTransmissionsUseCase interface {
	// Execute parses transmission data and generates reports.
	Execute(ctx context.Context, params ParseTransmissionsParams) error
}

// ParseTransmissionsParams represents parameters for parsing transmissions.
type ParseTransmissionsParams struct {
	InputPath    string
	OutputWriter io.Writer
	GroupBy      GroupByUnit
	OutputFormat OutputFormat
}

// GroupByUnit represents the unit for grouping data.
type GroupByUnit string

// GroupByUnit constants.
const (
	GroupByDay   GroupByUnit = "day"
	GroupByMonth GroupByUnit = "month"
	GroupByRound GroupByUnit = "round"
)

// OutputFormat represents the output format.
type OutputFormat string

// OutputFormat constants.
const (
	OutputFormatJSON OutputFormat = "json"
	OutputFormatYAML OutputFormat = "yaml"
)

// TransmissionAnalyzer analyzes transmission patterns.
type TransmissionAnalyzer interface {
	// AnalyzeObserverActivity analyzes observer participation.
	AnalyzeObserverActivity(transmissions []entities.Transmission) ([]entities.ObserverActivity, error)

	// DetectAnomalies detects anomalies in transmission patterns.
	DetectAnomalies(transmissions []entities.Transmission) ([]TransmissionAnomaly, error)

	// GenerateReport generates a comprehensive report.
	GenerateReport(transmissions []entities.Transmission, format OutputFormat) ([]byte, error)
}

// TransmissionAnomaly represents an anomaly in transmission patterns.
type TransmissionAnomaly struct {
	Type        AnomalyType
	Description string
	Severity    AnomalySeverity
	Timestamp   int64
	Details     map[string]interface{}
}

// AnomalyType represents the type of anomaly.
type AnomalyType string

// AnomalyType constants.
const (
	AnomalyTypeMissingRound     AnomalyType = "missing_round"
	AnomalyTypeDuplicateRound   AnomalyType = "duplicate_round"
	AnomalyTypeInactiveObserver AnomalyType = "inactive_observer"
	AnomalyTypeHighLatency      AnomalyType = "high_latency"
)

// AnomalySeverity represents the severity of an anomaly.
type AnomalySeverity string

// AnomalySeverity constants.
const (
	AnomalySeverityLow    AnomalySeverity = "low"
	AnomalySeverityMedium AnomalySeverity = "medium"
	AnomalySeverityHigh   AnomalySeverity = "high"
)
