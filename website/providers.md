# LLM Providers

HelixQA supports 40+ LLM providers. At startup, it auto-discovers which providers are available by scanning environment variables. Any provider with a valid API key is immediately usable — no code changes required.

## Quick Selection Guide

| Provider | Best For | Cost | Speed |
|----------|----------|------|-------|
| OpenRouter | Beginners — access to 100+ models via one key | Medium | Medium |
| DeepSeek | Cost-sensitive workloads | Lowest | Medium |
| Groq | Latency-sensitive pipelines | Low | Fastest |
| Anthropic | Highest quality vision analysis | High | Medium |
| OpenAI | Strong all-round capability | High | Medium |
| Ollama | Air-gapped / self-hosted environments | Free | Local |

## Configuration

Set at least one environment variable before running HelixQA:

```bash
# Recommended for most users
export OPENROUTER_API_KEY="sk-or-v1-..."

# Cheapest option
export DEEPSEEK_API_KEY="sk-..."

# Best vision analysis quality
export ANTHROPIC_API_KEY="sk-ant-..."

# Fastest inference
export GROQ_API_KEY="gsk_..."

# Self-hosted (no API key needed)
export HELIX_OLLAMA_URL="http://localhost:11434"
```

Multiple providers can be active simultaneously. The adaptive provider selects the best available one per request type (reasoning vs. vision).

## Full Provider Reference

### Tier 1 — Primary Providers

These providers have native client implementations in HelixQA:

| Provider | Environment Variable | Default Model | Notes |
|----------|---------------------|---------------|-------|
| Anthropic | `ANTHROPIC_API_KEY` | claude-sonnet-4 | Best for vision analysis |
| OpenAI | `OPENAI_API_KEY` | gpt-4o | Strong all-round |
| OpenRouter | `OPENROUTER_API_KEY` | anthropic/claude-sonnet-4 | 100+ models via one key |
| DeepSeek | `DEEPSEEK_API_KEY` | deepseek-chat | Most cost-effective |
| Groq | `GROQ_API_KEY` | llama-3.3-70b-versatile | Fastest inference |
| Ollama | `HELIX_OLLAMA_URL` | (your local model) | Self-hosted, no cost |

### Tier 2 — OpenAI-Compatible Providers

All providers below use the OpenAI `chat/completions` API format:

| Provider | Environment Variable | Default Model |
|----------|---------------------|---------------|
| AI21 | `AI21_API_KEY` | jamba-1.5-mini |
| Cerebras | `CEREBRAS_API_KEY` | llama-3.3-70b |
| Chutes | `CHUTES_API_KEY` | deepseek-chat |
| Cloudflare | `CLOUDFLARE_API_KEY` | (configured per account) |
| Codestral | `CODESTRAL_API_KEY` | codestral-latest |
| Cohere | `COHERE_API_KEY` | command-r-plus |
| Fireworks | `FIREWORKS_API_KEY` | llama-v3p3-70b-instruct |
| GitHub Models | `GITHUB_MODELS_API_KEY` | openai/gpt-4o |
| HuggingFace | `HUGGINGFACE_API_KEY` | (model-dependent) |
| Hyperbolic | `HYPERBOLIC_API_KEY` | deepseek-ai/DeepSeek-V3 |
| Kimi (Moonshot) | `KIMI_API_KEY` | moonshot-v1-8k |
| Mistral | `MISTRAL_API_KEY` | mistral-large-latest |
| Modal | `MODAL_API_KEY` | (configured per deployment) |
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

## Adaptive Provider

When multiple providers are configured, HelixQA uses an `AdaptiveProvider` that:

- Routes vision requests to providers with multimodal capability
- Routes reasoning/planning to the fastest available provider
- Falls back to the next available provider on rate limit or error
- Tracks provider health and avoids recently-failed endpoints

## Overriding the Model

To use a specific model from a provider, set the model via config or environment:

```bash
# Use a specific OpenRouter model
export OPENROUTER_API_KEY="sk-or-v1-..."
export HELIX_LLM_MODEL="meta-llama/llama-3.1-70b-instruct"
```

## Cost Optimization

For production use, consider a tiered strategy:

1. Use **Groq** (Llama 3.3 70B) for planning and test generation — fast and cheap
2. Use **Anthropic Claude** or **OpenAI GPT-4o** for vision analysis — best accuracy
3. Use **DeepSeek** for bulk text processing — lowest cost

OpenRouter lets you access all of the above with a single API key and unified billing.

## Self-Hosted with Ollama

For environments without internet access or with strict data privacy requirements:

```bash
# Install and start Ollama
curl -fsSL https://ollama.ai/install.sh | sh
ollama pull llama3.3
ollama pull llava          # vision-capable model

export HELIX_OLLAMA_URL="http://localhost:11434"
```

Ollama supports both text and vision models. For vision analysis, pull a vision-capable model such as `llava`, `bakllava`, or `moondream`.

## Related Pages

- [Installation](/installation) — setting up API keys
- [Configuration](/manual/config) — full environment variable reference
- [Architecture](/architecture) — how the adaptive provider fits in the pipeline
