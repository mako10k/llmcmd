# llmcmd 課題・TODO リスト

## 🔍 発見済み課題

### 1. LLMによるdiff/patch出力の過度な解釈問題
**現象**: 
- built-in関数 `Diff()` は正常に動作 ✅
- LLM経由でdiffコマンドを実行すると "No differences found" と返される ❌
- LLMがdiffコマンドの出力を解釈して独自判断している

**原因**: 
- システムプロンプトでdiff/patchコマンドの出力をそのまま返すよう明示的に指示されていない
- LLMが「意味のある差分かどうか」を判断している

**解決策**:
- システムプロンプトにdiff/patchコマンドの出力は解釈せずそのまま返すよう明記
- 特に「Technical commands (diff, patch, etc.) should output raw results without interpretation」

**優先度**: 高 (Phase 3の完全性に影響)

### 2. テスト実行時間の問題
**現象**:
- LLM経由のコマンドが timeout_seconds = 300 (5分) の設定で長時間実行される
- テスト効率が悪い

**解決策**:
- テスト用設定ファイルの作成
- timeout_seconds を短縮 (30秒程度)

**優先度**: 中

## ✅ 完了済み機能

### Phase 3: Tool Implementation (完了)
- [x] spawn tool: 動的コマンドマッピング実装
- [x] diff tool: Unified diff形式生成
- [x] patch tool: diff適用機能
- [x] built-in command動作確認
- [x] エラーハンドリング
- [x] --dry-run オプション標準化

### Code Quality Improvements (完了) 
- [x] コード重複大幅削除 (911行純減)
- [x] 共通関数の作成 (parseAndAssign*, validate*, createRunningCommand等)
- [x] 不要ファイルのクリーンアップ

## 📋 今後の作業計画

### Phase 4: Built-in Commands拡張
- [ ] head/tail/sort/wc/tr の最適化
- [ ] パフォーマンステスト
- [ ] メモリ使用量最適化

### Phase 5: Integration & Testing
- [ ] システムプロンプト最適化 ⭐ (最優先)
- [ ] 包括的テストスイート
- [ ] エラーケース網羅
- [ ] ドキュメント更新

### Phase 6: Release準備
- [ ] クロスプラットフォームビルド
- [ ] パッケージング
- [ ] リリースノート

## 🔧 技術メモ

### 動作確認済み
- spawn tool: Pattern 1-4 全て動作
- diff/patch: built-in関数レベルで完全動作
- API統合: OpenAI ChatCompletion正常
- 設定システム: .llmcmdrc読み込み正常

### 次回優先対応
1. システムプロンプト調整 (diff/patch出力の生出力指示)
2. テスト自動化スクリプト整備
3. Phase 4準備

---
*最終更新: 2025年7月29日*
*ステータス: Phase 3完了、システムプロンプト調整待ち*
