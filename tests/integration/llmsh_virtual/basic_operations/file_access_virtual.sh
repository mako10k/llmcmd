#!/bin/bash

# Test: llmsh --virtual file access functionality
# Validates --virtual -i/-o file access restrictions

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_NAME="file_access_virtual"
source "$SCRIPT_DIR/../../../framework/fsproxy_helpers.sh"

main() {
    echo "=== Testing llmsh --virtual file access ==="
    
    # Setup test environment
    setup_fsproxy_test_env "$TEST_NAME" "llmsh-virtual"
    
    # Check if llmsh binary exists
    local llmsh_binary="$SCRIPT_DIR/../../../../bin/llmsh"
    check_binary "$llmsh_binary"
    
    # Create test input file
    local input_file="$TEST_ENV_DIR/input/virtual_input.txt"
    cat > "$input_file" << 'EOF'
Virtual mode test data:
- This file should be accessible in virtual mode
- Content for LLM processing
- Testing virtual file system integration
EOF
    
    # Test virtual mode with input file
    echo "Testing virtual mode input file access..."
    local result
    result=$("$llmsh_binary" --virtual -i "$input_file" -c "cat \$1" 2>&1) || {
        echo "ERROR: llmsh --virtual failed with input file"
        cleanup_test_env
        exit 1
    }
    
    # Verify result contains expected content
    if ! echo "$result" | grep -q "Virtual mode test data"; then
        echo "ERROR: Virtual mode input file not properly processed"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ llmsh --virtual successfully accessed input file"
    echo "Output preview: ${result:0:100}..."
    
    # Test virtual mode with output file
    echo "Testing virtual mode output file access..."
    local output_file="$TEST_ENV_DIR/output/virtual_output.txt"
    
    "$llmsh_binary" --virtual -o "$output_file" -c "echo 'Virtual mode output test' > \$1" 2>&1 || {
        echo "ERROR: llmsh --virtual failed with output file"
        cleanup_test_env
        exit 1
    }
    
    # Verify output file was created
    if [[ ! -f "$output_file" ]]; then
        echo "ERROR: Virtual mode output file was not created"
        cleanup_test_env
        exit 1
    fi
    
    local output_content
    output_content=$(cat "$output_file")
    if ! echo "$output_content" | grep -q "Virtual mode output test"; then
        echo "ERROR: Virtual mode output file content incorrect"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ llmsh --virtual successfully created output file"
    
    # Test combined input/output in virtual mode
    echo "Testing virtual mode combined input/output..."
    local input_file2="$TEST_ENV_DIR/input/data.csv"
    local output_file2="$TEST_ENV_DIR/output/processed.txt"
    
    cat > "$input_file2" << 'EOF'
name,score
Alice,85
Bob,92
Charlie,78
EOF
    
    "$llmsh_binary" --virtual -i "$input_file2" -o "$output_file2" -c "grep -v '^name' \$1 | wc -l > \$2" 2>&1 || {
        echo "ERROR: llmsh --virtual failed with combined input/output"
        cleanup_test_env
        exit 1
    }
    
    # Verify processing worked correctly
    if [[ ! -f "$output_file2" ]]; then
        echo "ERROR: Combined mode output file was not created"
        cleanup_test_env
        exit 1
    fi
    
    local line_count
    line_count=$(cat "$output_file2" | tr -d '[:space:]')
    if [[ "$line_count" != "3" ]]; then
        echo "ERROR: CSV processing in virtual mode failed (expected 3, got $line_count)"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ llmsh --virtual combined input/output processing works"
    
    # Test that virtual mode restricts access to real filesystem
    echo "Testing virtual mode filesystem restrictions..."
    
    # Try to access /etc/passwd (should fail)
    if "$llmsh_binary" --virtual -c "cat /etc/passwd" >/dev/null 2>&1; then
        echo "ERROR: Virtual mode allowed access to /etc/passwd"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Virtual mode correctly restricts access to real filesystem"
    
    # Test that virtual mode restricts system commands
    echo "Testing virtual mode system command restrictions..."
    
    # Try to run system commands (should be limited)
    if "$llmsh_binary" --virtual -c "ps aux" >/dev/null 2>&1; then
        echo "⚠ Virtual mode allowed system command execution (may be expected)"
    else
        echo "✓ Virtual mode restricts system command execution"
    fi
    
    # Test stdin processing in virtual mode
    echo "Testing virtual mode stdin processing..."
    result=$(echo "Virtual stdin test data" | "$llmsh_binary" --virtual -c "cat" 2>&1) || {
        echo "ERROR: Virtual mode stdin processing failed"
        cleanup_test_env
        exit 1
    }
    
    if ! echo "$result" | grep -q "Virtual stdin test data"; then
        echo "ERROR: Virtual mode stdin not processed correctly"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Virtual mode stdin processing works"
    
    # Cleanup
    cleanup_test_env
    
    echo "=== file_access_virtual test PASSED ==="
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
