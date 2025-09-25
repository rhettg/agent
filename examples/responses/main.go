package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/rhettg/agent"
	"github.com/rhettg/agent/provider/openairesponses"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable not set")
	}

	// Create a Responses API provider with reasoning enabled
	p := openairesponses.New(apiKey, "gpt-5-mini",
		openairesponses.WithReasoningEffort("medium"),    // Set reasoning effort level
		openairesponses.WithReasoningSummary("detailed"), // Get detailed reasoning summaries
	)

	a := agent.New(p)

	a.Add(agent.RoleSystem, "You are a helpful assistant.")
	a.Add(agent.RoleUser, "Explain why the sky is blue.")

	fmt.Println("Making request to OpenAI Responses API...")

	resp, err := a.Step(context.Background())
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	content, err := resp.Content(context.Background())
	if err != nil {
		log.Fatalf("error getting message content: %v", err)
	}

	// Check if reasoning was included
	if resp.ReasoningContent != "" {
		fmt.Println("Reasoning:\n", resp.ReasoningContent)
	}

	if resp.ReasoningEncryptedContent != "" {
		fmt.Println("Encrypted reasoning available (length:", len(resp.ReasoningEncryptedContent), ")")
	}

	if len(resp.ReasoningSummaries) > 0 {
		fmt.Println("Reasoning summaries:")
		for i, summary := range resp.ReasoningSummaries {
			fmt.Printf("  %d: %s\n", i+1, summary)
		}
	}

	fmt.Println("Response:", content)
}
