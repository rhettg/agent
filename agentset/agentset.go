package agentset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/rhettg/agent"
	"github.com/rhettg/agent/tools"
)

var StartHelp = `Start a conversation with a new AI Agent`
var StopHelp = `Stop current conversation with an AI Agent and resume talking to the user`

var StartSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"agent": map[string]any{
			"type":        "string",
			"description": "name of an agent to start",
		},
	},
	"required": []string{"agent"},
}

var StopSchema = map[string]any{
	"type":       "object",
	"properties": map[string]any{},
}

type startFunc func() (*agent.Agent, string)

// enum of AgentSession states
type state int

const (
	// stateIdle is the default state of an agent session. No agent is running.
	stateIdle state = iota

	// stateStarted is the state of an agent session when the user has requested to start an agent.
	// Welcome message is likely pending
	stateStarted

	// stateRunning is the state of an agent session when the agent is running.
	stateRunning

	// stateWaiting is the state of an agent session when the agent is waiting for input.
	stateWaiting
)

type AgentSet struct {
	state          state
	name           string
	welcomeMsg     string
	agentAssistant *agent.Agent

	agentFns map[string]startFunc
}

func (a *AgentSet) Add(name string, f startFunc) {
	a.agentFns[name] = f
}

func (a *AgentSet) Idle() bool {
	return a.state == stateIdle
}

func (a *AgentSet) Start(ctx context.Context, arguments string) (string, error) {
	if a.state != stateIdle {
		return "Agent is already running", nil
	}

	a.state = stateStarted

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

func (a *AgentSet) CompletionFunc(nextStep agent.CompletionFunc) agent.CompletionFunc {
	return func(ctx context.Context, msgs []*agent.Message, tdfs []agent.ToolDef) (*agent.Message, error) {
		slog.Debug("AgentSet.CompletionFunc", "agent", a.name, "agent_state", a.state)
		switch a.state {
		case stateStarted:
			// This is one of the rare cases where a Step doesn't result in an
			// Completion call.  It's really just a way of working around the
			// API where the only way to add a message to the conversation is via
			// a call to Step.
			msg := agent.NewContentMessage(agent.RoleUser, a.welcomeMsg)
			a.state = stateWaiting
			slog.Debug("agent set state change", "agent", a.name, "state", a.state)
			return msg, nil
		case stateWaiting:
			lastMsg := msgs[len(msgs)-1]
			if lastMsg.Role == agent.RoleAssistant {
				if !lastMsg.HasToolCalls() {
					content, err := lastMsg.Content(ctx)
					if err != nil {
						return nil, err
					}

					a.agentAssistant.Add(agent.RoleUser, content)
					a.state = stateRunning
					slog.Debug("agent set state change", "agent", a.name, "state", a.state)
				} else if toolCall := lastMsg.GetFirstToolCall(); toolCall != nil && toolCall.Name != "agent_stop" {
					// When an agent is running we only allow the caller to call stop.
					// We need this special case handling because the often the
					// calling agent becomes confused *really* wanting to call
					// functions.
					msg := agent.NewContentMessage(agent.RoleUser, "I'm sorry, but I don't respond to functions like that. Just ask me directly.")
					return msg, nil
				}
			}
			// fall-through
		case stateRunning, stateIdle:
			// fall-through
		}

		// Phase two, now that we may have handled some state changes.

		switch a.state {
		case stateRunning:
			m, err := a.agentAssistant.Step(ctx)
			if err != nil {
				return nil, fmt.Errorf("error running agent: %w", err)
			}

			if m.Role == agent.RoleAssistant && !m.HasToolCalls() {
				rContent, _ := m.Content(ctx)
				a.state = stateWaiting
				slog.Debug("agent set state change", "agent", a.name, "state", a.state)
				return agent.NewContentMessage(agent.RoleUser, rContent), nil
			}
			return nil, nil
		case stateIdle, stateWaiting:
			return nextStep(ctx, msgs, tdfs)
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
	a.state = stateIdle

	return fmt.Sprintf("%s has left the chat", a.name), nil
}

func (a *AgentSet) Tools() *tools.Tools {
	ts := tools.New()
	ts.Add("agent_start", StartHelp, StartSchema, a.Start)
	ts.Add("agent_stop", StopHelp, StopSchema, a.Stop)
	return ts
}

func New() *AgentSet {
	return &AgentSet{
		state:    stateIdle,
		agentFns: make(map[string]startFunc),
	}
}

func NewFromAgentSet(as *AgentSet) *AgentSet {
	nas := New()
	for name, fn := range as.agentFns {
		nas.Add(name, fn)
	}

	return nas
}

func WithAgentSet(s *AgentSet) agent.Option {
	return agent.WithMiddleware(s.CompletionFunc)
}
