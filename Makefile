.PHONY: build test lint clean run docker-build docker-push help

APP_NAME := opsai-bot
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-X github.com/jonny/opsai-bot/pkg/version.Version=$(VERSION) \
	-X github.com/jonny/opsai-bot/pkg/version.Commit=$(COMMIT) \
	-X github.com/jonny/opsai-bot/pkg/version.BuildTime=$(BUILD_TIME)"

GO := go
GOTEST := $(GO) test
GOBUILD := $(GO) build

## build: Build the binary
build:
	$(GOBUILD) $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/opsai/

## test: Run all tests
test:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

## test-short: Run tests without verbose output
test-short:
	$(GOTEST) -race ./...

## coverage: Show test coverage
coverage: test
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run linter
lint:
	golangci-lint run ./...

## clean: Remove build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html

## run: Run the application
run: build
	./bin/$(APP_NAME) --config configs/config.yaml

## docker-build: Build Docker image
docker-build:
	docker build -t $(APP_NAME):$(VERSION) .

## help: Show this help
help:
	@echo "Available targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'
