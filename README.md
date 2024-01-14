# Coder

## Description

## Installation

## Usage

```go
// Provider specific configuration results in a generic "completer" interface.
s := openai.NewSession("my-secret-key")

usage := chatgpt.Usage{}
client := chat.NewClient(session, modelName)

completer := provider.NewOpenAIChat(s, "gpt-4", usage.Middleware)

// Functions are configured together as a set.
fs := function.NewFunctionSet()
fs.Add("hello", "Says hello", chat.EmptyParameters, func(ctx context.Context, args string) (string, error) {
	fmt.Println("Hello")
})

a := assistant.New(completer,
	WithMiddleware(usage.Middleware),
	WithFunctionSet(fs))

// Content is added to the assistant along with a Role.
a.Add(assistant.RoleSystem, systemPrompt)

a.Add(assistant.RoleUser, "Greetings!")

// Dynamic content can be added which will call the provided function to
// generate content on each interation.
a.AddDynamic(assistant.RoleUser, func(ctx context.Context) (string, error) {
	return fmt.Sprintf("Hello %s, today is %s", name, time.Now().Format("January 2, 2006"))
})

// A Filter can modify the messages prior to sending to a provider
a.Filter(func(ctx context.Context, msgs []*assistant.Message) ([]*assistant.Message), error) {
	return msgs
})

// Checks are called after a provider responds.
a.Check(func(ctx context.Context, msg *assistant.Message) (error) {
	if msg.FunctionCall != "hello" {
		a.Add(assistant.RoleUser, "required function hello not called")
	}
	return nil
})

for {
	msg, err := a.Step(context.Background())
}

// TODO: what about concurrency, half the point.
// Sub-assistants could run concurrently. But that breaks the model of asking
// the caller for each step. We still need some guardrails to be available... step limits?

maxSteps := 5
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error {
	assistant.RunUntil(ctx, a, assistant.AssistantResponded(maxSteps))
})
g.Go(func() error {
	assistant.RunUntil(ctx, a, assistant.AssistantResponded(maxSteps))
})

err := g.Wait()

// TODO: do I want to have a worker pool to restrict parallel requests to the model? Could the provider need to handle this?

// Or for even more flexibility:

assistant.RunUntil(ctx, a, func(ctx context.Context) bool {
	return true
})

// There are some helpful utility defaults
assistant.RunUntil(ctx, a, assistant.UntilAssistantResponded(maxSteps))

// Stopping the assistant can also be explicit
a.AddStop()
assistant.RunUntil(ctx, a, assistant.UntilStop)

// Layer assistants for another level of control

// TODO: What about stopping...? Does that mean it's a message now?
type Stepper interface {
	Step(context.Context) (*assistant.Message, error)
}

type StepFunc func(context.Context) (*assistant.Message, error)

func (f StepFunc) Step(ctx context.Context) (m *assistant.Message, err error) {
	return f(ctx)
}

func stepLayer(ctx context.Context) (*assistant.Message, error) {
	fmt.Println("about to step")
	resp, err := a.Step(ctx)
	if err != nil {
		fmt.Println("error while stepping")
		return resp, err
	}

	fmt.Println("done with step")
	return resp, nil
}

assistant.RunUntil(ctx, StepFunc(stepLayer), assistant.UntilStop)

```
