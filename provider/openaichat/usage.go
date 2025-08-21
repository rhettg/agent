package openaichat

import (
	"context"

	"github.com/openai/openai-go"
)

type Usage struct {
	Completions      int
	CompletionTokens int
	PromptTokens     int
	TotalTokens      int
	Errors           int
}

func (u *Usage) Middleware(
	ctx context.Context, params openai.ChatCompletionNewParams, next CreateCompletionFn,
) (*openai.ChatCompletion, error) {
	resp, err := next(ctx, params)
	if err != nil {
		u.Errors++
		return resp, err
	}

	if resp.Usage.TotalTokens > 0 {
		u.Completions++
	}

	u.CompletionTokens += int(resp.Usage.CompletionTokens)
	u.PromptTokens += int(resp.Usage.PromptTokens)
	u.TotalTokens += int(resp.Usage.TotalTokens)
	return resp, err
}
