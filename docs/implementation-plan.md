# llmcmd 実装計画書

## 実装フェーズ

### Phase 1: プロジェクト基盤構築

**目標**: 基本的なプロジェクト構造とビルド環境の整備

**実装項目**:
- [x] ドキュメント作成
- [ ] Go moduleの初期化
- [ ] ディレクトリ構造の作成
- [ ] 基本的なCLI引数解析
- [ ] 設定ファイル読み込み機能
- [ ] ログ機能の基本実装

**成果物**:
- 実行可能な最小限のCLIツール
- 設定ファイル読み込み機能
- ヘルプとバージョン表示

**期間**: 2-3日

### Phase 2: OpenAI API クライアント実装

**目標**: OpenAI ChatCompletion APIとの通信機能

**実装項目**:
- [ ] HTTP クライアントの実装
- [ ] OpenAI API 型定義
- [ ] 認証とエラーハンドリング
- [ ] レスポンス解析
- [ ] リトライ機能

**成果物**:
- OpenAI APIクライアント
- エラーハンドリング機能
- 基本的なAPI通信テスト

**期間**: 3-4日

### Phase 3: ツール機能の実装

**目標**: LLMが使用する4つのツール（read、write、pipe、exit）の実装

#### 3.1 read ツール
- [ ] ファイル読み込み機能
- [ ] 標準入力読み込み
- [ ] オフセット・サイズ指定読み込み
- [ ] エラーハンドリング

#### 3.2 write ツール  
- [ ] ファイル書き込み機能
- [ ] 標準出力書き込み
- [ ] 追記モード実装
- [ ] ファイルサイズ制限チェック

#### 3.3 pipe ツール
- [ ] 外部コマンド実行
- [ ] パイプ作成と管理
- [ ] コマンド許可リストチェック
- [ ] プロセス管理

#### 3.4 exit ツール
- [ ] プログラム終了処理
- [ ] 終了コード管理
- [ ] リソースクリーンアップ

**成果物**:
- 4つのツール実装
- ツール実行エンジン
- セキュリティチェック機能

**期間**: 5-6日

### Phase 4: 統合とエラーハンドリング

**目標**: 全機能の統合とロバストなエラーハンドリング

**実装項目**:
- [ ] メインアプリケーションロジック
- [ ] ツール呼び出しのオーケストレーション
- [ ] 包括的なエラーハンドリング
- [ ] セキュリティ機能の統合
- [ ] ログ機能の拡充

**成果物**:
- 完全に動作するllmcmdツール
- エラー処理とログ機能
- セキュリティ機能

**期間**: 3-4日

### Phase 5: テストとドキュメント

**目標**: 品質保証とユーザビリティ向上

**実装項目**:
- [ ] ユニットテスト作成
- [ ] 統合テスト作成  
- [ ] E2Eテスト作成
- [ ] ユーザーマニュアル作成
- [ ] 使用例の作成

**成果物**:
- テストスイート
- ユーザーマニュアル
- サンプル設定ファイル

**期間**: 4-5日

## 詳細実装計画

### Phase 1 詳細

#### 1.1 プロジェクト初期化

```bash
# Go module初期化
go mod init github.com/mako10k/llmcmd

# ディレクトリ構造作成
mkdir -p cmd/llmcmd internal/{cli,app,openai,tools,security} docs examples
```

#### 1.2 基本CLI実装

**ファイル**: `cmd/llmcmd/main.go`
```go
func main() {
    config, err := cli.ParseArgs(os.Args[1:])
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    if config.ShowHelp {
        cli.ShowHelp()
        return
    }
    
    if config.ShowVersion {
        cli.ShowVersion()
        return  
    }
    
    // アプリケーション実行
    app := app.New(config)
    if err := app.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

#### 1.3 設定管理実装

**ファイル**: `internal/cli/config.go`
```go
type Config struct {
    APIKey           string
    MaxInputBytes    int64
    MaxOutputBytes   int64
    AllowedCommands  []string
    InputFile        string
    OutputFile       string
    Prompt           string
    Verbose          bool
}

func LoadConfig() (*Config, error) {
    // 環境変数と設定ファイルから設定を読み込み
}
```

### Phase 2 詳細

#### 2.1 OpenAI API クライアント

**ファイル**: `internal/openai/client.go`
```go
type Client struct {
    httpClient *http.Client
    apiKey     string
    baseURL    string
}

func NewClient(apiKey string) *Client {
    return &Client{
        httpClient: &http.Client{Timeout: 30 * time.Second},
        apiKey:     apiKey,
        baseURL:    "https://api.openai.com/v1",
    }
}

func (c *Client) ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
    // API呼び出し実装
}
```

#### 2.2 型定義

**ファイル**: `internal/openai/types.go`
```go
type ChatCompletionRequest struct {
    Model       string    `json:"model"`
    Messages    []Message `json:"messages"`
    Tools       []Tool    `json:"tools,omitempty"`
    ToolChoice  string    `json:"tool_choice,omitempty"`
    Temperature float64   `json:"temperature,omitempty"`
    MaxTokens   int       `json:"max_tokens,omitempty"`
}

type ChatCompletionResponse struct {
    ID      string   `json:"id"`
    Object  string   `json:"object"`
    Created int64    `json:"created"`
    Model   string   `json:"model"`
    Choices []Choice `json:"choices"`
    Usage   Usage    `json:"usage"`
}
```

### Phase 3 詳細

#### 3.1 ツール実行エンジン

**ファイル**: `internal/tools/executor.go`
```go
type Executor struct {
    config     *cli.Config
    fileStates map[int]*FileState
    pipeStates map[int]*PipeState
}

func NewExecutor(config *cli.Config) *Executor {
    return &Executor{
        config:     config,
        fileStates: make(map[int]*FileState),
        pipeStates: make(map[int]*PipeState),
    }
}

func (e *Executor) ExecuteTool(ctx context.Context, call ToolCall) (interface{}, error) {
    switch call.Function.Name {
    case "read":
        return e.executeRead(ctx, call.Function.Arguments)
    case "write":
        return e.executeWrite(ctx, call.Function.Arguments)
    case "pipe":
        return e.executePipe(ctx, call.Function.Arguments)
    case "exit":
        return e.executeExit(ctx, call.Function.Arguments)
    default:
        return nil, fmt.Errorf("unknown tool: %s", call.Function.Name)
    }
}
```

#### 3.2 各ツール実装

**ファイル**: `internal/tools/read.go`
```go
type ReadArgs struct {
    InID   int `json:"in_id"`
    Offset int `json:"offset"`
    Size   int `json:"size,omitempty"`
}

type ReadResult struct {
    Input      string `json:"input"`
    NextOffset int    `json:"next_offset"`
    EOF        bool   `json:"eof"`
    Size       int    `json:"size"`
    Error      string `json:"error,omitempty"`
}

func (e *Executor) executeRead(ctx context.Context, args string) (*ReadResult, error) {
    var readArgs ReadArgs
    if err := json.Unmarshal([]byte(args), &readArgs); err != nil {
        return nil, err
    }
    
    // 読み込み処理実装
}
```

### Phase 4 詳細

#### 4.1 メインアプリケーション

**ファイル**: `internal/app/app.go`
```go
type App struct {
    config   *cli.Config
    client   *openai.Client
    executor *tools.Executor
}

func New(config *cli.Config) *App {
    return &App{
        config:   config,
        client:   openai.NewClient(config.APIKey),
        executor: tools.NewExecutor(config),
    }
}

func (a *App) Run() error {
    ctx := context.Background()
    
    // システムプロンプトの構築
    systemPrompt := a.buildSystemPrompt()
    
    // 初回LLM呼び出し
    req := &openai.ChatCompletionRequest{
        Model: "gpt-4o-mini",
        Messages: []openai.Message{
            {Role: "system", Content: systemPrompt},
            {Role: "user", Content: a.config.Prompt},
        },
        Tools: a.buildTools(),
        ToolChoice: "auto",
        Temperature: 0.1,
        MaxTokens: 4096,
    }
    
    // 会話ループ
    for {
        resp, err := a.client.ChatCompletion(ctx, req)
        if err != nil {
            return err
        }
        
        // ツール呼び出し処理
        if len(resp.Choices[0].Message.ToolCalls) > 0 {
            if err := a.handleToolCalls(ctx, resp.Choices[0].Message.ToolCalls, req); err != nil {
                return err
            }
        } else {
            // 最終回答の処理
            break
        }
    }
    
    return nil
}
```

## テスト計画

### テスト環境

```go
// テスト用モックの実装
type MockOpenAIClient struct {
    responses []*openai.ChatCompletionResponse
    errors    []error
    callCount int
}

func (m *MockOpenAIClient) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
    if m.callCount >= len(m.responses) {
        return nil, fmt.Errorf("no more mock responses")
    }
    
    resp := m.responses[m.callCount]
    err := m.errors[m.callCount]
    m.callCount++
    
    return resp, err
}
```

### テストケース

#### ユニットテスト
- [ ] CLI 引数解析テスト
- [ ] 設定ファイル読み込みテスト  
- [ ] 各ツールの個別テスト
- [ ] エラーハンドリングテスト

#### 統合テスト
- [ ] OpenAI API通信テスト（モック使用）
- [ ] ファイルI/Oテスト
- [ ] 外部コマンド実行テスト

#### E2Eテスト
- [ ] 実際のAPIを使ったテスト（CI/CD除外）
- [ ] 複数ツールを使った複雑なシナリオ
- [ ] エラー状況での動作テスト

## リリース計画

### v1.0.0 リリース目標

**必須機能**:
- [x] 基本CLI機能
- [ ] OpenAI API 通信
- [ ] 4つのツール（read、write、pipe、exit）
- [ ] 設定ファイル管理
- [ ] セキュリティ機能
- [ ] エラーハンドリング

**品質要件**:
- [ ] ユニットテストカバレッジ 80% 以上
- [ ] 主要なE2Eシナリオテスト
- [ ] メモリリーク なし
- [ ] ドキュメント完備

**配布物**:
- [ ] Linux/macOS/Windows バイナリ
- [ ] ユーザーマニュアル
- [ ] 設定ファイルテンプレート
- [ ] 使用例集

### マイルストーン

| マイルストーン | 目標日 | 主要成果物 |
|---------------|--------|-----------|
| M1: 基盤完成 | Day 3 | CLI + 設定管理 |
| M2: API統合 | Day 7 | OpenAI APIクライアント |
| M3: ツール完成 | Day 13 | 全ツール実装 |
| M4: 統合完成 | Day 17 | 動作する完全版 |
| M5: 品質保証 | Day 22 | テスト + ドキュメント |

## リスク管理

### 技術リスク

1. **OpenAI API制限**: レート制限やクォータ超過
   - **対策**: リトライ機能、エラーハンドリング強化

2. **ツール実行セキュリティ**: 悪意のあるコマンド実行
   - **対策**: ホワイトリスト、サンドボックス化

3. **メモリ消費**: 大ファイル処理時のメモリ不足
   - **対策**: ストリーミング処理、サイズ制限

### プロジェクトリスク

1. **スケジュール遅延**: 想定より複雑な実装
   - **対策**: 段階的実装、最小機能優先

2. **品質問題**: 十分なテストができない
   - **対策**: 自動テスト、継続的統合

## 次の開発段階

Phase 1の実装から始めて、各フェーズを順次完了していきます。現在の状況では、ドキュメント整備が完了しているため、すぐにGo言語での実装に取りかかることができます。
