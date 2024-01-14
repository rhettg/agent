package agent

import (
	"context"
	"testing"

	"github.com/rakyll/openai-go/chat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunctionSet(t *testing.T) {

	ctx := context.Background()

	fs := NewFunctionSet()

	fs.Add("hello", "Say hello", chat.EmptyParameters, func(ctx context.Context, args string) (string, error) {
		return "Hello world!", nil
	})

	msg, err := fs.call(ctx, "hello", "{}")
	require.NoError(t, err)

	content, err := msg.Content(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello world!", content)

	_, err = fs.call(ctx, "invalid", "{}")
	assert.Error(t, err)
}

func hello(ctx context.Context, args string) (string, error) {
	return "Hello world!", nil
}

func TestFunctionSet_CompletionFunc(t *testing.T) {
	ctx := context.Background()

	fs := NewFunctionSet()

	fs.Add("hello", "say hello", chat.EmptyParameters, func(ctx context.Context, args string) (string, error) {
		resp, err := hello(ctx, args)
		if err != nil {
			return "", err
		}

		return resp, nil
	})

	msg, err := fs.call(ctx, "hello", `{}`)
	require.NoError(t, err)

	content, err := msg.Content(ctx)
	require.NoError(t, err)
	assert.Contains(t, content, "Hello world")

	_, err = fs.call(ctx, "worldDomination", `{}`)
	require.EqualError(t, err, "function not found: worldDomination")
}
