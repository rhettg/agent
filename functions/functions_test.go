package functions

import (
	"context"
	"testing"

	"github.com/sashabaranov/go-openai/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var EmptyParameters = jsonschema.Definition{
	Type:       "object",
	Properties: map[string]jsonschema.Definition{},
}

func TestFunctionSet(t *testing.T) {
	ctx := context.Background()

	fs := New()

	fs.Add("hello", "Say hello", EmptyParameters, func(ctx context.Context, args string) (string, error) {
		return "Hello world!", nil
	})

	msg, err := fs.call(ctx, "hello", "{}")
	require.NoError(t, err)

	content, err := msg.Content(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello world!", content)

	r, err := fs.call(ctx, "invalid", "{}")
	rc, _ := r.Content(ctx)
	assert.Equal(t, "function not found: invalid", rc)
	assert.NoError(t, err)
}

func hello(ctx context.Context, args string) (string, error) {
	return "Hello world!", nil
}

func TestFunctionSet_CompletionFunc(t *testing.T) {
	ctx := context.Background()

	fs := New()

	fs.Add("hello", "say hello", EmptyParameters, func(ctx context.Context, args string) (string, error) {
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

	msg, err = fs.call(ctx, "worldDomination", `{}`)
	require.NoError(t, err)
	content, _ = msg.Content(ctx)
	require.Contains(t, content, "function not found: worldDomination")
}
