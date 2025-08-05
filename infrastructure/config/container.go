package config

import (
	"fmt"

	"chainlink-ocr-checker/application/services"
	"chainlink-ocr-checker/application/usecases"
	"chainlink-ocr-checker/domain/interfaces"
	"chainlink-ocr-checker/infrastructure/blockchain"
	"chainlink-ocr-checker/infrastructure/logger"
	"chainlink-ocr-checker/infrastructure/repository"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Container represents the dependency injection container
type Container struct {
	Config *Config
	
	// Infrastructure
	Logger           interfaces.Logger
	DB               *gorm.DB
	EthClient        *ethclient.Client
	BlockchainClient interfaces.BlockchainClient
	
	// Repositories
	JobRepository          interfaces.JobRepository
	TransmissionRepository interfaces.TransmissionRepository
	UnitOfWork            interfaces.UnitOfWork
	
	// Services
	OCR2AggregatorService interfaces.OCR2AggregatorService
	TransmissionFetcher   interfaces.TransmissionFetcher
	TransmissionAnalyzer  interfaces.TransmissionAnalyzer
	
	// Use Cases
	FetchTransmissionsUseCase interfaces.FetchTransmissionsUseCase
	WatchTransmittersUseCase interfaces.WatchTransmittersUseCase
	ParseTransmissionsUseCase interfaces.ParseTransmissionsUseCase
}

// NewContainer creates a new dependency injection container
func NewContainer(config *Config) (*Container, error) {
	container := &Container{
		Config: config,
	}
	
	// Initialize logger
	container.Logger = logger.NewLogrusLogger(config.LogLevel)
	
	// Initialize blockchain client
	if err := container.initBlockchainClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize blockchain client: %w", err)
	}
	
	// Initialize database (optional)
	if config.Database.Host != "" {
		if err := container.initDatabase(); err != nil {
			container.Logger.Warn("Failed to initialize database", "error", err)
			// Database is optional, so we continue
		}
	}
	
	// Initialize services
	container.initServices()
	
	// Initialize use cases
	container.initUseCases()
	
	return container, nil
}

// initBlockchainClient initializes the blockchain client
func (c *Container) initBlockchainClient() error {
	// Create Ethereum client
	ethClient, err := ethclient.Dial(c.Config.RPCAddr)
	if err != nil {
		return fmt.Errorf("failed to dial RPC: %w", err)
	}
	c.EthClient = ethClient
	
	// Create blockchain client wrapper
	blockchainClient, err := blockchain.NewEthereumClient(c.Config.RPCAddr, c.Config.ChainID)
	if err != nil {
		return fmt.Errorf("failed to create blockchain client: %w", err)
	}
	c.BlockchainClient = blockchainClient
	
	return nil
}

// initDatabase initializes the database connection
func (c *Container) initDatabase() error {
	dsn := c.Config.Database.GetDatabaseDSN()
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gorm.Logger(nil), // We use our own logger
	})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	
	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	
	sqlDB.SetMaxIdleConns(c.Config.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(c.Config.Database.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(c.Config.Database.ConnMaxLifetime)
	
	c.DB = db
	
	// Initialize repositories
	c.JobRepository = repository.NewJobRepository(db)
	c.TransmissionRepository = repository.NewTransmissionRepository(db)
	c.UnitOfWork = repository.NewUnitOfWork(db)
	
	return nil
}

// initServices initializes domain services
func (c *Container) initServices() {
	// OCR2 Aggregator Service
	c.OCR2AggregatorService = blockchain.NewOCR2AggregatorService(c.EthClient, c.Config.ChainID)
	
	// Transmission Fetcher
	c.TransmissionFetcher = blockchain.NewTransmissionFetcher(c.BlockchainClient, c.OCR2AggregatorService)
	
	// Transmission Analyzer
	c.TransmissionAnalyzer = services.NewTransmissionAnalyzer(c.Logger)
}

// initUseCases initializes use cases
func (c *Container) initUseCases() {
	// Fetch Transmissions Use Case
	c.FetchTransmissionsUseCase = usecases.NewFetchTransmissionsUseCase(
		c.TransmissionFetcher,
		c.TransmissionRepository,
		c.Logger,
	)
	
	// Watch Transmitters Use Case
	if c.JobRepository != nil {
		c.WatchTransmittersUseCase = usecases.NewWatchTransmittersUseCase(
			c.JobRepository,
			c.TransmissionFetcher,
			c.OCR2AggregatorService,
			c.Logger,
		)
	}
	
	// Parse Transmissions Use Case
	c.ParseTransmissionsUseCase = usecases.NewParseTransmissionsUseCase(
		c.TransmissionAnalyzer,
		c.Logger,
	)
}

// Close closes all resources
func (c *Container) Close() error {
	// Close blockchain client
	if c.BlockchainClient != nil {
		if err := c.BlockchainClient.Close(); err != nil {
			c.Logger.Error("Failed to close blockchain client", "error", err)
		}
	}
	
	// Close Ethereum client
	if c.EthClient != nil {
		c.EthClient.Close()
	}
	
	// Close database
	if c.DB != nil {
		sqlDB, err := c.DB.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				c.Logger.Error("Failed to close database", "error", err)
			}
		}
	}
	
	return nil
}