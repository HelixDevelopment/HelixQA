# Container Architecture

## Rootless Podman Compose

All containers run rootless via Podman, providing user-namespace isolation without root privileges.

### Container Images

| Service | Image | Tag | Purpose |
|---------|-------|-----|---------|
| `helix-api` | Custom build | `latest` | Go API server (built from Dockerfile) |
| `caddy` | `caddy:2-alpine` | `2` | Reverse proxy + TLS |
| `opendesign` | `ghcr.io/nexu-io/open-design` | `latest` | Design tool |
| `postgres` | `postgres` | `16-alpine` | Primary database |
| `redis` | `redis` | `7-alpine` | Cache layer |
| `nats` | `nats` | `2-alpine` | Event streaming |
| `minio` | `minio/minio` | `latest` | Object storage |
| `prometheus` | `prom/prometheus` | `latest` | Metrics |
| `grafana` | `grafana/grafana` | `latest` | Dashboards |

## Podman Compose File

```yaml
# docker-compose.yml (used with podman-compose)
version: "3.8"

services:
  # ── Application ──────────────────────────────────────

  helix-api:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: helix-api
    labels:
      io.containers.autoupdate: local
    env_file:
      - .env
    ports:
      - "127.0.0.1:8080:8080"
    volumes:
      - ./keys:/app/keys:ro
    networks:
      - helix-net
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      nats:
        condition: service_healthy
    restart: "no"

  caddy:
    image: caddy:2-alpine
    container_name: helix-caddy
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./config/Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
      - caddy-config:/config
    networks:
      - helix-net
    depends_on:
      - helix-api
    restart: "no"

  opendesign:
    image: ghcr.io/nexu-io/open-design:latest
    container_name: helix-opendesign
    labels:
      io.containers.autoupdate: registry
    env_file:
      - .env
    environment:
      - OD_SERVER_PORT=3000
      - OD_SERVER_HOST=0.0.0.0
    ports:
      - "127.0.0.1:3000:3000"
    volumes:
      - opendesign-data:/var/lib/opendesign
    networks:
      - helix-net
    depends_on:
      - postgres
      - redis
    restart: "no"

  # ── Data Stores ──────────────────────────────────────

  postgres:
    image: postgres:16-alpine
    container_name: helix-postgres
    labels:
      io.containers.autoupdate: registry
    environment:
      - POSTGRES_DB=helix_seller
      - POSTGRES_USER=${POSTGRES_USER:-helix}
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    ports:
      - "127.0.0.1:5432:5432"
    networks:
      - helix-net
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-helix} -d helix_seller"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 30s
    restart: "no"

  redis:
    image: redis:7-alpine
    container_name: helix-redis
    labels:
      io.containers.autoupdate: registry
    command: redis-server --appendonly yes --maxmemory 2gb --maxmemory-policy allkeys-lru
    volumes:
      - redis-data:/data
    ports:
      - "127.0.0.1:6379:6379"
    networks:
      - helix-net
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: "no"

  nats:
    image: nats:2-alpine
    container_name: helix-nats
    labels:
      io.containers.autoupdate: registry
    command: >
      --jetstream
      --store_dir /data
      --max_mem 1g
      --max_file 10g
      --port 4222
      --http_port 8222
    volumes:
      - nats-data:/data
    ports:
      - "127.0.0.1:4222:4222"
      - "127.0.0.1:8222:8222"
    networks:
      - helix-net
    healthcheck:
      test: ["CMD", "nats-server", "--signal", "ldm="]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: "no"

  minio:
    image: minio/minio:latest
    container_name: helix-minio
    labels:
      io.containers.autoupdate: registry
    command: server /data --console-address ":9001"
    environment:
      - MINIO_ROOT_USER=${MINIO_ROOT_USER:-minioadmin}
      - MINIO_ROOT_PASSWORD=${MINIO_ROOT_PASSWORD}
    volumes:
      - minio-data:/data
    ports:
      - "127.0.0.1:9000:9000"
      - "127.0.0.1:9001:9001"
    networks:
      - helix-net
    healthcheck:
      test: ["CMD", "mc", "ready", "local"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: "no"

  # ── Observability ─────────────────────────────────────

  prometheus:
    image: prom/prometheus:latest
    container_name: helix-prometheus
    volumes:
      - ./config/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    ports:
      - "127.0.0.1:9090:9090"
    networks:
      - helix-net
    restart: "no"

  grafana:
    image: grafana/grafana:latest
    container_name: helix-grafana
    environment:
      - GF_SECURITY_ADMIN_USER=${GRAFANA_USER:-admin}
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD}
      - GF_SERVER_ROOT_URL=https://sta.seller.hxd3v.com/grafana
    volumes:
      - grafana-data:/var/lib/grafana
      - ./config/grafana/provisioning:/etc/grafana/provisioning:ro
    ports:
      - "127.0.0.1:3001:3000"
    networks:
      - helix-net
    depends_on:
      - prometheus
    restart: "no"

volumes:
  caddy-data:
  caddy-config:
  opendesign-data:
  postgres-data:
  redis-data:
  nats-data:
  minio-data:
  prometheus-data:
  grafana-data:

networks:
  helix-net:
    driver: bridge
```

## Dockerfile

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/helix-seller cmd/server/main.go

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S helix \
    && adduser -S -G helix helix

COPY --from=builder /app/helix-seller /usr/local/bin/helix-seller

USER helix
WORKDIR /app

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["helix-seller"]
```

## Volume Mounts & Persistence

| Volume | Container Path | Purpose | Backup |
|--------|---------------|---------|--------|
| `postgres-data` | `/var/lib/postgresql/data` | Database files | pg_dump daily |
| `redis-data` | `/data` | RDB + AOF persistence | Copy RDB snapshot |
| `nats-data` | `/data` | JetStream streams | Copy directory |
| `minio-data` | `/data` | Object storage | MinIO mirror |
| `caddy-data` | `/data` | TLS certificates | Export from Let's Encrypt |
| `grafana-data` | `/var/lib/grafana` | Dashboard configs | Git-managed provisioning |
| `prometheus-data` | `/prometheus` | Metrics retention | Optional |

## Resource Limits

```yaml
# Add to each service definition
deploy:
  resources:
    limits:
      cpus: "4.0"
      memory: 8G
    reservations:
      cpus: "2.0"
      memory: 2G
```

### Recommended Limits

| Service | CPU Limit | Memory Limit | Notes |
|---------|-----------|--------------|-------|
| `helix-api` | 8 | 4 GB | Scales with worker count |
| `postgres` | 8 | 32 GB | shared_buffers = 8 GB |
| `redis` | 2 | 2 GB | maxmemory matches |
| `nats` | 2 | 2 GB | Memory limits for JetStream |
| `minio` | 4 | 4 GB | Handles file uploads |
| `opendesign` | 4 | 4 GB | UI rendering |

## Health Checks

All services include health checks. The API server exposes a `/health` endpoint:

```json
GET /health
{
  "status": "healthy",
  "time": "2026-07-23T12:00:00Z"
}
```

The `depends_on` with `condition: service_healthy` ensures services start in correct order.
