# Cheaper Vision Integration - API Reference

Module: `digital.vasic.helixqa`

All endpoints are served on the configured port (default `8080`). Request and response bodies are JSON. All timestamps are RFC 3339.

## Table of Contents

1. [Vision Analysis](#1-vision-analysis)
2. [Provider Management](#2-provider-management)
3. [Learning System](#3-learning-system)
4. [Health and Metrics](#4-health-and-metrics)
5. [Error Codes](#5-error-codes)

---

## 1. Vision Analysis

### POST /api/v1/vision/analyze

Analyze an image with a text prompt. The image must be Base64-encoded (standard encoding, no line breaks). The response reflects whether the result was served from cache or from a live provider call.

**Request**

```json
{
  "image": "<base64-encoded PNG, JPEG, or WebP>",
  "prompt": "Find the login button and return its screen coordinates",
  "options": {
    "strategy": "first_success",
    "timeout": "30s",
    "providers": ["glm-4v", "qwen2.5-vl"]
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `image` | string | Yes | Base64-encoded image bytes |
| `prompt` | string | Yes | Text instruction for the vision model |
| `options` | object | No | Per-call overrides; omit to use server defaults |
| `options.strategy` | string | No | `first_success`, `parallel`, `fallback`, or `weighted`. Defaults to the server-configured strategy |
| `options.timeout` | string | No | Go duration string (e.g. `"30s"`, `"2m"`). Overrides `HELIX_VISION_TIMEOUT` for this call |
| `options.providers` | array of string | No | Restrict this call to the named subset of providers |

**Response — cache miss (live provider call)**

```json
{
  "text": "The login button is located in the lower-right area at approximately (720, 480)",
  "provider": "glm-4v",
  "model": "glm-4v-flash",
  "duration_ms": 834,
  "cache_hit": false,
  "confidence": 0.0,
  "timestamp": "2026-04-13T14:22:01Z"
}
```

**Response — cache hit (L1 exact)**

```json
{
  "text": "The login button is located in the lower-right area at approximately (720, 480)",
  "provider": "glm-4v",
  "model": "glm-4v-flash",
  "duration_ms": 0,
  "cache_hit": true,
  "confidence": 0.0,
  "timestamp": "2026-04-13T14:22:01Z"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `text` | string | Primary textual interpretation returned by the provider |
| `provider` | string | Registered name of the provider that produced the result |
| `model` | string | Exact model identifier used for this call |
| `duration_ms` | integer | Wall-clock duration of the provider call in milliseconds. 0 on a cache hit |
| `cache_hit` | boolean | `true` when the result was served from any cache layer |
| `confidence` | float | Normalised [0, 1] confidence score. Providers that do not report confidence return 0 |
| `timestamp` | string | RFC 3339 time when the analysis call was initiated |

**Error responses**

```json
{
  "error": "cheaper: all providers failed: glm4v: API returned status 401: Unauthorized"
}
```

```json
{
  "error": "cheaper: image decoding failed: image: unknown format"
}
```

---

## 2. Provider Management

### GET /api/v1/providers

List all registered providers with their current health status and capability profile.

**Response**

```json
{
  "providers": [
    {
      "name": "glm-4v",
      "enabled": true,
      "healthy": true,
      "circuit_breaker_state": "closed",
      "capabilities": {
        "supports_streaming": false,
        "max_image_size": 10485760,
        "supported_formats": ["png", "jpg", "jpeg", "webp"],
        "average_latency_ms": 1000,
        "supports_batch": false,
        "cost_per_1m_tokens": 0
      }
    },
    {
      "name": "qwen2.5-vl",
      "enabled": true,
      "healthy": true,
      "circuit_breaker_state": "closed",
      "capabilities": {
        "supports_streaming": false,
        "max_image_size": 20971520,
        "supported_formats": ["png", "jpg", "jpeg", "webp", "gif"],
        "average_latency_ms": 3000,
        "supports_batch": false,
        "cost_per_1m_tokens": 0
      }
    },
    {
      "name": "ui-tars-1.5",
      "enabled": false,
      "healthy": false,
      "circuit_breaker_state": "open",
      "capabilities": {
        "supports_streaming": false,
        "max_image_size": 20971520,
        "supported_formats": ["png", "jpg", "jpeg", "webp"],
        "average_latency_ms": 2000,
        "supports_batch": false,
        "cost_per_1m_tokens": 0
      }
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Registered provider identifier |
| `enabled` | boolean | Whether the provider participates in execution |
| `healthy` | boolean | Result of the most recent `HealthCheck` call |
| `circuit_breaker_state` | string | `"closed"`, `"half-open"`, `"open"`, or `"unknown"` (when circuit breakers are disabled) |
| `capabilities.max_image_size` | integer | Maximum accepted image payload in bytes |
| `capabilities.supported_formats` | array of string | Accepted image format identifiers |
| `capabilities.average_latency_ms` | integer | Expected round-trip time in milliseconds |
| `capabilities.cost_per_1m_tokens` | float | USD cost per 1 million tokens. 0 for free or locally-hosted providers |

---

### POST /api/v1/providers/{name}/enable

Enable a previously disabled provider. The provider begins participating in execution immediately after this call succeeds.

**Path parameter:** `name` — the registered provider name (e.g. `glm-4v`, `ui-tars-1.5`)

**Request body:** empty

**Response**

```json
{
  "name": "ui-tars-1.5",
  "enabled": true
}
```

**Error**

```json
{
  "error": "provider \"unknown-provider\" is not registered"
}
```

---

### POST /api/v1/providers/{name}/disable

Disable a provider. The provider is immediately excluded from all execution strategies. Its circuit breaker state and metrics are preserved.

**Path parameter:** `name` — the registered provider name

**Request body:** empty

**Response**

```json
{
  "name": "ui-tars-1.5",
  "enabled": false
}
```

**Error**

```json
{
  "error": "provider \"unknown-provider\" is not registered"
}
```

---

## 3. Learning System

### GET /api/v1/learning/stats

Return statistics for all active learning layers.

**Response**

```json
{
  "exact_cache_size": 1523,
  "vector_memory_size": 8942,
  "provider_metrics": {
    "glm-4v": {
      "provider_name": "glm-4v",
      "total_requests": 4821,
      "successful_requests": 4780,
      "failed_requests": 41,
      "avg_latency_ms": 834,
      "button_accuracy": 0.94,
      "text_field_accuracy": 0.91,
      "image_accuracy": 0.88,
      "general_accuracy": 0.92,
      "last_used": "2026-04-13T14:22:01Z"
    },
    "qwen2.5-vl": {
      "provider_name": "qwen2.5-vl",
      "total_requests": 312,
      "successful_requests": 307,
      "failed_requests": 5,
      "avg_latency_ms": 2910,
      "button_accuracy": 0.89,
      "text_field_accuracy": 0.86,
      "image_accuracy": 0.91,
      "general_accuracy": 0.88,
      "last_used": "2026-04-13T14:19:45Z"
    }
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `exact_cache_size` | integer | Number of entries currently in the L1 exact cache |
| `vector_memory_size` | integer | Number of documents in the L3 chromem-go collection |
| `provider_metrics` | object | Map from provider name to `ProviderMetrics` |
| `provider_metrics.*.total_requests` | integer | Total calls dispatched to this provider |
| `provider_metrics.*.successful_requests` | integer | Calls that returned a non-error result |
| `provider_metrics.*.failed_requests` | integer | Calls that returned an error |
| `provider_metrics.*.avg_latency_ms` | integer | Exponential moving average of response latency |
| `provider_metrics.*.button_accuracy` | float | EMA accuracy for button identification tasks (signal: 1.0=success, 0.0=fail, alpha=0.1) |
| `provider_metrics.*.text_field_accuracy` | float | EMA accuracy for text field identification tasks |
| `provider_metrics.*.image_accuracy` | float | EMA accuracy for image/media element tasks |
| `provider_metrics.*.general_accuracy` | float | EMA accuracy for all other task types |
| `provider_metrics.*.last_used` | string | RFC 3339 timestamp of the most recent call |

---

### POST /api/v1/learning/clear

Clear all learning state: L1 exact cache, L2 differential cache, and L3 vector memory. Provider optimizer metrics are also reset. This operation is irreversible unless the vector memory was persisted to disk before the call.

**Request body:** empty

**Response**

```json
{
  "cleared": true,
  "exact_cache_entries_removed": 1523,
  "vector_memory_entries_removed": 8942
}
```

**Error**

```json
{
  "error": "memory: delete collection: chromem: collection not found"
}
```

---

## 4. Health and Metrics

### GET /health

Health check endpoint. Returns HTTP 200 when the server is running. Per-provider health is determined by calling `VisionProvider.HealthCheck` with a 5-second timeout.

**Response — all providers healthy**

```json
{
  "status": "healthy",
  "providers": {
    "glm-4v": "healthy",
    "qwen2.5-vl": "healthy",
    "showui-2b": "healthy"
  }
}
```

**Response — one or more providers degraded (HTTP 200 still returned)**

```json
{
  "status": "degraded",
  "providers": {
    "glm-4v": "healthy",
    "qwen2.5-vl": "healthy",
    "ui-tars-1.5": "unhealthy: uitars: health check returned status 503"
  }
}
```

The overall `status` field is `"healthy"` when all enabled providers pass their health checks, `"degraded"` when at least one fails but at least one passes, and `"unhealthy"` when all enabled providers fail.

---

### GET /metrics

Prometheus text-format metrics endpoint. Scrape this with your Prometheus instance.

**Response** (truncated example)

```
# HELP cheaper_vision_requests_total Total number of vision analysis requests dispatched to a provider.
# TYPE cheaper_vision_requests_total counter
cheaper_vision_requests_total{provider="glm-4v"} 4821
cheaper_vision_requests_total{provider="qwen2.5-vl"} 312

# HELP cheaper_vision_cache_hits_total Total number of cache hits by cache layer (exact, differential, vector).
# TYPE cheaper_vision_cache_hits_total counter
cheaper_vision_cache_hits_total{layer="differential"} 89
cheaper_vision_cache_hits_total{layer="exact"} 312
cheaper_vision_cache_hits_total{layer="vector"} 44

# HELP cheaper_vision_request_duration_seconds Wall-clock duration of vision analysis calls in seconds.
# TYPE cheaper_vision_request_duration_seconds histogram
cheaper_vision_request_duration_seconds_bucket{provider="glm-4v",le="0.1"} 0
cheaper_vision_request_duration_seconds_bucket{provider="glm-4v",le="0.25"} 3
cheaper_vision_request_duration_seconds_bucket{provider="glm-4v",le="0.5"} 182
cheaper_vision_request_duration_seconds_bucket{provider="glm-4v",le="1"} 4109
cheaper_vision_request_duration_seconds_bucket{provider="glm-4v",le="2.5"} 4780
cheaper_vision_request_duration_seconds_bucket{provider="glm-4v",le="+Inf"} 4821
cheaper_vision_request_duration_seconds_sum{provider="glm-4v"} 4025.2
cheaper_vision_request_duration_seconds_count{provider="glm-4v"} 4821

# HELP cheaper_vision_circuit_breaker_state Current circuit breaker state per provider: 0=closed, 1=half-open, 2=open.
# TYPE cheaper_vision_circuit_breaker_state gauge
cheaper_vision_circuit_breaker_state{provider="glm-4v"} 0
cheaper_vision_circuit_breaker_state{provider="qwen2.5-vl"} 0
cheaper_vision_circuit_breaker_state{provider="ui-tars-1.5"} 2
```

Recommended Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: "helixqa-vision"
    static_configs:
      - targets: ["localhost:8080"]
    metrics_path: /metrics
    scrape_interval: 15s
```

---

## 5. Error Codes

All error responses use standard HTTP status codes and include a JSON body with an `error` field containing a human-readable message.

| HTTP Status | Meaning | Common Causes |
|-------------|---------|---------------|
| `400 Bad Request` | Malformed request | Missing `image` or `prompt` field; invalid Base64; unknown strategy name |
| `404 Not Found` | Resource not found | Provider name in path does not match any registered provider |
| `422 Unprocessable Entity` | Valid JSON but semantically invalid | Unsupported image format; image exceeds provider's `max_image_size` |
| `500 Internal Server Error` | All providers failed | Every provider in the chain returned an error or has an open circuit breaker |
| `503 Service Unavailable` | No providers available | All providers are disabled or have open circuit breakers |

Error body structure:

```json
{
  "error": "<descriptive message including the provider name and root cause>"
}
```

Provider-specific errors are wrapped with context so the originating provider is always identifiable:

```
cheaper: all providers failed: glm4v: API returned status 429: rate limit exceeded
cheaper: fallback chain exhausted: qwen2.5-vl: HTTP request: dial tcp 127.0.0.1:9192: connect: connection refused
cheaper: provider "unknown-name" not found
```
