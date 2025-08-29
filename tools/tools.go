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

func (f *Tools) call(ctx context.Context, name, arguments string) (*agent.Message, error) {
	fn, ok := f.fns[name]
	if !ok {
		m := agent.NewContentMessage(agent.RoleTool, fmt.Sprintf("tool not found: %s", name))
		m.SetLegacyToolCall(name, arguments)
		return m, nil
	}

	resp, err := fn(ctx, arguments)
	if err != nil {
		return nil, err
	}

	m := agent.NewContentMessage(agent.RoleTool, resp)
	m.SetLegacyToolCall(name, arguments)

	return m, nil
}

func (f *Tools) CompletionFunc(nextStep agent.CompletionFunc) agent.CompletionFunc {
	return func(ctx context.Context, msgs []*agent.Message, tdfs []agent.ToolDef) (*agent.Message, error) {
		if len(msgs) > 0 {
			lastMsg := msgs[len(msgs)-1]
			if lastMsg.Role == agent.RoleAssistant && lastMsg.HasToolCalls() {
				if toolCall := lastMsg.GetFirstToolCall(); toolCall != nil {
					return f.call(ctx, toolCall.Name, toolCall.Arguments)
				}
			}
		}

		nfns := make([]agent.ToolDef, 0, len(tdfs)+len(f.defs))
		nfns = append(nfns, tdfs...)
		nfns = append(nfns, f.defs...)

		return nextStep(ctx, msgs, nfns)
	}
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
