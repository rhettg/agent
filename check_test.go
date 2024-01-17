package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck(t *testing.T) {
	var deliveredMsgs []*Message

	// Create a mock completion function
	mockFn := func(ctx context.Context, msgs []*Message, fns []ToolDef) (*Message, error) {
		deliveredMsgs = msgs[:]

		reply := NewContentMessage(RoleAssistant, "why hello there")
		return reply, nil
	}

	var check bool
	var checkCheck bool

	// Create assistant with mock
	a := New(mockFn,
		WithCheck(func(ctx context.Context, m *Message) error {
			assert.Equal(t, m.Role, RoleAssistant)
			check = true
			return nil
		}),
		WithCheck(func(ctx context.Context, m *Message) error {
			checkCheck = true
			return nil
		}),
	)

	// Add a message
	a.Add(RoleUser, "Hello")

	_, err := a.Step(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, len(deliveredMsgs))
	assert.True(t, check)
	assert.True(t, checkCheck)
}

func TestCheckError(t *testing.T) {
	var deliveredMsgs []*Message

	// Create a mock completion function
	mockFn := func(ctx context.Context, msgs []*Message, fns []ToolDef) (*Message, error) {
		deliveredMsgs = msgs[:]

		reply := NewContentMessage(RoleAssistant, "why hello there")
		return reply, nil
	}

	// Create assistant with mock
	a := New(mockFn,
		WithCheck(func(ctx context.Context, m *Message) error {
			assert.Equal(t, m.Role, RoleAssistant)
			return errors.New("first check")
		}),
		WithCheck(func(ctx context.Context, m *Message) error {
			return errors.New("second check")
		}),
	)

	// Add a message
	a.Add(RoleUser, "Hello")

	_, err := a.Step(context.Background())
	require.EqualError(t, err, "check failed: first check")

	assert.Equal(t, 1, len(deliveredMsgs))
}
