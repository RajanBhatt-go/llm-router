package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OpenRouterConfig struct {
	APIKey   string
	Model    string
	SiteURL  string
	SiteName string
	Timeout  time.Duration
}

type OpenRouterProvider struct {
	config OpenRouterConfig
	client *http.Client
}

func NewOpenRouterProvider(config OpenRouterConfig) *OpenRouterProvider {
	return &OpenRouterProvider{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

func (p *OpenRouterProvider) MaxContextWindow() int {
	return 128000
}

type openRouterRequestBody struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	ToolChoice  interface{}      `json:"tool_choice,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	Stop        []string         `json:"stop,omitempty"`
}

type openRouterChoice struct {
	Message Message `json:"message"`
	Index   int     `json:"index"`
}

type openRouterResponseBody struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Choices []openRouterChoice `json:"choices"`
	Usage   Usage              `json:"usage"`
}

func (p *OpenRouterProvider) Complete(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	body := openRouterRequestBody{
		Model:       p.config.Model,
		Messages:    req.Messages,
		Tools:       req.Tools,
		ToolChoice:  req.ToolChoice,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stop:        req.Stop,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openrouter marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://openrouter.ai/api/v1/chat/completions",
		bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("openrouter create req: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	httpReq.Header.Set("HTTP-Referer", p.config.SiteURL)
	httpReq.Header.Set("X-Title", p.config.SiteName)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openrouter do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openrouter status %d: %s", resp.StatusCode, string(respBody))
	}

	var orResp openRouterResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&orResp); err != nil {
		return nil, fmt.Errorf("openrouter decode: %w", err)
	}

	if len(orResp.Choices) == 0 {
		return nil, fmt.Errorf("openrouter: no choices")
	}

	return &LLMResponse{
		ID:       orResp.ID,
		Model:    orResp.Model,
		Message:  orResp.Choices[0].Message,
		Usage:    orResp.Usage,
		Provider: "openrouter",
	}, nil
}

func (p *OpenRouterProvider) HealthCheck(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://openrouter.ai/api/v1/auth/key", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}