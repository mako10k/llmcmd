# llmsh Integration Design Document

## Phase 3.1: llmsh-FSProxy Integration Architecture

### 概要
FSProxy Protocolとllmshを統合し、llmsh側でfd管理テーブルとFSProxy機能を活用できるようにする。

### 設計目標
1. **API互換性維持**: 既存llmshコマンドのAPIを変更せずに統合
2. **透明な統合**: ユーザーは統合を意識せずに使用可能
3. **パフォーマンス向上**: fd管理によるリソース効率化
4. **拡張性確保**: Pipeline supportへの準備

### アーキテクチャ設計

#### 1. VFS-FSProxy Adapter Layer
```go
// VFSFSProxyAdapter provides FSProxy functionality through VFS interface
type VFSFSProxyAdapter struct {
    fsProxy     *FSProxyManager
    vfs         *VirtualFileSystem  // Legacy VFS for compatibility
    fdTable     *FileDescriptorTable
    clientID    string
}

// Implement tools.VirtualFileSystem interface
func (adapter *VFSFSProxyAdapter) OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error)
func (adapter *VFSFSProxyAdapter) CreateTemp(dir, pattern string) (io.ReadWriteCloser, string, error)
func (adapter *VFSFSProxyAdapter) RemoveFile(name string) error
func (adapter *VFSFSProxyAdapter) Exists(name string) bool
```

#### 2. Enhanced VirtualFileSystem Integration
```go
// Enhanced VirtualFileSystem with FSProxy support
type VirtualFileSystem struct {
    // Legacy fields (maintained for compatibility)
    mu          sync.RWMutex
    files       map[string]*VirtualFile
    realFiles   map[string]io.ReadWriteCloser
    fileAccess  map[string]FileAccess
    
    // New FSProxy integration
    fsProxy     *FSProxyManager
    adapter     *VFSFSProxyAdapter
    useProxy    bool  // Enable/disable FSProxy integration
}
```

#### 3. File Operation Methods Enhancement
```go
// Enhanced OpenFile with FSProxy support
func (vfs *VirtualFileSystem) OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
    if vfs.useProxy && vfs.fsProxy != nil {
        // Use FSProxy for file operations
        return vfs.adapter.OpenFile(name, flag, perm)
    }
    
    // Fallback to legacy VFS implementation
    return vfs.openFileLegacy(name, flag, perm)
}
```

### 統合フェーズ計画

#### Week 1: 設計・PoC
- [ ] VFSFSProxyAdapter interface design
- [ ] Legacy VFS compatibility layer
- [ ] Basic integration PoC with simple read/write operations
- [ ] Unit test framework for adapter layer

#### Week 2: 本実装・テスト
- [ ] Complete VFSFSProxyAdapter implementation
- [ ] Enhanced VirtualFileSystem with FSProxy integration
- [ ] Migration of existing llmsh commands to use adapter
- [ ] Comprehensive unit tests and integration tests

#### Week 3: 統合テスト・最適化
- [ ] E2E testing with existing llmsh command suite
- [ ] Performance benchmarking (legacy vs FSProxy)
- [ ] Error handling and edge case validation
- [ ] Documentation and code review

### 実装詳細

#### Configuration Extension
```go
// Enhanced Config with FSProxy support
type Config struct {
    // Existing fields
    InputFiles   []string
    OutputFiles  []string
    VirtualMode  bool
    QuotaManager interface{}
    Debug        bool
    
    // New FSProxy configuration
    EnableFSProxy     bool
    FSProxyMode       bool  // VFS-only mode for restriction
    FSProxyPipeFile   string // Communication pipe path
}
```

#### Shell Enhancement
```go
// Enhanced Shell initialization
func NewShell(config *Config) (*Shell, error) {
    // Create enhanced VFS with optional FSProxy support
    vfs := NewVirtualFileSystemWithFSProxy(
        config.InputFiles, 
        config.OutputFiles,
        config.EnableFSProxy,
        config.FSProxyMode,
        config.FSProxyPipeFile,
    )
    
    // Rest of initialization remains the same
    executor := NewExecutor(vfs, nil, config.QuotaManager)
    
    return &Shell{
        config:   config,
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
