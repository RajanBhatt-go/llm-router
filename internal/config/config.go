package config

import (
	"os"
	"time"

	"github.com/spf13/viper"
)

type ProviderConfig struct {
	APIKey   string        `mapstructure:"api_key"`
	Model    string        `mapstructure:"model"`
	SiteURL  string        `mapstructure:"site_url"`
	SiteName string        `mapstructure:"site_name"`
	Endpoint string        `mapstructure:"endpoint"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

type FallbackConfig struct {
	Order                  []string      `mapstructure:"order"`
	RetryCount             int           `mapstructure:"retry_count"`
	CircuitBreakerThreshold int         `mapstructure:"circuit_breaker_threshold"`
	CircuitBreakerReset    time.Duration `mapstructure:"circuit_breaker_reset"`
}

type TruncationConfig struct {
	Strategy      string `mapstructure:"strategy"`
	PreserveSystem bool   `mapstructure:"preserve_system"`
	PreserveLastN  int    `mapstructure:"preserve_last_n"`
}

type Config struct {
	OpenRouter ProviderConfig   `mapstructure:"openrouter"`
	Ollama     ProviderConfig   `mapstructure:"ollama"`
	Fallback   FallbackConfig   `mapstructure:"fallback"`
	Truncation TruncationConfig `mapstructure:"truncation"`
}

func DefaultConfig() Config {
	return Config{
		OpenRouter: ProviderConfig{
			Model:    "deepseek/deepseek-v4-flash",
			SiteURL:  "http://localhost:8080",
			SiteName: "llm-router",
			Timeout:  120 * time.Second,
			APIKey:   "",
		},
		Ollama: ProviderConfig{
			Model:    "llama3.1:8b-instruct-fp16",
			Endpoint: "http://localhost:11434",
			Timeout:  120 * time.Second,
		},
		Fallback: FallbackConfig{
			Order:                  []string{"openrouter", "ollama"},
			RetryCount:             2,
			CircuitBreakerThreshold: 5,
			CircuitBreakerReset:    60 * time.Second,
		},
		Truncation: TruncationConfig{
			Strategy:       "sliding",
			PreserveSystem:  true,
			PreserveLastN:   5,
		},
	}
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("llm-router")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.config/llm-router")
	v.AddConfigPath("/etc/llm-router")

	v.SetDefault("openrouter.model", "deepseek/deepseek-v4-flash")
	v.SetDefault("openrouter.site_url", "http://localhost:8080")
	v.SetDefault("openrouter.site_name", "llm-router")
	v.SetDefault("openrouter.timeout", "120s")
	v.SetDefault("ollama.model", "llama3.1:8b-instruct-fp16")
	v.SetDefault("ollama.endpoint", "http://localhost:11434")
	v.SetDefault("ollama.timeout", "120s")
	v.SetDefault("fallback.order", []string{"openrouter", "ollama"})
	v.SetDefault("fallback.retry_count", 2)
	v.SetDefault("fallback.circuit_breaker_threshold", 5)
	v.SetDefault("fallback.circuit_breaker_reset", "60s")
	v.SetDefault("truncation.strategy", "sliding")
	v.SetDefault("truncation.preserve_system", true)
	v.SetDefault("truncation.preserve_last_n", 5)

	v.AutomaticEnv()
	v.SetEnvPrefix("LLM")

	_ = v.ReadInConfig()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if cfg.OpenRouter.APIKey == "" {
		cfg.OpenRouter.APIKey = os.Getenv("OPENROUTER_API_KEY")
	}

	return &cfg, nil
}

func InitConfigFile(path string) error {
	cfg := DefaultConfig()
	v := viper.New()
	v.Set("openrouter.api_key", cfg.OpenRouter.APIKey)
	v.Set("openrouter.model", cfg.OpenRouter.Model)
	v.Set("openrouter.site_url", cfg.OpenRouter.SiteURL)
	v.Set("openrouter.site_name", cfg.OpenRouter.SiteName)
	v.Set("openrouter.timeout", cfg.OpenRouter.Timeout.String())
	v.Set("ollama.model", cfg.Ollama.Model)
	v.Set("ollama.endpoint", cfg.Ollama.Endpoint)
	v.Set("ollama.timeout", cfg.Ollama.Timeout.String())
	v.Set("fallback.order", cfg.Fallback.Order)
	v.Set("fallback.retry_count", cfg.Fallback.RetryCount)
	v.Set("fallback.circuit_breaker_threshold", cfg.Fallback.CircuitBreakerThreshold)
	v.Set("fallback.circuit_breaker_reset", cfg.Fallback.CircuitBreakerReset.String())
	v.Set("truncation.strategy", cfg.Truncation.Strategy)
	v.Set("truncation.preserve_system", cfg.Truncation.PreserveSystem)
	v.Set("truncation.preserve_last_n", cfg.Truncation.PreserveLastN)
	return v.WriteConfigAs(path)
}