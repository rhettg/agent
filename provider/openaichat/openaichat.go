package openaichat

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
	"github.com/rhettg/agent"
)

type CreateCompletionFn func(context.Context, openai.ChatCompletionNewParams, ...option.RequestOption) (*openai.ChatCompletion, error)
type MiddlewareFunc func(context.Context, openai.ChatCompletionNewParams, CreateCompletionFn) (*openai.ChatCompletion, error)

const defaultTemperature = float64(1.0)

type provider struct {
	client      openai.Client
	temperature float64
	maxTokens   int
	mw          []MiddlewareFunc
	modelName   string
}

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
			aMsg.OfAssistant.ToolCalls = make([]openai.ChatCompletionMessageToolCallParam, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				aMsg.OfAssistant.ToolCalls[i] = openai.ChatCompletionMessageToolCallParam{
					ID: tc.ID,
					Function: openai.ChatCompletionMessageToolCallFunctionParam{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
					Type: "function",
				}
			}
			pMsgs = append(pMsgs, aMsg)
		case agent.RoleFunction, agent.RoleTool:
			// For tool responses, we need the tool call ID
			toolID := m.ToolCallID
			pMsgs = append(pMsgs, openai.ToolMessage(c, toolID))
		}
	}

	tools := make([]openai.ChatCompletionToolParam, 0, len(tdfs))
	for _, fd := range tdfs {
		tools = append(tools, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        fd.Name,
				Description: openai.String(fd.Description),
				Parameters:  shared.FunctionParameters(fd.Parameters.(map[string]any)),
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
	c := p.client.Chat.Completions.New
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
