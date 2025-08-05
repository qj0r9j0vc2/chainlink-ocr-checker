package blockchain

import (
	"context"
	"testing"
	"time"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/test/mocks"
	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransmissionFetcherOptimized_FetchByRounds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockBlockchainClient(ctrl)
	mockAggregator := mocks.NewMockOCR2AggregatorService(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)

	// Set up logger expectations
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()

	fetcher := NewTransmissionFetcherOptimized(mockClient, mockAggregator, mockLogger)
	ctx := context.Background()
	contractAddr := common.HexToAddress("0x1234567890abcdef")

	t.Run("successful fetch with binary search", func(t *testing.T) {
		startRound := uint32(100)
		endRound := uint32(105)

		// Mock current block
		mockClient.EXPECT().GetBlockNumber(ctx).Return(uint64(1000000), nil).Times(2)

		// Mock transmissions for binary search
		// First search for start round
		mockAggregator.EXPECT().
			GetTransmissions(ctx, contractAddr, gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, addr common.Address, start, end uint64) ([]entities.Transmission, error) {
				// Return transmissions based on block range
				if start >= 500000 && start <= 600000 {
					return []entities.Transmission{
						{
							Epoch:       0,
							Round:       100,
							BlockNumber: 550000,
						},
					}, nil
				}
				return []entities.Transmission{}, nil
			}).AnyTimes()

		// Mock final fetch
		mockAggregator.EXPECT().
			GetTransmissions(ctx, contractAddr, uint64(550000), gomock.Any()).
			Return([]entities.Transmission{
				{
					Epoch:       0,
					Round:       100,
					BlockNumber: 550000,
				},
				{
					Epoch:       0,
					Round:       102,
					BlockNumber: 551000,
				},
				{
					Epoch:       0,
					Round:       105,
					BlockNumber: 552000,
				},
			}, nil).Times(1)

		result, err := fetcher.FetchByRounds(ctx, contractAddr, startRound, endRound)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, contractAddr, result.ContractAddress)
		assert.Equal(t, startRound, result.StartRound)
		assert.Equal(t, endRound, result.EndRound)
		assert.Len(t, result.Transmissions, 3)
	})

	t.Run("invalid round range", func(t *testing.T) {
		result, err := fetcher.FetchByRounds(ctx, contractAddr, 200, 100)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid round range")
	})
}

func TestTransmissionFetcherOptimized_FetchByBlocks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockBlockchainClient(ctrl)
	mockAggregator := mocks.NewMockOCR2AggregatorService(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)

	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	fetcher := NewTransmissionFetcherOptimized(mockClient, mockAggregator, mockLogger)
	ctx := context.Background()
	contractAddr := common.HexToAddress("0x1234567890abcdef")

	t.Run("successful fetch with retry", func(t *testing.T) {
		startBlock := uint64(1000)
		endBlock := uint64(2000)

		// First attempt fails
		mockAggregator.EXPECT().
			GetTransmissions(ctx, contractAddr, startBlock, endBlock).
			Return(nil, assert.AnError).Times(1)

		// Set up logger for retry
		mockLogger.EXPECT().
			Warn("Failed to fetch transmissions, will retry", gomock.Any()).Times(1)

		// Second attempt succeeds
		mockAggregator.EXPECT().
			GetTransmissions(ctx, contractAddr, startBlock, endBlock).
			Return([]entities.Transmission{
				{
					Epoch:       1,
					Round:       10,
					BlockNumber: 1500,
				},
			}, nil).Times(1)

		result, err := fetcher.FetchByBlocks(ctx, contractAddr, startBlock, endBlock)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Transmissions, 1)
	})
}

func TestTransmissionFetcherOptimized_Cache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockBlockchainClient(ctrl)
	mockAggregator := mocks.NewMockOCR2AggregatorService(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)

	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	fetcherImpl := &transmissionFetcherOptimized{
		blockchainClient:  mockClient,
		aggregatorService: mockAggregator,
		concurrency:       10,
		cache: &roundBlockCache{
			entries: make(map[string]*cacheEntry),
		},
		logger: mockLogger,
	}

	// Test cache operations
	key := "0x1234-100"
	blockNumber := uint64(12345)

	// Initially cache should be empty
	assert.Equal(t, uint64(0), fetcherImpl.getFromCache(key))

	// Put to cache
	fetcherImpl.putToCache(key, blockNumber)

	// Should retrieve from cache
	assert.Equal(t, blockNumber, fetcherImpl.getFromCache(key))

	// Test cache expiration
	fetcherImpl.cache.entries[key].timestamp = time.Now().Add(-10 * time.Minute)
	assert.Equal(t, uint64(0), fetcherImpl.getFromCache(key))
}

func TestTransmissionFetcherOptimized_SplitBlockRange(t *testing.T) {
	fetcherImpl := &transmissionFetcherOptimized{
		concurrency: 10,
	}

	tests := []struct {
		name       string
		startBlock uint64
		endBlock   uint64
		expected   int
	}{
		{
			name:       "small range",
			startBlock: 1000,
			endBlock:   2000,
			expected:   1,
		},
		{
			name:       "exact chunk size",
			startBlock: 0,
			endBlock:   4999,
			expected:   1,
		},
		{
			name:       "multiple chunks",
			startBlock: 0,
			endBlock:   15000,
			expected:   4, // 0-4999, 5000-9999, 10000-14999, 15000-15000
		},
		{
			name:       "large range",
			startBlock: 0,
			endBlock:   100000,
			expected:   10, // Limited by concurrency
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := fetcherImpl.splitBlockRangeOptimized(tt.startBlock, tt.endBlock)
			assert.Len(t, chunks, tt.expected)

			// Verify chunks cover the entire range
			if len(chunks) > 0 {
				assert.Equal(t, tt.startBlock, chunks[0].StartBlock)
				assert.Equal(t, tt.endBlock, chunks[len(chunks)-1].EndBlock)
			}

			// Verify no gaps between chunks
			for i := 1; i < len(chunks); i++ {
				assert.Equal(t, chunks[i-1].EndBlock+1, chunks[i].StartBlock)
			}
		})
	}
}