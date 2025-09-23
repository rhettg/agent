package openairesponses

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
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
	includeReasoning          bool
	includeEncryptedReasoning bool
	includeReasoningSummary   bool
	store                     *bool
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

func WithMessageDeltaFunc(f MessageDeltaFunc) Option {
	return func(p *provider) {
		p.messageDeltaFunc = f
	}
}

func WithReasoning(enabled bool) Option {
	return func(p *provider) {
		p.includeReasoning = enabled
	}
}

func WithEncryptedReasoning(enabled bool) Option {
	return func(p *provider) {
		p.includeEncryptedReasoning = enabled
	}
}

func WithReasoningSummary(enabled bool) Option {
	return func(p *provider) {
		p.includeReasoningSummary = enabled
	}
}

func WithStore(enabled bool) Option {
	return func(p *provider) {
		p.store = &enabled
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
		
		// Default reasoning settings - encrypted for privacy
		includeEncryptedReasoning: true,
		store:                     &[]bool{false}[0], // Default to false for privacy
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
	
	// Convert tool definitions to function definitions
	var functions []ResponsesFunctionParam
	for _, tdf := range tdfs {
		desc := tdf.Description
		functions = append(functions, ResponsesFunctionParam{
			Name:        tdf.Name,
			Description: &desc,
			Parameters:  tdf.Parameters,
		})
	}
	
	// Build the request parameters
	params := ResponsesNewParams{
		Model: p.modelName,
		Input: inputItems,
	}
	
	// Set optional parameters
	if p.temperature != 0 {
		params.Temperature = &p.temperature
	}
	
	if p.maxTokens != 0 {
		maxTokens := int64(p.maxTokens)
		params.MaxOutputTokens = &maxTokens
	}
	
	if len(functions) > 0 {
		params.Functions = functions
	}
	
	// Set reasoning options
	if p.includeReasoning {
		params.IncludeReasoning = &[]bool{true}[0]
	}
	
	if p.includeEncryptedReasoning {
		params.IncludeEncryptedReasoning = &[]bool{true}[0]
	}
	
	if p.includeReasoningSummary {
		params.IncludeReasoningSummary = &[]bool{true}[0]
	}
	
	if p.store != nil {
		params.Store = p.store
	}
	
	// Handle streaming vs non-streaming
	if p.messageDeltaFunc != nil {
		return p.streamCompletion(ctx, params)
	} else {
		return p.nonStreamCompletion(ctx, params)
	}
}

func (p *provider) nonStreamCompletion(ctx context.Context, params ResponsesNewParams) (*agent.Message, error) {
	// Assemble the middleware chain
	completionFn := p.createCompletionFunc()
	for _, m := range p.mw {
		next := completionFn
		fm := m
		completionFn = func(ctx context.Context, params ResponsesNewParams, opts ...option.RequestOption) (*Response, error) {
			return fm(ctx, params, next)
		}
	}
	
	resp, err := completionFn(ctx, params)
	if err != nil {
		return nil, err
	}
	
	// Convert response output items to Agent message
	return p.mapOutputItemsToMessage(resp.Output)
}

func (p *provider) streamCompletion(ctx context.Context, params ResponsesNewParams) (*agent.Message, error) {
	// Enable streaming
	params.Stream = &[]bool{true}[0]
	
	// For now, fall back to non-streaming until streaming is fully implemented
	// TODO: Implement proper streaming when SDK supports it
	return p.nonStreamCompletion(ctx, params)
}

func (p *provider) createCompletionFunc() ResponsesCompletionFn {
	return func(ctx context.Context, params ResponsesNewParams, opts ...option.RequestOption) (*Response, error) {
		// NOTE: This is a placeholder implementation
		// The actual OpenAI Go SDK v2 may not have Responses API support yet
		// This would need to be updated when the SDK adds proper support
		
		// For now, return an error indicating the API is not yet available
		return nil, fmt.Errorf("OpenAI Responses API not yet available in Go SDK v2 - this is a placeholder implementation")
	}
}
