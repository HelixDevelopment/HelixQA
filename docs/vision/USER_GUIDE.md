# Cheaper Vision Integration - User Guide

## Table of Contents

1. [Quick Start](#1-quick-start)
2. [Configuration](#2-configuration)
3. [Provider Setup](#3-provider-setup)
4. [Execution Strategies](#4-execution-strategies)
5. [5-Layer Learning System](#5-5-layer-learning-system)
6. [Monitoring](#6-monitoring)
7. [Prompt Engineering](#7-prompt-engineering)

---

## 1. Quick Start

### Prerequisites

- Go 1.25 or later
- At least one vision provider accessible (local or cloud)
- `digital.vasic.helixqa` module checked out

### Install Dependencies

```bash
cd HelixQA
go mod download
```

### Minimal Configuration

Create a `.env` file (never commit it — `.gitignore` already covers it):

```bash
# Use GLM-4V free tier as the primary provider
HELIX_VISION_PROVIDER=glm4v
GLM4V_API_KEY=your-zhipu-ai-key-here
```

### Run the Vision Server

```bash
go run ./cmd/helixqa server
```

The server starts on port `8080` by default. Check readiness:

```bash
curl http://localhost:8080/health
```

### First Analysis Call

```bash
curl -s -X POST http://localhost:8080/api/v1/vision/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "image": "'$(base64 -w0 screenshot.png)'",
    "prompt": "Find the login button and return its position"
  }'
```

### Go Integration

```go
package main

import (
    "context"
    "fmt"

    "digital.vasic.helixqa/pkg/vision/cheaper"
    "digital.vasic.helixqa/pkg/vision/cheaper/adapters/glm4v"
)

func main() {
    registry := cheaper.NewRegistry()
    registry.Register("glm-4v", glm4v.NewGLM4VProvider)

    provider, err := registry.Create("glm-4v", map[string]interface{}{
        "api_key": "your-key-here",
        "model":   "glm-4v-flash",
    })
    if err != nil {
        panic(err)
    }

    ctx := context.Background()
    img := loadImage("screenshot.png") // your image loading logic

    result, err := provider.Analyze(ctx, img, "Find the login button")
    if err != nil {
        panic(err)
    }

    fmt.Printf("Provider: %s, Text: %s, Duration: %v\n",
        result.Provider, result.Text, result.Duration)
}
```

---

## 2. Configuration

All configuration is supplied via environment variables. There is no required config file — the server works with only the variables relevant to the providers you enable.

### Core Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HELIX_VISION_PROVIDER` | Primary provider name or `auto` to score all available providers | `auto` |
| `HELIX_VISION_FALLBACK_ENABLED` | Enable automatic provider fallback on failure | `true` |
| `HELIX_VISION_FALLBACK_CHAIN` | Comma-separated provider names tried in order when using the fallback strategy | `qwen25vl,glm4v,uitars,showui` |
| `HELIX_VISION_TIMEOUT` | Per-request wall-clock timeout (Go duration string) | `30s` |
| `HELIX_VISION_LEARNING_ENABLED` | Enable all learning layers | `true` |
| `HELIX_VISION_EXACT_CACHE` | Enable L1 exact-match SHA-256 cache | `true` |
| `HELIX_VISION_DIFFERENTIAL` | Enable L2 differential patch-hash cache | `true` |
| `HELIX_VISION_VECTOR_MEMORY` | Enable L3 chromem-go vector memory store | `true` |
| `HELIX_VISION_FEW_SHOT` | Enable L4 few-shot prompt augmentation | `true` |
| `HELIX_VISION_PERSIST_PATH` | Directory path for L3 vector memory on-disk persistence. Empty = in-memory only | `` (empty) |
| `HELIX_VISION_MAX_MEMORIES` | Maximum number of entries the vector memory store holds | `100000` |
| `HELIX_VISION_CHANGE_THRESHOLD` | Fraction of 24x24 pixel patches that must change before the L2 differential cache misses (value in [0, 1]) | `0.05` |

### Provider API Key Variables

| Variable | Provider | Required |
|----------|----------|----------|
| `UITARS_API_KEY` | UI-TARS via Hugging Face Inference API | Yes |
| `GLM4V_API_KEY` | GLM-4V via Zhipu AI | Yes |

Local providers (ShowUI, Qwen2.5-VL, OmniParser) do not require API keys.

---

## 3. Provider Setup

### UI-TARS 1.5-7B (Hugging Face)

ByteDance's UI-TARS 1.5-7B is served via the Hugging Face Inference API using an OpenAI-compatible chat-completions endpoint. An API token is required.

```yaml
vision:
  providers:
    - name: "ui-tars-1.5"
      enabled: true
      priority: 2
      config:
        api_key: "${UITARS_API_KEY}"
        base_url: "https://api-inference.huggingface.co"
        model: "ByteDance-Seed/UI-TARS-1.5-7B"
        timeout: 60
```

Images are sent as `data:image/png;base64,...` data URIs inside the `image_url` content part. Supported formats: `png`, `jpg`, `jpeg`, `webp`. Maximum image size: 20 MB.

### ShowUI-2B (Local)

ShowUI-2B is a locally-hosted GUI-understanding model. Launch it as a Gradio application; no API key is needed.

```yaml
vision:
  providers:
    - name: "showui-2b"
      enabled: true
      priority: 3
      config:
        api_url: "http://localhost:7860/api/predict"
        timeout: 30
```

The adapter POSTs `{"data": ["<base64_image>", "<prompt>"]}` to the Gradio `/api/predict` endpoint. Health checks perform a GET to the Gradio web UI root. Supported formats: `png`, `jpg`, `jpeg`. Maximum image size: 10 MB.

### GLM-4V-Flash (Zhipu AI — free tier)

Zhipu AI's `glm-4v-flash` model is available on a free tier. The API is OpenAI-compatible with one quirk: the image value must be a raw base64 string without the `data:image/png;base64,` prefix.

```yaml
vision:
  providers:
    - name: "glm-4v"
      enabled: true
      priority: 1
      config:
        api_key: "${GLM4V_API_KEY}"
        base_url: "https://open.bigmodel.cn/api/paas/v4"
        model: "glm-4v-flash"
        timeout: 60
```

To use the paid `glm-4v` model instead, set `model: "glm-4v"`. Supported formats: `png`, `jpg`, `jpeg`, `webp`. Maximum image size: 10 MB.

### Qwen2.5-VL (Local)

Qwen2.5-VL can be self-hosted using vLLM, llama.cpp server, or Ollama with OpenAI compatibility enabled. No API key is required.

```yaml
vision:
  providers:
    - name: "qwen2.5-vl"
      enabled: true
      priority: 2
      config:
        base_url: "http://localhost:9192/v1"
        model: "Qwen2.5-VL-7B-Instruct"
        timeout: 120
```

Images are sent as standard `data:image/png;base64,...` data URIs. Supported formats: `png`, `jpg`, `jpeg`, `webp`, `gif`. Maximum image size: 20 MB.

### OmniParser V2 (Local)

OmniParser V2 returns structured UI element data — bounding boxes, element types, and captions — making it well-suited for navigation tasks. Run it as a Gradio application on the local network.

```yaml
vision:
  providers:
    - name: "omniparser-v2"
      enabled: true
      priority: 2
      config:
        api_url: "http://localhost:7861/api/predict"
        timeout: 60
```

---

## 4. Execution Strategies

The `ResilientExecutor` supports four dispatch strategies. Choose the one that fits your latency and reliability requirements.

### first_success (Default)

Fires all configured providers concurrently. Returns the first successful result and cancels all in-flight calls. This is the lowest-latency strategy when multiple providers are healthy.

```yaml
vision:
  strategy: "first_success"
```

**Best for:** interactive sessions where latency matters and you have multiple fast providers available.

### fallback

Tries providers sequentially in the order specified by `FallbackChain`. Moves to the next provider only when the current one fails. When `FallbackChain` is empty, providers are tried in their registration order.

```yaml
vision:
  strategy: "fallback"
  fallback_chain:
    - "glm-4v"
    - "qwen2.5-vl"
    - "ui-tars-1.5"
    - "showui-2b"
```

**Best for:** cost-sensitive setups where you want to exhaust the cheapest/free providers before using paid ones, or when providers have very different latency characteristics.

### parallel

Fires all providers concurrently and waits for every one to finish. Returns the result with the shortest duration among all successful responses. This gives the best average latency when providers have variable response times, at the cost of always invoking every provider.

```yaml
vision:
  strategy: "parallel"
```

**Best for:** situations where provider latency is unpredictable and you want the fastest result regardless of cost.

### weighted

Tries providers in the order they appear in the providers list (highest `priority` integer first). Returns the first successful result without firing the remaining providers. Unlike `first_success`, this is purely sequential.

```yaml
vision:
  strategy: "weighted"
```

**Best for:** deterministic provider preference with a clear fallback order, when you want predictable invocation patterns.

---

## 5. 5-Layer Learning System

The learning system reduces provider invocations over time. Each layer is independent and can be enabled or disabled via environment variables.

### L1 — Exact Cache (< 1 ms)

The exact cache computes a combined SHA-256 hash of the raw pixel data and a 16-character hash of the prompt. Identical (screenshot, prompt) pairs are served directly from memory without any provider call.

- Storage: in-memory `map[string]*CachedResponse`, bounded by `HELIX_VISION_MAX_MEMORIES`
- Eviction: random when the map reaches capacity
- Lookup cost: O(1), sub-millisecond
- Enable: `HELIX_VISION_EXACT_CACHE=true`

### L2 — Differential Cache (10-100x speedup)

The differential cache divides an image into a 24x24 pixel patch grid and computes per-patch SHA-256 hashes. When a new screenshot arrives, it compares the patch-hash vector against the most recently stored frame. If the fraction of changed patches is below `HELIX_VISION_CHANGE_THRESHOLD` (default 5%), the previous response is reused.

- Storage: `go-cache` with a 5-minute TTL per frame state
- Threshold: `HELIX_VISION_CHANGE_THRESHOLD` (0.05 = 5% of patches must change for a miss)
- Particularly effective for screen sequences where only a small UI region changes between frames
- Enable: `HELIX_VISION_DIFFERENTIAL=true`

### L3 — Vector Memory (5-50x speedup)

The vector memory store uses `chromem-go` for embedding-based semantic retrieval. After each successful provider call, the interaction (prompt, response, UI element type) is embedded and stored. On subsequent calls, similar interactions are retrieved and, if the similarity score is high enough, the stored response is reused without a provider call.

- Backend: `chromem-go` (in-process, no external service required)
- Persistence: set `HELIX_VISION_PERSIST_PATH` to a writable directory for on-disk storage; leave empty for in-memory only
- Capacity: `HELIX_VISION_MAX_MEMORIES` entries (default 100,000)
- Enable: `HELIX_VISION_VECTOR_MEMORY=true`

### L4 — Few-Shot Learning (+5-15% accuracy)

The few-shot builder retrieves up to 5 semantically similar successful interactions from L3 and prepends them to the current prompt before sending it to the provider. Only examples with cosine similarity above 0.7 are included.

Example augmented prompt structure:
```
Here are examples of successful UI element identification:

Example 1:
Query: Find the login button
Correct Response: The login button is in the bottom-right corner at (720, 480)

Now, using these examples as reference, please respond to:
Find the search field
```

- Enable: `HELIX_VISION_FEW_SHOT=true`
- Requires L3 vector memory to be enabled
- Accuracy improves gradually as more successful interactions accumulate

### L5 — Provider Optimizer (10-20% failure reduction)

The provider optimizer tracks per-provider metrics using exponential moving averages (EMA with alpha=0.1):

- Success rate per provider
- Average latency per provider
- Per-UI-element-type accuracy (button, text, image, general)

When the optimizer is queried for the best provider for a given UI element type, it scores providers as `successRate * uiTypeAccuracy` and optionally penalizes high-latency providers when `prioritizeSpeed` is set. Providers whose `LastUsed` timestamp is older than 10 minutes are excluded as stale.

```go
bestProvider := optimizer.GetBestProvider("button", true /* prioritizeSpeed */)
```

---

## 6. Monitoring

### Prometheus Metrics

All metrics are registered under the `cheaper_vision` namespace. The server exposes them at `GET /metrics`.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `cheaper_vision_requests_total` | Counter | `provider` | Total vision analysis requests dispatched to each provider |
| `cheaper_vision_cache_hits_total` | Counter | `layer` | Cache hits by layer (`exact`, `differential`, `vector`) |
| `cheaper_vision_request_duration_seconds` | Histogram | `provider` | Wall-clock duration of provider calls. Buckets: 0.1, 0.25, 0.5, 1, 2.5, 5, 10 s |
| `cheaper_vision_circuit_breaker_state` | Gauge | `provider` | Circuit breaker state per provider: 0 = closed, 1 = half-open, 2 = open |

### Checking Metrics

```bash
curl http://localhost:8080/metrics | grep cheaper_vision
```

### Key Indicators

**Cache effectiveness:**

```bash
curl -s http://localhost:8080/metrics | grep cheaper_vision_cache_hits_total
# cheaper_vision_cache_hits_total{layer="exact"} 312
# cheaper_vision_cache_hits_total{layer="differential"} 89
# cheaper_vision_cache_hits_total{layer="vector"} 44
```

**Provider health:**

```bash
curl -s http://localhost:8080/metrics | grep cheaper_vision_circuit_breaker_state
# cheaper_vision_circuit_breaker_state{provider="glm-4v"} 0
# cheaper_vision_circuit_breaker_state{provider="ui-tars-1.5"} 2
```

A value of `2` means the circuit breaker is open — that provider is currently bypassed. The breaker transitions to half-open after the configured `CBTimeout` period.

**Latency percentiles:**

The `cheaper_vision_request_duration_seconds` histogram provides `_bucket`, `_count`, and `_sum` labels that Prometheus uses to compute percentile queries:

```promql
histogram_quantile(0.95,
  rate(cheaper_vision_request_duration_seconds_bucket[5m]))
```

---

## 7. Prompt Engineering

Precise, action-oriented prompts produce better results across all providers and reduce the need for retries.

### Good Prompts

```
"Find the login button and return its screen coordinates"
"Extract all text visible in the navigation header"
"Identify all form fields and return their labels and types"
"Is the 'Sign In' button visible on this screen? Answer yes or no."
"What is the title text of the currently focused list item?"
```

### Bad Prompts

```
"What's in this image?"
"Click the button"
"Process this screenshot"
"Analyze"
```

Bad prompts are vague, lack a concrete task definition, or ask the provider to take an action rather than describe what it sees.

### Guidelines

- **State the task explicitly.** Providers do not infer intent — tell them exactly what information you need.
- **Be specific about the expected output format.** "Return coordinates as (x, y)" or "Answer yes or no" gives the model less room to produce unparseable output.
- **Include context about the screen type when relevant.** "This is an Android TV home screen. Find the focused tile." helps models that are trained on diverse UI data.
- **Avoid compound prompts.** Ask for one thing per call. "Find the login button AND extract all text" is two tasks; split them into two calls.
- **Use element-type vocabulary.** Words like "button", "text field", "list item", "tab", and "menu item" map directly to UI-TARS and OmniParser's internal element taxonomy, which improves their confidence scores.
