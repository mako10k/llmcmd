#!/bin/bash
# Test: CSV data transformation pipeline
# Purpose: Verify complete data processing pipeline from CSV input to formatted output

TEST_NAME="CSV Data Transformation"
TEST_TYPE="pipeline"
TIMEOUT=25

# Source test framework
source "$(dirname "$0")/../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing CSV data transformation pipeline"
    
    # Create source CSV data
    cat > "$TEST_DATA/sales_data.csv" << 'EOF'
product,quantity,price,region
Laptop,5,1200.00,North
Mouse,25,25.50,South
Keyboard,15,75.00,East
Monitor,8,300.00,North
Laptop,3,1200.00,South
Mouse,30,25.50,North
Keyboard,12,75.00,West
EOF
    
    # Create pipeline instruction
    cat > "$TEST_DATA/instruction.txt" << 'EOF'
Please process the sales_data.csv file through the following pipeline:

1. Read the CSV data
2. Calculate total revenue for each product (quantity * price)
3. Group by product and sum the total quantities and revenues
4. Sort by total revenue (highest first)
5. Create a formatted report in report.txt with:
   - Header: "Sales Analysis Report"
   - Each product line: "Product: [name], Total Qty: [qty], Total Revenue: $[revenue]"
   - Footer: "Report generated successfully"

Please use available text processing tools to accomplish this transformation.
EOF
    
    # Execute pipeline
    if timeout 25 "$LLMCMD_BIN" \
        -i "$TEST_DATA/sales_data.csv" \
        -i "$TEST_DATA/instruction.txt" \
        -o "$TEST_DATA/report.txt" \
        "$TEST_DATA/instruction.txt" > "$LOG_FILE" 2>&1; then
        
        # Verify pipeline results
        if [ -f "$TEST_DATA/report.txt" ] && \
           grep -q "Sales Analysis Report" "$TEST_DATA/report.txt" && \
           grep -q "Laptop" "$TEST_DATA/report.txt" && \
           grep -q "Report generated successfully" "$TEST_DATA/report.txt"; then
            
            # Check if revenue calculations appear reasonable
            if grep -q -E "(9600|9,600)" "$TEST_DATA/report.txt" || \
               grep -q "Laptop.*Total Revenue" "$TEST_DATA/report.txt"; then
                log_success "CSV data transformation pipeline completed successfully"
                return 0
            else
                log_warning "Pipeline completed but revenue calculations may be incorrect"
                cat "$TEST_DATA/report.txt" >> "$LOG_FILE" 2>/dev/null
                return 1
            fi
        else
            log_error "Pipeline output missing expected format or content"
            cat "$TEST_DATA/report.txt" >> "$LOG_FILE" 2>/dev/null
            return 1
        fi
    else
        log_error "CSV data transformation pipeline failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
