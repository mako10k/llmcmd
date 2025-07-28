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

### Development Workflow
1. Use #mcp-shell-server for Git commands and build operations
2. Incremental implementation and testing
3. Proper commit messages with clear change descriptions
4. Phase-based development approach

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
- [ ] pipe tool: Built-in command execution and data transfer
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
