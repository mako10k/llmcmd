#!/bin/bash
# Test: Large data processing scenario
# Purpose: Test performance with larger datasets

TEST_NAME="Large Data Processing"
TEST_TYPE="scenario"
TIMEOUT=60

# Source test framework
source "$(dirname "$0")/../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing large data processing scenario"
    
    # Create large dataset
    log_info "Generating large test dataset..."
    {
        echo "timestamp,user_id,action,resource,status,response_time"
        for i in {1..1000}; do
            timestamp="2023-04-$(printf "%02d" $((i % 30 + 1))) $(printf "%02d" $((i % 24))):$(printf "%02d" $((i % 60))):$(printf "%02d" $((i % 60)))"
            user_id="user_$((i % 100 + 1))"
            action=$([ $((i % 3)) -eq 0 ] && echo "GET" || ([ $((i % 3)) -eq 1 ] && echo "POST" || echo "PUT"))
            resource="/api/v1/resource_$((i % 50 + 1))"
            status=$([ $((i % 10)) -eq 0 ] && echo "ERROR" || ([ $((i % 20)) -eq 0 ] && echo "WARN" || echo "SUCCESS"))
            response_time=$((RANDOM % 5000 + 100))
            echo "$timestamp,$user_id,$action,$resource,$status,$response_time"
        done
    } > "$TEST_DATA/large_access_log.csv"
    
    # Create performance analysis task
    cat > "$TEST_DATA/analysis_task.txt" << 'EOF'
Please perform comprehensive analysis of the large_access_log.csv file:

Performance Analysis Required:
1. Calculate average response time by action type (GET, POST, PUT)
2. Identify the top 10 slowest requests (highest response_time)
3. Count error rates by user_id (users with most errors)
4. Find peak usage hours (most requests per hour)
5. Analyze resource popularity (most accessed endpoints)

Output Requirements:
- performance_summary.txt: Executive summary with key metrics
- detailed_metrics.txt: Detailed statistics and analysis
- error_analysis.txt: Focus on error patterns and problematic users
- optimization_recommendations.txt: Suggestions for system improvements

The analysis should handle this large dataset efficiently and provide actionable insights.
EOF
    
    # Execute performance scenario
    log_info "Starting large data analysis (this may take a while)..."
    if timeout 60 "$LLMCMD_BIN" \
        -i "$TEST_DATA/large_access_log.csv" \
        -i "$TEST_DATA/analysis_task.txt" \
        -o "$TEST_DATA/performance_summary.txt" \
        -o "$TEST_DATA/detailed_metrics.txt" \
        -o "$TEST_DATA/error_analysis.txt" \
        -o "$TEST_DATA/optimization_recommendations.txt" \
        "$TEST_DATA/analysis_task.txt" > "$LOG_FILE" 2>&1; then
        
        # Verify performance analysis results
        if [ -f "$TEST_DATA/performance_summary.txt" ] && \
           [ -f "$TEST_DATA/detailed_metrics.txt" ] && \
           [ -f "$TEST_DATA/error_analysis.txt" ]; then
            
            # Check summary content
            if grep -q -i "response.*time\|average\|performance" "$TEST_DATA/performance_summary.txt" && \
               (grep -q -i "GET\|POST\|PUT" "$TEST_DATA/performance_summary.txt" || \
                grep -q -i "error\|request" "$TEST_DATA/performance_summary.txt"); then
                
                # Check if detailed metrics exist
                if [ -s "$TEST_DATA/detailed_metrics.txt" ]; then
                    log_success "Large data processing scenario completed successfully"
                    
                    # Log performance metrics
                    file_size=$(wc -l < "$TEST_DATA/large_access_log.csv")
                    log_info "Processed $file_size lines of data"
                    return 0
                else
                    log_warning "Processing completed but detailed metrics may be incomplete"
                    return 1
                fi
            else
                log_error "Performance summary missing expected analysis content"
                cat "$TEST_DATA/performance_summary.txt" >> "$LOG_FILE" 2>/dev/null
                return 1
            fi
        else
            log_error "Large data processing output files not created"
            return 1
        fi
    else
        log_error "Large data processing scenario failed or timed out"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
