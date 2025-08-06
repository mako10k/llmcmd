# FSProxy Protocol Comprehensive Test Plan

## 概要

fsproxy protocol実装の完全テスト計画。セキュリティ要件、多重実行モード、リソース管理機能を網羅的にテストします。

## テスト戦略

### 🎯 テスト目標
1. **機能完全性**: 全プロトコルコマンドの正常動作確認
2. **セキュリティ準拠**: セキュリティ要件への完全準拠確認
3. **リソース管理**: メモリリーク・プロセスリーク防止確認
4. **エラーハンドリング**: 異常系での適切な処理確認
5. **パフォーマンス**: 大量データ・長時間実行での安定性確認

## 1. バイナリ実行コマンド別テスト

### 1.1 llmcmd コマンドテスト

#### Test Category: Basic llmcmd Operations
```bash
# Test Suite: tests/integration/llmcmd/
├── basic_execution/
│   ├── single_input_file.sh      # llmcmd -i input.txt "prompt"
│   ├── single_output_file.sh     # llmcmd -o output.txt "prompt"
│   ├── input_output_combined.sh  # llmcmd -i input.txt -o output.txt "prompt"
│   ├── multiple_inputs.sh        # llmcmd -i file1.txt -i file2.txt "prompt"
│   ├── multiple_outputs.sh       # llmcmd -o out1.txt -o out2.txt "prompt"
│   └── stdin_processing.sh       # echo "data" | llmcmd "prompt"
```

#### Test Category: llmcmd Security Enforcement
```bash
# Test Suite: tests/security/llmcmd/
├── access_control/
│   ├── input_only_write_denied.sh   # -i file への書き込み拒否確認
│   ├── output_only_read_denied.sh   # -o file からの読み取り拒否確認
│   ├── unlisted_file_denied.sh      # 指定外ファイルへのアクセス拒否確認
│   ├── directory_traversal.sh       # ../../../etc/passwd 等の拒否確認
│   └── symlink_attack.sh            # シンボリックリンク経由攻撃の防止確認
```

### 1.2 llmsh --virtual コマンドテスト

#### Test Category: llmsh Virtual Mode Operations
```bash
# Test Suite: tests/integration/llmsh_virtual/
├── basic_operations/
│   ├── file_access_virtual.sh      # --virtual -i/-o ファイルアクセス
│   ├── internal_commands.sh        # llmcmd 内部コマンド実行
│   ├── pipeline_virtual.sh         # --virtual でのパイプライン処理
│   └── stream_operations.sh        # ストリーム処理での制限確認
```

#### Test Category: llmsh Virtual Security Enforcement
```bash
# Test Suite: tests/security/llmsh_virtual/
├── access_restrictions/
│   ├── real_file_denied.sh         # 実ファイルアクセス拒否確認
│   ├── system_command_limits.sh    # システムコマンド制限確認
│   ├── virtual_environment.sh      # VFS環境での隔離確認
│   └── llmcmd_equivalence.sh       # llmcmd と同等制限の確認
```

### 1.3 llmsh (非virtual) コマンドテスト

#### Test Category: llmsh Real Mode Operations
```bash
# Test Suite: tests/integration/llmsh_real/
├── top_level_access/
│   ├── real_file_access.sh         # トップレベルでの実ファイルアクセス
│   ├── system_integration.sh       # システムコマンドとの連携
│   ├── mixed_mode_operations.sh    # 実ファイル + llmcmd内部コマンド
│   └── complex_pipelines.sh        # 複雑なパイプライン処理
├── internal_command_restrictions/
│   ├── llmcmd_via_llmsh.sh         # llmsh経由llmcmdでの制限確認
│   ├── nested_execution.sh         # ネストした実行での制限確認
│   └── context_switching.sh        # コンテキスト切り替えでの制限確認
```

## 2. 内部コマンド別単体テスト

### 2.1 FSProxy Protocol Commands

#### Test Category: Core Protocol Commands
```bash
# Test Suite: tests/unit/fsproxy_protocol/
├── open_command/
│   ├── open_valid_modes.sh         # 全モード(r,w,a,r+,w+,a+)の動作確認
│   ├── open_invalid_modes.sh       # 無効モードでのエラー確認
│   ├── open_nonexistent_file.sh    # 存在しないファイルでの動作
│   ├── open_permission_denied.sh   # 権限エラーでの処理
│   └── open_concurrent_access.sh   # 同時アクセスでの動作
├── read_command/
│   ├── read_valid_sizes.sh         # 各種サイズでの読み取り
│   ├── read_zero_size.sh           # サイズ0での読み取り
│   ├── read_eof_handling.sh        # EOF処理の確認
│   ├── read_invalid_fd.sh          # 無効fdでのエラー確認
│   └── read_binary_data.sh         # バイナリデータの読み取り
├── write_command/
│   ├── write_text_data.sh          # テキストデータ書き込み
│   ├── write_binary_data.sh        # バイナリデータ書き込み
│   ├── write_large_data.sh         # 大容量データ書き込み
│   ├── write_invalid_fd.sh         # 無効fdでのエラー確認
│   └── write_readonly_file.sh      # 読み取り専用ファイルへの書き込み
├── close_command/
│   ├── close_valid_fd.sh           # 正常ファイルのクローズ
│   ├── close_invalid_fd.sh         # 無効fdでのクローズ
│   ├── close_already_closed.sh     # 二重クローズの処理
│   └── close_resource_cleanup.sh   # リソースクリーンアップ確認
```

#### Test Category: LLM Integration Commands
```bash
# Test Suite: tests/unit/llm_integration/
├── llm_chat_command/
│   ├── chat_top_level_execution.sh # トップレベル実行での動作
│   ├── chat_child_execution.sh     # 子プロセス実行での動作
│   ├── chat_with_input_files.sh    # 入力ファイル付き実行
│   ├── chat_pipeline_support.sh    # パイプライン対応確認
│   ├── chat_quota_management.sh    # クォータ管理確認
│   ├── chat_api_error_handling.sh  # API エラー処理
│   └── chat_response_parsing.sh    # レスポンス解析確認
├── llm_quota_command/
│   ├── quota_in_llm_context.sh     # LLMコンテキストでのアクセス
│   ├── quota_access_denied.sh      # 非LLMコンテキストでの拒否
│   ├── quota_information_format.sh # クォータ情報フォーマット確認
│   └── quota_real_time_update.sh   # リアルタイム更新確認
```

### 2.2 Resource Management

#### Test Category: Client and Process Management
```bash
# Test Suite: tests/unit/resource_management/
├── client_management/
│   ├── client_registration.sh      # クライアント登録処理
│   ├── client_file_tracking.sh     # ファイル関連付け追跡
│   ├── client_cleanup.sh           # クライアントクリーンアップ
│   └── client_llm_context.sh       # LLMコンテキスト管理
├── process_monitoring/
│   ├── process_lifecycle.sh        # プロセスライフサイクル管理
│   ├── abnormal_termination.sh     # 異常終了検出
│   ├── resource_cleanup.sh         # リソース自動クリーンアップ
│   └── monitoring_performance.sh   # 監視パフォーマンス確認
├── pipe_eof_handling/
│   ├── eof_detection.sh            # EOF検出処理
│   ├── automatic_cleanup.sh        # 自動クリーンアップ実行
│   ├── orphaned_resources.sh       # 孤立リソース処理
│   └── multiple_clients.sh         # 複数クライアント処理
```

## 3. パイプ・リダイレクトテスト

### 3.1 Pipeline Operations

#### Test Category: Basic Pipeline Support
```bash
# Test Suite: tests/integration/pipelines/
├── simple_pipes/
│   ├── text_processing_pipe.sh     # cat file.txt | llmcmd "summarize"
│   ├── data_transformation.sh      # grep pattern | llmcmd "analyze"
│   ├── multi_stage_pipeline.sh     # cmd1 | llmcmd | cmd2
│   └── bidirectional_pipe.sh       # 双方向パイプ処理
├── complex_pipes/
│   ├── branched_pipeline.sh        # 分岐パイプライン
│   ├── conditional_pipeline.sh     # 条件付きパイプライン
│   ├── parallel_processing.sh      # 並列パイプ処理
│   └── error_propagation.sh        # エラー伝播確認
```

#### Test Category: Stream Redirection
```bash
# Test Suite: tests/integration/redirection/
├── input_redirection/
│   ├── stdin_from_file.sh          # llmcmd < input.txt
│   ├── stdin_from_pipe.sh          # cmd | llmcmd
│   └── stdin_combined.sh           # 複合入力処理
├── output_redirection/
│   ├── stdout_to_file.sh           # llmcmd > output.txt
│   ├── stderr_handling.sh          # llmcmd 2> error.log
│   ├── combined_output.sh          # llmcmd > out.txt 2> err.txt
│   └── append_redirection.sh       # llmcmd >> output.txt
```

### 3.2 Advanced Stream Operations

#### Test Category: VFS Stream Integration
```bash
# Test Suite: tests/integration/vfs_streams/
├── fd_mapping/
│   ├── stdin_fd_mapping.sh         # stdin FDマッピング確認
│   ├── stdout_fd_mapping.sh        # stdout FDマッピング確認
│   ├── stderr_fd_mapping.sh        # stderr FDマッピング確認
│   └── custom_fd_mapping.sh        # カスタムFDマッピング確認
├── stream_security/
│   ├── fd_access_control.sh        # FDアクセス制御確認
│   ├── stream_isolation.sh         # ストリーム分離確認
│   └── unauthorized_fd_access.sh   # 未承認FDアクセス拒否
```

## 4. シナリオ的テスト

### 4.1 Real-World Usage Scenarios

#### Test Category: Typical User Workflows
```bash
# Test Suite: tests/scenarios/workflows/
├── data_analysis/
│   ├── csv_analysis_workflow.sh    # CSVデータ分析ワークフロー
│   ├── log_analysis_workflow.sh    # ログ分析ワークフロー
│   ├── report_generation.sh        # レポート生成ワークフロー
│   └── batch_processing.sh         # バッチ処理ワークフロー
├── content_creation/
│   ├── document_editing.sh         # 文書編集ワークフロー
│   ├── code_generation.sh          # コード生成ワークフロー
│   ├── translation_workflow.sh     # 翻訳ワークフロー
│   └── content_summarization.sh    # コンテンツ要約ワークフロー
├── development_workflows/
│   ├── code_review_workflow.sh     # コードレビューワークフロー
│   ├── testing_workflow.sh         # テストワークフロー
│   ├── documentation_gen.sh        # ドキュメント生成ワークフロー
│   └── deployment_assistance.sh    # デプロイ支援ワークフロー
```

#### Test Category: Error Recovery Scenarios
```bash
# Test Suite: tests/scenarios/error_recovery/
├── network_errors/
│   ├── api_timeout_recovery.sh     # APIタイムアウト回復
│   ├── connection_lost.sh          # 接続断エラー回復
│   ├── quota_exceeded.sh           # クォータ超過回復
│   └── rate_limit_handling.sh      # レート制限処理
├── system_errors/
│   ├── disk_space_full.sh          # ディスク容量不足
│   ├── permission_denied.sh        # 権限エラー処理
│   ├── memory_pressure.sh          # メモリ不足処理
│   └── process_limits.sh           # プロセス制限処理
├── user_errors/
│   ├── invalid_input_handling.sh   # 無効入力処理
│   ├── malformed_command.sh        # 不正コマンド処理
│   ├── configuration_errors.sh     # 設定エラー処理
│   └── file_not_found.sh           # ファイル不存在処理
```

### 4.2 Performance and Load Testing

#### Test Category: Scalability Tests
```bash
# Test Suite: tests/scenarios/performance/
├── high_volume/
│   ├── large_file_processing.sh    # 大容量ファイル処理
│   ├── many_small_files.sh         # 多数小ファイル処理
│   ├── concurrent_requests.sh      # 同時リクエスト処理
│   └── long_running_processes.sh   # 長時間実行プロセス
├── memory_usage/
│   ├── memory_leak_detection.sh    # メモリリーク検出
│   ├── resource_cleanup_test.sh    # リソースクリーンアップテスト
│   ├── client_limit_test.sh        # クライアント数制限テスト
│   └── fd_exhaustion_test.sh       # FD枯渇テスト
├── stress_testing/
│   ├── rapid_connect_disconnect.sh # 高速接続切断テスト
│   ├── process_spawn_stress.sh     # プロセス生成ストレステスト
│   ├── api_call_stress.sh          # API呼び出しストレステスト
│   └── concurrent_llm_requests.sh  # 同時LLMリクエストテスト
```

### 4.3 Security Penetration Testing

#### Test Category: Security Validation
```bash
# Test Suite: tests/scenarios/security/
├── access_control_bypass/
│   ├── path_traversal_attempts.sh  # パストラバーサル攻撃試行
│   ├── symlink_escape_attempts.sh  # シンボリックリンク脱出試行
│   ├── privilege_escalation.sh     # 権限昇格試行
│   └── sandbox_escape_attempts.sh  # サンドボックス脱出試行
├── input_validation/
│   ├── malicious_filenames.sh      # 悪意のあるファイル名
│   ├── buffer_overflow_attempts.sh # バッファオーバーフロー試行
│   ├── code_injection_attempts.sh  # コードインジェクション試行
│   └── command_injection.sh        # コマンドインジェクション試行
├── resource_exhaustion/
│   ├── fd_exhaustion_attack.sh     # FD枯渇攻撃
│   ├── memory_exhaustion.sh        # メモリ枯渇攻撃
│   ├── process_bomb.sh             # プロセスボム攻撃
│   └── disk_space_attack.sh        # ディスク容量攻撃
```

## 5. テスト実行フレームワーク

### 5.1 Test Infrastructure

#### Test Runner Implementation
```bash
# tests/framework/fsproxy_test_runner.sh

#!/bin/bash

# FSProxy Protocol Test Runner
# Comprehensive test execution with categorized reporting

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$TEST_ROOT/results/$(date +%Y%m%d_%H%M%S)"

# Test Categories
BINARY_TESTS=(
    "integration/llmcmd"
    "integration/llmsh_virtual" 
    "integration/llmsh_real"
)

UNIT_TESTS=(
    "unit/fsproxy_protocol"
    "unit/llm_integration"
    "unit/resource_management"
)

PIPELINE_TESTS=(
    "integration/pipelines"
    "integration/redirection"
    "integration/vfs_streams"
)

SCENARIO_TESTS=(
    "scenarios/workflows"
    "scenarios/error_recovery"
    "scenarios/performance"
    "scenarios/security"
)

# Test execution functions
run_category_tests() {
    local category="$1"
    shift
    local test_dirs=("$@")
    
    echo "======================================="
    echo "Running $category Tests"
    echo "======================================="
    
    for test_dir in "${test_dirs[@]}"; do
        run_test_directory "$TEST_ROOT/$test_dir"
    done
}

# Main execution
main() {
    mkdir -p "$RESULTS_DIR"
    
    echo "FSProxy Protocol Comprehensive Test Suite"
    echo "Results will be saved to: $RESULTS_DIR"
    echo ""
    
    # 1. Binary execution tests
    run_category_tests "BINARY" "${BINARY_TESTS[@]}"
    
    # 2. Unit tests
    run_category_tests "UNIT" "${UNIT_TESTS[@]}"
    
    # 3. Pipeline tests  
    run_category_tests "PIPELINE" "${PIPELINE_TESTS[@]}"
    
    # 4. Scenario tests
    run_category_tests "SCENARIO" "${SCENARIO_TESTS[@]}"
    
    # Generate final report
    generate_summary_report
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
```

#### Test Helpers and Utilities
```bash
# tests/framework/fsproxy_helpers.sh

# FSProxy specific test utilities

# Setup test environment with proper security configuration
setup_fsproxy_test_env() {
    local test_name="$1"
    local execution_mode="$2"  # llmcmd|llmsh-virtual|llmsh-real
    
    export TEST_ENV_DIR="/tmp/fsproxy_test_$$_$test_name"
    mkdir -p "$TEST_ENV_DIR"/{input,output,temp}
    
    # Configure based on execution mode
    case "$execution_mode" in
        "llmcmd")
            export FSPROXY_MODE="llmcmd"
            export FSPROXY_VIRTUAL="false"
            ;;
        "llmsh-virtual")
            export FSPROXY_MODE="llmsh-virtual"
            export FSPROXY_VIRTUAL="true"
            ;;
        "llmsh-real")
            export FSPROXY_MODE="llmsh-real"
            export FSPROXY_VIRTUAL="false"
            ;;
    esac
}

# Verify security policy enforcement
verify_access_control() {
    local file="$1"
    local mode="$2"
    local should_succeed="$3"
    
    # Test file access and verify result matches expectation
    if [[ "$should_succeed" == "true" ]]; then
        assert_file_accessible "$file" "$mode"
    else
        assert_access_denied "$file" "$mode"
    fi
}

# Monitor resource usage during test
monitor_resources() {
    local test_pid="$1"
    local monitor_file="$2"
    
    while kill -0 "$test_pid" 2>/dev/null; do
        echo "$(date): $(ps -o pid,ppid,rss,vsz,pcpu,comm -p "$test_pid")" >> "$monitor_file"
        sleep 1
    done
}
```

### 5.2 Continuous Integration Integration

#### Test Configuration
```yaml
# .github/workflows/fsproxy_tests.yml

name: FSProxy Protocol Tests

on:
  push:
    branches: [ main, feature/tool-layer-clean ]
  pull_request:
    branches: [ main ]

jobs:
  fsproxy-tests:
    runs-on: ubuntu-latest
    
    strategy:
      matrix:
        test-category: 
          - binary
          - unit  
          - pipeline
          - scenario
        
    steps:
    - uses: actions/checkout@v3
    
    - name: Setup Go
      uses: actions/setup-go@v3
      with:
        go-version: 1.21
    
    - name: Build binaries
      run: |
        go build -o bin/llmcmd ./cmd/llmcmd
        go build -o bin/llmsh ./cmd/llmsh
    
    - name: Setup test environment
      run: |
        chmod +x tests/framework/fsproxy_test_runner.sh
        mkdir -p tests/results
    
    - name: Run FSProxy tests
      run: |
        tests/framework/fsproxy_test_runner.sh ${{ matrix.test-category }}
    
    - name: Upload test results
      uses: actions/upload-artifact@v3
      with:
        name: fsproxy-test-results-${{ matrix.test-category }}
        path: tests/results/
```

## 6. テスト実行計画

### 6.1 Phase-based Execution

#### Phase 1: Core Functionality (Week 1)
1. **Binary Tests**: 基本的なllmcmd/llmsh実行テスト
2. **Unit Tests**: プロトコルコマンド単体テスト
3. **Basic Security**: 基本的なアクセス制御テスト

#### Phase 2: Integration Testing (Week 2)  
1. **Pipeline Tests**: パイプライン・リダイレクトテスト
2. **Resource Management**: リソース管理機能テスト
3. **LLM Integration**: LLM機能統合テスト

#### Phase 3: Advanced Testing (Week 3)
1. **Scenario Tests**: 実用的なワークフローテスト
2. **Performance Tests**: パフォーマンス・負荷テスト
3. **Security Tests**: セキュリティ侵入テスト

#### Phase 4: Production Readiness (Week 4)
1. **Regression Tests**: 回帰テスト実行
2. **Documentation**: テスト結果ドキュメント化
3. **CI/CD Integration**: 継続的テスト環境構築

### 6.2 Success Criteria

#### Acceptance Criteria
- **Functional**: 全プロトコルコマンドが仕様通り動作
- **Security**: セキュリティ要件に100%準拠
- **Performance**: 負荷テストで安定動作
- **Reliability**: 24時間連続実行で異常なし
- **Recovery**: エラー状況からの適切な回復

#### Quality Gates
- **Unit Test Coverage**: >95%
- **Integration Test Pass Rate**: 100%
- **Performance Benchmarks**: 仕様内
- **Security Tests**: 全侵入試行を防御
- **Resource Leaks**: ゼロ検出

この包括的なテスト計画により、fsproxy protocol実装の品質と信頼性を確保し、プロダクション環境での安全な運用を保証します。
