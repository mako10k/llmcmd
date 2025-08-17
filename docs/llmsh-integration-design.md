# llmsh Integration Design Document (Updated)

本書の旧版は FSProxy 前提の設計でしたが、現在は fsproxy を撤去し、I/O は `internal/mux.Conn` による stdio MUX と vfsd クライアントで統一されています。llmsh 連携は以下の方針に置き換えます。

## 統一方針（現行）
- VFS: `internal/app/vfsd_client.go` を通じて vfsd と通信（length‑prefixed JSON over stdio、`internal/mux.Conn`）。
- STDIO: 親エンジンが fd0/1/2 を stdio MUX で管理し、子プロセス（llmsh）は mux 経由の論理 FD を継承。
- Broker/Quota: LLM Broker はアプリ層でオプトイン。I/O 経路には関与しない（直列化と会計のみ担当）。

## 今後の作業
- 既存ドキュメントの FSProxy 記述は参照しないこと。
- テスト/例示は `stdin_fd`/`stdout_fd` の論理 FD と MUX 前提に更新すること。
        vfs:      vfs,
        executor: executor,
        parser:   parser.NewParser(),
        help:     NewHelpSystem(),
    }, nil
}
```

### 互換性保証

#### 1. API Compatibility
- 既存のVFS interface methodsを全て維持
- 既存のllmshコマンド（cat, grep, sed等）はコード変更なしで動作
- Error handling behaviorの一貫性維持

#### 2. Configuration Compatibility
- 既存の-i/-o flagsは従来通り動作
- 新しいFSProxy機能はopt-inで有効化
- Legacy modeでの完全な後方互換性

#### 3. Performance Compatibility
- Legacy VFS実装をfallback pathとして維持
- FSProxy統合による性能向上の測定・検証
- 必要に応じてlegacy mode自動選択

### テスト戦略

#### 1. Unit Tests
- VFSFSProxyAdapter各メソッドの単体テスト
- Legacy VFS compatibility tests
- Error handling and edge cases

#### 2. Integration Tests
- llmsh commands with FSProxy integration
- File operation consistency between legacy and FSProxy modes
- Resource management and cleanup verification

#### 3. E2E Tests
- Complete llmsh workflows with FSProxy enabled
- Performance benchmarking and resource usage monitoring
- Compatibility verification with existing test suites

### 成功指標

1. **機能完全性**: 既存llmshコマンドが100%動作
2. **性能向上**: FSProxy統合による5-10%の性能改善
3. **安定性**: race detector含む全テストが通過
4. **拡張性**: Pipeline support実装の準備完了

### リスク分析

#### 技術リスク
- **Legacy compatibility**: 既存VFS行動の完全再現難易度 → 段階的移行で対応
- **Performance regression**: FSProxy overhead → benchmarkingで検証・最適化
- **Resource management**: fd leakage risk → comprehensive cleanup testing

#### 対策
- Comprehensive test coverage (unit + integration + E2E)
- Performance monitoring and fallback mechanism
- Gradual rollout with feature flags for safety

---

このDocument通りに実装することで、llmsh integrationを安全かつ効率的に達成できます。
