# 技術選択と実装方針

## 言語選択: Go言語

### 選択理由

#### 1. シンプルさと保守性
- **読みやすい構文**: C言語ライクでありながら、現代的な機能を持つ
- **標準ライブラリの充実**: HTTP、JSON、ファイル操作など必要な機能が標準で提供
- **ガベージコレクション**: メモリ管理の負担軽減
- **静的型付け**: コンパイル時のエラー検出

#### 2. 運用面での優位性
- **単一バイナリ**: 依存関係を含む単一実行ファイルの生成
- **クロスプラットフォーム**: Linux、macOS、Windows対応
- **高速なコンパイル**: 開発サイクルの短縮
- **豊富なツール**: fmt、vet、testなど開発支援ツールが標準

#### 3. HTTP/JSON 処理の優秀さ
- **net/http**: 高性能なHTTPクライアント・サーバー
- **encoding/json**: 高速で使いやすいJSON処理
- **context**: タイムアウトとキャンセレーション管理

### 他言語との比較

| 項目 | Go | Rust | C |
|------|----|----|---|
| 学習コストl | 中 | 高 | 低 |
| 開発速度 | 高 | 中 | 低 |
| 実行性能 | 高 | 最高 | 最高 |
| メモリ安全性 | 中 | 最高 | 低 |
| HTTP/JSON標準サポート | 優秀 | 外部crate必要 | 外部ライブラリ必要 |
| 配布のしやすさ | 優秀 | 優秀 | 中 |

**結論**: シンプルさ、開発効率、標準ライブラリの充実度を重視してGo言語を選択

## アーキテクチャ設計

### レイヤードアーキテクチャ

```
┌─────────────────────────────────┐
│           CLI Layer             │  コマンドライン解析
├─────────────────────────────────┤
│        Application Layer        │  メインロジック
├─────────────────────────────────┤
│         Service Layer           │  OpenAI API, ツール実行
├─────────────────────────────────┤
│       Infrastructure Layer      │  ファイルI/O, プロセス実行
└─────────────────────────────────┘
```

### ディレクトリ構造

```
llmcmd/
├── cmd/llmcmd/           # エントリーポイント
│   └── main.go
├── internal/             # 内部パッケージ
│   ├── cli/              # CLI関連
│   │   ├── parser.go     # 引数解析
│   │   └── config.go     # 設定管理
│   ├── app/              # アプリケーションロジック
│   │   ├── app.go        # メインアプリケーション
│   │   └── context.go    # 実行コンテキスト
│   ├── openai/           # OpenAI API クライアント
│   │   ├── client.go     # APIクライアント
│   │   ├── types.go      # 型定義
│   │   └── tools.go      # ツール定義
│   ├── tools/            # ツール実装
│   │   ├── executor.go   # ツール実行エンジン
│   │   ├── read.go       # readツール
│   │   ├── write.go      # writeツール
│   │   ├── pipe.go       # pipeツール
│   │   └── exit.go       # exitツール
│   └── security/         # セキュリティ関連
│       ├── commands.go   # コマンド許可リスト
│       └── limits.go     # 制限チェック
├── pkg/                  # 外部公開可能パッケージ（将来拡張用）
├── docs/                 # ドキュメント
├── examples/             # 使用例
├── go.mod
├── go.sum
└── README.md
```

## 依存関係管理

### 基本方針: 最小依存

標準ライブラリを最大限活用し、外部依存関係を最小限に抑制

### 使用予定の標準ライブラリ

```go
import (
    "bytes"
    "context" 
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
    "time"
)
```

### 外部ライブラリ候補（必要に応じて）

| ライブラリ | 用途 | 必要性 |
|-----------|------|--------|
| なし | CLI解析 | 標準のflagで十分 |
| なし | HTTP クライアント | 標準のnet/httpで十分 |
| なし | JSON処理 | 標準のencoding/jsonで十分 |

## エラーハンドリング戦略

### エラー分類

1. **システムエラー**: OS、ファイルシステム関連
2. **設定エラー**: 設定ファイル、環境変数関連  
3. **APIエラー**: OpenAI API関連
4. **セキュリティエラー**: 許可されていない操作
5. **ユーザーエラー**: 不正な引数、ファイル不存在など

### エラーハンドリングパターン

```go
// カスタムエラー型
type LLMCmdError struct {
    Type    ErrorType
    Message string
    Cause   error
}

type ErrorType int

const (
    ErrorTypeSystem ErrorType = iota
    ErrorTypeConfig
    ErrorTypeAPI
    ErrorTypeSecurity
    ErrorTypeUser
)

func (e *LLMCmdError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %v", e.Message, e.Cause)
    }
    return e.Message
}

// エラーラッピング
func WrapError(errType ErrorType, message string, cause error) error {
    return &LLMCmdError{
        Type:    errType,
        Message: message,
        Cause:   cause,
    }
}
```

## セキュリティ設計

### 多層防御

1. **入力検証**: コマンドライン引数、設定値の検証
2. **コマンド制限**: 実行可能コマンドのホワイトリスト
3. **ファイルサイズ制限**: メモリ消費とディスク使用量の制御
4. **パス正規化**: ディレクトリトラバーサル攻撃の防止

### 実装方針

```go
// セキュリティチェック関数群
func ValidateCommand(cmd string, allowedCmds []string) error
func ValidateFileSize(path string, maxSize int64) error  
func NormalizePath(path string) (string, error)
func ValidateAPIKey(key string) error
```

## パフォーマンス設計

### 目標指標

- **起動時間**: 100ms以下
- **メモリ使用量**: 50MB以下（ベースライン）
- **レスポンス時間**: OpenAI API + 処理時間
- **ファイル処理**: ストリーミング処理で大ファイル対応

### 最適化ポイント

1. **HTTP接続の再利用**: keep-alive接続
2. **JSON ストリーミング**: 大きなレスポンスの段階的処理
3. **ファイルストリーミング**: メモリ効率的なファイル処理
4. **Goroutine活用**: I/O待機時間の最適化

## テスト戦略

### テストピラミッド

```
        E2E Tests (少数)
    ┌─────────────────────┐
    │   Integration Tests │  (適度)
    ├─────────────────────┤
    │    Unit Tests       │  (多数)
    └─────────────────────┘
```

### テスト分類

1. **ユニットテスト**: 各関数・メソッドの動作確認
2. **統合テスト**: OpenAI API、外部コマンドとの連携
3. **E2Eテスト**: 実際の使用シナリオでの動作確認

### モック戦略

```go
// OpenAI API のモック
type MockOpenAIClient struct {
    responses []ChatCompletionResponse
    errors    []error
    callIndex int
}

func (m *MockOpenAIClient) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
    // モック実装
}
```

## 配布戦略

### ビルド設定

```bash
# クロスプラットフォームビルド
GOOS=linux GOARCH=amd64 go build -o llmcmd-linux-amd64 ./cmd/llmcmd
GOOS=darwin GOARCH=amd64 go build -o llmcmd-darwin-amd64 ./cmd/llmcmd  
GOOS=windows GOARCH=amd64 go build -o llmcmd-windows-amd64.exe ./cmd/llmcmd

# 最適化ビルド
go build -ldflags="-s -w" -o llmcmd ./cmd/llmcmd
```

### パッケージング

1. **単一バイナリ**: 実行ファイルのみで動作
2. **設定テンプレート**: `.llmcmdrc`のサンプル提供
3. **ドキュメント**: README、man page
4. **インストールスクリプト**: 簡単インストール用

## 開発フロー

### Git ブランチ戦略

- `main`: 安定版
- `develop`: 開発版
- `feature/*`: 機能開発
- `release/*`: リリース準備

### CI/CD パイプライン

```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
        with:
          go-version: '1.21'
      - run: go test ./...
      - run: go build ./cmd/llmcmd
```

## 将来拡張ポイント

### 設計時考慮事項

1. **プラグインシステム**: ツールの動的追加
2. **複数LLMサポート**: OpenAI以外のAPI対応
3. **設定プロファイル**: 複数設定の切り替え
4. **ログ機能**: 操作履歴とデバッグ情報

### インターフェース設計

```go
// LLMクライアントインターフェース
type LLMClient interface {
    ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// ツール実行インターフェース  
type ToolExecutor interface {
    Execute(ctx context.Context, name string, args map[string]interface{}) (interface{}, error)
}
```

この設計により、シンプルで保守しやすく、将来の拡張に対応可能なllmcmdツールを実現します。
