package agent

import "context"

type FilterFunc func(context.Context, []*Message) ([]*Message, error)

func (f FilterFunc) CompletionFunc(nextStep CompletionFunc) CompletionFunc {
	return func(ctx context.Context, msgs []*Message, fns []FunctionDef) (*Message, error) {
		fMsgs, err := f(ctx, msgs)
		if err != nil {
			return nil, err
		}

		return nextStep(ctx, fMsgs, fns)
	}
}

func WithFilter(f FilterFunc) Option {
	return func(a *Agent) {
		a.filters = append(a.filters, f)
	}
}
