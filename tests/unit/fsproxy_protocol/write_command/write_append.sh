#!/bin/bash
# Test: Write command with append functionality
# Purpose: Verify write command can append to existing files

TEST_NAME="Write Append"
TEST_TYPE="unit"
TIMEOUT=10

# Source test framework
source "$(dirname "$0")/../../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing write command with append functionality"
    
    # Create initial file content
    echo "Initial content" > "$TEST_DATA/append_test.txt"
    
    # Create instruction to append content
    cat > "$TEST_DATA/instruction.txt" << 'EOF'
Please append the following line to the existing file append_test.txt:
"Appended content line"
EOF
    
    # Execute append operation
    if timeout 10 "$LLMCMD_BIN" \
        -i "$TEST_DATA/append_test.txt" \
        -i "$TEST_DATA/instruction.txt" \
        -o "$TEST_DATA/append_test.txt" \
        "$TEST_DATA/instruction.txt" > "$LOG_FILE" 2>&1; then
        
        # Check if both original and appended content exist
        if [ -f "$TEST_DATA/append_test.txt" ] && \
           grep -q "Initial content" "$TEST_DATA/append_test.txt" && \
           grep -q "Appended content" "$TEST_DATA/append_test.txt"; then
            log_success "Write append executed successfully"
            return 0
        else
            log_error "Append operation failed or content missing"
            return 1
        fi
    else
        log_error "Write append execution failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
