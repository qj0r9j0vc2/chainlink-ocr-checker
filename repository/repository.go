package repository

import (
	"chainlink-ocr-checker/models"
	"gorm.io/gorm"
)

type Repository struct {
	DB gorm.DB
}

func (r *Repository) FindOCR2Jobs() ([]models.Job, error) {
	var activeJobs []models.Job
	err := r.DB.Where("type = ?", "offchainreporting2").Find(&activeJobs).Error

	return activeJobs, err
}

func (r *Repository) FindJob(id int) (models.Job, error) {
	var activeJob models.Job
	err := r.DB.Where("id = ?", id).Find(&activeJob).Error

	return activeJob, err
}
