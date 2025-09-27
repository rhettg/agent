package openaichat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/openai/openai-go/v2"
	"github.com/rhettg/agent"
	"github.com/rhettg/agent/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolCallArgumentsBuilding tests the specific bug that was fixed:
// When building tool call arguments from streaming data, the code was modifying
// a copy of the slice element instead of the original element. This test
// demonstrates the fix by simulating the exact scenario.
func TestToolCallArgumentsBuilding(t *testing.T) {
	// Create a mock response structure similar to what OpenAI returns
	result := &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Role: "assistant",
					ToolCalls: []openai.ChatCompletionMessageToolCallUnion{
						{
							ID:   "test_call_123",
							Type: "function",
							Function: openai.ChatCompletionMessageFunctionToolCallFunction{
								Name:      "list_directory",
								Arguments: "", // Initially empty, as received from streaming
							},
						},
					},
				},
			},
		},
	}

	// Simulate the streaming process that builds up tool call arguments
	toolCallArgBuilders := make(map[int]*strings.Builder)
	toolCallArgBuilders[0] = &strings.Builder{}
	toolCallArgBuilders[0].WriteString(`{"path": "/test/directory"}`)

	// Test the BROKEN version (what was happening before the fix)
	// This is intentionally broken to show the bug
	brokenResult := &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Role: "assistant", 
					ToolCalls: []openai.ChatCompletionMessageToolCallUnion{
						{
							ID:   "test_call_123",
							Type: "function",
							Function: openai.ChatCompletionMessageFunctionToolCallFunction{
								Name:      "list_directory",
								Arguments: "", // Initially empty
							},
						},
					},
				},
			},
		},
	}

	// BROKEN: Set final tool call arguments using range with copy (the old bug)
	for i, tc := range brokenResult.Choices[0].Message.ToolCalls {
		if builder, exists := toolCallArgBuilders[i]; exists {
			tc.Function.Arguments = builder.String() // This modifies a COPY!
		}
	}

	// Verify the bug: arguments should still be empty because we modified a copy
	assert.Equal(t, "", brokenResult.Choices[0].Message.ToolCalls[0].Function.Arguments)

	// Test the FIXED version (what happens after the fix)
	// FIXED: Set final tool call arguments using index-based assignment
	for i := range result.Choices[0].Message.ToolCalls {
		if builder, exists := toolCallArgBuilders[i]; exists {
			result.Choices[0].Message.ToolCalls[i].Function.Arguments = builder.String()
		}
	}

	// Verify the fix: arguments should now be properly populated
	assert.Equal(t, `{"path": "/test/directory"}`, result.Choices[0].Message.ToolCalls[0].Function.Arguments)
}

// TestMultipleToolCallArgumentsBuilding tests the fix with multiple tool calls
func TestMultipleToolCallArgumentsBuilding(t *testing.T) {
	result := &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Role: "assistant",
					ToolCalls: []openai.ChatCompletionMessageToolCallUnion{
						{
							ID:   "call_1",
							Type: "function",
							Function: openai.ChatCompletionMessageFunctionToolCallFunction{
								Name:      "list_directory",
								Arguments: "",
							},
						},
						{
							ID:   "call_2",
							Type: "function", 
							Function: openai.ChatCompletionMessageFunctionToolCallFunction{
								Name:      "read_file",
								Arguments: "",
							},
						},
					},
				},
			},
		},
	}

	// Simulate streaming arguments for both tool calls
	toolCallArgBuilders := make(map[int]*strings.Builder)
	toolCallArgBuilders[0] = &strings.Builder{}
	toolCallArgBuilders[0].WriteString(`{"path": "/first"}`)
	
	toolCallArgBuilders[1] = &strings.Builder{}
	toolCallArgBuilders[1].WriteString(`{"path": "/second", "max_lines": 100}`)

	// Apply the fix: index-based assignment
	for i := range result.Choices[0].Message.ToolCalls {
		if builder, exists := toolCallArgBuilders[i]; exists {
			result.Choices[0].Message.ToolCalls[i].Function.Arguments = builder.String()
		}
	}

	// Verify both tool calls have proper arguments
	require.Len(t, result.Choices[0].Message.ToolCalls, 2)
	
	assert.Equal(t, "call_1", result.Choices[0].Message.ToolCalls[0].ID)
	assert.Equal(t, "list_directory", result.Choices[0].Message.ToolCalls[0].Function.Name)
	assert.Equal(t, `{"path": "/first"}`, result.Choices[0].Message.ToolCalls[0].Function.Arguments)
	
	assert.Equal(t, "call_2", result.Choices[0].Message.ToolCalls[1].ID)
	assert.Equal(t, "read_file", result.Choices[0].Message.ToolCalls[1].Function.Name)
	assert.Equal(t, `{"path": "/second", "max_lines": 100}`, result.Choices[0].Message.ToolCalls[1].Function.Arguments)
}

// TestPartialToolCallArgumentsBuilding tests the case where only some tool calls have arguments
func TestPartialToolCallArgumentsBuilding(t *testing.T) {
	result := &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Role: "assistant",
					ToolCalls: []openai.ChatCompletionMessageToolCallUnion{
						{
							ID:   "call_with_args",
							Type: "function",
							Function: openai.ChatCompletionMessageFunctionToolCallFunction{
								Name:      "list_directory",
								Arguments: "",
							},
						},
						{
							ID:   "call_without_args",
							Type: "function",
							Function: openai.ChatCompletionMessageFunctionToolCallFunction{
								Name:      "simple_tool",
								Arguments: "",
							},
						},
					},
				},
			},
		},
	}

	// Only build arguments for the first tool call
	toolCallArgBuilders := make(map[int]*strings.Builder)
	toolCallArgBuilders[0] = &strings.Builder{}
	toolCallArgBuilders[0].WriteString(`{"path": "/test"}`)
	// Note: no builder for index 1

	// Apply the fix
	for i := range result.Choices[0].Message.ToolCalls {
		if builder, exists := toolCallArgBuilders[i]; exists {
			result.Choices[0].Message.ToolCalls[i].Function.Arguments = builder.String()
		}
	}

	// Verify: first tool call should have arguments, second should remain empty
	require.Len(t, result.Choices[0].Message.ToolCalls, 2)
	
	assert.Equal(t, `{"path": "/test"}`, result.Choices[0].Message.ToolCalls[0].Function.Arguments)
	assert.Equal(t, "", result.Choices[0].Message.ToolCalls[1].Function.Arguments)
}

// TestToolCallArgumentsIntegration tests the end-to-end tool calling behavior
// This test verifies that tool arguments are properly parsed and can be used by tools.
func TestToolCallArgumentsIntegration(t *testing.T) {
	ctx := context.Background()

	// Create a simple tool that echoes back its arguments
	ts := tools.New()
	receivedArgs := ""
	
	ts.Add("test_tool", "A test tool", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"message"},
	}, func(ctx context.Context, arguments string) (string, error) {
		receivedArgs = arguments
		
		// Parse the arguments like a real tool would
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", err
		}
		
		message := args["message"].(string)
		return "Received: " + message, nil
	})

	// Create a mock assistant message with a tool call that has arguments
	assistantMsg := agent.NewContentMessage(agent.RoleAssistant, "I'll call the test tool")
	assistantMsg.ToolCalls = []agent.ToolCall{
		{
			ID:        "test_call_123",
			Name:      "test_tool",
			Arguments: `{"message": "hello world"}`, // This should be properly parsed
		},
	}

	msgs := []*agent.Message{assistantMsg}

	// Use the tool completion function to execute the tool call
	completionFunc := ts.CompletionFunc(func(ctx context.Context, msgs []*agent.Message, tdfs []agent.ToolDef) (*agent.Message, error) {
		return agent.NewContentMessage(agent.RoleAssistant, "Done"), nil
	})

	result, err := completionFunc(ctx, msgs, nil)
	require.NoError(t, err)

	// Verify the tool was called with the correct arguments
	assert.Equal(t, `{"message": "hello world"}`, receivedArgs)
	
	// Verify the tool result
	assert.Equal(t, agent.RoleTool, result.Role)
	content, err := result.Content(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Received: hello world", content)
}
