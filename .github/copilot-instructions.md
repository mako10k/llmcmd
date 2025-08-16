# llmcmd - LLM Command Line Tool

## 🧠 CRITICAL: Self-Improvement Memory System

### Before Starting ANY Task
1. **Search associative memory**: Use `mcp_mcp-assoc-mem_memory_search` for past failures, lessons, and design decisions
2. **Check shared memory**: Use `mcp_mcp-llm-gener_shared-memory-search` for team knowledge
3. **Consult personas**: Check with relevant team personas before major decisions
4. **Read project files**: Always check existing implementations and designs

### Specification Confirmation & Recording (Mandatory)
1. Before asking the user about existing specs or decisions, first search in associative memory: use `mcp_mcp-assoc-mem_memory_search` to find prior specifications and decisions (avoid re-asking users).
2. When a new specification decision is made, record it in associative memory using `mcp_mcp-assoc-mem_memory_store` under scope `work/llmcmd/spec-decisions` with a concise note (<=10 lines) including: context, decision, rationale, and impacts.
3. Apply this rule to design/architecture, CLI/flags, protocol, and built-in tool behaviors.
4. Reference these stored decisions in PR descriptions and help/docs where relevant.

### After Completing ANY Task
1. **Store lessons learned**: Use `mcp_mcp-assoc-mem_memory_store` to save failures, solutions, design decisions
2. **Update shared memory**: Use `mcp_mcp-llm-gener_shared-memory-create` or `shared-memory-update` for team knowledge
3. **Record problem patterns**: Document what went wrong and how to avoid it next time

### Problem Behavior Patterns (MUST AVOID)
- ❌ Starting implementation with unresolved design questions
- ❌ Skipping user agreement before coding 
- ❌ Repeating same mistakes due to memory limitations
- ❌ Acting without consulting team personas
- ❌ Forgetting to record lessons learned after completion
- ❌ Answering with speculation when memory is unclear
- ❌ Making claims without evidence from current context
- ❌ **CRITICAL**: Rushing to implementation when user shares conceptual ideas
- ❌ **CRITICAL**: Writing code during architecture discussion phase
- ❌ **CRITICAL**: Skipping design deliberation to jump into coding

### 🚨 MANDATORY: Design-First Process Control
**When user shares ideas or concepts:**
1. **NEVER immediately suggest implementation**
2. **ALWAYS engage in conceptual discussion first**
3. **Ask clarifying questions about requirements and constraints**
4. **Explore architectural implications thoroughly**
5. **Use mcp-confirm to verify understanding before ANY code creation**

**Phase Separation Rules:**
- **Concept Phase**: Ideas, requirements, high-level goals - NO CODE
- **Design Phase**: Architecture, structure, interfaces - NO CODE  
- **Implementation Phase**: Only after explicit user approval - CODE ALLOWED

**Required Confirmation Before Implementation:**
```
Before writing any code, I need to confirm:
- Have we fully explored the design requirements?
- Do you want to proceed to implementation now?
- Or should we continue the architectural discussion?
```

### 🚨 CRITICAL: Honest Communication Rules
**ALWAYS distinguish between training data patterns and current context memory:**
- **Context Memory**: Files, conversation history, MCP tool data within current session
- **Training Data**: General knowledge patterns from original LLM training (NOT reliable for specific project facts)

**When answering questions:**
1. **If evidence exists in context**: State facts with reference to source
2. **If no evidence in context**: Say "I don't have that information in the current context" 
3. **If memory is unclear**: Say "I don't remember" rather than guessing
4. **Never speculate**: Add "but this is speculation" if you must theorize

**Examples:**
- ❌ "I deleted the .gitignore entries" (speculation without evidence)
- ✅ "I don't remember if I modified .gitignore. Current context shows no MCP file exclusions."
- ❌ "The files were moved to a different location" (training data assumption)  
- ✅ "I don't see evidence of where those files went in the current context."

### 🚨 MANDATORY: Self-Improvement Framework
**EVERY TASK MUST FOLLOW**: Read and apply `docs/self-improvement-framework.md`

#### Required Process (NO EXCEPTIONS):
1. **Task Start**: Search past failures, design decisions, user instructions
2. **Concept Phase**: ONLY discussion, questions, understanding - NO CODE
3. **Design Phase**: Architecture, structure, interfaces - NO CODE
4. **Implementation Confirmation**: Use mcp-confirm before ANY code creation
5. **Implementation**: Progress reports, consultation when uncertain
6. **Completion**: Record lessons learned, update memories

#### Critical Memory Constraint Recognition:
- GitHub Copilot memory = 32k tokens only (forgets everything)
- Apologies are meaningless due to memory degradation
- MUST use permanent memory systems for continuity

#### Design Discussion Protocol:
**User shares conceptual ideas:**
1. Ask clarifying questions about goals and requirements
2. Explore architectural implications and constraints
3. Discuss alternative approaches and trade-offs
4. Document design decisions in external memory
5. ONLY proceed to implementation after explicit user approval

**Warning Signs of Implementation Rush:**
- User mentions "idea" or "concept" → Stay in discussion mode
- Feeling urge to write code → Use mcp-confirm first
- Unclear requirements → Continue design discussion

**🚨 CRITICAL: User Response Timeout Handling:**
- **When user response times out**: ALWAYS wait for user to respond
- **Never assume user intent**: Timeout means user needs more time to respond
- **Continue waiting patiently**: User will provide clarification when ready
- **No autonomous continuation**: Never proceed without explicit user instruction after timeout

## Project Overview

`llmcmd` is a command-line tool that enables Large Language Models (LLMs) to execute tasks using the OpenAI ChatCompletion API. The tool provides LLMs with secure, built-in functions for file operations and text processing without external command execution.

## Development Goals

1. **Security First**: All operations are sandboxed with built-in commands only
2. **Simplicity**: Clean, maintainable code using compiled languages
3. **Efficiency**: Optimized API usage with proper cost controls
4. **Extensibility**: Future-ready architecture for additional features
5. **Reliability**: Fail-First principle with contract programming

## Development Principles

### 🚨 CRITICAL: Fail-First Principle
- **Immediate Termination**: When an error occurs, terminate the program immediately
- **No Error Hiding**: NEVER hide, suppress, or continue processing after errors
- **Clear Error Messages**: Always provide clear, actionable error messages
- **Early Detection**: Detect and report errors as early as possible

### Contract Programming
- **Preconditions**: Validate all input parameters and system state before execution
- **Postconditions**: Verify expected outcomes where applicable
- **Assertions**: Use explicit checks for critical assumptions
- **Documentation**: Document all contracts in code comments

### Error Handling Rules
1. **Fatal Errors**: Use `log.Fatal()` or `os.Exit(1)` for unrecoverable errors
2. **Expected Errors**: Return explicit error values, handle at appropriate level
3. **Validation**: Check all inputs, nil pointers, and boundary conditions
4. **No Silent Failures**: Every error path must be visible and actionable

### Fallback Guidelines
**Immediate Termination Required:**
- **User Input Errors**: Parse errors, invalid values, wrong formats → User needs to fix
- **Configuration Errors**: Invalid settings, malformed files → User needs to correct
- **Logic Violations**: Precondition failures, contract violations → Programming error
- **Security Issues**: Authentication failures, permission denials → Must not continue

**Limited Fallback Allowed (with consultation):**
- **Missing Optional Files**: Use defaults when file absence is normal behavior
- **Network Transient Errors**: Retry with clear limits, then terminate
- **Resource Constraints**: Temporary issues that may resolve with retry

**Rule**: When in doubt, terminate immediately. User errors should cause immediate failure with clear error messages.

### Code Quality Standards
- **Defensive Programming**: Assume inputs are invalid until proven otherwise
- **Explicit Error Paths**: Every function that can fail must return an error
- **Resource Cleanup**: Always use proper cleanup (defer, close, etc.)
- **Testing**: Test error conditions extensively, not just happy paths

## Core Constraints

### Language Policy
- **Codebase**: English only (code, comments, variable names)
- **Commands**: English only
- **Runtime**: Japanese input allowed for user instructions
- **Documentation**: English preferred, Japanese acceptable for implementation notes

### Security Requirements
- **No external command execution**: All text processing via built-in functions
- **File access limited**: Only specified input/output files
- **API call limits**: Configurable maximum calls per session (default: 50)
- **Quota management**: Weighted token tracking with limits
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
- [x] Go module initialization  
- [x] Basic CLI argument parsing
- [x] Configuration file loading
- [x] Logging infrastructure

### Phase 2: OpenAI Integration (Days 4-7)
- [x] HTTP client implementation
- [x] OpenAI API type definitions
- [x] Authentication and error handling
- [x] Response parsing
- [x] Retry mechanisms

### Phase 3: Tool Implementation (Days 8-13)
- [x] read tool: File/stream reading with fd management
- [x] write tool: File/stream writing with size tracking
- [x] spawn tool: Background-only command execution and data transfer
- [x] exit tool: Program termination with cleanup

### Phase 4: Built-in Commands (Days 14-17)
- [x] cat: Data copying
- [x] grep: Pattern matching (basic regex)
- [x] sed: Text substitution (basic functionality)
- [x] head/tail: Line-based filtering
- [x] sort: Alphabetical sorting
- [x] wc: Counting (lines, words, characters)
- [x] tr: Character translation

### Phase 5: Integration & Testing (Days 18-22) - COMPLETED
- [x] Main application logic
- [x] Tool orchestration
- [x] Comprehensive error handling
- [x] Security feature integration
- [x] Performance optimization

### Phase 6: Advanced Features (v3.0.0) - COMPLETED
- [x] Complete Quota System with weighted tokens
- [x] Fail-First configuration validation
- [x] API call limits and enforcement
- [x] Enhanced error handling architecture
- [x] Real-time quota tracking and display

### Additional Enhancement: Preset Prompt System
- [x] Stage 1: CLI extension with --preset/-r flags
- [x] Stage 2: Configuration file extension with preset definitions
- [x] Stage 3: Preset resolution in application initialization
- [x] Built-in diff/patch commands for technical operations

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
- Configurable API call limits per session
- 300-second execution timeout
- Quota system with weighted token tracking (Input:1.0, Cached:0.25, Output:4.0)
- Real-time quota monitoring and enforcement
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
