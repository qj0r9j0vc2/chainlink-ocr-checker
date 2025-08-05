.PHONY: all build test test-unit test-integration test-e2e clean lint coverage mocks

# Variables
BINARY_NAME=ocr-checker
GO=go
GOTEST=$(GO) test
GOVET=$(GO) vet
GOMOD=$(GO) mod
MOCKGEN=$(shell go env GOPATH)/bin/mockgen

# Build
all: clean build

build:
	$(GO) build -o $(BINARY_NAME) .

clean:
	$(GO) clean
	rm -f $(BINARY_NAME)
	rm -rf coverage/

# Dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

deps-dev:
	$(GO) install github.com/golang/mock/mockgen@latest
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Testing
test: test-unit

test-unit:
	$(GOTEST) -v -short -race -coverprofile=coverage.out ./...

test-integration:
	$(GOTEST) -v -race -coverprofile=coverage.out -run Integration ./...

test-e2e:
	$(GOTEST) -v -race -coverprofile=coverage.out -run E2E ./...

test-all: test-unit test-integration test-e2e

# Coverage
coverage: test-unit
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

coverage-report: test-unit
	$(GO) tool cover -func=coverage.out

# Linting
lint:
	golangci-lint run --timeout=5m

lint-fix:
	golangci-lint run --fix

# Generate mocks
mocks:
	$(MOCKGEN) -source=domain/interfaces/blockchain.go -destination=test/mocks/mock_blockchain.go -package=mocks
	$(MOCKGEN) -source=domain/interfaces/repository.go -destination=test/mocks/mock_repository.go -package=mocks
	$(MOCKGEN) -source=domain/interfaces/usecase.go -destination=test/mocks/mock_usecase.go -package=mocks
	$(MOCKGEN) -source=domain/interfaces/logger.go -destination=test/mocks/mock_logger.go -package=mocks

# Development
run-fetch:
	$(GO) run . fetch $(ARGS)

run-watch:
	$(GO) run . watch $(ARGS)

run-parse:
	$(GO) run . parse $(ARGS)

# Docker
docker-build:
	docker build -t $(BINARY_NAME) .

docker-run:
	docker run --rm -v $(PWD)/config.toml:/app/config.toml $(BINARY_NAME) $(ARGS)

# CI/CD helpers
ci-test:
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...

ci-lint:
	golangci-lint run --out-format=github-actions

# Installation
install:
	$(GO) install

uninstall:
	rm -f $(GOPATH)/bin/$(BINARY_NAME)