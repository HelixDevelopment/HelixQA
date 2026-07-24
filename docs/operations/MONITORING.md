# Monitoring and Alerting Runbook

Monitoring, alerting, and observability procedures for the Helix Seller platform.

---

## Architecture Overview

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Application │────>│ Prometheus  │────>│   Grafana   │
│  (metrics)   │     │  (scrape)   │     │ (dashboards)│
└─────────────┘     └─────────────┘     └─────────────┘
       │                   │
       │                   ▼
       │             ┌─────────────┐
       │             │ Alertmanager│
       │             │  (alerts)   │
       │             └──────┬──────┘
       │                    │
       ▼                    ▼
┌─────────────┐     ┌─────────────┐
│  App Logs   │     │  Notification│
│  (stdout)   │     │ (Slack/email)│
└─────────────┘     └─────────────┘
       │
       ▼
┌─────────────┐
│ Loki / Promtail│
│ (log aggregate)│
└─────────────┘
```

---

## Grafana Dashboard Guide

### Essential Dashboards

| Dashboard | Purpose | Refresh |
|-----------|---------|---------|
| Application Overview | Request rate, error rate, latency | 30s |
| Database Health | Connections, query latency, locks | 30s |
| Infrastructure | CPU, memory, disk, network | 15s |
| Business Metrics | Orders, revenue, active users | 60s |

### Accessing Grafana

```bash
# Local
open http://localhost:3000

# Production
open https://grafana.helixseller.com
```

Default credentials: Check `.env` or vault. Change on first login.

### Key Panels to Monitor

**Application Health:**
- Request rate (req/s) — should be stable during normal operation
- Error rate (5xx/total) — alert if > 1%
- P95/P99 latency — alert if P99 > 500ms

**Database:**
- Active connections — alert if > 80% of max
- Query duration (p95) — alert if > 100ms
- Dead tuples — alert if > 10000

**System:**
- CPU utilization — alert if > 85% sustained
- Memory usage — alert if > 90%
- Disk usage — alert if > 85%

---

## Prometheus Alert Rules

### Application Alerts

```yaml
# /etc/prometheus/rules/helix-seller.yml
groups:
  - name: helix-seller-app
    rules:
      - alert: HighErrorRate
        expr: |
          sum(rate(http_requests_total{status=~"5.."}[5m]))
          /
          sum(rate(http_requests_total[5m]))
          > 0.01
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate on {{ $labels.instance }}"
          description: "5xx error rate is {{ $value | humanizePercentage }}"

      - alert: HighLatency
        expr: |
          histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le)) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High P99 latency on {{ $labels.instance }}"
          description: "P99 latency is {{ $value }}s"

      - alert: ServiceDown
        expr: up{job="helix-seller"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Service down on {{ $labels.instance }}"
          description: "Health endpoint unreachable for >1 minute"
```

### Database Alerts

```yaml
  - name: helix-seller-db
    rules:
      - alert: TooManyConnections
        expr: |
          pg_stat_activity_count > (pg_settings_max_connections * 0.8)
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Database connections near limit"
          description: "{{ $value }} active connections"

      - alert: SlowQueries
        expr: |
          pg_stat_activity_max_tx_duration{datname="helix_seller"} > 30
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Long-running query detected"
          description: "Query running for {{ $value }}s"

      - alert: Deadlocks
        expr: increase(pg_stat_database_deadlocks{datname="helix_seller"}[5m]) > 0
        labels:
          severity: critical
        annotations:
          summary: "Deadlock detected in helix_seller"
          description: "{{ $value }} deadlocks in last 5 minutes"

      - alert: DatabaseDiskSpace
        expr: |
          (pg_database_size_bytes{datname="helix_seller"}
          / pg_tablespace_bytes{spcname="pg_default"}) > 0.85
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Database disk usage high"
```

### Infrastructure Alerts

```yaml
  - name: infrastructure
    rules:
      - alert: HighCPU
        expr: 100 - (avg by(instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100) > 85
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High CPU on {{ $labels.instance }}"

      - alert: HighMemory
        expr: |
          (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100 > 90
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory on {{ $labels.instance }}"

      - alert: DiskSpaceLow
        expr: |
          (1 - node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"}) * 100 > 85
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Disk space low on {{ $labels.instance }}"
```

### Redis Alerts

```yaml
  - name: helix-seller-redis
    rules:
      - alert: RedisDown
        expr: redis_up == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Redis is down"

      - alert: RedisHighMemory
        expr: |
          redis_memory_used_bytes / redis_memory_max_bytes > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Redis memory usage high"
```

---

## Log Aggregation and Search

### Log Format

Application logs use JSON format (via zap):

```json
{
  "time": "2026-07-23T14:30:22Z",
  "level": "info",
  "caller": "handler/order.go:42",
  "msg": "Order created",
  "order_id": "ord_abc123",
  "user_id": "usr_xyz789",
  "amount": 99.99
}
```

### Searching Logs

```bash
# Recent errors
journalctl -u helix-seller --since "1 hour ago" --priority=err

# Specific order
journalctl -u helix-seller | grep "ord_abc123"

# Payment failures
journalctl -u helix-seller | grep -E "(payment.*fail|stripe.*error|paypal.*error)"

# Slow requests
journalctl -u helix-seller | grep -E "duration.*[0-9]+s" | grep -v "duration.*0s"
```

### Loki Query Examples (if using Loki)

```logql
# Errors in last hour
{job="helix-seller"} | json | level="error" | json | msg=~".*"

# Specific user activity
{job="helix-seller"} | json | user_id="usr_xyz789"

# Payment provider errors
{job="helix-seller"} | json | msg=~"payment.*fail|provider.*error"
```

---

## Performance Baseline Metrics

### Normal Operating Ranges

| Metric | Normal Range | Alert Threshold |
|--------|-------------|-----------------|
| Request rate | 50-200 req/s | ±50% deviation |
| Error rate (5xx) | <0.1% | >1% |
| P50 latency | <50ms | >100ms |
| P95 latency | <100ms | >250ms |
| P99 latency | <200ms | >500ms |
| DB connections | 10-50 | >80% of max |
| DB query time (p95) | <50ms | >100ms |
| Redis memory | <512MB | >1GB |
| CPU usage | 20-60% | >85% |
| Memory usage | 40-70% | >90% |
| Disk usage | <70% | >85% |

### Establishing Baselines

After deployment, capture baseline metrics:

```bash
# Request rate baseline (run for 5 minutes)
curl -s http://localhost:9090/api/v1/query?query=rate(http_requests_total[5m]) | jq .

# Latency baseline
curl -s "http://localhost:9090/api/v1/query?query=histogram_quantile(0.99,sum(rate(http_request_duration_seconds_bucket[5m]))by(le))" | jq .
```

---

## Capacity Planning

### Resource Monitoring

```bash
# Current resource utilization
echo "=== CPU ==="
uptime

echo "=== Memory ==="
free -h

echo "=== Disk ==="
df -h /var/lib/postgresql
df -h /

echo "=== Connections ==="
ss -s
```

### Scaling Triggers

| Resource | Current | Trigger | Action |
|----------|---------|---------|--------|
| CPU | 4 cores | >70% avg | Add 2 cores |
| Memory | 8GB | >80% avg | Add 4GB |
| Disk | 100GB | >75% | Add 50GB |
| DB connections | 100 max | >60% avg | Increase max or add read replica |
| Redis memory | 1GB | >70% | Upgrade or add shard |

### Load Testing

```bash
# Install hey (HTTP load generator)
go install github.com/rakyll/hey@latest

# Baseline load test (100 concurrent, 1000 requests)
hey -n 1000 -c 100 -m GET http://localhost:8080/health

# Sustained load (30 seconds)
hey -z 30s -c 50 -m GET http://localhost:8080/api/v1/products
```

---

## Incident Correlation

### Combining Metrics During Incidents

When investigating an incident, check these signals together:

```bash
# 1. Application errors during timeframe
curl -s "http://localhost:9090/api/v1/query_range?query=sum(rate(http_requests_total{status=~'5..'}[1m]))&start=$(date -d '1 hour ago' +%s)&end=$(date +%s)&step=60" | jq .

# 2. Database query latency
curl -s "http://localhost:9090/api/v1/query_range?query=histogram_quantile(0.95,sum(rate(pg_stat_activity_max_tx_duration[1m]))by(le))&start=$(date -d '1 hour ago' +%s)&end=$(date +%s)&step=60" | jq .

# 3. System metrics
curl -s "http://localhost:9090/api/v1/query_range?query=100-avg(irate(node_cpu_seconds_total{mode='idle'}[5m]))*100&start=$(date -d '1 hour ago' +%s)&end=$(date +%s)&step=60" | jq .

# 4. Log error count
journalctl -u helix-seller --since "1 hour ago" --priority=err | wc -l
```

### Correlation Checklist

When an alert fires:

- [ ] Check if correlated with system resource spike
- [ ] Check if correlated with deployment (recent code changes)
- [ ] Check if correlated with database issues
- [ ] Check if correlated with external provider outage
- [ ] Check if correlated with traffic spike ( legitimate or DDoS)
- [ ] Check if correlated with infrastructure change (DNS, network)

### Common Correlation Patterns

| Symptom | Likely Correlated With |
|---------|----------------------|
| 500 errors + high CPU | Infinite loop, missing cache, N+1 query |
| Slow responses + DB locks | Long-running migration, deadlocking transactions |
| Connection refused + high memory | OOM kill, connection pool exhaustion |
| 502/503 + normal CPU | Upstream timeout, connection pool full |
| Intermittent errors + network | DNS issues, firewall rules, provider outage |
