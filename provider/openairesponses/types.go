package openairesponses

import (
	"context"

	"github.com/openai/openai-go/v2/option"
)

// ResponsesNewParams represents the parameters for a Responses API call
// This is a placeholder until the OpenAI Go SDK adds official support
type ResponsesNewParams struct {
	Model                     string        `json:"model"`
	Input                     []interface{} `json:"input"`
	Functions                 []ResponsesFunctionParam `json:"functions,omitempty"`
	Temperature               *float64      `json:"temperature,omitempty"`
	MaxOutputTokens           *int64        `json:"max_output_tokens,omitempty"`
	Stream                    *bool         `json:"stream,omitempty"`
	IncludeReasoning          *bool         `json:"include_reasoning,omitempty"`
	IncludeEncryptedReasoning *bool         `json:"include_encrypted_reasoning,omitempty"`
	IncludeReasoningSummary   *bool         `json:"include_reasoning_summary,omitempty"`
	Store                     *bool         `json:"store,omitempty"`
}

// ResponsesFunctionParam represents a function parameter for the Responses API
type ResponsesFunctionParam struct {
	Name        string      `json:"name"`
	Description *string     `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// Response represents a response from the Responses API
// This is a placeholder until the OpenAI Go SDK adds official support
type Response struct {
	ID     string        `json:"id"`
	Object string        `json:"object"`
	Model  string        `json:"model"`
	Output []interface{} `json:"output"`
	Usage  Usage         `json:"usage"`
}

// ResponsesCompletionFn represents the function signature for making Responses API calls
type ResponsesCompletionFn func(context.Context, ResponsesNewParams, ...option.RequestOption) (*Response, error)

// MiddlewareFunc represents middleware that can wrap the Responses API call
type MiddlewareFunc func(context.Context, ResponsesNewParams, ResponsesCompletionFn) (*Response, error)

// MessageDelta represents streaming updates from the Responses API
type MessageDelta struct {
	Role              string
	Content           string
	ToolCallID        string
	ToolCallName      string
	ToolCallArguments string
	
	// Reasoning support for streaming
	ReasoningContent  string `json:"reasoning_content,omitempty"`
	ReasoningSummary  string `json:"reasoning_summary,omitempty"`
}

// MessageDeltaFunc is called for each streaming delta
type MessageDeltaFunc func(ctx context.Context, delta MessageDelta)

// Usage represents token usage information
type Usage struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
}
