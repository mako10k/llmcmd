# llmcmd - LLM Command Line Tool

OpenAI ChatCompletion APIを使用して、LLMがコマンドラインツールとして動作するためのプログラムです。

## 概要

`llmcmd`は、大規模言語モデル（LLM）に対してタスクを指示し、LLMが提供されたツールを使ってファイル操作やコマンド実行を行うことができるコマンドラインツールです。

## 特徴

- **シンプルな使用方法**: 自然言語でタスクを指示
- **豊富なツール**: ファイルの読み書き、外部コマンド実行、パイプ処理
- **セキュリティ**: コマンド制限、ファイルサイズ制限
- **クロスプラットフォーム**: Linux、macOS、Windows対応
- **単一バイナリ**: 依存関係を含む実行ファイル

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

## 設定

### OpenAI APIキーの設定

#### 環境変数（推奨）

```bash
export OPENAI_API_KEY="sk-..."
```

#### 設定ファイル

`~/.llmcmdrc`ファイルを作成：

```bash
OPENAI_API_KEY=sk-...
LLMCMD_MAX_INPUT_BYTES=10485760
LLMCMD_MAX_OUTPUT_BYTES=10485760
LLMCMD_ALLOWED_CMDS=ls,cat,grep,sort,wc,head,tail
```

## 使用方法

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

## LLMが使用できるツール

### read(in_id, offset, size)
ファイルまたは入力ストリームからデータを読み取ります。

```json
{
  "input": "読み取ったデータ",
  "next_offset": 1024,
  "eof": true,
  "size": 1024,
  "error": null
}
```

### write(out_id, data)
ファイルまたは出力ストリームにデータを書き込みます。

```json
{
  "success": true,
  "error": null
}
```

### pipe(cmd, in_id, out_id)
外部コマンドを実行し、パイプで接続します。

```json
{
  "success": true,
  "in_id": 2,
  "out_id": 3,
  "error": null
}
```

### exit(exitcode)
プログラムを終了します。

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
