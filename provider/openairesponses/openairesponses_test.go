package openairesponses

import (
	"context"
	"os"
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
