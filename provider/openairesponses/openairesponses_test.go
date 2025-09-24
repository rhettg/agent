package openairesponses

import (
	"context"
	"testing"

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
	msg.Reasoning = &agent.Reasoning{
		Content:   "Let me think about this step by step...",
		Summaries: []string{"Considered light scattering", "Analyzed wavelengths"},
	}

	// Test that reasoning is preserved
	assert.NotNil(t, msg.Reasoning)
	assert.Equal(t, "Let me think about this step by step...", msg.Reasoning.Content)
	assert.Len(t, msg.Reasoning.Summaries, 2)
}

func TestMessageCopyWithReasoning(t *testing.T) {
	// Test that NewMessageFromMessage preserves reasoning
	original := agent.NewContentMessage(agent.RoleAssistant, "Original content")
	original.Reasoning = &agent.Reasoning{
		Content:          "Original reasoning",
		EncryptedContent: "encrypted-blob",
		Summaries:        []string{"Summary 1", "Summary 2"},
	}

	copy := agent.NewMessageFromMessage(original)

	// Verify reasoning was copied
	require.NotNil(t, copy.Reasoning)
	assert.Equal(t, original.Reasoning.Content, copy.Reasoning.Content)
	assert.Equal(t, original.Reasoning.EncryptedContent, copy.Reasoning.EncryptedContent)
	assert.Equal(t, original.Reasoning.Summaries, copy.Reasoning.Summaries)

	// Verify it's a deep copy (modifying copy doesn't affect original)
	copy.Reasoning.Content = "Modified reasoning"
	assert.NotEqual(t, original.Reasoning.Content, copy.Reasoning.Content)
}

func TestCompletionWithoutAPIKey(t *testing.T) {
	p := New("", "gpt-4o-2024-08-06")

	msgs := []*agent.Message{
		agent.NewContentMessage(agent.RoleUser, "Hello!"),
	}

	// Should return error since no API key is provided
	_, err := p(context.Background(), msgs, nil)
	assert.Error(t, err)
	// The exact error will depend on the OpenAI SDK's validation
}
