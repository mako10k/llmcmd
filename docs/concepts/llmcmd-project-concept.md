# llmcmd Project Concept Document

## Status: ACTIVE  
**Created**: 2025-01-17  
**Last Updated**: 2025-01-17

## Project Overview

`llmcmd` is a secure command-line tool that enables Large Language Models (LLMs) to execute tasks using OpenAI ChatCompletion API with built-in function calling for file operations and text processing.

## Core Concept

### Security-First Design
- **No external command execution**: All operations through built-in functions only
- **Sandboxed operations**: Limited file access and secure processing
- **API cost controls**: Configurable limits and quota management

### Two-Tool Architecture
1. **llmcmd**: Main command-line interface for OpenAI API integration
2. **llmsh**: Mini shell environment with Virtual File System (VFS)

**Shared Components**:
- **VFS Implementation**: Common Virtual File System shared between llmcmd and llmsh

## Key Technical Concepts

### Virtual File System (VFS) Architecture
**3-Layer Design** (Shared between llmcmd and llmsh):
- **Client**: User interface layer
- **Proxy**: Translation and validation layer  
- **Server**: Actual I/O execution layer

**VFS Capabilities**:
- Memory-based virtual files
- Stream processing for large data
- Secure file descriptor management
- Pipeline support between tools
- **Cross-tool compatibility**: Same VFS implementation used by both llmcmd and llmsh

### Built-in Command System
**Text Processing Commands**:
- `cat`: Data copying and output
- `grep`: Pattern matching (regex support)
- `sed`: Text substitution
- `head/tail`: Line-based filtering
- `sort`: Alphabetical sorting
- `wc`: Counting (lines, words, characters)
- `tr`: Character translation

### Quota Management System
**Weighted Token Tracking**:
- Input tokens: 1.0x weight
- Cached tokens: 0.25x weight  
- Output tokens: 4.0x weight
- Real-time monitoring and enforcement

## Development Philosophy

### Design Principles
1. **Fail-First**: Immediate termination on errors
2. **Contract Programming**: Explicit preconditions and postconditions
3. **Security by Design**: No shell metacharacter interpretation
4. **Performance Optimization**: Stream-based processing

### Quality Standards
- **Memory Safety**: 4KB read buffers, 10MB file limits
- **Resource Management**: Proper cleanup and error handling
- **Testing Coverage**: ≥80% unit test coverage requirement
- **Documentation**: Complete technical documentation

## Current Status (as of 2025-01-17)

### Completed Systems
- ✅ **Core Architecture**: Go-based implementation with layered design
- ✅ **OpenAI Integration**: ChatCompletion API with function calling
- ✅ **VFS Implementation**: 3-layer virtual file system
- ✅ **Built-in Commands**: Complete text processing toolkit
- ✅ **Security Controls**: API limits and quota management
- ✅ **Tool Integration**: llmcmd + llmsh coordination

### Planned Architecture Change: VFS-Centralized LLM Execution

**New Design Concept**:
- **VFS Server becomes LLM execution center**: All OpenAI API calls routed through VFS Server
- **Simplified quota sharing**: Natural quota unification without complex SharedQuotaManager
- **Unified configuration**: Single point of OpenAI API configuration

**Migration Path**:
```
Current: llmcmd → OpenAI API, llmsh → OpenAI API (independent + SharedQuotaManager)
Target:  llmcmd → VFS Server → OpenAI API, llmsh → VFS Server → OpenAI API (unified)
```

**Benefits**:
1. **Simplified quota management**: No complex inter-process quota sharing logic
2. **Unified API configuration**: Single OpenAI configuration point  
3. **Consistent LLM execution**: All LLM calls through same execution pathway
4. **Enhanced debugging**: Single point for LLM call tracing and monitoring

### Technical Infrastructure
- **Language**: Go 1.21+
- **API**: OpenAI ChatCompletion with Function Calling
- **Build System**: Cross-platform binary generation
- **Development Tools**: MCP server integration for enhanced development workflow

## Future Directions

### Enhancement Areas
1. **Advanced VFS Features**: Extended virtual file capabilities
2. **Performance Optimization**: Stream processing improvements
3. **Security Hardening**: Additional sandboxing features
4. **Tool Ecosystem**: Additional built-in command extensions

### Quality Improvements
- Comprehensive test framework expansion
- Enhanced error handling and user experience
- Documentation system improvements
- Performance monitoring and optimization

## Related Documentation

**Design Documents** (to be created):
- [VFS Architecture Design](../designs/vfs-architecture.md)
- [Security Model Design](../designs/security-model.md)
- [API Integration Design](../designs/api-integration.md)
- [Testing Strategy Design](../designs/testing-strategy.md)

## Project Context

This tool addresses the need for secure LLM-to-system interaction without compromising system security. By providing built-in functions instead of shell access, it enables powerful text processing capabilities while maintaining strict security boundaries.

The project has evolved through multiple development sprints with a focus on quality, security, and maintainability over rapid feature addition.
