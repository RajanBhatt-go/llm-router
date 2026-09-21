package fallback

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"llm-router/internal/provider"
)

type Config struct {
	Order                 []string      `mapstructure:"order"`
	RetryCount            int           `mapstructure:"retry_count"`
	CircuitBreakerThreshold int         `mapstructure:"circuit_breaker_threshold"`
	CircuitBreakerReset   time.Duration `mapstructure:"circuit_breaker_reset"`
}

type Orchestrator struct {
	config         Config
	providers      map[string]provider.Provider
	circuitBreaker *CircuitBreaker
}

func NewOrchestrator(config Config, providers map[string]provider.Provider) *Orchestrator {
	return &Orchestrator{
		config:         config,
		providers:      providers,
		circuitBreaker: NewCircuitBreaker("openrouter", config.CircuitBreakerThreshold, config.CircuitBreakerReset),
	}
}

func (o *Orchestrator) CircuitBreaker() *CircuitBreaker {
	return o.circuitBreaker
}

func (o *Orchestrator) Execute(ctx context.Context, req *provider.LLMRequest) (*provider.LLMResponse, error) {
	for _, name := range o.config.Order {
		prov, ok := o.providers[name]
		if !ok {
			slog.Warn("provider not configured", "provider", name)
			continue
		}

		if name == "openrouter" {
			if !o.circuitBreaker.Allow() {
				slog.Warn("circuit breaker open, skipping", "provider", name)
				continue
			}
		}

		for attempt := 0; attempt <= o.config.RetryCount; attempt++ {
			if attempt > 0 {
				backoff := time.Duration(1<<uint(attempt)) * time.Second
				slog.Debug("retrying", "provider", name, "attempt", attempt, "backoff", backoff)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
				}
			}

			resp, err := prov.Complete(ctx, req)
			if err == nil {
				if name == "openrouter" {
					o.circuitBreaker.Success()
				}
				return resp, nil
			}

			slog.Warn("provider failed",
				"provider", name,
				"attempt", attempt,
				"error", err,
			)

			if name == "openrouter" {
				o.circuitBreaker.Failure()
			}
		}
	}

	return nil, fmt.Errorf("all providers exhausted")
}