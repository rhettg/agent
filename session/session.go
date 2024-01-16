package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"
	"time"

	"github.com/rhettg/agent/provider/openaichat"
	"github.com/sashabaranov/go-openai"
)

type SessionStore struct {
	sessionPath string
}

func NewStore(path string) *SessionStore {
	// Create a directory for data files basead on the current date and time:
	basePath := fmt.Sprintf("%s/%s", path, time.Now().Format("20060102-150405"))

	return &SessionStore{
		sessionPath: basePath,
	}
}

func generateName(basePath, storeType string) (string, error) {
	// Look at the existing files in the session directory and generate a new name
	// based on the highest number found. They are stored as <number>.<storeType>.json
	// so we can just look for the highest number.
	files, err := os.ReadDir(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to read session directory: %w", err)
	}

	var highest int
	for _, f := range files {
		var n int
		_, err := fmt.Sscanf(f.Name(), "%d", &n)
		if err != nil {
			continue
		}

		if n > highest {
			highest = n
		}
	}

	return path.Join(basePath, fmt.Sprintf("%03d.%s.json", highest+1, storeType)), nil
}

func (s *SessionStore) initPath() error {
	err := os.MkdirAll(s.sessionPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	return nil
}

func (s *SessionStore) SaveParams(p openai.ChatCompletionRequest) error {
	err := s.initPath()
	if err != nil {
		return fmt.Errorf("failed to initialize data directory: %w", err)
	}

	fname, err := generateName(s.sessionPath, "params")
	if err != nil {
		return fmt.Errorf("failed to generate filename: %w", err)
	}

	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	slog.Debug("saving params", "component", "SessionStore", "filename", fname)
	err = os.WriteFile(fname, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write data file: %w", err)
	}

	return nil
}

func (s *SessionStore) SaveResponse(r openai.ChatCompletionResponse) error {
	err := s.initPath()
	if err != nil {
		return fmt.Errorf("failed to initialize data directory: %w", err)
	}

	fname, err := generateName(s.sessionPath, "response")
	if err != nil {
		return fmt.Errorf("failed to generate filename: %w", err)
	}

	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	slog.Debug("saving response", "component", "SessionStore", "filename", fname)
	err = os.WriteFile(fname, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write data file: %w", err)
	}

	return nil
}

func (s *SessionStore) SaveError(responseErr error) error {
	err := s.initPath()
	if err != nil {
		return fmt.Errorf("failed to initialize data directory: %w", err)
	}

	fname, err := generateName(s.sessionPath, "error")
	if err != nil {
		return fmt.Errorf("failed to generate filename: %w", err)
	}

	// Most error are probably openai.APIError, so let's make sure we get all the
	// details saved out.
	var data []byte
	if apiErr, ok := responseErr.(*openai.APIError); ok {
		data, err = json.Marshal(apiErr)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}
	} else {
		jErr := struct {
			Error string `json:"error"`
		}{Error: responseErr.Error()}
		data, err = json.Marshal(jErr)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}
	}

	slog.Debug("saving error", "component", "SessionStore", "filename", fname)
	err = os.WriteFile(fname, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write data file: %w", err)
	}

	return nil
}

func (s *SessionStore) Middleware(
	ctx context.Context, params openai.ChatCompletionRequest, next openaichat.CreateCompletionFn,
) (openai.ChatCompletionResponse, error) {
	err := s.SaveParams(params)
	if err != nil {
		slog.Error("failed to save params to session store", "err", err)
	}

	resp, err := next(ctx, params)
	if err != nil {
		sErr := s.SaveError(err)
		if sErr != nil {
			slog.Error("failed to save error to session store", "err", err)
		}

		return resp, err
	}

	err = s.SaveResponse(resp)
	if err != nil {
		slog.Error("failed to save response to session store", "err", err)
	}

	return resp, err

}
