# GitHub Issue Proposal: mcp-assoc-memory ファイルエクスポート機能の信頼性問題

## Issue Title
[BUG] ファイルエクスポート機能の動作不安定性・信頼性問題

## Issue Body

### 問題の背景

`mcp-assoc-memory` の `memory_sync` export機能において、ファイル出力の信頼性に問題があることが発見されました。知識移転作業中にファイルエクスポート機能の動作が怪しいことが判明しています。

### 発生した問題

#### 1. **ファイルエクスポート動作の不安定性**
- `memory_sync` export操作での予期しない動作
- エクスポートファイルの内容検証が不十分
- ファイル出力の成功/失敗判定が曖昧

#### 2. **検証機能の不足**
```python
# 現在の問題: エクスポート後の検証が不十分
result = memory_sync(operation="export", file_path="backup.json")
# ファイルが正しく作成されたか、内容が正確かの検証が不足
```

#### 3. **エラーハンドリングの不明確性**
- ファイル書き込み失敗時の適切なエラー報告なし
- 部分的な書き込み失敗の検出困難
- ファイルサイズやチェックサムによる整合性確認なし

### 具体的な症状

1. **ファイル作成の不確実性**
   - exportコマンド実行後、ファイルが期待通りに作成されない場合がある
   - 空ファイルや不完全なファイルが生成される可能性

2. **内容の整合性問題**
   - エクスポートされたデータの完全性が保証されない
   - メモリ内容とエクスポートファイル内容の不一致

3. **進行状況の不透明性**
   - 大容量データのエクスポート時の進行状況が不明
   - エクスポート処理の成功/失敗判定が不明確

### 再現手順

```python
# 問題の再現例
from mcp_assoc_memory import memory_sync

# 1. 大量のメモリデータを作成
# 2. memory_sync export を実行
result = memory_sync(
    operation="export",
    file_path="test_export.json",
    scope="work"
)

# 3. ファイルの存在確認
# 4. ファイル内容の整合性確認
# → この段階で問題が発見される可能性
```

### 提案する修正

#### 1. **ファイル整合性検証機能**

```python
def export_with_verification(operation, file_path, **kwargs):
    """エクスポート後の検証機能付きexport"""
    
    # エクスポート実行
    result = memory_sync(operation=operation, file_path=file_path, **kwargs)
    
    # ファイル存在確認
    if not os.path.exists(file_path):
        raise FileNotFoundError(f"Export file not created: {file_path}")
    
    # ファイルサイズ確認
    file_size = os.path.getsize(file_path)
    if file_size == 0:
        raise ValueError("Export file is empty")
    
    # 内容整合性確認（JSON形式の場合）
    try:
        with open(file_path, 'r') as f:
            json.load(f)  # JSON形式確認
    except json.JSONDecodeError:
        raise ValueError("Export file contains invalid JSON")
    
    return result
```

#### 2. **進行状況表示機能**

```python
def export_with_progress(operation, file_path, **kwargs):
    """進行状況表示付きexport"""
    
    print(f"Starting export to {file_path}...")
    
    # メモリ数取得
    memory_count = get_memory_count(kwargs.get('scope'))
    print(f"Exporting {memory_count} memories...")
    
    result = memory_sync(operation=operation, file_path=file_path, **kwargs)
    
    print(f"Export completed: {file_path}")
    print(f"File size: {os.path.getsize(file_path)} bytes")
    
    return result
```

#### 3. **エラーハンドリング強化**

```python
def robust_export(operation, file_path, **kwargs):
    """堅牢なエクスポート機能"""
    
    try:
        # バックアップファイルパス生成
        backup_path = f"{file_path}.backup.{int(time.time())}"
        
        # エクスポート実行
        result = memory_sync(operation=operation, file_path=file_path, **kwargs)
        
        # 検証実行
        verify_export_file(file_path)
        
        # バックアップ作成
        shutil.copy2(file_path, backup_path)
        
        return result
        
    except Exception as e:
        # エラー詳細ログ
        logging.error(f"Export failed: {e}")
        logging.error(f"File path: {file_path}")
        logging.error(f"Parameters: {kwargs}")
        
        # 失敗ファイル削除
        if os.path.exists(file_path):
            os.remove(file_path)
        
        raise ExportError(f"Export operation failed: {e}")
```

### 期待される効果

1. **信頼性向上**: ファイルエクスポート操作の成功/失敗が明確に判定される
2. **デバッグ支援**: 問題発生時の詳細情報が得られる
3. **データ整合性**: エクスポートされたデータの完全性が保証される
4. **ユーザビリティ**: 進行状況が可視化され、操作の透明性が向上

### 実装優先度

**HIGH** - データの整合性とツールの信頼性に直結する重要な問題

### 関連ファイル

- `src/mcp_assoc_memory/server.py` - memory_sync実装
- `src/mcp_assoc_memory/storage/` - データストレージ関連
- `tests/` - テストケース追加が必要

### テストケース

```python
def test_export_file_creation():
    """エクスポートファイルが正しく作成されることを確認"""
    pass

def test_export_content_integrity():
    """エクスポート内容の整合性を確認"""
    pass

def test_export_error_handling():
    """エクスポート失敗時の適切なエラー処理を確認"""
    pass

def test_large_data_export():
    """大容量データのエクスポート動作を確認"""
    pass
```

---

この問題の解決により、`mcp-assoc-memory` のファイルエクスポート機能がより信頼性の高いものになり、知識移転や バックアップ操作を安心して実行できるようになります。

## Labels
- bug
- priority: high
- reliability
- file-operations

## Assignee
@mako10k

## Related Issues
- データの整合性保証
- エラーハンドリング強化
- ユーザビリティ向上
