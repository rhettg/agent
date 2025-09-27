package openairesponses

import (
	"context"
	"os"
	"testing"

	"github.com/openai/openai-go/v2/responses"
	"github.com/rhettg/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderOptions(t *testing.T) {
	p := New("test-key", "gpt-4o-2024-08-06",
		WithReasoningEffort("medium"),
		WithReasoningSummary("concise"),
		WithTemperature(0.7),
		WithMaxTokens(1000),
	)

	// Test that the provider was created successfully
	assert.NotNil(t, p)
}

func TestMessageMapping(t *testing.T) {
	p := &provider{
		modelName: "gpt-4o-2024-08-06",
	}

	// Test mapping messages to input items
	msgs := []*agent.Message{
		agent.NewContentMessage(agent.RoleSystem, "You are a helpful assistant."),
		agent.NewContentMessage(agent.RoleUser, "Hello!"),
	}

	inputUnion, err := p.mapMessagesToInputItems(context.Background(), msgs)
	require.NoError(t, err)
	
	// Check that we got an input item list
	items := inputUnion.OfInputItemList
	assert.Len(t, items, 2)

	// The actual structure is complex, so we just verify we got the right number of items
	// TODO: Add more detailed assertions when we understand the exact structure better
}

func TestMessageWithReasoning(t *testing.T) {
	// Test creating a message with reasoning
	msg := agent.NewContentMessage(agent.RoleAssistant, "The sky is blue because...")
	msg.ReasoningContent = "Let me think about this step by step..."
	msg.ReasoningSummaries = []string{"Considered light scattering", "Analyzed wavelengths"}

	// Test that reasoning is preserved
	assert.Equal(t, "Let me think about this step by step...", msg.ReasoningContent)
	assert.Len(t, msg.ReasoningSummaries, 2)
}

func TestMessageCopyWithReasoning(t *testing.T) {
	// Test that NewMessageFromMessage preserves reasoning
	original := agent.NewContentMessage(agent.RoleAssistant, "Original content")
	original.ReasoningContent = "Original reasoning"
	original.ReasoningEncryptedContent = "encrypted-blob"
	original.ReasoningSummaries = []string{"Summary 1", "Summary 2"}

	copy := agent.NewMessageFromMessage(original)

	// Verify reasoning was copied
	assert.Equal(t, original.ReasoningContent, copy.ReasoningContent)
	assert.Equal(t, original.ReasoningEncryptedContent, copy.ReasoningEncryptedContent)
	assert.Equal(t, original.ReasoningSummaries, copy.ReasoningSummaries)

	// Verify it's a deep copy (modifying copy doesn't affect original)
	copy.ReasoningContent = "Modified reasoning"
	assert.NotEqual(t, original.ReasoningContent, copy.ReasoningContent)
}

func TestCompletionWithoutAPIKey(t *testing.T) {
	// Skip this test by default to avoid making live API calls in CI
	if os.Getenv("OPENAI_API_TEST") == "" {
		t.Skip("Skipping live API test. Set OPENAI_API_TEST=1 to run.")
	}
	
	p := New("", "gpt-4o-2024-08-06")

	msgs := []*agent.Message{
		agent.NewContentMessage(agent.RoleUser, "Hello!"),
	}

	// Should return error since no API key is provided
	_, err := p(context.Background(), msgs, nil)
	assert.Error(t, err)
	// The exact error will depend on the OpenAI SDK's validation
}

// TestTemperatureZero tests that temperature can be explicitly set to 0
func TestTemperatureZero(t *testing.T) {
	p := &provider{
		modelName:   "gpt-4o-2024-08-06", 
		temperature: nil, // default is nil (use model default)
	}

	// Test that WithTemperature(0) should actually set temperature to 0
	WithTemperature(0)(p)
	require.NotNil(t, p.temperature)
	assert.Equal(t, 0.0, *p.temperature)

	// Test non-zero temperature
	WithTemperature(0.7)(p)
	require.NotNil(t, p.temperature)
	assert.Equal(t, 0.7, *p.temperature)
}

// TestStreamingEventCorrelation tests potential issues with streaming event correlation
func TestStreamingEventCorrelation(t *testing.T) {
	p := &provider{}
	
	// Create mock events that could cause correlation issues
	var mockItem responses.ResponseOutputItemUnion
	
	// Test delta extraction with empty item (simulating missing output_item.added)
	mockEvent := responses.ResponseStreamEventUnion{
		// In a real scenario, this would be a function_call_arguments.delta event
		Type: "response.function_call_arguments.delta",
	}
	
	delta := p.extractDeltaFromEvent(mockItem, mockEvent)
	
	// This should handle empty item gracefully
	// The current implementation might return empty ToolCallID/Name which could be problematic
	if delta != nil {
		// If we get a delta but ToolCallID/Name are empty, that's a potential issue
		t.Logf("Delta ToolCallID: %q, ToolCallName: %q", delta.ToolCallID, delta.ToolCallName)
		// In real usage, empty IDs/Names would make it impossible for consumers 
		// to correlate deltas with tool calls
		assert.Empty(t, delta.ToolCallID, "Expected empty ToolCallID with unset item")
		assert.Empty(t, delta.ToolCallName, "Expected empty ToolCallName with unset item")
	}
}

// TestToolParametersTypeAssertion tests the tool parameter type assertion
func TestToolParametersTypeAssertion(t *testing.T) {
	p := &provider{modelName: "test"}

	// Test with correct parameters type
	validTool := agent.ToolDef{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
			},
		},
	}

	msgs := []*agent.Message{
		agent.NewContentMessage(agent.RoleUser, "test"),
	}

	// This should not panic
	_, err := p.Completion(context.Background(), msgs, []agent.ToolDef{validTool})
	// We expect an error due to no API key, but not a panic from type assertion
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "invalid parameters type")

	// Test with incorrect parameters type
	invalidTool := agent.ToolDef{
		Name:        "bad_tool",
		Description: "A tool with wrong parameter type",
		Parameters:  "not a map", // This should cause the type assertion to fail gracefully
	}

	_, err = p.Completion(context.Background(), msgs, []agent.ToolDef{invalidTool})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid parameters type")
	assert.Contains(t, err.Error(), "expected map[string]any, got string")
}
