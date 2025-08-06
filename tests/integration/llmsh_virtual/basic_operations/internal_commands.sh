#!/bin/bash

# Test: llmsh --virtual internal commands execution
# Validates llmcmd internal commands work properly in virtual mode

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_NAME="internal_commands"
source "$SCRIPT_DIR/../../../framework/fsproxy_helpers.sh"

main() {
    echo "=== Testing llmsh --virtual internal commands ==="
    
    # Setup test environment
    setup_fsproxy_test_env "$TEST_NAME" "llmsh-virtual"
    
    # Check if llmsh binary exists
    local llmsh_binary="$SCRIPT_DIR/../../../../bin/llmsh"
    check_binary "$llmsh_binary"
    
    # Create test data files
    create_test_files "$TEST_ENV_DIR/input" "text"
    create_test_files "$TEST_ENV_DIR/input" "csv"
    
    # Test cat command in virtual mode
    echo "Testing cat command in virtual mode..."
    local input_file="$TEST_ENV_DIR/input/test.txt"
    local result
    result=$("$llmsh_binary" --virtual -i "$input_file" "cat \$1" 2>&1) || {
        echo "ERROR: cat command failed in virtual mode"
        cleanup_test_env
        exit 1
    }
    
    if ! echo "$result" | grep -q "This is a sample text file"; then
        echo "ERROR: cat command output incorrect in virtual mode"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ cat command works in virtual mode"
    
    # Test grep command in virtual mode
    echo "Testing grep command in virtual mode..."
    result=$("$llmsh_binary" --virtual -i "$input_file" "grep 'sample' \$1" 2>&1) || {
        echo "ERROR: grep command failed in virtual mode"
        cleanup_test_env
        exit 1
    }
    
    if ! echo "$result" | grep -q "sample"; then
        echo "ERROR: grep command output incorrect in virtual mode"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ grep command works in virtual mode"
    
    # Test wc command in virtual mode
    echo "Testing wc command in virtual mode..."
    result=$("$llmsh_binary" --virtual -i "$input_file" "wc -l \$1" 2>&1) || {
        echo "ERROR: wc command failed in virtual mode"
        cleanup_test_env
        exit 1
    }
    
    if ! echo "$result" | grep -q "3"; then
        echo "ERROR: wc command output incorrect in virtual mode (expected 3 lines)"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ wc command works in virtual mode"
    
    # Test sed command in virtual mode
    echo "Testing sed command in virtual mode..."
    local output_file="$TEST_ENV_DIR/output/sed_result.txt"
    "$llmsh_binary" --virtual -i "$input_file" -o "$output_file" "sed 's/sample/example/g' \$1 > \$2" 2>&1 || {
        echo "ERROR: sed command failed in virtual mode"
        cleanup_test_env
        exit 1
    }
    
    if [[ ! -f "$output_file" ]]; then
        echo "ERROR: sed command output file not created"
        cleanup_test_env
        exit 1
    fi
    
    if ! grep -q "example" "$output_file"; then
        echo "ERROR: sed command substitution failed"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ sed command works in virtual mode"
    
    # Test sort command in virtual mode
    echo "Testing sort command in virtual mode..."
    local csv_file="$TEST_ENV_DIR/input/test.csv"
    local sorted_file="$TEST_ENV_DIR/output/sorted.txt"
    "$llmsh_binary" --virtual -i "$csv_file" -o "$sorted_file" "sort \$1 > \$2" 2>&1 || {
        echo "ERROR: sort command failed in virtual mode"
        cleanup_test_env
        exit 1
    }
    
    if [[ ! -f "$sorted_file" ]]; then
        echo "ERROR: sort command output file not created"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ sort command works in virtual mode"
    
    # Test head command in virtual mode
    echo "Testing head command in virtual mode..."
    result=$("$llmsh_binary" --virtual -i "$csv_file" "head -2 \$1" 2>&1) || {
        echo "ERROR: head command failed in virtual mode"
        cleanup_test_env
        exit 1
    }
    
    local line_count
    line_count=$(echo "$result" | wc -l)
    if [[ "$line_count" -ne 2 ]]; then
        echo "ERROR: head command returned incorrect number of lines"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ head command works in virtual mode"
    
    # Test tail command in virtual mode
    echo "Testing tail command in virtual mode..."
    result=$("$llmsh_binary" --virtual -i "$csv_file" "tail -2 \$1" 2>&1) || {
        echo "ERROR: tail command failed in virtual mode"
        cleanup_test_env
        exit 1
    }
    
    line_count=$(echo "$result" | wc -l)
    if [[ "$line_count" -ne 2 ]]; then
        echo "ERROR: tail command returned incorrect number of lines"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ tail command works in virtual mode"
    
    # Test tr command in virtual mode
    echo "Testing tr command in virtual mode..."
    local tr_file="$TEST_ENV_DIR/output/tr_result.txt"
    "$llmsh_binary" --virtual -i "$input_file" -o "$tr_file" "tr 'a-z' 'A-Z' < \$1 > \$2" 2>&1 || {
        echo "ERROR: tr command failed in virtual mode"
        cleanup_test_env
        exit 1
    }
    
    if [[ ! -f "$tr_file" ]]; then
        echo "ERROR: tr command output file not created"
        cleanup_test_env
        exit 1
    fi
    
    if ! grep -q "THIS IS A SAMPLE" "$tr_file"; then
        echo "ERROR: tr command case conversion failed"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ tr command works in virtual mode"
    
    # Test command chaining in virtual mode
    echo "Testing command chaining in virtual mode..."
    local chain_file="$TEST_ENV_DIR/output/chain_result.txt"
    "$llmsh_binary" --virtual -i "$input_file" -o "$chain_file" "cat \$1 | grep 'sample' | wc -l > \$2" 2>&1 || {
        echo "ERROR: command chaining failed in virtual mode"
        cleanup_test_env
        exit 1
    }
    
    if [[ ! -f "$chain_file" ]]; then
        echo "ERROR: command chaining output file not created"
        cleanup_test_env
        exit 1
    fi
    
    local grep_count
    grep_count=$(cat "$chain_file" | tr -d '[:space:]')
    if [[ "$grep_count" != "1" ]]; then
        echo "ERROR: command chaining produced incorrect result"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ command chaining works in virtual mode"
    
    # Cleanup
    cleanup_test_env
    
    echo "=== internal_commands test PASSED ==="
}

cleanup_test_env() {
    if [[ -n "${TEST_ENV_DIR:-}" && -d "$TEST_ENV_DIR" ]]; then
        rm -rf "$TEST_ENV_DIR"
    fi
}

# Trap cleanup on exit
trap cleanup_test_env EXIT

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
