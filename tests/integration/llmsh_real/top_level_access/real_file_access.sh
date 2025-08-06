#!/bin/bash

# Test: llmsh real mode file access functionality
# Validates llmsh (without --virtual) real file system access

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_NAME="real_file_access"
source "$SCRIPT_DIR/../../../framework/fsproxy_helpers.sh"

main() {
    echo "=== Testing llmsh real mode file access ==="
    
    # Setup test environment
    setup_fsproxy_test_env "$TEST_NAME" "llmsh-real"
    
    # Check if llmsh binary exists
    local llmsh_binary="$SCRIPT_DIR/../../../../bin/llmsh"
    check_binary "$llmsh_binary"
    
    # Create test files in the test environment
    local real_input="$TEST_ENV_DIR/input/real_test.txt"
    local real_output="$TEST_ENV_DIR/output/real_result.txt"
    
    cat > "$real_input" << 'EOF'
Real mode test data:
- This file should be accessible in real mode
- Testing real filesystem integration
- Full system access capabilities
EOF
    
    # Test real mode basic file access
    echo "Testing real mode basic file access..."
    local result
    result=$("$llmsh_binary" -i "$real_input" "cat \$1" 2>&1) || {
        echo "ERROR: llmsh real mode failed with input file"
        cleanup_test_env
        exit 1
    }
    
    if ! echo "$result" | grep -q "Real mode test data"; then
        echo "ERROR: Real mode input file not properly processed"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ llmsh real mode successfully accessed input file"
    
    # Test real mode output file creation
    echo "Testing real mode output file creation..."
    "$llmsh_binary" -o "$real_output" "echo 'Real mode output test' > \$1" 2>&1 || {
        echo "ERROR: llmsh real mode failed with output file"
        cleanup_test_env
        exit 1
    }
    
    if [[ ! -f "$real_output" ]]; then
        echo "ERROR: Real mode output file was not created"
        cleanup_test_env
        exit 1
    fi
    
    local output_content
    output_content=$(cat "$real_output")
    if ! echo "$output_content" | grep -q "Real mode output test"; then
        echo "ERROR: Real mode output file content incorrect"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ llmsh real mode successfully created output file"
    
    # Test real mode system file access (should work in real mode)
    echo "Testing real mode system file access..."
    
    # Test reading from a safe system file
    result=$("$llmsh_binary" "cat /proc/version" 2>&1) || {
        echo "ERROR: llmsh real mode failed to access /proc/version"
        cleanup_test_env
        exit 1
    }
    
    if ! echo "$result" | grep -q "Linux"; then
        echo "ERROR: /proc/version content not as expected"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ llmsh real mode can access system files"
    
    # Test real mode with system commands
    echo "Testing real mode system command execution..."
    
    # Test basic system commands
    result=$("$llmsh_binary" "echo 'System command test'" 2>&1) || {
        echo "ERROR: llmsh real mode failed to execute echo command"
        cleanup_test_env
        exit 1
    }
    
    if ! echo "$result" | grep -q "System command test"; then
        echo "ERROR: System command output incorrect"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ llmsh real mode can execute system commands"
    
    # Test real mode file operations in current directory
    echo "Testing real mode file operations in current directory..."
    
    local temp_file="$TEST_ENV_DIR/temp_real_test.txt"
    "$llmsh_binary" "echo 'Temporary file content' > '$temp_file'" 2>&1 || {
        echo "ERROR: llmsh real mode failed to create temporary file"
        cleanup_test_env
        exit 1
    }
    
    if [[ ! -f "$temp_file" ]]; then
        echo "ERROR: Temporary file was not created"
        cleanup_test_env
        exit 1
    fi
    
    # Read the temporary file
    result=$("$llmsh_binary" "cat '$temp_file'" 2>&1) || {
        echo "ERROR: llmsh real mode failed to read temporary file"
        cleanup_test_env
        exit 1
    }
    
    if ! echo "$result" | grep -q "Temporary file content"; then
        echo "ERROR: Temporary file content incorrect"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ llmsh real mode can perform file operations"
    
    # Test real mode with pipes and redirections
    echo "Testing real mode pipes and redirections..."
    
    local pipe_result="$TEST_ENV_DIR/output/pipe_test.txt"
    "$llmsh_binary" "echo 'pipe test data' | grep 'test' > '$pipe_result'" 2>&1 || {
        echo "ERROR: llmsh real mode failed with pipes and redirections"
        cleanup_test_env
        exit 1
    }
    
    if [[ ! -f "$pipe_result" ]]; then
        echo "ERROR: Pipe result file was not created"
        cleanup_test_env
        exit 1
    fi
    
    if ! grep -q "pipe test data" "$pipe_result"; then
        echo "ERROR: Pipe operation result incorrect"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ llmsh real mode supports pipes and redirections"
    
    # Test real mode working directory operations
    echo "Testing real mode working directory operations..."
    
    # Create a subdirectory
    local subdir="$TEST_ENV_DIR/subdir"
    "$llmsh_binary" "mkdir -p '$subdir'" 2>&1 || {
        echo "ERROR: llmsh real mode failed to create directory"
        cleanup_test_env
        exit 1
    }
    
    if [[ ! -d "$subdir" ]]; then
        echo "ERROR: Subdirectory was not created"
        cleanup_test_env
        exit 1
    fi
    
    # Test directory listing
    result=$("$llmsh_binary" "ls '$TEST_ENV_DIR'" 2>&1) || {
        echo "ERROR: llmsh real mode failed to list directory"
        cleanup_test_env
        exit 1
    }
    
    if ! echo "$result" | grep -q "subdir"; then
        echo "ERROR: Directory listing does not show created subdirectory"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ llmsh real mode supports directory operations"
    
    # Test real mode environment variable access
    echo "Testing real mode environment variable access..."
    
    result=$("$llmsh_binary" "echo \"Current user: \$USER\"" 2>&1) || {
        echo "ERROR: llmsh real mode failed to access environment variables"
        cleanup_test_env
        exit 1
    }
    
    if ! echo "$result" | grep -q "Current user:"; then
        echo "ERROR: Environment variable access failed"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ llmsh real mode can access environment variables"
    
    # Test real mode with complex shell operations
    echo "Testing real mode complex shell operations..."
    
    local complex_result="$TEST_ENV_DIR/output/complex_test.txt"
    "$llmsh_binary" "for i in 1 2 3; do echo \"Line \$i\"; done > '$complex_result'" 2>&1 || {
        echo "ERROR: llmsh real mode failed with complex shell operations"
        cleanup_test_env
        exit 1
    }
    
    if [[ ! -f "$complex_result" ]]; then
        echo "ERROR: Complex operation result file was not created"
        cleanup_test_env
        exit 1
    fi
    
    local line_count
    line_count=$(wc -l < "$complex_result")
    if [[ "$line_count" -ne 3 ]]; then
        echo "ERROR: Complex operation produced incorrect number of lines"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ llmsh real mode supports complex shell operations"
    
    # Cleanup
    cleanup_test_env
    
    echo "=== real_file_access test PASSED ==="
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
