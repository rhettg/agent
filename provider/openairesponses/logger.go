package openairesponses

import (
	"context"
	"time"

	"log/slog"
)

func Logger(l *slog.Logger) MiddlewareFunc {
	return func(ctx context.Context, params ResponsesNewParams, next ResponsesCompletionFn) (*Response, error) {
		st := time.Now()
		resp, err := next(ctx, params)
		if err != nil {
			l.LogAttrs(ctx, slog.LevelError, "failed executing responses completion", slog.String("error", err.Error()))
			return resp, err
		}

		// Log basic completion info
		l.LogAttrs(ctx, slog.LevelDebug, "executed responses completion",
			slog.Duration("elapsed", time.Since(st)),
			slog.String("model", params.Model),
			slog.Bool("stream", params.Stream != nil && *params.Stream),
		)
		
		// Log reasoning settings if configured
		if params.IncludeReasoning != nil && *params.IncludeReasoning {
			l.LogAttrs(ctx, slog.LevelDebug, "reasoning enabled",
				slog.Bool("include_reasoning", true),
			)
		}
		
		return resp, err
	}
}
