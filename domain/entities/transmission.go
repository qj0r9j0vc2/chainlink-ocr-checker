// Package entities contains the core domain entities for the OCR checker application.
// It defines structures for jobs, transmissions, and related data types.
package entities

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// Transmission represents an OCR transmission event.
type Transmission struct {
	ContractAddress   common.Address
	ConfigDigest      [32]byte
	Epoch             uint32
	Round             uint8
	LatestAnswer      *big.Int
	LatestTimestamp   uint32
	TransmitterIndex  uint8
	TransmitterAddress common.Address
	ObserverIndex     uint8
	BlockNumber       uint64
	BlockTimestamp    time.Time
}

// TransmissionResult represents aggregated transmission data.
type TransmissionResult struct {
	ContractAddress common.Address
	StartRound      uint32
	EndRound        uint32
	Transmissions   []Transmission
}

// ObserverActivity represents observer participation statistics.
type ObserverActivity struct {
	ObserverIndex uint8
	Address       common.Address
	TotalCount    int
	DailyCount    map[string]int
	MonthlyCount  map[string]int
}

// TransmitterStatus represents the current status of a transmitter.
type TransmitterStatus struct {
	Address         common.Address
	JobID           string
	ContractAddress common.Address
	LastRound       uint32
	LastTimestamp   time.Time
	Status          JobStatus
	Error           error
}

// JobStatus represents the status of an OCR job.
type JobStatus string

// Job status constants.
const (
	JobStatusFound    JobStatus = "Found"
	JobStatusStale    JobStatus = "Stale"
	JobStatusMissing  JobStatus = "Missing"
	JobStatusNoActive JobStatus = "No Active"
	JobStatusError    JobStatus = "Error"
)

// OCR2Config represents OCR2 configuration.
type OCR2Config struct {
	ConfigDigest       [32]byte
	Signers            []common.Address
	Transmitters       []common.Address
	Threshold          uint8
	OnchainConfig      []byte
	EncodedConfigVersion uint64
	Encoded            []byte
}

// BlockRange represents a range of blocks.
type BlockRange struct {
	StartBlock uint64
	EndBlock   uint64
}

// Round represents an OCR round.
type Round struct {
	RoundID   uint32
	Answer    *big.Int
	Timestamp uint32
}