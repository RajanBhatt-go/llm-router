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

type OllamaConfig struct {
	Endpoint string
	Model    string
	Timeout  time.Duration
}

type OllamaProvider struct {
	config OllamaConfig
	client *http.Client
}

func NewOllamaProvider(config OllamaConfig) *OllamaProvider {
	return &OllamaProvider{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

func (p *OllamaProvider) MaxContextWindow() int {
	return 8192
}

type ollamaRequestBody struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Stream      bool             `json:"stream"`
	Options     ollamaOptions    `json:"options,omitempty"`
}

type ollamaOptions struct {
	NumPredict  int      `json:"num_predict,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

type ollamaChoice struct {
	Message Message `json:"message"`
}

type ollamaResponseBody struct {
	Model              string         `json:"model"`
	CreatedAt          string         `json:"created_at"`
	Choices            []ollamaChoice `json:"choices"`
	Done               bool           `json:"done"`
	PromptEvalCount    int            `json:"prompt_eval_count,omitempty"`
	EvalCount          int            `json:"eval_count,omitempty"`
}

func (p *OllamaProvider) Complete(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	body := ollamaRequestBody{
		Model:    p.config.Model,
		Messages: req.Messages,
		Tools:    req.Tools,
		Stream:   false,
		Options: ollamaOptions{
			NumPredict:  req.MaxTokens,
			Temperature: req.Temperature,
			Stop:        req.Stop,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama marshal: %w", err)
	}

	url := p.config.Endpoint + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("ollama create req: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama status %d: %s", resp.StatusCode, string(respBody))
	}

	var olResp ollamaResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&olResp); err != nil {
		return nil, fmt.Errorf("ollama decode: %w", err)
	}

	if len(olResp.Choices) == 0 {
		return nil, fmt.Errorf("ollama: no choices")
	}

	usage := Usage{
		PromptTokens:     olResp.PromptEvalCount,
		CompletionTokens: olResp.EvalCount,
		TotalTokens:      olResp.PromptEvalCount + olResp.EvalCount,
	}

	return &LLMResponse{
		ID:       olResp.Model + "-" + olResp.CreatedAt,
		Model:    olResp.Model,
		Message:  olResp.Choices[0].Message,
		Usage:    usage,
		Provider: "ollama",
	}, nil
}

func (p *OllamaProvider) HealthCheck(ctx context.Context) bool {
	url := p.config.Endpoint + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}