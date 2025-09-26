package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/rhettg/agent"
	"github.com/rhettg/agent/provider/openairesponses"
	"github.com/rhettg/agent/tools"
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
		if delta.ToolCallID != "" {
			d.mode = "tool-call"
			fmt.Println("Tool call:", delta.ToolCallID, delta.ToolCallName)
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
	} else if d.mode == "tool-call" {
		if delta.ToolCallArguments != "" {
			fmt.Print(delta.ToolCallArguments)
		} else {
			d.mode = ""
			fmt.Println()
		}
	}
}

// getCurrentTime is a simple tool that returns the current time
func getCurrentTime(ctx context.Context, arguments string) (string, error) {
	fmt.Println("\n[TOOL CALL] getCurrentTime called")
	return time.Now().Format("Monday, January 2, 2006 at 3:04 PM MST"), nil
}

// calculateSum is a tool that adds two numbers
func calculateSum(ctx context.Context, arguments string) (string, error) {
	fmt.Printf("\n[TOOL CALL] calculateSum called with arguments: %s\n", arguments)

	// Simple parsing - in a real implementation you'd use proper JSON parsing
	// This is just for demonstration
	args := strings.ReplaceAll(arguments, " ", "")
	if strings.Contains(args, `"a":`) && strings.Contains(args, `"b":`) {
		// Extract numbers (very basic parsing for demo)
		var a, b int
		if _, err := fmt.Sscanf(args, `{"a":%d,"b":%d}`, &a, &b); err != nil {
			return "", fmt.Errorf("failed to parse arguments: %v", err)
		}
		result := a + b
		return fmt.Sprintf("The sum of %d and %d is %d", a, b, result), nil
	}

	return "Unable to parse arguments. Please provide numbers a and b.", nil
}

// weatherTool simulates getting weather information
func weatherTool(ctx context.Context, arguments string) (string, error) {
	fmt.Printf("\n[TOOL CALL] weatherTool called with arguments: %s\n", arguments)

	// Simple simulation - always return sunny weather
	location := "unknown location"
	if strings.Contains(arguments, "location") {
		// Extract location (basic parsing for demo)
		if strings.Contains(arguments, "San Francisco") || strings.Contains(arguments, "san francisco") {
			location = "San Francisco"
		} else if strings.Contains(arguments, "New York") || strings.Contains(arguments, "new york") {
			location = "New York"
		}
	}

	return fmt.Sprintf("The weather in %s is currently sunny with a temperature of 72°F (22°C). Light winds from the west at 5 mph.", location), nil
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable not set")
	}

	dp := &deltaPrinter{}

	// Create a Responses API provider with streaming
	p := openairesponses.New(apiKey, "gpt-5-mini",
		openairesponses.WithMessageDeltaFunc(dp.printDelta),
	)

	// Create custom tool executor that handles empty tool calls
	ts := tools.New()

	// Add time tool
	ts.Add("get_current_time", "Get the current date and time", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}, getCurrentTime)

	// Add calculator tool
	ts.Add("calculate_sum", "Add two numbers together", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"a": map[string]interface{}{
				"type":        "integer",
				"description": "First number to add",
			},
			"b": map[string]interface{}{
				"type":        "integer",
				"description": "Second number to add",
			},
		},
		"required": []string{"a", "b"},
	}, calculateSum)

	// Add weather tool
	ts.Add("get_weather", "Get current weather information for a location", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"location": map[string]interface{}{
				"type":        "string",
				"description": "The city or location to get weather for",
			},
		},
		"required": []string{"location"},
	}, weatherTool)

	// Create agent with custom tool executor
	a := agent.New(p, agent.WithMiddleware(ts.CompletionFunc))

	a.Add(agent.RoleSystem, "You are a helpful assistant that can use tools to provide accurate information.")
	a.Add(agent.RoleUser, "What time is it? Also, what's 25 + 17? And can you tell me the weather in San Francisco?")

	fmt.Println("Making request to OpenAI Responses API with tool calling...")

	resp, err := a.Step(context.Background())
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Debug: print message details
	content, _ := resp.Content(context.Background())
	fmt.Printf("\n[DEBUG] Message role: %s, has tool calls: %v, content: %q\n", resp.Role, resp.HasToolCalls(), content)

	if resp.HasToolCalls() {
		for i, tc := range resp.ToolCalls {
			fmt.Printf("[DEBUG] Tool call %d: ID=%s, Name=%s, Args=%s\n", i, tc.ID, tc.Name, tc.Arguments)
		}
	}

	fmt.Println("\n\n[INFO] Conversation complete!")
}
