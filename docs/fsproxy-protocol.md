# FS Proxy Protocol Specification

## 概要

FS Proxyプロトコルは、LLM実行環境において子プロセスのファイルアクセスを制御するための通信プロトコルです。親プロセス（llmsh/llmcmd）がFS Proxy Managerとして動作し、子プロセスからのファイル操作リクエストを処理します。

## アーキテクチャ

```
┌─────────────────┐    FD 3 (pipe)    ┌─────────────────┐
│   子プロセス     │ ◄──────────────► │   親プロセス     │
│   (LLM実行)     │                  │   (llmsh/llmcmd) │
│                │                  │                │
│  FS Client     │                  │  FS Proxy      │
│                │                  │  Manager       │
└─────────────────┘                  └─────────────────┘
                                           │
                                           ▼
                                     ┌─────────────┐
                                     │    VFS      │
                                     │ (制限環境)   │
                                     └─────────────┘
```

## プロトコル仕様

### 通信方式

- **通信手段**: Unix pipe (os.Pipe())
- **継承方法**: 子プロセスはFD 3でFS Proxyにアクセス
- **データ形式**: テキストベース + バイナリデータ
- **同期方式**: リクエスト/レスポンス同期通信

### メッセージ形式

#### リクエスト形式

```
COMMAND param1 param2 ...\n
[binary_data]  # WRITEコマンドの場合のみ
```

#### レスポンス形式

```
STATUS data\n
[binary_data]  # READコマンドのレスポンスの場合
```

- **STATUS**: `OK` または `ERROR`
- **data**: ステータスに応じたデータまたはエラーメッセージ

## コマンド仕様

### 1. open (JSON プライマリ)

現在の実装は JSON 長さプリフィックス付きフレーミングを正規形とし、OPEN 等のテキストコマンドはレガシーとして非推奨です。`open` 成功時の結果は最小で `{"handle": <u32>}` のみを返します。作成や仮想ファイル等のメタ情報は返しません（将来 `stat` 相当を追加予定）。

モード:
- `r` 既存のみ読み込み（存在しない場合 `E_NOENT`）
- `w` 書き込み（truncate/create, 読み不可）
- `a` 追記のみ（append/create, 読み不可, 読み試行で `E_PERM`）
- `rw` 読み書き（存在しなければ create, truncate しない）

例 (JSON):
```
{"id":"7","op":"open","params":{"path":"out.txt","mode":"w"}}
→ {"id":"7","ok":true,"result":{"handle":5}}
```

### 2. read コマンド (JSON primary)

指定されたファイル番号から指定バイト数を読み込みます。IsTopLevelCmdがtrueの場合、VFSサーバが実ファイルを開きます。

#### リクエスト
```
READ fileno size isTopLevel\n
```

**パラメータ**:
- `fileno`: ファイル番号
- `size`: 読み込みバイト数
- `isTopLevel`: トップレベル実行フラグ (`true`|`false`)
  - `true`: VFSサーバが実ファイルを開く（制限なし）
  - `false`: VFS制限環境での読み込み
- `size`: 読み込みバイト数

#### レスポンス

**成功時**:
```
OK actual_size\n
[binary_data]
```
- `actual_size`: 実際に読み込まれたバイト数
- `binary_data`: 読み込まれたデータ（actual_sizeバイト）

**EOF時**:
```
OK 0\n
```

**エラー時**:
```
ERROR message\n
```

**エラーパターン**:
- `ERROR READ requires fileno, size, and isTopLevel` - パラメータ不足
- `ERROR invalid fileno: 99999` - 無効なファイル番号
- `ERROR invalid size: abc` - 無効なサイズ
- `ERROR invalid isTopLevel: maybe` - 無効なisTopLevelフラグ
- `ERROR READ not yet implemented` - 未実装（Phase 1）

### 3. write コマンド (JSON primary)

指定されたファイル番号に指定データを書き込みます。

#### リクエスト
```
WRITE fileno size\n
[binary_data]
```

**パラメータ**:
- `fileno`: ファイル番号
- `size`: 書き込みバイト数
- `binary_data`: 書き込むデータ（sizeバイト）

#### レスポンス

**成功時**:
```
OK written_size\n
```
- `written_size`: 実際に書き込まれたバイト数

**エラー時**:
```
ERROR message\n
```

**エラーパターン**:
- `ERROR WRITE requires fileno and size` - パラメータ不足
- `ERROR invalid fileno: 99999` - 無効なファイル番号
- `ERROR invalid size: abc` - 無効なサイズ
- `ERROR failed to read data: reason` - データ読み込みエラー
- `ERROR WRITE not yet implemented` - 未実装（Phase 1）

### 4. close コマンド (JSON primary)

指定されたファイル番号のファイルを閉じます。

#### リクエスト
```
CLOSE fileno\n
```

**パラメータ**:
- `fileno`: ファイル番号

#### レスポンス

**成功時**:
```
OK\n
```

**エラー時**:
```
ERROR message\n
```

**エラーパターン**:
- `ERROR CLOSE requires fileno` - パラメータ不足
- `ERROR invalid fileno: abc` - 無効なファイル番号
- `ERROR CLOSE not yet implemented` - 未実装（Phase 1）

## エラーハンドリング

### 通信レベルエラー

#### 1. パイプ切断
```go
if err == io.EOF {
    // 子プロセスがパイプを閉じた（正常終了）
    return nil
}
```

#### 2. 読み込みエラー
```go
log.Printf("FS Proxy: Error reading request: %v", err)
continue  // エラーをログに記録して継続
```

#### 3. 送信エラー
```go
log.Printf("FS Proxy: Error sending response: %v", err)
return err  // 致命的エラーとして終了
```

### プロトコルレベルエラー

#### 1. 空リクエスト
```
ERROR empty request
```

#### 2. 不明なコマンド
```
ERROR unknown command: INVALID
```

#### 3. パラメータエラー
```
ERROR OPEN requires filename and mode
ERROR invalid fileno: abc
ERROR invalid size: xyz
```

## 実装状況

### Phase 1 (完了)
- ✅ 基本プロトコル構造
- ✅ OPEN コマンド（基本実装）
- ✅ エラーハンドリング
- ✅ 通信基盤

### Phase 2 (予定)
- ⏳ 完全なfd管理テーブル
- ⏳ READ/WRITE/CLOSE完全実装
- ⏳ llmsh統合
- ⏳ パイプライン対応

## セキュリティ考慮事項

1. **VFS制限**: 子プロセスは親プロセスのVFS経由でのみファイルアクセス可能
2. **コンテキスト分離**: `internal`コンテキストではVFS制限を適用、`user`コンテキストでは制限なし
3. **fd管理**: 親プロセスが全てのファイルディスクリプタを管理
4. **パラメータ検証**: 全てのリクエストパラメータを検証
5. **エラー隔離**: 子プロセスのエラーが親プロセスに影響しない設計

### VFSコンテキスト制御

**internalコンテキスト**: LLM内部処理によるファイルアクセス
- VFS制限適用（ユーザー指定ファイルのみアクセス可能）
- セキュリティ最優先
- LLMが自動実行する際の安全性を確保

**userコンテキスト**: ユーザー明示指定によるファイルアクセス  
- VFS制限なし（システム全体へのアクセス可能）
- ユーザー責任での実行
- `-i/-o`フラグ指定時など、ユーザーが明示的に指定した場合

## 使用例 (Legacy テキスト例は簡略化)

```go
// 子プロセス側 (FS Client)
client, _ := fsclient.NewFSClient()

// JSON リクエスト例 (open -> write -> close)
send({"id":"1","op":"open","params":{"path":"log.txt","mode":"w"}})
→ {"id":"1","ok":true,"result":{"handle":4}}
send({"id":"2","op":"write","params":{"h":4,"data":"aGVsbG8="}})
→ {"id":"2","ok":true,"result":{"written":5}}
send({"id":"3","op":"close","params":{"h":4}})
→ {"id":"3","ok":true}

// データを書き込む
data := []byte("Hello, World!")
client.Write(fileno1, data)

// ファイルを閉じる
client.Close(fileno1)
client.Close(fileno2)
```

```go
// 親プロセス側 (FS Proxy Manager)
// 親側: 長さプリフィックス JSON ループでフレームを処理
// (擬似コード)
loop read 4 bytes -> len -> read JSON -> dispatch by op
```
