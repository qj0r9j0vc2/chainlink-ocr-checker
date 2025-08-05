package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/domain/errors"
	"chainlink-ocr-checker/domain/interfaces"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/smartcontractkit/libocr/offchainreporting2plus/chains/evmutil"
	ocr2aggregator "github.com/smartcontractkit/libocr/offchainreporting2plus/chains/evmutil/contract"
)

// ocr2AggregatorService implements the OCR2AggregatorService interface
type ocr2AggregatorService struct {
	client  *ethclient.Client
	chainID int64
}

// NewOCR2AggregatorService creates a new OCR2 aggregator service
func NewOCR2AggregatorService(client *ethclient.Client, chainID int64) interfaces.OCR2AggregatorService {
	return &ocr2AggregatorService{
		client:  client,
		chainID: chainID,
	}
}

// GetLatestRound returns the latest round data
func (s *ocr2AggregatorService) GetLatestRound(ctx context.Context, contractAddress common.Address) (*entities.Round, error) {
	aggregator, err := ocr2aggregator.NewOffchainAggregator(contractAddress, s.client)
	if err != nil {
		return nil, &errors.BlockchainError{
			Operation: "GetLatestRound.NewAggregator",
			ChainID:   s.chainID,
			Err:       err,
		}
	}

	roundData, err := aggregator.LatestAnswer(&bind.CallOpts{Context: ctx})
	if err != nil {
		return nil, &errors.BlockchainError{
			Operation: "GetLatestRound.LatestAnswer",
			ChainID:   s.chainID,
			Err:       err,
		}
	}

	latestRound, err := aggregator.LatestRound(&bind.CallOpts{Context: ctx})
	if err != nil {
		return nil, &errors.BlockchainError{
			Operation: "GetLatestRound.LatestRound",
			ChainID:   s.chainID,
			Err:       err,
		}
	}

	timestamp, err := aggregator.LatestTimestamp(&bind.CallOpts{Context: ctx})
	if err != nil {
		return nil, &errors.BlockchainError{
			Operation: "GetLatestRound.LatestTimestamp",
			ChainID:   s.chainID,
			Err:       err,
		}
	}

	return &entities.Round{
		RoundID:   uint32(latestRound),
		Answer:    roundData,
		Timestamp: uint32(timestamp.Uint64()),
	}, nil
}

// GetRoundData returns data for a specific round
func (s *ocr2AggregatorService) GetRoundData(ctx context.Context, contractAddress common.Address, roundID uint32) (*entities.Round, error) {
	aggregator, err := ocr2aggregator.NewOffchainAggregator(contractAddress, s.client)
	if err != nil {
		return nil, &errors.BlockchainError{
			Operation: "GetRoundData.NewAggregator",
			ChainID:   s.chainID,
			Err:       err,
		}
	}

	// Note: This requires the aggregator to have getRoundData method
	// For now, we'll return an error as standard OCR2 doesn't expose historical round data
	return nil, &errors.BlockchainError{
		Operation: "GetRoundData",
		ChainID:   s.chainID,
		Err:       fmt.Errorf("historical round data not available in standard OCR2 aggregator"),
	}
}

// GetTransmissions returns transmission events for a block range
func (s *ocr2AggregatorService) GetTransmissions(ctx context.Context, contractAddress common.Address, startBlock, endBlock uint64) ([]entities.Transmission, error) {
	aggregator, err := ocr2aggregator.NewOffchainAggregator(contractAddress, s.client)
	if err != nil {
		return nil, &errors.BlockchainError{
			Operation:   "GetTransmissions.NewAggregator",
			ChainID:     s.chainID,
			BlockNumber: startBlock,
			Err:         err,
		}
	}

	// Create filter for transmitted events
	filterOpts := &bind.FilterOpts{
		Start:   startBlock,
		End:     &endBlock,
		Context: ctx,
	}

	iter, err := aggregator.FilterTransmitted(filterOpts, nil)
	if err != nil {
		return nil, &errors.BlockchainError{
			Operation:   "GetTransmissions.FilterTransmitted",
			ChainID:     s.chainID,
			BlockNumber: startBlock,
			Err:         err,
		}
	}
	defer iter.Close()

	var transmissions []entities.Transmission

	for iter.Next() {
		event := iter.Event

		// Get block information
		block, err := s.client.BlockByNumber(ctx, big.NewInt(int64(event.Raw.BlockNumber)))
		if err != nil {
			return nil, &errors.BlockchainError{
				Operation:   "GetTransmissions.BlockByNumber",
				ChainID:     s.chainID,
				BlockNumber: event.Raw.BlockNumber,
				Err:         err,
			}
		}

		// Map transmitter index to observer index
		observerIndex, err := s.getObserverIndex(ctx, contractAddress, event.Transmitter, event.Raw.BlockNumber)
		if err != nil {
			// Log error but continue processing
			observerIndex = 255 // Unknown
		}

		transmission := entities.Transmission{
			ContractAddress:    contractAddress,
			ConfigDigest:       event.ConfigDigest,
			Epoch:              event.Epoch,
			Round:              event.Round,
			LatestAnswer:       event.Answer,
			LatestTimestamp:    uint32(event.Timestamp.Uint64()),
			TransmitterIndex:   uint8(event.Transmitter.Big().Uint64() % 256), // Simplified mapping
			TransmitterAddress: event.Transmitter,
			ObserverIndex:      observerIndex,
			BlockNumber:        event.Raw.BlockNumber,
			BlockTimestamp:     time.Unix(int64(block.Time()), 0),
		}

		transmissions = append(transmissions, transmission)
	}

	if err := iter.Error(); err != nil {
		return nil, &errors.BlockchainError{
			Operation:   "GetTransmissions.Iterator",
			ChainID:     s.chainID,
			BlockNumber: startBlock,
			Err:         err,
		}
	}

	return transmissions, nil
}

// GetConfig returns the current OCR2 configuration
func (s *ocr2AggregatorService) GetConfig(ctx context.Context, contractAddress common.Address) (*entities.OCR2Config, error) {
	return s.GetConfigFromBlock(ctx, contractAddress, 0) // 0 means latest block
}

// GetConfigFromBlock returns the OCR2 configuration at a specific block
func (s *ocr2AggregatorService) GetConfigFromBlock(ctx context.Context, contractAddress common.Address, blockNumber uint64) (*entities.OCR2Config, error) {
	aggregator, err := ocr2aggregator.NewOffchainAggregator(contractAddress, s.client)
	if err != nil {
		return nil, &errors.BlockchainError{
			Operation:   "GetConfigFromBlock.NewAggregator",
			ChainID:     s.chainID,
			BlockNumber: blockNumber,
			Err:         err,
		}
	}

	var callOpts *bind.CallOpts
	if blockNumber > 0 {
		callOpts = &bind.CallOpts{
			Context:     ctx,
			BlockNumber: big.NewInt(int64(blockNumber)),
		}
	} else {
		callOpts = &bind.CallOpts{Context: ctx}
	}

	// Get latest config details
	configDetails, err := aggregator.LatestConfigDetails(callOpts)
	if err != nil {
		return nil, &errors.BlockchainError{
			Operation:   "GetConfigFromBlock.LatestConfigDetails",
			ChainID:     s.chainID,
			BlockNumber: blockNumber,
			Err:         err,
		}
	}

	// Get transmitters
	transmitters, err := aggregator.GetTransmitters(callOpts)
	if err != nil {
		return nil, &errors.BlockchainError{
			Operation:   "GetConfigFromBlock.GetTransmitters",
			ChainID:     s.chainID,
			BlockNumber: blockNumber,
			Err:         err,
		}
	}

	// Create OCR2Config
	config := &entities.OCR2Config{
		ConfigDigest: configDetails.ConfigDigest,
		Transmitters: transmitters,
		Threshold:    uint8(configDetails.F + 1), // f+1 is the threshold
	}

	// Try to get signers if available
	if configuredAggregator, ok := aggregator.(interface {
		GetSigners(opts *bind.CallOpts) ([]common.Address, error)
	}); ok {
		signers, err := configuredAggregator.GetSigners(callOpts)
		if err == nil {
			config.Signers = signers
		}
	}

	return config, nil
}

// getObserverIndex maps transmitter address to observer index
func (s *ocr2AggregatorService) getObserverIndex(ctx context.Context, contractAddress common.Address, transmitterAddr common.Address, blockNumber uint64) (uint8, error) {
	config, err := s.GetConfigFromBlock(ctx, contractAddress, blockNumber)
	if err != nil {
		return 0, err
	}

	for i, transmitter := range config.Transmitters {
		if transmitter == transmitterAddr {
			return uint8(i), nil
		}
	}

	return 0, fmt.Errorf("transmitter %s not found in config", transmitterAddr.Hex())
}
