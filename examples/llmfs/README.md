# LLM File System Explorer

This example demonstrates how to create a command-line tool that uses LLM agents with filesystem exploration tools.

## Features

The agent can:
- List directory contents
- Read file contents (with optional line limits)
- Navigate through the filesystem to answer queries about projects

## Usage

First, set your OpenAI API key:
```bash
export OPENAI_API_KEY="your-api-key-here"
```

Then run the tool with a query:
```bash
# Build the tool
go build -o llmfs main.go

# Ask about the current directory
echo "what is this project?" | ./llmfs

# Ask about specific files or patterns
echo "find all Go files and tell me what this library does" | ./llmfs

# Ask about architecture
echo "how is the code organized and what are the main components?" | ./llmfs
```

## Implementation Details

- Uses OpenAI's GPT-4 model
- Implements two filesystem tools:
  - `list_directory`: Lists files and directories with sizes
  - `read_file`: Reads file contents with optional line truncation
- Handles both absolute and relative file paths
- Automatically converts relative paths based on the current working directory

## Tools API

### list_directory
- **Description**: List files and directories in a given path
- **Parameters**: 
  - `path` (string, required): Directory path to list

### read_file  
- **Description**: Read the contents of a file
- **Parameters**:
  - `path` (string, required): File path to read
  - `max_lines` (number, optional): Maximum lines to read (default: 100)
