package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/rhettg/agent"
)

// ToolLogger returns middleware that logs tool calls with timing information
func ToolLogger(l *slog.Logger) ToolMiddleware {
	return func(next ToolInvokeFunc) ToolInvokeFunc {
		return func(ctx context.Context, call *agent.ToolCall) (*agent.Message, error) {
			st := time.Now()
			l.LogAttrs(ctx, slog.LevelInfo, "tool_call_start",
				slog.String("tool_name", call.Name),
				slog.String("tool_call_id", call.ID),
				slog.Int("args_len", len(call.Arguments)),
			)

			msg, err := next(ctx, call)

			if err != nil {
				l.LogAttrs(ctx, slog.LevelError, "tool_call_error",
					slog.String("tool_name", call.Name),
					slog.String("tool_call_id", call.ID),
					slog.Duration("elapsed", time.Since(st)),
					slog.String("error", err.Error()),
				)
				return nil, err
			}

			// Add timing attribute for downstream checks/filters
			if msg != nil {
				if msg.Attributes == nil {
					msg.Attributes = agent.Attributes{}
				}
				msg.SetAttr("tool_elapsed_ms", time.Since(st).String())
			}

			l.LogAttrs(ctx, slog.LevelInfo, "tool_call_end",
				slog.String("tool_name", call.Name),
				slog.String("tool_call_id", call.ID),
				slog.Duration("elapsed", time.Since(st)),
			)
			return msg, nil
		}
	}
}
