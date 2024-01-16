package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilter(t *testing.T) {
	var deliveredMsgs []*Message

	// Create a mock completion function
	mockFn := func(ctx context.Context, msgs []*Message, fns []FunctionDef) (*Message, error) {
		deliveredMsgs = msgs[:]

		reply := NewContentMessage(RoleAssistant, "why hello there")
		return reply, nil
	}

	// Create assistant with mock
	a := New(mockFn,
		WithFilter(func(ctx context.Context, msgs []*Message) ([]*Message, error) {
			// Drop the first message
			return msgs[1:], nil
		}),
		WithFilter(func(ctx context.Context, msgs []*Message) ([]*Message, error) {
			// Drop the last message
			return msgs[0 : len(msgs)-1], nil
		}),
	)

	// Add a message
	a.
		Add(RoleUser, "Hello").
		Add(RoleAssistant, "why hello there").
		Add(RoleUser, "still talking")

	// Step through messages
	_, err := a.Step(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, len(deliveredMsgs))
}

func TestFilterError(t *testing.T) {
	var deliveredMsgs []*Message

	// Create a mock completion function
	mockFn := func(ctx context.Context, msgs []*Message, fns []FunctionDef) (*Message, error) {
		deliveredMsgs = msgs[:]

		reply := NewContentMessage(RoleAssistant, "why hello there")
		return reply, nil
	}

	// Create assistant with mock
	a := New(mockFn,
		WithFilter(func(ctx context.Context, msgs []*Message) ([]*Message, error) {
			return nil, errors.New("first filter")
		}),
		WithFilter(func(ctx context.Context, msgs []*Message) ([]*Message, error) {
			return nil, errors.New("second filter")
		}))

	// Add a message
	a.Add(RoleUser, "Hello")
	_, err := a.Step(context.Background())
	require.EqualError(t, err, "filter failed: second filter")

	assert.Equal(t, 0, len(deliveredMsgs))
}
