# llmcmd / llmsh 設計思想ドキュメント

最終更新: 2025-08-14  
対象: プロジェクト新規/継続開発者, アーキテクト, レビュー担当  

---
## 1. 目的と位置付け
`llmcmd` と `llmsh` は「LLM が安全な“閉じた計算環境”内で、ファイル加工・テキスト整形・自己再帰的分解を行いながら高品質出力を生成する」ための二形態インターフェースである。

| コンポーネント | 主用途 | モード | 代表特徴 |
|---------------|--------|--------|-----------|
| llmcmd | 1ショット / バッチ指向の LLM 指示実行 | コマンドラインツール | OpenAI API 直結 / プロンプト + 入出力束ね / プリセット / 再帰実行サポート |
| llmsh  | ミニマル“LLM向け”シェル / スクリプト合成 | 対話 / パイプライン | 安全な内部ビルトインのみ / 外部コマンド不許可 / VFS + Quota 継承 / LLM 主体の分割統治 |

両者は **共通 VFS / Quota / ツール実行エンジン** を共有し、利用者（あるいは上位オーケストレータ LLM）が状況に応じて「一発指示 (llmcmd)」と「段階的構築 (llmsh)」を切り替えられる統一モデルを志向する。

---
## 2. 共通コア哲学
1. **Security First**: 外部任意コマンドや環境汚染要素を排除し、ビルトイン関数 + 制限付き VFS のみで操作。  
2. **Fail-First**: 条件違反・設定異常・安全境界逸脱を即時検知・終了。エラーは “静かに無視” しない。  
3. **Simplicity → Composability**: 機能粒度を小さく (cat, grep, sed, wc 等) 保ち LLM によるパイプライン合成余地を最大化。  
4. **Deterministic & Auditable**: I/O, quota, API 呼び出し数を明示管理し再現性確保。  
5. **Extensibility**: ツール追加や VFS / fsproxy 統合を段階展開できる層状アーキテクチャ。  
6. **Cost Awareness**: Weighted token (Input:1 / Cached:0.25 / Output:4) に基づくリアルタイム残量監視。  
7. **Unified Abstractions**: llmcmd ↔ llmsh が同一プリミティブ上で動作し、差異は UX と使用文脈のみ。  

---
## 3. アーキテクチャ層 (概略)
```
User / Orchestrator LLM
        │
  (CLI / Shell Front)  ← llmcmd / llmsh
        │
   Application Layer (app, cli)
        │
   Tool Execution Engine (tools + builtin commands)
        │
   OpenAI Client (retry / quota / auth)
        │
   VFS / FS Proxy (仮想化 + サイズ/権限ガード)
        │
   Host Filesystem (制限付き)
```
- **共通ツール群**: read / write / spawn(制限) / exit + テキスト系ビルトイン。  
- **VFS**: 抽象化されたファイルアクセス制御面。fsproxy プロトコル統合計画により llmsh / llmcmd が統一 API 経由で安全操作。  
- **Quota Manager**: API 呼び出し回数・トークン消費を集約。サブシェル / 再帰で継承。  
- **Preset Prompt System**: 安全な標準化指示テンプレートで再現性・品質平準化。  

---
## 4. llmcmd の設計思想
| 観点 | 方針 | 背景 / 意図 |
|------|------|-------------|
| コア機能 | 単発・明示的指示を OpenAI API に送り、ファイル/標準入出力を束ね | スクリプト / CI / バッチ適性 |
| 再帰呼び出し | llmcmd 自身をプロンプト生成チェーン内で再入可能 | 複雑タスクの段階分解 (Two-Stage / Multi-Step) |
| Two-Stage モード | 計画 (大モデル) → 実行 (軽量モデル) | コスト最適化 + 高品質計画 |
| 構成管理 | CLI Flags + 環境変数 + rc (優先順位付き) | DevOps / CI パイプライン統合容易化 |
| Quota Enforcement | 超過即失敗 (Fail-First) | コスト暴走防止 / 透明性 |
| VFS 利用 | 入力集約 / 出力整形 / 生成成果物再利用 | 一貫した I/O モデル |
| エラー指針 | 実行前検証 (config, size, timeout) 即終了 | 後続副作用抑止 |
| プリセット | 標準化プロンプト (diff/patch 等) | 再現性と安全なツール誘導 |

### llmcmd が重視する非機能要求
- 冪等性 (入力と設定が同一なら同じ振る舞い)  
- 監査容易性 (ログ + 統計)  
- スクリプタビリティ (単一プロセス/戻りコード明確)  

---
## 5. llmsh の設計思想
| 観点 | 方針 | 補足 |
|------|------|------|
| ミニマルシェル | POSIX 完全互換を追わず “LLM に最小限必要” な構文 | 過剰機能は攻撃面拡大リスク |
| ビルトイン限定 | 外部プロセス禁止 (再帰 llmcmd など許可例外最小化) | サンドボックス堅牢化 |
| パイプライン指向 | cat | grep | sed | llmcmd の連結容易 | LLM による逐次精緻化を想定 |
| Quota 継承 | 親 llmcmd / 上位 llmsh から残量伝搬 | コスト境界の明示保持 |
| Virtual FS | fsproxy 統合後は仮想 FD テーブルで追跡 | ファイル逸脱抑止 / 容量課金制御 |
| エラー即露出 | 曖昧な黙殺 (書き込み失敗など) を避ける共通ハンドラ | 以前の“静かな失敗”撲滅施策 |
| スクリプト化 | LLM がシェルスクリプトを内部生成し実行 | “計画→操作” 反復効率向上 |

### llmsh の価値提供
- LLM 自身が「観察 → 変換 → 再評価」サイクルを内部で短いビルトイン連鎖に還元。  
- 外部 OS コマンドに依存しないためプラットフォーム差異縮小 / セキュリティ明瞭化。  

---
## 6. 両者の補完関係
| シナリオ | 推奨 | 理由 |
|----------|------|------|
| 単純処理 + 明確指示 | llmcmd | 起動オーバーヘッド最小 / 単発終了 |
| 複合ログ解析 / 段階抽出 | llmsh + 部分で llmcmd | インクリメンタル検査 + 要約 |
| 大型テキスト分割後集約 | llmsh (分割) → llmcmd (要約統合) | トークン利用最適化 |
| プロンプト品質テンプレ適用 | llmcmd (preset) → llmsh 生成結果検証 | 標準化 + 柔軟編集 |

---
## 7. セキュリティ & サンドボックス戦略
| 項目 | ポリシー |
|------|----------|
| 外部コマンド実行 | 原則禁止 (spawn は内部/制限的) |
| ファイルサイズ | 10MB 上限 (CONFIG / rc で検証) |
| 読取バッファ | 4KB ストリーミング + メモリ使用抑制 |
| API 呼び出し | セッション上限 50 (デフォルト) |
| Quota 重量付 | Input 1.0 / Cached 0.25 / Output 4.0 |
| 失敗時動作 | 即時終了 (log.Fatal / error 伝播) |

fsproxy / VFS 統合は「(a) 仮想 FD 管理」「(b) プロトコルレベル許可制」「(c) 監査可能イベントログ」によって最小権限実現を拡張予定。 

---
## 8. エラーモデルと Fail-First 運用
| 分類 | 例 | 振る舞い |
|------|----|----------|
| 設定異常 | 無効 JSON / 不正数値 | 直ちに終了 (再実行で再現) |
| ユーザ入力 | 予期しないフラグ | 明確メッセージ + 非 0 終了 |
| I/O 境界 | サイズ超過 / 書込失敗 | 直ちに終了 (部分結果無効化) |
| LLM API | 認証 / レート / 一時障害 | 限定リトライ → 失敗確定 |
| 内部前提崩壊 | 不変条件違反 | panic / fatal (バグ修正対象) |

改善履歴例: `HandleHelp` が書き込みエラー黙殺 → エラー伝播へ変更 (2025-08)。

---
## 9. コスト & Quota フィロソフィ
- **可視性**: 実行中に残量を逐次表示 (将来: TUI / JSON Stats)  
- **防波堤**: 想定外増幅 (再帰 / 無限分割) を上限カウンタで遮断。  
- **軽量試行優先**: 失敗しやすい初期探索は低トークン (mini モデル) / 決定フェーズのみ高品質モデル。  

---
## 10. 進化ロードマップ (高レベル)
| フェーズ | 概要 | 価値 |
|----------|------|------|
| v3.x 現状 | 統合ヘルプ / Quota / ビルトイン安定化 | 信頼性基盤 |
| fsproxy 完全統合 | llmsh へ安全 FD 経路 / 監査ログ | セキュリティ強化 |
| Unified VFS Server | 両コンポーネント同一抽象で集中管理 | 重複除去 & 一貫性 |
| Two-Stage 実行 | 複雑計画→実行分離 | コスト最適化 + 精度 |
| 高度プリセット/テンプレ | 規範プロンプト自動選択 | 品質 & 組織知共通化 |
| 静的解析 / Lint 強化 | サイレント失敗パターン検出 | 品質自動防衛 |

---
## 11. トレードオフ / 非ゴール
| 項目 | 非ゴール宣言 | 理由 |
|------|---------------|------|
| POSIX 互換完全性 | 目指さない | 守備範囲拡張は攻撃面と複雑性増大 |
| 任意外部コマンド | 追加予定なし | セキュリティモデル崩壊 |
| 巨大バイナリ処理 | 優先外 | LLM 主体ユースケースはテキスト偏重 |
| モデル切替の完全自動推論 | 長期視野 | 初期は明示フラグで制御し透明性維持 |

---
## 12. 代表的デザイン判断例 (Decision Log 抜粋)
| テーマ | 判断 | 根拠/影響 |
|--------|------|-----------|
| Help 出力統合 | `HandleHelp` 共通化 | 20 コマンドの重複排除 / 改修集約 |
| Help 書込失敗処理 | エラー伝播へ変更 | Fail-First 徹底 / サイレント抑止 |
| Dry-Run (patch) | 検証失敗でも exit 0 (現状) | UX 優先 (後で strict モード検討) |
| Quota 重量付 | Output 係数 4.0 | 出力生成コスト高リスク抑制 |
| 外部コマンド禁止 | 内部ビルトインのみ | 攻撃面縮小 + 再現性 |

---
## 13. 今後の改善アイデア (Backlog 候補)
- `patch --strict-dry-run` で検証失敗を非 0 戻り値化。  
- file.Close() エラー透過ラップユーティリティ導入。  
- サイレントパターン検出テスト (unused write / ignored error) 自動化。  
- プリセット選択ヒューリスティクス (ドメイン分類 → テンプレ選択)。  
- Quota 可視化 JSON API エンドポイント / メトリクス出力。  
- LLM 行動メトリクス (retry 分布 / 温度差異) 収集分析。  

---
## 14. ChatCompletion ループとツールプリミティブ
llmcmd は OpenAI ChatCompletion (Function Calling) を中核に「LLM←→ツール実行エンジン」間の反復ループを形成する。LLM 側は逐次的に計画 / 観察 / 変換を行い、以下の最小プリミティブのみを呼び出す:

| ツール | シグネチャ (論理) | 役割 | 主要エラーチェック (Fail-First) |
|--------|-------------------|------|----------------------------------|
| read  | read(fd [,count|lines]) -> data | FD からチャンク / 行読み込みし会話コンテキストへ追加 | fd 範囲 / readable 性 / 上限値 |
| write | write(fd, data[, newline][, eof]) -> status | FD へ書き込み (パイプ/子 stdin/出力集約) | fd 範囲 / writable 性 / 書き込み失敗 |
| spawn | spawn(script[, stdin_fd, stdout_fd]) -> {stdin_fd, stdout_fd, stderr_fd} | llmsh サブプロセス起動 / 既存 FD 直結 (dup2) / 未指定は新規 pipe | スクリプト検証 / 上限プロセス数 / FD 検証 |
| close | close(fd) -> status | 明示的クローズ (書き込みパイプ終端や再利用防止) | fd 範囲 / 既に閉鎖済み判定 |
| exit  | exit(status) -> terminate | ループ終了 (最終結果確定) | 実行中リソースクリーンアップ |

### ループの概念的フロー
```
User Prompt / 初期コンテキスト
     ↓
ChatCompletion (LLM 応答: content または function_call)
     ├─ content: 途中説明/推論を会話へ追加し次ターン
     └─ function_call: {name, arguments(JSON)} を Engine へ渡す
               ↓
          Engine: ExecuteToolCall -> executeRead/Write/Spawn/Close/Exit
               ↓ 成功/失敗結果を function 呼び出し結果メッセージとして LLM へ返却
     (Quota/Timeout/Call Limit 検査)
     ↓ 収束条件 (exit/status, コール上限, タイムアウト)
終了 / 出力確定
```

### デザイン意図
1. **最小集合**: read/write/spawn/close/exit に絞ることで LLM の探索空間を制御し安全性と推論安定性を確保。  
2. **明示的 I/O**: すべて fd 指向でストリーミング可能 (大きなファイルでも段階 read)。  
3. **Chain Introspection**: write(eof=true) / close() によりパイプライン境界を明示して再帰制御を容易化。  
4. **Fail-First**: 各ツールで前提違反は即エラー計上 → ChatContext に失敗理由が戻るため LLM が是正行動を学習。  
5. **再帰可能性**: spawn で起動される llmsh の内側に llmcmd ビルトインが存在し、再帰的に llmcmd を呼びチェーン分解・分析・再集約を行う。  

#### spawn における FD 直結 / 新規生成仕様 (実装準拠)
`spawn(script[, stdin_fd, stdout_fd])`:
1. `stdin_fd` 指定: 指定 FD を llmsh 子プロセス stdin に接続 (dup2 相当)。 fd!=0 なら親側はクローズし FD テーブルで closed マーク。
2. `stdout_fd` 指定: 指定 FD を llmsh 子プロセス stdout に接続。 fd!=1 なら親側で適切に所有権解放 (書込み/読取り方向に応じてクローズ)。
3. 未指定の場合:
     - stdin: 親→子 pipe を生成し親書き込み端の新 FD を割当 (戻り `stdin_fd`).
     - stdout: 子→親 pipe を生成し親読取り端の新 FD を割当 (戻り `stdout_fd`).
4. stderr: 常に新規 pipe を生成し `stderr_fd` を返す (将来オプション化予定)。
5. 後方互換: 旧 `in_fd` / `out_fd` は削除 (非互換変更)。

安全 / Fail-First チェック:
- `stdin_fd` / `stdout_fd` が範囲内かつ nil でないか検証。
- 指定 FD が期待方向 (Readable / Writable) でない場合エラー。
- 空 script / 空白のみ script は即エラー。
- プロセス起動失敗時は統計加算後エラー返却。

観測性:
- 戻り JSON: `{success, stdin_fd, stdout_fd, stderr_fd, pid, script_len}`。
- 既存 FD 直結時も `stdin_fd` / `stdout_fd` は再掲され LLM が後続 read/write 戦略を構築可能。

今後の改良候補:
- stderr の統合 / 分離ポリシー選択フラグ。
- 使用後自動 close / 明示 close ポリシー切替。
- 実行中コマンドの状態問い合わせツール追加。

### 再帰 (llmsh → llmcmd) のパターン
```
llmsh pipeline: cat data | grep ERROR | llmcmd "要約" | llmcmd "改善策抽出"
                                          ▲ (llmsh の builtin llmcmd 呼び出し)
```
- llmsh は軽量テキスト整形 / 抽出でノイズ除去 → llmcmd が高レベル要約 / 推論を実行。  
- Quota は継承され、深い再帰がコスト暴走しないよう上限で統制。  
- 失敗 (例: fd 不正) は中間段階で表面化し、上位再帰ステップが代替戦略 (追加フィルタ / chunk 再分割) を生成。  

### ツール実装とコード参照 (抜粋)
| 機能 | 実装関数例 | 主な内部指標更新 |
|------|------------|------------------|
| read  | engine.go: executeRead | ReadCalls / BytesRead / ErrorCount |
| write | engine.go: executeWrite | WriteCalls / BytesWritten / ErrorCount |
| spawn | engine.go: executeSpawn | SpawnCalls / runningCommands map |
| close | engine.go: executeClose | CloseCalls / FD 状態マーク |
| exit  | engine.go: executeExit | ExitCalls / 終了フラグ |

### エラーハンドリング方針の一貫性
- すべての executeXxx 関数は: 入力検証 → 状態確認 → 実処理 → メトリクス更新 → 明示的エラー or 正常戻り文字列。  
- io.EOF は read において「正常終端イベント」として文言を返却し LLM が次アクション判断 (追加 read 不要 / 別 fd へ移行) をしやすい形に整形。  

### 将来拡張余地
- **help** ツール (既存) の高度化: LLM が利用可能プリミティブ一覧と残 quota を 1 コールで取得。  
- **batch** 仮想ツール: 複数 write / read を 1 関数呼び出しに圧縮し API 往復削減 (要安全検証)。  

## 15. VFS (Virtual File System) 設計思想
VFS は llmcmd / llmsh の **安全・再現性・最小権限** を保証する中核抽象であり、外部 OS ファイルシステム差異・権限・潜在的な高コスト I/O を LLM 視点で統一/制御するレイヤである。

### 15.1 目的
1. セキュリティ境界: 許可されたファイル/サイズ/モード以外を即ブロック (Fail-First)。
2. コスト制御: 大容量/バイナリ誤読によるトークン浪費を事前遮断。
3. 冪等性/再現性: セッション内でのファイル可視集合を確定化し “隠れ入力” を排除。
4. 観測性: すべてのファイルアクセスをイベント/統計に反映させ後追い分析を可能化。
5. 拡張性: 物理 FS → fsproxy → リモート/VFS サーバ への段階的移行経路を提供。

### 15.2 レイヤ構造
```
LLM / Orchestrator
     ↓ (tools: read/write/spawn/close)
Engine FD Table (論理 FD -> Stream 抽象)
     ↓
VFS インターフェース (OpenFile / CreateTemp / Remove / List)
     ↓
FS Proxy Adapter (ポリシ & 許可リスト検査, 実ファイル制限)
     ↓
ホスト FileSystem (制限付き)
```

### 15.3 コア設計原則
| 原則 | 説明 | 具体例 |
|------|------|--------|
| Least Privilege | 明示許可されたファイル以外へアクセス禁止 | allow-list 外アクセスで即エラー |
| Text-Only Bias | バイナリを積極検出し拒否 | 512B スニッファ + 拡張子フィルタ |
| Deterministic View | セッション開始時の許可集合固定 (継承時は merge) | 再帰 llmsh が親集合を引き継ぐ |
| Fail-First Validation | サイズ/モード/存在/許可を最初に検証 | open → 失敗時即終了 |
| Stream Orientation | 巨大ファイルを分割 read | read(fd, count) チャンク駆動 |
| Extensible Backend | fsproxy → リモートストレージ差替余地 | VirtualFileSystem interface |
| Auditability | すべての open/close をメトリクス/ログに反映 | BytesRead, OpenCalls |

### 15.4 FD ライフサイクル
1. 割当: Engine 初期化 (0 stdin / 1 stdout / 2 stderr) + 入力ファイル列挙で拡張。  
2. 利用: read/write でストリーム操作 (ReadCalls/WriteCalls カウント)。  
3. 終端: write(eof=true) または close(fd) でチェイントラバース → exit code 収集。  
4. 再利用防止: closedFds マークで再操作阻止 (二重 close はエラー計上)。  
5. 監査: traverseChainOnEOF で最終結果メッセージ化。  

### 15.5 セキュリティポリシ (fsproxy 統合観点)
| 項目 | 方針 | 例 |
|------|------|----|
| アクセス制御 | 実行モード (llmcmd / llmsh-virtual / llmsh-real) 毎に許可判定 | real モードのみ限定的実 FS 許可 |
| サイズ上限 | 10MB 超は open 前に拒否 | 巨大ログの accidental 読込抑止 |
| バイナリ拒否 | Null/高非印字率閾値 >30% で拒否 | 画像/PDF 等を早期遮断 |
| Temp 管理 | CreateTemp は VFS 管理領域のみ | 任意 OS パス生成禁止 |
| 削除操作 | 明示 RemoveFile のみ | write 上書きでの暗黙削除禁止 |
| 権限モード | write 要求時は allow-list+モード検査 | w+, a モードの無制限拡張防止 |

### 15.6 代表インターフェース設計 (抽象)
```
type VirtualFileSystem interface {
     OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error)
     CreateTemp(pattern string) (io.ReadWriteCloser, string, error)
     RemoveFile(name string) error
     ListFiles() []string
}
```
実装はポリシ層 (FileAccessController) と結合し “存在しても許可されなければ開かない” を保証。

### 15.7 不変条件 (Invariants)
| ID | 不変条件 | 理由 | 破れた場合 |
|----|----------|------|------------|
| V1 | fileDescriptors[i] == nil ↔ fd 未使用 | ゴミ FD 誤参照防止 | panic/予期せぬ型アサート失敗 |
| V2 | 閉鎖済 FD へ read/write 不可 | データ競合/不正 I/O 避止 | Fail-First で即エラー |
| V3 | すべての open は BytesRead/Write カウンタと整合 | 監査/コスト追跡整合 | コスト表示ずれ |
| V4 | バイナリ検出後に read へ進まない | トークン浪費/汚染防止 | 高額無駄コスト |
| V5 | 再帰セッションで親許可集合 ⊆ 子許可集合 | Least Privilege 維持 | 子が広すぎる権限獲得 |

### 15.8 エラーモデル (VFS 特化)
| 分類 | 例 | 対応 |
|------|----|------|
| Permission | allow-list 外 | 即エラー & 統計 ErrorCount++ |
| Size | >10MB | open 前拒否 |
| Binary | ヒューリスティック一致 | open 拒否 (テキスト変換要求促す) |
| Closed FD | read/write/close on closed | “already closed” エラー |
| Unknown FD | 範囲外 index | “invalid file descriptor” |

### 15.9 将来拡張
| アイデア | 概要 | 期待効果 |
|---------|------|----------|
| Lazy Open | 初回 read 要求まで遅延 open | 無駄 file handle 削減 |
| Read-Ahead Cache | 小チャンク逐次 read をまとめる | API 往復回数削減 |
| Content Hashing | 読込テキストの重複検知 | 再生成コスト抑制 |
| Policy Profiles | “strict”, “lenient” プロファイル切替 | 実験 vs 本番 最適化 |
| Remote VFS | gRPC/Unix Socket 経由で隔離 | サンドボックス強化 |
| Fine-grained Quota | ファイル別最大行/バイト制限 | ホットスポット暴走防止 |

### 15.10 非ゴール / 制約
| 項目 | 非ゴール | 理由 |
|------|---------|------|
| 任意バイナリ補助変換 | Base64 自動化など | LLM の推論領域外/安全境界曖昧 |
| 階層全探索 API | 再帰ディレクトリ漫遊 | トークン/時間コスト膨張 |
| 透過巨大ストリーム (>GB) | ストリーム透過 | LLM 文脈サイズと乖離 |

### 15.11 まとめ (VFS)
VFS は「安全境界 (allow-list/サイズ/バイナリ)」「再現性 (決定的可視領域)」「コスト制御 (Text-Only / Streaming)」を同時達成する基盤。抽象インターフェース + fsproxy 統合により段階的高度化を可能にし、Fail-First で黙殺を防ぐ。これにより LLM は “信頼できる I/O 基盤” を前提に高レベル計画へ集中できる。

## 16. まとめ
llmcmd / llmsh は「LLM が安全・一貫・監査可能な形で自己拡張的処理を行う」ための二形態 UI であり、設計核心は以下に凝縮される:
- 小さく信頼できるビルトインプリミティブ
- Fail-First + 明示的 Quota によるコスト境界
- 再帰/段階的実行とパイプライン合成で複雑度制御
- 共有 VFS / fsproxy で統一 I/O セマンティクス
- セキュリティと拡張性のバランスを意図的に最適化

このドキュメントは新規コントリビュータが「どの価値基準で判断すべきか」を最短で把握するためのコンパスである。継続改善時は決定の根拠を本書に追記し知識の腐敗を防ぐこと。
