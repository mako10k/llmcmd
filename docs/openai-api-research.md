# OpenAI API 調査レポート

## 概要

このレポートは、llmcmd プロジェクトでのOpenAI ChatCompletion API使用に関する技術調査結果をまとめたものです。

## OpenAI ChatCompletion API 仕様（2024年版）

### 基本情報

- **エンドポイント**: `https://api.openai.com/v1/chat/completions`
- **認証方式**: Bearer Token（APIキー）
- **Content-Type**: `application/json`
- **メソッド**: POST

### 推奨モデル

| モデル | 用途 | コスト効率 | 性能 |
|--------|------|----------|------|
| gpt-4o | 複雑なタスク・高精度要求 | 中 | 高 |
| gpt-4o-mini | 軽量タスク・高速処理 | 高 | 中 |

**llmcmdでの選択**: gpt-4o-mini（コスト効率と性能のバランス）

### リクエスト形式

```json
{
  "model": "gpt-4o-mini",
  "messages": [
    {
      "role": "system",
      "content": "システムプロンプト"
    },
    {
      "role": "user", 
      "content": "ユーザー指示"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "function_name",
        "description": "関数の説明",
        "parameters": {
          "type": "object",
          "properties": {
            "param1": {
              "type": "string",
              "description": "パラメータの説明"
            }
          },
          "required": ["param1"]
        }
      }
    }
  ],
  "tool_choice": "auto",
  "temperature": 0.1,
  "max_tokens": 4096
}
```

### レスポンス形式

```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1699999999,
  "model": "gpt-4o-mini",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": null,
        "tool_calls": [
          {
            "id": "call_...",
            "type": "function",
            "function": {
              "name": "function_name",
              "arguments": "{\"param1\": \"value\"}"
            }
          }
        ]
      },
      "finish_reason": "tool_calls"
    }
  ],
  "usage": {
    "prompt_tokens": 100,
    "completion_tokens": 50,
    "total_tokens": 150
  }
}
```

## Function Calling（Tools）仕様

### ツール定義のベストプラクティス

1. **明確な関数名**: 動作が分かりやすい名前を使用
2. **詳細な説明**: 関数とパラメータの動作を明確に記述
3. **適切な型定義**: JSON Schemaに準拠した型定義
4. **必須パラメータの明示**: requiredフィールドで必須パラメータを指定

### llmcmd用ツール定義

#### read ツール

```json
{
  "type": "function",
  "function": {
    "name": "read",
    "description": "ファイルまたは入力ストリームからデータを読み取ります",
    "parameters": {
      "type": "object",
      "properties": {
        "in_id": {
          "type": "integer",
          "description": "入力ID（0: 入力ファイルまたは標準入力）",
          "default": 0
        },
        "offset": {
          "type": "integer", 
          "description": "読み取り開始位置（バイト単位）",
          "minimum": 0,
          "default": 0
        },
        "size": {
          "type": "integer",
          "description": "読み取りサイズ（バイト単位）。未指定の場合は可能な限り全て",
          "minimum": 1
        }
      },
      "required": []
    }
  }
}
```

#### write ツール

```json
{
  "type": "function", 
  "function": {
    "name": "write",
    "description": "ファイルまたは出力ストリームにデータを書き込みます",
    "parameters": {
      "type": "object",
      "properties": {
        "out_id": {
          "type": "integer",
          "description": "出力ID（1: 出力ファイルまたは標準出力）", 
          "default": 1
        },
        "data": {
          "type": "string",
          "description": "書き込むデータ"
        }
      },
      "required": ["data"]
    }
  }
}
```

#### pipe ツール

```json
{
  "type": "function",
  "function": {
    "name": "pipe", 
    "description": "外部コマンドを実行し、入力と出力をパイプで接続します",
    "parameters": {
      "type": "object",
      "properties": {
        "cmd": {
          "type": "string",
          "description": "実行するコマンド（許可リストに含まれるもののみ）"
        },
        "in_id": {
          "type": "integer",
          "description": "入力ID（未指定の場合は新しいパイプを作成）"
        },
        "out_id": {
          "type": "integer", 
          "description": "出力ID（未指定の場合は新しいパイプを作成）"
        }
      },
      "required": ["cmd"]
    }
  }
}
```

#### exit ツール

```json
{
  "type": "function",
  "function": {
    "name": "exit",
    "description": "プログラムを指定された終了コードで終了します",
    "parameters": {
      "type": "object", 
      "properties": {
        "exitcode": {
          "type": "integer",
          "description": "終了コード",
          "minimum": 0,
          "maximum": 255,
          "default": 0
        }
      },
      "required": []
    }
  }
}
```

## Go言語での実装考慮事項

### HTTP クライアント

Go標準ライブラリの`net/http`を使用：

```go
type OpenAIClient struct {
    httpClient *http.Client
    apiKey     string
    baseURL    string
}

func (c *OpenAIClient) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
    jsonData, err := json.Marshal(req)
    if err != nil {
        return nil, err
    }
    
    httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, err
    }
    
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
    
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // レスポンス処理...
}
```

### JSON 処理

構造体タグを使用した型安全なJSON処理：

```go
type ChatCompletionRequest struct {
    Model       string    `json:"model"`
    Messages    []Message `json:"messages"`
    Tools       []Tool    `json:"tools,omitempty"`
    ToolChoice  string    `json:"tool_choice,omitempty"`
    Temperature float64   `json:"temperature,omitempty"`
    MaxTokens   int       `json:"max_tokens,omitempty"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type Tool struct {
    Type     string   `json:"type"`
    Function Function `json:"function"`
}

type Function struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Parameters  interface{} `json:"parameters"`
}
```

## エラーハンドリング

### OpenAI API エラーレスポンス

```json
{
  "error": {
    "message": "Invalid API key provided",
    "type": "invalid_request_error", 
    "code": "invalid_api_key"
  }
}
```

### 主要なエラータイプ

- `invalid_api_key`: 無効なAPIキー
- `insufficient_quota`: クォータ不足
- `rate_limit_exceeded`: レート制限超過
- `model_overloaded`: モデル過負荷

### Go でのエラーハンドリング

```go
type OpenAIError struct {
    Message string `json:"message"`
    Type    string `json:"type"`
    Code    string `json:"code"`
}

func (e OpenAIError) Error() string {
    return fmt.Sprintf("OpenAI API error [%s]: %s", e.Type, e.Message)
}
```

## セキュリティ考慮事項

### APIキー管理

1. **環境変数優先**: 設定ファイルより環境変数を優先
2. **ファイル権限**: 設定ファイルは600（所有者のみ読み書き）
3. **ログ除外**: APIキーをログに出力しない

### リクエスト制限

1. **タイムアウト**: HTTP リクエストのタイムアウト設定（30秒）
2. **リトライ**: 一時的なエラーに対する指数バックオフリトライ
3. **レート制限**: API制限に応じた適切な間隔制御

## 推奨設定

### 本番環境

```go
const (
    DefaultModel       = "gpt-4o-mini"
    DefaultTemperature = 0.1
    DefaultMaxTokens   = 4096
    DefaultTimeout     = 30 * time.Second
    MaxRetries         = 3
)
```

### 開発環境

- より詳細なログ出力
- ドライランモード（API呼び出しなし）
- デバッグ情報の表示

## パフォーマンス最適化

1. **HTTP 接続の再利用**: `http.Client`の適切な設定
2. **JSON ストリーミング**: 大きなレスポンスのストリーミング処理
3. **メモリ効率**: 不要なデータのコピーを避ける
4. **並行処理**: 適切なコンテキスト管理
