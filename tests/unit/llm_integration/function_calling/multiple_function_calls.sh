#!/bin/bash
# Test: Multiple function calls in sequence
# Purpose: Verify LLM can chain multiple function calls

TEST_NAME="Multiple Function Calls"
TEST_TYPE="unit"
TIMEOUT=20

# Source test framework
source "$(dirname "$0")/../../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing multiple function calls in sequence"
    
    # Create source data
    cat > "$TEST_DATA/data.txt" << 'EOF'
apple,5,2.50
banana,10,1.20
orange,3,3.00
grape,8,4.50
EOF
    
    # Create complex instruction requiring multiple operations
    cat > "$TEST_DATA/instruction.txt" << 'EOF'
Please:
1. Read the file data.txt (CSV format: item,quantity,price)
2. Calculate the total value for each item (quantity * price)
3. Find the item with the highest total value
4. Write the result to summary.txt with format "Highest value item: [item] with total [value]"
EOF
    
    # Execute complex operation
    if timeout 20 "$LLMCMD_BIN" \
        -i "$TEST_DATA/data.txt" \
        -i "$TEST_DATA/instruction.txt" \
        -o "$TEST_DATA/summary.txt" \
        "$TEST_DATA/instruction.txt" > "$LOG_FILE" 2>&1; then
        
        # Check if complex calculation was performed (grape: 8*4.50=36.00 should be highest)
        if [ -f "$TEST_DATA/summary.txt" ] && \
           (grep -q -i "grape" "$TEST_DATA/summary.txt" && grep -q "36" "$TEST_DATA/summary.txt"); then
            log_success "Multiple function calls executed successfully"
            return 0
        else
            log_error "Multiple function calls didn't produce expected result"
            cat "$TEST_DATA/summary.txt" >> "$LOG_FILE" 2>/dev/null
            return 1
        fi
    else
        log_error "Multiple function calls execution failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
