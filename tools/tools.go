package tools

import (
	"context"
	"fmt"

	"github.com/rhettg/agent"
)

type Tools struct {
	fns  map[string]agent.Tool
	defs []agent.ToolDef
}

func (f *Tools) Add(name, description string, parameters any, fn agent.Tool) {
	def := agent.ToolDef{
		Name:        name,
		Description: description,
		Parameters:  parameters,
	}

	f.defs = append(f.defs, def)
	f.fns[name] = fn
}

func (f *Tools) AddTools(fs *Tools) {
	for _, def := range fs.defs {
		f.Add(def.Name, def.Description, def.Parameters, f.fns[def.Name])
	}
}

func (f *Tools) call(ctx context.Context, toolCall *agent.ToolCall) (*agent.Message, error) {
	fn, ok := f.fns[toolCall.Name]
	if !ok {
		m := agent.NewContentMessage(agent.RoleTool, fmt.Sprintf("tool not found: %s", toolCall.Name))
		m.ToolCallID = toolCall.ID
		return m, nil
	}

	resp, err := fn(ctx, toolCall.Arguments)
	if err != nil {
		return nil, err
	}

	m := agent.NewContentMessage(agent.RoleTool, resp)
	m.ToolCallID = toolCall.ID

	return m, nil
}

func (f *Tools) CompletionFunc(nextStep agent.CompletionFunc) agent.CompletionFunc {
	return func(ctx context.Context, msgs []*agent.Message, tdfs []agent.ToolDef) (*agent.Message, error) {
		// Find the first unexecuted tool call
		if toolCall := f.findUnexecutedToolCall(msgs); toolCall != nil {
			return f.call(ctx, toolCall)
		}

		nfns := make([]agent.ToolDef, 0, len(tdfs)+len(f.defs))
		nfns = append(nfns, tdfs...)
		nfns = append(nfns, f.defs...)

		return nextStep(ctx, msgs, nfns)
	}
}

// findUnexecutedToolCall searches through the conversation to find tool calls that haven't been executed yet
func (f *Tools) findUnexecutedToolCall(msgs []*agent.Message) *agent.ToolCall {
	// Track executed tool calls by their IDs
	executedCallIDs := make(map[string]bool)
	
	// First pass: collect all executed tool call IDs
	for _, msg := range msgs {
		if msg.Role == agent.RoleTool && msg.ToolCallID != "" {
			executedCallIDs[msg.ToolCallID] = true
		}
	}
	
	// Second pass: find unexecuted tool calls
	for _, msg := range msgs {
		if msg.Role == agent.RoleAssistant && msg.HasToolCalls() {
			for _, toolCall := range msg.ToolCalls {
				if !executedCallIDs[toolCall.ID] {
					return &toolCall
				}
			}
		}
	}
	
	return nil
}

func New() *Tools {
	return &Tools{
		fns:  make(map[string]agent.Tool),
		defs: make([]agent.ToolDef, 0),
	}
}

func NewToolsFromTools(fs *Tools) *Tools {
	nfs := New()
	nfs.AddTools(fs)

	return nfs
}

func WithTools(f *Tools) agent.Option {
	return agent.WithMiddleware(f.CompletionFunc)
}
