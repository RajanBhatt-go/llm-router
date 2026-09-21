package context

import (
	"fmt"
	"log/slog"

	"llm-router/internal/provider"
)

type Strategy string

const (
	StrategySliding Strategy = "sliding"
	StrategyError   Strategy = "error"
)

type Config struct {
	Strategy      Strategy `mapstructure:"strategy"`
	PreserveSystem bool    `mapstructure:"preserve_system"`
	PreserveLastN  int     `mapstructure:"preserve_last_n"`
}

type Manager struct {
	config Config
}

func NewManager(config Config) *Manager {
	return &Manager{config: config}
}

func (m *Manager) Strategy() string {
	return string(m.config.Strategy)
}

type TruncationResult struct {
	OriginalTokens  int
	NewTokens       int
	Truncated       bool
	MessagesDropped int
}

func EstimateTokens(messages []provider.Message) int {
	total := 0
	for _, msg := range messages {
		total += len(msg.Content) / 4
		if len(msg.Content) > 0 {
			total += 3
		}
	}
	return total
}

func (m *Manager) Truncate(req *provider.LLMRequest, maxTokens int) (*provider.LLMRequest, *TruncationResult, error) {
	total := EstimateTokens(req.Messages)

	res := &TruncationResult{OriginalTokens: total}

	if total <= maxTokens {
		return req, res, nil
	}

	switch m.config.Strategy {
	case StrategySliding:
		res.Truncated = true
		return m.slidingWindow(req, maxTokens, res)
	case StrategyError:
		return nil, res, fmt.Errorf("context too long: %d tokens exceeds max %d", total, maxTokens)
	default:
		return nil, res, fmt.Errorf("unknown truncation strategy: %s", m.config.Strategy)
	}
}

func (m *Manager) slidingWindow(req *provider.LLMRequest, maxTokens int, res *TruncationResult) (*provider.LLMRequest, *TruncationResult, error) {
	msgs := req.Messages

	var systemMsg *provider.Message
	nonSystem := make([]provider.Message, 0, len(msgs))
	for _, msg := range msgs {
		if msg.Role == provider.RoleSystem && m.config.PreserveSystem {
			sys := msg
			systemMsg = &sys
		} else {
			nonSystem = append(nonSystem, msg)
		}
	}

	if m.config.PreserveLastN > 0 && len(nonSystem) > m.config.PreserveLastN {
		keepEnd := nonSystem[len(nonSystem)-m.config.PreserveLastN:]
		var keepStart []provider.Message
		if len(nonSystem) > 1 {
			keepStart = nonSystem[0:1]
		}
		nonSystem = append(keepStart, keepEnd...)
	}

	newMsgs := make([]provider.Message, 0, len(msgs))
	if systemMsg != nil {
		newMsgs = append(newMsgs, *systemMsg)
	}
	newMsgs = append(newMsgs, nonSystem...)

	minMessages := 1
	if systemMsg != nil {
		minMessages = 2
	}

	for EstimateTokens(newMsgs) > maxTokens && len(newMsgs) > minMessages {
		startIdx := 0
		if systemMsg != nil {
			startIdx = 1
		}
		if len(newMsgs)-startIdx > 1 {
			mid := startIdx + (len(newMsgs)-startIdx)/2
			newMsgs = append(newMsgs[:mid], newMsgs[mid+1:]...)
			res.MessagesDropped++
		} else {
			break
		}
	}

	res.NewTokens = EstimateTokens(newMsgs)
	req.Messages = newMsgs

	slog.Debug("context truncated",
		"original_tokens", res.OriginalTokens,
		"new_tokens", res.NewTokens,
		"messages_dropped", res.MessagesDropped,
	)

	return req, res, nil
}