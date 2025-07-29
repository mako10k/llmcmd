# llmcmd - LLM Command Line Tool

A secure command-line tool that enables Large Language Models to execute text processing tasks using the OpenAI ChatCompletion API.

## Overview

`llmcmd` is a command-line tool that allows you to instruct large language models (LLMs) to perform tasks using built-in tools for file operations and text processing. All operations are sandboxed and secure, with no external command execution.

## Features

- **Natural Language Interface**: Instruct tasks in plain language
- **Secure Built-in Tools**: File reading/writing, text processing, statistics
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

# Security & Rate Limiting
max_api_calls=50
timeout_seconds=300
max_file_size=10485760    # 10MB
read_buffer_size=4096     # 4KB

# Retry Configuration
max_retries=3
retry_delay_ms=1000

# Advanced Options
# system_prompt=           # Custom system prompt
# disable_tools=false      # Disable LLM tools
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

### 基本的な使い方

```bash
llmcmd "指示内容"
```

### オプション

```bash
llmcmd [options] <instructions>

Options:
  -p, --prompt string   LLMへの指示
  -i, --input string    入力ファイル
  -o, --output string   出力ファイル
  -v, --verbose         冗長出力
  -V, --version         バージョン表示
  -h, --help            ヘルプ表示
```

## 使用例

### ファイル変換

```bash
# CSVをJSON形式に変換
llmcmd -i data.csv -o data.json "CSVファイルをJSON形式に変換してください"

# テキストファイルを整形
llmcmd -i input.txt -o output.txt "テキストを整形し、重複行を削除してください"
```

### データ分析

```bash
# ログファイルの分析
llmcmd -i access.log "ログファイルを分析し、エラーの統計を出力してください"

# CSVデータの統計
llmcmd -i sales.csv "売上データの月別統計を計算し、グラフ用のデータを生成してください"
```

### 標準入出力の使用

```bash
# パイプライン処理
cat data.txt | llmcmd "データを分析し、重要な情報を抽出してください"

# 複数ファイルの処理
find . -name "*.log" | llmcmd "ログファイルのリストから、エラーを含むファイルを特定してください"
```

## Available Tools for LLM

### read(fd, count|lines)
Reads data from file descriptors or streams.

**Parameters**:
- `fd`: File descriptor number (0=stdin, 3+=input files)
- `count`: Number of bytes to read (1-4096, default: 4096)
- `lines`: Number of lines to read (1-1000, default: 40) - alternative to count

**Response example**:
```json
{
  "input": "read data content",
  "size": 1024
}
```

### write(fd, data, newline)
Writes data to file descriptors or output streams.

**Parameters**:
- `fd`: File descriptor number (1=stdout, 2=stderr)
- `data`: Data to write
- `newline`: Whether to add newline at the end (true/false, default: false)

**Response example**:
```json
{
  "success": true,
  "size": 1024
}
```

### pipe(commands, input)
Executes pipeline of built-in commands.

**Supported commands**: cat, grep, sed, head, tail, sort, wc, tr, cut, uniq, nl, tee, rev

**Response example**:
```json
{
  "success": true,
  "output": "processed result"
}
```

### fstat(fd)
Gets file information and statistics for a file descriptor.

**Parameters**:
- `fd`: File descriptor number

**Response example**:
```json
{
  "fd": 3,
  "type": "file",
  "name": "input.txt",
  "size": 1024,
  "readable": true,
  "writable": false
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
