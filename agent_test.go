package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssistant_Step(t *testing.T) {
	// Create a mock completion function
	mockFn := func(ctx context.Context, msgs []*Message, fns []FunctionDef) (*Message, error) {
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

func TestAssistant_Filter(t *testing.T) {
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

func TestAssistant_FilterError(t *testing.T) {
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

func TestAssistant_Check(t *testing.T) {
	var deliveredMsgs []*Message

	// Create a mock completion function
	mockFn := func(ctx context.Context, msgs []*Message, fns []FunctionDef) (*Message, error) {
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

func TestAssistant_CheckError(t *testing.T) {
	var deliveredMsgs []*Message

	// Create a mock completion function
	mockFn := func(ctx context.Context, msgs []*Message, fns []FunctionDef) (*Message, error) {
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
