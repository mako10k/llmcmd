# llmcmd - LLM Command Line Tool

## QA大原則（品質保証基本方針）

### 四大原則
1. **Silent Fallback 実装の厳禁**
   - エラー発生時の無通知処理継続を禁止
   - Fail-First原則の徹底（errcheck、staticcheckで検出）

2. **同等機能の重複実装の厳禁**
   - 42コマンド重複問題の根本解決
   - jscpd、gocriticで検出、共通化・interface化推進

3. **ファイル長大化の厳禁**
   - 1ファイル1000行未満、1関数50行未満（推奨）
   - 適切なモジュール分割・責任分界の実施

4. **複雑度の増大の厳禁**
   - 循環的複雑度10未満（gocycloで測定）
   - 関数分割・Early Returnパターンで簡素化

### 自動チェック体制
- **pre-commitフック**: errcheck、jscpd、gocyclo、file-size-check
- **CIパイプライン**: GitHub Actions連携、PR時必須チェック
- **静的解析ツール**: gosec、golangci-lint、staticcheck

### 段階的導入
- Sprint 1: ツール導入・現状把握
- Sprint 2: 重大違反修正
- Sprint 3以降: 基準厳格化・監査自動化

### 🛡️ Premium Request Protection Protocol
**プレミアムリクエスト成果物保護原則**

#### 保護対象（価値ベース判定）
- **デバッグ・現実融和済み実装**: エラー解決、システム統合、実動作確認を経た成果物
- **時間投入済みコンポーネント**: 30分以上の実装・調整・検証を要した実装
- **現実知識統合済み**: 純粋生成を超えて実環境での問題解決を含む成果物

#### 保護対象外（再生成可能）
- **純粋パターン生成**: デバッグ・検証なしの標準的生成物
- **試作・実験段階**: 未検証・未統合の実装
- **明らかに再利用不可**: 一時的・特定状況限定の成果物

#### 保護手順
1. **WIPブランチ保存**: `feature/priority[N]-[feature-name]` 形式でブランチ作成
2. **詳細コミットメッセージ**: 実装内容、技術判断理由、保存根拠を明記
3. **リモートプッシュ**: GitHub上での永続保存を確実に実行
4. **代替検討**: 削除ではなく代替実装・改良アプローチを優先

#### 判断基準
- **保存基準**: デバッグ・現実融和プロセスを経た貴重な知識
- **保存判断**: ユーザーとの協議による価値ベース判定
- **文書化**: 保存理由・現実融和プロセスを必ず記録

#### 例外処理
- **セキュリティリスク**: 明確なセキュリティ問題がある場合のみ削除検討
- **法的問題**: ライセンス違反等の法的問題がある場合のみ削除検討
- **事前相談**: 削除が必要な場合は必ずユーザーと事前相談

**原則**: "Preserve debugged reality-integrated work, not just patterns" - デバッグ・現実融和した成果物は貴重な知識として保護、純粋生成物は再生成可能

## Project Team (MCP LLM Generator Contexts)

### Core Team Context IDs
- **PersonalityManager**: `context-mdtvqb20-opmlhg` - 人格管理マネージャー（チーム構成・役割設計）
- **ProductOwner**: `context-mdtvs82o-1ekq4d` - 製品責任者（ビジョン策定・要件優先度決定）
- **ScrumMaster**: `context-mdtvso8o-bljl6h` - プロセス促進者（障害除去・チーム生産性向上）
- **TechnicalLead**: `context-mdtvu1aq-e07dnk` - 技術責任者（アーキテクチャ設計・技術意思決定）

### Quality & Process Team Context IDs
- **QAEngineer**: `context-mdtvvnz6-exfq23` - 品質保証エンジニア（テスト戦略・品質基準・自動化推進）
- **GitWorkflowSpecialist**: `context-mdtvvzyi-95b60o` - Git管理スペシャリスト（ブランチ戦略・リリース管理）
- **PragmaticAdvisor**: `context-mdtvwj2v-h5ld7v` - 現実的調整役（率直な意見・実装可能性・リスク評価）

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

### 🧠 Associative Memory Usage for Development Workflow

**Purpose**: Use MCP Associative Memory (`#mcp-mcp-assoc-mem`) as external memory augmentation for complex development projects.

**MANDATORY Usage Patterns:**
1. **Project State Persistence**: 
   - `memory_store` critical findings, architectural discoveries, implementation gaps
   - Store immediately after major technical discoveries or design decisions
   - Include context and rationale, not just raw facts

2. **Context Continuity**: 
   - `memory_search` before starting new analysis to avoid redundant work
   - Maintain knowledge across sessions and conversation boundaries
   - Build on previous discoveries rather than reanalyzing

3. **Discovery Tracking**:
   - Record important code findings, especially in large codebases
   - Track implementation status, component relationships
   - Update assessments when new discoveries change understanding

4. **Sprint & Project Management**:
   - Store sprint progress, issues, retrospective learnings
   - Track technical debt, architectural decisions
   - Preserve strategic direction and priority rationale

**Memory Organization Strategy:**
```
work/projects/llmcmd/     # Project-specific knowledge
  ├── architecture/       # System design insights
  ├── implementation/     # Code structure findings  
  ├── sprint-management/  # Agile process tracking
  └── technical-issues/   # Problems and solutions

workflow/                 # Cross-project methodologies
  ├── memory-usage/       # Meta-usage patterns
  ├── development/        # General dev practices
  └── tools/             # Tool-specific learnings
```

**Integration Rules:**
- **Search First**: Always `memory_search` relevant topics before deep analysis
- **Store Immediately**: Use `memory_store` after significant findings or decisions
- **Update When Changed**: Correct previous assessments with new discoveries
- **Use Descriptive Categories**: Enable future searchability with clear tags
- **Include Context**: Store not just what, but why and how decisions were made

**Benefits:**
- Prevents redundant analysis of large codebases
- Maintains project knowledge across development sessions  
- Enables faster onboarding and context switching
- Creates searchable project knowledge base
- Supports complex, multi-session development workflows

**Example Workflow:**
```
1. memory_search "VFS implementation llmcmd" 
2. Analyze findings, build on existing knowledge
3. memory_store new discoveries with context
4. Continue development with enhanced understanding
```

This creates persistent, searchable knowledge that augments GitHub Copilot's capabilities for complex, long-term development projects.

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
