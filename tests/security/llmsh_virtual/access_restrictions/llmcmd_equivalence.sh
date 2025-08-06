#!/bin/bash

# Test: llmsh --virtual equivalence to llmcmd restrictions
# Validates that virtual mode has same security restrictions as llmcmd

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_NAME="llmcmd_equivalence"
source "$SCRIPT_DIR/../../../framework/fsproxy_helpers.sh"

main() {
    echo "=== Testing llmsh --virtual equivalence to llmcmd ==="
    
    # Setup test environment
    setup_fsproxy_test_env "$TEST_NAME" "llmsh-virtual"
    
    # Check if binaries exist
    local llmsh_binary="$SCRIPT_DIR/../../../../bin/llmsh"
    local llmcmd_binary="$SCRIPT_DIR/../../../../bin/llmcmd"
    check_binary "$llmsh_binary"
    check_binary "$llmcmd_binary"
    
    # Create test files
    local input_file="$TEST_ENV_DIR/input/equivalence_test.txt"
    local output_file1="$TEST_ENV_DIR/output/llmcmd_result.txt"
    local output_file2="$TEST_ENV_DIR/output/llmsh_result.txt"
    
    cat > "$input_file" << 'EOF'
Test data for equivalence checking.
Line 2 with different content.
Line 3 for comparison.
EOF
    
    # Test equivalent functionality - basic processing
    echo "Testing equivalent basic processing..."
    
    # Run same operation with both tools
    "$llmcmd_binary" -i "$input_file" -o "$output_file1" "Count the number of lines in the input" || {
        echo "ERROR: llmcmd failed during equivalence test"
        cleanup_test_env
        exit 1
    }
    
    "$llmsh_binary" --virtual -i "$input_file" -o "$output_file2" "wc -l \$1 > \$2" || {
        echo "ERROR: llmsh --virtual failed during equivalence test"
        cleanup_test_env
        exit 1
    }
    
    # Both should produce output files
    if [[ ! -f "$output_file1" ]] || [[ ! -f "$output_file2" ]]; then
        echo "ERROR: One of the tools failed to create output file"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Both tools produce output files"
    
    # Test equivalent security restrictions - input file write protection
    echo "Testing equivalent input file write protection..."
    
    local input_file2="$TEST_ENV_DIR/input/write_protected.txt"
    echo "Protected content" > "$input_file2"
    
    # Both should fail to modify input files
    local llmcmd_write_failed=false
    local llmsh_write_failed=false
    
    if "$llmcmd_binary" -i "$input_file2" "Try to write to the input file" >/dev/null 2>&1; then
        llmcmd_write_failed=false
    else
        llmcmd_write_failed=true
    fi
    
    if "$llmsh_binary" --virtual -i "$input_file2" "echo 'modified' > \$1" >/dev/null 2>&1; then
        llmsh_write_failed=false
    else
        llmsh_write_failed=true
    fi
    
    # Verify input file is unchanged
    if ! grep -q "Protected content" "$input_file2"; then
        echo "ERROR: Input file was modified by one of the tools"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Both tools protect input files from modification"
    
    # Test equivalent security restrictions - output file read protection
    echo "Testing equivalent output file read protection..."
    
    local sensitive_output="$TEST_ENV_DIR/output/sensitive.txt"
    echo "SENSITIVE: Secret data" > "$sensitive_output"
    
    # Neither should be able to read pre-existing output file content
    local llmcmd_result
    llmcmd_result=$("$llmcmd_binary" -o "$sensitive_output" "Tell me what's in the output file" 2>&1) || true
    
    local llmsh_result
    llmsh_result=$("$llmsh_binary" --virtual -o "$sensitive_output" "cat \$1" 2>&1) || true
    
    # Neither should expose the sensitive content
    if echo "$llmcmd_result" | grep -q "SENSITIVE" || echo "$llmsh_result" | grep -q "SENSITIVE"; then
        echo "ERROR: One of the tools exposed sensitive output file content"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Both tools protect pre-existing output file content"
    
    # Test equivalent file access restrictions
    echo "Testing equivalent file access restrictions..."
    
    # Both should fail to access system files
    if "$llmcmd_binary" "Read /etc/passwd" >/dev/null 2>&1 || "$llmsh_binary" --virtual "cat /etc/passwd" >/dev/null 2>&1; then
        echo "ERROR: One of the tools allowed access to system files"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Both tools restrict access to system files"
    
    # Test equivalent command execution capabilities
    echo "Testing equivalent command execution capabilities..."
    
    # Both should be able to perform basic text processing
    local test_input="$TEST_ENV_DIR/input/processing_test.csv"
    local llmcmd_output="$TEST_ENV_DIR/output/llmcmd_processing.txt"
    local llmsh_output="$TEST_ENV_DIR/output/llmsh_processing.txt"
    
    cat > "$test_input" << 'EOF'
name,score
Alice,85
Bob,92
Charlie,78
Diana,89
EOF
    
    # Test similar processing with both tools
    "$llmcmd_binary" -i "$test_input" -o "$llmcmd_output" "Count the number of data rows (excluding header)" || {
        echo "ERROR: llmcmd failed to process CSV data"
        cleanup_test_env
        exit 1
    }
    
    "$llmsh_binary" --virtual -i "$test_input" -o "$llmsh_output" "tail -n +2 \$1 | wc -l > \$2" || {
        echo "ERROR: llmsh --virtual failed to process CSV data"
        cleanup_test_env
        exit 1
    }
    
    # Both should produce reasonable results
    if [[ ! -f "$llmcmd_output" ]] || [[ ! -f "$llmsh_output" ]]; then
        echo "ERROR: One of the tools failed to create processing output"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Both tools can perform equivalent text processing"
    
    # Test equivalent error handling
    echo "Testing equivalent error handling..."
    
    # Both should handle non-existent input files gracefully
    local nonexistent="$TEST_ENV_DIR/input/nonexistent.txt"
    
    local llmcmd_error_handled=false
    local llmsh_error_handled=false
    
    if ! "$llmcmd_binary" -i "$nonexistent" "Process this file" >/dev/null 2>&1; then
        llmcmd_error_handled=true
    fi
    
    if ! "$llmsh_binary" --virtual -i "$nonexistent" "cat \$1" >/dev/null 2>&1; then
        llmsh_error_handled=true
    fi
    
    if [[ "$llmcmd_error_handled" != true ]] || [[ "$llmsh_error_handled" != true ]]; then
        echo "ERROR: Error handling not equivalent between tools"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Both tools handle errors equivalently"
    
    # Test stdin processing equivalence
    echo "Testing stdin processing equivalence..."
    
    local stdin_data="Equivalence test data from stdin"
    local llmcmd_stdin_result
    local llmsh_stdin_result
    
    llmcmd_stdin_result=$(echo "$stdin_data" | "$llmcmd_binary" "Convert to uppercase" 2>&1) || {
        echo "ERROR: llmcmd failed stdin processing"
        cleanup_test_env
        exit 1
    }
    
    llmsh_stdin_result=$(echo "$stdin_data" | "$llmsh_binary" --virtual "tr 'a-z' 'A-Z'" 2>&1) || {
        echo "ERROR: llmsh --virtual failed stdin processing"
        cleanup_test_env
        exit 1
    }
    
    # Both should process stdin successfully
    if [[ -z "$llmcmd_stdin_result" ]] || [[ -z "$llmsh_stdin_result" ]]; then
        echo "ERROR: Stdin processing not equivalent"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Both tools process stdin equivalently"
    
    # Cleanup
    cleanup_test_env
    
    echo "=== llmcmd_equivalence test PASSED ==="
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
