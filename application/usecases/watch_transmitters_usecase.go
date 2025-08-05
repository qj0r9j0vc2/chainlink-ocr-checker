// Package usecases contains application use cases that orchestrate business logic.
// It implements the primary operations for fetching, parsing, and watching OCR transmissions.
package usecases

import (
	"context"
	"time"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/domain/errors"
	"chainlink-ocr-checker/domain/interfaces"
	"github.com/ethereum/go-ethereum/common"
)

// watchTransmittersUseCase implements the WatchTransmittersUseCase interface.
type watchTransmittersUseCase struct {
	jobRepository      interfaces.JobRepository
	transmissionFetcher interfaces.TransmissionFetcher
	aggregatorService  interfaces.OCR2AggregatorService
	logger             interfaces.Logger
}

// NewWatchTransmittersUseCase creates a new watch transmitters use case.
func NewWatchTransmittersUseCase(
	jobRepository interfaces.JobRepository,
	transmissionFetcher interfaces.TransmissionFetcher,
	aggregatorService interfaces.OCR2AggregatorService,
	logger interfaces.Logger,
) interfaces.WatchTransmittersUseCase {
	return &watchTransmittersUseCase{
		jobRepository:      jobRepository,
		transmissionFetcher: transmissionFetcher,
		aggregatorService:  aggregatorService,
		logger:             logger,
	}
}

// Execute watches transmitter activity.
func (uc *watchTransmittersUseCase) Execute(
	ctx context.Context,
	params interfaces.WatchTransmittersParams,
) (*interfaces.WatchTransmittersResult, error) {
	// Validate parameters
	if err := uc.validateParams(params); err != nil {
		return nil, err
	}
	
	uc.logger.Info("Watching transmitter activity",
		"transmitter", params.TransmitterAddress.Hex(),
		"rounds", params.RoundsToCheck,
		"daysToIgnore", params.DaysToIgnore)
	
	// Find jobs for the transmitter
	jobs, err := uc.jobRepository.FindByTransmitter(ctx, params.TransmitterAddress)
	if err != nil {
		uc.logger.Error("Failed to find jobs", "error", err)
		return nil, err
	}
	
	if len(jobs) == 0 {
		uc.logger.Warn("No jobs found for transmitter", "transmitter", params.TransmitterAddress.Hex())
		return &interfaces.WatchTransmittersResult{
			Statuses: []entities.TransmitterStatus{},
			Summary: interfaces.TransmitterSummary{
				TotalJobs: 0,
			},
		}, nil
	}
	
	// Check each job's status
	statuses := make([]entities.TransmitterStatus, 0, len(jobs))
	summary := interfaces.TransmitterSummary{
		TotalJobs: len(jobs),
	}
	
	cutoffTime := time.Now().AddDate(0, 0, -params.DaysToIgnore)
	
	for _, job := range jobs {
		status := uc.checkJobStatus(ctx, job, params.RoundsToCheck, cutoffTime)
		statuses = append(statuses, status)
		
		// Update summary
		switch status.Status {
		case entities.JobStatusFound:
			summary.FoundJobs++
		case entities.JobStatusStale:
			summary.StaleJobs++
		case entities.JobStatusMissing:
			summary.MissingJobs++
		case entities.JobStatusNoActive:
			summary.NoActiveJobs++
		case entities.JobStatusError:
			summary.ErrorJobs++
		}
	}
	
	uc.logger.Info("Transmitter watch completed",
		"transmitter", params.TransmitterAddress.Hex(),
		"total", summary.TotalJobs,
		"found", summary.FoundJobs,
		"stale", summary.StaleJobs,
		"missing", summary.MissingJobs,
		"noActive", summary.NoActiveJobs,
		"error", summary.ErrorJobs)
	
	return &interfaces.WatchTransmittersResult{
		Statuses: statuses,
		Summary:  summary,
	}, nil
}

// validateParams validates the watch parameters.
func (uc *watchTransmittersUseCase) validateParams(params interfaces.WatchTransmittersParams) error {
	validationErr := &errors.ValidationError{}
	
	if params.TransmitterAddress == (common.Address{}) {
		validationErr.AddFieldError("transmitter_address", "transmitter address is required")
	}
	
	if params.RoundsToCheck <= 0 {
		validationErr.AddFieldError("rounds_to_check", "rounds to check must be positive")
	}
	
	if params.RoundsToCheck > 100 {
		validationErr.AddFieldError("rounds_to_check", "rounds to check must not exceed 100")
	}
	
	if params.DaysToIgnore < 0 {
		validationErr.AddFieldError("days_to_ignore", "days to ignore cannot be negative")
	}
	
	if validationErr.HasErrors() {
		return validationErr
	}
	
	return nil
}

// checkJobStatus checks the status of a single job.
func (uc *watchTransmittersUseCase) checkJobStatus(
	ctx context.Context,
	job entities.Job,
	roundsToCheck int,
	cutoffTime time.Time,
) entities.TransmitterStatus {
	status := entities.TransmitterStatus{
		Address:         job.TransmitterAddress,
		JobID:           job.ExternalJobID,
		ContractAddress: job.OracleSpec.ContractAddress,
	}
	
	// Check if job is active.
	if !job.Active {
		status.Status = entities.JobStatusNoActive
		return status
	}
	
	// Get latest round from the aggregator.
	latestRound, err := uc.aggregatorService.GetLatestRound(ctx, job.OracleSpec.ContractAddress)
	if err != nil {
		uc.logger.Error("Failed to get latest round",
			"contract", job.OracleSpec.ContractAddress.Hex(),
			"error", err)
		status.Status = entities.JobStatusError
		status.Error = err
		return status
	}
	
	// Calculate the round range to check.
	endRound := latestRound.RoundID
	var startRound uint32
	// Safe conversion with bounds check
	if roundsToCheck > int(endRound) {
		startRound = 1
	} else {
		startRound = endRound - uint32(roundsToCheck) + 1 // #nosec G115 -- bounds checked
		if startRound < 1 {
			startRound = 1
		}
	}
	
	// Fetch transmissions for the round range.
	result, err := uc.transmissionFetcher.FetchByRounds(
		ctx,
		job.OracleSpec.ContractAddress,
		startRound,
		endRound,
	)
	if err != nil {
		uc.logger.Error("Failed to fetch transmissions",
			"contract", job.OracleSpec.ContractAddress.Hex(),
			"error", err)
		status.Status = entities.JobStatusError
		status.Error = err
		return status
	}
	
	// Find transmissions from our transmitter.
	found := false
	var lastTransmissionTime time.Time
	
	for _, tx := range result.Transmissions {
		if tx.TransmitterAddress == job.TransmitterAddress {
			found = true
			if tx.BlockTimestamp.After(lastTransmissionTime) {
				lastTransmissionTime = tx.BlockTimestamp
				status.LastRound = tx.Epoch<<8 | uint32(tx.Round)
				status.LastTimestamp = tx.BlockTimestamp
			}
		}
	}
	
	// Determine status based on findings.
	switch {
	case !found:
		status.Status = entities.JobStatusMissing
	case lastTransmissionTime.Before(cutoffTime):
		status.Status = entities.JobStatusStale
	default:
		status.Status = entities.JobStatusFound
	}
	
	return status
}