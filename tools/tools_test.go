package tools

import (
	"context"
	"testing"

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

	msg, err := ts.call(ctx, "hello", "{}")
	require.NoError(t, err)

	content, err := msg.Content(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello world!", content)

	r, err := ts.call(ctx, "invalid", "{}")
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

	msg, err := ts.call(ctx, "hello", `{}`)
	require.NoError(t, err)

	content, err := msg.Content(ctx)
	require.NoError(t, err)
	assert.Contains(t, content, "Hello world")

	msg, err = ts.call(ctx, "worldDomination", `{}`)
	require.NoError(t, err)
	content, _ = msg.Content(ctx)
	require.Contains(t, content, "tool not found: worldDomination")
}
