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
	msg.Reasoning = &Reasoning{
		Content:          "Let me think about this step by step...",
		EncryptedContent: "encrypted-reasoning-blob",
		Summaries:        []string{"Considered light physics", "Analyzed wavelengths"},
	}

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
	require.NotNil(t, importedMsg.Reasoning)
	assert.Equal(t, "Let me think about this step by step...", importedMsg.Reasoning.Content)
	assert.Equal(t, "encrypted-reasoning-blob", importedMsg.Reasoning.EncryptedContent)
	assert.Equal(t, []string{"Considered light physics", "Analyzed wavelengths"}, importedMsg.Reasoning.Summaries)
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
	assert.Nil(t, importedMsg.Reasoning)
}
