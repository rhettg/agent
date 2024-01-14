package ollamachat

import (
	"context"
	"testing"

	"github.com/rhettg/agent"
	"github.com/stretchr/testify/require"
)

func TestFormatDialogLlama(t *testing.T) {
	msgs := []*agent.Message{
		agent.NewContentMessage(agent.RoleUser, "Hello!"),
	}

	dialog, err := formatDialogLlama(context.Background(), msgs)
	require.NoError(t, err)

	expected := "[INST] Hello! [/INST]"
	require.Equal(t, expected, dialog)

	// Insert a system message into front of msgs
	systemMsg := agent.NewContentMessage(agent.RoleSystem, "You are a code completion AI designed to seamlessly integrate with surrounding code.")

	msgs = []*agent.Message{systemMsg, msgs[0]}

	dialog, err = formatDialogLlama(context.Background(), msgs)
	require.NoError(t, err)
	expected = "[INST] <<SYS>>\nYou are a code completion AI designed to seamlessly integrate with surrounding code.\n<</SYS>>\n\nHello! [/INST]"
	require.Equal(t, expected, dialog)

	msgs = append(msgs, agent.NewContentMessage(agent.RoleAssistant, "How can I help you?"))
	msgs = append(msgs, agent.NewContentMessage(agent.RoleUser, "Will you be my friend?"))

	dialog, err = formatDialogLlama(context.Background(), msgs)
	require.NoError(t, err)
	expected = "[INST] <<SYS>>\nYou are a code completion AI designed to seamlessly integrate with surrounding code.\n<</SYS>>\n\nHello! [/INST] How can I help you? [INST] Will you be my friend? [/INST]"
	require.Equal(t, expected, dialog)
}

func TestFormatDialogMistral(t *testing.T) {
	msgs := []*agent.Message{
		agent.NewContentMessage(agent.RoleUser, "Hello!"),
	}

	dialog, err := formatDialogMistral(context.Background(), msgs)
	require.NoError(t, err)

	expected := "[INST] Hello! [/INST]"
	require.Equal(t, expected, dialog)

	// Insert a system message into front of msgs
	systemMsg := agent.NewContentMessage(agent.RoleSystem, "You are a code completion AI designed to seamlessly integrate with surrounding code.")

	msgs = []*agent.Message{systemMsg, msgs[0]}

	dialog, err = formatDialogMistral(context.Background(), msgs)
	require.NoError(t, err)
	expected = "[INST] You are a code completion AI designed to seamlessly integrate with surrounding code.\n\nHello! [/INST]"
	require.Equal(t, expected, dialog)

	msgs = append(msgs, agent.NewContentMessage(agent.RoleAssistant, "How can I help you?"))
	msgs = append(msgs, agent.NewContentMessage(agent.RoleUser, "Will you be my friend?"))

	dialog, err = formatDialogMistral(context.Background(), msgs)
	require.NoError(t, err)
	expected = "[INST] You are a code completion AI designed to seamlessly integrate with surrounding code.\n\nHello! [/INST] How can I help you? [INST] Will you be my friend? [/INST]"
	require.Equal(t, expected, dialog)
}
