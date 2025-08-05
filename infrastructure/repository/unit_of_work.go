package repository

import (
	"chainlink-ocr-checker/domain/interfaces"
	"gorm.io/gorm"
)

// unitOfWork implements the UnitOfWork interface
type unitOfWork struct {
	db                     *gorm.DB
	tx                     *gorm.DB
	jobRepository          interfaces.JobRepository
	transmissionRepository interfaces.TransmissionRepository
}

// NewUnitOfWork creates a new unit of work
func NewUnitOfWork(db *gorm.DB) interfaces.UnitOfWork {
	return &unitOfWork{
		db: db,
	}
}

// Begin starts a new transaction
func (u *unitOfWork) Begin() error {
	u.tx = u.db.Begin()
	if u.tx.Error != nil {
		return u.tx.Error
	}

	// Initialize repositories with transaction
	u.jobRepository = NewJobRepository(u.tx)
	u.transmissionRepository = NewTransmissionRepository(u.tx)

	return nil
}

// Jobs returns the job repository
func (u *unitOfWork) Jobs() interfaces.JobRepository {
	if u.jobRepository == nil {
		if u.tx != nil {
			u.jobRepository = NewJobRepository(u.tx)
		} else {
			u.jobRepository = NewJobRepository(u.db)
		}
	}
	return u.jobRepository
}

// Transmissions returns the transmission repository
func (u *unitOfWork) Transmissions() interfaces.TransmissionRepository {
	if u.transmissionRepository == nil {
		if u.tx != nil {
			u.transmissionRepository = NewTransmissionRepository(u.tx)
		} else {
			u.transmissionRepository = NewTransmissionRepository(u.db)
		}
	}
	return u.transmissionRepository
}

// Commit commits the transaction
func (u *unitOfWork) Commit() error {
	if u.tx == nil {
		return nil
	}

	err := u.tx.Commit().Error
	u.tx = nil
	return err
}

// Rollback rolls back the transaction
func (u *unitOfWork) Rollback() error {
	if u.tx == nil {
		return nil
	}

	err := u.tx.Rollback().Error
	u.tx = nil
	return err
}
