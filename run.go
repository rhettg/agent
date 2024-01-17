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

const StopTag = "agt:stop"

func Run(ctx context.Context, s Stepper) error {
	until := func(ctx context.Context, m *Message) bool {
		if m != nil {
			return m.HasTag(StopTag)
		}
		return false
	}
	return RunUntil(ctx, s, until)
}

// StopOnReply is a check function that marks the message as a stop if
// the assistant replies to the user.
//
// This is common in agent chats where a dialog should continue for many steps
// until the assistant actually directly responds to the user.
func StopOnReply(ctx context.Context, m *Message) error {
	if m == nil {
		return nil
	}
	if m.Role == RoleAssistant && m.FunctionCallName == "" {
		m.Tag(StopTag)
	}
	return nil
}
