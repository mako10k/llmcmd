#!/bin/bash
# Test: Basic write command functionality
# Purpose: Verify write command can create and write to files correctly

TEST_NAME="Basic Write Command"
TEST_TYPE="unit"
TIMEOUT=10

# Source test framework
source "$(dirname "$0")/../../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing basic write command functionality"
    
    # Create instruction to write content
    cat > "$TEST_DATA/instruction.txt" << 'EOF'
Please create a file named result.txt and write the following content to it:
"Testing write functionality
Multiple lines supported
End of test content"
EOF
    
    # Execute write operation
    if timeout 10 "$LLMCMD_BIN" \
        -i "$TEST_DATA/instruction.txt" \
        -o "$TEST_DATA/result.txt" \
        "$TEST_DATA/instruction.txt" > "$LOG_FILE" 2>&1; then
        
        # Check if the file was created and contains expected content
        if [ -f "$TEST_DATA/result.txt" ] && \
           grep -q "Testing write functionality" "$TEST_DATA/result.txt" && \
           grep -q "Multiple lines supported" "$TEST_DATA/result.txt"; then
            log_success "Write command executed successfully"
            return 0
        else
            log_error "Output file not created or doesn't contain expected content"
            return 1
        fi
    else
        log_error "Write command execution failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
