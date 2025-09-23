package openaichat

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/shared"
	"github.com/rhettg/agent"
)

type CreateCompletionFn func(context.Context, openai.ChatCompletionNewParams, ...option.RequestOption) (*openai.ChatCompletion, error)
type MiddlewareFunc func(context.Context, openai.ChatCompletionNewParams, CreateCompletionFn) (*openai.ChatCompletion, error)

const defaultTemperature = float64(1.0)

type provider struct {
	client           openai.Client
	temperature      float64
	maxTokens        int
	mw               []MiddlewareFunc
	modelName        string
	messageDeltaFunc MessageDeltaFunc
}

type MessageDelta struct {
	Role              string
	Content           string
	ToolCallID        string
	ToolCallName      string
	ToolCallArguments string
}

type MessageDeltaFunc func(ctx context.Context, delta MessageDelta)

type Option func(p *provider)

func WithMiddleware(m MiddlewareFunc) func(p *provider) {
	return func(p *provider) {
		p.mw = append(p.mw, m)
	}
}

func WithTemperature(t float64) func(p *provider) {
	return func(p *provider) {
		p.temperature = t
	}
}

func WithMaxTokens(m int) func(p *provider) {
	return func(p *provider) {
		p.maxTokens = m
	}
}

func WithMessageDeltaFunc(f MessageDeltaFunc) func(p *provider) {
	return func(p *provider) {
		p.messageDeltaFunc = f
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
	}

	for _, o := range opts {
		o(p)
	}

	return p.Completion
}

func (p *provider) stream(ctx context.Context, params openai.ChatCompletionNewParams, opts ...option.RequestOption) (*openai.ChatCompletion, error) {
	stream := p.client.Chat.Completions.NewStreaming(ctx, params, opts...)
	defer stream.Close()

	// Initialize result structure
	result := &openai.ChatCompletion{
		ID:      "",
		Object:  "chat.completion",
		Model:   string(params.Model),
		Created: 0,
		Choices: []openai.ChatCompletionChoice{{
			Index: 0,
			Message: openai.ChatCompletionMessage{
				Role:      "assistant",
				Content:   "",
				ToolCalls: []openai.ChatCompletionMessageToolCallUnion{},
			},
			FinishReason: "",
		}},
		Usage: openai.CompletionUsage{},
	}

	// Use strings.Builder for efficient string concatenation
	var contentBuilder strings.Builder
	toolCallArgBuilders := make(map[int]*strings.Builder)

	for stream.Next() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		evt := stream.Current()

		// Set metadata from first event
		if result.ID == "" {
			result.ID = evt.ID
			result.Created = evt.Created
		}

		if len(evt.Choices) > 0 {
			delta := evt.Choices[0].Delta
			choice := &result.Choices[0]

			// Accumulate content
			if delta.Content != "" {
				if p.messageDeltaFunc != nil {
					md := MessageDelta{
						Role:    delta.Role,
						Content: delta.Content,
					}
					p.messageDeltaFunc(ctx, md)
				}
				contentBuilder.WriteString(delta.Content)
			}

			// Handle tool calls
			if len(delta.ToolCalls) > 0 {
				for _, toolCall := range delta.ToolCalls {
					// Extend tool calls array if needed
					for len(choice.Message.ToolCalls) <= int(toolCall.Index) {
						choice.Message.ToolCalls = append(choice.Message.ToolCalls, openai.ChatCompletionMessageToolCallUnion{})
					}

					tc := &choice.Message.ToolCalls[toolCall.Index]
					if toolCall.ID != "" {
						tc.ID = toolCall.ID
					}
					if toolCall.Type != "" {
						tc.Type = toolCall.Type
					}
					if toolCall.Function.Name != "" {
						tc.Function.Name = toolCall.Function.Name
					}
					if toolCall.Function.Arguments != "" {
						if p.messageDeltaFunc != nil {
							md := MessageDelta{
								ToolCallID:        toolCall.ID,
								ToolCallName:      toolCall.Function.Name,
								ToolCallArguments: toolCall.Function.Arguments,
							}
							p.messageDeltaFunc(ctx, md)
						}

						// Get or create builder for this tool call index
						builder, exists := toolCallArgBuilders[int(toolCall.Index)]
						if !exists {
							builder = &strings.Builder{}
							toolCallArgBuilders[int(toolCall.Index)] = builder
						}
						builder.WriteString(toolCall.Function.Arguments)
					}
				}
			}

			// Set finish reason
			if evt.Choices[0].FinishReason != "" {
				choice.FinishReason = evt.Choices[0].FinishReason
			}
		}

		// Set usage if available (check if Usage has values)
		if evt.Usage.PromptTokens != 0 || evt.Usage.CompletionTokens != 0 || evt.Usage.TotalTokens != 0 {
			result.Usage = evt.Usage
		}
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Set final content from builder
	result.Choices[0].Message.Content = contentBuilder.String()

	// Set final tool call arguments from builders
	for i, tc := range result.Choices[0].Message.ToolCalls {
		if builder, exists := toolCallArgBuilders[i]; exists {
			tc.Function.Arguments = builder.String()
		}
	}

	return result, nil
}

func (p *provider) Completion(
	ctx context.Context, msgs []*agent.Message, tdfs []agent.ToolDef,
) (*agent.Message, error) {
	pMsgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(msgs))
	for _, m := range msgs {
		c, err := m.Content(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get message content: %w", err)
		}

		switch m.Role {
		case agent.RoleSystem:
			pMsgs = append(pMsgs, openai.SystemMessage(c))
		case agent.RoleUser:
			if len(m.Images()) > 0 {
				// Handle multimodal content
				content := make([]openai.ChatCompletionContentPartUnionParam, 0)
				if c != "" {
					content = append(content, openai.TextContentPart(c))
				}

				for _, img := range m.Images() {
					mimeType := mimeType(img.Name)
					imageURL := encodeImageURL(mimeType, img.Data)
					content = append(content, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
						URL: imageURL,
					}))
				}

				pMsgs = append(pMsgs, openai.UserMessage(content))
			} else {
				pMsgs = append(pMsgs, openai.UserMessage(c))
			}
		case agent.RoleAssistant:
			aMsg := openai.AssistantMessage(c)
			aMsg.OfAssistant.ToolCalls = make([]openai.ChatCompletionMessageToolCallUnionParam, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				aMsg.OfAssistant.ToolCalls[i] = openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: tc.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: tc.Arguments,
						},
						Type: "function",
					},
				}
			}
			pMsgs = append(pMsgs, aMsg)
		case agent.RoleTool:
			// For tool responses, we need the tool call ID
			toolID := m.ToolCallID
			pMsgs = append(pMsgs, openai.ToolMessage(c, toolID))
		}
	}

	tools := make([]openai.ChatCompletionToolUnionParam, 0, len(tdfs))
	for _, fd := range tdfs {
		tools = append(tools, openai.ChatCompletionToolUnionParam{
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Function: shared.FunctionDefinitionParam{
					Name:        fd.Name,
					Description: openai.String(fd.Description),
					Parameters:  shared.FunctionParameters(fd.Parameters.(map[string]any)),
				},
			},
		})
	}

	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(p.modelName),
		Messages: pMsgs,
	}

	if len(tools) > 0 {
		params.Tools = tools
	}

	if p.maxTokens != 0 {
		params.MaxTokens = openai.Int(int64(p.maxTokens))
	}

	if p.temperature != 0 {
		params.Temperature = openai.Float(p.temperature)
	}

	// Assemble the middleware chain
	c := p.stream
	for _, m := range p.mw {
		next := c
		fm := m
		c = func(ctx context.Context, params openai.ChatCompletionNewParams, opts ...option.RequestOption) (*openai.ChatCompletion, error) {
			return fm(ctx, params, next)
		}
	}

	resp, err := c(ctx, params)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no completion returned")
	}

	rMsg := resp.Choices[0].Message
	m := agent.NewContentMessage(agent.Role(rMsg.Role), rMsg.Content)

	if len(rMsg.ToolCalls) > 0 {
		// Populate the new ToolCalls field with all tool calls
		m.ToolCalls = make([]agent.ToolCall, len(rMsg.ToolCalls))
		for i, tc := range rMsg.ToolCalls {
			m.ToolCalls[i] = agent.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
	}

	return m, nil
}

func mimeType(name string) string {
	dot := strings.LastIndex(name, ".")
	if dot == -1 || dot == len(name)-1 {
		// Just a guess
		return "image/jpeg"
	}

	return "image/" + strings.ToLower(name[dot+1:])
}

func encodeImageURL(mimeType string, data []byte) string {
	// Based on the python reference code in
	// https://platform.openai.com/docs/guides/vision/uploading-base-64-encoded-images
	// this should be the parallel of:
	//     base64.b64encode(image_file.read()).decode('utf-8')
	// which defaults to the standard base64 encoding.  I would have guessed
	// it would be using the URL-safe encoding but that isn't what the code is
	// saying.
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(dst, data)

	image_url := strings.Builder{}
	image_url.WriteString("data:")
	image_url.WriteString(mimeType)
	image_url.WriteString(";base64,")
	image_url.Write(dst)

	return image_url.String()
}
