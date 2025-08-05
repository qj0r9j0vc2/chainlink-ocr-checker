// Package blockchain provides blockchain infrastructure implementations for the OCR checker application.
// It contains Ethereum client wrappers, OCR2 aggregator services, and transmission fetchers.
package blockchain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/domain/errors"
	"chainlink-ocr-checker/domain/interfaces"
	"github.com/ethereum/go-ethereum/common"
)

const (
	maxConcurrency       = 30
	defaultBlockInterval = 5000 // Most RPC providers limit to 5000 blocks per request
)

// transmissionFetcher implements the TransmissionFetcher interface.
type transmissionFetcher struct {
	blockchainClient  interfaces.BlockchainClient
	aggregatorService interfaces.OCR2AggregatorService
	concurrency       int
}

// NewTransmissionFetcher creates a new transmission fetcher.
func NewTransmissionFetcher(
	blockchainClient interfaces.BlockchainClient,
	aggregatorService interfaces.OCR2AggregatorService,
) interfaces.TransmissionFetcher {
	return &transmissionFetcher{
		blockchainClient:  blockchainClient,
		aggregatorService: aggregatorService,
		concurrency:       maxConcurrency,
	}
}

// NewOptimizedTransmissionFetcher creates a new optimized transmission fetcher.
// This should be used in production environments.
func NewOptimizedTransmissionFetcher(
	blockchainClient interfaces.BlockchainClient,
	aggregatorService interfaces.OCR2AggregatorService,
	logger interfaces.Logger,
) interfaces.TransmissionFetcher {
	return NewTransmissionFetcherOptimized(blockchainClient, aggregatorService, logger)
}

// FetchByRounds fetches transmissions for a range of rounds.
func (f *transmissionFetcher) FetchByRounds(
	ctx context.Context,
	contractAddress common.Address,
	startRound, endRound uint32,
) (*entities.TransmissionResult, error) {
	if startRound > endRound {
		return nil, errors.NewDomainError(errors.ErrInvalidInput,
			fmt.Sprintf("invalid round range: start=%d, end=%d", startRound, endRound))
	}

	// Get current block number.
	currentBlock, err := f.blockchainClient.GetBlockNumber(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch all transmissions from genesis to current block.
	// This is a simplified approach - in production, we'd optimize this.
	transmissions, err := f.fetchTransmissionsInRange(ctx, contractAddress, 0, currentBlock)
	if err != nil {
		return nil, err
	}

	// Filter by round range.
	var filteredTransmissions []entities.Transmission
	for _, tx := range transmissions {
		roundID := tx.Epoch<<8 | uint32(tx.Round)
		if roundID >= startRound && roundID <= endRound {
			filteredTransmissions = append(filteredTransmissions, tx)
		}
	}

	return &entities.TransmissionResult{
		ContractAddress: contractAddress,
		StartRound:      startRound,
		EndRound:        endRound,
		Transmissions:   filteredTransmissions,
	}, nil
}

// FetchByBlocks fetches transmissions for a range of blocks.
func (f *transmissionFetcher) FetchByBlocks(
	ctx context.Context,
	contractAddress common.Address,
	startBlock, endBlock uint64,
) (*entities.TransmissionResult, error) {
	if startBlock > endBlock {
		return nil, errors.NewDomainError(errors.ErrInvalidInput,
			fmt.Sprintf("invalid block range: start=%d, end=%d", startBlock, endBlock))
	}

	transmissions, err := f.fetchTransmissionsInRange(ctx, contractAddress, startBlock, endBlock)
	if err != nil {
		return nil, err
	}

	// Calculate round range from transmissions.
	var startRound, endRound uint32
	if len(transmissions) > 0 {
		startRound = transmissions[0].Epoch<<8 | uint32(transmissions[0].Round)
		endRound = transmissions[len(transmissions)-1].Epoch<<8 | uint32(transmissions[len(transmissions)-1].Round)
	}

	return &entities.TransmissionResult{
		ContractAddress: contractAddress,
		StartRound:      startRound,
		EndRound:        endRound,
		Transmissions:   transmissions,
	}, nil
}

// FetchByTimeRange fetches transmissions for a time range.
func (f *transmissionFetcher) FetchByTimeRange(
	ctx context.Context,
	contractAddress common.Address,
	startTime, endTime time.Time,
) (*entities.TransmissionResult, error) {
	if startTime.After(endTime) {
		return nil, errors.NewDomainError(errors.ErrInvalidInput,
			fmt.Sprintf("invalid time range: start=%v, end=%v", startTime, endTime))
	}

	// Find block numbers for the time range.
	startBlock, err := f.blockchainClient.GetBlockByTimestamp(ctx, startTime)
	if err != nil {
		return nil, err
	}

	endBlock, err := f.blockchainClient.GetBlockByTimestamp(ctx, endTime)
	if err != nil {
		return nil, err
	}

	return f.FetchByBlocks(ctx, contractAddress, startBlock, endBlock)
}

// fetchTransmissionsInRange fetches transmissions in parallel for a block range.
func (f *transmissionFetcher) fetchTransmissionsInRange(
	ctx context.Context,
	contractAddress common.Address,
	startBlock, endBlock uint64,
) ([]entities.Transmission, error) {
	// Split the range into chunks.
	chunks := f.splitBlockRange(startBlock, endBlock)

	// Create channels for results and errors.
	resultsChan := make(chan []entities.Transmission, len(chunks))
	errorsChan := make(chan error, len(chunks))

	// Use semaphore to limit concurrency.
	sem := make(chan struct{}, f.concurrency)

	var wg sync.WaitGroup
	wg.Add(len(chunks))

	// Fetch chunks in parallel.
	for _, chunk := range chunks {
		go func(start, end uint64) {
			defer wg.Done()

			// Acquire semaphore.
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check context cancellation.
			select {
			case <-ctx.Done():
				errorsChan <- ctx.Err()
				return
			default:
			}

			// Fetch transmissions for this chunk.
			transmissions, err := f.aggregatorService.GetTransmissions(ctx, contractAddress, start, end)
			if err != nil {
				errorsChan <- err
				return
			}

			resultsChan <- transmissions
		}(chunk.StartBlock, chunk.EndBlock)
	}

	// Wait for all goroutines to complete.
	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	// Check for errors.
	for err := range errorsChan {
		if err != nil {
			return nil, err
		}
	}

	// Collect all results.
	var allTransmissions []entities.Transmission
	for transmissions := range resultsChan {
		allTransmissions = append(allTransmissions, transmissions...)
	}

	return allTransmissions, nil
}

// splitBlockRange splits a block range into smaller chunks.
func (f *transmissionFetcher) splitBlockRange(startBlock, endBlock uint64) []entities.BlockRange {
	var chunks []entities.BlockRange

	totalBlocks := endBlock - startBlock + 1
	chunkSize := uint64(defaultBlockInterval)

	// Adjust chunk size based on total range.
	// #nosec G115 -- concurrency is always positive
	if totalBlocks < uint64(f.concurrency)*chunkSize {
		// #nosec G115 -- concurrency is always positive
		chunkSize = totalBlocks / uint64(f.concurrency)
		if chunkSize < 1 {
			chunkSize = 1
		}
	}

	for start := startBlock; start <= endBlock; start += chunkSize {
		end := start + chunkSize - 1
		if end > endBlock {
			end = endBlock
		}

		chunks = append(chunks, entities.BlockRange{
			StartBlock: start,
			EndBlock:   end,
		})
	}

	return chunks
}
