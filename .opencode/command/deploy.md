---
description: Build Docker image and deploy llm-router to GCP Cloud Run.
agent: build
---

1. Build the Go binary for linux/amd64
2. Create a Dockerfile using gcr.io/distroless/static-debian12
3. Build and tag the Docker image
4. Push to Artifact Registry
5. Deploy to Cloud Run with env vars for OPENROUTER_API_KEY and OLLAMA_ENDPOINT

Args: $ARGUMENTS