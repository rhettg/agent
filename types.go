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
type MiddlewareFunc func(nextStep CompletionFunc) CompletionFunc

type Function func(context.Context, string) (string, error)

type FunctionDef struct {
	Name        string
	Description string

	Parameters any
}
