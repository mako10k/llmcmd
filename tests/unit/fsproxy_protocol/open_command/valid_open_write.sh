#!/bin/bash
# Test: Valid open command with write permission
# Purpose: Verify basic open command functionality for writing files

TEST_NAME="Valid Open Write"
TEST_TYPE="unit"
TIMEOUT=10

# Source test framework
source "$(dirname "$0")/../../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing valid open command with write permission"
    
    # Create test input
    cat > "$TEST_DATA/instruction.txt" << 'EOF'
Please create a file named output.txt and write "Hello World" to it.
EOF
    
    # Execute with write access
    if timeout 10 "$LLMCMD_BIN" \
        -i "$TEST_DATA/instruction.txt" \
        -o "$TEST_DATA/output.txt" \
        "$TEST_DATA/instruction.txt" > "$LOG_FILE" 2>&1; then
        
        # Check if file was created and written
        if [ -f "$TEST_DATA/output.txt" ] && grep -q "Hello World" "$TEST_DATA/output.txt"; then
            log_success "Open command with write executed successfully"
            return 0
        else
            log_error "Output file not created or doesn't contain expected content"
            return 1
        fi
    else
        log_error "llmcmd execution failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
