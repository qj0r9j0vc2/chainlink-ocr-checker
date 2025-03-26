package config

import (
	"chainlink-ocr-checker/repository"
	"database/sql"
	"fmt"
	"github.com/pkg/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
)

type Database struct {
	User     string `toml:"user"`
	Password string `toml:"password"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	DbName   string `toml:"dbName"`

	SSLMode string `toml:"sslMode"` // e.g., "disable", "require"
}

func GetDatabase(dbConfig Database) (*sql.DB, error) {
	db, err := getDBConnection(dbConfig)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func getDBConnection(dbConfig Database) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.User,
		dbConfig.Password,
		dbConfig.DbName,
		dbConfig.SSLMode,
	)

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to database. possible drivers: "+strings.Join(sql.Drivers(), ", "))
	}

	return sqlDB, nil
}

func newRepository(db *sql.DB) (*repository.Repository, error) {
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to database")
	}

	return &repository.Repository{DB: *gormDB}, nil
}
