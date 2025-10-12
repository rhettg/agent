package tools

import (
	"context"
	"fmt"

	"github.com/rhettg/agent"
)

// ToolInvokeFunc executes a single tool call and returns the result message
type ToolInvokeFunc func(context.Context, *agent.ToolCall) (*agent.Message, error)

// ToolMiddleware wraps a ToolInvokeFunc to add cross-cutting concerns
type ToolMiddleware func(next ToolInvokeFunc) ToolInvokeFunc

// Option configures a Tools instance
type Option func(*Tools)

type Tools struct {
	fns  map[string]agent.Tool
	afns map[string]agent.AttributesTool
	defs []agent.ToolDef
	mws  []ToolMiddleware
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

func (f *Tools) AddAttributesTool(name, description string, parameters any, fn agent.AttributesTool) {
	def := agent.ToolDef{
		Name:        name,
		Description: description,
		Parameters:  parameters,
	}

	f.defs = append(f.defs, def)
	f.afns[name] = fn
}

func (f *Tools) AddTools(fs *Tools) {
	for _, def := range fs.defs {
		if fn, ok := fs.afns[def.Name]; ok {
			f.AddAttributesTool(def.Name, def.Description, def.Parameters, fn)
		} else {
			f.Add(def.Name, def.Description, def.Parameters, f.fns[def.Name])
		}
	}
}

// WithMiddleware adds middleware to the tool execution stack
func WithMiddleware(mw ToolMiddleware) Option {
	return func(t *Tools) {
		t.mws = append(t.mws, mw)
	}
}

// runTool executes the actual tool function without middleware
func (f *Tools) runTool(ctx context.Context, toolCall *agent.ToolCall) (*agent.Message, error) {
	if fn, ok := f.fns[toolCall.Name]; ok {
		resp, err := fn(ctx, toolCall.Arguments)
		if err != nil {
			return nil, err
		}

		m := agent.NewContentMessage(agent.RoleTool, resp)
		m.ToolCallID = toolCall.ID

		return m, nil
	}

	if fn, ok := f.afns[toolCall.Name]; ok {
		attrs := agent.Attributes{}
		resp, err := fn(ctx, attrs, toolCall.Arguments)
		if err != nil {
			return nil, err
		}

		m := agent.NewContentMessage(agent.RoleTool, resp)
		m.ToolCallID = toolCall.ID
		m.Attributes = attrs

		return m, nil
	}

	m := agent.NewContentMessage(agent.RoleTool, fmt.Sprintf("tool not found: %s", toolCall.Name))
	m.ToolCallID = toolCall.ID

	return m, nil
}

// call wraps runTool with the middleware stack
func (f *Tools) call(ctx context.Context, toolCall *agent.ToolCall) (*agent.Message, error) {
	handler := f.runTool
	// Compose middleware onion from last to first
	for i := len(f.mws) - 1; i >= 0; i-- {
		handler = f.mws[i](handler)
	}
	return handler(ctx, toolCall)
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
			for i := range msg.ToolCalls {
				tc := &msg.ToolCalls[i]
				if !executedCallIDs[tc.ID] {
					return tc
				}
			}
		}
	}

	return nil
}

func New(opts ...Option) *Tools {
	t := &Tools{
		fns:  make(map[string]agent.Tool),
		defs: make([]agent.ToolDef, 0),
		afns: make(map[string]agent.AttributesTool),
		mws:  make([]ToolMiddleware, 0),
	}
	
	for _, opt := range opts {
		opt(t)
	}
	
	return t
}

func NewToolsFromTools(fs *Tools) *Tools {
	nfs := New()
	nfs.AddTools(fs)

	return nfs
}

func WithTools(f *Tools) agent.Option {
	return agent.WithMiddleware(f.CompletionFunc)
}
