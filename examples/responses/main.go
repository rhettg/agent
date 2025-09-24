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
	p := openairesponses.New(apiKey, "gpt-4o-2024-08-06",
		openairesponses.WithReasoningEffort("medium"), // Set reasoning effort level
		openairesponses.WithReasoningSummary("concise"), // Get concise reasoning summaries
	)

	a := agent.New(p)

	a.Add(agent.RoleSystem, "You are a helpful assistant that shows your reasoning.")
	a.Add(agent.RoleUser, "Explain why the sky is blue, and show your reasoning process.")

	fmt.Println("Making request to OpenAI Responses API...")
	
	resp, err := a.Step(context.Background())
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	content, err := resp.Content(context.Background())
	if err != nil {
		log.Fatalf("error getting message content: %v", err)
	}

	fmt.Println("Response:", content)

	// Check if reasoning was included
	if resp.ReasoningContent != "" || resp.ReasoningEncryptedContent != "" || len(resp.ReasoningSummaries) > 0 {
		fmt.Println("\n--- Reasoning ---")
		if resp.ReasoningContent != "" {
			fmt.Println("Plain reasoning:", resp.ReasoningContent)
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
	}
}
