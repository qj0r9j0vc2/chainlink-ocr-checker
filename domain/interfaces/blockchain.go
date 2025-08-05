// Package interfaces defines contracts and interfaces for the OCR checker domain layer.
// It contains interfaces for blockchain operations, repositories, use cases, and logging.
package interfaces

import (
	"context"
	"math/big"
	"time"

	"chainlink-ocr-checker/domain/entities"
	"github.com/ethereum/go-ethereum/common"
)

// BlockchainClient represents the interface for blockchain operations.
type BlockchainClient interface {
	// GetBlockNumber returns the current block number.
	GetBlockNumber(ctx context.Context) (uint64, error)

	// GetBlockByNumber returns block information by block number.
	GetBlockByNumber(ctx context.Context, number *big.Int) (*Block, error)

	// GetBlockByTimestamp returns the block number closest to the given timestamp.
	GetBlockByTimestamp(ctx context.Context, timestamp time.Time) (uint64, error)

	// Close closes the blockchain client connection.
	Close() error
}

// Block represents a blockchain block.
type Block struct {
	Number    uint64
	Timestamp time.Time
	Hash      common.Hash
}

// OCR2AggregatorService handles OCR2 aggregator contract interactions.
type OCR2AggregatorService interface {
	// GetLatestRound returns the latest round data.
	GetLatestRound(ctx context.Context, contractAddress common.Address) (*entities.Round, error)

	// GetRoundData returns data for a specific round.
	GetRoundData(ctx context.Context, contractAddress common.Address, roundID uint32) (*entities.Round, error)

	// GetTransmissions returns transmission events for a block range.
	GetTransmissions(
		ctx context.Context,
		contractAddress common.Address,
		startBlock, endBlock uint64,
	) ([]entities.Transmission, error)

	// GetConfig returns the current OCR2 configuration.
	GetConfig(ctx context.Context, contractAddress common.Address) (*entities.OCR2Config, error)

	// GetConfigFromBlock returns the OCR2 configuration at a specific block.
	GetConfigFromBlock(
		ctx context.Context,
		contractAddress common.Address,
		blockNumber uint64,
	) (*entities.OCR2Config, error)
}

// TransmissionFetcher handles fetching transmission data.
type TransmissionFetcher interface {
	// FetchByRounds fetches transmissions for a range of rounds.
	FetchByRounds(
		ctx context.Context,
		contractAddress common.Address,
		startRound, endRound uint32,
	) (*entities.TransmissionResult, error)

	// FetchByBlocks fetches transmissions for a range of blocks.
	FetchByBlocks(
		ctx context.Context,
		contractAddress common.Address,
		startBlock, endBlock uint64,
	) (*entities.TransmissionResult, error)

	// FetchByTimeRange fetches transmissions for a time range.
	FetchByTimeRange(
		ctx context.Context,
		contractAddress common.Address,
		startTime, endTime time.Time,
	) (*entities.TransmissionResult, error)
}

// TransmissionWatcher monitors transmissions in real-time.
type TransmissionWatcher interface {
	// WatchTransmissions monitors transmissions for multiple contracts.
	WatchTransmissions(ctx context.Context, contracts []common.Address, callback TransmissionCallback) error

	// GetLatestTransmissions returns the latest transmissions for a transmitter.
	GetLatestTransmissions(
		ctx context.Context,
		transmitterAddress common.Address,
		limit int,
	) ([]entities.TransmitterStatus, error)
}

// TransmissionCallback is called when new transmissions are detected.
type TransmissionCallback func(transmission entities.Transmission) error
