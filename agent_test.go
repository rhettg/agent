package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStep(t *testing.T) {
	// Create a mock completion function
	mockFn := func(ctx context.Context, msgs []*Message, fns []ToolDef) (*Message, error) {
		reply := NewContentMessage(RoleAssistant, "why hello there")
		return reply, nil
	}

	// Create assistant with mock
	a := New(mockFn)

	// Add a message
	a.Add(RoleUser, "Hello")

	// Step through messages
	resp, err := a.Step(context.Background())
	require.NoError(t, err)

	// Validate response
	assert.Equal(t, RoleAssistant, resp.Role)
	content, err := resp.Content(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "why hello there", content)
}
