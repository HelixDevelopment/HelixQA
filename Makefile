# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: Apache-2.0

.PHONY: all build test test-race vet lint clean help install

# Default target
all: vet test build

## Build the helixqa binary
build:
	go build -o bin/helixqa ./cmd/helixqa

## Install the helixqa binary
install:
	go install ./cmd/helixqa

## Run all tests
test:
	go test ./... -count=1

## Run all tests with race detection
test-race:
	go test ./... -race -count=1

## Run tests with coverage report
test-cover:
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -html=coverage.out -o coverage.html

## Run go vet
vet:
	go vet ./...

## Run static analysis (requires golangci-lint)
lint:
	golangci-lint run ./...

## Format all Go files
fmt:
	gofmt -w .

## Tidy go.mod
tidy:
	go mod tidy

## Clean build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html qa-results/ results/ evidence/

## Show help
help:
	@echo "HelixQA - QA Orchestration Framework"
	@echo ""
	@echo "Targets:"
	@echo "  all        Build, vet, and test (default)"
	@echo "  build      Build the helixqa binary"
	@echo "  install    Install the helixqa binary"
	@echo "  test       Run all tests"
	@echo "  test-race  Run tests with race detection"
	@echo "  test-cover Run tests with coverage report"
	@echo "  vet        Run go vet"
	@echo "  lint       Run golangci-lint"
	@echo "  fmt        Format Go files"
	@echo "  tidy       Tidy go.mod"
	@echo "  clean      Clean build artifacts"
	@echo "  help       Show this help"
