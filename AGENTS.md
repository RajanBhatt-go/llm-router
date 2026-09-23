# llm-router — Agent Guide

## Project

LLM request gateway with OpenRouter (primary) → Ollama (fallback). Deployed on Cloud Run.

## Quick Commands

```bash
# Build binary
GOOS=linux GOARCH=amd64 go build -o llm-router .

# Build & push Docker image (Artifact Registry)
docker build -t llm-router .
docker tag llm-router us-central1-docker.pkg.dev/rajan-llm-gaeway/rajancentralrepo/llm-router:latest
docker push us-central1-docker.pkg.dev/rajan-llm-gaeway/rajancentralrepo/llm-router:latest

# Deploy router to Cloud Run
gcloud run deploy llm-router \
  --image us-central1-docker.pkg.dev/rajan-llm-gaeway/rajancentralrepo/llm-router:latest \
  --region us-central1 --cpu 1 --memory 512Mi \
  --min-instances 0 --max-instances 10 --concurrency 80 --timeout 120 \
  --update-env-vars "OPENROUTER_API_KEY=sk-or-v1-...,LLM_OLLAMA_ENDPOINT=https://ollama-....run.app,LLM_OLLAMA_MODEL=llama3.2:3b" \
  --allow-unauthenticated

# Deploy Ollama to Cloud Run
gcloud run deploy ollama \
  --image ollama/ollama:latest --region us-central1 \
  --cpu 2 --memory 8Gi --min-instances 0 --max-instances 1 \
  --concurrency 1 --timeout 300 \
  --set-env-vars "OLLAMA_HOST=0.0.0.0:8080,OLLAMA_MODELS=/tmp/ollama/models" \
  --allow-unauthenticated

# Pull model on Ollama
curl -X POST https://ollama-751353592714.us-central1.run.app/api/pull -d '{"name": "llama3.2:3b"}'
```

## GCP Context

| Item | Value |
|---|---|
| Project ID | `rajan-llm-gaeway` |
| Region | `us-central1` |
| Artifact Registry | `rajancentralrepo` |
| Router URL | `https://llm-router-751353592714.us-central1.run.app` |
| Ollama URL | `https://ollama-751353592714.us-central1.run.app` |

## Testing

```bash
# Health check
curl https://llm-router-751353592714.us-central1.run.app/health

# Chat (OpenRouter path)
curl -s https://llm-router-751353592714.us-central1.run.app/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}'

# Direct Ollama
curl -s https://ollama-751353592714.us-central1.run.app/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"llama3.2:3b","messages":[{"role":"user","content":"Hi"}],"stream":false}'
```

## Env Vars

| Variable | Purpose |
|---|---|
| `OPENROUTER_API_KEY` | OpenRouter API key |
| `LLM_OLLAMA_ENDPOINT` | Ollama Cloud Run URL |
| `LLM_OLLAMA_MODEL` | Ollama model (e.g. `llama3.2:3b`) |
| `LLM_OPENROUTER_MODEL` | OpenRouter model override |

## Architecture

Two separate Cloud Run services:
- `llm-router` — receives requests, tries OpenRouter first, falls back to Ollama
- `ollama` — runs `ollama/ollama` with `llama3.2:3b`, pulled lazily at first request

## Git Conventions

- Branch: `master`
- Commit style: descriptive one-line messages
- Key files: `Dockerfile` (router), `Dockerfile.ollama`, `README.md`