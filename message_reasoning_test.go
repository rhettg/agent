package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYAMLExportImportWithReasoning(t *testing.T) {
	// Create a message with reasoning
	msg := NewContentMessage(RoleAssistant, "The sky is blue because of light scattering.")
	msg.ReasoningContent = "Let me think about this step by step..."
	msg.ReasoningEncryptedContent = "encrypted-reasoning-blob"
	msg.ReasoningSummaries = []string{"Considered light physics", "Analyzed wavelengths"}

	messages := []*Message{msg}

	// Export to YAML
	yamlStr, err := ExportMessagesToYAML(context.Background(), messages)
	require.NoError(t, err)
	assert.Contains(t, yamlStr, "Reasoning:")
	assert.Contains(t, yamlStr, "Let me think about this step by step...")
	assert.Contains(t, yamlStr, "encrypted-reasoning-blob")
	assert.Contains(t, yamlStr, "Considered light physics")

	// Import from YAML
	importedMessages, err := ImportMessagesFromYAML(yamlStr)
	require.NoError(t, err)
	require.Len(t, importedMessages, 1)

	importedMsg := importedMessages[0]
	assert.Equal(t, RoleAssistant, importedMsg.Role)

	content, err := importedMsg.Content(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "The sky is blue because of light scattering.", content)

	// Verify reasoning was preserved
	assert.Equal(t, "Let me think about this step by step...", importedMsg.ReasoningContent)
	assert.Equal(t, "encrypted-reasoning-blob", importedMsg.ReasoningEncryptedContent)
	assert.Equal(t, []string{"Considered light physics", "Analyzed wavelengths"}, importedMsg.ReasoningSummaries)
}

func TestYAMLExportImportWithoutReasoning(t *testing.T) {
	// Create a message without reasoning
	msg := NewContentMessage(RoleUser, "Hello!")
	messages := []*Message{msg}

	// Export to YAML
	yamlStr, err := ExportMessagesToYAML(context.Background(), messages)
	require.NoError(t, err)
	assert.NotContains(t, yamlStr, "Reasoning:")

	// Import from YAML
	importedMessages, err := ImportMessagesFromYAML(yamlStr)
	require.NoError(t, err)
	require.Len(t, importedMessages, 1)

	importedMsg := importedMessages[0]
	assert.Equal(t, RoleUser, importedMsg.Role)

	content, err := importedMsg.Content(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Hello!", content)

	// Verify no reasoning
	assert.Empty(t, importedMsg.ReasoningContent)
	assert.Empty(t, importedMsg.ReasoningEncryptedContent)
	assert.Empty(t, importedMsg.ReasoningSummaries)
}

func TestNewFromAgentWithReasoning(t *testing.T) {
	// Create a mock completion function
	mockFn := func(ctx context.Context, msgs []*Message, fns []ToolDef) (*Message, error) {
		return NewContentMessage(RoleAssistant, "response"), nil
	}

	// Create original agent with a message that has reasoning
	original := New(mockFn)
	msg := NewContentMessage(RoleAssistant, "Original message")
	msg.ReasoningContent = "Original reasoning"
	msg.ReasoningSummaries = []string{"Summary 1"}
	original.AddMessage(msg)

	// Create new agent from original
	copy := NewFromAgent(original)

	// Verify messages were copied
	originalMsgs := original.Messages()
	copyMsgs := copy.Messages()
	require.Len(t, copyMsgs, 1)
	require.Len(t, originalMsgs, 1)

	// Verify reasoning was deep copied
	assert.Equal(t, "Original reasoning", copyMsgs[0].ReasoningContent)
	assert.Equal(t, []string{"Summary 1"}, copyMsgs[0].ReasoningSummaries)

	// Verify it's a deep copy (modifying copy doesn't affect original)
	copyMsgs[0].ReasoningContent = "Modified reasoning"
	assert.Equal(t, "Original reasoning", originalMsgs[0].ReasoningContent)
	assert.Equal(t, "Modified reasoning", copyMsgs[0].ReasoningContent)
}
