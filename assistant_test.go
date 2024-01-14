package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
func TestPruneMessages(t *testing.T) {
	enc, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Create a new a instance
	a := New(nil)

	// Add some messages to the assistant
	a.AddMessage(RoleSystem, "Hi there!")
	a.AddMessage(RoleUser, "Hello")
	a.AddMessage(RoleAssistant, "How are you?")
	a.AddMessage(RoleUser, "I'm doing well, thanks!")

	et, err := a.EstimateTokens(enc)
	require.NoError(t, err)
	require.Equal(t, 46, et)

	pruned, err := a.PruneMessages(enc, 45)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	require.Equal(t, 39, pruned)
	require.Equal(t, 3, len(a.Messages()))
	require.Equal(t, "I'm doing well, thanks!", a.Messages()[2].Content)

	// Prune harder (only system message should remain)
	pruned, err = a.PruneMessages(enc, 1)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	require.Equal(t, 12, pruned)
	require.Equal(t, 1, len(a.Messages()))
}
*/

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
	a := New(mockFn)

	// Add a message
	a.
		Add(RoleUser, "Hello").
		Add(RoleAssistant, "why hello there").
		Add(RoleUser, "still talking").
		Filter(func(ctx context.Context, msgs []*Message) ([]*Message, error) {
			// Drop the first message
			return msgs[1:], nil
		}).
		Filter(func(ctx context.Context, msgs []*Message) ([]*Message, error) {
			// Drop the last message
			return msgs[0 : len(msgs)-1], nil
		})

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
	a := New(mockFn)

	// Add a message
	a.
		Add(RoleUser, "Hello").
		Filter(func(ctx context.Context, msgs []*Message) ([]*Message, error) {
			return nil, errors.New("first filter")
		}).
		Filter(func(ctx context.Context, msgs []*Message) ([]*Message, error) {
			return nil, errors.New("second filter")
		})

	_, err := a.Step(context.Background())
	require.EqualError(t, err, "filter failed: first filter")

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

	// Create assistant with mock
	a := New(mockFn)

	var check bool
	var checkCheck bool

	// Add a message
	a.
		Add(RoleUser, "Hello").
		Check(func(ctx context.Context, m *Message) error {
			assert.Equal(t, m.Role, RoleAssistant)
			check = true
			return nil
		}).
		Check(func(ctx context.Context, m *Message) error {
			checkCheck = true
			return nil
		})

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
	a := New(mockFn)

	// Add a message
	a.
		Add(RoleUser, "Hello").
		Check(func(ctx context.Context, m *Message) error {
			assert.Equal(t, m.Role, RoleAssistant)
			return errors.New("first check")
		}).
		Check(func(ctx context.Context, m *Message) error {
			return errors.New("second check")
		})

	_, err := a.Step(context.Background())
	require.EqualError(t, err, "check failed: first check")

	assert.Equal(t, 1, len(deliveredMsgs))
}
