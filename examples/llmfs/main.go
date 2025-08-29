package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/rhettg/agent"
	"github.com/rhettg/agent/provider/openaichat"
	"github.com/rhettg/agent/tools"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable not set")
	}

	// Read query from stdin
	reader := bufio.NewReader(os.Stdin)
	query, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		log.Fatalf("error reading from stdin: %v", err)
	}
	query = strings.TrimSpace(query)

	if query == "" {
		log.Fatal("no query provided")
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("error getting current directory: %v", err)
	}

	p := openaichat.New(apiKey, "gpt-5-2025-08-07")

	ts := tools.New()
	createFilesystemTools(ts, cwd)

	a := agent.New(p, tools.WithTools(ts))

	systemPrompt := fmt.Sprintf(`You are a file system explorer AI. You can list directories and read files to help answer questions about projects and codebases.

Current working directory: %s

Available tools:
- list_directory: List files and directories in a given path
- read_file: Read the contents of a file

When exploring, start with the current directory and navigate as needed to answer the user's question. Be thorough but efficient.`, cwd)

	a.Add(agent.RoleSystem, systemPrompt)
	a.Add(agent.RoleUser, query)

	for {
		m, err := a.Step(context.Background())
		if err != nil {
			log.Fatalf("error handling message: %v", err)
		}

		if m.Role == agent.RoleAssistant {
			content, err := m.Content(context.Background())
			if err != nil {
				log.Fatalf("error getting message content: %v", err)
			}

			if content != "" {
				fmt.Println(content)
			}

			// If no tool calls in the message, we're done
			if len(m.ToolCalls) == 0 {
				break
			}
		}
	}
}

func createFilesystemTools(ts *tools.Tools, basePath string) {
	// List directory tool
	listDirParameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The directory path to list (absolute or relative to current directory)",
			},
		},
		"required": []string{"path"},
	}

	listDirTool := func(ctx context.Context, arguments string) (string, error) {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("could not parse arguments as JSON: %w", err)
		}

		pathArg, ok := args["path"].(string)
		if !ok {
			return "", fmt.Errorf("path parameter must be a string")
		}

		// Convert relative paths to absolute
		if !filepath.IsAbs(pathArg) {
			pathArg = filepath.Join(basePath, pathArg)
		}

		entries, err := os.ReadDir(pathArg)
		if err != nil {
			return "", fmt.Errorf("error reading directory %s: %w", pathArg, err)
		}

		var result strings.Builder
		result.WriteString(fmt.Sprintf("Contents of %s:\n", pathArg))

		for _, entry := range entries {
			if entry.IsDir() {
				result.WriteString(fmt.Sprintf("  %s/\n", entry.Name()))
			} else {
				info, err := entry.Info()
				if err == nil {
					result.WriteString(fmt.Sprintf("  %s (%d bytes)\n", entry.Name(), info.Size()))
				} else {
					result.WriteString(fmt.Sprintf("  %s\n", entry.Name()))
				}
			}
		}

		return result.String(), nil
	}

	ts.Add("list_directory", "List files and directories in a given path", listDirParameters, listDirTool)

	// Read file tool
	readFileParameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The file path to read (absolute or relative to current directory)",
			},
			"max_lines": map[string]interface{}{
				"type":        "number",
				"description": "Maximum number of lines to read (default: 100)",
			},
		},
		"required": []string{"path"},
	}

	readFileTool := func(ctx context.Context, arguments string) (string, error) {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("could not parse arguments as JSON: %w", err)
		}

		pathArg, ok := args["path"].(string)
		if !ok {
			return "", fmt.Errorf("path parameter must be a string")
		}

		// Convert relative paths to absolute
		if !filepath.IsAbs(pathArg) {
			pathArg = filepath.Join(basePath, pathArg)
		}

		maxLines := 100
		if ml, ok := args["max_lines"].(float64); ok {
			maxLines = int(ml)
		}

		content, err := os.ReadFile(pathArg)
		if err != nil {
			return "", fmt.Errorf("error reading file %s: %w", pathArg, err)
		}

		lines := strings.Split(string(content), "\n")
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			lines = append(lines, fmt.Sprintf("... (truncated at %d lines)", maxLines))
		}

		return fmt.Sprintf("Contents of %s:\n%s", pathArg, strings.Join(lines, "\n")), nil
	}

	ts.Add("read_file", "Read the contents of a file", readFileParameters, readFileTool)
}
