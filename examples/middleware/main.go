package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/rakyll/openai-go"
	"github.com/rhettg/agent"
	"github.com/rhettg/agent/functions"
	"github.com/rhettg/agent/provider/openaichat"
	"github.com/rhettg/agent/set"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable not set")
	}

	s := openai.NewSession(apiKey)
	p := openaichat.New(s, "gpt-4")

	as := set.New()
	fs := functions.New()
	fs.AddFunctions(as.Functions())

	// Adding middleware is like an onion, the first ones added will be closest
	// to the destination provider (the API call itself.)
	// The order is up to you and your usecase, but the recommended order would be:

	a := agent.New(p,
		// First add filters because the most likely use is to remove messages
		// based on some heuristic. This gives all the other middleware the
		// opporutnity to add or tag messages before this happens.
		agent.WithFilter(limitMessagesFilter),

		// Next add functions because they are one of the rare cases where we
		// intercept and directly handle completions.
		functions.WithFunctions(fs),

		// Next add our agent set because if we're talking to a sub-agent then
		// we maybe do not want functions to be handled but rather handed to the
		// sub-agent. If this isn't true, reverse these!
		set.WithAgentSet(as),

		// Finally add checks because they are the last thing to run and verify
		// any content in generated messages.
		agent.WithCheck(checkForSecret),
	)

	a.Add(agent.RoleSystem, "You are a helpful assistant.")
	a.Add(agent.RoleUser, "Are you alive?")

	r, err := a.Step(context.Background())
	if err != nil {
		log.Fatalf("error from Agent: %v", err)
	}

	c, err := r.Content(context.Background())
	if err != nil {
		log.Fatalf("error from Message: %v", err)
	}

	fmt.Println(c)
}

func limitMessagesFilter(ctx context.Context, msgs []*agent.Message) ([]*agent.Message, error) {
	if len(msgs) > 10 {
		return msgs[:10], nil
	}
	return msgs, nil
}

func checkForSecret(ctx context.Context, msg *agent.Message) error {
	c, _ := msg.Content(ctx)
	if strings.Contains(c, "duck") {
		return errors.New("you said the secret word!!")
	}

	return nil
}
