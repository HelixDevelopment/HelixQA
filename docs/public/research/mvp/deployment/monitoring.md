# Monitoring & Alerting

## Observability Stack

| Component | Tool | Purpose |
|-----------|------|---------|
| Metrics | Prometheus + Grafana | System and application metrics |
| Logging | Structured JSON (zap) | Application log aggregation |
| Tracing | OpenTelemetry (planned) | Distributed request tracing |
| Health | `/health` endpoint | Liveness and readiness checks |

## OpenTelemetry Setup

### Integration (Planned)

```go
// pkg/telemetry/telemetry.go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/trace"
)

func Init(serviceName, collectorURL string) (*trace.TracerProvider, error) {
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(collectorURL),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(serviceName),
        )),
    )

    otel.SetTracerProvider(tp)
    return tp, nil
}
```

### Environment Variables

```env
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
OTEL_SERVICE_NAME=helix-seller
OTEL_RESOURCE_ATTRIBUTES=environment=production,version=1.0.0
```

## Prometheus Metrics

### Configuration

```yaml
# config/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: "helix-api"
    static_configs:
      - targets: ["helix-api:8080"]
    metrics_path: "/metrics"
    scrape_interval: 10s

  - job_name: "node-exporter"
    static_configs:
      - targets: ["localhost:9100"]

  - job_name: "postgres-exporter"
    static_configs:
      - targets: ["localhost:9187"]

  - job_name: "redis-exporter"
    static_configs:
      - targets: ["localhost:9121"]

  - job_name: "nats"
    static_configs:
      - targets: ["localhost:8222"]

rule_files:
  - "alert_rules.yml"

alerting:
  alertmanagers:
    - static_configs:
        - targets: ["localhost:9093"]
```

### Application Metrics (Go)

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "helix_http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "helix_http_request_duration_seconds",
            Help:    "HTTP request duration",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path"},
    )

    dbQueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "helix_db_query_duration_seconds",
            Help:    "Database query duration",
            Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
        },
        []string{"operation"},
    )

    activeConnections = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "helix_active_connections",
            Help: "Number of active connections",
        },
    )

    backgroundJobsProcessed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "helix_background_jobs_total",
            Help: "Background jobs processed",
        },
        []string{"type", "status"},
    )
)

func init() {
    prometheus.MustRegister(
        httpRequestsTotal,
        httpRequestDuration,
        dbQueryDuration,
        activeConnections,
        backgroundJobsProcessed,
    )
}
```

### Metrics Endpoint

```
GET /metrics
# Returns Prometheus-format metrics
```

## Grafana Dashboards

### Dashboard Provisioning

```yaml
# config/grafana/provisioning/dashboards/dashboards.yml
apiVersion: 1

providers:
  - name: "Helix Seller"
    orgId: 1
    folder: "Helix"
    type: file
    disableDeletion: false
    editable: true
    options:
      path: /etc/grafana/provisioning/dashboards
      foldersFromFilesStructure: false
```

### Key Dashboards

| Dashboard | Metrics |
|-----------|---------|
| API Overview | Request rate, latency (p50/p95/p99), error rate |
| Database | Connection pool, query duration, active queries |
| Redis | Hit rate, memory usage, connected clients |
| NATS | Message rate, queue depth, consumer lag |
| System | CPU, memory, disk I/O, network |

### Sample Dashboard JSON

```json
{
  "dashboard": {
    "title": "Helix Seller API",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [{
          "expr": "rate(helix_http_requests_total[5m])",
          "legendFormat": "{{method}} {{path}} {{status}}"
        }]
      },
      {
        "title": "Response Time (p95)",
        "type": "graph",
        "targets": [{
          "expr": "histogram_quantile(0.95, rate(helix_http_request_duration_seconds_bucket[5m]))",
          "legendFormat": "{{method}} {{path}}"
        }]
      },
      {
        "title": "Error Rate",
        "type": "graph",
        "targets": [{
          "expr": "rate(helix_http_requests_total{status=~'5..'}[5m]) / rate(helix_http_requests_total[5m])",
          "legendFormat": "{{method}} {{path}}"
        }]
      }
    ]
  }
}
```

## Alert Rules

### Prometheus Alert Rules

```yaml
# config/prometheus/alert_rules.yml
groups:
  - name: helix-alerts
    rules:
      # API Health
      - alert: APIDown
        expr: up{job="helix-api"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Helix API is down"
          description: "API server has been unreachable for 1 minute"

      - alert: HighErrorRate
        expr: rate(helix_http_requests_total{status=~"5.."}[5m]) / rate(helix_http_requests_total[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High 5xx error rate"
          description: "Error rate is {{ $value | humanizePercentage }}"

      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(helix_http_request_duration_seconds_bucket[5m])) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High API latency (p95 > 1s)"

      # Database
      - alert: PostgreSQLDown
        expr: up{job="postgres-exporter"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "PostgreSQL is down"

      - alert: PostgreSQLHighConnections
        expr: pg_stat_activity_count > 80
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "PostgreSQL connection count high"

      # Redis
      - alert: RedisDown
        expr: up{job="redis-exporter"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Redis is down"

      - alert: RedisHighMemory
        expr: redis_memory_used_bytes / redis_memory_max_bytes > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Redis memory usage > 90%"

      # System
      - alert: HighCPU
        expr: 100 - (avg by(instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100) > 85
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "CPU usage > 85%"

      - alert: DiskSpaceLow
        expr: (node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"}) < 0.15
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Disk space < 15%"

      - alert: NVMeWearHigh
        expr: nvme_smart_temperature_celsius > 70
        for: 30m
        labels:
          severity: warning
        annotations:
          summary: "NVMe temperature high"
```

## Log Aggregation

### Structured Logging

All application logs use structured JSON format via `go.uber.org/zap`:

```json
{
  "time": "2026-07-23T12:00:00Z",
  "level": "info",
  "msg": "Request processed",
  "method": "POST",
  "path": "/api/v1/orders",
  "status": 201,
  "duration_ms": 45,
  "request_id": "req_abc123"
}
```

### Log Levels

| Level | Usage |
|-------|-------|
| `debug` | Detailed diagnostic information |
| `info` | Normal operation events |
| `warn` | Degraded but functioning |
| `error` | Failures requiring attention |

### Log Collection

```bash
# View logs in real-time
podman-compose logs -f helix-api

# View logs for all services
podman-compose logs -f

# View logs for specific time range
podman-compose logs --since "2026-07-23T10:00:00" helix-api
```

## Health Check Endpoints

### Liveness Check

```
GET /health
```

```json
{
  "status": "healthy",
  "time": "2026-07-23T12:00:00Z"
}
```

### Readiness Check (Planned)

```
GET /ready
```

```json
{
  "status": "ready",
  "checks": {
    "database": "ok",
    "redis": "ok",
    "nats": "ok"
  }
}
```

### Kubernetes-Style Health Probes (for systemd integration)

```bash
#!/bin/bash
# scripts/health-check.sh

# Liveness: is the process running?
curl -sf http://localhost:8080/health > /dev/null 2>&1
exit $?

# Readiness: are dependencies healthy?
curl -sf http://localhost:8080/ready > /dev/null 2>&1
exit $?
```

## Systemd Health Monitoring

```ini
# /etc/systemd/system/helix-seller.service
[Unit]
Description=Helix Seller API
After=network.target podman.socket
Requires=podman.socket

[Service]
Type=simple
User=helix
ExecStart=/usr/bin/podman-compose -f /var/lib/helix-seller/docker-compose.yml up
ExecStop=/usr/bin/podman-compose -f /var/lib/helix-seller/docker-compose.yml down
Restart=on-failure
RestartSec=10

# Health check
WatchdogSec=300
ExecStartPost=/bin/bash -c 'sleep 10 && curl -sf http://localhost:8080/health || exit 1'

[Install]
WantedBy=multi-user.target
```
