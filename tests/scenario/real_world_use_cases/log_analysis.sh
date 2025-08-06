#!/bin/bash
# Test: Log analysis scenario
# Purpose: Real-world log file analysis and reporting

TEST_NAME="Log Analysis Scenario"
TEST_TYPE="scenario"
TIMEOUT=40

# Source test framework
source "$(dirname "$0")/../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing real-world log analysis scenario"
    
    # Create realistic log file
    cat > "$TEST_DATA/application.log" << 'EOF'
2023-04-10 08:00:00.123 [INFO] Application startup initiated
2023-04-10 08:00:01.456 [INFO] Database connection established
2023-04-10 08:00:02.789 [INFO] Cache initialization complete
2023-04-10 08:00:03.012 [INFO] Server listening on port 8080
2023-04-10 08:15:23.456 [WARN] High memory usage detected: 85%
2023-04-10 08:30:45.789 [ERROR] Database query timeout: SELECT * FROM users
2023-04-10 08:31:12.345 [INFO] Database connection restored
2023-04-10 09:15:33.678 [ERROR] Authentication failure for user: hacker@evil.com
2023-04-10 09:16:45.901 [WARN] Multiple failed login attempts from IP: 192.168.1.100
2023-04-10 09:45:12.234 [ERROR] Memory allocation failed in user session handler
2023-04-10 10:00:00.567 [INFO] Scheduled backup started
2023-04-10 10:15:30.890 [ERROR] Backup process failed: insufficient disk space
2023-04-10 10:16:00.123 [WARN] Disk usage critical: 95%
2023-04-10 11:30:45.456 [INFO] User admin logged in successfully
2023-04-10 12:00:00.789 [INFO] Daily maintenance routine started
EOF
    
    # Create analysis task
    cat > "$TEST_DATA/task.txt" << 'EOF'
As a system administrator, I need a comprehensive analysis of the application.log file. Please:

1. Extract and count all ERROR entries with their timestamps
2. Identify any security-related events (authentication failures, suspicious IPs)
3. Check for system health issues (memory, disk, database problems)
4. Create a priority-sorted incident report with:
   - Critical issues (errors that need immediate attention)
   - Warning issues (potential problems to monitor)
   - Security concerns (authentication failures, suspicious activity)
   - System resource alerts (memory, disk usage)

Generate two output files:
- incident_report.txt: Executive summary with key findings and recommendations
- detailed_analysis.txt: Technical details with timestamps and specific log entries

This analysis should help prioritize our response to system issues.
EOF
    
    # Execute scenario
    if timeout 40 "$LLMCMD_BIN" \
        -i "$TEST_DATA/application.log" \
        -i "$TEST_DATA/task.txt" \
        -o "$TEST_DATA/incident_report.txt" \
        -o "$TEST_DATA/detailed_analysis.txt" \
        "$TEST_DATA/task.txt" > "$LOG_FILE" 2>&1; then
        
        # Verify analysis results
        if [ -f "$TEST_DATA/incident_report.txt" ] && [ -f "$TEST_DATA/detailed_analysis.txt" ]; then
            
            # Check incident report content
            if grep -q -i "critical\|error\|incident" "$TEST_DATA/incident_report.txt" && \
               (grep -q -i "backup.*fail\|disk.*space\|memory" "$TEST_DATA/incident_report.txt" || \
                grep -q -i "database\|authentication" "$TEST_DATA/incident_report.txt"); then
                
                # Check detailed analysis content
                if grep -q "08:30:45\|09:15:33\|10:15:30" "$TEST_DATA/detailed_analysis.txt" && \
                   grep -q -i "error\|failed" "$TEST_DATA/detailed_analysis.txt"; then
                    log_success "Log analysis scenario completed successfully"
                    return 0
                else
                    log_warning "Scenario completed but detailed analysis may be incomplete"
                    return 1
                fi
            else
                log_error "Incident report missing expected critical analysis"
                cat "$TEST_DATA/incident_report.txt" >> "$LOG_FILE" 2>/dev/null
                return 1
            fi
        else
            log_error "Scenario output files not created"
            return 1
        fi
    else
        log_error "Log analysis scenario failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
