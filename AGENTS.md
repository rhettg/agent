# AGENTS.md - Developer Guide

## Commands
- **Build**: `go build ./...` 
- **Test all**: `go test ./...`
- **Test single package**: `go test ./[package]` (e.g., `go test ./tools`, `go test ./provider/ollamachat`)
- **Test with coverage**: `go test -cover ./...`
- **Dependencies**: `go mod tidy`

## Architecture
Go library for building LLM-based applications with middleware architecture inspired by net/http. Core components: `Agent` (dialog management), `CompletionFunc` (LLM interface), middleware layers (filters, checks, tools). Key packages: `provider/` (OpenAI/Ollama integrations), `tools/` (function calling), `agentset/` (multi-agent workflows).

## Code Style
- **Naming**: PascalCase for exports, camelCase for private fields
- **Patterns**: Functional options (`WithMiddleware`, `WithCheck`), middleware chains, builder patterns
- **Interfaces**: Function types as interfaces (`CompletionFunc`, `MiddlewareFunc`, `FilterFunc`)
- **Errors**: Wrap with context using `fmt.Errorf("context: %w", err)`
- **Logging**: Use `slog` for structured logging
- **Imports**: Standard lib → third-party → local packages (properly grouped)
- **Receivers**: Pointer receivers for state modification, value for immutable operations
- **Memory**: Use proper capacity hints (`make([]*Message, 0, len(messages))`)
- **Testing**: Uses testify framework (`assert`, `require` packages)

## Documentation
- `README.md` contains usage examples that should be kept up-to-date

## Dependencies
Primary: OpenAI SDK, Ollama client, testify (testing), tiktoken (tokenization), YAML processing
