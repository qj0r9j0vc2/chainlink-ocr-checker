package interfaces

import (
	"context"

	"chainlink-ocr-checker/domain/entities"
	"github.com/ethereum/go-ethereum/common"
)

// JobRepository handles job data persistence
type JobRepository interface {
	// FindByTransmitter finds jobs by transmitter address
	FindByTransmitter(ctx context.Context, transmitterAddress common.Address) ([]entities.Job, error)

	// FindByContract finds jobs by contract address
	FindByContract(ctx context.Context, contractAddress common.Address) ([]entities.Job, error)

	// FindByFilter finds jobs matching the given filter
	FindByFilter(ctx context.Context, filter entities.JobFilter) (*entities.JobSearchResult, error)

	// FindByID finds a job by its ID
	FindByID(ctx context.Context, id int32) (*entities.Job, error)

	// FindActiveJobs returns all active jobs
	FindActiveJobs(ctx context.Context) ([]entities.Job, error)
}

// TransmissionRepository handles transmission data persistence
type TransmissionRepository interface {
	// Save saves transmission data
	Save(ctx context.Context, transmission entities.Transmission) error

	// SaveBatch saves multiple transmissions
	SaveBatch(ctx context.Context, transmissions []entities.Transmission) error

	// FindByContract finds transmissions by contract address
	FindByContract(ctx context.Context, contractAddress common.Address, limit int) ([]entities.Transmission, error)

	// FindByTransmitter finds transmissions by transmitter address
	FindByTransmitter(ctx context.Context, transmitterAddress common.Address, limit int) ([]entities.Transmission, error)

	// FindByRoundRange finds transmissions within a round range
	FindByRoundRange(ctx context.Context, contractAddress common.Address, startRound, endRound uint32) ([]entities.Transmission, error)

	// FindByTimeRange finds transmissions within a time range
	FindByTimeRange(ctx context.Context, contractAddress common.Address, startTime, endTime int64) ([]entities.Transmission, error)

	// GetLatestRound returns the latest round for a contract
	GetLatestRound(ctx context.Context, contractAddress common.Address) (uint32, error)
}

// UnitOfWork represents a unit of work pattern for transactions
type UnitOfWork interface {
	// Jobs returns the job repository
	Jobs() JobRepository

	// Transmissions returns the transmission repository
	Transmissions() TransmissionRepository

	// Commit commits the transaction
	Commit() error

	// Rollback rolls back the transaction
	Rollback() error
}
