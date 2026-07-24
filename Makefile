.PHONY: build run test lint fmt clean help migrate-up migrate-down deps-up deps-down

APP_NAME=helix-seller
BUILD_DIR=bin
MAIN_DIR=cmd/server

all: build

build:
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_DIR)/main.go

run:
	@go run $(MAIN_DIR)/main.go

test:
	@go test -race ./...

test-cover:
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

lint:
	@golangci-lint run ./...

fmt:
	@gofmt -w .
	@goimports -w .

clean:
	@rm -rf $(BUILD_DIR) coverage.out coverage.html

migrate-up:
	@go run cmd/migrate/main.go up

migrate-down:
	@go run cmd/migrate/main.go down

deps-up:
	@podman-compose -f podman-compose.yml up -d

deps-down:
	@podman-compose -f podman-compose.yml down

tidy:
	@go mod tidy

vet:
	@go vet ./...

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build        Build the application"
	@echo "  run          Run the application"
	@echo "  test         Run tests with race detector"
	@echo "  test-cover   Run tests with coverage report"
	@echo "  lint         Run golangci-lint"
	@echo "  fmt          Format code"
	@echo "  clean        Remove build artifacts"
	@echo "  migrate-up   Run database migrations"
	@echo "  migrate-down Rollback database migrations"
	@echo "  deps-up      Start dependencies (Postgres, Redis, NATS)"
	@echo "  deps-down    Stop dependencies"
	@echo "  tidy         Run go mod tidy"
	@echo "  vet          Run go vet"
