#!/bin/bash
# Test: LLM text processing with built-in commands
# Purpose: Verify LLM can use built-in text processing tools

TEST_NAME="Text Processing Commands"
TEST_TYPE="unit"
TIMEOUT=15

# Source test framework
source "$(dirname "$0")/../../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing LLM text processing with built-in commands"
    
    # Create test data for text processing
    cat > "$TEST_DATA/log_data.txt" << 'EOF'
ERROR: Failed to connect to database
INFO: User login successful
ERROR: Memory allocation failed
DEBUG: Processing request
ERROR: Network timeout
INFO: System startup complete
WARN: Disk space low
ERROR: Authentication failed
EOF
    
    # Create instruction for text processing
    cat > "$TEST_DATA/instruction.txt" << 'EOF'
Please read log_data.txt and:
1. Count how many ERROR lines there are
2. Extract all ERROR lines and save them to errors.txt
3. Write a summary to summary.txt with the format "Found [count] errors"
EOF
    
    # Execute text processing
    if timeout 15 "$LLMCMD_BIN" \
        -i "$TEST_DATA/log_data.txt" \
        -i "$TEST_DATA/instruction.txt" \
        -o "$TEST_DATA/errors.txt" \
        -o "$TEST_DATA/summary.txt" \
        "$TEST_DATA/instruction.txt" > "$LOG_FILE" 2>&1; then
        
        # Check if text processing was performed correctly (should find 4 errors)
        if [ -f "$TEST_DATA/summary.txt" ] && \
           (grep -q "Found 4 errors" "$TEST_DATA/summary.txt" || grep -q "4" "$TEST_DATA/summary.txt"); then
            log_success "Text processing commands executed successfully"
            return 0
        else
            log_error "Text processing didn't produce expected result"
            cat "$TEST_DATA/summary.txt" >> "$LOG_FILE" 2>/dev/null
            return 1
        fi
    else
        log_error "Text processing execution failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
