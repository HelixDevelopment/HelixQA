# Helix Seller — Quick Start Guide

## Prerequisites

- Go 1.22+
- Podman 5+
- Podman Compose
- Git

## 1. Clone the Repository

```bash
git clone https://github.com/your-org/helix-seller.git
cd helix-seller
```

## 2. Start Dependencies

```bash
# Start PostgreSQL and Redis
podman-compose -f podman-compose.opendesign.yml up -d postgres redis
```

## 3. Set Up Database

```bash
# Run migrations
go run cmd/migrate/main.go up
```

## 4. Configure Environment

Create a `.env` file:

```bash
# Database
DATABASE_URL=postgresql://opendesign:opendesign@localhost:5432/helix_seller

# Redis
REDIS_URL=redis://localhost:6379

# Server
SERVER_PORT=8080
SERVER_HTTP3_PORT=8443

# Logging
LOG_LEVEL=info
```

## 5. Run the Server

```bash
go run cmd/server/main.go
```

The server will start at:
- HTTP/1.1: http://localhost:8080
- HTTP/3: https://localhost:8443

## 6. Verify Installation

```bash
# Health check
curl http://localhost:8080/health

# API check
curl http://localhost:8080/api/v1/sellers
```

## Development Commands

```bash
# Run tests
go test ./...

# Build binary
go build -o bin/helix-seller cmd/server/main.go

# Lint code
golangci-lint run

# Format code
gofmt -w .
```

## OpenDesign Setup (for UI/UX work)

```bash
# Start OpenDesign server
podman-compose -f podman-compose.opendesign.yml up -d

# Access OpenDesign
open http://localhost:3000
```

## Systemd Integration (User Level)

```bash
# Generate systemd services
./scripts/generate-systemd-services.sh

# Enable and start
systemctl --user enable opendesign.service
systemctl --user start opendesign.service

# Check status
systemctl --user status opendesign.service
```

## Project Structure

```
helix_seller/
├── cmd/                    # Application entry points
├── internal/               # Private application code
├── pkg/                    # Public library code
├── api/                    # API documentation
├── migrations/             # Database migrations
├── scripts/                # Build and deployment scripts
├── docs/                   # Project documentation
├── .specify/               # Spec Kit configuration
└── .opencode/              # OpenCode configuration
```

## Next Steps

1. Review the [Tech Stack Documentation](guides/TECH_STACK.md)
2. Check the [Project Constitution](../.specify/memory/constitution.md)
3. Explore the [Research Materials](../docs/research/mvp/)
4. Set up your development environment

## Troubleshooting

### Port Already in Use
```bash
# Find process using port
lsof -i :8080

# Kill process
kill -9 <PID>
```

### Database Connection Issues
```bash
# Check PostgreSQL status
podman exec opendesign-postgres pg_isready

# Reset database
podman-compose -f podman-compose.opendesign.yml down -v
podman-compose -f podman-compose.opendesign.yml up -d postgres
go run cmd/migrate/main.go up
```

### Redis Connection Issues
```bash
# Check Redis status
podman exec opendesign-redis redis-cli ping

# Reset Redis
podman-compose -f podman-compose.opendesign.yml restart redis
```

## Getting Help

- Check the [Documentation](./)
- Review the [Constitution](../.specify/memory/constitution.md)
- Open an issue on GitHub
