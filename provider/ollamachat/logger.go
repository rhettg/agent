package ollamachat

import (
	"context"
	"time"

	"log/slog"

	"github.com/jmorganca/ollama/api"
)

func Logger(l *slog.Logger) MiddlewareFunc {
	return func(ctx context.Context, params *api.GenerateRequest, next GenerateFunc) (api.GenerateResponse, error) {
		st := time.Now()
		resp, err := next(ctx, params)
		if err != nil {
			l.LogAttrs(ctx, slog.LevelError, "failed executing completion", slog.String("error", err.Error()))
			return resp, err
		}

		l.LogAttrs(ctx, slog.LevelDebug, "executed completion",
			slog.Duration("elapsed", time.Since(st)),
			slog.Duration("load_duration", resp.LoadDuration),
			slog.Int("eval_count", resp.EvalCount),
			slog.Int("prompt_eval_count", resp.PromptEvalCount),
		)
		return resp, err
	}
}
