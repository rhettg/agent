package openaichat

import (
	"context"
	"encoding/base64"
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

		for _, img := range m.Images() {
			mimeType := mimeType(img.Name)
			imageURL := encodeImageURL(mimeType, img.Data)

			ic := openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL:    imageURL,
					Detail: openai.ImageURLDetailAuto,
				},
			}

			content = append(content, ic)
		}

		pm := openai.ChatCompletionMessage{
			Role: string(m.Role),
		}

		if len(content) == 0 {
			pm.Content = ""
		} else if len(content) > 1 || content[0].Type != openai.ChatMessagePartTypeText {
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
