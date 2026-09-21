package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"llm-router/internal/config"
	ctxmanager "llm-router/internal/context"
	"llm-router/internal/fallback"
	"llm-router/internal/provider"
	"llm-router/internal/router"
	"llm-router/internal/tools"

	"github.com/spf13/cobra"
)

var runFlags struct {
	model       string
	provider    string
	temperature float64
	maxTokens   int
}

var runCmd = &cobra.Command{
	Use:   "run [prompt]",
	Short: "Send a prompt to the LLM router",
	Long: `Execute a single LLM request. Reads the prompt from args or stdin.
Tries OpenRouter first, falls back to Ollama on failure.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

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

		prompt := ""
		if len(args) > 0 {
			prompt = args[0]
		} else {
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				data, _ := os.ReadFile("/dev/stdin")
				prompt = string(data)
			}
		}

		if prompt == "" {
			return fmt.Errorf("prompt required (argument or stdin)")
		}

		req := &provider.LLMRequest{
			Model:    cfg.OpenRouter.Model,
			Messages: []provider.Message{{Role: provider.RoleUser, Content: prompt}},
			MaxTokens:    runFlags.maxTokens,
			Temperature:  runFlags.temperature,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		resp, err := eng.ExecuteWithFallbackTracking(ctx, req)
		if err != nil {
			return fmt.Errorf("router: %w", err)
		}

		output := struct {
			Provider string            `json:"provider"`
			Model    string            `json:"model"`
			Content  string            `json:"content"`
			Usage    provider.Usage    `json:"usage"`
			ToolCalls []provider.ToolCall `json:"tool_calls,omitempty"`
		}{
			Provider:  resp.Provider,
			Model:     resp.Model,
			Content:   resp.Message.Content,
			Usage:     resp.Usage,
			ToolCalls: resp.Message.ToolCalls,
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	},
}

func init() {
	runCmd.Flags().StringVar(&runFlags.model, "model", "", "model name override")
	runCmd.Flags().StringVar(&runFlags.provider, "provider", "", "force a specific provider")
	runCmd.Flags().Float64Var(&runFlags.temperature, "temperature", 0.7, "response temperature")
	runCmd.Flags().IntVar(&runFlags.maxTokens, "max-tokens", 4096, "max completion tokens")
	rootCmd.AddCommand(runCmd)
}