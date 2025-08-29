package tools

import (
	"context"
	"testing"

	"github.com/rhettg/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var EmptyParameters = map[string]any{
	"type":       "object",
	"properties": map[string]any{},
}

func TestTools(t *testing.T) {
	ctx := context.Background()

	ts := New()

	ts.Add("hello", "Say hello", EmptyParameters, func(ctx context.Context, args string) (string, error) {
		return "Hello world!", nil
	})

	toolCall := &agent.ToolCall{ID: "test1", Name: "hello", Arguments: "{}"}
	msg, err := ts.call(ctx, toolCall)
	require.NoError(t, err)

	content, err := msg.Content(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello world!", content)

	invalidToolCall := &agent.ToolCall{ID: "test2", Name: "invalid", Arguments: "{}"}
	r, err := ts.call(ctx, invalidToolCall)
	rc, _ := r.Content(ctx)
	assert.Equal(t, "tool not found: invalid", rc)
	assert.NoError(t, err)
}

func hello(ctx context.Context, args string) (string, error) {
	return "Hello world!", nil
}

func TestTools_CompletionFunc(t *testing.T) {
	ctx := context.Background()

	ts := New()

	ts.Add("hello", "say hello", EmptyParameters, func(ctx context.Context, args string) (string, error) {
		resp, err := hello(ctx, args)
		if err != nil {
			return "", err
		}

		return resp, nil
	})

	toolCall := &agent.ToolCall{ID: "test3", Name: "hello", Arguments: "{}"}
	msg, err := ts.call(ctx, toolCall)
	require.NoError(t, err)

	content, err := msg.Content(ctx)
	require.NoError(t, err)
	assert.Contains(t, content, "Hello world")

	invalidToolCall := &agent.ToolCall{ID: "test4", Name: "worldDomination", Arguments: "{}"}
	msg, err = ts.call(ctx, invalidToolCall)
	require.NoError(t, err)
	content, _ = msg.Content(ctx)
	require.Contains(t, content, "tool not found: worldDomination")
}

func TestMultipleToolCalls(t *testing.T) {
	ctx := context.Background()
	ts := New()

	ts.Add("add", "Add two numbers", EmptyParameters, func(ctx context.Context, args string) (string, error) {
		return "Sum: 7", nil
	})
	
	ts.Add("multiply", "Multiply two numbers", EmptyParameters, func(ctx context.Context, args string) (string, error) {
		return "Product: 12", nil
	})

	// Create an assistant message with multiple tool calls
	assistantMsg := agent.NewContentMessage(agent.RoleAssistant, "I'll help you with both calculations.")
	assistantMsg.ToolCalls = []agent.ToolCall{
		{ID: "call_1", Name: "add", Arguments: `{"a": 3, "b": 4}`},
		{ID: "call_2", Name: "multiply", Arguments: `{"a": 3, "b": 4}`},
	}

	msgs := []*agent.Message{assistantMsg}

	// First call should execute the first unexecuted tool call
	completionFunc := ts.CompletionFunc(func(ctx context.Context, msgs []*agent.Message, tdfs []agent.ToolDef) (*agent.Message, error) {
		t.Error("Should not reach next step")
		return nil, nil
	})

	result1, err := completionFunc(ctx, msgs, nil)
	require.NoError(t, err)
	assert.Equal(t, agent.RoleTool, result1.Role)
	assert.Equal(t, "call_1", result1.ToolCallID)
	content1, _ := result1.Content(ctx)
	assert.Equal(t, "Sum: 7", content1)

	// Add the first tool result to messages
	msgs = append(msgs, result1)

	// Second call should execute the remaining tool call
	result2, err := completionFunc(ctx, msgs, nil)
	require.NoError(t, err)
	assert.Equal(t, agent.RoleTool, result2.Role)
	assert.Equal(t, "call_2", result2.ToolCallID)
	content2, _ := result2.Content(ctx)
	assert.Equal(t, "Product: 12", content2)

	// Add the second tool result to messages  
	msgs = append(msgs, result2)

	// Third call should pass to next step since all tools are executed
	nextStepCalled := false
	completionFunc = ts.CompletionFunc(func(ctx context.Context, msgs []*agent.Message, tdfs []agent.ToolDef) (*agent.Message, error) {
		nextStepCalled = true
		return agent.NewContentMessage(agent.RoleAssistant, "All done!"), nil
	})

	result3, err := completionFunc(ctx, msgs, nil)
	require.NoError(t, err)
	assert.True(t, nextStepCalled)
	content3, _ := result3.Content(ctx)
	assert.Equal(t, "All done!", content3)
}
