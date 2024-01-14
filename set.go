package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/rakyll/openai-go/chat"
)

var AgentStartHelp = `Start a conversation with a new AI Agent`
var AgentStopHelp = `Stop current conversation with an AI Agent and resume talking to the user`

var AgentStartSchema = chat.Schema{
	Type: "object",
	Properties: map[string]chat.Schema{
		"agent": {
			Type:        "string",
			Description: "name of an agent to start",
		},
	},
	Required: []string{"agent"},
}

var AgentStopSchema = chat.EmptyParameters

type agentStartFunc func() (*Assistant, string)

// enum of AgentSession states
type agentSetState int

const (
	// agentSetStateIdle is the default state of an agent session. No agent is running.
	agentSetStateIdle agentSetState = iota

	// agentSetStateStarted is the state of an agent session when the user has requested to start an agent.
	// Welcome message is likely pending
	agentSetStateStarted

	// agentSetStateRunning is the state of an agent session when the agent is running.
	agentSetStateRunning

	// agentSetStateWaiting is the state of an agent session when the agent is waiting for input.
	agentSetStateWaiting
)

type AgentSet struct {
	state          agentSetState
	name           string
	welcomeMsg     string
	agentAssistant *Assistant

	agentFns map[string]agentStartFunc
}

func (a *AgentSet) Add(name string, f agentStartFunc) {
	a.agentFns[name] = f
}

func (a *AgentSet) Idle() bool {
	return a.state == agentSetStateIdle
}

func (a *AgentSet) Start(ctx context.Context, arguments string) (string, error) {
	if a.state != agentSetStateIdle {
		return "Agent is already running", nil
	}

	a.state = agentSetStateStarted

	args := struct {
		Agent string `json:"agent"`
	}{}

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		return "", err
	}

	fn, ok := a.agentFns[args.Agent]
	if !ok {
		return fmt.Sprintf("Agent %s not found", args.Agent), nil
	}

	a.name = args.Agent

	a.agentAssistant, a.welcomeMsg = fn()

	return fmt.Sprintf("%s has entered the chat", a.name), nil
}

func (a *AgentSet) CompletionFunc(nextStep CompletionFunc) CompletionFunc {
	return func(ctx context.Context, msgs []*Message, fns []FunctionDef) (*Message, error) {
		slog.Debug("AgentSet.CompletionFunc", "agent", a.name, "agent_state", a.state)
		switch a.state {
		case agentSetStateStarted:
			// This is one of the rare cases where a Step doesn't result in an
			// Completion call.  It's really just a way of working around the
			// API where the only way to add a message to the conversation is via
			// a call to Step.
			msg := NewContentMessage(RoleUser, a.welcomeMsg)
			a.state = agentSetStateWaiting
			slog.Debug("agent set state change", "agent", a.name, "state", a.state)
			return msg, nil
		case agentSetStateWaiting:
			lastMsg := msgs[len(msgs)-1]
			if lastMsg.Role == RoleAssistant {
				if lastMsg.FunctionCallName == "" {
					content, err := lastMsg.Content(ctx)
					if err != nil {
						return nil, err
					}

					a.agentAssistant.Add(RoleUser, content)
					a.state = agentSetStateRunning
					slog.Debug("agent set state change", "agent", a.name, "state", a.state)
				} else if lastMsg.FunctionCallName != "agent_stop" {
					// When an agent is running we only allow the caller to call stop.
					// We need this special case handling because the often the
					// calling agent becomes confused *really* wanting to call
					// functions.
					msg := NewContentMessage(RoleUser, "I'm sorry, but I don't respond to functions like that. Just ask me directly.")
					return msg, nil
				}
			}
			// fall-through
		case agentSetStateRunning, agentSetStateIdle:
			// fall-through
		}

		// Phase two, now that we may have handled some state changes.

		switch a.state {
		case agentSetStateRunning:
			m, err := a.agentAssistant.Step(ctx)
			if err != nil {
				return nil, fmt.Errorf("error running agent: %w", err)
			}

			if m.Role == RoleAssistant && m.FunctionCallName == "" {
				rContent, _ := m.Content(ctx)
				a.state = agentSetStateWaiting
				slog.Debug("agent set state change", "agent", a.name, "state", a.state)
				return NewContentMessage(RoleUser, rContent), nil
			}
			return nil, nil
		case agentSetStateIdle, agentSetStateWaiting:
			return nextStep(ctx, msgs, fns)
		}

		return nil, errors.New("unknown agent set state")
	}
}

func (a *AgentSet) Stop(ctx context.Context, arguments string) (string, error) {
	if a.name == "" {
		return "No agent is currently running", nil
	}

	a.name = ""
	a.agentAssistant = nil
	a.state = agentSetStateIdle

	return fmt.Sprintf("%s has left the chat", a.name), nil
}

func (a *AgentSet) FunctionSet() *FunctionSet {
	fs := NewFunctionSet()
	fs.Add("agent_start", AgentStartHelp, AgentStartSchema, a.Start)
	fs.Add("agent_stop", AgentStopHelp, AgentStopSchema, a.Stop)
	return fs
}

func NewAgentSet() *AgentSet {
	return &AgentSet{
		state:    agentSetStateIdle,
		agentFns: make(map[string]agentStartFunc),
	}
}

func NewAgentSetFromAgentSet(as *AgentSet) *AgentSet {
	nas := NewAgentSet()
	for name, fn := range as.agentFns {
		nas.Add(name, fn)
	}

	return nas
}

func WithAgentSet(s *AgentSet) Option {
	return func(a *Assistant) {
		a.AgentSet = s
	}
}
