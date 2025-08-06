#!/bin/bash
# Test: Invalid open command - unauthorized file
# Purpose: Verify security restrictions for file access

TEST_NAME="Invalid Open Unauthorized"
TEST_TYPE="unit"
TIMEOUT=10

# Source test framework
source "$(dirname "$0")/../../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing open command with unauthorized file access"
    
    # Create test file outside of allowed access
    echo "secret content" > "/tmp/secret.txt"
    
    # Create instruction that tries to read unauthorized file
    cat > "$TEST_DATA/instruction.txt" << 'EOF'
Please read the file /tmp/secret.txt and tell me what it contains.
EOF
    
    # Execute - should fail due to security restrictions
    if timeout 10 "$LLMCMD_BIN" \
        -i "$TEST_DATA/instruction.txt" \
        -o "$TEST_DATA/output.txt" \
        "$TEST_DATA/instruction.txt" > "$LOG_FILE" 2>&1; then
        
        # Check if the operation was properly denied
        if grep -q -i "access denied\|permission denied\|unauthorized\|not allowed" "$LOG_FILE" || \
           grep -q -i "access denied\|permission denied\|unauthorized\|not allowed" "$TEST_DATA/output.txt"; then
            log_success "Unauthorized access properly denied"
            return 0
        else
            log_error "Security restriction not enforced - unauthorized access may have succeeded"
            return 1
        fi
    else
        # Failure is expected for security violation
        if grep -q -i "access denied\|permission denied\|unauthorized\|not allowed" "$LOG_FILE"; then
            log_success "Unauthorized access properly denied"
            return 0
        else
            log_error "Command failed but not due to expected security restriction"
            return 1
        fi
    fi
}

# Execute test
main_test_wrapper "$@"
