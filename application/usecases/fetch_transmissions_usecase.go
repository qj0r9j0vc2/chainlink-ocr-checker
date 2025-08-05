// Package usecases contains application use cases that orchestrate business logic.
// It implements the primary operations for fetching, parsing, and watching OCR transmissions.
package usecases

import (
	"context"
	"fmt"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/domain/errors"
	"chainlink-ocr-checker/domain/interfaces"
	"github.com/ethereum/go-ethereum/common"
)

// fetchTransmissionsUseCase implements the FetchTransmissionsUseCase interface.
type fetchTransmissionsUseCase struct {
	transmissionFetcher    interfaces.TransmissionFetcher
	transmissionRepository interfaces.TransmissionRepository
	logger                 interfaces.Logger
}

// NewFetchTransmissionsUseCase creates a new fetch transmissions use case.
func NewFetchTransmissionsUseCase(
	transmissionFetcher interfaces.TransmissionFetcher,
	transmissionRepository interfaces.TransmissionRepository,
	logger interfaces.Logger,
) interfaces.FetchTransmissionsUseCase {
	return &fetchTransmissionsUseCase{
		transmissionFetcher:    transmissionFetcher,
		transmissionRepository: transmissionRepository,
		logger:                 logger,
	}
}

// Execute fetches transmissions for the given parameters.
func (uc *fetchTransmissionsUseCase) Execute(
	ctx context.Context,
	params interfaces.FetchTransmissionsParams,
) (*entities.TransmissionResult, error) {
	// Validate parameters
	if err := uc.validateParams(params); err != nil {
		return nil, err
	}

	uc.logger.Info("Fetching transmissions",
		"contract", params.ContractAddress.Hex(),
		"startRound", params.StartRound,
		"endRound", params.EndRound)

	// Fetch transmissions from blockchain
	result, err := uc.transmissionFetcher.FetchByRounds(
		ctx,
		params.ContractAddress,
		params.StartRound,
		params.EndRound,
	)
	if err != nil {
		uc.logger.Error("Failed to fetch transmissions", "error", err)
		return nil, err
	}

	uc.logger.Info("Fetched transmissions",
		"contract", params.ContractAddress.Hex(),
		"count", len(result.Transmissions))

	// Optionally save to repository if configured
	if uc.transmissionRepository != nil && len(result.Transmissions) > 0 {
		if err := uc.saveTransmissions(ctx, result.Transmissions); err != nil {
			// Log error but don't fail the operation
			uc.logger.Warn("Failed to save transmissions to repository", "error", err)
		}
	}

	return result, nil
}

// validateParams validates the fetch parameters.
func (uc *fetchTransmissionsUseCase) validateParams(params interfaces.FetchTransmissionsParams) error {
	validationErr := &errors.ValidationError{}

	if params.ContractAddress == (common.Address{}) {
		validationErr.AddFieldError("contract_address", "contract address is required")
	}

	if params.StartRound > params.EndRound {
		validationErr.AddFieldError(
			"rounds",
			fmt.Sprintf("invalid range: start=%d > end=%d", params.StartRound, params.EndRound),
		)
	}

	if params.EndRound-params.StartRound > 10000 {
		validationErr.AddFieldError("rounds", "round range too large (max 10000)")
	}

	if validationErr.HasErrors() {
		return validationErr
	}

	return nil
}

// saveTransmissions saves transmissions to the repository.
func (uc *fetchTransmissionsUseCase) saveTransmissions(
	ctx context.Context,
	transmissions []entities.Transmission,
) error {
	// Save in batches to avoid overwhelming the database.
	batchSize := 100
	for i := 0; i < len(transmissions); i += batchSize {
		end := i + batchSize
		if end > len(transmissions) {
			end = len(transmissions)
		}

		batch := transmissions[i:end]
		if err := uc.transmissionRepository.SaveBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to save batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}
