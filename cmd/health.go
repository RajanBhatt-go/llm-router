package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"llm-router/internal/config"
	"llm-router/internal/provider"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check health of all configured providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		results := make(map[string]interface{})

		if cfg.OpenRouter.APIKey != "" {
			p := provider.NewOpenRouterProvider(provider.OpenRouterConfig{
				APIKey:   cfg.OpenRouter.APIKey,
				Model:    cfg.OpenRouter.Model,
				SiteURL:  cfg.OpenRouter.SiteURL,
				SiteName: cfg.OpenRouter.SiteName,
				Timeout:  10 * time.Second,
			})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			ok := p.HealthCheck(ctx)
			cancel()
			results["openrouter"] = map[string]interface{}{
				"healthy": ok,
				"model":   cfg.OpenRouter.Model,
			}
		}

		{
			p := provider.NewOllamaProvider(provider.OllamaConfig{
				Endpoint: cfg.Ollama.Endpoint,
				Model:    cfg.Ollama.Model,
				Timeout:  5 * time.Second,
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			ok := p.HealthCheck(ctx)
			cancel()
			results["ollama"] = map[string]interface{}{
				"healthy":  ok,
				"model":    cfg.Ollama.Model,
				"endpoint": cfg.Ollama.Endpoint,
			}
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}