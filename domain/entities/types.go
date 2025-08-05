// Package entities contains the core domain entities for the OCR checker application.
// It defines structures for jobs, transmissions, and related data types.
package entities

import (
	"database/sql/driver"
	"fmt"
	"math/big"
)

// BigInt is a wrapper for *big.Int that implements Scanner and Valuer.
type BigInt struct {
	*big.Int
}

// Scan implements the sql.Scanner interface.
func (b *BigInt) Scan(value interface{}) error {
	if value == nil {
		b.Int = nil
		return nil
	}
	
	switch v := value.(type) {
	case string:
		if b.Int == nil {
			b.Int = new(big.Int)
		}
		_, ok := b.SetString(v, 10)
		if !ok {
			return fmt.Errorf("failed to parse BigInt from string: %s", v)
		}
		return nil
	case int64:
		if b.Int == nil {
			b.Int = new(big.Int)
		}
		b.SetInt64(v)
		return nil
	case []byte:
		if b.Int == nil {
			b.Int = new(big.Int)
		}
		_, ok := b.SetString(string(v), 10)
		if !ok {
			return fmt.Errorf("failed to parse BigInt from bytes: %s", v)
		}
		return nil
	default:
		return fmt.Errorf("cannot scan type %T into BigInt", value)
	}
}

// Value implements the driver.Valuer interface.
func (b BigInt) Value() (driver.Value, error) {
	if b.Int == nil {
		return nil, nil
	}
	return b.String(), nil
}

// NewBigInt creates a new BigInt from a *big.Int.
func NewBigInt(i *big.Int) BigInt {
	return BigInt{i}
}

// NewBigIntFromInt64 creates a new BigInt from an int64.
func NewBigIntFromInt64(i int64) BigInt {
	return BigInt{big.NewInt(i)}
}

// ToBigInt converts BigInt back to *big.Int.
func (b BigInt) ToBigInt() *big.Int {
	return b.Int
}