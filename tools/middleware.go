package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/rhettg/agent"
)

// ToolCallStartNotifier returns middleware that sends tool calls to a channel
// prior to execution.
func ToolCallStartNotifier(ch chan<- agent.ToolCall) ToolMiddleware {
	return func(next ToolInvokeFunc) ToolInvokeFunc {
		return func(ctx context.Context, call *agent.ToolCall) (*agent.Message, error) {

			select {
			case ch <- *call:
			default:
			}

			return next(ctx, call)
		}
	}
}

// ToolCallEvent represents a tool call event sent over a channel
type ToolCallEvent struct {
	ToolCall *agent.ToolCall
	Message  *agent.Message
	Error    error
	Started  time.Time
	Finished time.Time
}

// ToolCallNotifier returns middleware that sends tool call results to a channel.
// This is useful for observability and monitoring of tool execution.
// The channel is non-blocking - if the channel is full, events are dropped.
func ToolCallNotifier(ch chan<- ToolCallEvent) ToolMiddleware {
	return func(next ToolInvokeFunc) ToolInvokeFunc {
		return func(ctx context.Context, call *agent.ToolCall) (*agent.Message, error) {
			started := time.Now()

			msg, err := next(ctx, call)

			event := ToolCallEvent{
				ToolCall: call,
				Message:  msg,
				Error:    err,
				Started:  started,
				Finished: time.Now(),
			}

			// Non-blocking send
			select {
			case ch <- event:
			default:
			}

			return msg, err
		}
	}
}

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
