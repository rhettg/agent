package agent

import (
	"context"
	"fmt"
)

type FilterFunc func(context.Context, []*Message) ([]*Message, error)

func (f FilterFunc) CompletionFunc(nextStep CompletionFunc) CompletionFunc {
	return func(ctx context.Context, msgs []*Message, fns []FunctionDef) (*Message, error) {
		fMsgs, err := f(ctx, msgs)
		if err != nil {
			return nil, fmt.Errorf("filter failed: %w", err)
		}

		return nextStep(ctx, fMsgs, fns)
	}
}

func WithFilter(f FilterFunc) Option {
	return WithMiddleware(f.CompletionFunc)
}

func LimitMessagesFilter(max int) FilterFunc {
	return func(ctx context.Context, msgs []*Message) ([]*Message, error) {
		if len(msgs) > max {
			return msgs[len(msgs)-max:], nil
		}
		return msgs, nil
	}
}
