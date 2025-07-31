# Release Notes - llmcmd v3.1.1

## Overview
This release completes the transition from `get_usages` tool to `help` tool for improved user experience and clarity.

## Key Changes

### 🔄 Tool Renaming
- **Breaking Change**: `get_usages` tool renamed to `help` for better clarity
- Updated all API definitions and system prompts
- Maintains backward compatibility for tool functionality

### 🛠️ Technical Improvements
- Complete OpenAI API integration updates
- Updated system prompts with new tool references
- All tests updated and passing
- Comprehensive code consistency across all modules

## Fixed Issues
- Tool name confusion between usage statistics and command help
- Inconsistent naming across codebase modules

## Files Modified
- `internal/tools/engine.go`: Tool execution routing updated
- `internal/tools/builtin/`: Command registration and implementation
- `internal/openai/types.go`: API tool definitions
- `internal/openai/client.go`: System prompt updates
- Test files: Complete test suite updates

## Compatibility
- **API**: Updated tool name from `get_usages` to `help`
- **Functionality**: No changes to tool behavior or output format
- **Configuration**: No configuration changes required

## Build Information
- Go version: 1.21+
- Platforms: Linux, macOS, Windows (AMD64 & ARM64)
- Build optimization: `-ldflags="-s -w"` for reduced binary size

## Download Sizes
- llmcmd binaries: ~5.1-5.5MB
- llmsh binaries: ~5.5-5.9MB

## Usage
The help tool provides comprehensive usage information:
```bash
# Basic operations
llmcmd --prompt "help(['basic_operations'])"

# Debugging information  
llmcmd --prompt "help(['debugging'])"

# Multiple categories
llmcmd --prompt "help(['data_analysis', 'text_processing'])"
```

## Verification
All platform binaries have been tested for:
- ✅ Compilation success
- ✅ Help tool functionality
- ✅ Core tool operations
- ✅ Cross-platform compatibility

---

**Full Changelog**: [View on GitHub](https://github.com/mako10k/llmcmd/compare/v3.1.0...v3.1.1)
