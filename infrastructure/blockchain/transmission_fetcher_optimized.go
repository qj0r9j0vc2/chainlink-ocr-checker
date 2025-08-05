// Package blockchain provides blockchain infrastructure implementations for the OCR checker application.
package blockchain

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/domain/errors"
	"chainlink-ocr-checker/domain/interfaces"
	"github.com/ethereum/go-ethereum/common"
)

const (
	maxRetries      = 3
	retryDelay      = time.Second
	cacheExpiration = 5 * time.Minute
)

// roundBlockCache caches round to block mappings
type roundBlockCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry // key: contractAddress-roundID
}

type cacheEntry struct {
	blockNumber uint64
	timestamp   time.Time
}

// transmissionFetcherOptimized implements an optimized TransmissionFetcher interface.
type transmissionFetcherOptimized struct {
	blockchainClient  interfaces.BlockchainClient
	aggregatorService interfaces.OCR2AggregatorService
	concurrency       int
	cache             *roundBlockCache
	logger            interfaces.Logger
}

// NewTransmissionFetcherOptimized creates a new optimized transmission fetcher.
func NewTransmissionFetcherOptimized(
	blockchainClient interfaces.BlockchainClient,
	aggregatorService interfaces.OCR2AggregatorService,
	logger interfaces.Logger,
) interfaces.TransmissionFetcher {
	return &transmissionFetcherOptimized{
		blockchainClient:  blockchainClient,
		aggregatorService: aggregatorService,
		concurrency:       maxConcurrency,
		cache: &roundBlockCache{
			entries: make(map[string]*cacheEntry),
		},
		logger: logger,
	}
}

// FetchByRounds fetches transmissions for a range of rounds using optimized approach.
func (f *transmissionFetcherOptimized) FetchByRounds(
	ctx context.Context,
	contractAddress common.Address,
	startRound, endRound uint32,
) (*entities.TransmissionResult, error) {
	if startRound > endRound {
		return nil, errors.NewDomainError(errors.ErrInvalidInput,
			fmt.Sprintf("invalid round range: start=%d, end=%d", startRound, endRound))
	}

	f.logger.Info("Fetching transmissions by rounds",
		"contract", contractAddress.Hex(),
		"startRound", startRound,
		"endRound", endRound)

	// Find block range for the rounds using binary search
	startBlock, err := f.findBlockForRound(ctx, contractAddress, startRound, true)
	if err != nil {
		return nil, fmt.Errorf("failed to find start block for round %d: %w", startRound, err)
	}

	endBlock, err := f.findBlockForRound(ctx, contractAddress, endRound, false)
	if err != nil {
		return nil, fmt.Errorf("failed to find end block for round %d: %w", endRound, err)
	}

	f.logger.Info("Found block range for rounds",
		"startBlock", startBlock,
		"endBlock", endBlock)

	// Fetch transmissions in the block range
	transmissions, err := f.fetchTransmissionsInRangeWithRetry(ctx, contractAddress, startBlock, endBlock)
	if err != nil {
		return nil, err
	}

	// Filter by exact round range (in case block boundaries don't align perfectly)
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

// findBlockForRound uses binary search to find the block containing a specific round
func (f *transmissionFetcherOptimized) findBlockForRound(
	ctx context.Context,
	contractAddress common.Address,
	targetRound uint32,
	isStartRound bool,
) (uint64, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s-%d", contractAddress.Hex(), targetRound)
	if block := f.getFromCache(cacheKey); block > 0 {
		f.logger.Debug("Found block in cache", "round", targetRound, "block", block)
		return block, nil
	}

	// Get current block as upper bound
	currentBlock, err := f.blockchainClient.GetBlockNumber(ctx)
	if err != nil {
		return 0, err
	}

	// Binary search for the block
	left := uint64(0)
	right := currentBlock
	var resultBlock uint64

	// First, try to get a sample transmission to estimate block range
	sampleBlock := f.estimateBlockForRound(ctx, contractAddress, targetRound, currentBlock)
	if sampleBlock > 0 {
		// Narrow the search range based on estimate
		margin := uint64(100000) // 100k blocks margin
		if sampleBlock > margin {
			left = sampleBlock - margin
		}
		if sampleBlock+margin < right {
			right = sampleBlock + margin
		}
	}

	f.logger.Debug("Starting binary search",
		"targetRound", targetRound,
		"leftBlock", left,
		"rightBlock", right)

	for left <= right {
		mid := (left + right) / 2

		// Fetch a small range around mid to check rounds
		searchStart := mid
		searchEnd := mid + 1000 // Check 1000 blocks at a time
		if searchEnd > currentBlock {
			searchEnd = currentBlock
		}

		transmissions, err := f.aggregatorService.GetTransmissions(ctx, contractAddress, searchStart, searchEnd)
		if err != nil {
			// If error, try a smaller range
			searchEnd = searchStart + 100
			transmissions, err = f.aggregatorService.GetTransmissions(ctx, contractAddress, searchStart, searchEnd)
			if err != nil {
				return 0, fmt.Errorf("failed to fetch transmissions at block %d: %w", mid, err)
			}
		}

		if len(transmissions) == 0 {
			// No transmissions in this range, binary search
			if isStartRound {
				left = mid + 1
			} else {
				right = mid - 1
			}
			continue
		}

		// Check rounds in transmissions
		minRound := uint32(math.MaxUint32)
		maxRound := uint32(0)
		for _, tx := range transmissions {
			roundID := tx.Epoch<<8 | uint32(tx.Round)
			if roundID < minRound {
				minRound = roundID
			}
			if roundID > maxRound {
				maxRound = roundID
			}
		}

		if targetRound >= minRound && targetRound <= maxRound {
			// Found the target round in this range
			for _, tx := range transmissions {
				roundID := tx.Epoch<<8 | uint32(tx.Round)
				if roundID == targetRound {
					resultBlock = tx.BlockNumber
					f.putToCache(cacheKey, resultBlock)
					return resultBlock, nil
				}
			}
		}

		// Adjust search range based on rounds found
		if targetRound < minRound {
			right = searchStart - 1
		} else {
			left = searchEnd + 1
			// Keep track of the closest block for the round
			if isStartRound && maxRound < targetRound {
				resultBlock = searchEnd
			} else if !isStartRound && minRound > targetRound {
				resultBlock = searchStart
			}
		}
	}

	// If exact round not found, return the closest block
	if resultBlock > 0 {
		f.logger.Warn("Exact round not found, using closest block",
			"targetRound", targetRound,
			"block", resultBlock)
		return resultBlock, nil
	}

	return 0, fmt.Errorf("could not find block for round %d", targetRound)
}

// estimateBlockForRound estimates the block number for a round based on sampling
func (f *transmissionFetcherOptimized) estimateBlockForRound(
	ctx context.Context,
	contractAddress common.Address,
	targetRound uint32,
	currentBlock uint64,
) uint64 {
	// Try to get some recent transmissions to estimate round progression
	sampleSize := uint64(10000)
	if currentBlock < sampleSize {
		sampleSize = currentBlock
	}

	sampleStart := currentBlock - sampleSize
	transmissions, err := f.aggregatorService.GetTransmissions(ctx, contractAddress, sampleStart, currentBlock)
	if err != nil || len(transmissions) < 2 {
		return 0
	}

	// Calculate average blocks per round
	firstTx := transmissions[0]
	lastTx := transmissions[len(transmissions)-1]
	
	firstRound := firstTx.Epoch<<8 | uint32(firstTx.Round)
	lastRound := lastTx.Epoch<<8 | uint32(lastTx.Round)
	
	if lastRound <= firstRound {
		return 0
	}

	blocksPerRound := float64(lastTx.BlockNumber-firstTx.BlockNumber) / float64(lastRound-firstRound)
	
	// Estimate block for target round
	roundDiff := int64(targetRound) - int64(lastRound)
	estimatedBlock := int64(lastTx.BlockNumber) + int64(blocksPerRound*float64(roundDiff))
	
	if estimatedBlock < 0 {
		return 0
	}
	if uint64(estimatedBlock) > currentBlock {
		return currentBlock
	}
	
	return uint64(estimatedBlock)
}

// fetchTransmissionsInRangeWithRetry fetches transmissions with retry logic
func (f *transmissionFetcherOptimized) fetchTransmissionsInRangeWithRetry(
	ctx context.Context,
	contractAddress common.Address,
	startBlock, endBlock uint64,
) ([]entities.Transmission, error) {
	var lastErr error
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := time.Duration(math.Pow(2, float64(attempt))) * retryDelay
			f.logger.Debug("Retrying after delay",
				"attempt", attempt+1,
				"delay", delay)
			
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		transmissions, err := f.fetchTransmissionsInRange(ctx, contractAddress, startBlock, endBlock)
		if err == nil {
			return transmissions, nil
		}

		lastErr = err
		f.logger.Warn("Failed to fetch transmissions, will retry",
			"attempt", attempt+1,
			"error", err)
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// FetchByBlocks fetches transmissions for a range of blocks.
func (f *transmissionFetcherOptimized) FetchByBlocks(
	ctx context.Context,
	contractAddress common.Address,
	startBlock, endBlock uint64,
) (*entities.TransmissionResult, error) {
	if startBlock > endBlock {
		return nil, errors.NewDomainError(errors.ErrInvalidInput,
			fmt.Sprintf("invalid block range: start=%d, end=%d", startBlock, endBlock))
	}

	transmissions, err := f.fetchTransmissionsInRangeWithRetry(ctx, contractAddress, startBlock, endBlock)
	if err != nil {
		return nil, err
	}

	// Sort transmissions by block number to ensure correct round range calculation
	sort.Slice(transmissions, func(i, j int) bool {
		return transmissions[i].BlockNumber < transmissions[j].BlockNumber
	})

	// Calculate round range from transmissions
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
func (f *transmissionFetcherOptimized) FetchByTimeRange(
	ctx context.Context,
	contractAddress common.Address,
	startTime, endTime time.Time,
) (*entities.TransmissionResult, error) {
	if startTime.After(endTime) {
		return nil, errors.NewDomainError(errors.ErrInvalidInput,
			fmt.Sprintf("invalid time range: start=%v, end=%v", startTime, endTime))
	}

	// Find block numbers for the time range
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

// fetchTransmissionsInRange fetches transmissions in parallel for a block range
func (f *transmissionFetcherOptimized) fetchTransmissionsInRange(
	ctx context.Context,
	contractAddress common.Address,
	startBlock, endBlock uint64,
) ([]entities.Transmission, error) {
	// Split the range into optimal chunks
	chunks := f.splitBlockRangeOptimized(startBlock, endBlock)
	
	f.logger.Debug("Split block range into chunks",
		"totalBlocks", endBlock-startBlock+1,
		"chunks", len(chunks))

	// Create channels for results
	type chunkResult struct {
		transmissions []entities.Transmission
		chunkIndex    int
	}
	
	resultsChan := make(chan chunkResult, len(chunks))
	errorsChan := make(chan error, len(chunks))

	// Use semaphore to limit concurrency
	sem := make(chan struct{}, f.concurrency)

	var wg sync.WaitGroup
	wg.Add(len(chunks))

	// Fetch chunks in parallel
	for i, chunk := range chunks {
		go func(index int, start, end uint64) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errorsChan <- ctx.Err()
				return
			}

			// Check context cancellation
			select {
			case <-ctx.Done():
				errorsChan <- ctx.Err()
				return
			default:
			}

			// Fetch transmissions for this chunk
			transmissions, err := f.aggregatorService.GetTransmissions(ctx, contractAddress, start, end)
			if err != nil {
				errorsChan <- fmt.Errorf("failed to fetch chunk %d-%d: %w", start, end, err)
				return
			}

			resultsChan <- chunkResult{
				transmissions: transmissions,
				chunkIndex:    index,
			}
		}(i, chunk.StartBlock, chunk.EndBlock)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	// Check for errors
	for err := range errorsChan {
		if err != nil {
			return nil, err
		}
	}

	// Collect and sort results by chunk index to maintain order
	results := make([]chunkResult, 0, len(chunks))
	for result := range resultsChan {
		results = append(results, result)
	}
	
	sort.Slice(results, func(i, j int) bool {
		return results[i].chunkIndex < results[j].chunkIndex
	})

	// Merge all transmissions
	var allTransmissions []entities.Transmission
	for _, result := range results {
		allTransmissions = append(allTransmissions, result.transmissions...)
	}

	f.logger.Debug("Fetched transmissions",
		"total", len(allTransmissions),
		"chunks", len(chunks))

	return allTransmissions, nil
}

// splitBlockRangeOptimized splits a block range into optimal chunks
func (f *transmissionFetcherOptimized) splitBlockRangeOptimized(startBlock, endBlock uint64) []entities.BlockRange {
	var chunks []entities.BlockRange

	totalBlocks := endBlock - startBlock + 1
	optimalChunkSize := uint64(defaultBlockInterval)

	// Calculate optimal number of chunks based on concurrency and total blocks
	optimalChunks := int(math.Ceil(float64(totalBlocks) / float64(optimalChunkSize)))
	
	// Adjust if we have more chunks than concurrency allows
	if optimalChunks > f.concurrency {
		// Recalculate chunk size to fit within concurrency limit
		optimalChunkSize = uint64(math.Ceil(float64(totalBlocks) / float64(f.concurrency)))
	}

	// Ensure chunk size doesn't exceed RPC limit
	if optimalChunkSize > defaultBlockInterval {
		optimalChunkSize = defaultBlockInterval
	}

	// Create chunks
	for start := startBlock; start <= endBlock; start += optimalChunkSize {
		end := start + optimalChunkSize - 1
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

// Cache management methods
func (f *transmissionFetcherOptimized) getFromCache(key string) uint64 {
	f.cache.mu.RLock()
	defer f.cache.mu.RUnlock()

	entry, exists := f.cache.entries[key]
	if !exists {
		return 0
	}

	// Check if cache entry is still valid
	if time.Since(entry.timestamp) > cacheExpiration {
		return 0
	}

	return entry.blockNumber
}

func (f *transmissionFetcherOptimized) putToCache(key string, blockNumber uint64) {
	f.cache.mu.Lock()
	defer f.cache.mu.Unlock()

	f.cache.entries[key] = &cacheEntry{
		blockNumber: blockNumber,
		timestamp:   time.Now(),
	}

	// Clean up old entries periodically
	if len(f.cache.entries) > 1000 {
		f.cleanupCache()
	}
}

func (f *transmissionFetcherOptimized) cleanupCache() {
	now := time.Now()
	for key, entry := range f.cache.entries {
		if now.Sub(entry.timestamp) > cacheExpiration {
			delete(f.cache.entries, key)
		}
	}
}