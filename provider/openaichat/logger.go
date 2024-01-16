package openaichat

import (
	"context"
	"time"

	"log/slog"

	"github.com/sashabaranov/go-openai"
)

func Logger(l *slog.Logger) MiddlewareFunc {
	return func(ctx context.Context, params openai.ChatCompletionRequest, next CreateCompletionFn) (openai.ChatCompletionResponse, error) {
		st := time.Now()
		resp, err := next(ctx, params)
		if err != nil {
			l.LogAttrs(ctx, slog.LevelError, "failed executing completion", slog.String("error", err.Error()))
			return resp, err
		}

		l.LogAttrs(ctx, slog.LevelDebug, "executed completion",
			slog.Duration("elapsed", time.Since(st)),
			slog.Int("prompt_tokens", resp.Usage.PromptTokens),
			slog.Int("completion_tokens", resp.Usage.CompletionTokens),
			slog.String("finish_reason", string(resp.Choices[0].FinishReason)),
		)
		return resp, err
	}
}
