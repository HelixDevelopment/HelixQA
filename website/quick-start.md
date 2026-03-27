# Quick Start

Get HelixQA running in under 5 minutes.

## 1. Build

```bash
cd HelixQA
go build -o bin/helixqa ./cmd/helixqa
```

## 2. Set an LLM API Key

```bash
# Pick one (OpenRouter recommended — access to 100+ models)
export OPENROUTER_API_KEY="sk-or-v1-..."
# Or: ANTHROPIC_API_KEY, OPENAI_API_KEY, DEEPSEEK_API_KEY, GROQ_API_KEY
```

## 3. Run

```bash
helixqa autonomous --project /path/to/your/project --platforms web --timeout 10m
```

## 4. Review Results

```bash
# Auto-generated issue tickets
cat docs/issues/HELIX-001-*.md

# Session report
cat qa-results/session-*/pipeline-report.json

# Screenshots
ls qa-results/session-*/screenshots/
```

## What Happens

1. HelixQA reads your project docs, codebase, and git history
2. An LLM generates comprehensive test cases
3. Tests execute with screenshots and crash detection
4. LLM vision analyzes every screenshot for issues
5. Issue tickets appear in `docs/issues/`

## Next Steps

- [Architecture](/architecture) — understand the pipeline
- [User Manual](/manual/cli) — full CLI reference
- [Multi-Pass QA](/manual/multi-pass) — iterative testing
