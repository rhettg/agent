package agent

import (
	"context"
	"fmt"
)

type CheckFunc func(context.Context, *Message) error

func (c CheckFunc) CompletionFunc(nextStep CompletionFunc) CompletionFunc {
	return func(ctx context.Context, msgs []*Message, fns []FunctionDef) (*Message, error) {
		msg, err := nextStep(ctx, msgs, fns)
		if err != nil {
			return nil, err
		}

		err = c(ctx, msg)
		if err != nil {
			return nil, fmt.Errorf("check failed: %w", err)
		}

		return msg, nil
	}
}

func WithCheck(c CheckFunc) Option {
	return func(a *Assistant) {
		a.checks = append(a.checks, c)
	}
}
