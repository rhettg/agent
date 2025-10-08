package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/responses"
	"github.com/openai/openai-go/v2/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStore_SaveParams(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel("gpt-4"),
	}

	err := store.SaveParams(params)
	require.NoError(t, err)

	files, err := os.ReadDir(store.sessionPath)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0].Name(), "params.json")

	data, err := os.ReadFile(filepath.Join(store.sessionPath, files[0].Name()))
	require.NoError(t, err)

	var savedParams responses.ResponseNewParams
	err = json.Unmarshal(data, &savedParams)
	require.NoError(t, err)
	assert.Equal(t, shared.ResponsesModel("gpt-4"), savedParams.Model)
}

func TestSessionStore_SaveResponse(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	resp := &responses.Response{
		ID: "test-id",
	}

	err := store.SaveResponse(resp)
	require.NoError(t, err)

	files, err := os.ReadDir(store.sessionPath)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0].Name(), "response.json")
}

func TestSessionStore_SaveError(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	testErr := fmt.Errorf("test error")

	err := store.SaveError(testErr)
	require.NoError(t, err)

	files, err := os.ReadDir(store.sessionPath)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0].Name(), "error.json")

	data, err := os.ReadFile(filepath.Join(store.sessionPath, files[0].Name()))
	require.NoError(t, err)

	var savedErr struct {
		Error string `json:"error"`
	}
	err = json.Unmarshal(data, &savedErr)
	require.NoError(t, err)
	assert.Equal(t, "test error", savedErr.Error)
}

func TestSessionStore_Middleware(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel("gpt-4"),
	}

	expectedResp := &responses.Response{
		ID: "test-response-id",
	}

	nextCalled := false
	next := func(ctx context.Context, p responses.ResponseNewParams, opts ...option.RequestOption) (*responses.Response, error) {
		nextCalled = true
		assert.Equal(t, params, p)
		return expectedResp, nil
	}

	resp, err := store.Middleware(context.Background(), params, next)
	require.NoError(t, err)
	assert.True(t, nextCalled)
	assert.Equal(t, expectedResp, resp)

	files, err := os.ReadDir(store.sessionPath)
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestSessionStore_MiddlewareWithError(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel("gpt-4"),
	}

	expectedErr := fmt.Errorf("completion error")

	next := func(ctx context.Context, p responses.ResponseNewParams, opts ...option.RequestOption) (*responses.Response, error) {
		return nil, expectedErr
	}

	resp, err := store.Middleware(context.Background(), params, next)
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, resp)

	files, err := os.ReadDir(store.sessionPath)
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestSessionStore_MultipleRequests(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel("gpt-4"),
	}

	next := func(ctx context.Context, p responses.ResponseNewParams, opts ...option.RequestOption) (*responses.Response, error) {
		return &responses.Response{ID: "test"}, nil
	}

	_, err := store.Middleware(context.Background(), params, next)
	require.NoError(t, err)

	_, err = store.Middleware(context.Background(), params, next)
	require.NoError(t, err)

	files, err := os.ReadDir(store.sessionPath)
	require.NoError(t, err)
	assert.Len(t, files, 4)
}
