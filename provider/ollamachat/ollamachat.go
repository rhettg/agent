package ollamachat

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jmorganca/ollama/api"
	"github.com/rhettg/agent"
)

type GenerateFunc func(context.Context, *api.GenerateRequest) (api.GenerateResponse, error)
type MiddlewareFunc func(context.Context, *api.GenerateRequest, GenerateFunc) (api.GenerateResponse, error)

func newGenerateFunc(c *api.Client) GenerateFunc {
	return func(ctx context.Context, req *api.GenerateRequest) (api.GenerateResponse, error) {
		r := make(chan api.GenerateResponse, 1)

		handleResponse := func(resp api.GenerateResponse) error {
			if !resp.Done {
				return errors.New("streaming response not supported")
			}

			r <- resp
			close(r)
			return nil
		}

		err := c.Generate(ctx, req, handleResponse)
		if err != nil {
			return api.GenerateResponse{}, err
		}

		resp := <-r

		return resp, nil
	}
}

type provider struct {
	client    *api.Client
	modelName string

	mw []MiddlewareFunc
}

type Option func(p *provider)

func WithMiddleware(m MiddlewareFunc) func(p *provider) {
	return func(p *provider) {
		p.mw = append(p.mw, m)
	}
}

func New(c *api.Client, modelName string, opts ...Option) agent.CompletionFunc {
	p := &provider{
		client:    c,
		modelName: modelName,
	}

	for _, o := range opts {
		o(p)
	}

	return p.Completion
}

type formatDialogFn func(context.Context, []*agent.Message) (string, error)

// formatDialogLlama formats the conversation into a document that an LLM will understand
//
// This is based on the llama2 python code in https://github.com/facebookresearch/llama/blob/ef351e9cd9496c579bf9f2bb036ef11bdc5ca3d2/llama/generation.py#L284-L395
func formatDialogLlama(ctx context.Context, msgs []*agent.Message) (string, error) {
	content := strings.Builder{}

	system := ""
	for ndx, m := range msgs {
		// Assistant (replies) will have been added already.
		if m.Role == agent.RoleAssistant {
			continue
		}

		c, err := m.Content(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get message content: %w", err)
		}

		// System messages will be included in the very first instruction.
		if m.Role == agent.RoleSystem {
			system = c
			continue
		}

		if m.Role != agent.RoleUser {
			return "", errors.New("unsupported role")
		}

		content.WriteString("[INST] ")
		if system != "" {
			content.WriteString("<<SYS>>\n")
			content.WriteString(strings.TrimSpace(system))
			content.WriteString("\n<</SYS>>\n\n")
			system = ""
		}

		content.WriteString(strings.TrimSpace(c))

		content.WriteString(" [/INST]")

		// Do we have an answer?
		if ndx < len(msgs)-1 && msgs[ndx+1].Role == agent.RoleAssistant {
			content.WriteString(" ")
			ac, err := msgs[ndx+1].Content(ctx)
			if err != nil {
				return "", err
			}

			content.WriteString(strings.TrimSpace(ac))
			content.WriteString(" ")
		}
	}

	return content.String(), nil
}

// formatDialogMistral formats the conversation into a document that an LLM will understand
//
// This is based on the spec provided in https://docs.mistral.ai/llm/mistral-instruct-v0.1
func formatDialogMistral(ctx context.Context, msgs []*agent.Message) (string, error) {
	content := strings.Builder{}

	eos := false
	system := ""
	for ndx, m := range msgs {
		// Assistant (replies) will have been added already.
		if m.Role == agent.RoleAssistant {
			continue
		}

		c, err := m.Content(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get message content: %w", err)
		}

		// System messages will be included in the very first instruction.
		if m.Role == agent.RoleSystem {
			system = c
			continue
		}

		if m.Role != agent.RoleUser {
			return "", errors.New("unsupported role")
		}

		content.WriteString("[INST] ")
		if system != "" {
			content.WriteString(strings.TrimSpace(system))
			content.WriteString("\n\n")
			system = ""
		}

		content.WriteString(strings.TrimSpace(c))

		content.WriteString(" [/INST]")
		if !eos {
			content.WriteString("</s>")
			eos = true
		}

		// Do we have an answer?
		if ndx < len(msgs)-1 && msgs[ndx+1].Role == agent.RoleAssistant {
			content.WriteString(" ")
			ac, err := msgs[ndx+1].Content(ctx)
			if err != nil {
				return "", err
			}

			content.WriteString(strings.TrimSpace(ac))
			content.WriteString(" ")
		}
	}

	return content.String(), nil
}

func (p *provider) Completion(
	ctx context.Context, msgs []*agent.Message, tdfs []agent.ToolDef,
) (*agent.Message, error) {

	var formatDialog formatDialogFn
	switch p.modelName {
	case "mistral", "mistral:instruct":
		formatDialog = formatDialogMistral
	case "llama", "llama2", "llama2:instruct":
		formatDialog = formatDialogLlama
	default:
		return nil, errors.New("unsupported model name")
	}

	content, err := formatDialog(ctx, msgs)
	if err != nil {
		return nil, err
	}

	stream := false
	req := &api.GenerateRequest{
		Prompt: content,
		Model:  p.modelName,
		Stream: &stream,
		Raw:    true,
	}

	// Assemble the middleware chain
	gc := newGenerateFunc(p.client)
	for _, m := range p.mw {
		next := gc
		fm := m
		gc = func(ctx context.Context, req *api.GenerateRequest) (api.GenerateResponse, error) {
			return fm(ctx, req, next)
		}
	}

	resp, err := gc(ctx, req)
	if err != nil {
		return nil, err
	}

	cleanResp := strings.TrimSpace(resp.Response)
	// Sometimes we see a EOS token to begin the response, remove that just in case
	cleanResp = strings.TrimPrefix(cleanResp, "</s>")

	m := agent.NewContentMessage(agent.RoleAssistant, cleanResp)

	return m, nil
}
