# llmcmd 未実装機能実装計画

## 現在の状況（2025年7月28日）

### ✅ **実装完了項目**
- **Phase 1**: プロジェクト基盤構築 ✅
- **Phase 2**: OpenAI API統合 ✅ 
- **Phase 3**: ツール機能（read、write、pipe、exit）✅
- **Phase 4**: Built-inコマンド（cat、grep、sed、head、tail、sort、wc、tr）✅
- **Built-inコマンド拡張**: cut、uniq、nl、tee、rev コマンド ✅
- **環境変数サポート**: LLMCMD_* 系統の設定上書き機能 ✅
- **統計機能**: 詳細統計表示（--stats フラグ）✅
- **GitHub公開**: オープンソースプロジェクトとして公開 ✅

### ❌ **未実装項目（優先度順）**

## 実装優先度 1: Built-inコマンド完成（高実用性）

### 1.1 cut コマンド（最高優先度）
**実用性**: ★★★★★ - CSVやTSVファイル処理に必須

**仕様**:
```bash
cut -d',' -f1,3    # カンマ区切りで1列目と3列目を抽出
cut -c1-10         # 1-10文字目を抽出
cut -f2            # デフォルト（タブ）区切りで2列目を抽出
```

**実装**:
- 列指定（-f）: フィールド番号範囲指定
- 文字指定（-c）: 文字位置範囲指定  
- 区切り文字指定（-d）: カスタム区切り文字
- 複数フィールド選択（1,3,5-7形式）

**期間**: 0.5日

### 1.2 uniq コマンド（高優先度）
**実用性**: ★★★★☆ - データ重複除去に頻繁に使用

**仕様**:
```bash
uniq              # 連続する重複行を削除
uniq -c           # 重複回数をカウント
uniq -d           # 重複行のみ表示
uniq -u           # 非重複行のみ表示
```

**実装**:
- 基本重複除去機能
- カウント表示（-c）
- 重複行表示（-d）
- 一意行表示（-u）

**期間**: 0.5日

### 1.3 nl コマンド（中優先度）
**実用性**: ★★★☆☆ - 行番号付けに便利

**仕様**:
```bash
nl                # 行番号を付ける
nl -b a           # 全行に番号付け
nl -s ": "        # 区切り文字指定
```

**実装**:
- 基本行番号付け
- 番号付け対象指定（-b）
- 区切り文字指定（-s）

**期間**: 0.3日

### 1.4 tee コマンド（中優先度）
**実用性**: ★★★☆☆ - パイプライン分岐に有用

**仕様**:
```bash
tee output.txt    # 標準出力とファイルに同時出力
tee -a file.txt   # ファイルに追記モード
```

**実装**:
- 標準出力とファイル同時書き込み
- 追記モード（-a）
- 複数ファイル対応

**期間**: 0.3日

### 1.5 rev コマンド（低優先度）
**実用性**: ★★☆☆☆ - 特殊用途向け

**仕様**:
```bash
rev               # 各行の文字を逆順にする
```

**実装**:
- 行単位文字反転
- Unicode対応

**期間**: 0.2日

**Built-inコマンド実装総期間**: 1.8日

## 実装優先度 2: 設定・環境変数サポート（中実用性）

### 2.1 環境変数による設定読み込み ✅ **完了**
**実用性**: ★★★★☆ - CI/CD環境での利用に重要

**実装済み項目**:
- ✅ `OPENAI_API_KEY`: OpenAI APIキー
- ✅ `OPENAI_BASE_URL`: OpenAI API ベースURL
- ✅ `LLMCMD_MODEL`: 使用モデル設定
- ✅ `LLMCMD_MAX_TOKENS`: 最大トークン数
- ✅ `LLMCMD_TEMPERATURE`: 温度設定
- ✅ `LLMCMD_MAX_API_CALLS`: 最大API呼び出し回数
- ✅ `LLMCMD_TIMEOUT_SECONDS`: タイムアウト設定

**設定優先度**: デフォルト < 設定ファイル < 環境変数

**期間**: 0.5日 → **実装済み**

### 2.2 System Installation Support ✅ **COMPLETED**
**Practical Value**: ★★★★☆ - Essential for production deployment

**Implemented Items**:
- ✅ `llmcmd --install` command for system-wide installation
- ✅ Binary distribution setup (GitHub Releases)  
- ✅ Installation script creation (`install.sh`)
- ✅ System PATH integration
- ✅ Makefile for build automation
- ✅ Cross-platform binary builds

**Duration**: 0.5 day → **COMPLETED**

## 実装優先度 3: エラーハンドリング強化（中実用性）

### 3.1 リトライメカニズム
**実用性**: ★★★★☆ - API制限対応に必須

**実装項目**:
- 指数バックオフリトライ
- レート制限エラー処理
- 最大リトライ回数設定
- リトライ状況のログ出力

**期間**: 0.7日

### 3.2 詳細エラー情報
**実用性**: ★★★☆☆ - トラブルシューティング向上

**実装項目**:
- エラーコード分類
- 具体的なエラーメッセージ
- 解決策の提示
- デバッグ情報の詳細化

**期間**: 0.5日

## 実装優先度 4: テスト充実（長期品質）

### 4.1 テストカバレッジ向上
**実用性**: ★★★☆☆ - 長期保守性向上

**現状**: 32.3% → **目標**: 80%+

**実装項目**:
- アプリケーションレイヤーテスト
- ツールエンジンテスト
- エラーケーステスト
- エンドツーエンドテスト

**期間**: 2日

### 4.2 統合テスト
**実用性**: ★★☆☆☆ - CI/CD品質保証

**実装項目**:
- OpenAI APIモックテスト
- ファイルI/Oテスト
- コマンドパイプラインテスト

**期間**: 1日

## 実装優先度 5: ドキュメント・配布（ユーザビリティ）

### 5.1 ユーザーガイド作成
**実用性**: ★★★☆☆ - 新規ユーザー獲得

**実装項目**:
- チュートリアル作成
- 実用例集
- トラブルシューティングガイド
- FAQ作成

**期間**: 1日

### 5.2 CI/CD & リリース自動化
**実用性**: ★★☆☆☆ - 継続開発効率化

**実装項目**:
- GitHub Actions設定
- 自動テスト実行
- クロスプラットフォームビルド
- リリース自動化

**期間**: 1日

## 📋 **推奨実装順序**

### Week 1: 高実用性機能完成 ✅ **完了**
1. ✅ **Day 1**: `cut` コマンド実装（0.5日）+ `uniq` コマンド実装（0.5日）
2. ✅ **Day 2**: `nl`, `tee`, `rev` コマンド実装（0.8日）+ 環境変数サポート（0.5日）
3. **Day 3**: リトライメカニズム実装（0.7日）+ 設定テンプレート（0.3日）

### Week 2: 品質・保守性向上
4. **Day 4-5**: テストカバレッジ向上（2日）
5. **Day 6**: 統合テスト実装（1日）
6. **Day 7**: ドキュメント作成（1日）

### Week 3: 継続開発基盤
7. **Day 8**: CI/CD設定（1日）

## 🐛 **Issues Found in Use Case Testing**

### July 28th, 2025 - Production Testing Results

#### ✅ **Verified Working Features**
- Basic file processing (read, write operations)
- Simple CSV filtering (e.g., Tokyo residents extraction)
- Built-in commands individual operations (cut, head, tail, etc.)
- Environment variable configuration override (LLMCMD_MODEL, LLMCMD_MAX_TOKENS, etc.)
- Statistics display functionality (--stats flag)

#### ❌ **Discovered Issues**

##### 1. **OpenAI API Error (High Priority)** ✅ **FIXED**
**Problem**: Complex tasks fail with "Invalid value for 'content': expected a string, got null" error
```
Error example: Occurred during complex CSV analysis requests
- Basic operations: ✅ Working
- Complex analysis: ❌ API Error → Now works!
```
**Root Cause**: Tool response message creation resulted in null content
**Impact**: Cannot execute practical complex tasks
**Solution**: ✅ Modified CreateToolResponseMessage to handle empty results with "(no output)" fallback

##### 2. **Standard Input/Output Processing (Medium Priority)** ✅ **FIXED**
**Problem**: Standard input/output specification not working
```bash
echo "data" | ./llmcmd -i - "process this"
# Error: input file does not exist: -  → Now works!

./llmcmd -i input.txt -o - "process and output to stdout"
# Standard output specification was unsupported → Now works!
```
**Root Cause**: File existence check didn't consider stdin/stdout
**Impact**: Pipeline processing was impossible
**Solution**: 
- ✅ `-i -` : Standard input specification
- ✅ `-o -` : Standard output specification
- ✅ No arguments defaults to `-i -` interpretation

##### 3. **LLM解釈の確率的な性質（制限事項として受容）**
**問題**: 「最初の3行」要求で4行表示される等の不一貫性
```
要求: "Show me the first 3 lines"
実際: 4行出力（ヘッダー含む）
期待: 3行のみ出力
```
**原因**: LLMの確率的性質（温度0.1でも発生）
**影響**: 軽微（機能的には問題なし）
**制限事項**: LLMを使用する以上の宿命として受容
**軽減策**: プロンプト例示の充実化（完全解決は困難）

##### 4. **プロンプト解釈の一貫性（中優先度）**
**問題**: 同じ要求でも結果が不安定
- 「最初の3行」→ 時々4行出力
- 複雑な要求 → APIエラー率が高い

**原因**: 
- LLMの解釈のゆれ
- エラー処理の不備
- プロンプトの曖昧さ

**対策**: 
- システムプロンプトの改善
- エラーハンドリング強化
- 例示の追加

#### 📋 **優先修正項目**

##### Immediate Priority (Week 1) ✅ **COMPLETED**
1. **OpenAI API Error Fix** (0.5 day) ✅ **FIXED**
   - ✅ Message creation logic investigated
   - ✅ Content null problem resolved
   
2. **Standard Input/Output Processing Implementation** (0.3 day) ✅ **FIXED**
   - ✅ `-i -` : Standard input specification
   - ✅ `-o -` : Standard output specification  
   - ✅ No arguments interpreted as `-i -`
   - ✅ Pipeline operation confirmed

##### Continuous Improvement (Week 2)
3. **Prompt Example Enhancement** (0.3 day)
   - System prompt example addition
   - Complete solution is difficult (accepted as LLM limitation)

4. **Error Handling Enhancement** (0.5 day)
   - Detailed error messages
   - Retry mechanism
   - Log improvements

**次の実装ターゲット**: 
1. 🔥 **OpenAI APIエラー修正**（最優先）
2. **標準入力・標準出力処理**（-i -, -o -, 引数なし対応）  
3. **プロンプト例示充実**（LLM制限事項の軽減）

この問題点の修正により、実用レベルでの安定動作を実現します。
