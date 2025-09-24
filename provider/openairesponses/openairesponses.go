package openairesponses

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/packages/ssestream"
	"github.com/openai/openai-go/v2/responses"
	"github.com/openai/openai-go/v2/shared"
	"github.com/rhettg/agent"
)

const defaultTemperature = float64(1.0)

type provider struct {
	client           openai.Client
	temperature      float64
	maxTokens        int
	mw               []MiddlewareFunc
	modelName        string
	messageDeltaFunc MessageDeltaFunc
	
	// Responses API specific options
	reasoningEffort  shared.ReasoningEffort
	reasoningSummary shared.ReasoningSummary
}

type Option func(p *provider)

func WithMiddleware(m MiddlewareFunc) Option {
	return func(p *provider) {
		p.mw = append(p.mw, m)
	}
}

func WithTemperature(t float64) Option {
	return func(p *provider) {
		p.temperature = t
	}
}

func WithMaxTokens(m int) Option {
	return func(p *provider) {
		p.maxTokens = m
	}
}

// WithMessageDeltaFunc sets a callback for streaming message deltas.
func WithMessageDeltaFunc(f MessageDeltaFunc) Option {
	return func(p *provider) {
		p.messageDeltaFunc = f
	}
}

// WithReasoningEffort sets the reasoning effort level for reasoning models.
// Supported values: "minimal", "low", "medium", "high".
// Reducing reasoning effort can result in faster responses and fewer tokens used on reasoning.
func WithReasoningEffort(effort shared.ReasoningEffort) Option {
	return func(p *provider) {
		p.reasoningEffort = effort
	}
}

// WithReasoningSummary sets the reasoning summary level for reasoning models.
// Supported values: "auto", "concise", "detailed".
// This provides a summary of the reasoning performed by the model.
func WithReasoningSummary(summary shared.ReasoningSummary) Option {
	return func(p *provider) {
		p.reasoningSummary = summary
	}
}

func New(apiKey string, modelName string, opts ...Option) agent.CompletionFunc {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return NewWithClient(client, modelName, opts...)
}

func NewWithClient(client openai.Client, modelName string, opts ...Option) agent.CompletionFunc {
	p := &provider{
		client:      client,
		modelName:   modelName,
		temperature: defaultTemperature,
		
		// Default reasoning settings - use model defaults when not specified
		reasoningEffort:  "", // Empty means use model default
		reasoningSummary: "", // Empty means use model default
	}

	for _, o := range opts {
		o(p)
	}

	return p.Completion
}

func (p *provider) Completion(
	ctx context.Context, msgs []*agent.Message, tdfs []agent.ToolDef,
) (*agent.Message, error) {
	// Convert messages to input items for Responses API
	inputItems, err := p.mapMessagesToInputItems(ctx, msgs)
	if err != nil {
		return nil, fmt.Errorf("failed to map messages to input items: %w", err)
	}
	
	// Convert tool definitions to function tools
	var tools []responses.ToolUnionParam
	for _, tdf := range tdfs {
		tools = append(tools, responses.ToolParamOfFunction(
			tdf.Name,
			tdf.Parameters.(map[string]any),
			false, // strict mode
		))
	}
	
	// Build the request parameters
	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(p.modelName),
		Input: inputItems,
	}
	
	// Set optional parameters
	if p.temperature != 0 {
		params.Temperature = openai.Float(p.temperature)
	}
	
	if p.maxTokens != 0 {
		params.MaxOutputTokens = openai.Int(int64(p.maxTokens))
	}
	
	if len(tools) > 0 {
		params.Tools = tools
	}
	
	// Set reasoning options if specified
	if p.reasoningEffort != "" || p.reasoningSummary != "" {
		reasoning := shared.ReasoningParam{}
		if p.reasoningEffort != "" {
			reasoning.Effort = p.reasoningEffort
		}
		if p.reasoningSummary != "" {
			reasoning.Summary = p.reasoningSummary
		}
		params.Reasoning = reasoning
	}
	
	// Handle streaming vs non-streaming
	if p.messageDeltaFunc != nil {
		return p.streamCompletion(ctx, params)
	} else {
		return p.nonStreamCompletion(ctx, params)
	}
}

func (p *provider) nonStreamCompletion(ctx context.Context, params responses.ResponseNewParams) (*agent.Message, error) {
	// Assemble the middleware chain
	completionFn := p.createCompletionFunc()
	for _, m := range p.mw {
		next := completionFn
		fm := m
		completionFn = func(ctx context.Context, params responses.ResponseNewParams, opts ...option.RequestOption) (*responses.Response, error) {
			return fm(ctx, params, next)
		}
	}
	
	resp, err := completionFn(ctx, params)
	if err != nil {
		return nil, err
	}
	
	// Convert response to Agent message
	return p.mapResponseToMessage(resp)
}

func (p *provider) streamCompletion(ctx context.Context, params responses.ResponseNewParams) (*agent.Message, error) {
	// Create streaming completion function
	streamingFn := p.createStreamingCompletionFunc()
	
	// Get the stream
	stream := streamingFn(ctx, params)
	defer stream.Close()
	
	// Accumulate the response
	var finalResponse *responses.Response
	
	for stream.Next() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		
		event := stream.Current()
		
		// Call the delta function if provided
		if p.messageDeltaFunc != nil {
			delta := p.extractDeltaFromEvent(event)
			if delta != nil {
				p.messageDeltaFunc(ctx, *delta)
			}
		}
		
		// Check if this is the completion event
		if completedEvent := event.AsResponseCompleted(); completedEvent.Type != "" {
			finalResponse = &completedEvent.Response
			break
		}
	}
	
	if err := stream.Err(); err != nil {
		return nil, err
	}
	
	if finalResponse == nil {
		return nil, fmt.Errorf("no final response received from stream")
	}
	
	// Convert response to Agent message
	return p.mapResponseToMessage(finalResponse)
}

func (p *provider) createCompletionFunc() ResponsesCompletionFn {
	return func(ctx context.Context, params responses.ResponseNewParams, opts ...option.RequestOption) (*responses.Response, error) {
		return p.client.Responses.New(ctx, params, opts...)
	}
}

func (p *provider) createStreamingCompletionFunc() StreamingCompletionFn {
	return func(ctx context.Context, params responses.ResponseNewParams, opts ...option.RequestOption) *ssestream.Stream[responses.ResponseStreamEventUnion] {
		return p.client.Responses.NewStreaming(ctx, params, opts...)
	}
}

// extractDeltaFromEvent extracts delta information from streaming events
func (p *provider) extractDeltaFromEvent(event responses.ResponseStreamEventUnion) *MessageDelta {
	// Handle text deltas
	if textDelta := event.AsResponseOutputTextDelta(); textDelta.Type != "" {
		return &MessageDelta{
			Role:    "assistant",
			Content: textDelta.Delta,
		}
	}
	
	// Handle function call argument deltas
	if funcDelta := event.AsResponseFunctionCallArgumentsDelta(); funcDelta.Type != "" {
		return &MessageDelta{
			Role:              "assistant",
			ToolCallID:        funcDelta.ItemID,
			ToolCallArguments: funcDelta.Delta,
		}
	}
	
	// Handle reasoning text deltas
	if reasoningDelta := event.AsResponseReasoningTextDelta(); reasoningDelta.Type != "" {
		return &MessageDelta{
			Role:             "assistant",
			ReasoningContent: reasoningDelta.Delta,
		}
	}
	
	// Handle reasoning summary deltas
	if reasoningSummaryDelta := event.AsResponseReasoningSummaryTextDelta(); reasoningSummaryDelta.Type != "" {
		return &MessageDelta{
			Role:             "assistant",
			ReasoningSummary: reasoningSummaryDelta.Delta,
		}
	}
	
	return nil
}
