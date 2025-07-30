# Two-Stage LLM Architecture Implementation Plan

## Overview

llmcmdの次世代アーキテクチャとして、GPT-4oによる高度な推論・計画とGPT-4o-miniによる効率的実行を組み合わせた二段階処理システムを実装する。

## Architecture Concept

### Stage 1: Planning & Analysis (GPT-4o)
- **役割**: 意図解釈、仕組み理解、実行計画生成
- **入力**: ユーザープロンプト、ファイル情報、システム制約
- **出力**: 中間言語による実行計画
- **責任範囲**:
  - 複雑なタスクの分解・構造化
  - 最適なツール組み合わせの決定
  - エラー処理戦略の立案
  - データフロー設計

### Stage 2: Execution (GPT-4o-mini)
- **役割**: 計画に基づく具体的実行
- **入力**: 中間言語実行計画、入力データ
- **出力**: 最終結果
- **責任範囲**:
  - ツール呼び出しの実行
  - テキスト変換・データ処理
  - パイプライン管理
  - 結果出力

## Intermediate Language Design

### ExecutionPlan Structure
```json
{
  "version": "1.0",
  "metadata": {
    "task_type": "text_conversion",
    "complexity": "medium",
    "estimated_steps": 3,
    "fallback_strategy": "manual_conversion"
  },
  "workflow": [
    {
      "step": 1,
      "action": "read_input",
      "params": {
        "source": "fd:3",
        "type": "markdown",
        "expected_size": "medium"
      }
    },
    {
      "step": 2,
      "action": "spawn_command",
      "params": {
        "cmd": "sed",
        "args": ["s/^#/h1./g"],
        "input_fd": 3,
        "pattern": "markdown_to_textile_headers"
      }
    },
    {
      "step": 3,
      "action": "manual_transform",
      "params": {
        "type": "text_processing",
        "operations": [
          {"from": "**bold**", "to": "*bold*"},
          {"from": "*italic*", "to": "_italic_"}
        ]
      }
    },
    {
      "step": 4,
      "action": "write_output",
      "params": {
        "destination": "fd:1",
        "format": "textile"
      }
    }
  ]
}
```

### Action Types
- `read_input`: データ読み込み
- `spawn_command`: built-inコマンド実行
- `manual_transform`: LLMによるテキスト変換
- `write_output`: 結果出力
- `conditional_branch`: 条件分岐
- `error_recovery`: エラー回復

## Implementation Roadmap

### Phase 1: Foundation (v3.1.0)
- [ ] 中間言語仕様の策定
- [ ] ExecutionPlan構造体定義
- [ ] GPT-4o用プランニングプロンプト作成
- [ ] 基本的なパーサー実装

### Phase 2: Core Implementation (v3.2.0)
- [ ] TwoStageOrchestrator実装
- [ ] 中間言語→ツール実行変換器
- [ ] GPT-4o-mini用実行プロンプト
- [ ] エラーハンドリング機構

### Phase 3: Configuration & Control (v3.3.0)
- [ ] 設定ファイルでのモデル選択
- [ ] 自動/手動モード切り替え
- [ ] デバッグ・ログ機能
- [ ] パフォーマンス監視

### Phase 4: Advanced Features (v3.4.0)
- [ ] 複雑性自動判定
- [ ] 適応的モデル選択
- [ ] コスト最適化
- [ ] プラン再利用・キャッシュ

## Technical Specifications

### Configuration Extension
```yaml
llm:
  # Two-stage configuration
  two_stage:
    enabled: true
    complexity_threshold: 0.7
    
  # Model assignments
  planning_model: "gpt-4o"
  execution_model: "gpt-4o-mini"
  
  # Cost controls
  max_planning_tokens: 2000
  max_execution_tokens: 8000
```

### CLI Extension
```bash
# Force single-stage mode
llmcmd --single-stage "simple task"

# Force two-stage mode
llmcmd --two-stage "complex task"

# Show execution plan only
llmcmd --plan-only "analyze this task"

# Execute with custom plan
llmcmd --execute-plan plan.json
```

## Benefits & Trade-offs

### Expected Benefits
- **品質向上**: 複雑タスクでの推論品質向上
- **コスト効率**: 適材適所のモデル使用
- **透明性**: 実行計画の可視化
- **デバッグ性**: 段階的な問題特定

### Trade-offs
- **レイテンシ**: 二段階処理による遅延増加
- **複雑性**: アーキテクチャの複雑化
- **開発コスト**: 実装・テスト工数増加

## Risk Mitigation

### Fallback Strategies
1. **プランニング失敗**: 既存single-stage処理へフォールバック
2. **実行失敗**: マニュアル実行モードへ切り替え
3. **パフォーマンス問題**: 動的なsingle-stage切り替え

### Compatibility
- 既存のAPIと完全互換性維持
- 段階的移行可能な設計
- レガシーモードの永続サポート

## Success Metrics

### Quality Metrics
- タスク成功率の向上
- ユーザー満足度スコア
- エラー率の削減

### Efficiency Metrics
- 平均実行時間
- トークン使用効率
- コスト削減率

### Technical Metrics
- システム可用性
- レスポンス時間
- スループット

## Timeline

- **Phase 1**: 4週間 (仕様策定・基盤実装)
- **Phase 2**: 6週間 (コア機能実装)
- **Phase 3**: 4週間 (設定・制御機能)
- **Phase 4**: 6週間 (高度機能・最適化)

**Total**: 約5ヶ月での完成予定

## Conclusion

この二段階アーキテクチャにより、llmcmdは複雑なタスクに対してより高品質な結果を、効率的なコストで提供できるようになる。段階的な実装とフォールバック戦略により、リスクを最小化しながら革新的な機能を提供する。

---
*Last Updated: 2025-07-30*
*Version: Draft v1.0*
