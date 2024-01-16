package openaichat

import (
	"context"
	"fmt"
	"strings"

	"github.com/rhettg/agent"
	"github.com/sashabaranov/go-openai"
)

type CreateCompletionFn func(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
type MiddlewareFunc func(context.Context, openai.ChatCompletionRequest, CreateCompletionFn) (openai.ChatCompletionResponse, error)

const defaultTemperature = float64(0.9)

type provider struct {
	client      *openai.Client
	temperature float64
	maxTokens   int
	mw          []MiddlewareFunc
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

func New(c *openai.Client, modelName string, opts ...Option) agent.CompletionFunc {
	p := &provider{
		client:      c,
		temperature: defaultTemperature,
	}

	for _, o := range opts {
		o(p)
	}

	return p.Completion
}

func (p *provider) Completion(
	ctx context.Context, msgs []*agent.Message, fns []agent.FunctionDef,
) (*agent.Message, error) {
	pMsgs := make([]openai.ChatCompletionMessage, 0, len(msgs))
	for _, m := range msgs {
		c, err := m.Content(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get message content: %w", err)
		}

		content := make([]openai.ChatMessagePart, 0)
		if c != "" {
			content = append(content, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: c,
			})
		}

		/*
			TODO: reimplement images. Need base64 encoding
			for _, img := range m.Images() {
				ic := openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: imageURL,
				}

				//ic, err := chat.NewContentFromImage(mimeType(img.Name), img.Data)
				//if err != nil {
					//return nil, fmt.Errorf("failed to create content from image: %w", err)
				//}
				content = append(content, ic)
			}
		*/

		pm := openai.ChatCompletionMessage{
			Role: string(m.Role),
		}

		if len(content) > 1 || content[0].Type != openai.ChatMessagePartTypeText {
			pm.MultiContent = content
		} else {
			pm.Content = content[0].Text
		}

		if m.FunctionCallName != "" {
			switch m.Role {
			case agent.RoleFunction:
				pm.Name = m.FunctionCallName
			default:
				pm.FunctionCall = &openai.FunctionCall{
					Name:      m.FunctionCallName,
					Arguments: m.FunctionCallArgs,
				}
			}
		}

		pMsgs = append(pMsgs, pm)
	}

	tools := make([]openai.Tool, 0, len(fns))
	for _, fd := range fns {
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionDefinition{
				Name:        fd.Name,
				Description: fd.Description,
				Parameters:  fd.Parameters,
			},
		})
	}

	params := openai.ChatCompletionRequest{
		Messages: pMsgs,
		Tools:    tools,
	}

	if p.maxTokens != 0 {
		params.MaxTokens = p.maxTokens
	}

	// Assemble the middleware chain
	c := p.client.CreateChatCompletion
	for _, m := range p.mw {
		next := c
		fm := m
		c = func(ctx context.Context, params openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
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

	if rMsg.FunctionCall != nil {
		m.FunctionCallName = rMsg.FunctionCall.Name
		m.FunctionCallArgs = rMsg.FunctionCall.Arguments
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
