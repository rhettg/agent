package agent

import (
	"context"
)

type UntilFunc func(context.Context, *Message) bool

type Stepper interface {
	Step(context.Context) (*Message, error)
}

type StepFunc func(context.Context) (*Message, error)

func (f StepFunc) Step(ctx context.Context) (m *Message, err error) {
	return f(ctx)
}

func RunUntil(ctx context.Context, s Stepper, uf UntilFunc) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := s.Step(ctx)
		if err != nil {
			return err
		}

		if uf(ctx, msg) {
			return nil
		}
	}
}

// Run until the context is done
func Run(ctx context.Context, s Stepper) error {
	until := func(ctx context.Context, _ *Message) bool {
		return false
	}
	return RunUntil(ctx, s, until)
}
