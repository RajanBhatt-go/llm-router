package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"llm-router/internal/config"
	ctxmanager "llm-router/internal/context"
	"llm-router/internal/fallback"
	"llm-router/internal/metrics"
	"llm-router/internal/provider"
	"llm-router/internal/router"
	"llm-router/internal/tools"

	"github.com/spf13/cobra"
)

var serveFlags struct {
	port int
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server with OpenAI-compatible API",
	Long: `Starts an HTTP server exposing:
  POST /v1/chat/completions  - OpenAI-compatible chat endpoint
  GET  /metrics               - Prometheus metrics
  GET  /health                - Health check`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
		slog.SetDefault(logger)

		ctxMgr := ctxmanager.NewManager(ctxmanager.Config{
			Strategy:       ctxmanager.Strategy(cfg.Truncation.Strategy),
			PreserveSystem: cfg.Truncation.PreserveSystem,
			PreserveLastN:  cfg.Truncation.PreserveLastN,
		})

		providers := make(map[string]provider.Provider)

		if cfg.OpenRouter.APIKey != "" {
			providers["openrouter"] = provider.NewOpenRouterProvider(provider.OpenRouterConfig{
				APIKey:   cfg.OpenRouter.APIKey,
				Model:    cfg.OpenRouter.Model,
				SiteURL:  cfg.OpenRouter.SiteURL,
				SiteName: cfg.OpenRouter.SiteName,
				Timeout:  cfg.OpenRouter.Timeout,
			})
		}

		providers["ollama"] = provider.NewOllamaProvider(provider.OllamaConfig{
			Endpoint: cfg.Ollama.Endpoint,
			Model:    cfg.Ollama.Model,
			Timeout:  cfg.Ollama.Timeout,
		})

		orch := fallback.NewOrchestrator(fallback.Config{
			Order:                  cfg.Fallback.Order,
			RetryCount:             cfg.Fallback.RetryCount,
			CircuitBreakerThreshold: cfg.Fallback.CircuitBreakerThreshold,
			CircuitBreakerReset:    cfg.Fallback.CircuitBreakerReset,
		}, providers)

		var maxTokens int
		if p, ok := providers["openrouter"]; ok {
			maxTokens = p.MaxContextWindow()
		} else if p, ok := providers["ollama"]; ok {
			maxTokens = p.MaxContextWindow()
		} else {
			maxTokens = 8192
		}

		eng := router.NewEngine(ctxMgr, orch, tools.NewTranslator(), maxTokens)

		mux := http.NewServeMux()

		mux.Handle("/metrics", metrics.Handler())

		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			status := map[string]string{"status": "ok"}
			json.NewEncoder(w).Encode(status)
		})

		mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			var reqBody struct {
				Model       string                    `json:"model"`
				Messages    []provider.Message        `json:"messages"`
				Tools       []provider.ToolDefinition `json:"tools,omitempty"`
				ToolChoice  interface{}               `json:"tool_choice,omitempty"`
				MaxTokens   int                       `json:"max_tokens,omitempty"`
				Temperature float64                   `json:"temperature,omitempty"`
				Stop        []string                  `json:"stop,omitempty"`
			}

			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"bad request: %s"}`, err), http.StatusBadRequest)
				return
			}

			req := &provider.LLMRequest{
				Model:       reqBody.Model,
				Messages:    reqBody.Messages,
				Tools:       reqBody.Tools,
				ToolChoice:  reqBody.ToolChoice,
				MaxTokens:   reqBody.MaxTokens,
				Temperature: reqBody.Temperature,
				Stop:        reqBody.Stop,
			}

			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
			defer cancel()

			resp, err := eng.ExecuteWithFallbackTracking(ctx, req)
			if err != nil {
				slog.Error("request failed", "error", err)
				respErr := map[string]interface{}{
					"error": map[string]string{
						"message": err.Error(),
						"type":    "router_error",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(respErr)
				return
			}

			orResp := map[string]interface{}{
				"id":      resp.ID,
				"model":   resp.Model,
				"object":  "chat.completion",
				"provider": resp.Provider,
				"choices": []map[string]interface{}{
					{
						"index":    0,
						"message":  resp.Message,
						"finish_reason": "stop",
					},
				},
				"usage": resp.Usage,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(orResp)
		})

		addr := fmt.Sprintf(":%d", serveFlags.port)
		server := &http.Server{
			Addr:    addr,
			Handler: mux,
		}

		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			slog.Info("server starting", "addr", addr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("server error", "error", err)
				os.Exit(1)
			}
		}()

		<-stop
		slog.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	},
}

func init() {
	serveCmd.Flags().IntVarP(&serveFlags.port, "port", "p", 8080, "port to listen on")
	rootCmd.AddCommand(serveCmd)
}