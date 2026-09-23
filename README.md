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

## Local Usage

### Prerequisites

```bash
# Build from source
go build -o llm-router .

# Set your OpenRouter API key
export OPENROUTER_API_KEY="sk-or-v1-..."
```

### Run a prompt (CLI)

```bash
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
curl http://localhost:8080/health
```

## GCP Deployment

The router deploys as two separate Cloud Run services:

```
┌──────────────┐     ┌─────────────────────────────────────┐
│   User/CLI   │     │         Google Cloud (us-central1)   │
│   (curl)     │     │                                     │
└──────┬───────┘     │  ┌───────────────────────────────┐  │
       │             │  │      llm-router Cloud Run      │  │
       │ POST /v1/   │  │  ┌──────────┐  ┌──────────┐  │  │
       │ chat/comp.  │  │  │OpenRouter│  │  Ollama  │  │  │
       ├─────────────┼──┤  │ Provider │  │ Provider │  │  │
       │             │  │  │(primary) │  │(fallback)│  │  │
       │             │  │  └────┬─────┘  └─────┬────┘  │  │
       │             │  └───────┼──────────────┼────────┘  │
       │             │          │              │           │
       │             │          │              │           │
       │             │   ┌──────┴──────┐  ┌────┴──────┐   │
       │             │   │  OpenRouter │  │  Ollama    │   │
       │             │   │  API        │  │  Cloud Run │   │
       │             │   │  (external) │  │  :8080     │   │
       │             │   └─────────────┘  └────────────┘   │
       │             └─────────────────────────────────────┘
```

| Service | URL | Image | CPU | Memory |
|---|---|---|---|---|
| `llm-router` | `https://llm-router-751353592714.us-central1.run.app` | Custom (Artifact Registry) | 1 vCPU | 512Mi |
| `ollama` | `https://ollama-751353592714.us-central1.run.app` | `ollama/ollama:latest` | 2 vCPU | 8Gi |

### Deploy router

```bash
# Build binary and Docker image
GOOS=linux GOARCH=amd64 go build -o llm-router .
docker build -t llm-router .
docker tag llm-router us-central1-docker.pkg.dev/<project>/<repo>/llm-router:latest

# Push to Artifact Registry
docker push us-central1-docker.pkg.dev/<project>/<repo>/llm-router:latest

# Deploy
gcloud run deploy llm-router \
  --image us-central1-docker.pkg.dev/<project>/<repo>/llm-router:latest \
  --region us-central1 \
  --cpu 1 --memory 512Mi \
  --min-instances 0 --max-instances 10 \
  --concurrency 80 --timeout 120 \
  --update-env-vars "OPENROUTER_API_KEY=sk-or-v1-...,LLM_OLLAMA_ENDPOINT=https://ollama-....run.app,LLM_OLLAMA_MODEL=llama3.2:3b" \
  --allow-unauthenticated
```

### Deploy Ollama

```bash
gcloud run deploy ollama \
  --image ollama/ollama:latest \
  --region us-central1 \
  --cpu 2 --memory 8Gi \
  --min-instances 0 --max-instances 1 \
  --concurrency 1 --timeout 300 \
  --set-env-vars "OLLAMA_HOST=0.0.0.0:8080,OLLAMA_MODELS=/tmp/ollama/models" \
  --allow-unauthenticated
```

Pull the model after deployment:

```bash
curl -X POST https://ollama-....run.app/api/pull -d '{"name": "llama3.2:3b"}'
```

## Testing

### Test OpenRouter path (primary)

```bash
curl -s https://llm-router-751353592714.us-central1.run.app/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}'
```

Expected response (model may vary):

```json
{"choices":[{"message":{"content":"Hello! ..."}}],"provider":"openrouter"}
```

### Test Ollama directly

```bash
curl -s https://ollama-751353592714.us-central1.run.app/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"llama3.2:3b","messages":[{"role":"user","content":"Hi"}],"stream":false}'
```

### Health check

```bash
curl https://llm-router-751353592714.us-central1.run.app/health
# {"status":"ok"}
```

### Metrics

```bash
curl https://llm-router-751353592714.us-central1.run.app/metrics
```

## Configuration

`~/.config/llm-router/llm-router.yaml`:

```yaml
openrouter:
  api_key: ""                    # or env OPENROUTER_API_KEY
  model: deepseek/deepseek-v4-flash
  site_url: http://localhost:8080
  site_name: llm-router
  timeout: 120s
ollama:
  model: llama3.2:3b
  endpoint: http://localhost:11434
  timeout: 120s
fallback:
  order: [openrouter, ollama]
  retry_count: 2
  circuit_breaker_threshold: 5
  circuit_breaker_reset: 60s
truncation:
  strategy: sliding
  preserve_system: true
  preserve_last_n: 5
```

Environment variable overrides (prefix `LLM_`):

| Variable | Maps to |
|---|---|
| `OPENROUTER_API_KEY` | `openrouter.api_key` |
| `LLM_OPENROUTER_MODEL` | `openrouter.model` |
| `LLM_OLLAMA_ENDPOINT` | `ollama.endpoint` |
| `LLM_OLLAMA_MODEL` | `ollama.model` |
| `LLM_FALLBACK_ORDER` | `fallback.order` |

## How It Works

```
                  ┌──────────────────────────────┐
Request ────────▶│         Router Engine         │
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

1. Request arrives at `/v1/chat/completions`
2. Router Engine applies context truncation and tool normalization
3. Fallback Orchestrator tries OpenRouter first
4. If OpenRouter fails (timeout, 5xx, circuit breaker open), falls back to Ollama
5. Response returned to caller with provider name in metadata

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