package openaichat

import (
	"context"

	"github.com/sashabaranov/go-openai"
)

type Usage struct {
	Completions      int
	CompletionTokens int
	PromptTokens     int
	TotalTokens      int
	Errors           int
}

func (u *Usage) Middleware(
	ctx context.Context, params openai.ChatCompletionRequest, next CreateCompletionFn,
) (openai.ChatCompletionResponse, error) {
	resp, err := next(ctx, params)
	if err != nil {
		u.Errors++
		return resp, err
	}

	if resp.Usage.TotalTokens > 0 {
		u.Completions++
	}

	u.CompletionTokens += resp.Usage.CompletionTokens
	u.PromptTokens += resp.Usage.PromptTokens
	u.TotalTokens += resp.Usage.TotalTokens
	return resp, err
}
