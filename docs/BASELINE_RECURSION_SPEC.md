# Baseline Recursion & Pipeline Spec (L1-Rec)

目的: チャットで揺れた前提を固定し、以後はこのファイルとの差分 (Change Log) のみ議論する。

## 1. スコープ (L1-Rec 完了条件)
- Pipeline: `cmd1 | cmd2 | ...` の線形実行。空白区切りのみ。Quoting / heredoc なし。
- Builtins (whitelist 初期セット): `cat, grep, sed, head, tail, sort, wc, tr`
- Spawn 再帰: llmcmd(parent) → spawn → llmsh(batch) → (内部で) llmcmd(child) … 深さ制限あり。
- Stdin: 子(=spawn 対象) に 1MB 以内テキストを書き込める。NUL バイト拒否。Close で EOF。
- Exit code: 子パイプライン内コマンドのいずれか失敗で 非0。成功時 0。詳細分類は後回し。
- Depth 上限: デフォルト 5 (親=depth1)。超過時: depth_exceeded エラー (非0)。
- Quota 継承: 親の使用量 + 子の追加分を合算し、超過なら即エラー。子開始前に見積/起動後超過検出時 Fail-First。
- VFS: 単一 allowlist モード。executionMode 分岐/継承なし。子で許可ファイルは拡大不可。
- サイズ制限: 
  - Script テキスト (spawn/pipeline 入力) ≤ 8KB
  - Pipeline 出力合計 ≤ 1MB (超過で truncate ではなく error)
  - stdin 書込総量 ≤ 1MB
- エラー分類 (最小): `invalid_command`, `depth_exceeded`, `quota_exceeded`, `stdin_limit`, `stdin_binary`, `output_limit`, `pipeline_parse_error`。
- Tests (必須): 単純 1 段 / 3 段 / 深さ超過 / quota 超過 / stdin サイズ超過 / NUL / invalid command。
- Docs: 本ファイル + 簡易使用例 + 後回しリスト。同梱。
- TODO 整理: Later: prefix のもの以外は L1 完了時ゼロ。

## 2. 再帰モデル
```
llmcmd(depth n)
  spawn(script)
    -> llmsh(batch interpreter) depth n+1
         executes pipeline of builtins
         may include another spawn ... (until depth limit)
```
- 深さカウンタは spawn 呼び出し境界で +1。
- 子失敗時: 親は exit code と stderr (最初のエラー) を受取る。
- 並列 spawn / background は Later。

## 3. パイプライン仕様 (最小)
- 分割: `strings.Split(script, "|")` → trim → 空だったら parse error。
- 各セグメント: `strings.Fields()` でコマンド + 引数
- コマンド名が whitelist に無ければ `invalid_command`
- 最大段数 (stages): 10
- 各引数長合計 (1 stage) ≤ 2KB
- 全 script バイト長 ≤ 8KB (UTF-8 想定)
- 実行: in-memory chaining (段ごとに bytes.Buffer)。ストリーミング最適化は Later。

## 4. Builtin 呼び出し規約
- 署名想定: `func(cmd string, args []string, stdin io.Reader, stdout io.Writer) error`
- 既存個別実装を薄いアダプタで wrap。副作用 (ファイル read/write) は VFS 経由。
- sed: 単一 `s/pat/repl/g` 想定 (拡張正規表現/複数置換 Later)。

## 5. Quota / 安全
| 項目 | 制限 | 超過時 | Later 拡張 |
|------|------|--------|------------|
| depth | 5 | depth_exceeded | 動的設定, per-user |
| script size | 8KB | pipeline_parse_error | 拡張 32KB |
| stdin total | 1MB | stdin_limit | バイナリ許容切替 |
| output size | 1MB | output_limit | スニペット返却 |
| stages | 10 | pipeline_parse_error | 並列分岐 |

## 6. 後回し (Later:) リスト
- Later: quoting ("..." / escape)
- Later: heredoc (<<EOF)
- Later: tee / redirect (> file)
- Later: background / cancel / worker 分離
- Later: advanced exit code taxonomy
- Later: streaming line-by-line executor
- Later: interactive REPL (行編集・履歴)
- Later: metrics 詳細 (lines_processed, time_per_stage)
- Later: fuzz / property tests

## 7. 非スコープ (Out of Scope for L1)
- OS 任意コマンド実行
- 動的 builtin 追加
- 環境変数展開 / ワイルドカード
- 変数代入 / サブシェル

## 8. 変更管理
- 本ファイルを唯一の “真実の基準” とし、改訂は PR で diff 明示。
- 追加項目は必ず `Later:` → `Promote:` のステータス遷移記録。

## 9. 用語
| 用語 | 定義 |
|------|------|
| spawn | 再帰層を 1 つ深く作る操作 (llmsh バッチ呼び) |
| pipeline | `|` 区切りの builtin 連鎖 |
| depth | 親からの spawn ネスト数 (親=1) |
| quota | 設定された統制リソース枠 (tokens/bytes/stages/depth) |

## 10. 完了判定チェックリスト (L1-Rec)
- [ ] stdin (1MB, NUL 拒否) 実装 / テスト
- [ ] exit code 0/非0 + エラー分類最小
- [ ] pipeline executor + stages 上限 + invalid tests
- [ ] builtin adapter 8種
- [ ] depth/ quota 継承チェック
- [ ] VFS 単一化 (executionMode 削除) & テスト緑
- [ ] output size ガード
- [ ] docs 本ファイル + usage snippet
- [ ] TODO 整理 (Later: のみ残存)

---
更新履歴:
- v0.1 (draft): 初版生成 (date: YYYY-MM-DD)
