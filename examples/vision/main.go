package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rhettg/agent"
	"github.com/rhettg/agent/provider/openaichat"
)

func main() {
	p := openaichat.New(os.Getenv("OPENAI_API_KEY"), "gpt-4-vision-preview",

		// By default, gpt-4-vision-preview is configured with a very small
		// default max tokens. Make it bigger.
		openaichat.WithMaxTokens(1024),
	)

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <image file>\n", os.Args[0])
		os.Exit(1)
	}

	a := agent.New(p)

	imgName := os.Args[1]
	imgData, err := os.ReadFile(imgName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading image file: %v\n", err)
		os.Exit(1)
	}

	im := agent.NewImageMessage(agent.RoleUser, "please explain", imgName, imgData)
	a.AddMessage(im)

	m, err := a.Step(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error handling message: %v\n", err)
		os.Exit(1)
	}

	content, err := m.Content(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting message content: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(content)
}
