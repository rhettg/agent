package agent

import (
	"context"
)

type Role string

const (
	RoleSystem    = Role("system")
	RoleUser      = Role("user")
	RoleAssistant = Role("assistant")
	RoleTool      = Role("tool")
)

type CompletionFunc func(context.Context, []*Message, []ToolDef) (*Message, error)
type MiddlewareFunc func(nextStep CompletionFunc) CompletionFunc

type Tool func(context.Context, string) (string, error)

// AttributesTool is a tool that, in addition to accepting and returning a string, can set Message Attributes
// This is helpful for tools that need to interact with the larger Agent loop in
// a way that a simple string response can't support
type AttributesTool func(context.Context, Attributes, string) (string, error)

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type ToolDef struct {
	Name        string
	Description string

	Parameters any
}
