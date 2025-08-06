#!/bin/bash
# Test: Multi-file workflow integration
# Purpose: Verify complete workflow across multiple input/output files

TEST_NAME="Multi-file Workflow"
TEST_TYPE="pipeline"
TIMEOUT=35

# Source test framework
source "$(dirname "$0")/../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing multi-file workflow integration"
    
    # Create multiple source files
    cat > "$TEST_DATA/config.txt" << 'EOF'
max_connections=100
timeout=30
retry_count=3
log_level=INFO
EOF
    
    cat > "$TEST_DATA/users.txt" << 'EOF'
admin,active,2023-01-15
user1,active,2023-03-20
user2,inactive,2023-02-10
user3,active,2023-04-05
user4,suspended,2023-01-30
EOF
    
    cat > "$TEST_DATA/logs.txt" << 'EOF'
2023-04-10 10:00:00 INFO System startup
2023-04-10 10:01:00 INFO User admin logged in
2023-04-10 10:05:00 WARN Connection timeout for user2
2023-04-10 10:10:00 ERROR Failed authentication for user5
2023-04-10 10:15:00 INFO User user1 logged in
2023-04-10 10:20:00 ERROR Database connection failed
EOF
    
    # Create workflow instruction
    cat > "$TEST_DATA/instruction.txt" << 'EOF'
Please execute this complete workflow:

1. Read config.txt and extract the retry_count value
2. Read users.txt and count how many users are "active"
3. Read logs.txt and count ERROR entries
4. Generate a status report in status_report.txt with:
   - System configuration summary (retry count from config)
   - User activity summary (active user count)
   - System health summary (error count from logs)
   - Overall status assessment (Good/Warning/Critical based on error count)
5. Create a detailed log in workflow.log with:
   - Each processing step and its result
   - Timestamp for workflow completion
   - Success/failure status for each component

Complete this entire workflow in sequence and provide comprehensive reporting.
EOF
    
    # Execute workflow
    if timeout 35 "$LLMCMD_BIN" \
        -i "$TEST_DATA/config.txt" \
        -i "$TEST_DATA/users.txt" \
        -i "$TEST_DATA/logs.txt" \
        -i "$TEST_DATA/instruction.txt" \
        -o "$TEST_DATA/status_report.txt" \
        -o "$TEST_DATA/workflow.log" \
        "$TEST_DATA/instruction.txt" > "$LOG_FILE" 2>&1; then
        
        # Verify workflow completion
        if [ -f "$TEST_DATA/status_report.txt" ] && [ -f "$TEST_DATA/workflow.log" ]; then
            
            # Check status report content
            if grep -q -i "retry.*3\|configuration" "$TEST_DATA/status_report.txt" && \
               grep -q -i "active.*user\|user.*active" "$TEST_DATA/status_report.txt" && \
               grep -q -i "error\|health" "$TEST_DATA/status_report.txt"; then
                
                # Check workflow log content
                if grep -q -i "workflow\|processing\|step" "$TEST_DATA/workflow.log"; then
                    log_success "Multi-file workflow completed successfully"
                    return 0
                else
                    log_warning "Workflow completed but logging may be incomplete"
                    return 1
                fi
            else
                log_error "Status report missing expected content"
                cat "$TEST_DATA/status_report.txt" >> "$LOG_FILE" 2>/dev/null
                return 1
            fi
        else
            log_error "Workflow output files not created"
            return 1
        fi
    else
        log_error "Multi-file workflow failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
