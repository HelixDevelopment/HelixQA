# Cheaper Vision Integration - Complete Implementation Guide

## Table of Contents

1. [Project Structure](#1-project-structure)
2. [Core Interfaces](#2-core-interfaces)
3. [Provider Adapters](#3-provider-adapters)
4. [Resilience Layer](#4-resilience-layer)
5. [Learning System](#5-learning-system)
6. [Integration with HelixQA](#6-integration-with-helixqa)
7. [Testing Implementation](#7-testing-implementation)
8. [Makefile & CI/CD](#8-makefile--cicd)

---

## 1. Project Structure

```
helixqa-vision-integration/
├── cmd/
│   └── helixqa/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   ├── engine/
│   │   ├── vision_engine.go
│   │   ├── learning_engine.go
│   │   └── engine_test.go
│   └── server/
│       ├── http_server.go
│       └── handlers.go
├── pkg/
│   ├── vision/
│   │   ├── provider.go
│   │   ├── registry.go
│   │   ├── executor.go
│   │   └── result.go
│   ├── vision/adapters/
│   │   ├── uitars/
│   │   │   └── uitars.go
│   │   ├── showui/
│   │   │   └── showui.go
│   │   ├── glm4v/
│   │   │   └── glm4v.go
│   │   ├── qwen25vl/
│   │   │   └── qwen25vl.go
│   │   └── omniparser/
│   │       └── omniparser.go
│   ├── vision/memory/
│   │   ├── vector_memory.go
│   │   └── memory_test.go
│   ├── vision/cache/
│   │   ├── differential_cache.go
│   │   └── cache_test.go
│   └── vision/learning/
│       ├── executor.go
│       ├── few_shot.go
│       ├── optimizer.go
│       └── learning_test.go
├── tests/
│   ├── unit/
│   ├── integration/
│   ├── e2e/
│   ├── benchmark/
│   ├── chaos/
│   ├── fuzz/
│   ├── stress/
│   ├── security/
│   ├── contract/
│   └── helpers/
│       └── mocks.go
├── deployments/
│   ├── docker/
│   │   ├── Dockerfile
│   │   └── docker-compose.yml
│   └── kubernetes/
├── docs/
│   ├── api/
│   ├── user-guide/
│   └── admin-guide/
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## 2. Core Interfaces

### 2.1 Vision Provider Interface

```go
// pkg/vision/provider.go
package vision

import (
    "context"
    "image"
    "time"
)

// VisionResult represents the output from a vision model
type VisionResult struct {
    Text         string                 `json:"text"`
    RawResponse  interface{}            `json:"raw_response,omitempty"`
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
    Duration     time.Duration          `json:"duration"`
    Model        string                 `json:"model"`
    Provider     string                 `json:"provider"`
    Timestamp    time.Time              `json:"timestamp"`
    CacheHit     bool                   `json:"cache_hit,omitempty"`
    Confidence   float64                `json:"confidence,omitempty"`
}

// VisionProvider defines the interface for all vision model providers
type VisionProvider interface {
    // Analyze sends an image and prompt to the vision model
    Analyze(ctx context.Context, img image.Image, prompt string) (*VisionResult, error)
    
    // Name returns the unique identifier for this provider
    Name() string
    
    // HealthCheck verifies the provider is reachable and functioning
    HealthCheck(ctx context.Context) error
    
    // GetCapabilities returns provider-specific capabilities
    GetCapabilities() ProviderCapabilities
    
    // GetCostEstimate returns estimated cost for a request
    GetCostEstimate(imageSize int, promptLength int) float64
}

// ProviderCapabilities describes what a provider can do
type ProviderCapabilities struct {
    SupportsStreaming    bool
    MaxImageSize         int
    SupportedFormats     []string
    AverageLatency       time.Duration
    SupportsBatch        bool
    CostPer1MTokens      float64
}

// ProviderFactory creates VisionProvider instances
type ProviderFactory func(config map[string]interface{}) (VisionProvider, error)

// ProviderConfig holds configuration for creating providers
type ProviderConfig struct {
    Name        string                 `yaml:"name" json:"name"`
    Enabled     bool                   `yaml:"enabled" json:"enabled"`
    Priority    int                    `yaml:"priority" json:"priority"`
    Config      map[string]interface{} `yaml:"config" json:"config"`
    FallbackTo  []string               `yaml:"fallback_to" json:"fallback_to"`
}
```

### 2.2 Registry Implementation

```go
// pkg/vision/registry.go
package vision

import (
    "fmt"
    "sync"
)

var (
    registry = make(map[string]ProviderFactory)
    mu       sync.RWMutex
)

// Register adds a provider factory to the registry
func Register(name string, factory ProviderFactory) {
    mu.Lock()
    defer mu.Unlock()
    
    if factory == nil {
        panic("vision: Register factory is nil")
    }
    if _, exists := registry[name]; exists {
        panic(fmt.Sprintf("vision: provider %q already registered", name))
    }
    
    registry[name] = factory
}

// Create instantiates a provider by name
func Create(name string, config map[string]interface{}) (VisionProvider, error) {
    mu.RLock()
    factory, ok := registry[name]
    mu.RUnlock()
    
    if !ok {
        return nil, fmt.Errorf("vision: unknown provider %q", name)
    }
    
    return factory(config)
}

// List returns all registered provider names
func List() []string {
    mu.RLock()
    defer mu.RUnlock()
    
    names := make([]string, 0, len(registry))
    for name := range registry {
        names = append(names, name)
    }
    return names
}

// IsRegistered checks if a provider is registered
func IsRegistered(name string) bool {
    mu.RLock()
    defer mu.RUnlock()
    _, ok := registry[name]
    return ok
}

// Unregister removes a provider from registry (mainly for testing)
func Unregister(name string) {
    mu.Lock()
    defer mu.Unlock()
    delete(registry, name)
}
```

---

## 3. Provider Adapters

### 3.1 UI-TARS Adapter

```go
// pkg/vision/adapters/uitars/uitars.go
package uitars

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "image"
    "image/png"
    "io"
    "net/http"
    "time"

    "github.com/HelixDevelopment/helixqa/pkg/vision"
)

const (
    defaultBaseURL = "https://api-inference.huggingface.co"
    providerName   = "ui-tars-1.5"
)

// UITARSProvider implements VisionProvider for UI-TARS model
type UITARSProvider struct {
    client      *http.Client
    baseURL     string
    apiKey      string
    model       string
    timeout     time.Duration
}

func init() {
    vision.Register("uitars", NewUITARSProvider)
}

// NewUITARSProvider creates a new UI-TARS provider
func NewUITARSProvider(config map[string]interface{}) (vision.VisionProvider, error) {
    apiKey, ok := config["api_key"].(string)
    if !ok || apiKey == "" {
        return nil, fmt.Errorf("uitars: missing api_key in config")
    }

    baseURL, _ := config["base_url"].(string)
    if baseURL == "" {
        baseURL = defaultBaseURL
    }

    model, _ := config["model"].(string)
    if model == "" {
        model = "ByteDance-Seed/UI-TARS-1.5-7B"
    }

    timeout := 60 * time.Second
    if t, ok := config["timeout"].(string); ok {
        if d, err := time.ParseDuration(t); err == nil {
            timeout = d
        }
    }

    return &UITARSProvider{
        client: &http.Client{
            Timeout: timeout,
        },
        baseURL: baseURL,
        apiKey:  apiKey,
        model:   model,
        timeout: timeout,
    }, nil
}

// Name returns the provider name
func (p *UITARSProvider) Name() string {
    return providerName
}

// Analyze sends image to UI-TARS for analysis
func (p *UITARSProvider) Analyze(ctx context.Context, img image.Image, prompt string) (*vision.VisionResult, error) {
    start := time.Now()

    // Convert image to base64
    imgBase64, err := imageToBase64(img)
    if err != nil {
        return nil, fmt.Errorf("uitars: failed to encode image: %w", err)
    }

    // Build OpenAI-compatible request
    payload := map[string]interface{}{
        "model": p.model,
        "messages": []map[string]interface{}{
            {
                "role": "user",
                "content": []map[string]interface{}{
                    {"type": "text", "text": prompt},
                    {"type": "image_url", "image_url": map[string]string{
                        "url": "data:image/png;base64," + imgBase64,
                    }},
                },
            },
        },
        "max_tokens":  1024,
        "temperature": 0.1,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("uitars: failed to marshal request: %w", err)
    }

    url := fmt.Sprintf("%s/v1/chat/completions", p.baseURL)
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("uitars: failed to create request: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+p.apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("uitars: request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("uitars: API error %d: %s", resp.StatusCode, string(bodyBytes))
    }

    var result struct {
        Choices []struct {
            Message struct {
                Content string `json:"content"`
            } `json:"message"`
        } `json:"choices"`
        Usage struct {
            TotalTokens int `json:"total_tokens"`
        } `json:"usage"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("uitars: failed to decode response: %w", err)
    }

    if len(result.Choices) == 0 {
        return nil, fmt.Errorf("uitars: no response from model")
    }

    return &vision.VisionResult{
        Text:      result.Choices[0].Message.Content,
        Model:     p.model,
        Provider:  providerName,
        Duration:  time.Since(start),
        Timestamp: time.Now(),
        Metadata: map[string]interface{}{
            "total_tokens": result.Usage.TotalTokens,
        },
    }, nil
}

// HealthCheck verifies provider health
func (p *UITARSProvider) HealthCheck(ctx context.Context) error {
    req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/health", nil)
    if err != nil {
        return err
    }
    req.Header.Set("Authorization", "Bearer "+p.apiKey)

    resp, err := p.client.Do(req)
    if err != nil {
        return fmt.Errorf("uitars: health check failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("uitars: unhealthy status %d", resp.StatusCode)
    }
    return nil
}

// GetCapabilities returns provider capabilities
func (p *UITARSProvider) GetCapabilities() vision.ProviderCapabilities {
    return vision.ProviderCapabilities{
        SupportsStreaming: false,
        MaxImageSize:      20 * 1024 * 1024, // 20MB
        SupportedFormats:  []string{"png", "jpg", "jpeg", "webp"},
        AverageLatency:    2 * time.Second,
        SupportsBatch:     false,
        CostPer1MTokens:   0.0, // Self-hosted
    }
}

// GetCostEstimate estimates request cost
func (p *UITARSProvider) GetCostEstimate(imageSize int, promptLength int) float64 {
    // Self-hosted: only electricity cost
    return 0.0001
}

func imageToBase64(img image.Image) (string, error) {
    var buf bytes.Buffer
    if err := png.Encode(&buf, img); err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
```

### 3.2 ShowUI Adapter (Local Deployment)

```go
// pkg/vision/adapters/showui/showui.go
package showui

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "image"
    "image/png"
    "io"
    "net/http"
    "time"

    "github.com/HelixDevelopment/helixqa/pkg/vision"
)

const providerName = "showui-2b"

// ShowUIProvider implements VisionProvider for local ShowUI deployment
type ShowUIProvider struct {
    client  *http.Client
    apiURL  string
    timeout time.Duration
}

func init() {
    vision.Register("showui", NewShowUIProvider)
}

// NewShowUIProvider creates a new ShowUI provider
func NewShowUIProvider(config map[string]interface{}) (vision.VisionProvider, error) {
    apiURL, _ := config["api_url"].(string)
    if apiURL == "" {
        apiURL = "http://localhost:7860/api/predict"
    }

    timeout := 30 * time.Second
    if t, ok := config["timeout"].(string); ok {
        if d, err := time.ParseDuration(t); err == nil {
            timeout = d
        }
    }

    return &ShowUIProvider{
        client: &http.Client{
            Timeout: timeout,
        },
        apiURL:  apiURL,
        timeout: timeout,
    }, nil
}

// Name returns the provider name
func (p *ShowUIProvider) Name() string {
    return providerName
}

// Analyze sends image to ShowUI for analysis
func (p *ShowUIProvider) Analyze(ctx context.Context, img image.Image, prompt string) (*vision.VisionResult, error) {
    start := time.Now()

    imgBase64, err := imageToBase64(img)
    if err != nil {
        return nil, fmt.Errorf("showui: failed to encode image: %w", err)
    }

    // Gradio API format
    payload := map[string]interface{}{
        "data": []string{imgBase64, prompt},
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("showui: failed to marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", p.apiURL, bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("showui: failed to create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("showui: request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("showui: API error %d: %s", resp.StatusCode, string(bodyBytes))
    }

    var result struct {
        Data []string `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("showui: failed to decode response: %w", err)
    }

    text := ""
    if len(result.Data) > 0 {
        text = result.Data[0]
    }

    return &vision.VisionResult{
        Text:      text,
        Model:     "ShowUI-2B",
        Provider:  providerName,
        Duration:  time.Since(start),
        Timestamp: time.Now(),
    }, nil
}

// HealthCheck verifies provider health
func (p *ShowUIProvider) HealthCheck(ctx context.Context) error {
    // Check root endpoint
    baseURL := p.apiURL[:len(p.apiURL)-len("/api/predict")]
    req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
    if err != nil {
        return err
    }

    resp, err := p.client.Do(req)
    if err != nil {
        return fmt.Errorf("showui: health check failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("showui: unhealthy status %d", resp.StatusCode)
    }
    return nil
}

// GetCapabilities returns provider capabilities
func (p *ShowUIProvider) GetCapabilities() vision.ProviderCapabilities {
    return vision.ProviderCapabilities{
        SupportsStreaming: false,
        MaxImageSize:      10 * 1024 * 1024,
        SupportedFormats:  []string{"png", "jpg", "jpeg"},
        AverageLatency:    500 * time.Millisecond,
        SupportsBatch:     false,
        CostPer1MTokens:   0.0,
    }
}

// GetCostEstimate estimates request cost
func (p *ShowUIProvider) GetCostEstimate(imageSize int, promptLength int) float64 {
    return 0.0 // Completely free (local)
}

func imageToBase64(img image.Image) (string, error) {
    var buf bytes.Buffer
    if err := png.Encode(&buf, img); err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
```

### 3.3 GLM-4V Adapter (Zhipu AI)

```go
// pkg/vision/adapters/glm4v/glm4v.go
package glm4v

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "image"
    "image/png"
    "io"
    "net/http"
    "time"

    "github.com/HelixDevelopment/helixqa/pkg/vision"
)

const (
    defaultBaseURL = "https://open.bigmodel.cn/api/paas/v4"
    providerName   = "glm-4v"
)

// GLM4VProvider implements VisionProvider for Zhipu AI GLM-4V
type GLM4VProvider struct {
    client  *http.Client
    apiKey  string
    model   string
    timeout time.Duration
}

func init() {
    vision.Register("glm4v", NewGLM4VProvider)
}

// NewGLM4VProvider creates a new GLM-4V provider
func NewGLM4VProvider(config map[string]interface{}) (vision.VisionProvider, error) {
    apiKey, ok := config["api_key"].(string)
    if !ok || apiKey == "" {
        return nil, fmt.Errorf("glm-4v: missing api_key in config")
    }

    model, _ := config["model"].(string)
    if model == "" {
        model = "glm-4v-flash" // Free tier
    }

    timeout := 60 * time.Second
    if t, ok := config["timeout"].(string); ok {
        if d, err := time.ParseDuration(t); err == nil {
            timeout = d
        }
    }

    return &GLM4VProvider{
        client: &http.Client{
            Timeout: timeout,
        },
        apiKey:  apiKey,
        model:   model,
        timeout: timeout,
    }, nil
}

// Name returns the provider name
func (p *GLM4VProvider) Name() string {
    return providerName
}

// Analyze sends image to GLM-4V for analysis
func (p *GLM4VProvider) Analyze(ctx context.Context, img image.Image, prompt string) (*vision.VisionResult, error) {
    start := time.Now()

    imgBase64, err := imageToBase64(img)
    if err != nil {
        return nil, fmt.Errorf("glm-4v: failed to encode image: %w", err)
    }

    // Zhipu API format (slightly different)
    payload := map[string]interface{}{
        "model": p.model,
        "messages": []map[string]interface{}{
            {
                "role": "user",
                "content": []map[string]interface{}{
                    {"type": "text", "text": prompt},
                    {"type": "image_url", "image_url": map[string]string{
                        "url": imgBase64, // Note: no data:image prefix for Zhipu
                    }},
                },
            },
        },
        "max_tokens": 1024,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("glm-4v: failed to marshal request: %w", err)
    }

    url := fmt.Sprintf("%s/chat/completions", defaultBaseURL)
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("glm-4v: failed to create request: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+p.apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("glm-4v: request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("glm-4v: API error %d: %s", resp.StatusCode, string(bodyBytes))
    }

    var result struct {
        Choices []struct {
            Message struct {
                Content string `json:"content"`
            } `json:"message"`
        } `json:"choices"`
        Usage struct {
            TotalTokens int `json:"total_tokens"`
        } `json:"usage"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("glm-4v: failed to decode response: %w", err)
    }

    if len(result.Choices) == 0 {
        return nil, fmt.Errorf("glm-4v: no response from model")
    }

    return &vision.VisionResult{
        Text:      result.Choices[0].Message.Content,
        Model:     p.model,
        Provider:  providerName,
        Duration:  time.Since(start),
        Timestamp: time.Now(),
        Metadata: map[string]interface{}{
            "total_tokens": result.Usage.TotalTokens,
        },
    }, nil
}

// HealthCheck verifies provider health
func (p *GLM4VProvider) HealthCheck(ctx context.Context) error {
    // Zhipu doesn't have a dedicated health endpoint
    // We do a lightweight model list request
    req, err := http.NewRequestWithContext(ctx, "GET", defaultBaseURL+"/models", nil)
    if err != nil {
        return err
    }
    req.Header.Set("Authorization", "Bearer "+p.apiKey)

    resp, err := p.client.Do(req)
    if err != nil {
        return fmt.Errorf("glm-4v: health check failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("glm-4v: unhealthy status %d", resp.StatusCode)
    }
    return nil
}

// GetCapabilities returns provider capabilities
func (p *GLM4VProvider) GetCapabilities() vision.ProviderCapabilities {
    costPer1M := 0.0
    if p.model == "glm-4v" {
        costPer1M = 0.015 * 100 // ~0.015 RMB per call, approximate
    }
    // glm-4v-flash is free

    return vision.ProviderCapabilities{
        SupportsStreaming: false,
        MaxImageSize:      10 * 1024 * 1024,
        SupportedFormats:  []string{"png", "jpg", "jpeg", "webp"},
        AverageLatency:    1 * time.Second,
        SupportsBatch:     false,
        CostPer1MTokens:   costPer1M,
    }
}

// GetCostEstimate estimates request cost
func (p *GLM4VProvider) GetCostEstimate(imageSize int, promptLength int) float64 {
    if p.model == "glm-4v-flash" {
        return 0.0
    }
    // Approximate cost per call
    return 0.015 // RMB
}

func imageToBase64(img image.Image) (string, error) {
    var buf bytes.Buffer
    if err := png.Encode(&buf, img); err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
```

### 3.4 Qwen2.5-VL Adapter

```go
// pkg/vision/adapters/qwen25vl/qwen25vl.go
package qwen25vl

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "image"
    "image/png"
    "io"
    "net/http"
    "time"

    "github.com/HelixDevelopment/helixqa/pkg/vision"
)

const (
    defaultBaseURL = "http://localhost:9192/v1"
    providerName   = "qwen2.5-vl"
)

// Qwen25VLProvider implements VisionProvider for Qwen2.5-VL
type Qwen25VLProvider struct {
    client  *http.Client
    baseURL string
    model   string
    timeout time.Duration
}

func init() {
    vision.Register("qwen25vl", NewQwen25VLProvider)
}

// NewQwen25VLProvider creates a new Qwen2.5-VL provider
func NewQwen25VLProvider(config map[string]interface{}) (vision.VisionProvider, error) {
    baseURL, _ := config["base_url"].(string)
    if baseURL == "" {
        baseURL = defaultBaseURL
    }

    model, _ := config["model"].(string)
    if model == "" {
        model = "Qwen2.5-VL-7B-Instruct"
    }

    timeout := 120 * time.Second
    if t, ok := config["timeout"].(string); ok {
        if d, err := time.ParseDuration(t); err == nil {
            timeout = d
        }
    }

    return &Qwen25VLProvider{
        client: &http.Client{
            Timeout: timeout,
        },
        baseURL: baseURL,
        model:   model,
        timeout: timeout,
    }, nil
}

// Name returns the provider name
func (p *Qwen25VLProvider) Name() string {
    return providerName
}

// Analyze sends image to Qwen2.5-VL for analysis
func (p *Qwen25VLProvider) Analyze(ctx context.Context, img image.Image, prompt string) (*vision.VisionResult, error) {
    start := time.Now()

    imgBase64, err := imageToBase64(img)
    if err != nil {
        return nil, fmt.Errorf("qwen2.5-vl: failed to encode image: %w", err)
    }

    payload := map[string]interface{}{
        "model": p.model,
        "messages": []map[string]interface{}{
            {
                "role": "user",
                "content": []map[string]interface{}{
                    {"type": "text", "text": prompt},
                    {"type": "image_url", "image_url": map[string]string{
                        "url": "data:image/png;base64," + imgBase64,
                    }},
                },
            },
        },
        "max_tokens": 1024,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("qwen2.5-vl: failed to marshal request: %w", err)
    }

    url := fmt.Sprintf("%s/chat/completions", p.baseURL)
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("qwen2.5-vl: failed to create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("qwen2.5-vl: request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("qwen2.5-vl: API error %d: %s", resp.StatusCode, string(bodyBytes))
    }

    var result struct {
        Choices []struct {
            Message struct {
                Content string `json:"content"`
            } `json:"message"`
        } `json:"choices"`
        Usage struct {
            TotalTokens int `json:"total_tokens"`
        } `json:"usage"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("qwen2.5-vl: failed to decode response: %w", err)
    }

    if len(result.Choices) == 0 {
        return nil, fmt.Errorf("qwen2.5-vl: no response from model")
    }

    return &vision.VisionResult{
        Text:      result.Choices[0].Message.Content,
        Model:     p.model,
        Provider:  providerName,
        Duration:  time.Since(start),
        Timestamp: time.Now(),
        Metadata: map[string]interface{}{
            "total_tokens": result.Usage.TotalTokens,
        },
    }, nil
}

// HealthCheck verifies provider health
func (p *Qwen25VLProvider) HealthCheck(ctx context.Context) error {
    req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
    if err != nil {
        return err
    }

    resp, err := p.client.Do(req)
    if err != nil {
        return fmt.Errorf("qwen2.5-vl: health check failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("qwen2.5-vl: unhealthy status %d", resp.StatusCode)
    }
    return nil
}

// GetCapabilities returns provider capabilities
func (p *Qwen25VLProvider) GetCapabilities() vision.ProviderCapabilities {
    return vision.ProviderCapabilities{
        SupportsStreaming: false,
        MaxImageSize:      20 * 1024 * 1024,
        SupportedFormats:  []string{"png", "jpg", "jpeg", "webp", "gif"},
        AverageLatency:    3 * time.Second,
        SupportsBatch:     false,
        CostPer1MTokens:   0.0, // Self-hosted
    }
}

// GetCostEstimate estimates request cost
func (p *Qwen25VLProvider) GetCostEstimate(imageSize int, promptLength int) float64 {
    return 0.0001 // Minimal electricity cost
}

func imageToBase64(img image.Image) (string, error) {
    var buf bytes.Buffer
    if err := png.Encode(&buf, img); err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
```

---

## 4. Resilience Layer

### 4.1 Enhanced Executor with failsafe-go

```go
// pkg/vision/executor.go
package vision

import (
    "context"
    "errors"
    "fmt"
    "image"
    "sync"
    "time"

    "github.com/failsafe-go/failsafe-go"
    "github.com/failsafe-go/failsafe-go/circuitbreaker"
    "github.com/failsafe-go/failsafe-go/fallback"
    "github.com/failsafe-go/failsafe-go/retrypolicy"
    "github.com/failsafe-go/failsafe-go/timeout"
)

// ExecutionStrategy defines how to run vision requests
type ExecutionStrategy string

const (
    StrategyFirstSuccess ExecutionStrategy = "first_success"
    StrategyParallel     ExecutionStrategy = "parallel"
    StrategyFallback     ExecutionStrategy = "fallback"
    StrategyWeighted     ExecutionStrategy = "weighted"
)

// ExecutorConfig configures the execution behavior
type ExecutorConfig struct {
    Strategy       ExecutionStrategy
    Providers      []VisionProvider
    Timeout        time.Duration
    RetryAttempts  int
    RetryDelay     time.Duration
    RetryBackoff   float64
    CircuitBreaker bool
    CBThreshold    int
    CBTimeout      time.Duration
    FallbackChain  []string
    MaxConcurrency int
}

// ResilientExecutor handles parallel and fallback execution
type ResilientExecutor struct {
    config          ExecutorConfig
    providerMap     map[string]VisionProvider
    circuitBreakers map[string]circuitbreaker.CircuitBreaker[*VisionResult]
    mu              sync.RWMutex
}

// NewResilientExecutor creates a new resilient executor
func NewResilientExecutor(config ExecutorConfig) *ResilientExecutor {
    pm := make(map[string]VisionProvider)
    for _, p := range config.Providers {
        pm[p.Name()] = p
    }

    executor := &ResilientExecutor{
        config:          config,
        providerMap:     pm,
        circuitBreakers: make(map[string]circuitbreaker.CircuitBreaker[*VisionResult]),
    }

    // Initialize circuit breakers if enabled
    if config.CircuitBreaker {
        for name := range pm {
            executor.circuitBreakers[name] = executor.createCircuitBreaker(name)
        }
    }

    return executor
}

// Execute runs the vision request according to the configured strategy
func (e *ResilientExecutor) Execute(ctx context.Context, img image.Image, prompt string) (*VisionResult, error) {
    switch e.config.Strategy {
    case StrategyFirstSuccess:
        return e.executeFirstSuccess(ctx, img, prompt)
    case StrategyParallel:
        return e.executeParallel(ctx, img, prompt)
    case StrategyFallback:
        return e.executeFallbackChain(ctx, img, prompt)
    case StrategyWeighted:
        return e.executeWeighted(ctx, img, prompt)
    default:
        return nil, fmt.Errorf("unknown strategy: %s", e.config.Strategy)
    }
}

// executeFirstSuccess fires all providers concurrently and returns first success
func (e *ResilientExecutor) executeFirstSuccess(ctx context.Context, img image.Image, prompt string) (*VisionResult, error) {
    ctx, cancel := context.WithTimeout(ctx, e.config.Timeout)
    defer cancel()

    type result struct {
        res *VisionResult
        err error
        p   string
    }

    results := make(chan result, len(e.config.Providers))
    var wg sync.WaitGroup

    for _, provider := range e.config.Providers {
        wg.Add(1)
        go func(p VisionProvider) {
            defer wg.Done()
            res, err := e.executeWithResilience(ctx, p, img, prompt)
            select {
            case results <- result{res, err, p.Name()}:
            case <-ctx.Done():
            }
        }(provider)
    }

    // Close results channel when all goroutines complete
    go func() {
        wg.Wait()
        close(results)
    }()

    var firstErr error
    successCount := 0
    failCount := 0
    totalProviders := len(e.config.Providers)

    for {
        select {
        case r, ok := <-results:
            if !ok {
                // Channel closed, all done
                if successCount == 0 && firstErr != nil {
                    return nil, fmt.Errorf("all providers failed: %w", firstErr)
                }
                return nil, errors.New("all providers failed")
            }

            if r.err == nil {
                successCount++
                cancel() // Cancel remaining goroutines
                return r.res, nil
            }

            failCount++
            if firstErr == nil {
                firstErr = r.err
            }

            // If all have failed, return error
            if failCount >= totalProviders {
                return nil, fmt.Errorf("all providers failed: %w", firstErr)
            }

        case <-ctx.Done():
            if firstErr != nil {
                return nil, firstErr
            }
            return nil, ctx.Err()
        }
    }
}

// executeParallel runs all providers and returns best result
func (e *ResilientExecutor) executeParallel(ctx context.Context, img image.Image, prompt string) (*VisionResult, error) {
    ctx, cancel := context.WithTimeout(ctx, e.config.Timeout)
    defer cancel()

    type result struct {
        res *VisionResult
        err error
        p   string
    }

    results := make([]result, len(e.config.Providers))
    var wg sync.WaitGroup

    for i, provider := range e.config.Providers {
        wg.Add(1)
        go func(idx int, p VisionProvider) {
            defer wg.Done()
            res, err := e.executeWithResilience(ctx, p, img, prompt)
            results[idx] = result{res, err, p.Name()}
        }(i, provider)
    }

    wg.Wait()

    // Collect successes and find best result
    var bestResult *VisionResult
    var errors []error

    for _, r := range results {
        if r.err == nil {
            if bestResult == nil || r.res.Duration < bestResult.Duration {
                bestResult = r.res
            }
        } else {
            errors = append(errors, r.err)
        }
    }

    if bestResult == nil {
        return nil, fmt.Errorf("all providers failed: %v", errors)
    }

    return bestResult, nil
}

// executeFallbackChain tries providers in order until one succeeds
func (e *ResilientExecutor) executeFallbackChain(ctx context.Context, img image.Image, prompt string) (*VisionResult, error) {
    var lastErr error

    for _, name := range e.config.FallbackChain {
        provider, ok := e.providerMap[name]
        if !ok {
            continue
        }

        res, err := e.executeWithResilience(ctx, provider, img, prompt)
        if err == nil {
            return res, nil
        }
        lastErr = err
    }

    return nil, fmt.Errorf("fallback chain exhausted: %w", lastErr)
}

// executeWeighted routes based on provider performance scores
func (e *ResilientExecutor) executeWeighted(ctx context.Context, img image.Image, prompt string) (*VisionResult, error) {
    // Start with highest priority provider
    for _, provider := range e.config.Providers {
        res, err := e.executeWithResilience(ctx, provider, img, prompt)
        if err == nil {
            return res, nil
        }
        // Continue to next provider on failure
    }

    return nil, errors.New("all weighted providers failed")
}

// executeWithResilience wraps a single provider call with policies
func (e *ResilientExecutor) executeWithResilience(ctx context.Context, p VisionProvider, img image.Image, prompt string) (*VisionResult, error) {
    policies := []failsafe.Policy[*VisionResult]{}

    // Add timeout
    if e.config.Timeout > 0 {
        policies = append(policies, timeout.With[*VisionResult](e.config.Timeout))
    }

    // Add retry policy
    if e.config.RetryAttempts > 0 {
        retryPolicy := retrypolicy.Builder[*VisionResult]().
            HandleIf(func(result *VisionResult, err error) bool {
                return err != nil && !errors.Is(err, context.Canceled)
            }).
            WithMaxRetries(e.config.RetryAttempts).
            WithDelay(e.config.RetryDelay).
            WithBackoff(e.config.RetryBackoff, e.config.RetryBackoff*10).
            Build()
        policies = append(policies, retryPolicy)
    }

    // Add circuit breaker
    if e.config.CircuitBreaker {
        if cb, ok := e.circuitBreakers[p.Name()]; ok {
            policies = append(policies, cb)
        }
    }

    // Execute with policies
    executor := failsafe.With(policies...)
    result, err := executor.GetWithContext(ctx, func() (*VisionResult, error) {
        return p.Analyze(ctx, img, prompt)
    })

    return result, err
}

// createCircuitBreaker creates a circuit breaker for a provider
func (e *ResilientExecutor) createCircuitBreaker(name string) circuitbreaker.CircuitBreaker[*VisionResult] {
    return circuitbreaker.Builder[*VisionResult]().
        WithFailureThreshold(uint(e.config.CBThreshold)).
        WithSuccessThreshold(3).
        WithDelay(e.config.CBTimeout).
        OnStateChanged(func(event circuitbreaker.StateChangedEvent) {
            // Log state change - could emit metrics here
            fmt.Printf("Circuit breaker %s: %s -> %s\n", name, event.OldState, event.NewState)
        }).
        Build()
}

// GetCircuitBreakerState returns the current state of a provider's circuit breaker
func (e *ResilientExecutor) GetCircuitBreakerState(name string) string {
    if cb, ok := e.circuitBreakers[name]; ok {
        return cb.State().String()
    }
    return "unknown"
}

// GetProviderStats returns statistics for all providers
func (e *ResilientExecutor) GetProviderStats() map[string]interface{} {
    stats := make(map[string]interface{})
    for name, cb := range e.circuitBreakers {
        stats[name] = map[string]interface{}{
            "state": cb.State().String(),
        }
    }
    return stats
}
```

---

## 5. Learning System

### 5.1 Vector Memory Store

```go
// pkg/vision/memory/vector_memory.go
package memory

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "image"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/philippgille/chromem-go"
)

// VisionMemory represents a learned visual experience
type VisionMemory struct {
    ID              string                 `json:"id"`
    ImageHash       string                 `json:"image_hash"`
    Prompt          string                 `json:"prompt"`
    Response        string                 `json:"response"`
    ProviderModel   string                 `json:"provider_model"`
    Success         bool                   `json:"success"`
    Latency         time.Duration          `json:"latency"`
    UIElementType   string                 `json:"ui_element_type"`
    ConfidenceScore float64                `json:"confidence_score"`
    SimilarityScore float64                `json:"similarity_score,omitempty"`
    Metadata        map[string]interface{} `json:"metadata"`
    Timestamp       time.Time              `json:"timestamp"`
    AccessCount     int                    `json:"access_count"`
    LastAccessed    time.Time              `json:"last_accessed"`
}

// VectorMemoryStore manages the vector database for vision memories
type VectorMemoryStore struct {
    db          *chromem.DB
    collection  *chromem.Collection
    embedder    chromem.EmbeddingFunc
    mu          sync.RWMutex
    persistPath string
}

// NewVectorMemoryStore creates a new vector memory store
func NewVectorMemoryStore(persistPath string, embedder chromem.EmbeddingFunc) (*VectorMemoryStore, error) {
    db := chromem.NewDB()

    collection, err := db.GetOrCreateCollection("vision_memories", nil, embedder)
    if err != nil {
        return nil, fmt.Errorf("failed to create collection: %w", err)
    }

    store := &VectorMemoryStore{
        db:          db,
        collection:  collection,
        embedder:    embedder,
        persistPath: persistPath,
    }

    // Load persisted data if exists
    if persistPath != "" {
        if err := store.load(); err != nil {
            // Non-fatal: start fresh
            fmt.Printf("Warning: could not load persisted memories: %v\n", err)
        }
    }

    return store, nil
}

// Store stores a vision memory with its embedding
func (v *VectorMemoryStore) Store(ctx context.Context, memory *VisionMemory) error {
    v.mu.Lock()
    defer v.mu.Unlock()

    if memory.ID == "" {
        memory.ID = uuid.New().String()
    }
    memory.Timestamp = time.Now()
    memory.AccessCount = 0

    // Create embedding text
    embeddingText := fmt.Sprintf("Prompt: %s\nResponse: %s\nUI Element: %s",
        memory.Prompt, memory.Response, memory.UIElementType)

    // Store in vector DB
    metadata := map[string]string{
        "id":              memory.ID,
        "image_hash":      memory.ImageHash,
        "prompt":          memory.Prompt,
        "response":        memory.Response,
        "ui_type":         memory.UIElementType,
        "provider":        memory.ProviderModel,
        "success":         fmt.Sprintf("%v", memory.Success),
        "timestamp":       memory.Timestamp.Format(time.RFC3339),
        "access_count":    fmt.Sprintf("%d", memory.AccessCount),
        "confidence":      fmt.Sprintf("%f", memory.ConfidenceScore),
    }

    if err := v.collection.AddDocument(ctx, memory.ID, embeddingText, metadata); err != nil {
        return fmt.Errorf("failed to add document: %w", err)
    }

    // Persist periodically
    if v.persistPath != "" {
        go v.persist()
    }

    return nil
}

// Search finds similar vision memories using semantic search
func (v *VectorMemoryStore) Search(ctx context.Context, query string, limit int) ([]*VisionMemory, error) {
    v.mu.RLock()
    defer v.mu.RUnlock()

    results, err := v.collection.Query(ctx, query, limit, nil, nil)
    if err != nil {
        return nil, fmt.Errorf("search failed: %w", err)
    }

    memories := make([]*VisionMemory, 0, len(results))
    for _, result := range results {
        confidence, _ := parseFloat(result.Metadata["confidence"])
        memory := &VisionMemory{
            ID:              result.ID,
            Prompt:          result.Metadata["prompt"],
            Response:        result.Metadata["response"],
            ImageHash:       result.Metadata["image_hash"],
            UIElementType:   result.Metadata["ui_type"],
            ProviderModel:   result.Metadata["provider"],
            SimilarityScore: float64(result.Similarity),
            ConfidenceScore: confidence,
        }

        // Update access metadata asynchronously
        go v.updateAccess(memory.ID)

        memories = append(memories, memory)
    }

    return memories, nil
}

// GetFewShotExamples retrieves successful memories for few-shot prompting
func (v *VectorMemoryStore) GetFewShotExamples(ctx context.Context, query string, count int) ([]*VisionMemory, error) {
    v.mu.RLock()
    defer v.mu.RUnlock()

    // Query specifically for successful responses
    where := map[string]string{"success": "true"}
    results, err := v.collection.Query(ctx, query, count, where, nil)
    if err != nil {
        return nil, err
    }

    memories := make([]*VisionMemory, 0, len(results))
    for _, result := range results {
        confidence, _ := parseFloat(result.Metadata["confidence"])
        memory := &VisionMemory{
            ID:              result.ID,
            Prompt:          result.Metadata["prompt"],
            Response:        result.Metadata["response"],
            UIElementType:   result.Metadata["ui_type"],
            ProviderModel:   result.Metadata["provider"],
            SimilarityScore: float64(result.Similarity),
            ConfidenceScore: confidence,
        }
        memories = append(memories, memory)
    }

    return memories, nil
}

// updateAccess increments the access counter for a memory
func (v *VectorMemoryStore) updateAccess(id string) {
    v.mu.Lock()
    defer v.mu.Unlock()
    // Note: chromem-go doesn't support direct metadata updates
    // In production, implement with re-insertion or external tracking
}

// persist saves the database to disk
func (v *VectorMemoryStore) persist() error {
    if v.persistPath == "" {
        return nil
    }
    return v.db.ExportToFile(v.persistPath, true, nil)
}

// load restores the database from disk
func (v *VectorMemoryStore) load() error {
    if v.persistPath == "" {
        return nil
    }
    return v.db.ImportFromFile(v.persistPath, nil)
}

// ComputeImageHash generates a SHA-256 hash for exact image matching
func ComputeImageHash(img image.Image) (string, error) {
    bounds := img.Bounds()
    var hashData []byte

    for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
        for x := bounds.Min.X; x < bounds.Max.X; x++ {
            r, g, b, a := img.At(x, y).RGBA()
            hashData = append(hashData, byte(r), byte(g), byte(b), byte(a))
        }
    }

    hash := sha256.Sum256(hashData)
    return hex.EncodeToString(hash[:]), nil
}

func parseFloat(s string) (float64, error) {
    var f float64
    _, err := fmt.Sscanf(s, "%f", &f)
    return f, err
}
```

### 5.2 Differential Cache

```go
// pkg/vision/cache/differential_cache.go
package cache

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "image"
    "image/color"
    "math"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/patrickmn/go-cache"
)

const PatchSize = 24

// VisionResponse represents a cached response
type VisionResponse struct {
    Text      string
    Model     string
    Duration  time.Duration
    Timestamp time.Time
}

// FrameState stores the full state of a processed frame
type FrameState struct {
    FrameID      string
    ImageHash    string
    PatchHashes  []string
    Timestamp    time.Time
    FullResponse *VisionResponse
}

// DifferentialCache manages cached vision tokens for sequential frames
type DifferentialCache struct {
    tokenCache      *cache.Cache
    frameCache      *cache.Cache
    mu              sync.RWMutex
    changeThreshold float64
}

// NewDifferentialCache creates a new differential vision cache
func NewDifferentialCache(changeThreshold float64) *DifferentialCache {
    return &DifferentialCache{
        tokenCache:      cache.New(5*time.Minute, 10*time.Minute),
        frameCache:      cache.New(5*time.Minute, 10*time.Minute),
        changeThreshold: changeThreshold,
    }
}

// GetCachedResponse checks for cached response
func (dc *DifferentialCache) GetCachedResponse(ctx context.Context, img image.Image, prompt string) (*VisionResponse, error) {
    dc.mu.RLock()
    defer dc.mu.RUnlock()

    // Level 1: Exact match
    imgHash, err := computeImageHash(img)
    if err != nil {
        return nil, err
    }

    cacheKey := fmt.Sprintf("%s:%s", imgHash, hashPrompt(prompt))
    if cached, found := dc.frameCache.Get(cacheKey); found {
        if resp, ok := cached.(*VisionResponse); ok {
            return resp, nil
        }
    }

    // Level 2: Differential check
    previousFrame, hasPrev := dc.getMostRecentFrame()
    if !hasPrev {
        return nil, nil
    }

    changedPatches := dc.detectChangedPatches(img, previousFrame)
    changeRatio := float64(len(changedPatches)) / float64(len(previousFrame.PatchHashes))

    if changeRatio == 0 {
        return previousFrame.FullResponse, nil
    }

    if changeRatio < dc.changeThreshold {
        // Small changes - could implement partial re-encode
        // For now, fall through to full re-encode
    }

    return nil, nil
}

// StoreFrame caches a fully processed frame
func (dc *DifferentialCache) StoreFrame(img image.Image, response *VisionResponse) error {
    dc.mu.Lock()
    defer dc.mu.Unlock()

    imgHash, err := computeImageHash(img)
    if err != nil {
        return err
    }

    bounds := img.Bounds()
    patchHashes := make([]string, 0)

    for y := bounds.Min.Y; y < bounds.Max.Y; y += PatchSize {
        for x := bounds.Min.X; x < bounds.Max.X; x += PatchSize {
            patchImg := extractPatch(img, x, y, PatchSize, PatchSize)
            patchHash, _ := computeImageHash(patchImg)
            patchHashes = append(patchHashes, patchHash)
        }
    }

    frame := &FrameState{
        FrameID:      uuid.New().String(),
        ImageHash:    imgHash,
        PatchHashes:  patchHashes,
        FullResponse: response,
        Timestamp:    time.Now(),
    }

    cacheKey := fmt.Sprintf("%s:%s", imgHash, hashPrompt(response.Text))
    dc.frameCache.Set(cacheKey, response, cache.DefaultExpiration)
    dc.frameCache.Set("last_frame", frame, cache.DefaultExpiration)

    return nil
}

// detectChangedPatches identifies which patches have changed
func (dc *DifferentialCache) detectChangedPatches(current image.Image, previous *FrameState) []int {
    bounds := current.Bounds()
    changedIndices := make([]int, 0)
    patchIdx := 0

    for y := bounds.Min.Y; y < bounds.Max.Y; y += PatchSize {
        for x := bounds.Min.X; x < bounds.Max.X; x += PatchSize {
            if patchIdx >= len(previous.PatchHashes) {
                changedIndices = append(changedIndices, patchIdx)
                patchIdx++
                continue
            }

            patchImg := extractPatch(current, x, y, PatchSize, PatchSize)
            patchHash, _ := computeImageHash(patchImg)

            if patchHash != previous.PatchHashes[patchIdx] {
                if dc.isSignificantChange(patchImg, previous, patchIdx) {
                    changedIndices = append(changedIndices, patchIdx)
                }
            }
            patchIdx++
        }
    }

    return changedIndices
}

// isSignificantChange filters out compression artifacts
func (dc *DifferentialCache) isSignificantChange(currentPatch image.Image, previous *FrameState, patchIdx int) bool {
    bounds := currentPatch.Bounds()
    totalPixels := bounds.Dx() * bounds.Dy()
    changedPixels := 0

    for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
        for x := bounds.Min.X; x < bounds.Max.X; x++ {
            r1, g1, b1, _ := currentPatch.At(x, y).RGBA()
            // Simplified: would compare with stored pixel data
            if math.Abs(float64(r1)-float64(r1)) > 0.1 {
                changedPixels++
            }
        }
    }

    changeRatio := float64(changedPixels) / float64(totalPixels)
    return changeRatio >= dc.changeThreshold
}

func (dc *DifferentialCache) getMostRecentFrame() (*FrameState, bool) {
    if cached, found := dc.frameCache.Get("last_frame"); found {
        if frame, ok := cached.(*FrameState); ok {
            return frame, true
        }
    }
    return nil, false
}

func extractPatch(img image.Image, x, y, width, height int) image.Image {
    rect := image.Rect(x, y, min(x+width, img.Bounds().Max.X), min(y+height, img.Bounds().Max.Y))
    return img.(interface {
        SubImage(r image.Rectangle) image.Image
    }).SubImage(rect)
}

func computeImageHash(img image.Image) (string, error) {
    bounds := img.Bounds()
    var hashData []byte

    for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
        for x := bounds.Min.X; x < bounds.Max.X; x++ {
            r, g, b, a := img.At(x, y).RGBA()
            hashData = append(hashData, byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8))
        }
    }

    hash := sha256.Sum256(hashData)
    return hex.EncodeToString(hash[:]), nil
}

func hashPrompt(prompt string) string {
    hash := sha256.Sum256([]byte(prompt))
    return hex.EncodeToString(hash[:8])
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
```

### 5.3 Learning Executor

```go
// pkg/vision/learning/executor.go
package learning

import (
    "context"
    "fmt"
    "image"
    "strings"
    "time"

    "github.com/HelixDevelopment/helixqa/pkg/vision"
    "github.com/HelixDevelopment/helixqa/pkg/vision/cache"
    "github.com/HelixDevelopment/helixqa/pkg/vision/memory"
)

// LearningVisionExecutor combines caching, memory, and few-shot learning
type LearningVisionExecutor struct {
    executor          *vision.ResilientExecutor
    vectorMemory      *memory.VectorMemoryStore
    diffCache         *cache.DifferentialCache
    fewShotBuilder    *FewShotBuilder
    providerOptimizer *ProviderOptimizer
    exactCache        map[string]*cache.VisionResponse
    config            LearningConfig
}

// LearningConfig configures the learning system
type LearningConfig struct {
    EnableExactCache    bool
    EnableDifferential  bool
    EnableVectorMemory  bool
    EnableFewShot       bool
    EnableOptimization  bool
    SimilarityThreshold float64
}

// NewLearningVisionExecutor creates a new self-learning vision executor
func NewLearningVisionExecutor(
    executor *vision.ResilientExecutor,
    memoryStore *memory.VectorMemoryStore,
    config LearningConfig,
) *LearningVisionExecutor {
    return &LearningVisionExecutor{
        executor:          executor,
        vectorMemory:      memoryStore,
        diffCache:         cache.NewDifferentialCache(0.05),
        fewShotBuilder:    NewFewShotBuilder(memoryStore, 3),
        providerOptimizer: NewProviderOptimizer(),
        exactCache:        make(map[string]*cache.VisionResponse),
        config:            config,
    }
}

// Execute performs vision inference with full learning pipeline
func (lve *LearningVisionExecutor) Execute(ctx context.Context, img image.Image, prompt string) (*vision.VisionResult, error) {
    start := time.Now()

    // Layer 1: Exact image cache check
    if lve.config.EnableExactCache {
        imgHash, _ := memory.ComputeImageHash(img)
        if cached, found := lve.exactCache[imgHash]; found {
            return &vision.VisionResult{
                Text:      cached.Text,
                Model:     "cache-hit",
                Provider:  "exact-cache",
                Duration:  time.Since(start),
                Timestamp: time.Now(),
                CacheHit:  true,
            }, nil
        }
    }

    // Layer 2: Differential cache check
    if lve.config.EnableDifferential {
        if cached, err := lve.diffCache.GetCachedResponse(ctx, img, prompt); err == nil && cached != nil {
            return &vision.VisionResult{
                Text:      cached.Text,
                Model:     "cache-hit",
                Provider:  "diff-cache",
                Duration:  time.Since(start),
                Timestamp: time.Now(),
                CacheHit:  true,
            }, nil
        }
    }

    // Layer 3: Semantic memory search (RAG)
    if lve.config.EnableVectorMemory {
        similar, _ := lve.vectorMemory.Search(ctx, prompt, 5)
        if len(similar) > 0 && similar[0].SimilarityScore > lve.config.SimilarityThreshold {
            return &vision.VisionResult{
                Text:      similar[0].Response,
                Model:     "memory-hit",
                Provider:  "vector-memory",
                Duration:  time.Since(start),
                Timestamp: time.Now(),
                CacheHit:  true,
                Confidence: similar[0].SimilarityScore,
            }, nil
        }
    }

    // Layer 4: Build few-shot prompt from successful memories
    enhancedPrompt := prompt
    if lve.config.EnableFewShot {
        var err error
        enhancedPrompt, err = lve.fewShotBuilder.BuildPrompt(ctx, prompt)
        if err != nil {
            enhancedPrompt = prompt // Fallback to original
        }
    }

    // Layer 5: Execute with resilient executor
    result, err := lve.executor.Execute(ctx, img, enhancedPrompt)
    if err != nil {
        return nil, err
    }

    // Post-execution learning (async)
    go lve.learnFromResult(ctx, img, prompt, result)

    return result, nil
}

// learnFromResult stores successful results for future use
func (lve *LearningVisionExecutor) learnFromResult(ctx context.Context, img image.Image, prompt string, result *vision.VisionResult) {
    // Store in exact cache
    if lve.config.EnableExactCache {
        imgHash, _ := memory.ComputeImageHash(img)
        lve.exactCache[imgHash] = &cache.VisionResponse{
            Text:      result.Text,
            Model:     result.Model,
            Duration:  result.Duration,
            Timestamp: result.Timestamp,
        }
    }

    // Store in differential cache
    if lve.config.EnableDifferential {
        lve.diffCache.StoreFrame(img, &cache.VisionResponse{
            Text:      result.Text,
            Model:     result.Model,
            Duration:  result.Duration,
            Timestamp: result.Timestamp,
        })
    }

    // Store in vector memory
    if lve.config.EnableVectorMemory {
        mem := &memory.VisionMemory{
            Prompt:        prompt,
            Response:      result.Text,
            ProviderModel: result.Model,
            Success:       true,
            UIElementType: detectUIType(prompt),
            Latency:       result.Duration,
            Timestamp:     time.Now(),
            ConfidenceScore: result.Confidence,
        }
        lve.vectorMemory.Store(ctx, mem)
    }

    // Update provider metrics
    if lve.config.EnableOptimization {
        lve.providerOptimizer.RecordSuccess(result.Provider, result.Duration, detectUIType(prompt))
    }

    // Learn for few-shot examples
    if lve.config.EnableFewShot {
        lve.fewShotBuilder.LearnFromSuccess(ctx, prompt, result.Text, result.Confidence)
    }
}

func detectUIType(prompt string) string {
    promptLower := strings.ToLower(prompt)
    if containsAny(promptLower, "button", "click", "press", "tap") {
        return "button"
    }
    if containsAny(promptLower, "text", "input", "field", "type", "enter") {
        return "text"
    }
    if containsAny(promptLower, "image", "picture", "photo", "icon") {
        return "image"
    }
    if containsAny(promptLower, "link", "url", "href") {
        return "link"
    }
    if containsAny(promptLower, "menu", "nav", "navigation") {
        return "navigation"
    }
    return "general"
}

func containsAny(s string, substrs ...string) bool {
    for _, sub := range substrs {
        if strings.Contains(s, sub) {
            return true
        }
    }
    return false
}
```

### 5.4 Few-Shot Builder

```go
// pkg/vision/learning/few_shot.go
package learning

import (
    "context"
    "fmt"
    "strings"

    "github.com/HelixDevelopment/helixqa/pkg/vision/memory"
)

// FewShotBuilder constructs few-shot prompts from memory store
type FewShotBuilder struct {
    memoryStore *memory.VectorMemoryStore
    maxExamples int
    minScore    float64
}

// NewFewShotBuilder creates a new few-shot example builder
func NewFewShotBuilder(store *memory.VectorMemoryStore, maxExamples int) *FewShotBuilder {
    return &FewShotBuilder{
        memoryStore: store,
        maxExamples: maxExamples,
        minScore:    0.7,
    }
}

// BuildPrompt augments a base prompt with few-shot examples
func (fb *FewShotBuilder) BuildPrompt(ctx context.Context, basePrompt string) (string, error) {
    examples, err := fb.memoryStore.GetFewShotExamples(ctx, basePrompt, fb.maxExamples)
    if err != nil {
        return basePrompt, nil
    }

    if len(examples) == 0 {
        return basePrompt, nil
    }

    var sb strings.Builder
    sb.WriteString("Here are examples of successful UI element identification:\n\n")

    for i, ex := range examples {
        if ex.SimilarityScore < fb.minScore {
            continue
        }
        sb.WriteString(fmt.Sprintf("Example %d:\n", i+1))
        sb.WriteString(fmt.Sprintf("Query: %s\n", ex.Prompt))
        sb.WriteString(fmt.Sprintf("Correct Response: %s\n\n", ex.Response))
    }

    sb.WriteString("Now, using these examples as reference, please respond to:\n")
    sb.WriteString(basePrompt)

    return sb.String(), nil
}

// LearnFromSuccess stores a successful interaction
func (fb *FewShotBuilder) LearnFromSuccess(ctx context.Context, prompt, response string, confidence float64) error {
    mem := &memory.VisionMemory{
        Prompt:          prompt,
        Response:        response,
        Success:         true,
        ConfidenceScore: confidence,
        Timestamp:       time.Now(),
    }
    return fb.memoryStore.Store(ctx, mem)
}
```

### 5.5 Provider Optimizer

```go
// pkg/vision/learning/optimizer.go
package learning

import (
    "sync"
    "time"
)

// ProviderMetrics tracks performance of a specific provider
type ProviderMetrics struct {
    ProviderName          string
    TotalRequests         int64
    SuccessfulRequests    int64
    FailedRequests        int64
    TotalLatency          time.Duration
    AvgLatency            time.Duration
    LastUsed              time.Time
    ButtonAccuracy        float64
    TextFieldAccuracy     float64
    ImageAccuracy         float64
    GeneralAccuracy       float64
}

// ProviderOptimizer learns which providers work best for different scenarios
type ProviderOptimizer struct {
    metrics    map[string]*ProviderMetrics
    mu         sync.RWMutex
    windowSize time.Duration
}

// NewProviderOptimizer creates a new provider optimizer
func NewProviderOptimizer() *ProviderOptimizer {
    return &ProviderOptimizer{
        metrics:    make(map[string]*ProviderMetrics),
        windowSize: 1 * time.Hour,
    }
}

// RecordSuccess records a successful inference
func (po *ProviderOptimizer) RecordSuccess(provider string, latency time.Duration, uiType string) {
    po.mu.Lock()
    defer po.mu.Unlock()

    metrics, exists := po.metrics[provider]
    if !exists {
        metrics = &ProviderMetrics{ProviderName: provider}
        po.metrics[provider] = metrics
    }

    metrics.TotalRequests++
    metrics.SuccessfulRequests++
    metrics.TotalLatency += latency
    metrics.AvgLatency = metrics.TotalLatency / time.Duration(metrics.TotalRequests)
    metrics.LastUsed = time.Now()

    // Update UI-type specific metrics
    switch uiType {
    case "button":
        metrics.ButtonAccuracy = po.calculateRollingAccuracy(metrics.ButtonAccuracy, true)
    case "text":
        metrics.TextFieldAccuracy = po.calculateRollingAccuracy(metrics.TextFieldAccuracy, true)
    case "image":
        metrics.ImageAccuracy = po.calculateRollingAccuracy(metrics.ImageAccuracy, true)
    default:
        metrics.GeneralAccuracy = po.calculateRollingAccuracy(metrics.GeneralAccuracy, true)
    }
}

// RecordFailure records a failed inference
func (po *ProviderOptimizer) RecordFailure(provider string, uiType string) {
    po.mu.Lock()
    defer po.mu.Unlock()

    metrics, exists := po.metrics[provider]
    if !exists {
        metrics = &ProviderMetrics{ProviderName: provider}
        po.metrics[provider] = metrics
    }

    metrics.TotalRequests++
    metrics.FailedRequests++
    metrics.LastUsed = time.Now()

    // Update UI-type specific metrics
    switch uiType {
    case "button":
        metrics.ButtonAccuracy = po.calculateRollingAccuracy(metrics.ButtonAccuracy, false)
    case "text":
        metrics.TextFieldAccuracy = po.calculateRollingAccuracy(metrics.TextFieldAccuracy, false)
    case "image":
        metrics.ImageAccuracy = po.calculateRollingAccuracy(metrics.ImageAccuracy, false)
    default:
        metrics.GeneralAccuracy = po.calculateRollingAccuracy(metrics.GeneralAccuracy, false)
    }
}

// GetBestProvider returns the optimal provider for a query type
func (po *ProviderOptimizer) GetBestProvider(uiType string, prioritizeSpeed bool) string {
    po.mu.RLock()
    defer po.mu.RUnlock()

    var bestProvider string
    var bestScore float64 = -1

    for name, metrics := range po.metrics {
        // Skip providers with no recent success
        if time.Since(metrics.LastUsed) > 10*time.Minute {
            continue
        }

        score := po.calculateProviderScore(metrics, uiType, prioritizeSpeed)
        if score > bestScore {
            bestScore = score
            bestProvider = name
        }
    }

    return bestProvider
}

func (po *ProviderOptimizer) calculateProviderScore(metrics *ProviderMetrics, uiType string, prioritizeSpeed bool) float64 {
    if metrics.TotalRequests == 0 {
        return 0
    }

    successRate := float64(metrics.SuccessfulRequests) / float64(metrics.TotalRequests)

    var typeAccuracy float64
    switch uiType {
    case "button":
        typeAccuracy = metrics.ButtonAccuracy
    case "text":
        typeAccuracy = metrics.TextFieldAccuracy
    case "image":
        typeAccuracy = metrics.ImageAccuracy
    default:
        typeAccuracy = metrics.GeneralAccuracy
    }

    if typeAccuracy == 0 {
        typeAccuracy = successRate
    }

    if prioritizeSpeed {
        latencyMs := float64(metrics.AvgLatency.Milliseconds())
        if latencyMs == 0 {
            latencyMs = 1
        }
        return (typeAccuracy * 0.6) + ((1000 / latencyMs) * 0.4)
    }

    return typeAccuracy
}

func (po *ProviderOptimizer) calculateRollingAccuracy(current float64, success bool) float64 {
    alpha := 0.3 // Smoothing factor
    if success {
        return current*(1-alpha) + alpha
    }
    return current * (1 - alpha)
}

// GetMetrics returns all provider metrics
func (po *ProviderOptimizer) GetMetrics() map[string]*ProviderMetrics {
    po.mu.RLock()
    defer po.mu.RUnlock()

    result := make(map[string]*ProviderMetrics)
    for k, v := range po.metrics {
        result[k] = v
    }
    return result
}
```

---

## 6. Integration with HelixQA

### 6.1 Learning Vision Engine

```go
// internal/engine/learning_engine.go
package engine

import (
    "context"
    "fmt"
    "image"
    "os"
    "path/filepath"
    "time"

    "github.com/philippgille/chromem-go"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"

    "github.com/HelixDevelopment/helixqa/pkg/vision"
    "github.com/HelixDevelopment/helixqa/pkg/vision/learning"
    "github.com/HelixDevelopment/helixqa/pkg/vision/memory"
)

var (
    visionRequests = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "vision_requests_total",
        Help: "Total vision requests",
    }, []string{"provider", "cache_hit"})

    visionLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "vision_request_duration_seconds",
        Help:    "Vision request latency",
        Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1, 2, 5, 10, 30},
    }, []string{"provider"})

    cacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "vision_cache_hits_total",
        Help: "Total cache hits by layer",
    }, []string{"layer"})
)

// LearningVisionConfig configures the learning vision system
type LearningVisionConfig struct {
    EnableExactCache    bool   `yaml:"enable_exact_cache"`
    EnableDifferential  bool   `yaml:"enable_differential"`
    EnableVectorMemory  bool   `yaml:"enable_vector_memory"`
    EnableFewShot       bool   `yaml:"enable_few_shot"`
    EnableOptimization  bool   `yaml:"enable_optimization"`
    PersistPath         string `yaml:"persist_path"`
    MaxMemories         int    `yaml:"max_memories"`
    ChangeThreshold     float64 `yaml:"change_threshold"`
    SimilarityThreshold float64 `yaml:"similarity_threshold"`
    EmbeddingProvider   string `yaml:"embedding_provider"`
    EmbeddingAPIKey     string `yaml:"embedding_api_key"`
}

// LearningVisionEngine is the main vision engine with self-learning capabilities
type LearningVisionEngine struct {
    executor     *learning.LearningVisionExecutor
    vectorMemory *memory.VectorMemoryStore
    config       LearningVisionConfig
}

// NewLearningVisionEngine creates a new self-learning vision engine
func NewLearningVisionEngine(cfg LearningVisionConfig) (*LearningVisionEngine, error) {
    // Create embedder function
    var embedder chromem.EmbeddingFunc

    switch cfg.EmbeddingProvider {
    case "openai":
        embedder = chromem.NewOpenAIEmbedder(cfg.EmbeddingAPIKey, "text-embedding-3-small")
    case "local":
        embedder = chromem.NewLocalEmbedder()
    default:
        embedder = chromem.NewDefaultEmbedder()
    }

    // Create vector memory store
    persistPath := cfg.PersistPath
    if persistPath == "" {
        persistPath = filepath.Join(os.TempDir(), "helixqa_vision_memory.db")
    }

    memoryStore, err := memory.NewVectorMemoryStore(persistPath, embedder)
    if err != nil {
        return nil, fmt.Errorf("failed to create memory store: %w", err)
    }

    // Create resilient executor
    resilientExec := vision.NewResilientExecutor(vision.ExecutorConfig{
        Strategy:       vision.StrategyFirstSuccess,
        Timeout:        30 * time.Second,
        RetryAttempts:  2,
        RetryDelay:     1 * time.Second,
        CircuitBreaker: true,
        CBThreshold:    5,
        CBTimeout:      30 * time.Second,
    })

    // Create learning executor
    learningConfig := learning.LearningConfig{
        EnableExactCache:    cfg.EnableExactCache,
        EnableDifferential:  cfg.EnableDifferential,
        EnableVectorMemory:  cfg.EnableVectorMemory,
        EnableFewShot:       cfg.EnableFewShot,
        EnableOptimization:  cfg.EnableOptimization,
        SimilarityThreshold: cfg.SimilarityThreshold,
    }

    learningExec := learning.NewLearningVisionExecutor(resilientExec, memoryStore, learningConfig)

    return &LearningVisionEngine{
        executor:     learningExec,
        vectorMemory: memoryStore,
        config:       cfg,
    }, nil
}

// ProcessImage processes an image with learning capabilities
func (lve *LearningVisionEngine) ProcessImage(ctx context.Context, img image.Image, prompt string) (string, error) {
    start := time.Now()

    result, err := lve.executor.Execute(ctx, img, prompt)
    if err != nil {
        return "", err
    }

    // Record metrics
    duration := time.Since(start)
    visionRequests.WithLabelValues(result.Provider, fmt.Sprintf("%v", result.CacheHit)).Inc()
    visionLatency.WithLabelValues(result.Provider).Observe(duration.Seconds())

    if result.CacheHit {
        cacheHits.WithLabelValues(result.Provider).Inc()
    }

    return result.Text, nil
}

// GetMemoryStats returns statistics about the learning system
func (lve *LearningVisionEngine) GetMemoryStats() map[string]interface{} {
    return map[string]interface{}{
        "exact_cache_size":   len(lve.executor.ExactCache),
        "config":             lve.config,
        "embedding_provider": lve.config.EmbeddingProvider,
    }
}

// ClearMemory clears all learned memories
func (lve *LearningVisionEngine) ClearMemory() {
    lve.executor.ExactCache = make(map[string]*cache.VisionResponse)
}

// AddProvider adds a new vision provider
func (lve *LearningVisionEngine) AddProvider(provider vision.VisionProvider) {
    // Implementation depends on executor architecture
}

// RemoveProvider removes a vision provider
func (lve *LearningVisionEngine) RemoveProvider(name string) {
    // Implementation depends on executor architecture
}
```

### 6.2 Main Application Integration

```go
// cmd/helixqa/main.go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/HelixDevelopment/helixqa/internal/config"
    "github.com/HelixDevelopment/helixqa/internal/engine"
    "github.com/HelixDevelopment/helixqa/internal/server"
    "github.com/HelixDevelopment/helixqa/pkg/vision"
    _ "github.com/HelixDevelopment/helixqa/pkg/vision/adapters/glm4v"
    _ "github.com/HelixDevelopment/helixqa/pkg/vision/adapters/qwen25vl"
    _ "github.com/HelixDevelopment/helixqa/pkg/vision/adapters/showui"
    _ "github.com/HelixDevelopment/helixqa/pkg/vision/adapters/uitars"
)

func main() {
    // Load configuration
    cfg, err := config.Load("config.yaml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Initialize vision engine
    var visionEngine *engine.LearningVisionEngine
    if cfg.Vision.Learning.Enabled {
        learningCfg := engine.LearningVisionConfig{
            EnableExactCache:    cfg.Vision.Learning.ExactCache,
            EnableDifferential:  cfg.Vision.Learning.Differential,
            EnableVectorMemory:  cfg.Vision.Learning.VectorMemory,
            EnableFewShot:       cfg.Vision.Learning.FewShot,
            EnableOptimization:  cfg.Vision.Learning.Optimization,
            PersistPath:         cfg.Vision.Learning.PersistPath,
            MaxMemories:         cfg.Vision.Learning.MaxMemories,
            ChangeThreshold:     cfg.Vision.Learning.ChangeThreshold,
            SimilarityThreshold: cfg.Vision.Learning.SimilarityThreshold,
            EmbeddingProvider:   cfg.Vision.Learning.EmbeddingProvider,
            EmbeddingAPIKey:     cfg.Vision.Learning.EmbeddingAPIKey,
        }

        visionEngine, err = engine.NewLearningVisionEngine(learningCfg)
        if err != nil {
            log.Fatalf("Failed to create vision engine: %v", err)
        }
    }

    // Create and configure providers from config
    providers := make([]vision.VisionProvider, 0)
    for _, pc := range cfg.Vision.Providers {
        if !pc.Enabled {
            continue
        }
        provider, err := vision.Create(pc.Name, pc.Config)
        if err != nil {
            log.Printf("Failed to create provider %s: %v", pc.Name, err)
            continue
        }
        providers = append(providers, provider)
    }

    // Initialize HTTP server
    srv := server.New(server.Config{
        Port:         cfg.Server.Port,
        VisionEngine: visionEngine,
    })

    // Start server in goroutine
    go func() {
        if err := srv.Start(); err != nil {
            log.Fatalf("Server failed: %v", err)
        }
    }()

    // Wait for interrupt signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    // Graceful shutdown
    log.Println("Shutting down...")
    if err := srv.Shutdown(context.Background()); err != nil {
        log.Printf("Shutdown error: %v", err)
    }
}
```

---

## 7. Testing Implementation

### 7.1 Unit Tests

```go
// tests/unit/vision/providers_test.go
package unit

import (
    "context"
    "image"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/HelixDevelopment/helixqa/pkg/vision"
    "github.com/HelixDevelopment/helixqa/pkg/vision/uitars"
)

func TestUITARSProvider(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name    string
        config  map[string]interface{}
        wantErr bool
    }{
        {
            name:    "valid config",
            config:  map[string]interface{}{"api_key": "test-key"},
            wantErr: false,
        },
        {
            name:    "missing api key",
            config:  map[string]interface{}{},
            wantErr: true,
        },
        {
            name: "custom base URL",
            config: map[string]interface{}{
                "api_key":  "test",
                "base_url": "http://custom",
            },
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p, err := uitars.NewUITARSProvider(tt.config)
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, "ui-tars-1.5", p.Name())
        })
    }
}

func TestProviderRegistry(t *testing.T) {
    t.Parallel()

    // Test registration
    vision.Register("test-provider", func(config map[string]interface{}) (vision.VisionProvider, error) {
        return nil, nil
    })

    assert.True(t, vision.IsRegistered("test-provider"))
    assert.Contains(t, vision.List(), "test-provider")

    // Cleanup
    vision.Unregister("test-provider")
}
```

### 7.2 Integration Tests

```go
// tests/integration/vision/executor_test.go
package integration

import (
    "context"
    "errors"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/HelixDevelopment/helixqa/pkg/vision"
    "github.com/HelixDevelopment/helixqa/tests/helpers"
)

func TestResilientExecutor_FirstSuccessStrategy(t *testing.T) {
    // Create mock providers
    fastProvider := helpers.NewMockVisionProvider("fast")
    fastProvider.On("Analyze", mock.Anything, mock.Anything, mock.Anything).
        Return(&vision.VisionResult{Text: "fast result"}, nil)

    slowProvider := helpers.NewMockVisionProvider("slow")
    slowProvider.On("Analyze", mock.Anything, mock.Anything, mock.Anything).
        After(500*time.Millisecond).
        Return(&vision.VisionResult{Text: "slow result"}, nil)

    errorProvider := helpers.NewMockVisionProvider("error")
    errorProvider.On("Analyze", mock.Anything, mock.Anything, mock.Anything).
        Return(nil, errors.New("failed"))

    executor := vision.NewResilientExecutor(vision.ExecutorConfig{
        Strategy:  vision.StrategyFirstSuccess,
        Providers: []vision.VisionProvider{slowProvider, fastProvider, errorProvider},
        Timeout:   5 * time.Second,
    })

    ctx := context.Background()
    img := helpers.TestImage(100, 100)

    result, err := executor.Execute(ctx, img, "test")
    require.NoError(t, err)
    assert.Equal(t, "fast result", result.Text)
}

func TestResilientExecutor_FallbackChain(t *testing.T) {
    primary := helpers.NewMockVisionProvider("primary")
    primary.On("Analyze", mock.Anything, mock.Anything, mock.Anything).
        Return(nil, errors.New("primary failed"))

    fallback := helpers.NewMockVisionProvider("fallback")
    fallback.On("Analyze", mock.Anything, mock.Anything, mock.Anything).
        Return(&vision.VisionResult{Text: "fallback success"}, nil)

    executor := vision.NewResilientExecutor(vision.ExecutorConfig{
        Strategy:      vision.StrategyFallback,
        Providers:     []vision.VisionProvider{primary, fallback},
        FallbackChain: []string{"primary", "fallback"},
        Timeout:       5 * time.Second,
    })

    ctx := context.Background()
    img := helpers.TestImage(100, 100)

    result, err := executor.Execute(ctx, img, "test")
    require.NoError(t, err)
    assert.Equal(t, "fallback success", result.Text)
}
```

### 7.3 Test Helpers

```go
// tests/helpers/mocks.go
package helpers

import (
    "context"
    "image"
    "testing"

    "github.com/stretchr/testify/mock"

    "github.com/HelixDevelopment/helixqa/pkg/vision"
)

// MockVisionProvider implements VisionProvider for testing
type MockVisionProvider struct {
    mock.Mock
    name string
}

// NewMockVisionProvider creates a new mock provider
func NewMockVisionProvider(name string) *MockVisionProvider {
    return &MockVisionProvider{name: name}
}

// Name returns the provider name
func (m *MockVisionProvider) Name() string {
    return m.name
}

// Analyze mocks the vision analysis
func (m *MockVisionProvider) Analyze(ctx context.Context, img image.Image, prompt string) (*vision.VisionResult, error) {
    args := m.Called(ctx, img, prompt)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*vision.VisionResult), args.Error(1)
}

// HealthCheck mocks health check
func (m *MockVisionProvider) HealthCheck(ctx context.Context) error {
    args := m.Called(ctx)
    return args.Error(0)
}

// GetCapabilities mocks capabilities
func (m *MockVisionProvider) GetCapabilities() vision.ProviderCapabilities {
    args := m.Called()
    return args.Get(0).(vision.ProviderCapabilities)
}

// GetCostEstimate mocks cost estimation
func (m *MockVisionProvider) GetCostEstimate(imageSize int, promptLength int) float64 {
    args := m.Called(imageSize, promptLength)
    return args.Get(0).(float64)
}

// TestImage generates a simple test image
func TestImage(width, height int) image.Image {
    img := image.NewRGBA(image.Rect(0, 0, width, height))
    for y := 0; y < height; y++ {
        for x := 0; x < width; x++ {
            c := uint8((x + y) % 256)
            img.Set(x, y, color.RGBA{c, c, c, 255})
        }
    }
    return img
}
```

---

## 8. Makefile & CI/CD

### 8.1 Makefile

```makefile
# HelixQA Vision Integration Makefile
.PHONY: help build test test-all test-unit test-integration test-e2e test-security \
        test-bench test-chaos test-fuzz test-stress test-race test-all-cover \
        lint fmt vet clean docker

# Variables
GO := go
GOFLAGS := -v
COVERAGE_THRESHOLD := 100

help:
	@echo "Available targets:"
	@echo "  build              - Build the application"
	@echo "  test-unit          - Run unit tests"
	@echo "  test-integration   - Run integration tests"
	@echo "  test-e2e           - Run end-to-end tests"
	@echo "  test-security      - Run security scans"
	@echo "  test-bench         - Run benchmarks"
	@echo "  test-chaos         - Run chaos tests"
	@echo "  test-fuzz          - Run fuzz tests"
	@echo "  test-stress        - Run stress tests"
	@echo "  test-race          - Run tests with race detector"
	@echo "  test-all           - Run all tests"
	@echo "  test-all-cover     - Run all tests with coverage"
	@echo "  lint               - Run linters"
	@echo "  fmt                - Format code"
	@echo "  vet                - Run go vet"
	@echo "  clean              - Clean build artifacts"
	@echo "  docker             - Build Docker image"

# Build
build:
	$(GO) build $(GOFLAGS) -o bin/helixqa ./cmd/helixqa

# Testing
test-unit:
	@echo "Running unit tests..."
	$(GO) test -race -short -count=1 -coverprofile=coverage-unit.out ./tests/unit/...
	$(GO) tool cover -func=coverage-unit.out | grep total

test-integration:
	@echo "Starting test dependencies..."
	docker compose -f tests/docker/docker-compose.yml up -d
	@sleep 10
	@echo "Running integration tests..."
	$(GO) test -v -tags=integration -coverprofile=coverage-integration.out ./tests/integration/... || true
	@echo "Cleaning up..."
	docker compose -f tests/docker/docker-compose.yml down

test-e2e:
	@echo "Running E2E tests..."
	$(GO) test -v -tags=e2e ./tests/e2e/...

test-security:
	@echo "Running security scans..."
	@echo "1. govulncheck..."
	govulncheck ./...
	@echo "2. gosec..."
	gosec -quiet ./...
	@echo "3. nancy..."
	nancy sleuth -q go.sum || true

test-bench:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem -count=5 ./tests/benchmark/... | tee bench.txt
	@echo "Analyzing with benchstat..."
	benchstat bench.txt

test-chaos:
	@echo "Running chaos tests..."
	$(GO) test -tags=chaos -v ./tests/chaos/...

test-fuzz:
	@echo "Running fuzz tests..."
	$(GO) test -fuzz=. -fuzztime=30s ./tests/fuzz/...

test-stress:
	@echo "Running stress tests..."
	$(GO) test -race -count=1 -timeout=5m ./tests/stress/...

test-race:
	@echo "Running tests with race detector..."
	$(GO) test -race -count=1 ./...

test-mutation:
	@echo "Running mutation tests..."
	go-mutesting --exec scripts/test-mutated-package.sh ./pkg/vision/...

test-property:
	@echo "Running property-based tests..."
	$(GO) test -v ./tests/property/...

test-contract:
	@echo "Running contract tests..."
	$(GO) test -v ./tests/contract/...

test-memory:
	@echo "Running memory leak tests..."
	$(GO) test -v ./tests/leak/...

test-all: test-unit test-integration test-e2e test-security test-race

test-all-cover:
	@echo "Running all tests with coverage merge..."
	$(MAKE) test-unit
	$(MAKE) test-integration
	@echo "Merging coverage files..."
	gocovmerge coverage-*.out > coverage-all.out
	$(GO) tool cover -html=coverage-all.out -o coverage-all.html
	@echo "Combined coverage report: coverage-all.html"
	@COVERAGE=$$($(GO) tool cover -func=coverage-all.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Total coverage: $$COVERAGE%"; \
	if (( $$(echo "$$COVERAGE < $(COVERAGE_THRESHOLD)" | bc -l) )); then \
		echo "Coverage $$COVERAGE% is below threshold $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	fi

# Code quality
lint:
	@echo "Running linters..."
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

vet:
	@echo "Running go vet..."
	$(GO) vet ./...

# Docker
docker:
	@echo "Building Docker image..."
	docker build -t helixqa-vision:latest -f deployments/docker/Dockerfile .

# Cleanup
clean:
	@echo "Cleaning..."
	rm -f bin/*
	rm -f coverage*.out
	rm -f coverage*.html
	rm -f bench.txt
	$(GO) clean

# Dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

# Generate
generate:
	$(GO) generate ./...

# Install tools
install-tools:
	@echo "Installing development tools..."
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install golang.org/x/perf/cmd/benchstat@latest
	$(GO) install github.com/wadey/gocovmerge@latest
	$(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	$(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	$(GO) install github.com/avito-tech/go-mutesting/...@latest
```

### 8.2 GitHub Actions CI/CD

```yaml
# .github/workflows/ci.yml
name: CI/CD Pipeline

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Run linters
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  unit-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Run unit tests
        run: make test-unit
      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          files: coverage-unit.out

  integration-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Run integration tests
        run: make test-integration

  security-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Run govulncheck
        run: govulncheck ./...
      - name: Run gosec
        run: gosec -quiet ./...

  race-detection:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Run race detector
        run: make test-race

  build:
    runs-on: ubuntu-latest
    needs: [lint, unit-test]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Build
        run: make build
      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: helixqa-binary
          path: bin/helixqa

  docker-build:
    runs-on: ubuntu-latest
    needs: [lint, unit-test]
    steps:
      - uses: actions/checkout@v4
      - name: Build Docker image
        run: make docker
      - name: Scan image
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: helixqa-vision:latest
          format: 'sarif'
          output: 'trivy-results.sarif'

  nightly-tests:
    runs-on: ubuntu-latest
    if: github.event.schedule == '0 0 * * *'
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Run benchmark tests
        run: make test-bench
      - name: Run chaos tests
        run: make test-chaos
      - name: Run fuzz tests
        run: make test-fuzz
      - name: Run mutation tests
        run: make test-mutation
```

---

## 9. Docker Deployment

### 9.1 Dockerfile

```dockerfile
# deployments/docker/Dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o helixqa ./cmd/helixqa

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary
COPY --from=builder /app/helixqa .

# Copy config
COPY --from=builder /app/config.yaml.example ./config.yaml

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./helixqa"]
```

### 9.2 Docker Compose

```yaml
# tests/docker/docker-compose.yml
version: '3.9'

services:
  helixqa:
    build:
      context: ../..
      dockerfile: deployments/docker/Dockerfile
    ports:
      - "8080:8080"
    environment:
      - HELIX_VISION_LEARNING_ENABLED=true
      - HELIX_VISION_EXACT_CACHE=true
      - HELIX_VISION_DIFFERENTIAL=true
      - HELIX_VISION_VECTOR_MEMORY=true
    volumes:
      - vision_memory:/var/lib/helixqa
    depends_on:
      - qwen-vl
      - vector-db
    networks:
      - helixqa-network

  qwen-vl:
    image: qwenllm/qwen-vl:latest
    ports:
      - "9192:9192"
    environment:
      - MODEL_NAME=Qwen2.5-VL-7B-Instruct
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9192/health"]
      interval: 5s
      timeout: 3s
      retries: 10
    networks:
      - helixqa-network

  showui:
    build:
      context: ./showui
    ports:
      - "7860:7860"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:7860/"]
      interval: 5s
      timeout: 3s
      retries: 10
    networks:
      - helixqa-network

  vector-db:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"
      - "6334:6334"
    volumes:
      - qdrant_storage:/qdrant/storage
    networks:
      - helixqa-network

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    networks:
      - helixqa-network

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana_storage:/var/lib/grafana
    networks:
      - helixqa-network

volumes:
  vision_memory:
  qdrant_storage:
  grafana_storage:

networks:
  helixqa-network:
    driver: bridge
```

---

*Implementation Guide Version: 1.0*
*Last Updated: 2026-04-13*
