# Chainlink OCR Checker

[![CI](https://github.com/anjin-u/chainlink-ocr-checker/actions/workflows/ci.yml/badge.svg)](https://github.com/anjin-u/chainlink-ocr-checker/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/anjin-u/chainlink-ocr-checker)](https://goreportcard.com/report/github.com/anjin-u/chainlink-ocr-checker)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A production-ready monitoring tool for Chainlink OCR2 (Off-Chain Reporting v2) protocol. This tool helps track transmitter participation, analyze observer activity, and monitor protocol performance across different blockchain networks.

## Features

- 🔍 **Transmitter Monitoring**: Track transmitter activity across multiple OCR2 jobs
- 📊 **Observer Analysis**: Analyze observer participation patterns and detect anomalies
- 🔄 **Real-time Monitoring**: Watch recent rounds for transmitter participation
- 📈 **Historical Analysis**: Fetch and analyze historical transmission data
- 🎯 **Multi-format Output**: Support for JSON, YAML, CSV, and text output formats
- 🏗️ **Clean Architecture**: Built with SOLID principles and dependency injection
- ✅ **Comprehensive Testing**: Unit tests, integration tests, and E2E tests
- 🚀 **Production Ready**: Error handling, structured logging, and CI/CD pipeline

## Architecture

This project follows Clean Architecture principles with clear separation of concerns:

```
├── domain/           # Domain entities and interfaces
├── application/      # Use cases and business logic
├── infrastructure/   # External dependencies (database, blockchain)
├── cmd/             # CLI commands
└── test/            # Tests and mocks
```

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/anjin-u/chainlink-ocr-checker.git
cd chainlink-ocr-checker

# Build the binary
go build -o ocr-checker .

# Or use make
make build
```

### Using Go Install

```bash
go install github.com/anjin-u/chainlink-ocr-checker@latest
```

## Configuration

Create a `config.toml` file:

```toml
log_level = "info"
chain_id = 137
rpc_addr = "https://polygon.drpc.org"

# Optional: Database configuration for watch command
[database]
user = 'postgres'
password = 'password'
host = 'localhost'
port = '5432'
dbName = 'chainlink'
sslMode = 'disable'
```

You can also use environment variables with the `OCR_` prefix:

```bash
export OCR_LOG_LEVEL=debug
export OCR_CHAIN_ID=137
export OCR_RPC_ADDR=https://polygon.drpc.org
```

## Usage

### Fetch Transmissions

Fetch historical OCR transmission data for a specific contract:

```bash
# Fetch transmissions for rounds 1-100
./ocr-checker fetch 0xa142BB41f409599603D3bB16842D0d274AAeDcf5 1 100

# Output to JSON format
./ocr-checker fetch --format json --output results.json 0xa142BB41f409599603D3bB16842D0d274AAeDcf5 1 100
```

### Watch Transmitter Activity

Monitor transmitter participation across OCR2 jobs (requires database configuration):

```bash
# Check last 10 rounds, ignore if no activity in last 7 days
./ocr-checker watch 0x2dbbd12bf0f6a23cf4455cc6be874b7a246288ce 10 7

# Output in JSON format
./ocr-checker watch --output json 0x2dbbd12bf0f6a23cf4455cc6be874b7a246288ce 10
```

### Parse and Analyze Data

Parse fetched data and generate observer activity reports:

```bash
# Group by day
./ocr-checker parse results/data.yaml day > daily_report.txt

# Output as CSV
./ocr-checker parse --format csv --output report.csv results/data.yaml month
```

### Version Information

```bash
./ocr-checker version
```

## Development

### Prerequisites

- Go 1.23 or higher
- PostgreSQL (optional, for watch command)
- Make (optional, for using Makefile commands)

### Building

```bash
# Build the binary
make build

# Run tests
make test

# Run linting
make lint

# Generate mocks
make mocks
```

### Testing

```bash
# Unit tests
make test-unit

# Integration tests
make test-integration

# E2E tests
RUN_E2E_TESTS=true make test-e2e

# All tests with coverage
make test-all
```

## Project Structure

```
chainlink-ocr-checker/
├── domain/
│   ├── entities/        # Domain entities
│   ├── interfaces/      # Domain interfaces
│   └── errors/          # Domain errors
├── application/
│   ├── usecases/        # Business use cases
│   └── services/        # Application services
├── infrastructure/
│   ├── blockchain/      # Blockchain client implementations
│   ├── repository/      # Database repositories
│   ├── config/          # Configuration and DI container
│   └── logger/          # Logging implementation
├── cmd/
│   └── ocr-checker/     # CLI application
│       └── commands/    # CLI commands
└── test/
    ├── mocks/           # Generated mocks
    ├── fixtures/        # Test fixtures
    ├── helpers/         # Test helpers
    └── e2e/             # End-to-end tests
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow Go best practices and conventions
- Write tests for new features
- Update documentation as needed
- Ensure all tests pass before submitting PR
- Add appropriate logging and error handling

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Chainlink](https://chain.link/) for the OCR2 protocol
- [go-ethereum](https://github.com/ethereum/go-ethereum) for Ethereum client
- [Cobra](https://github.com/spf13/cobra) for CLI framework
- [GORM](https://gorm.io/) for database ORM