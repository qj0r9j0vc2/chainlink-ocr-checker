// Package blockchain provides blockchain infrastructure implementations for the OCR checker application.
// It contains Ethereum client wrappers, OCR2 aggregator services, and transmission fetchers.
package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"chainlink-ocr-checker/domain/errors"
	"chainlink-ocr-checker/domain/interfaces"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ethereumClient implements the BlockchainClient interface.
type ethereumClient struct {
	client  *ethclient.Client
	chainID int64
}

// NewEthereumClient creates a new Ethereum client.
func NewEthereumClient(rpcURL string, chainID int64) (interfaces.BlockchainClient, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, &errors.BlockchainError{
			Operation: "Dial",
			ChainID:   chainID,
			Err:       err,
		}
	}

	// Verify chain ID.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	networkID, err := client.ChainID(ctx)
	if err != nil {
		client.Close()
		return nil, &errors.BlockchainError{
			Operation: "ChainID",
			ChainID:   chainID,
			Err:       err,
		}
	}

	if networkID.Int64() != chainID {
		client.Close()
		return nil, &errors.BlockchainError{
			Operation: "ChainID",
			ChainID:   chainID,
			Err:       fmt.Errorf("chain ID mismatch: expected %d, got %d", chainID, networkID.Int64()),
		}
	}

	return &ethereumClient{
		client:  client,
		chainID: chainID,
	}, nil
}

// GetBlockNumber returns the current block number.
func (c *ethereumClient) GetBlockNumber(ctx context.Context) (uint64, error) {
	blockNumber, err := c.client.BlockNumber(ctx)
	if err != nil {
		return 0, &errors.BlockchainError{
			Operation:   "GetBlockNumber",
			ChainID:     c.chainID,
			BlockNumber: 0,
			Err:         err,
		}
	}

	return blockNumber, nil
}

// GetBlockByNumber returns block information by block number.
func (c *ethereumClient) GetBlockByNumber(ctx context.Context, number *big.Int) (*interfaces.Block, error) {
	block, err := c.client.BlockByNumber(ctx, number)
	if err != nil {
		return nil, &errors.BlockchainError{
			Operation:   "GetBlockByNumber",
			ChainID:     c.chainID,
			BlockNumber: number.Uint64(),
			Err:         err,
		}
	}

	return &interfaces.Block{
		Number:    block.NumberU64(),
		Timestamp: time.Unix(int64(block.Time()), 0), // #nosec G115 -- block timestamp is always valid
		Hash:      block.Hash(),
	}, nil
}

// GetBlockByTimestamp returns the block number closest to the given timestamp.
func (c *ethereumClient) GetBlockByTimestamp(ctx context.Context, targetTime time.Time) (uint64, error) {
	// Get current block.
	currentBlock, err := c.client.BlockByNumber(ctx, nil)
	if err != nil {
		return 0, &errors.BlockchainError{
			Operation: "GetBlockByTimestamp.CurrentBlock",
			ChainID:   c.chainID,
			Err:       err,
		}
	}

	// Binary search for the target block.
	targetTimestamp := targetTime.Unix()
	low := uint64(0)
	high := currentBlock.NumberU64()

	// Estimate average block time (adjust based on chain).
	avgBlockTime := int64(12) // Ethereum mainnet average
	if c.chainID == 137 {
		avgBlockTime = 2 // Polygon
	}

	// Initial estimate.
	currentTime := int64(currentBlock.Time()) // #nosec G115 -- block timestamp is always valid
	timeDiff := currentTime - targetTimestamp
	blocksDiff := timeDiff / avgBlockTime

	estimatedBlock := int64(high) - blocksDiff // #nosec G115 -- high is always positive
	if estimatedBlock < 0 {
		estimatedBlock = 0
	}

	// Binary search with optimization.
	maxIterations := 50
	for i := 0; i < maxIterations && low <= high; i++ {
		var mid uint64
		if i == 0 && estimatedBlock > 0 {
			mid = uint64(estimatedBlock)
		} else {
			mid = (low + high) / 2
		}

		block, err := c.client.BlockByNumber(ctx, big.NewInt(int64(mid))) // #nosec G115 -- mid is always positive
		if err != nil {
			return 0, &errors.BlockchainError{
				Operation:   "GetBlockByTimestamp.Search",
				ChainID:     c.chainID,
				BlockNumber: mid,
				Err:         err,
			}
		}

		blockTime := int64(block.Time()) // #nosec G115 -- block timestamp is always valid

		switch {
		case blockTime == targetTimestamp:
			return mid, nil
		case blockTime < targetTimestamp:
			low = mid + 1
		default:
			high = mid - 1
		}

		// If we're close enough (within 1 block), return.
		if high-low <= 1 {
			// Return the block that's closest to target time.
			if targetTimestamp-blockTime < avgBlockTime {
				return mid, nil
			}
			return low, nil
		}
	}

	// Return the best estimate we found.
	return (low + high) / 2, nil
}

// Close closes the blockchain client connection.
func (c *ethereumClient) Close() error {
	c.client.Close()
	return nil
}
