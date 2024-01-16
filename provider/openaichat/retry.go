package openaichat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

func isRetry(err error) bool {
	switch {
	case strings.Contains(err.Error(), "rate_limit_exceeded"):
		return true
	case strings.Contains(err.Error(), "error making request"):
		return true
	case strings.Contains(err.Error(), "You can retry your request"):
		return true
	default:
		return false
	}
}

func Retry(limit int) MiddlewareFunc {

	backoff := 100 * time.Millisecond

	return func(ctx context.Context, params openai.ChatCompletionRequest, next CreateCompletionFn) (openai.ChatCompletionResponse, error) {
		for try := 0; try < limit; try++ {
			resp, err := next(ctx, params)
			if err != nil {
				if !isRetry(err) {
					return resp, err
				}

				time.Sleep(backoff)
				backoff *= 2

				continue
			}

			return resp, nil
		}

		return openai.ChatCompletionResponse{}, fmt.Errorf("reached retry limit")
	}
}
