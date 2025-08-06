#!/bin/bash
# Test: Valid open command with read-only permission
# Purpose: Verify basic open command functionality for reading files

TEST_NAME="Valid Open Read"
TEST_TYPE="unit"
TIMEOUT=10

# Source test framework
source "$(dirname "$0")/../../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing valid open command with read permission"
    
    # Create test file
    echo "test content" > "$TEST_DATA/input.txt"
    
    # Test open command with llmcmd
    cat > "$TEST_DATA/test_prompt.txt" << 'EOF'
Please read the file input.txt and tell me what it contains.
EOF
    
    # Execute with file access
    if timeout 10 "$LLMCMD_BIN" \
        -i "$TEST_DATA/input.txt" \
        -i "$TEST_DATA/test_prompt.txt" \
        -o "$TEST_DATA/output.txt" \
        "$TEST_DATA/test_prompt.txt" > "$LOG_FILE" 2>&1; then
        
        # Check if file was actually opened (output should reference the content)
        if [ -s "$TEST_DATA/output.txt" ]; then
            log_success "Open command executed successfully"
            return 0
        else
            log_error "No output generated - open may have failed"
            return 1
        fi
    else
        log_error "llmcmd execution failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
