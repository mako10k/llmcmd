# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [3.1.0] - 2025-07-31

### Added
- **Dynamic Quota Management**: Output weight-aware token budgeting
  - Quota-aware input data reading with UTF-8 safe truncation
  - Real-time quota tracking with weighted token calculations
  - Dynamic token limits based on remaining quota with 4x output weight consideration
- **get_usages Tool Integration**: Complete usage statistics with 11 categories
- **Binary Processing Safety**: System prompt limits for binary file processing (4-16 byte chunks)
- **Tools-Disabled Mode Enhancement**: Token-aware input data inclusion for better context
- **Enhanced Error Handling**: Improved quota validation and edge case handling

### Fixed
- Missing get_usages tool switch case in tool execution engine
- Binary analysis infinite loop issues with proper timeout handling
- Input data visibility in tools-disabled mode
- Token estimation accuracy with 3.5 chars/token ratio
- Code formatting issues across the codebase

### Changed
- parseQuotaStatus function now properly considers 4x output weight in calculations
- Input data reading dynamically adjusts based on remaining quota
- Quota display shows both API calls and weighted token usage
- Improved verbose logging for quota monitoring

### Technical Details
- Quota integration with output weight consideration (4x multiplier)
- UTF-8 safe truncation for multi-byte character handling
- Dynamic token budgeting reserves 2000 weighted tokens for response
- Enhanced quota status parsing from API responses

## [3.0.3] - 2025-07-30

### Added
- Complete quota system with weighted tokens
- Model-specific quota weights and system prompts
- Preset prompt system with built-in presets
- Enhanced error handling architecture

### Fixed
- API call limits and enforcement
- Configuration validation improvements
- Tool execution reliability

## [3.0.0] - 2025-07-28

### Added
- Initial release with core functionality
- OpenAI API integration
- Built-in command tools (cat, grep, sed, head, tail, sort, wc, tr)
- Security-focused design with sandboxed execution
- Configuration file support
- Tool orchestration engine

### Features
- Secure file operations without external command execution
- Configurable API limits and quotas
- Comprehensive error handling
- Multi-model support
