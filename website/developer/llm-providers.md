# LLM Provider Configuration

HelixQA supports over 40 LLM providers through a unified `Provider` interface and auto-discovery registry. This page is the developer reference for provider configuration, selection strategy, fallback behaviour, cost management, and custom provider implementation.

## Provider Architecture

The LLM subsystem lives in `pkg/llm/` and consists of four components:

| Component | File | Role |
|-----------|------|------|
| Interface | `provider.go` | Defines `Provider`, `Message`, `Response` types |
| Adaptive router | `adaptive.go` | Routes requests to the best available provider |
| Registry | `providers_registry.go` | Auto-discovers providers from environment variables |
| Implementations | `anthropic.go`, `openai.go`, `ollama.go` | Concrete provider clients |

### Provider Interface

Every provider implements:

```go
type Provider interface {
    Chat(ctx context.Context, messages []Message) (*Response, error)
    Vision(ctx context.Context, image []byte, prompt string) (*Response, error)
    Name() string
    SupportsVision() bool
}
```

- `Chat` handles text-based LLM requests (planning, test generation, log analysis)
- `Vision` handles multimodal requests (screenshot analysis)
- `SupportsVision` declares whether the provider supports image inputs
- `Name` returns the canonical provider identifier

---

## Supported Providers

### Tier 1: Primary Providers

These providers have native client implementations with full feature support:

| Provider | Env Variable | Default Model | Vision | Notes |
|----------|-------------|---------------|--------|-------|
| Anthropic | `ANTHROPIC_API_KEY` | claude-sonnet-4 | Yes | Best vision accuracy |
| OpenAI | `OPENAI_API_KEY` | gpt-4o | Yes | Strong all-round |
| Google | `GOOGLE_API_KEY` | gemini-pro | Yes | Multimodal capable |
| OpenRouter | `OPENROUTER_API_KEY` | anthropic/claude-sonnet-4 | Varies | 100+ models via one key |
| DeepSeek | `DEEPSEEK_API_KEY` | deepseek-chat | No | Most cost-effective |
| Groq | `GROQ_API_KEY` | llama-3.3-70b-versatile | No | Fastest inference |
| Ollama | `HELIX_OLLAMA_URL` | (local model) | Optional | Self-hosted, no cost |

### Tier 2: OpenAI-Compatible Providers

All Tier 2 providers use the OpenAI `chat/completions` API format. The registry creates an OpenAI-compatible client with the provider's base URL:

| Provider | Env Variable | Default Model |
|----------|-------------|---------------|
| AI21 | `AI21_API_KEY` | jamba-1.5-mini |
| Cerebras | `CEREBRAS_API_KEY` | llama-3.3-70b |
| Chutes | `CHUTES_API_KEY` | deepseek-chat |
| Cloudflare | `CLOUDFLARE_API_KEY` | (per account) |
| Codestral | `CODESTRAL_API_KEY` | codestral-latest |
| Cohere | `COHERE_API_KEY` | command-r-plus |
| Fireworks | `FIREWORKS_API_KEY` | llama-v3p3-70b-instruct |
| GitHub Models | `GITHUB_MODELS_API_KEY` | openai/gpt-4o |
| HuggingFace | `HUGGINGFACE_API_KEY` | (model-dependent) |
| Hyperbolic | `HYPERBOLIC_API_KEY` | deepseek-ai/DeepSeek-V3 |
| Kimi (Moonshot) | `KIMI_API_KEY` | moonshot-v1-8k |
| Mistral | `MISTRAL_API_KEY` | mistral-large-latest |
| Modal | `MODAL_API_KEY` | (per deployment) |
| Nia | `NIA_API_KEY` | (provider default) |
| NLPCloud | `NLPCLOUD_API_KEY` | (provider default) |
| Novita | `NOVITA_API_KEY` | (provider default) |
| NVIDIA NIM | `NVIDIA_API_KEY` | meta/llama-3.3-70b-instruct |
| Perplexity | `PERPLEXITY_API_KEY` | sonar |
| PublicAI | `PUBLICAI_API_KEY` | (provider default) |
| Qwen (Alibaba) | `QWEN_API_KEY` | qwen-plus |
| Replicate | `REPLICATE_API_KEY` | (model-dependent) |
| SambaNova | `SAMBANOVA_API_KEY` | Meta-Llama-3.3-70B-Instruct |
| Sarvam | `SARVAM_API_KEY` | (provider default) |
| SiliconFlow | `SILICONFLOW_API_KEY` | deepseek-ai/DeepSeek-V3 |
| Together AI | `TOGETHER_API_KEY` | Llama-3.3-70B-Instruct-Turbo |
| Upstage | `UPSTAGE_API_KEY` | solar-pro |
| Venice | `VENICE_API_KEY` | (provider default) |
| Vulavula | `VULAVULA_API_KEY` | (provider default) |
| xAI (Grok) | `XAI_API_KEY` | grok-3 |
| ZAI (BigModel) | `ZAI_API_KEY` | glm-4-flash |
| Zen | `ZEN_API_KEY` | (provider default) |
| Zhipu | `ZHIPU_API_KEY` | glm-4-flash |

---

## API Key Configuration

### Single Provider

Set one environment variable and HelixQA uses that provider for all requests:

```bash
export OPENROUTER_API_KEY="sk-or-v1-..."
```

### Multiple Providers

Set multiple keys to enable the adaptive provider:

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
export GROQ_API_KEY="gsk_..."
export DEEPSEEK_API_KEY="sk-..."
```

### Via .env File

Create a `.env` file in your project root:

```env
ANTHROPIC_API_KEY=sk-ant-...
GROQ_API_KEY=gsk_...
DEEPSEEK_API_KEY=sk-...
```

Pass it to HelixQA:

```bash
helixqa autonomous --project . --platforms web --env .env
```

### Model Override

To use a specific model instead of the provider's default:

```bash
export HELIX_LLM_MODEL="gpt-4o-mini"
export HELIX_VISION_MODEL="claude-opus-4"
export HELIX_OLLAMA_MODEL="llama3.3"
```

---

## Provider Selection Strategy

### Auto-Discovery

At startup, the registry scans environment variables and creates provider instances for every key it finds. The registry runs in `providers_registry.go`:

```go
func DiscoverProviders() []Provider {
    var providers []Provider
    for _, entry := range registryEntries {
        if key := os.Getenv(entry.envKey); key != "" {
            providers = append(providers,
                entry.factory(key))
        }
    }
    return providers
}
```

### Adaptive Routing

When multiple providers are available, the `AdaptiveProvider` routes each request to the best provider based on request type:

| Request Type | Routing Logic |
|-------------|--------------|
| Vision (screenshot analysis) | Route to provider where `SupportsVision() == true`. Prefer Anthropic, then OpenAI, then Ollama with vision model. |
| Reasoning (planning, test gen) | Route to the fastest available provider. Prefer Groq, then Cerebras, then DeepSeek. |
| General chat | Route to any available provider. |

### Health Tracking

The adaptive provider tracks provider health:

- On successful request: provider stays in the active pool
- On rate limit (HTTP 429): provider is removed for a cooldown period (60 seconds default)
- On error (HTTP 5xx): provider is removed for a longer cooldown (300 seconds)
- On timeout: provider is removed for 120 seconds

After cooldown, the provider is returned to the active pool.

### Fallback Chains

When the primary provider for a request type fails, the adaptive provider cascades through the available providers:

```
Vision request:
    Anthropic -> OpenAI -> Ollama (with llava) -> FAIL

Reasoning request:
    Groq -> Cerebras -> DeepSeek -> OpenAI -> Anthropic -> FAIL
```

If all providers fail, the request returns an error with details from each attempt.

---

## Cost Management

### Token Usage Tracking

HelixQA tracks token usage per provider per session. The session report includes a cost breakdown:

```json
{
  "llm_usage": {
    "anthropic": {
      "input_tokens": 45000,
      "output_tokens": 12000,
      "requests": 28,
      "estimated_cost_usd": 0.42
    },
    "groq": {
      "input_tokens": 120000,
      "output_tokens": 35000,
      "requests": 15,
      "estimated_cost_usd": 0.02
    }
  }
}
```

### Cost Optimization Strategies

#### Strategy 1: Tiered Provider Selection

Use cheap providers for bulk work and expensive providers only for vision:

```env
# Fast and cheap for planning
GROQ_API_KEY=gsk_...

# Best accuracy for screenshot analysis
ANTHROPIC_API_KEY=sk-ant-...
```

The adaptive provider automatically routes vision requests to Anthropic and reasoning requests to Groq.

#### Strategy 2: OpenRouter Unified Billing

OpenRouter provides access to 100+ models with a single API key and unified billing. You can control which model is used per request type:

```env
OPENROUTER_API_KEY=sk-or-v1-...
HELIX_LLM_MODEL=meta-llama/llama-3.1-70b-instruct
HELIX_VISION_MODEL=anthropic/claude-sonnet-4
```

#### Strategy 3: Self-Hosted with Ollama

For zero ongoing cost (beyond compute), use Ollama:

```bash
ollama pull llama3.3          # text model
ollama pull llava              # vision model

export HELIX_OLLAMA_URL="http://localhost:11434"
export HELIX_OLLAMA_MODEL="llama3.3"
```

Ollama is ideal for:

- Air-gapped environments with no internet access
- Privacy-sensitive projects where data must not leave the network
- Development and testing where cost matters more than quality
- High-volume sessions where API costs would be prohibitive

#### Strategy 4: Session Budget Limits

Use shorter timeouts and disable curiosity for routine regression runs:

```bash
# Expensive: full exploration
helixqa autonomous --timeout 2h --curiosity=true --curiosity-timeout 30m

# Cheap: focused regression
helixqa autonomous --timeout 15m --curiosity=false
```

---

## Custom Provider Implementation

### OpenAI-Compatible Provider

For providers that support the OpenAI `chat/completions` format, add an entry to the registry in `providers_registry.go`:

```go
var registryEntries = []registryEntry{
    // Existing entries...

    {
        name:    "newprovider",
        envKey:  "NEWPROVIDER_API_KEY",
        baseURL: "https://api.newprovider.com/v1",
        model:   "default-model",
        factory: func(key string) Provider {
            return NewOpenAICompatible(
                key,
                "https://api.newprovider.com/v1",
                "default-model",
                "newprovider",
            )
        },
    },
}
```

### Fully Custom Provider

For providers with a non-standard API, implement the `Provider` interface directly. See [Extending HelixQA](/developer/extending) for the complete implementation pattern.

Key requirements:

1. **Thread safety** -- implementations must be safe for concurrent use
2. **Context respect** -- honour `ctx.Done()` for cancellation and timeouts
3. **Error wrapping** -- wrap errors with `fmt.Errorf("provider: %w", err)`
4. **Retry handling** -- return structured errors that the adaptive provider can distinguish (rate limit vs. server error vs. auth failure)

### Testing a Custom Provider

```go
func TestNewProvider_Chat(t *testing.T) {
    p := NewCustomProvider("test-key")

    messages := []Message{
        {Role: RoleUser, Content: "hello"},
    }

    resp, err := p.Chat(context.Background(), messages)
    require.NoError(t, err)
    assert.NotEmpty(t, resp.Content)
    assert.Equal(t, "newprovider", p.Name())
}

func TestNewProvider_Vision_NotSupported(t *testing.T) {
    p := NewCustomProvider("test-key")
    assert.False(t, p.SupportsVision())

    _, err := p.Vision(
        context.Background(),
        []byte("image-data"),
        "describe this",
    )
    assert.Error(t, err)
}
```

## Related Pages

- [LLM Providers Overview](/providers) -- quick selection guide and cost comparison
- [Configuration](/reference/config) -- all environment variables
- [Architecture Reference](/developer/architecture) -- where the LLM package fits
- [Extending HelixQA](/developer/extending) -- adding new providers
