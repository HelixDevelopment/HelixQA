# Helix Seller — Tech Stack Documentation

## Overview

Helix Seller is a seller platform built with a modern, performant tech stack designed for reliability, scalability, and developer experience.

## Core Technologies

### Go (Golang)
- **Version:** 1.22+
- **Purpose:** Primary application language
- **Why Go:** Fast compilation, strong typing, excellent concurrency, small binary size, great standard library

### Gin Gonic
- **Version:** v1.9+
- **Purpose:** HTTP web framework
- **Why Gin:** High performance, minimalistic API, middleware support, validation, rendering

### HTTP/3 (QUIC/Cronet)
- **Purpose:** Transport protocol for API and real-time communication
- **Why HTTP/3:** Faster connection setup, better performance on unreliable networks, multiplexing without head-of-line blocking
- **Implementation:** Using `quic-go` library for server-side HTTP/3 support

### PostgreSQL
- **Version:** 16+
- **Purpose:** Primary database
- **Why PostgreSQL:** ACID compliance, JSON support, full-text search, extensibility, proven reliability

### Redis
- **Version:** 7+
- **Purpose:** Caching, session management, real-time pub/sub
- **Why Redis:** In-memory performance, data structures, pub/sub for WebSocket scaling, Lua scripting

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      Clients                            │
│  (Web Browsers, Mobile Apps, CLI, API Consumers)        │
└─────────────────────────────────────────────────────────┘
                           │
                           │ HTTP/3 (QUIC) / HTTPS
                           ▼
┌─────────────────────────────────────────────────────────┐
│                   Load Balancer                         │
│              (Nginx / Traefik / Caddy)                  │
└─────────────────────────────────────────────────────────┘
                           │
                           │ HTTP/1.1 / HTTP/2
                           ▼
┌─────────────────────────────────────────────────────────┐
│                 Go/Gin Application                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │   Router     │  │ Middleware  │  │  Handlers   │    │
│  │  (Gin)      │  │  Stack      │  │             │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │  Services   │  │  Repos      │  │  Models     │    │
│  │  (Business  │  │  (Data      │  │  (Domain)   │    │
│  │   Logic)    │  │   Access)   │  │             │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
└─────────────────────────────────────────────────────────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │ Postgres │ │  Redis   │ │  Object  │
        │ (Primary)│ │ (Cache)  │ │  Storage │
        └──────────┘ └──────────┘ └──────────┘
```

## Directory Structure

```
helix_seller/
├── cmd/                    # Application entry points
│   └── server/            # Main server binary
├── internal/               # Private application code
│   ├── config/            # Configuration loading
│   ├── database/          # Database connection and migrations
│   ├── handler/           # HTTP handlers (controllers)
│   ├── middleware/         # HTTP middleware
│   ├── model/             # Domain models
│   ├── repository/        # Data access layer
│   ├── service/           # Business logic layer
│   └── validator/         # Custom validators
├── pkg/                    # Public library code
│   ├── errors/            # Error types and handling
│   ├── logger/            # Structured logging
│   └── utils/             # Utility functions
├── api/                    # API documentation
│   └── openapi/           # OpenAPI/Swagger specs
├── migrations/             # Database migrations
├── scripts/                # Build and deployment scripts
├── docs/                   # Project documentation
├── .specify/               # Spec Kit configuration
└── .opencode/              # OpenCode configuration
```

## Development Workflow

### Prerequisites
- Go 1.22+
- PostgreSQL 16+
- Redis 7+
- Podman (for containerized services)
- OpenDesign (for UI/UX design)

### Local Development

1. **Start dependencies:**
   ```bash
   podman-compose -f podman-compose.opendesign.yml up -d postgres redis
   ```

2. **Run database migrations:**
   ```bash
   go run cmd/migrate/main.go up
   ```

3. **Start the server:**
   ```bash
   go run cmd/server/main.go
   ```

4. **Access the API:**
   - HTTP/1.1: http://localhost:8080
   - HTTP/3: https://localhost:8443 (requires certificate)

### Building

```bash
# Build for current platform
go build -o bin/helix-seller cmd/server/main.go

# Build for Linux (deployment)
GOOS=linux GOARCH=amd64 go build -o bin/helix-seller-linux-amd64 cmd/server/main.go
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -run TestName ./path/to/package
```

## Configuration

Configuration is loaded from environment variables and config files:

- `DATABASE_URL` - PostgreSQL connection string
- `REDIS_URL` - Redis connection string
- `SERVER_PORT` - HTTP server port (default: 8080)
- `SERVER_HTTP3_PORT` - HTTP/3 server port (default: 8443)
- `LOG_LEVEL` - Logging level (debug, info, warn, error)

## API Design

### RESTful Endpoints
- `/api/v1/sellers` - Seller management
- `/api/v1/products` - Product catalog
- `/api/v1/orders` - Order processing
- `/api/v1/inventory` - Inventory management
- `/api/v1/analytics` - Business analytics

### WebSocket Endpoints
- `/ws/realtime` - Real-time updates (orders, inventory)
- `/ws/notifications` - Push notifications

### Authentication
- JWT-based authentication
- OAuth2 support for third-party integrations
- API key authentication for programmatic access

## Security Considerations

- All data encrypted at rest (PostgreSQL TDE, Redis AUTH)
- TLS 1.3 for all connections
- HTTP/3 with QUIC encryption
- Rate limiting on all endpoints
- Input validation and sanitization
- SQL injection prevention (parameterized queries)
- XSS protection (Content Security Policy)
- CSRF protection

## Performance Targets

- API response time: < 100ms (p95)
- WebSocket message latency: < 50ms
- Database query time: < 10ms (p95)
- Cache hit rate: > 90%
- Uptime: 99.9%

## Monitoring and Observability

- Structured logging (JSON format)
- Metrics collection (Prometheus)
- Distributed tracing (OpenTelemetry)
- Health checks at `/health`
- Readiness checks at `/ready`

## Deployment

### Container Deployment
```bash
# Build container
podman build -t helix-seller .

# Run container
podman run -d \
  --name helix-seller \
  -p 8080:8080 \
  -p 8443:8443 \
  helix-seller
```

### Kubernetes Deployment
- Helm chart available in `deploy/helm/`
- Supports horizontal pod autoscaling
- Configurable resource limits

## Further Reading

- [Go Documentation](https://go.dev/doc/)
- [Gin Documentation](https://gin-gonic.com/docs/)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [Redis Documentation](https://redis.io/documentation)
- [HTTP/3 Specification](https://www.rfc-editor.org/rfc/rfc9114)
