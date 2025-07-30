# llmcmd - LLM Command Line Tool

[![Version](https://img.shields.io/badge/version-3.0.0-blue.svg)](https://github.com/mako10k/llmcmd/releases)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org/)

A secure command-line tool that enables Large Language Models to execute text processing tasks using the OpenAI ChatCompletion API with advanced quota management and specialized prompt presets.

## Overview

`llmcmd` is a command-line tool that allows you to instruct large language models (LLMs) to perform tasks using built-in tools for file operations and text processing. All operations are sandboxed and secure, with no external command execution.

**Version 3.0.0** introduces the **Complete Quota System** with weighted token tracking, **Fail-First validation**, and enhanced API call management - alongside the existing **Preset Prompt System** for specialized task optimization.

## Features

- **Natural Language Interface**: Instruct tasks in plain language
- **Complete Quota System**: Weighted token tracking with real-time monitoring and limits
- **Fail-First Validation**: Strict configuration validation with immediate error reporting
- **API Call Management**: Configurable limits with graceful termination on final calls
- **Preset Prompt System**: Specialized prompts for different task types (general, diff/patch, code review, data processing)
- **Smart File Analysis**: Automatic file information pre-loading and size/type detection
- **Secure Built-in Tools**: File reading/writing, text processing pipelines
- **No External Commands**: All operations use built-in functions only
- **Cross-platform**: Linux, macOS, Windows support
- **Single Binary**: Self-contained executable with no dependencies
- **API Integration**: Powered by OpenAI ChatCompletion API with function calling

## Installation

### Quick Install (Linux/macOS)

```bash
curl -sSL https://raw.githubusercontent.com/mako10k/llmcmd/main/install.sh | bash
```

### Manual Installation

#### Download Binary (Recommended)

Download the latest release binary for your platform:

```bash
# Linux AMD64
wget https://github.com/mako10k/llmcmd/releases/latest/download/llmcmd-linux-amd64
chmod +x llmcmd-linux-amd64
sudo mv llmcmd-linux-amd64 /usr/local/bin/llmcmd

# Linux ARM64
wget https://github.com/mako10k/llmcmd/releases/latest/download/llmcmd-linux-arm64
chmod +x llmcmd-linux-arm64
sudo mv llmcmd-linux-arm64 /usr/local/bin/llmcmd

# macOS AMD64 (Intel)
wget https://github.com/mako10k/llmcmd/releases/latest/download/llmcmd-darwin-amd64
chmod +x llmcmd-darwin-amd64
sudo mv llmcmd-darwin-amd64 /usr/local/bin/llmcmd

# macOS ARM64 (Apple Silicon)
wget https://github.com/mako10k/llmcmd/releases/latest/download/llmcmd-darwin-arm64
chmod +x llmcmd-darwin-arm64
sudo mv llmcmd-darwin-arm64 /usr/local/bin/llmcmd

# Windows AMD64
# Download llmcmd-windows-amd64.exe and place it in your PATH
```

#### Build from Source

```bash
git clone https://github.com/mako10k/llmcmd.git
cd llmcmd
make build
sudo make install
```

#### System Installation from Local Binary

If you already have the binary built locally:

```bash
# Build first
go build -o llmcmd ./cmd/llmcmd

# Install system-wide (requires sudo)
sudo ./llmcmd --install
```

## Configuration

### OpenAI API Key Setup

#### Environment Variable (Recommended)

```bash
export OPENAI_API_KEY="sk-your-api-key-here"
```

#### Configuration File

Create a configuration file at `~/.llmcmdrc`:

```bash
# Copy the example configuration
curl -sL https://raw.githubusercontent.com/mako10k/llmcmd/main/.llmcmdrc.example -o ~/.llmcmdrc

# Edit with your API key
nano ~/.llmcmdrc
```

### Configuration Options

The configuration file supports the following options:

```ini
# OpenAI API Configuration
openai_api_key=your-api-key-here
model=gpt-4o-mini
max_tokens=4096
temperature=0.1

# API Call & Quota Management
max_api_calls=50
quota_max_tokens=0          # 0 = unlimited, or set a limit
quota_weights_input=1.0     # Weight for input tokens
quota_weights_cached=0.25   # Weight for cached input tokens  
quota_weights_output=4.0    # Weight for output tokens

# Security & Rate Limiting
timeout_seconds=300
max_file_size=10485760    # 10MB
read_buffer_size=4096     # 4KB

# Retry Configuration
max_retries=3
retry_delay_ms=1000

# Prompt Configuration
default_prompt=general     # Default preset to use when no --prompt or --preset specified

# Advanced Options
# system_prompt=           # Custom system prompt
# disable_tools=false      # Disable LLM tools
```

#### Custom Preset Configuration

You can define custom presets in your configuration file using JSON format:

```ini
# Custom presets (JSON format)
prompt_presets={
  "my_custom": {
    "key": "my_custom",
    "description": "My custom prompt for specific tasks",
    "content": "You are a specialist for my specific use case..."
  },
  "debug": {
    "key": "debug", 
    "description": "Debug-focused prompt with detailed logging",
    "content": "You are a debugging assistant. Provide detailed step-by-step analysis..."
  }
}
```

### Environment Variables

You can also configure via environment variables:

```bash
export OPENAI_API_KEY="sk-your-api-key-here"
export LLMCMD_MODEL="gpt-4o-mini"
export LLMCMD_MAX_TOKENS="4096"
export LLMCMD_TEMPERATURE="0.1"
export LLMCMD_MAX_API_CALLS="50"
export LLMCMD_TIMEOUT="300"
```

### Configuration Priority

Settings are applied in this order (highest to lowest priority):
1. Command line options
2. Configuration file (`~/.llmcmdrc`)
3. Environment variables
4. Default values

## Security Considerations

⚠️ **Important Privacy Notice**

When using llmcmd, be aware that:

- **All input data is sent to OpenAI's API** for processing
- **Sensitive information** (passwords, API keys, personal data) in your input files or stdin will be transmitted to OpenAI
- **Configuration files** containing secrets should not be processed directly
- **Log files** may contain sensitive data - use `-v` (verbose) option with caution

### Best Practices

```bash
# ❌ DON'T: Process files containing sensitive data
llmcmd -i ~/.ssh/private_key "analyze this file"
llmcmd -i database_passwords.txt "summarize this"

# ✅ DO: Sanitize or exclude sensitive content first
grep -v "password\|secret\|key" config.txt | llmcmd "analyze this configuration"
```

### Data Handling

- **Your responsibility**: Ensure no sensitive data is sent to the API
- **OpenAI's policies**: Refer to [OpenAI's Privacy Policy](https://openai.com/privacy) for data handling details
- **Logging**: Disable verbose logging (`-v`) when processing sensitive content

## Usage

### Basic Usage

```bash
# Use default general-purpose prompt
llmcmd "your instructions here"

# Use specific preset for specialized tasks
llmcmd --preset diff_patch "compare these files"
llmcmd -r code_review "analyze this code"
```

### Command Line Options

```bash
llmcmd [options] <instructions>

Options:
  -p, --prompt <text>     LLM prompt/instructions (free text)
  -r, --preset <key>      Use predefined prompt preset (see --list-presets)
  --list-presets          List available prompt presets and exit
  -i, --input <file>      Input file path (can be specified multiple times)
  -o, --output <file>     Output file path  
  -c, --config <file>     Configuration file path (default: ~/.llmcmdrc)
  -v, --verbose           Enable verbose logging
  -s, --stats             Show detailed statistics after execution
  -n, --no-stdin          Skip reading from stdin
  -h, --help              Show this help message
  -V, --version           Show version information
```

### Preset Prompt System

llmcmd includes specialized prompts optimized for different task types:

```bash
# List available presets
llmcmd --list-presets

# Available presets:
#   general      - General-purpose prompt for various tasks
#   diff_patch   - Specialized prompt for diff/patch operations and file comparison
#   code_review  - Focused prompt for code analysis and review tasks
#   data_proc    - Optimized prompt for data processing and text manipulation
```

#### Preset Usage Examples

```bash
# File comparison with specialized diff prompt
llmcmd --preset diff_patch -i file1.txt -i file2.txt "Compare these files"

# Code analysis with review-focused prompt  
llmcmd --preset code_review -i main.go "Analyze this code for potential issues"

# Data processing with specialized prompt
llmcmd --preset data_proc -i data.csv "Process this CSV and extract unique values"

# General tasks (default behavior)
llmcmd --preset general "Help me with this text transformation"
# or simply:
llmcmd "Help me with this text transformation"
```

## Examples

### File Operations

```bash
# Data conversion with general preset
llmcmd -i data.csv -o data.json "Convert this CSV file to JSON format"

# Text processing with data processing preset
llmcmd --preset data_proc -i input.txt -o output.txt "Clean up text and remove duplicates"
```

### Code Analysis

```bash
# Code review with specialized preset
llmcmd --preset code_review -i main.go "Analyze this code for potential issues and improvements"

# Multiple file code analysis
llmcmd --preset code_review -i *.go "Review all Go files for security vulnerabilities"
```

### File Comparison & Diff Operations

```bash
# File comparison with specialized diff preset
llmcmd --preset diff_patch -i file1.txt -i file2.txt "Compare these files and show differences"

# Apply patch with diff preset
llmcmd --preset diff_patch -i changes.patch -i original.txt "Apply this patch to the file"

# Generate patch file
llmcmd --preset diff_patch -i old.txt -i new.txt -o changes.patch "Generate a patch file"
```

### Data Processing Tasks

```bash
# Log analysis with data processing preset
llmcmd --preset data_proc -i access.log "Analyze log file and extract error statistics"

# CSV data analysis with specialized preset
llmcmd --preset data_proc -i sales.csv "Calculate monthly statistics and generate summary"
```

### Pipeline Operations

```bash
# Standard input/output processing
cat data.txt | llmcmd --preset data_proc "Analyze and extract important information"

# Multi-file processing
find . -name "*.log" | llmcmd --preset data_proc "Identify files containing errors"
```

### Smart File Analysis

```bash
# Large file analysis (automatic file info preloading)
llmcmd --preset data_proc -i large_dataset.csv "Summarize this dataset without loading all content"

# Multiple file types with appropriate presets
llmcmd -i large_data.csv "このCSVファイルの構造を分析し、適切な処理方法を提案してください"

# 複数ファイルの統合処理（各ファイルの情報が事前に把握されます）
llmcmd -i config.json -o settings.conf "JSONファイルを設定ファイル形式に変換してください"

# ファイル形式の自動判定とカスタム処理
llmcmd -i unknown_format.txt "ファイルの内容と形式を判定し、適切な処理を行ってください"
```

## Available Tools for LLM

### read(fd, [lines], [count])
Reads data from file descriptors or streams.

**Parameters**:
- `fd`: File descriptor number (0=stdin, 3+=input files)
- `lines`: Number of lines to read (optional, alternative to count)
- `count`: Number of bytes to read (optional, alternative to lines)

**Response example**:
```json
{
  "input": "read data content",
  "size": 1024
}
```

### write(fd, data, [newline])
Writes data to file descriptors or output streams.

**Parameters**:
- `fd`: File descriptor number (1=stdout, 2=stderr, or pipe input fd)
- `data`: Data to write
- `newline`: Whether to add newline at the end (optional, default: false)

**Response example**:
```json
{
  "success": true,
  "size": 1024
}
```

### pipe(cmd, [args], [in_fd], [out_fd], [size])
Executes built-in commands with flexible input/output patterns.

**Four Execution Patterns**:
1. `pipe({cmd, args})` → `{in_fd, out_fd}` - Background execution with new file descriptors
2. `pipe({cmd, args, in_fd, size})` → `{out_fd}` - Background with input from existing fd
3. `pipe({cmd, args, out_fd})` → `{in_fd}` - Background with output to existing fd
4. `pipe({cmd, args, in_fd, out_fd, [size]})` → `{exit_code}` - Foreground synchronous execution

**Supported commands**: cat, grep, sed, head, tail, sort, wc, tr, cut, uniq, nl, rev

**Response examples**:
```json
// Pattern 1: Background with new fds
{"success": true, "in_fd": 10, "out_fd": 11}

// Pattern 4: Foreground execution
{"success": true, "exit_code": 0}
```

### tee(in_fd, out_fds)
Copies input from one file descriptor to multiple outputs (1:many relationship).

**Parameters**:
- `in_fd`: Source file descriptor to read from
- `out_fds`: Array of destination file descriptors (1=stdout, 2=stderr, or other fds)

**Response example**:
```json
{
  "success": true,
  "bytes_copied": 1024
}
```

### close(fd)
Closes file descriptor and waits for command termination. Returns exit code for command input fds.

**Parameters**:
- `fd`: File descriptor to close (respects dependency order to prevent deadlock)

**Response example**:
```json
{
  "success": true,
  "exit_code": 0,
  "message": "Command 'grep [pattern]' terminated with exit code 0"
}
```

### exit(code)
Terminates the program.

**Parameters**:
- `code`: Exit code (0=success, 1-255=error)

**Response example**:
```json
{
  "success": true,
  "message": "Exit requested with code 0"
}
```

## Advanced Features

### Background Command Execution
`llmcmd` supports sophisticated background command execution with proper process management:

- **Asynchronous Processing**: Commands run in background goroutines
- **File Descriptor Management**: Automatic fd allocation and cleanup
- **Exit Code Handling**: Proper command termination and result reporting
- **Deadlock Prevention**: Dependency-aware close() ordering

### Pipeline Chaining
Create complex processing pipelines by chaining commands:

```bash
# Example: Multi-step text processing
echo "data" | llmcmd "extract lines containing 'error', sort them, and count unique entries"
```

The LLM will automatically:
1. Use `pipe()` to start grep in background
2. Use `pipe()` to start sort with grep output as input  
3. Use `pipe()` to start uniq with sort output
4. Return final count

### Dependency Management
Automatic dependency tracking prevents deadlocks:

- **pipe()** creates 1:1 dependencies (input_fd → output_fd)
- **tee()** creates 1:many dependencies (input_fd → [output_fds])
- **close()** enforces proper order (outputs before inputs)

```json
// Safe closing order
close({fd: 11})  // Output fd first
close({fd: 10})  // Input fd second, returns exit code
```

## Smart File Information Pre-loading

`llmcmd` automatically analyzes input files and provides comprehensive file information upfront to help the LLM make informed decisions about processing large or binary files.

### File Descriptor Mapping with Pre-loaded Information

```
FILE DESCRIPTOR MAPPING:
- fd=0: stdin <- large_data.csv [2.1 MB, text, large]
- fd=1: stdout -> output.txt
- fd=2: stderr (error output)
- fd=3: archive.tar.gz (input file #1) [15.0 MB, archive, very_large]
```

### File Information Categories

- **Size Display**: Automatic conversion to appropriate units (bytes/KB/MB/GB)
- **File Type Detection**: text, binary, archive, structured_text, image, etc.
- **Size Categories**: small, medium, large, very_large
- **Stream Detection**: Identifies pipes, terminals, and file redirections

This feature helps prevent expensive API calls with inappropriate content and enables smarter processing strategies for large files.

## セキュリティ

### コマンド制限

`LLMCMD_ALLOWED_CMDS`環境変数または設定ファイルで、実行可能なコマンドを制限できます：

```bash
LLMCMD_ALLOWED_CMDS=ls,cat,grep,sort,wc,head,tail,awk,sed
```

### ファイルサイズ制限

大きなファイルによるメモリ枯渇を防ぐため、入出力ファイルサイズに制限を設けています：

```bash
LLMCMD_MAX_INPUT_BYTES=10485760    # 10MB
LLMCMD_MAX_OUTPUT_BYTES=10485760   # 10MB
```

## トラブルシューティング

### よくある問題

#### APIキーエラー
```
Error: OpenAI API key not found
```
→ `OPENAI_API_KEY`環境変数または設定ファイルを確認してください。

#### コマンド実行エラー
```
Error: command 'rm' not allowed
```
→ `LLMCMD_ALLOWED_CMDS`設定を確認し、必要なコマンドを追加してください。

#### ファイルサイズエラー
```
Error: input file too large
```
→ `LLMCMD_MAX_INPUT_BYTES`の値を増やすか、より小さなファイルを使用してください。

### ログの確認

詳細なログを確認するには、`-v`オプションを使用してください：

```bash
llmcmd -v "タスクの指示"
```

## 開発

### 要件

- Go 1.21+
- OpenAI API キー

### ビルド

```bash
git clone https://github.com/mako10k/llmcmd.git
cd llmcmd
go mod download
go build -o llmcmd ./cmd/llmcmd
```

### テスト

```bash
go test ./...
```

### Code Quality & Duplication Detection

This project uses `jscpd` for duplicate code detection:

```bash
# Install dependencies (run once)
npm install

# Run duplicate detection
npm run cpd               # Basic detection
npm run cpd:report        # Generate HTML + JSON reports
npm run cpd:verbose       # Verbose output with details

# View reports
open reports/jscpd/html/index.html  # macOS
xdg-open reports/jscpd/html/index.html  # Linux
```

The duplication detection is configured to:
- Minimum 5 lines or 50 tokens for detection
- Skip large files (>1000 lines) like `engine.go`
- Generate detailed HTML reports with source highlighting
- Export JSON data for CI/CD integration

### 貢献

Issues や Pull Requests を歓迎します。開発ガイドラインは `.github/copilot-instructions.md` を参照してください。

## ライセンス

MIT License - 詳細は `LICENSE` ファイルを参照してください。

## ドキュメント

- [仕様書](docs/specification.md)
- [技術選択](docs/technical-decisions.md)
- [OpenAI API調査レポート](docs/openai-api-research.md)
- [実装計画](docs/implementation-plan.md)

## サポート

- GitHub Issues: バグ報告や機能要求
- Discussions: 質問や使用方法の相談

---

**注意**: このツールは開発中です。本番環境での使用前に十分にテストしてください。
