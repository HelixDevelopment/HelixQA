# Environment Setup Guide

## Development Environment

### Local Setup (No Containers)

Run services directly on the host for fastest iteration:

```bash
# 1. Install dependencies
make deps-up

# 2. Create environment file
cp .env.example .env
# Edit .env with local defaults (localhost URLs)

# 3. Generate JWT keys
mkdir -p keys
openssl genrsa -out keys/jwt_private.pem 2048
openssl rsa -in keys/jwt_private.pem -pubout -out keys/jwt_public.pem
chmod 600 keys/jwt_private.pem

# 4. Run migrations
make migrate-up

# 5. Start the server
make run
```

### Local Setup (Containerized Dependencies)

Run only the backing services in containers:

```bash
# Start PostgreSQL, Redis, NATS in containers
make deps-up

# Run the Go server locally
make run
```

### Development URLs

| Service | URL |
|---------|-----|
| API Server | http://localhost:8080 |
| Health Check | http://localhost:8080/health |
| PostgreSQL | localhost:5432 |
| Redis | localhost:6379 |
| NATS | localhost:4222 |
| NATS Monitoring | http://localhost:8222 |

## Staging Environment

### Target: `sta.seller.hxd3v.com`

```bash
# 1. SSH into server
ssh helix@<server-ip>

# 2. Clone or pull latest
cd /var/lib/helix-seller
git pull origin main

# 3. Set up environment
cp .env.example .env.staging
# Configure for staging (see Environment Variables below)

# 4. Deploy
podman-compose -f docker-compose.yml --env-file .env.staging up -d

# 5. Verify
curl -k https://sta.seller.hxd3v.com/health
```

### Staging-Specific Configuration

```env
# .env.staging
SERVER_PORT=8080
DATABASE_URL=postgresql://helix:<password>@localhost:5432/helix_seller_staging
REDIS_URL=redis://localhost:6379/1
NATS_URL=nats://localhost:4222
LOG_LEVEL=debug
LOG_FORMAT=json
```

## Production Environment

### Target: `seller.hxd3v.com`

```bash
# 1. SSH into server
ssh helix@<server-ip>

# 2. Deploy latest release
cd /var/lib/helix-seller
git fetch --tags
git checkout v<version>

# 3. Build Go binary
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags="-s -w" -o bin/helix-seller cmd/server/main.go

# 4. Pull container images
podman-compose pull

# 5. Run migrations
podman-compose exec helix-api ./helix-seller migrate up

# 6. Restart services
podman-compose up -d --force-recreate

# 7. Verify
curl https://seller.hxd3v.com/health
```

## Environment Variables

All variables from `.env.example`:

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8080` | HTTP listen port |
| `SERVER_HTTP3_PORT` | `8443` | HTTP/3 (QUIC) port |
| `SERVER_HOST` | `0.0.0.0` | Bind address |

### Database

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgresql://helix:helix@localhost:5432/helix_seller` | PostgreSQL connection string |

### Redis

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | `redis://localhost:6379` | Redis connection string |

### NATS

| Variable | Default | Description |
|----------|---------|-------------|
| `NATS_URL` | `nats://localhost:4222` | NATS connection string |

### JWT Authentication

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_PRIVATE_KEY_PATH` | `keys/jwt_private.pem` | RS256 private key |
| `JWT_PUBLIC_KEY_PATH` | `keys/jwt_public.pem` | RS256 public key |
| `JWT_ACCESS_EXPIRY` | `15m` | Access token TTL |
| `JWT_REFRESH_EXPIRY` | `168h` | Refresh token TTL (7 days) |

### Logging

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | `json` or `console` |

### Encryption

| Variable | Default | Description |
|----------|---------|-------------|
| `ENCRYPTION_KEY` | `000...000` | AES-256-GCM key (64 hex chars / 32 bytes) |

### Payment Providers

| Variable | Default | Description |
|----------|---------|-------------|
| `STRIPE_API_KEY` | `sk_test_xxx` | Stripe secret key |
| `STRIPE_WEBHOOK_SECRET` | `whsec_xxx` | Stripe webhook verification |
| `PAYPAL_CLIENT_ID` | — | PayPal OAuth client ID |
| `PAYPAL_SECRET` | — | PayPal OAuth secret |
| `PAYPAL_WEBHOOK_ID` | — | PayPal webhook ID |
| `SQUARE_ACCESS_TOKEN` | — | Square API token |
| `SQUARE_APPLICATION_ID` | — | Square application ID |
| `SQUARE_WEBHOOK_SIGNATURE_KEY` | — | Square webhook verification |

### Application Tuning

| Variable | Default | Description |
|----------|---------|-------------|
| `RATE_LIMIT_RPS` | `100` | Requests per second per IP |
| `BACKGROUND_WORKERS` | `4` | Background worker goroutine count |
| `BACKGROUND_POLL_INTERVAL` | `5s` | Worker polling interval |
| `IDEMPOTENCY_TTL_HOURS` | `24` | Idempotency key TTL |
| `RECONCILIATION_INTERVAL` | `1h` | Payment reconciliation interval |

### External Services

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENDESIGN_URL` | `http://localhost:3000` | OpenDesign server URL |

## Secrets Management

### JWT Keys

Generate RS256 key pair:

```bash
mkdir -p keys
openssl genrsa -out keys/jwt_private.pem 2048
openssl rsa -in keys/jwt_private.pem -pubout -out keys/jwt_public.pem
chmod 600 keys/jwt_private.pem
```

### Encryption Key

Generate a 32-byte (64 hex character) key:

```bash
openssl rand -hex 32
```

### Production Secrets

- Store secrets in `.env.production` with restricted permissions (`chmod 600`)
- Never commit `.env*` files to git
- Use different keys/passwords for each environment
- Rotate payment provider keys periodically

```bash
chmod 600 .env.production
chown helix:helix .env.production
```

## TLS Certificate Setup

### Using acme.sh

```bash
# Install acme.sh
curl https://get.acme.sh | sh -s email=admin@hxd3v.com

# Issue certificates
~/.acme.sh/acme.sh --issue -d seller.hxd3v.com --standalone
~/.acme.sh/acme.sh --issue -d sta.seller.hxd3v.com --standalone
~/.acme.sh/acme.sh --issue -d dev.seller.hxd3v.com --standalone

# Install certificates
~/.acme.sh/acme.sh --install-cert -d seller.hxd3v.com \
  --key-file /var/lib/helix-seller/keys/seller.key \
  --fullchain-file /var/lib/helix-seller/keys/seller.crt

# Auto-renewal is handled by acme.sh cron job
# Verify: crontab -l | grep acme
```

### Using Caddy (Automatic)

If using Caddy as reverse proxy, TLS is automatic:

```
# Caddyfile
seller.hxd3v.com {
    reverse_proxy helix-api:8080
}

sta.seller.hxd3v.com {
    reverse_proxy helix-api:8080
}

dev.seller.hxd3v.com {
    reverse_proxy helix-api:8080
}
```

Caddy automatically provisions and renews Let's Encrypt certificates.
