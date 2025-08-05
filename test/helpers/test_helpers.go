package helpers

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// TestContext creates a test context with timeout
func TestContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// RandomAddress generates a random Ethereum address for testing
func RandomAddress() common.Address {
	return common.HexToAddress("0x" + RandomHex(40))
}

// RandomHex generates a random hex string of the specified length
func RandomHex(length int) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, length)
	for i := range result {
		result[i] = hexChars[time.Now().UnixNano()%int64(len(hexChars))]
	}
	return string(result)
}

// RandomHash generates a random hash for testing
func RandomHash() common.Hash {
	return common.HexToHash("0x" + RandomHex(64))
}

// RandomBigInt generates a random big.Int for testing
func RandomBigInt(max int64) *big.Int {
	return big.NewInt(time.Now().UnixNano() % max)
}

// AssertEventually asserts that a condition is met within a timeout
func AssertEventually(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.Fail(t, message)
}

// TestFixture represents a test fixture
type TestFixture struct {
	t *testing.T
}

// NewTestFixture creates a new test fixture
func NewTestFixture(t *testing.T) *TestFixture {
	return &TestFixture{t: t}
}

// Cleanup registers a cleanup function
func (f *TestFixture) Cleanup(fn func()) {
	f.t.Cleanup(fn)
}

// SkipIfShort skips the test if running in short mode
func SkipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}
}

// SkipIfNoDatabase skips the test if database is not available
func SkipIfNoDatabase(t *testing.T) {
	// This can be enhanced to actually check database connectivity
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}
}

// SkipIfNoBlockchain skips the test if blockchain is not available
func SkipIfNoBlockchain(t *testing.T) {
	// This can be enhanced to actually check blockchain connectivity
	if testing.Short() {
		t.Skip("Skipping blockchain test in short mode")
	}
}