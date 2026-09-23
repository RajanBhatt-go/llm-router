---
name: deploy-to-gcp
description: Use ONLY when deploying the llm-router Go application to Google Cloud Platform. Covers Docker build, Artifact Registry, Cloud Run, gcloud, and IAM.
---

# Deploy to GCP

## Prerequisites

- `gcloud` CLI installed and authenticated (`gcloud auth login`)
- Docker installed
- Project set: `gcloud config set project <project-id>`

## Build & Deploy Pipeline

### 1. Build Go binary

```bash
GOOS=linux GOARCH=amd64 go build -o llm-router .
```

### 2. Build & tag Docker image

```dockerfile
FROM gcr.io/distroless/static-debian12:nonroot
COPY llm-router /llm-router
EXPOSE 8080
ENTRYPOINT ["/llm-router", "serve"]
```

```bash
docker build -t llm-router .
docker tag llm-router us-central1-docker.pkg.dev/<project-id>/llm-router/llm-router:latest
```

### 3. Push to Artifact Registry

```bash
docker push us-central1-docker.pkg.dev/<project-id>/llm-router/llm-router:latest
```

### 4. Deploy to Cloud Run

```bash
gcloud run deploy llm-router \
  --image us-central1-docker.pkg.dev/<project-id>/llm-router/llm-router:latest \
  --region us-central1 \
  --cpu 1 \
  --memory 512Mi \
  --min-instances 0 \
  --max-instances 10 \
  --concurrency 80 \
  --timeout 120 \
  --set-env-vars "OPENROUTER_API_KEY=sk-or-v1-...,OLLAMA_ENDPOINT=http://ollama-service:11434" \
  --allow-unauthenticated
```

## Notes

- Cloud Run auto-scales to zero — no cost when idle
- Use `--update-env-vars` to change env vars on an existing service
- Ollama must run as a separate Cloud Run service or GKE if needed as fallback
- Metrics: Prometheus/Grafana can be deployed as separate Cloud Run services or on GKE