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
		openairesponses.WithReasoning(true),
		openairesponses.WithReasoningSummary(true),
		openairesponses.WithStore(false), // Don't store for privacy
	)

	a := agent.New(p)

	a.Add(agent.RoleSystem, "You are a helpful assistant that shows your reasoning.")
	a.Add(agent.RoleUser, "Explain why the sky is blue, and show your reasoning process.")

	fmt.Println("Making request to OpenAI Responses API...")
	
	// This will currently return an error since the SDK doesn't support Responses API yet
	// But it demonstrates the intended usage
	resp, err := a.Step(context.Background())
	if err != nil {
		fmt.Printf("Expected error (Responses API not yet available): %v\n", err)
		fmt.Println("\nThis example demonstrates the intended usage of the Responses API provider.")
		fmt.Println("Once OpenAI's Go SDK adds Responses API support, this will work seamlessly.")
		return
	}

	content, err := resp.Content(context.Background())
	if err != nil {
		log.Fatalf("error getting message content: %v", err)
	}

	fmt.Println("Response:", content)

	// Check if reasoning was included
	if resp.Reasoning != nil {
		fmt.Println("\n--- Reasoning ---")
		if resp.Reasoning.Content != "" {
			fmt.Println("Plain reasoning:", resp.Reasoning.Content)
		}
		if resp.Reasoning.EncryptedContent != "" {
			fmt.Println("Encrypted reasoning available (length:", len(resp.Reasoning.EncryptedContent), ")")
		}
		if len(resp.Reasoning.Summaries) > 0 {
			fmt.Println("Reasoning summaries:")
			for i, summary := range resp.Reasoning.Summaries {
				fmt.Printf("  %d: %s\n", i+1, summary)
			}
		}
	}
}
