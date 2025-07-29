# llmcmd Pipe Tool 改善設計案

## 現在の問題

1. **FD競合問題**: Background実行時の同一FDアクセス競合
2. **バッファあふれ**: Foregroundでの出力バッファ満杯によるブロック
3. **コンテキスト肥大**: LLMに全出力が送信される問題

## 提案する改善設計

### 1. Smart Output Handling (スマート出力処理)

```
pipe ツールの新しいパラメータ:
- output_mode: "full" | "summary" | "silent" | "sample"
  - full: 全出力をLLMに送信（現在の動作）
  - summary: 出力を要約してLLMに送信（先頭100行 + 末尾100行 + 統計）
  - silent: 出力をFDに書き込むのみ、LLMには送信しない
  - sample: 出力の一部サンプルのみLLMに送信（先頭50行）
```

### 2. Streaming Buffer Management

```
- 小さなチャンク単位での処理（4KB）
- 循環バッファによるメモリ効率化
- バックグラウンド実行時は非同期書き込み
```

### 3. FD Isolation (FD分離)

```
- 各pipeコマンドに専用の一時ファイルまたはパイプを作成
- FD番号の重複を避ける自動管理
- 依存関係チェーンの明示的管理
```

### 4. Progressive Processing (段階的処理)

```
Phase 1: データ変換実行（silent mode）
Phase 2: 結果確認（sample mode） 
Phase 3: 詳細分析（summary mode）
```

## 実装例

### Smart Output Mode

```go
type PipeArgs struct {
    Cmd        string   `json:"cmd"`
    Args       []string `json:"args"`
    InFd       int      `json:"in_fd"`
    OutFd      int      `json:"out_fd"`
    OutputMode string   `json:"output_mode"` // "full", "summary", "silent", "sample"
    MaxLines   int      `json:"max_lines,omitempty"` // summary/sampleモード用
}
```

### Output Summarizer

```go
func summarizeOutput(data []byte, mode string, maxLines int) ([]byte, OutputStats) {
    lines := bytes.Split(data, []byte("\n"))
    stats := OutputStats{
        TotalLines: len(lines),
        TotalBytes: len(data),
    }
    
    switch mode {
    case "silent":
        return []byte("Output written to file descriptor (silent mode)"), stats
    case "sample":
        sample := lines[:min(maxLines, len(lines))]
        return bytes.Join(sample, []byte("\n")), stats
    case "summary":
        head := lines[:min(maxLines/2, len(lines))]
        tail := lines[max(0, len(lines)-maxLines/2):]
        summary := append(head, []byte("... [truncated] ..."))
        summary = append(summary, tail...)
        return bytes.Join(summary, []byte("\n")), stats
    default: // "full"
        return data, stats
    }
}
```

## 使用例

```bash
# Silent mode: 結果をファイルに保存、LLMには統計のみ送信
llmcmd 'Convert C to JavaScript using silent pipe, then show file info'

# Sample mode: 変換結果の一部のみ確認
llmcmd 'Convert C to JavaScript and show first 50 lines as sample'

# Summary mode: 変換結果の要約を確認
llmcmd 'Convert C to JavaScript and provide summary of changes'
```

## メリット

1. **メモリ効率**: 大きなファイルでもメモリ使用量制限
2. **コンテキスト最適化**: LLMに送信するデータ量制御
3. **柔軟性**: 用途に応じた出力モード選択
4. **スケーラビリティ**: 大規模ファイル処理対応
