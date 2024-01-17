package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStopOnReply(t *testing.T) {
	count := 0
	mockFn := func(ctx context.Context, msgs []*Message, fns []ToolDef) (*Message, error) {
		reply := NewContentMessage(RoleAssistant, "why hello there")
		count++
		return reply, nil
	}

	a := New(mockFn, WithCheck(StopOnReply))

	a.Add(RoleUser, "Hello")

	err := Run(context.Background(), a)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	msgs := a.Messages()
	require.Equal(t, 2, len(msgs))

	resp := msgs[1]

	assert.Equal(t, RoleAssistant, resp.Role)
	content, err := resp.Content(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "why hello there", content)
}
