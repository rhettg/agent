package agent

import (
	"context"
	"encoding/json"

	"github.com/tiktoken-go/tokenizer"
)

func EstimateTokens(ctx context.Context, t tokenizer.Codec, m *Message) (int, error) {
	type functionCall struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}

	content, err := m.Content(ctx)
	if err != nil {
		return 0, err
	}

	cm := struct {
		Role         string        `json:"role"`
		Name         string        `json:"name"`
		Content      string        `json:"content"`
		FunctionCall *functionCall `json:"functionCall,omitempty"`
	}{
		Role:    string(m.Role),
		Name:    m.Name,
		Content: content,
	}

	// Handle tool calls for token counting
	if len(m.ToolCalls) > 0 {
		// Use the first tool call for legacy compatibility in token counting
		cm.FunctionCall = &functionCall{
			Name:      m.ToolCalls[0].Name,
			Arguments: m.ToolCalls[0].Arguments,
		}
	}

	data, err := json.Marshal(m)
	if err != nil {
		return 0, err
	}

	tokens, _, err := t.Encode(string(data))
	if err != nil {
		return 0, err
	}

	return len(tokens), nil
}
