# System Prompt Redesign Plan

## 現状の問題
1. システムプロンプトが長大すぎる（トークンコストが高い）
2. 固定化したいがプロンプトキャッシュを活用できていない
3. 詳細な使用方法がシステムプロンプトに埋め込まれている

## 新しい設計方針

### 1. 短縮システムプロンプト
システムプロンプトは最小限の情報のみ含む：
- llmcmd/llmsh の基本的な役割
- 利用可能ツールのリスト（詳細な説明なし）
- get_usage ツールで詳細情報を取得する指示

### 2. get_usage ツールの実装
- 用途別の詳細な使用方法を動的に取得
- キー指定で必要な情報のみ取得
- トークンコストを最適化

## get_usage ツールのキー設計

### 利用可能キー一覧
1. **"basic"** - 基本的なワークフロー
2. **"file_io"** - ファイル入出力操作
3. **"shell_commands"** - shell コマンド実行
4. **"virtual_files"** - 仮想ファイル操作
5. **"workflows"** - 標準的な処理パターン
6. **"troubleshooting"** - エラー対処法
7. **"examples"** - 具体的な使用例

### キー別詳細内容

#### "basic"
- read/write/exit の基本操作
- 最小限のワークフロー
- FD の概念

#### "file_io"
- open/close の詳細
- モード指定
- PIPE動作の説明

#### "shell_commands"
- spawn の詳細使用法
- 利用可能な組み込みコマンド
- shell スクリプト構文

#### "virtual_files"
- 仮想ファイル作成・管理
- PIPE風動作の詳細
- 一度読み込み制限

#### "workflows"
- A) Simple Processing パターン
- B) Shell Command Processing パターン
- C) Virtual File Operations パターン
- D) File Analysis パターン

#### "troubleshooting"
- よくあるエラーと対処法
- デバッグ方法
- 失敗時のリカバリ

#### "examples"
- 実用的なコード例
- パターン別の実装例
- ベストプラクティス

## 短縮システムプロンプト案

```
You are llmcmd, a text processing assistant within the llmsh shell environment.

🏠 ENVIRONMENT: llmsh (LLM-powered shell)
🔧 TOOLS: read, write, open, spawn, close, exit, get_usage
🛠️ COMMANDS: Built-in text processing (cat, grep, sed, head, tail, sort, wc, tr)

📖 USAGE GUIDE:
Use get_usage(keys) for detailed information:
- basic: fundamental operations
- file_io: file operations  
- shell_commands: spawn execution
- virtual_files: temporary files
- workflows: standard patterns
- troubleshooting: error handling
- examples: code samples

🎯 WORKFLOW: read input → process → write output → exit
```

## 実装手順

### Phase 1: get_usage ツール実装
1. internal/tools/builtin/ に get_usage.go 作成
2. キー別コンテンツをハードコード実装
3. tools/engine.go に統合

### Phase 2: システムプロンプト短縮
1. client.go のシステムプロンプトを短縮版に置き換え
2. 長大な説明を get_usage のコンテンツに移行

### Phase 3: テスト・調整
1. 各キーの内容をテスト
2. トークン使用量の測定
3. 必要に応じてキー構成を調整

## 期待効果
- システムプロンプトのトークンコスト削減（推定70%減）
- プロンプトキャッシュの効果最大化
- 必要な情報のみ動的取得でコスト最適化
- より柔軟な情報提供
