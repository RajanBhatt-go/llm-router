package tools

import (
	"fmt"

	"llm-router/internal/provider"
)

type Translator struct{}

func NewTranslator() *Translator {
	return &Translator{}
}

func (t *Translator) NormalizeToolCalls(response *provider.LLMResponse) error {
	for i := range response.Message.ToolCalls {
		tc := &response.Message.ToolCalls[i]
		if tc.Type == "" {
			tc.Type = "function"
		}
		if tc.ID == "" {
			return fmt.Errorf("tool call missing id")
		}
		if tc.Function.Name == "" {
			return fmt.Errorf("tool call missing function name")
		}
	}
	return nil
}

func (t *Translator) ValidateToolDefs(defs []provider.ToolDefinition) error {
	for _, d := range defs {
		if d.Type == "" {
			d.Type = "function"
		}
		if d.Function.Name == "" {
			return fmt.Errorf("tool definition missing function name")
		}
	}
	return nil
}

func (t *Translator) EnsureOpenAICompat(req *provider.LLMRequest) {
	for i, d := range req.Tools {
		if d.Type == "" {
			req.Tools[i].Type = "function"
		}
	}
}