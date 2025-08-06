#!/bin/bash
# Test: Read command with large file
# Purpose: Verify read command can handle larger files appropriately

TEST_NAME="Read Large File"
TEST_TYPE="unit"
TIMEOUT=15

# Source test framework
source "$(dirname "$0")/../../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing read command with large file"
    
    # Create larger test file (within reasonable limits)
    for i in {1..100}; do
        echo "Line $i: This is test content for line number $i in our large file test."
    done > "$TEST_DATA/large_data.txt"
    
    # Create instruction to read the large file
    cat > "$TEST_DATA/instruction.txt" << 'EOF'
Please read the file large_data.txt and tell me how many lines it contains.
EOF
    
    # Execute read operation
    if timeout 15 "$LLMCMD_BIN" \
        -i "$TEST_DATA/large_data.txt" \
        -i "$TEST_DATA/instruction.txt" \
        -o "$TEST_DATA/output.txt" \
        "$TEST_DATA/instruction.txt" > "$LOG_FILE" 2>&1; then
        
        # Check if the output indicates successful reading
        if [ -s "$TEST_DATA/output.txt" ] && \
           (grep -q "100" "$TEST_DATA/output.txt" || grep -q "Line 1" "$TEST_DATA/output.txt"); then
            log_success "Large file read executed successfully"
            return 0
        else
            log_error "Output doesn't indicate successful large file reading"
            return 1
        fi
    else
        log_error "Large file read execution failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
