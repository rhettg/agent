package agent

import (
	"context"
)

type Agent struct {
	completionFunc CompletionFunc
	messages       []*Message
}

type Option func(a *Agent)

func WithMiddleware(m MiddlewareFunc) Option {
	return func(a *Agent) {
		a.completionFunc = m(a.completionFunc)
	}
}

func New(c CompletionFunc, opts ...Option) *Agent {
	a := &Agent{
		completionFunc: c,
		messages:       make([]*Message, 0),
	}

	for _, o := range opts {
		o(a)
	}

	return a
}

// NewFromAgent creates a new agent based on an existing agent.
//
// The new agent will have the same capabilities and history as the previous
// agent, but any changes will not be propogated to the original.
func NewFromAgent(a *Agent) *Agent {
	na := &Agent{
		completionFunc: a.completionFunc,
		messages:       make([]*Message, 0, len(a.messages)),
	}

	for _, m := range a.messages {
		na.messages = append(na.messages, NewMessageFromMessage(m))
	}

	return na
}

func (a *Agent) Add(role Role, content string) *Agent {
	msg := newMessage()
	msg.Role = role
	msg.content = content

	a.messages = append(a.messages, msg)
	return a
}

func (a *Agent) AddMessage(m *Message) *Agent {
	a.messages = append(a.messages, m)
	return a
}

func (a *Agent) Messages() []*Message {
	msgs := make([]*Message, len(a.messages))
	copy(msgs, a.messages)
	return msgs
}

func (a *Agent) Stop() {
	msg := newMessage()
	msg.stop = true
	a.messages = append(a.messages, msg)
}

func (a *Agent) Step(ctx context.Context) (*Message, error) {
	nextMsg, err := a.completionFunc(ctx, a.messages, nil)
	if err != nil {
		return nil, err
	}

	// An empty step is oke. This would be possible if there is some
	// internal state, like a sub-assistant, that hasn't yet resulted in a
	// message.
	if nextMsg == nil {
		return nil, nil
	}

	a.messages = append(a.messages, nextMsg)

	return nextMsg, nil
}
