# Cheaper Vision Integration - Complete Documentation

## Table of Contents

1. [User Guide](#1-user-guide)
2. [Administrator Guide](#2-administrator-guide)
3. [API Reference](#3-api-reference)
4. [Troubleshooting](#4-troubleshooting)
5. [Best Practices](#5-best-practices)

---

## 1. User Guide

### 1.1 Quick Start

#### Installation

```bash
# Clone the repository
git clone https://github.com/HelixDevelopment/helixqa.git
cd helixqa

# Install dependencies
go mod download

# Copy example configuration
cp config.yaml.example config.yaml

# Edit configuration with your API keys
vim config.yaml

# Build and run
make build
./bin/helixqa
```

#### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "image"

    "github.com/HelixDevelopment/helixqa/internal/engine"
)

func main() {
    // Create vision engine
    cfg := engine.LearningVisionConfig{
        EnableExactCache:   true,
        EnableDifferential: true,
        EnableVectorMemory: true,
        EnableFewShot:      true,
    }

    visionEngine, err := engine.NewLearningVisionEngine(cfg)
    if err != nil {
        panic(err)
    }

    // Process an image
    ctx := context.Background()
    img := loadImage("screenshot.png")
    
    result, err := visionEngine.ProcessImage(ctx, img, "Find the login button")
    if err != nil {
        panic(err)
    }

    fmt.Println("Result:", result)
}
```

### 1.2 Configuration Options

#### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HELIX_VISION_PROVIDER` | Primary provider | `auto` |
| `HELIX_VISION_FALLBACK_ENABLED` | Enable fallbacks | `true` |
| `HELIX_VISION_FALLBACK_CHAIN` | Fallback order | `qwen25vl,glm4v,uitars,showui` |
| `HELIX_VISION_PARALLEL_EXECUTION` | Run in parallel | `true` |
| `HELIX_VISION_TIMEOUT` | Request timeout | `30s` |
| `HELIX_VISION_LEARNING_ENABLED` | Enable learning | `true` |
| `HELIX_VISION_EXACT_CACHE` | Enable exact cache | `true` |
| `HELIX_VISION_DIFFERENTIAL` | Enable differential cache | `true` |
| `HELIX_VISION_VECTOR_MEMORY` | Enable vector memory | `true` |
| `HELIX_VISION_FEW_SHOT` | Enable few-shot learning | `true` |
| `HELIX_VISION_PERSIST_PATH` | Memory persistence path | `/var/lib/helixqa/vision_memory.db` |
| `HELIX_VISION_MAX_MEMORIES` | Max stored memories | `100000` |
| `HELIX_VISION_CHANGE_THRESHOLD` | Differential threshold | `0.05` |
| `HELIX_VISION_EMBEDDING_PROVIDER` | Embedding provider | `openai` |
| `HELIX_VISION_EMBEDDING_API_KEY` | Embedding API key | - |

#### Provider-Specific Variables

| Variable | Provider | Required |
|----------|----------|----------|
| `UITARS_API_KEY` | UI-TARS | Yes |
| `GLM4V_API_KEY` | GLM-4V | Yes |
| `OPENAI_API_KEY` | OpenAI Embeddings | If using OpenAI |

### 1.3 Using Different Providers

#### UI-TARS (HuggingFace)

```yaml
vision:
  providers:
    - name: "uitars"
      enabled: true
      config:
        api_key: "${UITARS_API_KEY}"
        base_url: "https://api-inference.huggingface.co"
        model: "ByteDance-Seed/UI-TARS-1.5-7B"
```

#### ShowUI (Local)

```yaml
vision:
  providers:
    - name: "showui"
      enabled: true
      config:
        api_url: "http://localhost:7860/api/predict"
```

#### GLM-4V (Zhipu AI)

```yaml
vision:
  providers:
    - name: "glm4v"
      enabled: true
      config:
        api_key: "${GLM4V_API_KEY}"
        model: "glm-4v-flash"  # Free tier
```

#### Qwen2.5-VL (Local)

```yaml
vision:
  providers:
    - name: "qwen25vl"
      enabled: true
      config:
        base_url: "http://localhost:9192/v1"
        model: "Qwen2.5-VL-7B-Instruct"
```

### 1.4 Execution Strategies

#### First Success (Default)

```yaml
vision:
  strategy: "first_success"
```

Runs all providers in parallel, returns the first successful result. Best for low latency.

#### Fallback Chain

```yaml
vision:
  strategy: "fallback"
  fallback_chain:
    - "qwen25vl"
    - "glm4v"
    - "uitars"
```

Tries providers in order until one succeeds. Best for reliability.

#### Parallel

```yaml
vision:
  strategy: "parallel"
```

Runs all providers, returns the best result. Best for accuracy.

### 1.5 Learning System

The learning system improves performance over time through multiple layers:

#### Layer 1: Exact Cache
- SHA-256 hash of entire image
- O(1) lookup time
- < 1ms response for exact matches

#### Layer 2: Differential Cache
- 24x24 patch-based hashing
- Detects minimal changes between frames
- 10-100x speedup on similar screens

#### Layer 3: Vector Memory (RAG)
- Semantic similarity search
- Embedding-based retrieval
- 5-50x faster than inference

#### Layer 4: Few-Shot Learning
- Retrieves similar successful examples
- Augments prompts with examples
- +5-15% accuracy improvement

#### Layer 5: Provider Optimization
- Tracks per-provider performance
- Routes to best provider per query type
- 10-20% failure reduction

### 1.6 Monitoring

#### Prometheus Metrics

| Metric | Description |
|--------|-------------|
| `vision_requests_total` | Total requests by provider |
| `vision_request_duration_seconds` | Request latency histogram |
| `vision_cache_hits_total` | Cache hits by layer |
| `vision_learning_memories_stored_total` | Memories stored |
| `vision_circuit_breaker_state` | Circuit breaker state |

#### Grafana Dashboard

Access Grafana at `http://localhost:3000` (default: admin/admin)

Pre-configured dashboards:
- Vision Performance
- Provider Health
- Cache Effectiveness
- Learning Progress

---

## 2. Administrator Guide

### 2.1 Deployment Options

#### Docker (Recommended)

```bash
# Build image
make docker

# Run with docker-compose
docker compose -f tests/docker/docker-compose.yml up -d
```

#### Kubernetes

```bash
# Apply manifests
kubectl apply -f deployments/kubernetes/

# Check status
kubectl get pods -n helixqa
```

#### Bare Metal

```bash
# Build binary
make build

# Copy to server
scp bin/helixqa user@server:/opt/helixqa/

# Create systemd service
sudo cp deployments/systemd/helixqa.service /etc/systemd/system/
sudo systemctl enable helixqa
sudo systemctl start helixqa
```

### 2.2 Scaling

#### Horizontal Scaling

```yaml
# docker-compose.yml
services:
  helixqa:
    deploy:
      replicas: 3
    environment:
      - HELIX_VISION_PERSIST_PATH=/shared/vision_memory.db
    volumes:
      - shared_memory:/var/lib/helixqa
```

#### Load Balancing

```nginx
# nginx.conf
upstream helixqa {
    least_conn;
    server helixqa-1:8080;
    server helixqa-2:8080;
    server helixqa-3:8080;
}

server {
    location / {
        proxy_pass http://helixqa;
    }
}
```

### 2.3 Backup and Recovery

#### Vector Memory Backup

```bash
# Automated backup script
#!/bin/bash
BACKUP_DIR="/backups/helixqa"
DATE=$(date +%Y%m%d_%H%M%S)

# Backup vector memory
cp /var/lib/helixqa/vision_memory.db "$BACKUP_DIR/vision_memory_$DATE.db"

# Compress
gzip "$BACKUP_DIR/vision_memory_$DATE.db"

# Keep only last 30 days
find "$BACKUP_DIR" -name "vision_memory_*.db.gz" -mtime +30 -delete
```

#### Restore

```bash
# Stop service
sudo systemctl stop helixqa

# Restore from backup
cp /backups/helixqa/vision_memory_20260115_120000.db.gz /var/lib/helixqa/
gunzip /var/lib/helixqa/vision_memory_20260115_120000.db.gz
mv /var/lib/helixqa/vision_memory_20260115_120000.db /var/lib/helixqa/vision_memory.db

# Start service
sudo systemctl start helixqa
```

### 2.4 Security

#### API Key Management

```bash
# Use environment variables (recommended)
export UITARS_API_KEY="your-key"
export GLM4V_API_KEY="your-key"

# Or use a secrets manager
export UITARS_API_KEY=$(aws secretsmanager get-secret-value --secret-id uitars-key)
```

#### Network Security

```yaml
# docker-compose.yml with network isolation
services:
  helixqa:
    networks:
      - frontend
      - backend

  qwen-vl:
    networks:
      - backend

networks:
  frontend:
    driver: bridge
  backend:
    driver: bridge
    internal: true  # No external access
```

#### TLS Configuration

```yaml
# config.yaml
server:
  tls:
    enabled: true
    cert_file: "/etc/helixqa/server.crt"
    key_file: "/etc/helixqa/server.key"
```

### 2.5 Performance Tuning

#### Cache Tuning

```yaml
vision:
  learning:
    exact_cache:
      max_size: 10000  # Max cached images
      ttl: "1h"        # Time to live
    
    differential:
      change_threshold: 0.05  # 5% pixel change
      patch_size: 24          # 24x24 patches
      ttl: "5m"
    
    vector_memory:
      max_memories: 100000
      similarity_threshold: 0.85
```

#### Circuit Breaker Tuning

```yaml
vision:
  resilience:
    circuit_breaker:
      failure_threshold: 5    # Open after 5 failures
      success_threshold: 3    # Close after 3 successes
      timeout: "30s"          # Wait before half-open
```

#### Provider Weights

```yaml
vision:
  providers:
    - name: "glm4v"
      enabled: true
      priority: 1  # Highest priority
      fallback_to: ["qwen25vl", "uitars"]
    
    - name: "qwen25vl"
      enabled: true
      priority: 2
```

### 2.6 Troubleshooting

#### Common Issues

**Issue: High latency**
```bash
# Check cache hit rate
curl http://localhost:8080/metrics | grep vision_cache_hits

# Enable more aggressive caching
export HELIX_VISION_EXACT_CACHE=true
export HELIX_VISION_DIFFERENTIAL=true
```

**Issue: Provider failures**
```bash
# Check circuit breaker state
curl http://localhost:8080/metrics | grep circuit_breaker

# Reset circuit breakers
curl -X POST http://localhost:8080/admin/reset-circuit-breakers
```

**Issue: Memory usage**
```bash
# Check memory stats
curl http://localhost:8080/api/stats/memory

# Clear caches
curl -X POST http://localhost:8080/admin/clear-caches
```

#### Log Analysis

```bash
# View logs
journalctl -u helixqa -f

# Search for errors
journalctl -u helixqa | grep ERROR

# Filter by provider
journalctl -u helixqa | grep "provider=glm4v"
```

---

## 3. API Reference

### 3.1 Vision Analysis

#### POST /api/v1/vision/analyze

Analyze an image with a prompt.

**Request:**
```json
{
  "image": "base64-encoded-image",
  "prompt": "Find the login button",
  "options": {
    "strategy": "first_success",
    "timeout": "30s"
  }
}
```

**Response:**
```json
{
  "text": "The login button is located at coordinates (100, 50)",
  "provider": "glm-4v",
  "model": "glm-4v-flash",
  "duration_ms": 523,
  "cache_hit": false,
  "confidence": 0.95
}
```

### 3.2 Provider Management

#### GET /api/v1/providers

List all configured providers.

**Response:**
```json
{
  "providers": [
    {
      "name": "glm4v",
      "enabled": true,
      "healthy": true,
      "capabilities": {
        "max_image_size": 10485760,
        "supported_formats": ["png", "jpg", "jpeg"]
      }
    }
  ]
}
```

#### POST /api/v1/providers/{name}/enable

Enable a provider.

#### POST /api/v1/providers/{name}/disable

Disable a provider.

### 3.3 Learning System

#### GET /api/v1/learning/stats

Get learning system statistics.

**Response:**
```json
{
  "exact_cache_size": 1523,
  "vector_memories": 8942,
  "provider_metrics": {
    "glm4v": {
      "success_rate": 0.98,
      "avg_latency_ms": 523
    }
  }
}
```

#### POST /api/v1/learning/clear

Clear all learned memories.

### 3.4 Health & Metrics

#### GET /health

Health check endpoint.

**Response:**
```json
{
  "status": "healthy",
  "providers": {
    "glm4v": "healthy",
    "qwen25vl": "healthy"
  }
}
```

#### GET /metrics

Prometheus metrics endpoint.

---

## 4. Troubleshooting

### 4.1 Provider Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| `uitars: API error 401` | Invalid API key | Check `UITARS_API_KEY` |
| `glm-4v: unhealthy` | Service down | Check Zhipu AI status |
| `qwen2.5-vl: connection refused` | Service not running | Start Qwen service |
| `showui: timeout` | Slow response | Increase timeout |

### 4.2 Performance Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| High latency | Cache miss | Enable exact/differential cache |
| Memory growth | Unbounded cache | Set cache limits |
| CPU spikes | Too many providers | Reduce parallel providers |
| Network errors | Circuit breaker open | Wait for recovery |

### 4.3 Learning System Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| No cache hits | Cache disabled | Enable `HELIX_VISION_EXACT_CACHE` |
| Low similarity scores | Wrong embedding model | Check embedding provider |
| Memory not persisting | Wrong persist path | Verify `HELIX_VISION_PERSIST_PATH` |
| Few-shot not working | No examples stored | Run more queries |

---

## 5. Best Practices

### 5.1 Provider Selection

**For Maximum Cost Savings:**
- Use GLM-4.6V-Flash (FREE)
- Self-host ShowUI-2B or Qwen2.5-VL
- Deploy on existing GPU infrastructure

**For Best Performance:**
- Use UI-TARS-1.5-7B for complex UI tasks
- Enable all cache layers
- Use "first_success" strategy

**For Maximum Reliability:**
- Configure multiple providers
- Use "fallback" strategy
- Enable circuit breakers

### 5.2 Prompt Engineering

**Good Prompts:**
```
"Find the login button and return its coordinates"
"Extract all text from the header section"
"Identify the form fields and their labels"
```

**Bad Prompts:**
```
"What's in this image?"  # Too vague
"Click the button"        # Ambiguous
"Process this"            # No clear task
```

### 5.3 Monitoring Checklist

- [ ] Set up Prometheus scraping
- [ ] Configure Grafana dashboards
- [ ] Set up alerting for provider failures
- [ ] Monitor cache hit rates
- [ ] Track learning system effectiveness
- [ ] Monitor memory usage
- [ ] Set up log aggregation

### 5.4 Security Checklist

- [ ] Rotate API keys regularly
- [ ] Use environment variables for secrets
- [ ] Enable TLS for production
- [ ] Restrict network access
- [ ] Audit provider access
- [ ] Monitor for unusual usage patterns

---

*Documentation Version: 1.0*
*Last Updated: 2026-04-13*
