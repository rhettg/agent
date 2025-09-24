package openairesponses

import (
	"context"

	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/packages/ssestream"
	"github.com/openai/openai-go/v2/responses"
)

// ResponsesCompletionFn represents the function signature for making Responses API calls
type ResponsesCompletionFn func(context.Context, responses.ResponseNewParams, ...option.RequestOption) (*responses.Response, error)

// MiddlewareFunc represents middleware that can wrap the Responses API call
type MiddlewareFunc func(context.Context, responses.ResponseNewParams, ResponsesCompletionFn) (*responses.Response, error)

// StreamingCompletionFn represents the function signature for making streaming Responses API calls
type StreamingCompletionFn func(context.Context, responses.ResponseNewParams, ...option.RequestOption) *ssestream.Stream[responses.ResponseStreamEventUnion]

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
