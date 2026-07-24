# Helix Seller

Unified payment processing facade providing a single REST API across Stripe, PayPal, and Square.

## Quick Start

```bash
# Start dependencies
make deps-up

# Run the server
make run

# Or build and run
make build
./bin/helix-seller
```

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│  Angular    │────▶│  Go Server   │────▶│  PostgreSQL 16  │
│  Web Portal │     │  (Gin/HTTP3) │     └─────────────────┘
└─────────────┘     │              │     ┌─────────────────┐
                    │              │────▶│  Redis 7        │
┌─────────────┐     │              │     └─────────────────┘
│  Go CLI     │────▶│              │     ┌─────────────────┐
└─────────────┘     │              │────▶│  NATS JetStream │
                    └──────────────┘     └─────────────────┘
                           │
            ┌──────────────┼──────────────┐
            ▼              ▼              ▼
       ┌─────────┐   ┌─────────┐   ┌─────────┐
       │ Stripe  │   │ PayPal  │   │ Square  │
       └─────────┘   └─────────┘   └─────────┘
```

## Tech Stack

- **Backend**: Go 1.22+, Gin Gonic, HTTP/3 (QUIC/Cronet)
- **Database**: PostgreSQL 16, Redis 7, NATS JetStream
- **Frontend**: Angular 19 (standalone components, lazy loading)
- **Auth**: JWT RS256, Argon2id passwords, TOTP MFA
- **Observability**: OpenTelemetry, Prometheus metrics
- **Deployment**: Rootless Podman Compose

## Project Structure

```
├── cmd/
│   ├── server/          # API server entry point
│   └── cli/             # CLI entry point (Cobra)
├── internal/
│   ├── handler/         # 21 HTTP handlers (62 endpoints)
│   ├── service/         # 16 business logic services
│   ├── repository/      # 12 data access layers
│   ├── model/           # 16 domain models
│   ├── middleware/       # 10 middleware (auth, RBAC, rate-limit, ...)
│   ├── websocket/       # Real-time WebSocket hub
│   └── observability/   # OpenTelemetry + Prometheus
├── pkg/
│   ├── crypto/          # AES-256-GCM encryption
│   ├── errors/          # Structured error handling
│   ├── logger/          # Zap structured logging
│   └── utils/           # UUID, currency, time utilities
├── sdk/
│   ├── go/              # Go client SDK
│   ├── typescript/      # TypeScript client SDK
│   └── python/          # Python client SDK
├── migrations/          # 34 PostgreSQL migrations
├── scripts/             # Backup & DR scripts
├── web/                 # Angular 19 portal
└── specs/               # API contract (OpenAPI 3.1)
```

## API Endpoints

| Category       | Endpoints | Description |
|----------------|-----------|-------------|
| Auth           | 6         | Login, register, refresh, MFA, API keys |
| Merchants      | 5         | CRUD + onboarding |
| Transactions   | 5         | Create, list, detail, refund |
| Customers      | 5         | CRUD operations |
| Subscriptions  | 5         | CRUD + lifecycle |
| Invoices       | 5         | CRUD + send/void |
| Payouts        | 5         | Create, list, detail |
| Disputes       | 5         | Create, list, evidence upload |
| Webhooks       | 5         | Config CRUD + ingress |
| Providers      | 5         | Payment provider config |
| Analytics      | 3         | Summary, transactions, export |
| Health         | 3         | Health, ready, live |
| Metrics        | 1         | Prometheus /metrics |
| WebSocket      | 1         | Real-time updates |

## Configuration

Environment variables (see `.env.example`):

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://helix:helix_dev@localhost:5432/helix_seller?sslmode=disable` |
| `REDIS_URL` | Redis connection string | `redis://localhost:6379` |
| `NATS_URL` | NATS JetStream URL | `nats://localhost:4222` |
| `JWT_PRIVATE_KEY_PATH` | RSA private key for signing | `certs/jwt_private.pem` |
| `JWT_PUBLIC_KEY_PATH` | RSA public key for verification | `certs/jwt_public.pem` |
| `ENCRYPTION_KEY` | 32-byte key for AES-256-GCM | (required) |
| `SERVER_PORT` | HTTP listen port | `8080` |

## CLI Usage

```bash
# Merchant management
helix-seller merchant list
helix-seller merchant create --name "Acme Corp" --email "admin@acme.com"
helix-seller merchant get <id>

# Transactions
helix-seller transaction list --merchant <id>
helix-seller transaction create --merchant <id> --amount 5000 --currency USD

# Analytics
helix-seller analytics summary --merchant <id> --period 30d
helix-seller analytics export --merchant <id> --format csv

# Auth
helix-seller auth login --email admin@acme.com --password secret
helix-seller auth mfa enable
```

## SDKs

### Go
```go
client := helix.NewClient("https://api.helixseller.com", "your-api-key")
merchants, _ := client.Merchants.List(ctx, nil)
```

### TypeScript
```typescript
const client = new HelixClient('https://api.helixseller.com', 'your-api-key');
const merchants = await client.merchants.list();
```

### Python
```python
client = HelixClient("https://api.helixseller.com", api_key="your-api-key")
merchants = client.merchants.list()
```

## Development

```bash
# Run tests
make test

# Run with coverage
make test-cover

# Lint
make lint

# Format
make fmt

# Build
make build
```

## Deployment

```bash
# Start all services
make deps-up

# Run database migrations
make migrate-up

# Start the server
make run
```

## License

Proprietary — Helix Seller Platform
