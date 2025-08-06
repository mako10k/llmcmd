#!/bin/bash
# Test: Basic read command functionality
# Purpose: Verify read command can read file content correctly

TEST_NAME="Basic Read Command"
TEST_TYPE="unit"
TIMEOUT=10

# Source test framework
source "$(dirname "$0")/../../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing basic read command functionality"
    
    # Create test file with known content
    cat > "$TEST_DATA/data.txt" << 'EOF'
Line 1: Hello World
Line 2: Test Content
Line 3: End of File
EOF
    
    # Create instruction to read the file
    cat > "$TEST_DATA/instruction.txt" << 'EOF'
Please read the contents of data.txt and show me exactly what you read.
EOF
    
    # Execute read operation
    if timeout 10 "$LLMCMD_BIN" \
        -i "$TEST_DATA/data.txt" \
        -i "$TEST_DATA/instruction.txt" \
        -o "$TEST_DATA/output.txt" \
        "$TEST_DATA/instruction.txt" > "$LOG_FILE" 2>&1; then
        
        # Check if the output contains the expected content
        if [ -s "$TEST_DATA/output.txt" ] && \
           grep -q "Hello World" "$TEST_DATA/output.txt" && \
           grep -q "Test Content" "$TEST_DATA/output.txt"; then
            log_success "Read command executed successfully"
            return 0
        else
            log_error "Output doesn't contain expected read content"
            return 1
        fi
    else
        log_error "Read command execution failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
