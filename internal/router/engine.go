package router

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	ctxmanager "llm-router/internal/context"
	"llm-router/internal/fallback"
	"llm-router/internal/metrics"
	"llm-router/internal/provider"
	"llm-router/internal/tools"
)

type Engine struct {
	contextMgr   *ctxmanager.Manager
	orchestrator *fallback.Orchestrator
	toolTrans    *tools.Translator
	maxTokens    int
}

func NewEngine(
	ctxMgr *ctxmanager.Manager,
	orch *fallback.Orchestrator,
	toolTrans *tools.Translator,
	maxTokens int,
) *Engine {
	return &Engine{
		contextMgr:   ctxMgr,
		orchestrator: orch,
		toolTrans:    toolTrans,
		maxTokens:    maxTokens,
	}
}

func (e *Engine) Execute(ctx context.Context, req *provider.LLMRequest) (*provider.LLMResponse, error) {
	start := time.Now()
	slog.Info("processing request",
		"model", req.Model,
		"messages", len(req.Messages),
		"tools", len(req.Tools),
	)

	e.toolTrans.EnsureOpenAICompat(req)

	truncatedReq, tr, err := e.contextMgr.Truncate(req, e.maxTokens)
	if err != nil {
		metrics.RequestsTotal.WithLabelValues("router", "truncation_error").Inc()
		return nil, fmt.Errorf("truncation: %w", err)
	}

	if tr.Truncated {
		metrics.ContextTruncatedTotal.WithLabelValues(e.contextMgr.Strategy()).Inc()
		slog.Warn("context truncated", "original", tr.OriginalTokens, "new", tr.NewTokens)
	}

	req = truncatedReq

	resp, err := e.orchestrator.Execute(ctx, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		metrics.RequestsTotal.WithLabelValues("router", "error").Inc()
		return nil, fmt.Errorf("execution: %w", err)
	}

	metrics.RequestsTotal.WithLabelValues(resp.Provider, "success").Inc()
	metrics.RequestDuration.WithLabelValues(resp.Provider).Observe(duration)
	metrics.TokenCount.WithLabelValues("prompt").Observe(float64(resp.Usage.PromptTokens))
	metrics.TokenCount.WithLabelValues("completion").Observe(float64(resp.Usage.CompletionTokens))

	cb := e.orchestrator.CircuitBreaker()
	metrics.CircuitBreakerState.WithLabelValues("openrouter").Set(float64(cb.State()))

	if err := e.toolTrans.NormalizeToolCalls(resp); err != nil {
		slog.Warn("tool call normalization failed", "error", err)
	}

	slog.Info("request completed",
		"provider", resp.Provider,
		"model", resp.Model,
		"duration_seconds", duration,
		"prompt_tokens", resp.Usage.PromptTokens,
		"completion_tokens", resp.Usage.CompletionTokens,
	)

	return resp, nil
}

func (e *Engine) ExecuteWithFallbackTracking(ctx context.Context, req *provider.LLMRequest) (*provider.LLMResponse, error) {
	resp, err := e.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Provider != "openrouter" {
		metrics.FallbackTotal.WithLabelValues("openrouter", resp.Provider).Inc()
	}

	return resp, nil
}