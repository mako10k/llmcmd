## コマンド統合・重複削除チェックリスト

### 発見されたコマンド実装

#### basic/text コマンド
- [x] **echo** - internal/tools/builtin/commands/echo.go ✅ ファイル抽出完了
- [ ] **cat** - internal/tools/builtin/commands.go:Cat ⚠️ 誤報告：抽出作業は未実施でした
- [ ] **grep** - internal/tools/builtin/commands.go:Grep
- [ ] **head** - internal/tools/builtin/commands.go:Head
- [ ] **tail** - internal/tools/builtin/commands.go:Tail
- [ ] **sort** - internal/tools/builtin/commands.go:Sort
- [ ] **wc** - internal/tools/builtin/commands.go:Wc
- [ ] **tr** - internal/tools/builtin/commands.go:Tr
- [ ] **sed** - internal/tools/builtin/commands.go:Sed

#### extended コマンド
- [ ] **cut** - internal/tools/builtin/extended.go:Cut
- [ ] **uniq** - internal/tools/builtin/extended.go:Uniq
- [ ] **nl** - internal/tools/builtin/extended.go:Nl
- [ ] **tee** - internal/tools/builtin/extended.go:Tee
- [ ] **rev** - internal/tools/builtin/extended.go:Rev

#### diff/patch コマンド
- [ ] **diff** - internal/tools/builtin/diff.go:Diff
- [ ] **patch** - internal/tools/builtin/patch.go:Patch

#### 計算コマンド
- [ ] **bc** 
  - internal/llmsh/commands/calculation.go:ExecuteBc
  - internal/llmsh/commands_extra.go:executeBc (重複？)
- [ ] **dc**
  - internal/llmsh/commands/calculation.go:ExecuteDc
  - internal/llmsh/commands_extra.go:executeDc (重複？)
- [ ] **expr**
  - internal/llmsh/commands/calculation.go:ExecuteExpr
  - internal/llmsh/commands_extra.go:executeExpr (重複？)

#### 変換コマンド
- [ ] **fold**
  - internal/llmsh/commands/conversion.go:ExecuteFold
  - internal/llmsh/commands_extra.go:executeFold (重複？)
- [ ] **expand**
  - internal/llmsh/commands/conversion.go:ExecuteExpand
  - internal/llmsh/commands_extra.go:executeExpand (重複？)

#### 圧縮コマンド
- [ ] **gzip**
  - internal/llmsh/commands/encoding.go:ExecuteGzip
  - internal/llmsh/commands/external.go:ExecuteExternalGzip
  - internal/llmsh/commands_extra.go:executeGzip (重複？)
- [ ] **gunzip**
  - internal/llmsh/commands/encoding.go:ExecuteGunzip
  - internal/llmsh/commands/external.go:ExecuteExternalGunzip
  - internal/llmsh/commands_extra.go:executeGunzip (重複？)

### 作業ステータス
- [x] Phase 1: 実装コマンド完全ピックアップ ✅
- [x] Phase 2: チェックリスト作成 ✅
- [x] Phase 3: commands ディレクトリ作成 ✅ (ユーザーによる手動作成)
- [x] Phase 4: common.go 基盤ファイル作成 ✅ (ユーザーによる手動作成)
- [ ] Phase 5: 個別コマンドファイル抽出
- [ ] Phase 6: 不要コード削除

### 次の作業
1. **cat** コマンドから個別ファイル抽出開始
2. 既存実装から新しい commands/ 構造への移行
