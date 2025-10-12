package tools

import (
	"context"
	"testing"
	"time"

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

func TestAttributesTool(t *testing.T) {
	ctx := context.Background()

	ts := New()

	ts.AddAttributesTool("hello", "Say hello", EmptyParameters, func(ctx context.Context, attrs agent.Attributes, args string) (string, error) {
		attrs.Tag("test")
		return "Hello world!", nil
	})

	toolCall := &agent.ToolCall{ID: "test1", Name: "hello", Arguments: "{}"}
	msg, err := ts.call(ctx, toolCall)
	require.NoError(t, err)

	content, err := msg.Content(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello world!", content)
	assert.True(t, msg.HasTag("test"))
}

func TestToolMiddleware(t *testing.T) {
	ctx := context.Background()

	// Add middleware that logs tool calls
	var called []string
	ts := New(WithMiddleware(func(next ToolInvokeFunc) ToolInvokeFunc {
		return func(ctx context.Context, call *agent.ToolCall) (*agent.Message, error) {
			called = append(called, "before:"+call.Name)
			msg, err := next(ctx, call)
			called = append(called, "after:"+call.Name)
			return msg, err
		}
	}))

	ts.Add("hello", "Say hello", EmptyParameters, func(ctx context.Context, args string) (string, error) {
		return "Hello world!", nil
	})

	toolCall := &agent.ToolCall{ID: "test1", Name: "hello", Arguments: "{}"}
	msg, err := ts.call(ctx, toolCall)
	require.NoError(t, err)

	content, err := msg.Content(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello world!", content)
	assert.Equal(t, []string{"before:hello", "after:hello"}, called)
}

func TestToolMiddlewareComposition(t *testing.T) {
	ctx := context.Background()

	// Add multiple middleware layers
	var order []string
	ts := New(
		WithMiddleware(func(next ToolInvokeFunc) ToolInvokeFunc {
			return func(ctx context.Context, call *agent.ToolCall) (*agent.Message, error) {
				order = append(order, "outer:before")
				msg, err := next(ctx, call)
				order = append(order, "outer:after")
				return msg, err
			}
		}),
		WithMiddleware(func(next ToolInvokeFunc) ToolInvokeFunc {
			return func(ctx context.Context, call *agent.ToolCall) (*agent.Message, error) {
				order = append(order, "inner:before")
				msg, err := next(ctx, call)
				order = append(order, "inner:after")
				return msg, err
			}
		}),
	)

	ts.Add("greet", "Greet someone", EmptyParameters, func(ctx context.Context, args string) (string, error) {
		return "Greetings!", nil
	})

	toolCall := &agent.ToolCall{ID: "test1", Name: "greet", Arguments: "{}"}
	msg, err := ts.call(ctx, toolCall)
	require.NoError(t, err)

	content, err := msg.Content(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Greetings!", content)

	// Middleware should execute in onion order: outer before, inner before, tool, inner after, outer after
	assert.Equal(t, []string{
		"outer:before",
		"inner:before",
		"inner:after",
		"outer:after",
	}, order)
}

func TestToolMiddlewareWithAttributes(t *testing.T) {
	ctx := context.Background()

	// Middleware that adds attributes to the message
	ts := New(WithMiddleware(func(next ToolInvokeFunc) ToolInvokeFunc {
		return func(ctx context.Context, call *agent.ToolCall) (*agent.Message, error) {
			msg, err := next(ctx, call)
			if err == nil && msg != nil {
				if msg.Attributes == nil {
					msg.Attributes = agent.Attributes{}
				}
				msg.Attributes["middleware"] = "was-here"
			}
			return msg, err
		}
	}))

	ts.AddAttributesTool("hello", "Say hello", EmptyParameters, func(ctx context.Context, attrs agent.Attributes, args string) (string, error) {
		attrs.Tag("from-tool")
		return "Hello!", nil
	})

	toolCall := &agent.ToolCall{ID: "test1", Name: "hello", Arguments: "{}"}
	msg, err := ts.call(ctx, toolCall)
	require.NoError(t, err)

	assert.True(t, msg.HasTag("from-tool"))
	assert.Equal(t, "was-here", msg.Attributes["middleware"])
}

func TestToolCallNotifier(t *testing.T) {
	ctx := context.Background()

	// Create a buffered channel to receive events
	eventCh := make(chan ToolCallEvent, 10)

	ts := New(WithMiddleware(ToolCallNotifier(eventCh)))
	ts.Add("greet", "Greet someone", EmptyParameters, func(ctx context.Context, args string) (string, error) {
		return "Hello!", nil
	})

	toolCall := &agent.ToolCall{ID: "test1", Name: "greet", Arguments: "{}"}
	msg, err := ts.call(ctx, toolCall)
	require.NoError(t, err)

	// Check the event was sent
	select {
	case event := <-eventCh:
		assert.Equal(t, "test1", event.ToolCall.ID)
		assert.Equal(t, "greet", event.ToolCall.Name)
		assert.NotNil(t, event.Message)
		assert.NoError(t, event.Error)
		assert.False(t, event.Started.IsZero())
		assert.False(t, event.Finished.IsZero())
		assert.True(t, event.Finished.After(event.Started) || event.Finished.Equal(event.Started))

		content, _ := event.Message.Content(ctx)
		assert.Equal(t, "Hello!", content)
	default:
		t.Fatal("Expected event on channel")
	}

	// Verify message is correct
	content, _ := msg.Content(ctx)
	assert.Equal(t, "Hello!", content)
}

func TestToolCallStartNotifier(t *testing.T) {
	ctx := context.Background()

	toolCallCh := make(chan agent.ToolCall, 1)

	ts := New(WithMiddleware(ToolCallStartNotifier(toolCallCh)))
	ts.Add("greet", "Greet someone", EmptyParameters, func(ctx context.Context, args string) (string, error) {
		return "Hello!", nil
	})

	toolCall := &agent.ToolCall{ID: "test1", Name: "greet", Arguments: "{}"}

	msg, err := ts.call(ctx, toolCall)
	require.NoError(t, err)
	require.Equal(t, "test1", msg.ToolCallID)

	// Receive from channel with timeout
	select {
	case call := <-toolCallCh:
		assert.Equal(t, "test1", call.ID)
		assert.Equal(t, "greet", call.Name)
		assert.Equal(t, "{}", call.Arguments)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for tool call on channel")
	}
}

func TestToolCallNotifierWithError(t *testing.T) {
	ctx := context.Background()

	eventCh := make(chan ToolCallEvent, 10)

	ts := New(WithMiddleware(ToolCallNotifier(eventCh)))
	ts.Add("fail", "Fail intentionally", EmptyParameters, func(ctx context.Context, args string) (string, error) {
		return "", assert.AnError
	})

	toolCall := &agent.ToolCall{ID: "test1", Name: "fail", Arguments: "{}"}
	msg, err := ts.call(ctx, toolCall)
	require.Error(t, err)
	assert.Nil(t, msg)

	// Check the error event was sent
	select {
	case event := <-eventCh:
		assert.Equal(t, "test1", event.ToolCall.ID)
		assert.Equal(t, "fail", event.ToolCall.Name)
		assert.Nil(t, event.Message)
		assert.Error(t, event.Error)
		assert.Equal(t, assert.AnError, event.Error)
	default:
		t.Fatal("Expected error event on channel")
	}
}
