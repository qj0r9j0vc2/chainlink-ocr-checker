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

// transmissionRepository implements the TransmissionRepository interface
type transmissionRepository struct {
	db *gorm.DB
}

// NewTransmissionRepository creates a new transmission repository
func NewTransmissionRepository(db *gorm.DB) interfaces.TransmissionRepository {
	return &transmissionRepository{db: db}
}

// Save saves transmission data
func (r *transmissionRepository) Save(ctx context.Context, transmission entities.Transmission) error {
	if err := r.db.WithContext(ctx).Create(&transmission).Error; err != nil {
		return &errors.RepositoryError{
			Operation: "Save",
			Entity:    "Transmission",
			Err:       err,
		}
	}
	return nil
}

// SaveBatch saves multiple transmissions
func (r *transmissionRepository) SaveBatch(ctx context.Context, transmissions []entities.Transmission) error {
	if len(transmissions) == 0 {
		return nil
	}

	// Use batch insert for better performance
	batchSize := 100
	for i := 0; i < len(transmissions); i += batchSize {
		end := i + batchSize
		if end > len(transmissions) {
			end = len(transmissions)
		}

		if err := r.db.WithContext(ctx).CreateInBatches(transmissions[i:end], batchSize).Error; err != nil {
			return &errors.RepositoryError{
				Operation: "SaveBatch",
				Entity:    "Transmission",
				Err:       err,
			}
		}
	}

	return nil
}

// FindByContract finds transmissions by contract address
func (r *transmissionRepository) FindByContract(ctx context.Context, contractAddress common.Address, limit int) ([]entities.Transmission, error) {
	var transmissions []entities.Transmission

	query := r.db.WithContext(ctx).
		Where("contract_address = ?", contractAddress.Hex()).
		Order("block_number DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&transmissions).Error; err != nil {
		return nil, &errors.RepositoryError{
			Operation: "FindByContract",
			Entity:    "Transmission",
			Err:       err,
		}
	}

	return transmissions, nil
}

// FindByTransmitter finds transmissions by transmitter address
func (r *transmissionRepository) FindByTransmitter(ctx context.Context, transmitterAddress common.Address, limit int) ([]entities.Transmission, error) {
	var transmissions []entities.Transmission

	query := r.db.WithContext(ctx).
		Where("transmitter_address = ?", transmitterAddress.Hex()).
		Order("block_number DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&transmissions).Error; err != nil {
		return nil, &errors.RepositoryError{
			Operation: "FindByTransmitter",
			Entity:    "Transmission",
			Err:       err,
		}
	}

	return transmissions, nil
}

// FindByRoundRange finds transmissions within a round range
func (r *transmissionRepository) FindByRoundRange(ctx context.Context, contractAddress common.Address, startRound, endRound uint32) ([]entities.Transmission, error) {
	var transmissions []entities.Transmission

	query := r.db.WithContext(ctx).
		Where("contract_address = ? AND round >= ? AND round <= ?",
			contractAddress.Hex(), startRound, endRound).
		Order("round ASC")

	if err := query.Find(&transmissions).Error; err != nil {
		return nil, &errors.RepositoryError{
			Operation: "FindByRoundRange",
			Entity:    "Transmission",
			Err:       err,
		}
	}

	return transmissions, nil
}

// FindByTimeRange finds transmissions within a time range
func (r *transmissionRepository) FindByTimeRange(ctx context.Context, contractAddress common.Address, startTime, endTime int64) ([]entities.Transmission, error) {
	var transmissions []entities.Transmission

	query := r.db.WithContext(ctx).
		Where("contract_address = ? AND latest_timestamp >= ? AND latest_timestamp <= ?",
			contractAddress.Hex(), startTime, endTime).
		Order("latest_timestamp ASC")

	if err := query.Find(&transmissions).Error; err != nil {
		return nil, &errors.RepositoryError{
			Operation: "FindByTimeRange",
			Entity:    "Transmission",
			Err:       err,
		}
	}

	return transmissions, nil
}

// GetLatestRound returns the latest round for a contract
func (r *transmissionRepository) GetLatestRound(ctx context.Context, contractAddress common.Address) (uint32, error) {
	var result struct {
		MaxRound uint32
	}

	err := r.db.WithContext(ctx).
		Model(&entities.Transmission{}).
		Select("MAX(round) as max_round").
		Where("contract_address = ?", contractAddress.Hex()).
		First(&result).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, errors.NewDomainError(errors.ErrNotFound,
				fmt.Sprintf("no transmissions found for contract %s", contractAddress.Hex()))
		}
		return 0, &errors.RepositoryError{
			Operation: "GetLatestRound",
			Entity:    "Transmission",
			Err:       err,
		}
	}

	return result.MaxRound, nil
}
