// Package config provides configuration management and dependency injection for the OCR checker application.
// It handles loading configuration from files and environment variables, and sets up the DI container.
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration.
type Config struct {
	LogLevel string `mapstructure:"log_level"`
	ChainID  int64  `mapstructure:"chain_id"`
	RPCAddr  string `mapstructure:"rpc_addr"`

	Database DatabaseConfig `mapstructure:"database"`

	// Timeouts and limits.
	BlockchainTimeout    time.Duration `mapstructure:"blockchain_timeout"`
	MaxConcurrency       int           `mapstructure:"max_concurrency"`
	DefaultBlockInterval int           `mapstructure:"default_block_interval"`
}

// DatabaseConfig represents database configuration.
type DatabaseConfig struct {
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	DBName   string `mapstructure:"dbName"`
	SSLMode  string `mapstructure:"sslMode"`

	// Connection pool settings.
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// LoadConfig loads configuration from file and environment.
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults.
	v.SetDefault("log_level", "info")
	v.SetDefault("blockchain_timeout", "30s")
	v.SetDefault("max_concurrency", 30)
	v.SetDefault("default_block_interval", 5000)
	v.SetDefault("database.sslMode", "disable")
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.max_open_conns", 100)
	v.SetDefault("database.conn_max_lifetime", "1h")

	// Set config file.
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("toml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/ocr-checker")
	}

	// Enable environment variables.
	v.SetEnvPrefix("OCR")
	v.AutomaticEnv()

	// Read config file.
	if err := v.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist.
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration.
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.ChainID <= 0 {
		return fmt.Errorf("chain_id must be positive")
	}

	if c.RPCAddr == "" {
		return fmt.Errorf("rpc_addr is required")
	}

	if c.MaxConcurrency <= 0 {
		return fmt.Errorf("max_concurrency must be positive")
	}

	if c.DefaultBlockInterval <= 0 {
		return fmt.Errorf("default_block_interval must be positive")
	}

	return nil
}

// GetDatabaseDSN returns the database connection string.
func (c *DatabaseConfig) GetDatabaseDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}
