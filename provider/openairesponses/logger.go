package openairesponses

import (
	"context"
	"time"

	"log/slog"

	"github.com/openai/openai-go/v2/responses"
)

func Logger(l *slog.Logger) MiddlewareFunc {
	return func(ctx context.Context, params responses.ResponseNewParams, next ResponsesCompletionFn) (*responses.Response, error) {
		st := time.Now()
		resp, err := next(ctx, params)
		if err != nil {
			l.LogAttrs(ctx, slog.LevelError, "failed executing responses completion", slog.String("error", err.Error()))
			return resp, err
		}

		// Log basic completion info
		l.LogAttrs(ctx, slog.LevelDebug, "executed responses completion",
			slog.Duration("elapsed", time.Since(st)),
			slog.String("model", string(params.Model)),
		)
		
		// Log reasoning settings if configured
		if params.Reasoning.Effort != "" || params.Reasoning.Summary != "" {
			l.LogAttrs(ctx, slog.LevelDebug, "reasoning configured",
				slog.String("effort", string(params.Reasoning.Effort)),
				slog.String("summary", string(params.Reasoning.Summary)),
			)
		}
		
		return resp, err
	}
}
