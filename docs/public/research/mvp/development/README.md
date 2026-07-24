# Helix Seller — Development Guide

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.22+ | Application language |
| Node.js | 18+ | Frontend assets (if applicable) |
| PostgreSQL | 16+ | Primary database |
| Redis | 7+ | Cache and sessions |
| NATS | 2.x | Event streaming |
| Podman | 4+ | Container runtime |
| podman-compose | 2+ | Multi-container orchestration |
| golangci-lint | latest | Code linting |
| goimports | latest | Import formatting |

## Getting Started

```bash
# 1. Clone the repository
git clone https://github.com/helix-seller/helix-seller.git
cd helix-seller

# 2. Install Go dependencies
make tidy

# 3. Start backing services (PostgreSQL, Redis, NATS)
make deps-up

# 4. Create environment configuration
cp .env.example .env

# 5. Generate JWT keys
mkdir -p keys
openssl genrsa -out keys/jwt_private.pem 2048
openssl rsa -in keys/jwt_private.pem -pubout -out keys/jwt_public.pem
chmod 600 keys/jwt_private.pem

# 6. Run database migrations
make migrate-up

# 7. Start the development server
make run
```

## Project Structure

```
helix-seller/
├── api/                          # API definitions (OpenAPI/Swagger)
├── cmd/
│   ├── server/
│   │   └── main.go               # Application entrypoint
│   └── migrate/
│       └── main.go               # Database migration tool
├── constitution/                  # Project principles and agent rules
├── docs/
│   ├── guides/                   # User guides
│   ├── operations/               # Operational documentation
│   ├── private/                  # Internal documentation
│   └── public/
│       └── research/mvp/
│           ├── api/              # API research
│           ├── architecture/     # Architecture decisions
│           ├── database/         # Schema design
│           ├── deployment/       # Deployment docs
│           ├── development/      # This directory
│           └── testing/          # Test strategy
├── internal/
│   ├── config/                   # Configuration loading
│   ├── database/                 # Database connection and queries
│   ├── eventbus/                 # NATS event bus
│   ├── handler/                  # HTTP request handlers
│   ├── middleware/               # HTTP middleware
│   ├── model/                    # Data models
│   ├── provider/                 # Payment providers (Stripe, PayPal, Square)
│   ├── repository/               # Data access layer
│   ├── service/                  # Business logic
│   └── validator/                # Request validation
├── migrations/                   # SQL migration files
├── pkg/                          # Shared packages
├── scripts/                      # Utility scripts
├── specs/                        # Feature specifications
├── tests/                        # Integration and E2E tests
├── web/                          # Frontend assets
├── .env.example                  # Environment template
├── Makefile                      # Build automation
├── go.mod                        # Go module definition
├── go.sum                        # Go dependency checksums
└── podman-compose.opendesign.yml # OpenDesign container stack
```

## Code Style

### Go

```bash
# Format all code
make fmt

# This runs:
#   gofmt -w .
#   goimports -w .
```

### Linting

```bash
# Run linter
make lint

# This runs:
#   golangci-lint run ./...
```

### Style Conventions

- **Error handling:** Always check errors, use `fmt.Errorf` with `%w` for wrapping
- **Naming:** Use camelCase for unexported, PascalCase for exported
- **Package names:** Lowercase, single-word, no underscores
- **File names:** snake_case
- **Comments:** Exported functions and types must have doc comments
- **Imports:** Grouped as stdlib, external, internal

### Example Code Style

```go
// GetUserByID retrieves a user by their unique identifier.
// Returns ErrNotFound if the user does not exist.
func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
    var user model.User

    query := `SELECT id, email, created_at FROM users WHERE id = $1`
    err := r.db.QueryRow(ctx, query, id).Scan(&user.ID, &user.Email, &user.CreatedAt)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, ErrNotFound
        }
        return nil, fmt.Errorf("get user by id: %w", err)
    }

    return &user, nil
}
```

## Testing

```bash
# Run all tests with race detector
make test

# Run tests with coverage report
make test-cover

# Run a specific test
go test -race ./internal/repository/ -run TestGetUserByID

# Run tests in a package
go test -race ./internal/service/...

# Run tests matching a pattern
go test -race -run "TestCreate.*Order" ./...
```

### Test Organization

- Unit tests: `*_test.go` files alongside source code
- Table-driven tests preferred
- Use `testify` for assertions
- Mock external dependencies
- Integration tests in `tests/` directory

### Test Example

```go
func TestCreateOrder(t *testing.T) {
    tests := []struct {
        name    string
        input   model.CreateOrderRequest
        want    *model.Order
        wantErr error
    }{
        {
            name: "valid order",
            input: model.CreateOrderRequest{
                ProductID: uuid.New(),
                Quantity:  1,
            },
            want:    &model.Order{Status: "pending"},
            wantErr: nil,
        },
        {
            name:    "missing product",
            input:   model.CreateOrderRequest{},
            want:    nil,
            wantErr: ErrInvalidInput,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            svc := NewOrderService(mockRepo)
            got, err := svc.CreateOrder(context.Background(), tt.input)

            if tt.wantErr != nil {
                assert.ErrorIs(t, err, tt.wantErr)
                return
            }

            assert.NoError(t, err)
            assert.Equal(t, tt.want.Status, got.Status)
        })
    }
}
```

## Building

```bash
# Build binary
make build
# Output: bin/helix-seller

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags="-s -w" -o bin/helix-seller-linux-amd64 cmd/server/main.go

# Build Docker image
podman build -t helix-seller:latest .
```

## Database Migrations

```bash
# Apply all pending migrations
make migrate-up

# Rollback last migration
make migrate-down

# Create a new migration
migrate create -ext sql -dir migrations -seq add_users_table
```

### Migration File Format

```
migrations/
├── 000001_create_users.up.sql
├── 000001_create_users.down.sql
├── 000002_create_products.up.sql
├── 000002_create_products.down.sql
└── ...
```

## Debugging

### Delve Debugger

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Run with debugger
dlv debug cmd/server/main.go

# Run tests with debugger
dlv test ./internal/service/ -run TestCreateOrder
```

### Logging

```bash
# Development: console output
LOG_LEVEL=debug LOG_FORMAT=console make run

# Production: JSON output
LOG_LEVEL=info LOG_FORMAT=json make run
```

### Profiling

```go
import _ "net/http/pprof"

// In main.go or router setup
go func() {
    http.ListenAndServe("localhost:6060", nil)
}()
```

```bash
# CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Memory profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine dump
curl http://localhost:6060/debug/pprof/goroutine?debug=1
```

## Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the application binary |
| `make run` | Run the development server |
| `make test` | Run tests with race detector |
| `make test-cover` | Run tests with coverage report |
| `make lint` | Run golangci-lint |
| `make fmt` | Format code with gofmt and goimports |
| `make clean` | Remove build artifacts |
| `make migrate-up` | Apply database migrations |
| `make migrate-down` | Rollback database migrations |
| `make deps-up` | Start backing services |
| `make deps-down` | Stop backing services |
| `make tidy` | Run go mod tidy |
| `make vet` | Run go vet |
| `make help` | Show all available targets |
