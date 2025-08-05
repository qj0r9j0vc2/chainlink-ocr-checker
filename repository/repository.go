// Package repository provides data access layer implementations for the OCR checker application.
// It contains repository patterns for database operations and data management.
package repository

import (
	"chainlink-ocr-checker/models"
	"gorm.io/gorm"
)

// Repository provides data access methods for the OCR checker application.
type Repository struct {
	DB gorm.DB
}

// FindOCR2Jobs finds all active OCR2 jobs.
func (r *Repository) FindOCR2Jobs() ([]models.Job, error) {
	var activeJobs []models.Job
	err := r.DB.Where("type = ?", "offchainreporting2").Find(&activeJobs).Error

	return activeJobs, err
}

// FindJob finds a job by its ID.
func (r *Repository) FindJob(id int) (models.Job, error) {
	var activeJob models.Job
	err := r.DB.Where("id = ?", id).Find(&activeJob).Error

	return activeJob, err
}
