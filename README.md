# Agent - Go library for bulding LLM-based applications

## Description

This library provides an architecture for building LLM-based applications.

Specifically it wraps underlying Chat interfaces for LLMs providing an
application architecture that can be useful for building agents. The focus is to
provide first-class support for tools and complex workflows.

This can be thought of as a native-go approach to Python frameworks such as
[langchain](https://github.com/langchain-ai/langchain).

## Usage

```go
s := openai.NewClient("my-api-key")
p := openaichat.New(s, "gpt-4")

fs := functions.New()

a := agent.New(p,
	agent.WithFilter(agent.LimitMessagesFilter(5)),

	// Next add functions because they are one of the rare cases where we
	// intercept and directly handle completions.
	functions.WithFunctions(fs),
)

a.Add(agent.RoleSystem, "You are a helpful assistant.")
a.Add(agent.RoleUser, "Are you alive?")

// Allow a single call to the underlying provider.
r, err := a.Step(context.Background())
if err != nil {
	log.Fatalf("error from Agent: %v", err)
}

c, err := r.Content(context.Background())
if err != nil {
	log.Fatalf("error from Message: %v", err)
}

fmt.Println(c)
```

## Design

### Dialog

The Agent is at root a data model for a dialog. It contains a sequence of
messages between multiple entities (including `system`, `user`, `assistant`, and
`function`).

These messages are passed to a CompletionFunc which maybe be a chain of
middleware providing additional functionality.

### CompletionFunc

The design for Agent is inspired by [`net/http`](https://pkg.go.dev/net/http).
Most features are implemented as middleware around the underlying provider.

```go
type CompletionFunc func(context.Context, []*Message, []FunctionDef) (*Message, error)
```

All providers need to implement this signature to issue completion requests to the underlying LLM.

"Middleware" is implemented by wrapping the provider in layers, like an onion.

### Provider

The provider is the interface for the underlying LLM. It is responsible for
sending the available messages and functions to the LLM. The provided example
providers also make use of a similar middleware pattern for providing additional
functionality.

For example, logging for the `openaichat` provider is configured like:

```go
slog.SetDefault(slog.New(&slogor.Handler{
	Mutex:      new(sync.Mutex),
	Level:      slog.LevelDebug,
	TimeFormat: time.Stamp,
}))

s := openai.NewClient("my-api-key")
p := openaichat.New(s, "gpt-4",
	openaichat.WithMiddleware(openaichat.Logger(slog.Default())),
)
```

While the implementation itself composes like an onion:

```go
func Logger(l *slog.Logger) MiddlewareFunc {
	return func(ctx context.Context, params openai.ChatCompletionRequest, next CreateCompletionFn) (openai.ChatCompletionResponse, error) {
		st := time.Now()

		resp, err := next(ctx, params)
		if err != nil {
			l.LogAttrs(ctx, slog.LevelError, "failed executing completion", slog.String("error", err.Error()))
			return resp, err
		}

		l.LogAttrs(ctx, slog.LevelDebug, "executed completion",
			slog.Duration("elapsed", time.Since(st)),
			slog.Int("prompt_tokens", resp.Usage.PromptTokens),
			slog.Int("completion_tokens", resp.Usage.CompletionTokens),
			slog.String("finish_reason", string(resp.Choices[0].FinishReason)),
		)
		return resp, err
	}
}
```

## Features

### Filters

Filters allow for breaking the 1-1 relationship between Agent messages and what
is sent to the underlying provider.

LLMs have limited Context windows, so this provides helpful functionality for
having greater control over the context without throwing away potentially
valuable information.

A simple example of a filter might limit how many messages to send:

```go
func LimitMessagesFilter(max int) FilterFunc {
	return func(ctx context.Context, msgs []*Message) ([]*Message, error) {
		if len(msgs) > max {
			return msgs[len(msgs)-max:], nil
		}
		return msgs, nil
	}
}
```

This can then be added to the agent configuration:

```go
a := agent.New(c, "gpt-4", agent.WithFilter(LimitMessagesFilter(5)))
```

### Checks

Checks are the response side of filters: it's a convinient way to intercept responses from the provider chain.

```go
func hasSecret(ctx context.Context, msg *agent.Message) error {
	c, _ := msg.Content(ctx)
	if strings.Contains(c, "secret") {
		return errors.New("has a secret")
	}

	return nil
}
```

This can then be added to the agent configuration:

```go
a := agent.New(c, "gpt-4", agent.WithCheck(hasSecret))
```

### Functions

### Vision

Messages can include image data:

```go
imgData, _ := os.ReadFile("camera.jpg")
m := agent.NewImageMessage(agent.RoleUser, "please explain", "camera.jpg", imgData)
a.AddMessage(m)
```

See [example](./examples/vision/main.go)

### Agent Set

An Agent Set allows an LLM to start a dialog with another LLM. It exposes two new tools for your primary agent to call:

* `agent_start` - starts an agent
* `agent_stop` - stops an agent

```go
s := set.New()
s.Add("eyes", EyesAgentStartFunc())

fs := functions.New()
fs.AddFunctions(a.Functions())

a := agent.New(c, set.WithAgentSet(s))
```

### Message Attributes

Each message may contain a set of attributes that are not likely to be used by
the LLM, but can provide important contextual clues for other parts of the
application.

```go
m := agent.NewContentMessage(agent.RoleUser, "content...")
m.SetAttr("important", "true")
```

This works well with filters.

### Dynamic Messages

The API for retrieving the content of a message is designed to support more than simply returning a string.

```go
	content, err := m.Content(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting message content: %v\n", err)
		os.Exit(1)
	}
```

Messages support running functions to dynamically generate content.

```go
func generateSystem(ctx context.Context) (string, error) {
    currentDate := time.Now().Format("2006-01-02")

    content := fmt.Sprintf("System Message - Date: %s", currentDate)

    return content, nil
}

msg := agent.NewDynamicMessage(agent.RoleSystem, generateSystem)
a.AddMessage(msg)
```

### Run patterns
