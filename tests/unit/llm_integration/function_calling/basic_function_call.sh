#!/bin/bash
# Test: Basic LLM function calling
# Purpose: Verify LLM can properly call available functions

TEST_NAME="Basic Function Calling"
TEST_TYPE="unit"
TIMEOUT=15

# Source test framework
source "$(dirname "$0")/../../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing basic LLM function calling"
    
    # Create test data for LLM to process
    cat > "$TEST_DATA/numbers.txt" << 'EOF'
10
20
30
40
50
EOF
    
    # Create instruction that should trigger function calls
    cat > "$TEST_DATA/instruction.txt" << 'EOF'
Please read the file numbers.txt, calculate the sum of all numbers, and write the result to output.txt with the format "Total: [sum]".
EOF
    
    # Execute with function calling expected
    if timeout 15 "$LLMCMD_BIN" \
        -i "$TEST_DATA/numbers.txt" \
        -i "$TEST_DATA/instruction.txt" \
        -o "$TEST_DATA/output.txt" \
        "$TEST_DATA/instruction.txt" > "$LOG_FILE" 2>&1; then
        
        # Check if the calculation was performed correctly
        if [ -f "$TEST_DATA/output.txt" ] && \
           (grep -q "Total: 150" "$TEST_DATA/output.txt" || grep -q "150" "$TEST_DATA/output.txt"); then
            log_success "LLM function calling executed successfully"
            return 0
        else
            log_error "Function calling didn't produce expected calculation result"
            cat "$TEST_DATA/output.txt" >> "$LOG_FILE" 2>/dev/null
            return 1
        fi
    else
        log_error "LLM function calling execution failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
