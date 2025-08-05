package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"chainlink-ocr-checker/domain/interfaces"
	"chainlink-ocr-checker/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEthereumClient_GetBlockNumber(t *testing.T) {
	// Skip if no blockchain connection.
	helpers.SkipIfNoBlockchain(t)

	ctx := helpers.TestContext(t)

	// This would use a test RPC endpoint in a real test.
	client, err := NewEthereumClient("https://eth-mainnet.g.alchemy.com/v2/demo", 1)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	blockNumber, err := client.GetBlockNumber(ctx)
	require.NoError(t, err)
	assert.Greater(t, blockNumber, uint64(0))
}

func TestEthereumClient_GetBlockByNumber(t *testing.T) {
	// Skip if no blockchain connection.
	helpers.SkipIfNoBlockchain(t)

	ctx := helpers.TestContext(t)

	client, err := NewEthereumClient("https://eth-mainnet.g.alchemy.com/v2/demo", 1)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Get genesis block.
	block, err := client.GetBlockByNumber(ctx, big.NewInt(0))
	require.NoError(t, err)
	assert.Equal(t, uint64(0), block.Number)
	assert.NotZero(t, block.Timestamp)
	assert.NotEmpty(t, block.Hash)
}

func TestEthereumClient_GetBlockByTimestamp(t *testing.T) {
	// Skip if no blockchain connection.
	helpers.SkipIfNoBlockchain(t)

	ctx := helpers.TestContext(t)

	client, err := NewEthereumClient("https://eth-mainnet.g.alchemy.com/v2/demo", 1)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Test with a known timestamp (approximately block 15537393).
	targetTime := time.Date(2022, 9, 15, 6, 42, 42, 0, time.UTC)

	blockNumber, err := client.GetBlockByTimestamp(ctx, targetTime)
	require.NoError(t, err)

	// Verify the block is close to our target.
	block, err := client.GetBlockByNumber(ctx, big.NewInt(int64(blockNumber))) // #nosec G115 -- test value
	require.NoError(t, err)

	timeDiff := block.Timestamp.Sub(targetTime).Abs()
	assert.Less(t, timeDiff, 5*time.Minute, "Block timestamp should be within 5 minutes of target")
}

// MockEthereumClient for unit testing.
type MockEthereumClient struct {
	blockNumber uint64
	blocks      map[uint64]*interfaces.Block
	err         error
}

func (m *MockEthereumClient) GetBlockNumber(_ context.Context) (uint64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.blockNumber, nil
}

func (m *MockEthereumClient) GetBlockByNumber(_ context.Context, number *big.Int) (*interfaces.Block, error) {
	if m.err != nil {
		return nil, m.err
	}

	block, ok := m.blocks[number.Uint64()]
	if !ok {
		return nil, fmt.Errorf("block not found")
	}

	return block, nil
}

func (m *MockEthereumClient) GetBlockByTimestamp(_ context.Context, targetTime time.Time) (uint64, error) {
	if m.err != nil {
		return 0, m.err
	}

	// Simple mock implementation.
	for blockNum, block := range m.blocks {
		if block.Timestamp.Equal(targetTime) || block.Timestamp.After(targetTime) {
			return blockNum, nil
		}
	}

	return 0, fmt.Errorf("no block found for timestamp")
}

func (m *MockEthereumClient) Close() error {
	return nil
}
