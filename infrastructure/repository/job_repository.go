package repository

import (
	"context"
	"fmt"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/domain/errors"
	"chainlink-ocr-checker/domain/interfaces"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
)

// jobRepository implements the JobRepository interface.
type jobRepository struct {
	db *gorm.DB
}

// NewJobRepository creates a new job repository.
func NewJobRepository(db *gorm.DB) interfaces.JobRepository {
	return &jobRepository{db: db}
}

// FindByTransmitter finds jobs by transmitter address.
func (r *jobRepository) FindByTransmitter(
	ctx context.Context,
	transmitterAddress common.Address,
) ([]entities.Job, error) {
	var jobs []entities.Job

	query := r.db.WithContext(ctx).
		Table("ocr2_oracle_specs o").
		Select(`
			j.id,
			j.external_job_id,
			j.created_at,
			o.contract_address,
			o.relay,
			o.relay_config,
			o.p2pv2_bootstrappers,
			o.ocr_key_bundle_id,
			o.transmitter_id,
			o.blockchain_timeout,
			o.contract_config_tracker_poll_interval,
			o.contract_config_confirmations,
			o.juels_per_fee_coin_pipeline,
			o.mercury_transmitter,
			t.from_address as transmitter_address
		`).
		Joins("JOIN jobs j ON j.ocr2_oracle_spec_id = o.id").
		Joins("JOIN transmitters t ON t.id = o.transmitter_id").
		Where("t.from_address = ?", transmitterAddress.Hex())

	err := query.Find(&jobs).Error
	if err != nil {
		return nil, &errors.RepositoryError{
			Operation: "FindByTransmitter",
			Entity:    "Job",
			Err:       err,
		}
	}

	return jobs, nil
}

// FindByContract finds jobs by contract address.
func (r *jobRepository) FindByContract(ctx context.Context, contractAddress common.Address) ([]entities.Job, error) {
	var jobs []entities.Job

	query := r.db.WithContext(ctx).
		Table("ocr2_oracle_specs o").
		Select(`
			j.id,
			j.external_job_id,
			j.created_at,
			o.contract_address,
			o.relay,
			o.relay_config,
			t.from_address as transmitter_address
		`).
		Joins("JOIN jobs j ON j.ocr2_oracle_spec_id = o.id").
		Joins("JOIN transmitters t ON t.id = o.transmitter_id").
		Where("o.contract_address = ?", contractAddress.Hex())

	err := query.Find(&jobs).Error
	if err != nil {
		return nil, &errors.RepositoryError{
			Operation: "FindByContract",
			Entity:    "Job",
			Err:       err,
		}
	}

	return jobs, nil
}

// FindByFilter finds jobs matching the given filter.
func (r *jobRepository) FindByFilter(
	ctx context.Context,
	filter entities.JobFilter,
) (*entities.JobSearchResult, error) {
	var jobs []entities.Job
	var totalCount int64

	query := r.db.WithContext(ctx).
		Table("ocr2_oracle_specs o").
		Select(`
			j.id,
			j.external_job_id,
			j.created_at,
			o.contract_address,
			o.relay,
			o.relay_config,
			t.from_address as transmitter_address
		`).
		Joins("JOIN jobs j ON j.ocr2_oracle_spec_id = o.id").
		Joins("JOIN transmitters t ON t.id = o.transmitter_id")

	// Apply filters
	if filter.TransmitterAddress != nil {
		query = query.Where("t.from_address = ?", filter.TransmitterAddress.Hex())
	}

	if filter.ContractAddress != nil {
		query = query.Where("o.contract_address = ?", filter.ContractAddress.Hex())
	}

	if filter.EVMChainID != nil {
		// Parse relay_config JSONB to filter by chain ID
		query = query.Where("o.relay_config->>'ChainID' = ?", filter.EVMChainID.String())
	}

	if filter.Active != nil {
		if *filter.Active {
			query = query.Where("j.deleted_at IS NULL")
		} else {
			query = query.Where("j.deleted_at IS NOT NULL")
		}
	}

	// Count total
	countQuery := query
	err := countQuery.Count(&totalCount).Error
	if err != nil {
		return nil, &errors.RepositoryError{
			Operation: "FindByFilter.Count",
			Entity:    "Job",
			Err:       err,
		}
	}

	// Get results
	err = query.Find(&jobs).Error
	if err != nil {
		return nil, &errors.RepositoryError{
			Operation: "FindByFilter",
			Entity:    "Job",
			Err:       err,
		}
	}

	return &entities.JobSearchResult{
		Jobs:       jobs,
		TotalCount: int(totalCount),
	}, nil
}

// FindByID finds a job by its ID.
func (r *jobRepository) FindByID(ctx context.Context, id int32) (*entities.Job, error) {
	var job entities.Job

	query := r.db.WithContext(ctx).
		Table("ocr2_oracle_specs o").
		Select(`
			j.id,
			j.external_job_id,
			j.created_at,
			o.contract_address,
			o.relay,
			o.relay_config,
			t.from_address as transmitter_address
		`).
		Joins("JOIN jobs j ON j.ocr2_oracle_spec_id = o.id").
		Joins("JOIN transmitters t ON t.id = o.transmitter_id").
		Where("j.id = ?", id)

	err := query.First(&job).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.NewDomainError(errors.ErrNotFound, fmt.Sprintf("job with id %d not found", id))
		}
		return nil, &errors.RepositoryError{
			Operation: "FindByID",
			Entity:    "Job",
			Err:       err,
		}
	}

	return &job, nil
}

// FindActiveJobs returns all active jobs.
func (r *jobRepository) FindActiveJobs(ctx context.Context) ([]entities.Job, error) {
	var jobs []entities.Job

	query := r.db.WithContext(ctx).
		Table("ocr2_oracle_specs o").
		Select(`
			j.id,
			j.external_job_id,
			j.created_at,
			o.contract_address,
			o.relay,
			o.relay_config,
			t.from_address as transmitter_address
		`).
		Joins("JOIN jobs j ON j.ocr2_oracle_spec_id = o.id").
		Joins("JOIN transmitters t ON t.id = o.transmitter_id").
		Where("j.deleted_at IS NULL")

	err := query.Find(&jobs).Error
	if err != nil {
		return nil, &errors.RepositoryError{
			Operation: "FindActiveJobs",
			Entity:    "Job",
			Err:       err,
		}
	}

	return jobs, nil
}
