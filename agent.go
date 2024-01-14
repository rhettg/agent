package agent

import (
	"context"
)

type Role string

const (
	RoleSystem    = Role("system")
	RoleUser      = Role("user")
	RoleAssistant = Role("assistant")
	RoleFunction  = Role("function")
)

type CompletionFunc func(context.Context, []*Message, []FunctionDef) (*Message, error)

type Agent struct {
	completionFunc CompletionFunc
	messages       []*Message
	FunctionSet    *FunctionSet
	AgentSet       *AgentSet

	filters []FilterFunc
	checks  []CheckFunc
}

type Option func(a *Agent)

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

// NewFromAssistant creates a new assistant based on an existing assistant.
//
// The new assistant will have the same capabilities and history as the previous
// assistant, but any changes will not be propogated to the original.
func NewFromAssistant(a *Agent) *Agent {
	na := &Agent{
		completionFunc: a.completionFunc,
		messages:       make([]*Message, 0, len(a.messages)),
	}

	for _, m := range a.messages {
		na.messages = append(na.messages, NewMessageFromMessage(m))
	}

	if a.FunctionSet != nil {
		na.FunctionSet = NewFunctionSetFromFunctionSet(a.FunctionSet)
	}

	if a.AgentSet != nil {
		na.AgentSet = NewAgentSetFromAgentSet(a.AgentSet)
	}

	for _, f := range a.filters {
		na.filters = append(na.filters, f)
	}

	for _, c := range a.checks {
		na.checks = append(na.checks, c)
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

func (a *Agent) AddDynamic(role Role, contentFn ContentFn) *Agent {
	msg := newMessage()
	msg.Role = role
	msg.contentFn = contentFn

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
	var err error

	// Build our final completion function by wrapping appropriate middleware
	// around it.  This pattern is pretty flexible.  It would be entirely
	// possible for our Assistant to not know anything about FunctionSets or
	// AgentSets, however from an API perspective it's real easy to set this up
	// incorrectly. It isn't obvious ahead of time how or why functions are
	// related to the "provider". Consider this some Sugar.
	var cf CompletionFunc

	for _, f := range a.filters {
		cf = f.CompletionFunc(cf)
	}

	cf = a.completionFunc
	if a.FunctionSet != nil {
		cf = a.FunctionSet.CompletionFunc(cf)
	}

	if a.AgentSet != nil {
		cf = a.AgentSet.CompletionFunc(cf)
	}

	for _, c := range a.checks {
		cf = c.CompletionFunc(cf)
	}

	nextMsg, err := cf(ctx, a.messages, nil)
	if err != nil {
		return nil, err
	}

	// We support an empty step. This would be possible if there is some
	// internal state, like a sub-assistant, that hasn't yet resulted in a
	// message.
	if nextMsg == nil {
		return nil, nil
	}

	// Now that our checks have passed, include the new message in our conversation.
	a.messages = append(a.messages, nextMsg)

	return nextMsg, nil
}
