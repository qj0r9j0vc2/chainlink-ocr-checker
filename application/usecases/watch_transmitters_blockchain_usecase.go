// Package usecases provides application use cases for the OCR checker.
package usecases

import (
	"context"
	"fmt"
	"sort"
	"time"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/domain/interfaces"
	"github.com/ethereum/go-ethereum/common"
)

// watchTransmittersBlockchainUseCase implements blockchain-based transmitter watching without database.
type watchTransmittersBlockchainUseCase struct {
	blockchainClient      interfaces.BlockchainClient
	transmissionFetcher   interfaces.TransmissionFetcher
	aggregatorService     interfaces.OCR2AggregatorService
	logger                interfaces.Logger
}

// NewWatchTransmittersBlockchainUseCase creates a new blockchain-based watch transmitters use case.
func NewWatchTransmittersBlockchainUseCase(
	blockchainClient interfaces.BlockchainClient,
	transmissionFetcher interfaces.TransmissionFetcher,
	aggregatorService interfaces.OCR2AggregatorService,
	logger interfaces.Logger,
) interfaces.WatchTransmittersUseCase {
	return &watchTransmittersBlockchainUseCase{
		blockchainClient:    blockchainClient,
		transmissionFetcher: transmissionFetcher,
		aggregatorService:   aggregatorService,
		logger:              logger,
	}
}

// Execute watches transmitter activity by scanning blockchain for associated contracts.
func (uc *watchTransmittersBlockchainUseCase) Execute(
	ctx context.Context,
	params interfaces.WatchTransmittersParams,
) (*interfaces.WatchTransmittersResult, error) {
	// Validate parameters
	if err := uc.validateParams(params); err != nil {
		return nil, err
	}

	uc.logger.Info("Watching transmitter activity on blockchain",
		"transmitter", params.TransmitterAddress.Hex(),
		"rounds", params.RoundsToCheck,
		"daysToIgnore", params.DaysToIgnore)

	// Get current block
	currentBlock, err := uc.blockchainClient.GetBlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current block: %w", err)
	}

	// Calculate block range to scan (approximately last 7 days)
	// Polygon has ~2-3 second block time, so ~30,000 blocks per day
	blocksToScan := uint64(30000 * 7) // 7 days
	if blocksToScan > currentBlock {
		blocksToScan = currentBlock
	}
	startBlock := currentBlock - blocksToScan

	uc.logger.Info("Scanning for transmitter activity",
		"startBlock", startBlock,
		"endBlock", currentBlock,
		"transmitter", params.TransmitterAddress.Hex())

	// Find contracts where this transmitter is active
	contracts, err := uc.findActiveContracts(ctx, params.TransmitterAddress, startBlock, currentBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to find active contracts: %w", err)
	}

	if len(contracts) == 0 {
		uc.logger.Warn("No active contracts found for transmitter",
			"transmitter", params.TransmitterAddress.Hex())
		return &interfaces.WatchTransmittersResult{
			Statuses: []entities.TransmitterStatus{},
			Summary: interfaces.TransmitterSummary{
				TotalJobs: 0,
			},
		}, nil
	}

	// Check status for each contract
	statuses := make([]entities.TransmitterStatus, 0, len(contracts))
	summary := interfaces.TransmitterSummary{
		TotalJobs: len(contracts),
	}

	cutoffTime := time.Now().AddDate(0, 0, -params.DaysToIgnore)

	for _, contractAddr := range contracts {
		status := uc.checkContractStatus(ctx, contractAddr, params.TransmitterAddress, params.RoundsToCheck, cutoffTime)
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
		"missing", summary.MissingJobs)

	return &interfaces.WatchTransmittersResult{
		Statuses: statuses,
		Summary:  summary,
	}, nil
}

// findActiveContracts finds contracts where the transmitter is active.
func (uc *watchTransmittersBlockchainUseCase) findActiveContracts(
	ctx context.Context,
	transmitterAddr common.Address,
	startBlock, endBlock uint64,
) ([]common.Address, error) {
	// For now, we'll return a list of known contracts
	// In a production system, you would scan logs or use a registry
	knownContracts := []common.Address{
		// Polygon mainnet OCR contracts
		common.HexToAddress("0xa142BB41f409599603D3bB16842D0d274AAeDcf5"),
		common.HexToAddress("0x4A5e7D4BE70969E9e315d2655EB7d639C6E11A1a"),
		common.HexToAddress("0x9381Ea71066835a58b9F4055a7B7793E6e365732"),
		common.HexToAddress("0x420c24B9f0B11105F4366EeE822002E1ADEF17a8"),
		common.HexToAddress("0x5f4d57fD4FBf7Fc29228A9269F492d806435Dc34"),
		common.HexToAddress("0xed2a7Db60e32c0818Ae3eA2f82465FAA24c45773"),
		common.HexToAddress("0x9dd18534b8f456557d11B9DDB14dA89b2e52e308"),
		common.HexToAddress("0x73f88269629ce4e2dc10106F5e97AFa802F38763"),
		common.HexToAddress("0x336e0163502A2092c0FcC26B66F84a8f5fBE7C8F"),
		common.HexToAddress("0xC907E116054Ad103354f2D350FD2514433D57F6f"),
		// Add more known OCR contracts here
	}

	// Filter contracts where transmitter is actually active
	activeContracts := make([]common.Address, 0)
	
	for _, contract := range knownContracts {
		// Check recent activity to see if transmitter is active
		// Get last 1000 blocks of activity
		recentBlocks := uint64(1000)
		checkStartBlock := endBlock - recentBlocks
		if checkStartBlock < startBlock {
			checkStartBlock = startBlock
		}
		
		transmissions, err := uc.aggregatorService.GetTransmissions(ctx, contract, checkStartBlock, endBlock)
		if err != nil {
			uc.logger.Debug("Failed to get transmissions for contract",
				"contract", contract.Hex(),
				"error", err)
			continue
		}

		// Check if transmitter has any recent activity
		for _, tx := range transmissions {
			if tx.TransmitterAddress == transmitterAddr {
				activeContracts = append(activeContracts, contract)
				uc.logger.Info("Found active contract for transmitter",
					"contract", contract.Hex(),
					"transmitter", transmitterAddr.Hex())
				break
			}
		}
	}

	return activeContracts, nil
}

// checkContractStatus checks the status of a transmitter on a specific contract.
func (uc *watchTransmittersBlockchainUseCase) checkContractStatus(
	ctx context.Context,
	contractAddr, transmitterAddr common.Address,
	roundsToCheck int,
	cutoffTime time.Time,
) entities.TransmitterStatus {
	status := entities.TransmitterStatus{
		Address:         transmitterAddr,
		JobID:           fmt.Sprintf("contract-%s", contractAddr.Hex()),
		ContractAddress: contractAddr,
		Status:          entities.JobStatusMissing,
	}

	// Get current block
	currentBlock, err := uc.blockchainClient.GetBlockNumber(ctx)
	if err != nil {
		status.Status = entities.JobStatusError
		status.Error = err
		return status
	}

	// Estimate blocks for rounds to check (assuming ~1 minute per round)
	blocksToCheck := uint64(roundsToCheck * 30) // ~30 blocks per round on Polygon
	if blocksToCheck > currentBlock {
		blocksToCheck = currentBlock
	}
	startBlock := currentBlock - blocksToCheck

	// Fetch transmissions
	transmissions, err := uc.aggregatorService.GetTransmissions(ctx, contractAddr, startBlock, currentBlock)
	if err != nil {
		status.Status = entities.JobStatusError
		status.Error = err
		return status
	}

	// Filter transmissions by this transmitter
	transmitterTransmissions := make([]entities.Transmission, 0)
	for _, tx := range transmissions {
		if tx.TransmitterAddress == transmitterAddr {
			transmitterTransmissions = append(transmitterTransmissions, tx)
		}
	}

	// Sort by timestamp
	sort.Slice(transmitterTransmissions, func(i, j int) bool {
		return transmitterTransmissions[i].BlockTimestamp.After(transmitterTransmissions[j].BlockTimestamp)
	})

	if len(transmitterTransmissions) == 0 {
		status.Status = entities.JobStatusMissing
		return status
	}

	// Get latest transmission
	latestTransmission := transmitterTransmissions[0]
	status.LastRound = latestTransmission.Epoch<<8 | uint32(latestTransmission.Round)
	status.LastTimestamp = latestTransmission.BlockTimestamp

	// Check if stale
	if latestTransmission.BlockTimestamp.Before(cutoffTime) {
		status.Status = entities.JobStatusStale
	} else {
		status.Status = entities.JobStatusFound
	}

	return status
}

// validateParams validates the watch parameters.
func (uc *watchTransmittersBlockchainUseCase) validateParams(params interfaces.WatchTransmittersParams) error {
	if params.TransmitterAddress == (common.Address{}) {
		return fmt.Errorf("transmitter address is required")
	}

	if params.RoundsToCheck <= 0 {
		return fmt.Errorf("rounds to check must be positive")
	}

	if params.RoundsToCheck > 100 {
		return fmt.Errorf("rounds to check must not exceed 100")
	}

	if params.DaysToIgnore < 0 {
		return fmt.Errorf("days to ignore cannot be negative")
	}

	return nil
}