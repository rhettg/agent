package openairesponses

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/responses"
	"github.com/openai/openai-go/v2/shared"
	"github.com/rhettg/agent"
)

const defaultTemperature = float64(1.0)

type provider struct {
	client           openai.Client
	temperature      *float64 // Use pointer to distinguish between unset and 0
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
		p.temperature = &t
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
func WithReasoningEffort(effort string) Option {
	return func(p *provider) {
		p.reasoningEffort = shared.ReasoningEffort(effort)
	}
}

// WithReasoningSummary sets the reasoning summary level for reasoning models.
// Supported values: "auto", "concise", "detailed".
// This provides a summary of the reasoning performed by the model.
func WithReasoningSummary(summary string) Option {
	return func(p *provider) {
		p.reasoningSummary = shared.ReasoningSummary(summary)
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
		temperature: nil, // nil means use model default

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
	inputItems, err := p.mapMessagesToInputItems(ctx, msgs)
	if err != nil {
		return nil, fmt.Errorf("failed to map messages to input items: %w", err)
	}

	var tools []responses.ToolUnionParam
	for _, tdf := range tdfs {
		params, ok := tdf.Parameters.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("tool %s has invalid parameters type: expected map[string]any, got %T", tdf.Name, tdf.Parameters)
		}
		tools = append(tools, responses.ToolParamOfFunction(
			tdf.Name,
			params,
			false, // strict mode
		))
	}

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(p.modelName),
		Input: inputItems,
	}

	if p.temperature != nil {
		params.Temperature = openai.Float(*p.temperature)
	}

	if p.maxTokens != 0 {
		params.MaxOutputTokens = openai.Int(int64(p.maxTokens))
	}

	if len(tools) > 0 {
		params.Tools = tools
	}

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

	completionFn := p.createCompletionFunc()

	resp, err := completionFn(ctx, params)
	if err != nil {
		return nil, err
	}

	return p.mapResponseToMessage(resp)
}

// doStreaming performs streaming internally but returns the final response
// This allows middleware to be applied while still maintaining streaming behavior
func (p *provider) doStreaming(ctx context.Context, params responses.ResponseNewParams, opts ...option.RequestOption) (*responses.Response, error) {
	stream := p.client.Responses.NewStreaming(ctx, params, opts...)
	defer stream.Close()

	// Accumulate the response
	var finalResponse *responses.Response
	var item responses.ResponseOutputItemUnion

	for stream.Next() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		event := stream.Current()

		switch event.Type {
		case "response.output_item.added":
			item = event.AsResponseOutputItemAdded().Item
		case "response.output_item.done":
			item = responses.ResponseOutputItemUnion{}
		}

		delta := p.extractDeltaFromEvent(item, event)

		if delta != nil && p.messageDeltaFunc != nil {
			p.messageDeltaFunc(ctx, *delta)
		}

		// The fully constructed response comes as an event that we want to
		// collect as the return value.
		if completedEvent := event.AsResponseCompleted(); completedEvent.Type == "response.completed" {
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

	return finalResponse, nil
}

func (p *provider) createCompletionFunc() ResponsesCompletionFn {
	var completionFn ResponsesCompletionFn
	if p.messageDeltaFunc != nil {
		completionFn = p.doStreaming
	} else {
		completionFn = p.client.Responses.New
	}

	for _, m := range p.mw {
		next := completionFn
		fm := m
		completionFn = func(ctx context.Context, params responses.ResponseNewParams, opts ...option.RequestOption) (*responses.Response, error) {
			return fm(ctx, params, next)
		}
	}

	return completionFn
}

// extractDeltaFromEvent extracts delta information from streaming events
func (p *provider) extractDeltaFromEvent(item responses.ResponseOutputItemUnion, event responses.ResponseStreamEventUnion) *MessageDelta {
	switch event.Type {
	case "response.output_text.delta":
		textDelta := event.AsResponseOutputTextDelta()
		return &MessageDelta{
			Role:    "assistant",
			Content: textDelta.Delta,
		}
	case "response.output_text.done":
		return &MessageDelta{
			Role: "assistant",
		}
	case "response.reasoning_text.delta":
		reasoningDelta := event.AsResponseReasoningTextDelta()
		return &MessageDelta{
			Role:             "assistant",
			ReasoningContent: reasoningDelta.Delta,
		}
	case "response.reasoning_text.done":
		return &MessageDelta{
			Role: "assistant",
		}
	case "response.reasoning_summary_text.delta":
		reasoningSummaryDelta := event.AsResponseReasoningSummaryTextDelta()
		return &MessageDelta{
			Role:             "assistant",
			ReasoningSummary: reasoningSummaryDelta.Delta,
		}
	case "response.reasoning_summary_text.done":
		return &MessageDelta{
			Role: "assistant",
		}
	case "response.function_call_arguments.delta":
		toolCallDelta := event.AsResponseFunctionCallArgumentsDelta()
		return &MessageDelta{
			Role:              "assistant",
			ToolCallID:        item.CallID,
			ToolCallName:      item.Name,
			ToolCallArguments: toolCallDelta.Delta,
		}
	case "response.function_call_arguments.done":
		return &MessageDelta{
			Role:         "assistant",
			ToolCallID:   item.CallID,
			ToolCallName: item.Name,
		}
	default:
		//fmt.Println("unhandled event type: ", event.Type)
		return nil
	}
}
