# Helix Seller — Deployment Overview

## Infrastructure

- **Host:** Hetzner dedicated server
- **Container Runtime:** Rootless Podman Compose
- **Orchestration:** systemd-managed container stacks

## Environments

| Environment | URL | Purpose |
|-------------|-----|---------|
| Development | dev.seller.hxd3v.com | Active development and feature testing |
| Staging | sta.seller.hxd3v.com | Pre-production validation |
| Production | seller.hxd3v.com | Live customer-facing service |

## Container Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Reverse Proxy (Caddy)                  │
│                   TLS termination + routing               │
└──────────────┬──────────────┬──────────────┬────────────┘
               │              │              │
    ┌──────────▼──────────┐  │  ┌───────────▼──────────┐
    │     Go API Server   │  │  │   OpenDesign Server   │
    │   (HTTP/3, REST)    │  │  │    (Design Tool)      │
    └──────────┬──────────┘  │  └───────────┬──────────┘
               │              │              │
    ┌──────────▼──────────────────────────────▼──────────┐
    │                 Shared Service Layer                 │
    ├─────────────┬──────────┬───────────┬───────────────┤
    │ PostgreSQL  │  Redis   │   NATS    │    MinIO      │
    │  (Primary)  │ (Cache)  │ (Events)  │ (Object Store)│
    └─────────────┴──────────┴───────────┴───────────────┘
               │
    ┌──────────▼──────────┐
    │  Observability Stack │
    │  Prometheus + Grafana│
    └─────────────────────┘
```

## Services

| Service | Image | Port | Purpose |
|---------|-------|------|---------|
| `helix-api` | Custom Go build | 8080 | REST API server |
| `opendesign` | `ghcr.io/nexu-io/open-design:latest` | 3000 | Design tool |
| `postgres` | `postgres:16-alpine` | 5432 | Primary database |
| `redis` | `redis:7-alpine` | 6379 | Cache, sessions, rate limiting |
| `nats` | `nats:2-alpine` | 4222 | Event streaming (JetStream) |
| `minio` | `minio/minio:latest` | 9000/9001 | Object storage (documents, images) |
| `prometheus` | `prom/prometheus:latest` | 9090 | Metrics collection |
| `grafana` | `grafana/grafana:latest` | 3001 | Metrics dashboards |

## Port Mapping

All services bind to `127.0.0.1` except the reverse proxy. Only Caddy exposes public ports.

| Internal Service | Host Port | Binding |
|-----------------|-----------|---------|
| Caddy (HTTPS) | 443 | 0.0.0.0 |
| Caddy (HTTP) | 80 | 0.0.0.0 |
| PostgreSQL | 5432 | 127.0.0.1 |
| Redis | 6379 | 127.0.0.1 |
| NATS | 4222 | 127.0.0.1 |
| MinIO API | 9000 | 127.0.0.1 |
| MinIO Console | 9001 | 127.0.0.1 |
| Prometheus | 9090 | 127.0.0.1 |
| Grafana | 3001 | 127.0.0.1 |

## Deployment Flow

1. Build Go binary locally or in CI
2. Copy binary and configuration to server
3. Run `podman-compose pull` to update images
4. Run `podman-compose up -d` to apply changes
5. Verify health checks via `/health` endpoint

## Documentation

- [Infrastructure Requirements](infrastructure.md)
- [Container Architecture](container-architecture.md)
- [Environment Setup](environment-setup.md)
- [Monitoring & Alerting](monitoring.md)
