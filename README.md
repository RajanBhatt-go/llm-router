# llm-router

Intelligent LLM request gateway with automatic fallback from cloud to local models.

```
Cloud (OpenRouter / DeepSeek V4 Flash) ── timeout/error ──> Local (Ollama)
```

## Features

- **Smart fallback** — Circuit breaker with exponential backoff; auto-fails over to Ollama when OpenRouter is down or rate-limited
- **Context window management** — Sliding-window truncation preserves system prompt + recent messages when context exceeds model limits
- **Tool calling** — Normalizes tool definitions across providers (OpenAI-compatible wire format)
- **Prometheus metrics** — Request rate, latency, fallback count, circuit breaker state, truncation events, token usage — all labeled by provider
- **CLI + HTTP server** — Single-shot prompts or OpenAI-compatible `/v1/chat/completions` endpoint
- **Single binary** — No runtime dependencies beyond the Go binary itself

## Install

```bash
# Build from source
go build -o llm-router .

# Or generate config first
llm-router config init
# Edit ~/.config/llm-router/llm-router.yaml
```

## Usage

### Run a prompt (CLI)

```bash
export OPENROUTER_API_KEY="sk-or-v1-..."

llm-router run "write a haiku about Go"
```

### Start HTTP server

```bash
llm-router serve --port 8080
```

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek/deepseek-v4-flash",
    "messages": [{"role": "user", "content": "hello"}]
  }'
```

### Health check

```bash
llm-router health
```

### Monitoring stack

```bash
make docker-up
# Grafana: http://localhost:3000 (admin/admin)
# Prometheus: http://localhost:9090
```

## Configuration

`~/.config/llm-router/llm-router.yaml`:

```yaml
openrouter:
  api_key: ""                    # or env OPENROUTER_API_KEY
  model: deepseek/deepseek-v4-flash
  site_url: http://localhost:8080
  site_name: llm-router
  timeout: 30s
ollama:
  model: llama3
  endpoint: http://localhost:11434
  timeout: 120s
fallback:
  order: [openrouter, ollama]
  retry_count: 2
  circuit_breaker_threshold: 5
  circuit_breaker_reset: 60s
truncation:
  strategy: sliding              # sliding | error
  preserve_system: true
  preserve_last_n: 5
```

## How It Works

```
                  ┌──────────────────────────────┐
Request ─────────▶│         Router Engine         │
                  │  ┌──────────┐  ┌───────────┐  │
                  │  │  Context  │─▶   Tool    │  │
                  │  │  Manager  │  │ Translator│  │
                  │  └────┬─────┘  └───────────┘  │
                  │       ▼                       │
                  │  ┌──────────────────────────┐  │
                  │  │   Fallback Orchestrator   │  │
                  │  │   Retry → Circuit → Next │  │
                  │  └──────────────────────────┘  │
                  └──────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    ▼                   ▼
              ┌──────────┐       ┌──────────┐
              │ OpenRouter│ ──▶  │  Ollama  │
              │ (Primary)│       │ (Local)  │
              └──────────┘       └──────────┘
```

## Architecture

| Package | Role |
|---|---|
| `cmd/` | Cobra CLI commands: `run`, `serve`, `health`, `config` |
| `internal/provider/` | Unified `Provider` interface + OpenRouter/Ollama adapters |
| `internal/fallback/` | Circuit breaker + retry orchestrator with ordered fallback |
| `internal/context/` | Sliding-window context truncation with token estimation |
| `internal/tools/` | Tool-call format normalizer (OpenAI-compatible) |
| `internal/metrics/` | Prometheus metrics: counters, histograms, gauges |
| `internal/config/` | Viper-based YAML + env var configuration |
| `internal/router/` | Engine tying truncation → fallback → metrics together |