# Local Development Setup

## Go Environment

### Installation

```bash
# macOS (Homebrew)
brew install go

# Ubuntu/Debian
sudo apt install golang-go

# Verify installation
go version
# Should output: go version go1.22.x linux/amd64
```

### Go Configuration

```bash
# Set GOPATH (optional, defaults to $HOME/go)
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin

# Enable Go modules
export GO111MODULE=on

# Configure proxy for China/slow networks (optional)
export GOPROXY=https://proxy.golang.org,direct
```

### Install Development Tools

```bash
# golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# goimports
go install golang.org/x/tools/cmd/goimports@latest

# migrate (database migrations)
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# delve (debugger)
go install github.com/go-delve/delve/cmd/dlv@latest
```

## PostgreSQL Local Setup

### Option 1: System Package

```bash
# Ubuntu/Debian
sudo apt install postgresql postgresql-contrib
sudo systemctl start postgresql
sudo systemctl enable postgresql

# Create database and user
sudo -u postgres psql
CREATE USER helix WITH PASSWORD 'helix';
CREATE DATABASE helix_seller OWNER helix;
GRANT ALL PRIVILEGES ON DATABASE helix_seller TO helix;
\q
```

### Option 2: Container (Recommended)

```bash
# Using Podman
podman run -d \
  --name helix-postgres \
  -e POSTGRES_DB=helix_seller \
  -e POSTGRES_USER=helix \
  -e POSTGRES_PASSWORD=helix \
  -p 5432:5432 \
  -v postgres-data:/var/lib/postgresql/data \
  postgres:16-alpine

# Verify
podman ps
psql -h localhost -U helix -d helix_seller
```

### Option 3: Make Target

```bash
make deps-up
# Starts PostgreSQL, Redis, NATS in containers
```

## Redis Local Setup

### Option 1: System Package

```bash
# Ubuntu/Debian
sudo apt install redis-server
sudo systemctl start redis
sudo systemctl enable redis

# Verify
redis-cli ping
# Should output: PONG
```

### Option 2: Container

```bash
podman run -d \
  --name helix-redis \
  -p 6379:6379 \
  -v redis-data:/data \
  redis:7-alpine

# Verify
podman exec helix-redis redis-cli ping
```

## NATS Local Setup

### Option 1: Binary Download

```bash
# Download NATS server
curl -sf https://binaries.nats.dev/nats-io/nats-server/v2@latest | sh

# Run with JetStream
nats-server --jetstream --store_dir /tmp/nats-data

# Verify
nats server info
```

### Option 2: Container

```bash
podman run -d \
  --name helix-nats \
  -p 4222:4222 \
  -p 8222:8222 \
  -v nats-data:/data \
  nats:2-alpine --jetstream

# Verify monitoring UI
curl http://localhost:8222/varz
```

## Running the Server

### Quick Start (with containerized dependencies)

```bash
# 1. Start backing services
make deps-up

# 2. Create .env file
cp .env.example .env

# 3. Generate JWT keys
mkdir -p keys
openssl genrsa -out keys/jwt_private.pem 2048
openssl rsa -in keys/jwt_private.pem -pubout -out keys/jwt_public.pem
chmod 600 keys/jwt_private.pem

# 4. Run migrations
make migrate-up

# 5. Start server
make run
```

### Manual Start

```bash
# Ensure PostgreSQL, Redis, NATS are running

# Set environment variables
export DATABASE_URL=postgresql://helix:helix@localhost:5432/helix_seller
export REDIS_URL=redis://localhost:6379
export NATS_URL=nats://localhost:4222

# Run the server
go run cmd/server/main.go
```

### Server Output

```
{"level":"info","time":"2026-07-23T12:00:00Z","msg":"Starting server","addr":"0.0.0.0:8080"}
```

### Verify

```bash
# Health check
curl http://localhost:8080/health
# {"status":"healthy","time":"2026-07-23T12:00:00Z"}

# API base
curl http://localhost:8080/api/v1
```

## Running Tests

### All Tests

```bash
make test
# Runs: go test -race ./...
```

### Specific Package

```bash
go test -race ./internal/service/...
```

### Specific Test

```bash
go test -race ./internal/repository/ -run TestGetUserByID
```

### With Verbose Output

```bash
go test -race -v ./internal/service/...
```

### Coverage Report

```bash
make test-cover
# Generates coverage.html

# View coverage
open coverage.html  # macOS
xdg-open coverage.html  # Linux
```

### Integration Tests

```bash
# Ensure dependencies are running
make deps-up

# Run integration tests
go test -race -tags=integration ./tests/...
```

## Debugging Tips

### Using Delve

```bash
# Debug the server
dlv debug cmd/server/main.go

# Common Delve commands:
# break main     - Set breakpoint at main
# continue       - Continue execution
# step           - Step into function
# next           - Step over function
# print variable - Print variable value
# vars           - List all variables
# goroutines     - List goroutines
```

### Debugging Tests

```bash
# Debug a specific test
dlv test ./internal/service/ -run TestCreateOrder

# In Delve:
# break TestCreateOrder
# continue
# step
```

### Logging for Debug

```bash
# Run with debug logging
LOG_LEVEL=debug LOG_FORMAT=console go run cmd/server/main.go
```

### Network Debugging

```bash
# Check if ports are in use
lsof -i :8080
lsof -i :5432
lsof -i :6379
lsof -i :4222

# Test database connection
psql -h localhost -U helix -d helix_seller -c "SELECT 1;"

# Test Redis connection
redis-cli ping

# Test NATS connection
nats server info
```

### Container Debugging

```bash
# View container logs
podman logs helix-postgres
podman logs helix-redis
podman logs helix-nats

# Execute command in container
podman exec -it helix-postgres psql -U helix -d helix_seller

# Check container health
podman inspect --format='{{.State.Health.Status}}' helix-postgres
```

### Common Issues

#### Port Already in Use

```bash
# Find process using the port
lsof -i :8080

# Kill the process
kill -9 <PID>

# Or use a different port
SERVER_PORT=8081 make run
```

#### Database Connection Refused

```bash
# Check if PostgreSQL is running
podman ps | grep postgres

# Check PostgreSQL logs
podman logs helix-postgres

# Restart PostgreSQL
podman restart helix-postgres
```

#### Migration Failures

```bash
# Check migration status
go run cmd/migrate/main.go status

# Force migration version (if stuck)
go run cmd/migrate/main.go force <version>

# Reset database (WARNING: deletes data)
go run cmd/migrate/main.go drop
make migrate-up
```

#### Module Errors

```bash
# Clean module cache
go clean -modcache

# Re-download dependencies
go mod download

# Tidy modules
make tidy
```

## IDE Setup

### VS Code

Recommended extensions:

- Go (golang.go)
- Go Test Explorer (prempvlad.go-test-explorer)
- Error Lens (usernamehw.errorlens)
- GitLens (eamodio.gitlens)

`.vscode/settings.json`:

```json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast"],
  "go.testTimeout": "30s",
  "editor.formatOnSave": true,
  "[go]": {
    "editor.defaultFormatter": "golang.go"
  }
}
```

### GoLand

- Built-in Go support
- Run configurations for server and tests
- Database tool for PostgreSQL
- Docker/Podman integration
