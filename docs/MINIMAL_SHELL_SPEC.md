# Minimal Shell Specification (llmsh)

## 概要
llmcmdの中間言語として、また独立したコマンドとしても利用可能な最小限のシェル仕様。
人間が書きやすく、LLMが理解しやすい、セキュアなサブセット実装。

## 設計原則
- **最小機能**: 必要最小限の機能のみ
- **安全性**: 外部コマンド実行禁止、ファイルアクセス制限
- **可読性**: 人間が直感的に理解・記述可能
- **拡張性**: 段階的な機能追加が可能

## 対応機能

### 基本コマンド
```bash
# 現在のbuilt-inコマンドをベース（高速・確実）
cat, grep, sed, head, tail, sort, wc, tr, cut, uniq, nl, tee, rev, diff, patch

# 基本テキスト処理（LLMの知識ベース実装）
echo, printf, true, false, test, [
yes, basename, dirname, seq
od, hexdump, base64
fmt, fold, expand, unexpand, join, comm
csplit, split

# 数値・計算処理
bc, dc, expr

# 圧縮・アーカイブ（データ処理のみ、ファイルシステム操作なし）
gzip, gunzip, bzip2, bunzip2, xz, unxz
base64, uuencode, uudecode

# 特殊コマンド
llmcmd  # 再帰的LLM実行（gpt-4o-mini、Quota継承）
llmsh   # llmshサブシェル実行（Quota継承）
help, man  # ヘルプシステム

# ❌ 除外されるコマンド（セキュリティ上の理由）
# システム情報: whoami, pwd, env, which, type, date, uname
# ファイルシステム: ls, find, locate, stat, touch, mkdir, rmdir, ln, cp, mv, rm
# 権限・所有者: chmod, chown, chgrp, umask
# プロセス: ps, top, htop, kill, killall, pgrep, pkill, nohup, timeout
# ネットワーク: curl, wget, ping, netstat, ss, lsof, iptables
# スクリプト言語: awk, perl, python, ruby, node, php, lua
# エディタ・ターミナル: vim, nano, emacs, less, more, screen, tmux
# VCS・転送: git, svn, hg, rsync, scp, ssh, telnet, ftp
# マウント・ディスク: mount, umount, lsblk, fdisk, fsck, du, df
# アーカイブ（ファイル操作含む）: tar, zip, unzip
```

### パイプライン
```bash
# 基本パイプ
cat input.txt | grep pattern | sort

# 複数段パイプ
echo "hello world" | tr ' ' '\n' | sort | uniq
```

### 基本リダイレクト
```bash
# 出力リダイレクト
cat file.txt > output.txt
echo "append" >> output.txt

# 入力リダイレクト
grep pattern < input.txt

# エラー出力
command 2> error.log
command &> all.log
```

### 最小制御構造
```bash
# 条件実行のみ（シンプル）
test -f file.txt && echo "exists"
grep pattern file.txt || echo "not found"

# command1 && command2  # 成功時のみ実行
# command1 || command2  # 失敗時のみ実行
```

### コマンド区切り
```bash
# 順次実行
command1; command2; command3

# 改行区切りも可
command1
command2
command3
```

## 除外機能

### 変数機能（完全除外）
```bash
# ❌ これらは実装しない
VAR=value
$VAR, ${VAR}
$((expression))
```

### 高度な制御構造（除外）
```bash
# ❌ これらは複雑すぎるため除外
if...then...else...fi
for...in...do...done
while...do...done
function definitions
```

### ジョブ制御（除外）
```bash
# ❌ セキュリティ・複雑性の理由で除外
command &  # バックグラウンド実行
jobs, fg, bg, kill, wait
```

### 高度なリダイレクト（除外）
```bash
# ❌ 複雑すぎるため除外
exec {fd}>file
exec {fd}<&{fd2}
2>&1 以外のfd操作
```

## 特殊機能

### llmcmd再帰実行
```bash
# LLMによる段階的処理
echo "複雑なタスク" | llmcmd "この内容を分析して要約を作成"

# パイプライン内でのLLM処理
cat document.txt | llmcmd "この文書から重要なポイントを3つ抜き出して" | tee summary.txt

# 複数段階の処理
cat data.csv | llmcmd "CSVを解析してエラー行を特定" | llmcmd "エラーの原因を分析"
```

#### 制約・仕様
- **モデル**: gpt-4o-mini固定（コスト効率重視）
- **Quota継承**: 親プロセスのQuota制限を引き継ぎ
- **入力制限**: stdin経由、またはプロンプト引数
- **出力**: stdout経由でパイプライン連携
- **エラー処理**: 標準的なexit codeとstderr

### ヘルプシステム
```bash
# 全て同じ出力を表示
man command_name
command_name --help
help command_name

# 利用可能コマンド一覧
help
man
```

#### ヘルプ内容
- **使用法**: コマンドの基本構文
- **オプション**: 利用可能なフラグとパラメータ  
- **例**: 実用的な使用例
- **関連**: 類似・関連コマンド
- **LLM知識ベース**: 一般的なUnix/Linuxコマンドの動作

## 仮想ファイルシステム
```bash
# ✅ 許可されるファイル
stdin, stdout, stderr
-i で指定された入力ファイル
-o で指定された出力ファイル

# ✅ 仮想ファイル（一時的な名前付きパイプ）
command > temp_file
other_command < temp_file
```

### 制限されたファイルアクセス
```bash
### 使用例
```bash
# データの一時保存と再利用
grep "error" log.txt > errors
grep "warning" log.txt > warnings
cat errors warnings | sort | uniq > summary
```

## コマンド実装方針

### Built-in実装
現在実装済みのコマンドは、既存のbuiltin実装を利用
```bash
cat, grep, sed, head, tail, sort, wc, tr, cut, uniq, nl, tee, rev, diff, patch
```

### LLM知識ベース実装
純粋なテキスト処理・データ変換のみ実装
```bash
echo, printf, test, [, true, false
yes, basename, dirname, seq
od, hexdump, base64, fmt, fold, expand, unexpand
join, comm, csplit, split
bc, dc, expr
gzip, gunzip, bzip2, bunzip2, xz, unxz
```

### 特殊実装
- **llmcmd**: 再帰的LLM呼び出し（gpt-4o-mini + Quota継承）
- **llmsh**: サブシェル実行（Quota継承）
- **help/man**: 統合ヘルプシステム
- **test/[**: 条件判定（文字列比較、数値比較、基本的なファイル存在チェックのみ）
```

## 実装アーキテクチャ

### Phase 1: 基本パーサー・ヘルプシステム
- トークナイザー（単語分割、クォート処理）
- 基本構文解析（パイプ、リダイレクト）
- エラー検出・報告
- 統合ヘルプシステム（help/man/--help）

### Phase 2: 実行エンジン・LLM知識ベース
- パイプライン実行
- ファイルディスクリプター管理
- 仮想ファイルシステム
- LLM知識ベースによるコマンドシミュレート

### Phase 3: 再帰実行・統合最適化
- llmcmd再帰実行（gpt-4o-mini + Quota継承）
- llmcmdとの統合
- 独立コマンド（llmsh）の実装
- パフォーマンス最適化

## 使用例

### 再帰的LLM処理
```bash
# 段階的な文書処理
cat document.txt | llmcmd "重要な部分を抜き出して" | llmcmd "箇条書きにまとめて"

# データ分析パイプライン
cat data.csv | llmcmd "異常値を特定" | tee anomalies.txt | llmcmd "対策を提案"

# llmshサブシェル実行
echo "grep TODO *.go | head -5" | llmsh | llmcmd "優先度を分析"
```

### 安全なテキスト処理
```bash
# 基本的なテキスト変換
echo "Hello World" | tr 'A-Z' 'a-z' | base64 | base64 -d

# 数値計算パイプライン  
seq 1 100 | tr '\n' '+' | sed 's/+$/\n/' | bc

# 圧縮・変換
cat data.txt | gzip | base64 > compressed_data.txt
```

### ヘルプシステムの活用
```bash
# コマンドの使い方確認
help grep
man base64
bc --help

# 利用可能コマンド一覧
help
```

### テキスト処理パイプライン
```bash
# ログ解析（従来通り）
cat access.log | grep "ERROR" | cut -d' ' -f1,4 | sort | uniq -c > error_summary.txt

# LLM拡張ログ解析
cat access.log | grep "ERROR" | llmcmd "エラーパターンを分析して分類" | sort > categorized_errors.txt

# データ変換・圧縮
cat large_data.txt | llmcmd "重要な情報を抽出" | gzip > summary.gz
```

### 条件処理
```bash
# 基本的な条件判定（文字列・数値比較のみ）
test "hello" = "hello" && echo "match" || echo "no match"
[ -z "$input" ] && echo "empty" || echo "has content"

# LLM条件処理
test -s data.txt && cat data.txt | llmcmd "データ品質チェック" || echo "No data to process"
```

## 利点

### 人間にとって
- 直感的なシェル構文
- 学習コストが低い（既存Unix知識活用）
- デバッグが容易
- 豊富なコマンドセット（LLM知識ベース）
- 統合ヘルプシステム

### LLMにとって
- 理解しやすい構文
- 既存知識を最大活用
- 予測可能な動作
- 段階的処理（llmcmd再帰実行）

### システムにとって
- セキュリティリスクが限定的
- 実装コストが適度（LLM知識ベース活用）
- 段階的拡張が可能
- コスト効率的（gpt-4o-mini使用）

## 譲歩ポイントの整理

| 機能カテゴリ | 元仕様 | 譲歩案 | 理由 |
|------------|--------|--------|------|
| 変数 | 完全対応 | ❌ 除外 | セキュリティ・複雑性 |
| 制御構造 | if/for/while | ❌ 除外 | 複雑性・無限ループリスク |
| ジョブ制御 | 完全対応 | ❌ 除外 | セキュリティリスク |
| パイプライン | 基本対応 | ✅ 維持 | 核心機能 |
| リダイレクト | 高度対応 | 🔄 簡素化 | 基本機能は重要 |
| 条件実行 | なし | ✅ 追加 | && || は実用的 |
| コマンド数 | 16個 | 🔄 大幅拡張 | LLM知識ベース活用 |
| 再帰実行 | なし | ✅ 追加 | 段階的LLM処理 |
| ヘルプ | なし | ✅ 追加 | 使いやすさ向上 |

## 実装上の考慮事項

### LLM知識ベース実装
- **利点**: 実装コスト削減、豊富な機能
- **課題**: 一貫性の保証、エラー処理
- **解決策**: 基本動作の標準化、テストケース充実

### Quota管理
```go
// 再帰実行時のQuota継承例
type RecursiveExecutor struct {
    parentQuota *QuotaManager
    model      string // "gpt-4o-mini"
}

func (r *RecursiveExecutor) Execute(prompt string) error {
    if !r.parentQuota.CanMakeCall() {
        return ErrQuotaExceeded
    }
    // gpt-4o-miniで実行
    // parentQuotaから消費
}
```

### セキュリティ考慮
- **ファイルアクセス**: 仮想ファイルシステム内のみ
- **外部実行**: 完全禁止（llmcmd再帰のみ例外）
- **リソース制限**: Quota継承による制御

この拡張仕様であれば、実用性と安全性のバランスが取れていると思いますが、いかがでしょうか？
