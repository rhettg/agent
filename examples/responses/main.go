package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/rhettg/agent"
	"github.com/rhettg/agent/provider/openairesponses"
)

type deltaPrinter struct {
	mode string
}

func (d *deltaPrinter) printDelta(ctx context.Context, delta openairesponses.MessageDelta) {
	if d.mode == "" {
		if delta.Content != "" {
			d.mode = "response"
			fmt.Println("Response:")
		}
		if delta.ReasoningContent != "" {
			d.mode = "reasoning"
			fmt.Println("Reasoning:")
		}
		if delta.ReasoningSummary != "" {
			d.mode = "reasoning-summary"
			fmt.Println("Reasoning summary:")
		}
	}
	if d.mode == "response" {
		if delta.Content != "" {
			fmt.Print(delta.Content)
		} else {
			d.mode = ""
			fmt.Println()
		}
	} else if d.mode == "reasoning" {
		if delta.ReasoningContent != "" {
			fmt.Print(delta.ReasoningContent)
		} else {
			d.mode = ""
			fmt.Println()
		}
	} else if d.mode == "reasoning-summary" {
		if delta.ReasoningSummary != "" {
			fmt.Print(delta.ReasoningSummary)
		} else {
			d.mode = ""
			fmt.Println()
		}
	}
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable not set")
	}

	dp := &deltaPrinter{}

	// Create a Responses API provider with reasoning enabled
	p := openairesponses.New(apiKey, "gpt-5-mini",
		openairesponses.WithReasoningEffort("medium"),    // Set reasoning effort level
		openairesponses.WithReasoningSummary("detailed"), // Get detailed reasoning summaries
		openairesponses.WithMessageDeltaFunc(dp.printDelta),
	)

	a := agent.New(p)

	a.Add(agent.RoleSystem, "You are a helpful assistant.")
	a.Add(agent.RoleUser, "Explain why the sky is blue.")

	fmt.Println("Making request to OpenAI Responses API...")

	_, err := a.Step(context.Background())
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	/*
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
	*/
}
