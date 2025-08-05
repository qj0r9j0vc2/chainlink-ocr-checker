// Package entities contains the core domain entities for the OCR checker application.
// It defines structures for jobs, transmissions, and related data types.
package entities

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// Job represents a Chainlink OCR2 job.
type Job struct {
	ID                     int32
	ExternalJobID          string
	OracleSpec             OracleSpec
	TransmitterAddress     common.Address
	Active                 bool
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// OracleSpec represents the oracle specification.
type OracleSpec struct {
	ContractAddress          common.Address
	EVMChainID               *big.Int `gorm:"-"`
	TransmitterAddress       common.Address
	DatabaseTimeout          time.Duration
	DatabaseMaxIdleConns     int
	DatabaseMaxOpenConns     int
	ObservationGracePeriod   time.Duration
}

// JobFilter represents filters for querying jobs.
type JobFilter struct {
	TransmitterAddress *common.Address
	ContractAddress    *common.Address
	EVMChainID         *big.Int
	Active             *bool
}

// JobSearchResult represents the result of a job search.
type JobSearchResult struct {
	Jobs       []Job
	TotalCount int
}