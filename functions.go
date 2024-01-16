package agent

import (
	"context"
	"fmt"

	"github.com/rakyll/openai-go/chat"
)

type Function func(context.Context, string) (string, error)

type FunctionDef struct {
	Name        string
	Description string

	// TODO: decouple this from openai
	Parameters chat.Schema
	fn         func(context.Context, string) (string, error)
}

type FunctionSet struct {
	fns       map[string]Function
	functions []FunctionDef
}

func (f *FunctionSet) Add(name, description string, parameters chat.Schema, fn Function) {
	def := FunctionDef{
		Name:        name,
		Description: description,
		Parameters:  parameters,
		fn:          fn,
	}

	f.functions = append(f.functions, def)
	f.fns[name] = fn
}

func (f *FunctionSet) AddFunctionSet(fs *FunctionSet) {
	for _, def := range fs.functions {
		f.Add(def.Name, def.Description, def.Parameters, def.fn)
	}
}

func (f *FunctionSet) call(ctx context.Context, name, arguments string) (*Message, error) {
	fn, ok := f.fns[name]
	if !ok {
		m := NewContentMessage(RoleFunction, fmt.Sprintf("function not found: %s", name))
		m.FunctionCallName = name
		return m, nil
	}

	resp, err := fn(ctx, arguments)
	if err != nil {
		return nil, err
	}

	m := NewContentMessage(RoleFunction, resp)
	m.FunctionCallName = name

	return m, nil
}

func (f *FunctionSet) CompletionFunc(nextStep CompletionFunc) CompletionFunc {
	return func(ctx context.Context, msgs []*Message, fns []FunctionDef) (*Message, error) {
		if len(msgs) > 0 {
			lastMsg := msgs[len(msgs)-1]
			if lastMsg.Role == RoleAssistant && lastMsg.FunctionCallName != "" {
				return f.call(ctx, lastMsg.FunctionCallName, lastMsg.FunctionCallArgs)
			}
		}

		nfns := make([]FunctionDef, 0, len(fns)+len(f.functions))
		nfns = append(nfns, fns...)
		nfns = append(nfns, f.functions...)

		return nextStep(ctx, msgs, nfns)
	}
}

func NewFunctionSet() *FunctionSet {
	return &FunctionSet{
		fns:       make(map[string]Function),
		functions: make([]FunctionDef, 0),
	}
}

func NewFunctionSetFromFunctionSet(fs *FunctionSet) *FunctionSet {
	nfs := NewFunctionSet()
	nfs.AddFunctionSet(fs)

	return nfs
}

func WithFunctionSet(f *FunctionSet) Option {
	return WithMiddleware(f.CompletionFunc)
}
