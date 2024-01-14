package openaichat

import (
	"context"
	"fmt"
	"strings"

	"github.com/rakyll/openai-go"
	"github.com/rakyll/openai-go/chat"
	"github.com/rhettg/agent"
)

type CreateCompletionFn func(context.Context, *chat.CreateMMCompletionParams) (*chat.CreateCompletionResponse, error)
type MiddlewareFunc func(context.Context, *chat.CreateMMCompletionParams, CreateCompletionFn) (*chat.CreateCompletionResponse, error)

const defaultTemperature = float64(0.9)

type provider struct {
	client      *chat.Client
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

func New(s *openai.Session, modelName string, opts ...Option) agent.CompletionFunc {
	// Create the OpenAI API client
	client := chat.NewClient(s, modelName)

	p := &provider{
		client:      client,
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
	pMsgs := make([]*chat.MMMessage, 0, len(msgs))
	for _, m := range msgs {
		c, err := m.Content(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get message content: %w", err)
		}

		content := make([]chat.Content, 0)
		if c != "" {
			content = append(content, chat.NewContentFromText(c))
		}

		for _, img := range m.Images() {
			ic, err := chat.NewContentFromImage(mimeType(img.Name), img.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to create content from image: %w", err)
			}
			content = append(content, ic)
		}

		pm := &chat.MMMessage{
			Role:    string(m.Role),
			Content: content,
		}

		if m.FunctionCallName != "" {
			switch m.Role {
			case agent.RoleFunction:
				pm.Name = m.FunctionCallName
			default:
				pm.FunctionCall = &chat.FunctionCall{
					Name:      m.FunctionCallName,
					Arguments: m.FunctionCallArgs,
				}
			}
		}

		pMsgs = append(pMsgs, pm)
	}

	funcs := make([]chat.Function, 0, len(fns))
	for _, fd := range fns {
		funcs = append(funcs, chat.Function{
			Name:        fd.Name,
			Description: fd.Description,
			Parameters:  fd.Parameters,
		})
	}

	params := &chat.CreateMMCompletionParams{
		Messages:  pMsgs,
		Functions: funcs,
	}

	if p.maxTokens != 0 {
		params.MaxTokens = p.maxTokens
	}

	// Assemble the middleware chain
	c := p.client.CreateMMCompletion
	for _, m := range p.mw {
		next := c
		fm := m
		c = func(ctx context.Context, params *chat.CreateMMCompletionParams) (*chat.CreateCompletionResponse, error) {
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
