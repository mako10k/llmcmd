# llmcmd - LLM Command Line Tool

## Project Overview

`llmcmd` is a command-line tool that enables Large Language Models (LLMs) to execute tasks using the OpenAI ChatCompletion API. The tool provides LLMs with secure, built-in functions for file operations and text processing without external command execution.

## Development Goals

1. **Security First**: All operations are sandboxed with built-in commands only
2. **Simplicity**: Clean, maintainable code using compiled languages
3. **Efficiency**: Optimized API usage with proper cost controls
4. **Extensibility**: Future-ready architecture for additional features

## Core Constraints

### Language Policy
- **Codebase**: English only (code, comments, variable names)
- **Commands**: English only
- **Runtime**: Japanese input allowed for user instructions
- **Documentation**: English preferred, Japanese acceptable for implementation notes

### Security Requirements
- **No external command execution**: All text processing via built-in functions
- **File access limited**: Only specified input/output files
- **API call limits**: Maximum 50 calls per session with 300s timeout
- **Memory limits**: 4KB read buffer, 10MB file size limits

## Architecture

### Technology Stack
- **Language**: Go 1.21+
- **API**: OpenAI ChatCompletion with Function Calling
- **Architecture**: Layered design with clean separation

### Core Components
1. **CLI Parser**: Command-line argument processing
2. **Configuration Manager**: Config file and environment variable handling
3. **OpenAI API Client**: ChatCompletion API communication
4. **Tool Execution Engine**: LLM tool call processing
5. **Built-in Commands**: Security-focused text processing functions

## Development Environment

### Required Tools
- **Shell Operations**: Use #mcp-shell-server for all terminal commands
- **Code Editor**: VS Code
- **Go Version**: 1.21 or higher

### ⚠️ CRITICAL: Tool Selection Requirements

**🚫 DO NOT USE `run_in_terminal`** - This tool has known output reading bugs and cannot reliably capture command outputs, especially when they are truncated or large.

**✅ ALWAYS USE `#mcp-shell-server`** - This is the ONLY approved method for executing shell commands in this project. It provides:
- Reliable output capture with truncation handling
- Background process management
- Complete output retrieval via `output_id`
- Proper error handling and status monitoring

**Violation of this rule will cause development failures and incomplete command outputs.**

### mcp-shell-server Usage Notes
- **Output Tracking**: When command output is truncated, use the returned `output_id` with `read_execution_output` to get complete results
- **Common Mistake**: Don't assume output is complete when truncated - always check for `output_truncated: true` and use `output_id` to retrieve full content
- **Background Processes**: Long-running commands automatically switch to background mode and provide `execution_id` for status monitoring

### Development Workflow
1. **🔥 MANDATORY**: Use #mcp-shell-server for Git commands and build operations - NEVER use run_in_terminal
2. Incremental implementation and testing
3. Proper commit messages with clear change descriptions
4. Phase-based development approach

### Shell Command Execution Rules
- **Git Operations**: `#mcp-shell-server` only (git status, git commit, git push, etc.)
- **Build Commands**: `#mcp-shell-server` only (go build, go test, go mod, etc.)
- **File Operations**: `#mcp-shell-server` only (grep, find, ls, etc.)
- **ANY Shell Command**: `#mcp-shell-server` only - NO EXCEPTIONS

**Reason**: `run_in_terminal` has output reading bugs that cause incomplete results and development workflow failures.

## Implementation Phases

### Phase 1: Foundation (Days 1-3)
- [x] Project structure and documentation
- [ ] Go module initialization  
- [ ] Basic CLI argument parsing
- [ ] Configuration file loading
- [ ] Logging infrastructure

### Phase 2: OpenAI Integration (Days 4-7)
- [ ] HTTP client implementation
- [ ] OpenAI API type definitions
- [ ] Authentication and error handling
- [ ] Response parsing
- [ ] Retry mechanisms

### Phase 3: Tool Implementation (Days 8-13)
- [ ] read tool: File/stream reading with fd management
- [ ] write tool: File/stream writing with size tracking
- [ ] spawn tool: Background-only command execution and data transfer
- [ ] exit tool: Program termination with cleanup

### Phase 4: Built-in Commands (Days 14-17)
- [ ] cat: Data copying
- [ ] grep: Pattern matching (basic regex)
- [ ] sed: Text substitution (basic functionality)
- [ ] head/tail: Line-based filtering
- [ ] sort: Alphabetical sorting
- [ ] wc: Counting (lines, words, characters)
- [ ] tr: Character translation

### Phase 5: Integration & Testing (Days 18-22)
- [ ] Main application logic
- [ ] Tool orchestration
- [ ] Comprehensive error handling
- [ ] Security feature integration
- [ ] Performance optimization

## Project Structure

```
llmcmd/
├── cmd/llmcmd/           # Entry point
├── internal/
│   ├── app/              # Application logic
│   ├── cli/              # CLI parsing and configuration
│   ├── openai/           # OpenAI API client
│   ├── tools/            # Tool execution engine
│   │   └── builtin/      # Built-in command implementations
│   └── security/         # Security controls
├── docs/                 # Documentation
├── examples/             # Usage examples
└── README.md
```

## Security Guidelines

### Built-in Command Security
- No file system access beyond specified files
- No shell metacharacter interpretation
- No environment variable access
- Stream-based processing only

### API Cost Controls
- Maximum 50 API calls per session
- 300-second execution timeout
- 4KB read buffer limit
- 10MB file size limits

### Error Handling
- Graceful degradation
- Detailed error logging
- Secure error messages (no sensitive data exposure)

## Testing Strategy

### Test Pyramid
- **Unit Tests**: Individual function validation (high coverage)
- **Integration Tests**: API and tool interaction testing
- **E2E Tests**: Real-world scenario validation

### Mock Strategy
- OpenAI API mocking for reproducible tests
- File system mocking for I/O testing
- Command execution simulation

## Deliverables

### Version 1.0.0 Release
- [ ] Executable binary `llmcmd`
- [ ] Configuration template `.llmcmdrc`
- [ ] Complete documentation suite
- [ ] Comprehensive test suite
- [ ] Cross-platform binaries (Linux, macOS, Windows)

### Quality Requirements
- Unit test coverage ≥ 80%
- All major E2E scenarios tested
- Memory leak free
- Complete documentation
- Security audit passed

## Development Best Practices

### Code Quality
- Follow Go conventions and idioms
- Use standard library when possible
- Minimize external dependencies
- Clear, descriptive naming

### Git Workflow
- Feature branches for new functionality
- Descriptive commit messages
- Regular commits with logical groupings
- Proper branch management

### Performance Targets
- Startup time: < 100ms
- Memory usage: < 50MB baseline
- API response time: OpenAI API + minimal processing overhead
- File processing: Streaming for large files

This structured approach ensures secure, maintainable, and efficient development of the llmcmd tool.
