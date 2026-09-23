---
name: go-dev
description: Use when writing, reviewing, or debugging Go code in the llm-router project. Covers conventions, project structure, and common patterns.
---

# Go Development

## Project Conventions

- **Module**: `llm-router`
- **Style**: Standard Go conventions — `gofmt`, `go vet`, no unused imports/vars
- **Error handling**: Always check errors. Wrap with `fmt.Errorf("context: %w", err)` not `errors.Wrap`
- **Logging**: Use `log/slog` (stdlib). Structured attributes, not formatted strings
- **Testing**: `go test ./...` — test files next to their packages

## Project Layout

```
llm-router/
├── cmd/              # Cobra command definitions
├── internal/         # Private packages
│   ├── provider/     # Provider interface + adapters
│   ├── fallback/     # Circuit breaker + orchestrator
│   ├── context/      # Context window truncation
│   ├── tools/        # Tool-call normalization
│   ├── metrics/      # Prometheus metrics
│   ├── config/       # Viper config
│   └── router/       # Router engine
├── deploy/           # Docker Compose, Prometheus, Grafana
└── main.go
```

## Common Commands

```bash
go build -o llm-router .   # Build binary
go test ./...               # Run all tests
go vet ./...                # Static analysis
go mod tidy                 # Clean dependencies
```